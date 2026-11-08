// Lead-Dienst (E-38): `swarvex project lead` lässt den Lead eines Projekts als Dienst in einer eigenen Sandbox laufen,
// statt mit allen Rechten auf dem Rechner des Visionärs. Er meldet sich als eigenes Gerät an (Device Flow) und bindet
// diese Anmeldung an genau ein Projekt. Eine gebundene Anmeldung
//   - sieht nur die Lead-Werkzeuge, und nur für dieses Projekt (mcp.ts, LEAD_DIENST_WERKZEUGE),
//   - entscheidet keine Freigaben: Die bleiben beim Menschen (Mein Konto oder seine eigene Sitzung),
//   - holt Git-Zugang nur für das Repository dieses Projekts (kurzlebiges Token der GitHub App, ohne Workflow-Rechte).
// Jeder Aufruf hält die Anwesenheit aktuell; schweigt der Dienst eine Stunde, bekommt der Visionär eine Nachricht.
// Lead-Kraft (E-39): Mehrere Lead-Dienste je Projekt teilen sich die Arbeit. Genau einer hält die Hauptschleife (der
// Halter, projekte.lead_halter); weitere warten in Bereitschaft und übernehmen, wenn er 15 Minuten schweigt. Ein Dienst
// mit Bereich (Bereichs-Sitzung des Leads, take_area, oder Dienst eines Bereichs-Leads) hält nur seinen Bereich.
// Bereichs-Leads binden den Dienst an ihr Projekt wie der Lead, holen das Repository aber nur lesend: Ihre eigene Arbeit
// geht wie jede andere durch Merge-Zug und Gegenprüfung.
import { transaktion, type Db } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt, officersVon, projektPerSlug, rollenVon, type Wer } from "./aufgaben.ts";
import { appBereit, repoAus, repoToken } from "./github.ts";

/** Werkzeuge, die eine an ein Projekt gebundene Lead-Anmeldung sieht. Alles andere (Aufgaben übernehmen, einreichen,
 *  Gegenprüfungen, Peer-Reviews, Medien-Zusagen, Bereichs-Leads ernennen) bleibt Mitstreitern bzw. dem Menschen. */
export const LEAD_DIENST_WERKZEUGE = new Set([
  "project_context", "weekly_report", "metrics", "matching_tasks", "task_details",
  "create_task", "create_tasks", "close_task", "complete_own_task", "rework_patterns", "wait_for_work", "pending_submissions",
  "report_regression", "review_submission", "send_message", "inbox", "answer_message",
  "donation_pledges", "create_donation_brief", "donation_briefs", "review_donation", "preview_feedback", "mark_feedback", "set_translation",
  "take_area", "leave_area", "release_level_c", "report_lead_cycle", "open_lead_position", "close_lead_position",
  "request_work", "work_requests", "answer_work_request", "withdraw_work_request", "submission_images",
  "estimate_milestone", "find_assets",
  "bind_lead_service", // erneutes `swarvex project lead setup`: bestätigt die Bindung (nur dasselbe Projekt)
]);

/** Nach so langer Stille gilt der Lead-Dienst als nicht erreichbar (Projektseite, Nachricht an den Visionär). */
export const STILLE_MINUTEN = 60;
/** So lange gilt er nach dem letzten Aufruf als aktiv (er wartet höchstens 10 Minuten am Stück auf Arbeit). */
export const AKTIV_MINUTEN = 15;
/** Schweigt der Halter der Hauptschleife so lange, übernimmt ein Dienst in Bereitschaft. */
export const HALTER_STILLE_MINUTEN = 15;

/** Bindet die Anmeldung eines Runners an ein Projekt, dessen Lead, Visionär oder Bereichs-Lead sein Mensch ist. Endgültig
 *  für diese Anmeldung: lösen heißt abmelden (Mein Konto). */
export async function leadDienstBinden(db: Db, wer: Wer, projektSlug: string) {
  if (wer.runner !== true) throw new Abgelehnt("only the swarvex runner sets up a lead service (swarvex project lead setup)");
  return transaktion(db, async (tx) => {
    const p = await projektPerSlug(tx, projektSlug);
    const rollen = await rollenVon(tx, wer.menschId, Number(p.id));
    const lead = rollen.includes("lead") || rollen.includes("visionaer");
    const bereiche = lead ? [] : await officersVon(tx, wer.menschId, Number(p.id));
    if (!lead && !bereiche.length) throw new Abgelehnt("only the project's lead, visionary or one of its area leads runs a lead service for it");
    const a = (await tx.query("select lead_projekt_id from agenten where id = $1 for update", [wer.agentId])).rows[0];
    if (a.lead_projekt_id !== null && Number(a.lead_projekt_id) !== Number(p.id)) throw new Abgelehnt("this sign-in already belongs to the lead service of another project");
    if (a.lead_projekt_id === null) {
      await tx.query("update agenten set lead_projekt_id = $2 where id = $1", [wer.agentId, p.id]);
      await protokollieren(tx, { projekt_id: Number(p.id), akteur: `mensch:${wer.menschId}`, art: "lead_dienst_verbunden", bezug: `agent:${wer.agentId}`,
        daten: lead ? {} : { bereich: bereiche[0].bereich } });
    }
    return { projekt: p.slug, gebunden: true, ...(lead ? { rolle: "lead" } : { rolle: "bereichs_lead", bereich: bereiche[0].bereich, pfade: bereiche[0].pfade, faehigkeiten: bereiche[0].faehigkeiten }) };
  });
}

/** Prüft, ob ein Werkzeug-Aufruf einer gebundenen Anmeldung ihr Projekt betrifft. Liefert den Ablehnungsgrund oder null. */
export async function gebundenPruefen(db: Db, projektId: number, werkzeug: string, a: Record<string, unknown>): Promise<string | null> {
  if (!LEAD_DIENST_WERKZEUGE.has(werkzeug)) return "the lead service does not use this tool";
  if (werkzeug === "answer_message" && a.decision !== undefined)
    return "approvals are decided by the visionary (My account on the website, or their own session), not by the lead service";
  const p = (await db.query("select slug from projekte where id = $1", [projektId])).rows[0];
  const fremd = `the lead service works only on project ${p.slug}`;
  if ("project" in a || werkzeug === "create_donation_brief") {
    if (a.project !== p.slug) return werkzeug === "create_donation_brief" && a.project === undefined ? `pass project: "${p.slug}"` : fremd;
  }
  const nachschlagen: [string, string][] = [
    ["task_id", "select projekt_id from aufgaben where id = $1"],
    ["submission_id", "select a.projekt_id from einreichungen e join aufgaben a on a.id = e.aufgabe_id where e.id = $1"],
    ["message_id", "select projekt_id from nachrichten where id = $1"],
    ["brief_id", "select projekt_id from spendenauftraege where id = $1"],
    ["feedback_id", "select projekt_id from vorschau_feedback where id = $1"],
    // Anfragen beantwortet das angefragte Projekt, zurückziehen kann sie nur das anfragende
    ["request_id", werkzeug === "withdraw_work_request" ? "select von_projekt as projekt_id from bedarf where id = $1" : "select an_projekt as projekt_id from bedarf where id = $1"],
  ];
  for (const [feld, sql] of nachschlagen) {
    if (a[feld] === undefined) continue;
    const z = (await db.query(sql, [a[feld]])).rows[0];
    if (z && Number(z.projekt_id) !== projektId) return fremd;
  }
  for (const t of Array.isArray(a.tasks) ? a.tasks : []) {
    // create_tasks: Felder je Aufgabe tragen kein Projekt; depends_on und after verweisen auf Aufgaben, die prüft aufgabeAnlegen
    if (t && typeof t === "object" && "project" in t && (t as { project: unknown }).project !== p.slug) return fremd;
  }
  return null;
}

export type Stand = { halter: boolean; bereich: string | null; halter_name?: string; ohne_amt?: boolean };

/** Jeder Aufruf eines Lead-Dienstes hält sein Lebenszeichen aktuell (höchstens einmal je halbe Minute geschrieben) und
 *  vergibt die Hauptschleife: frei oder ihr Halter schweigt seit HALTER_STILLE_MINUTEN, dann übernimmt dieser Dienst. */
export async function leadGesehen(db: Db, projektId: number, agentId: number): Promise<Stand> {
  const s = (await db.query(
    "select id, bereich, gesehen < now() - interval '30 seconds' as alt from bereichs_sitzungen where projekt_id = $1 and agent_id = $2 and beendet is null",
    [projektId, agentId])).rows[0];
  if (s) {
    if (s.alt) await db.query("update bereichs_sitzungen set gesehen = now() where id = $1", [s.id]);
    return { halter: false, bereich: s.bereich };
  }
  const a = (await db.query(
    `select a.mensch_id, exists (select 1 from projekt_rollen r where r.projekt_id = $1 and r.mensch_id = a.mensch_id and r.rolle in ('lead', 'visionaer')) as lead
     from agenten a where a.id = $2`, [projektId, agentId])).rows[0];
  if (!a.lead) {
    const o = (await officersVon(db, Number(a.mensch_id), projektId))[0];
    return o ? { halter: false, bereich: o.bereich } : { halter: false, bereich: null, ohne_amt: true };
  }
  const p = (await db.query(
    `select p.lead_halter, p.lead_gesehen > now() - interval '30 seconds' as frisch,
            p.lead_halter is null or p.lead_gesehen is null or p.lead_gesehen < now() - make_interval(mins => $2)
              or exists (select 1 from agenten h where h.id = p.lead_halter and (h.abgemeldet is not null or h.lead_projekt_id is distinct from p.id)) as frei,
            (select name from agenten where id = p.lead_halter) as halter_name
     from projekte p where p.id = $1`, [projektId, HALTER_STILLE_MINUTEN])).rows[0];
  if (p.lead_halter !== null && Number(p.lead_halter) === agentId) {
    if (!p.frisch) await db.query("update projekte set lead_gesehen = now() where id = $1 and lead_halter = $2", [projektId, agentId]);
    return { halter: true, bereich: null };
  }
  if (!p.frei) return { halter: false, bereich: null, halter_name: p.halter_name };
  // Übernehmen, nur wenn niemand schneller war
  return transaktion(db, async (tx) => {
    const r = await tx.query("update projekte set lead_halter = $2, lead_gesehen = now() where id = $1 and lead_halter is not distinct from $3",
      [projektId, agentId, p.lead_halter]);
    if (!r.rowCount) return { halter: false, bereich: null };
    if (p.lead_halter !== null) {
      await protokollieren(tx, { projekt_id: projektId, akteur: `agent:${agentId}`, art: "lead_dienst_uebernommen", bezug: `agent:${agentId}`,
        daten: { von: `agent:${p.lead_halter}` } });
    }
    return { halter: true, bereich: null };
  });
}

/** wait_for_work eines Dienstes ohne Hauptschleife und ohne Bereich: Er wartet in Bereitschaft (ohne Runde) und
 *  übernimmt, sobald der Halter schweigt. null: Er arbeitet normal weiter. */
export async function bereitschaft(db: Db, projektId: number, agentId: number, maxSekunden: number,
  schlaf = (ms: number) => new Promise((r) => setTimeout(r, ms))) {
  const ende = Date.now() + Math.min(Math.max(maxSekunden, 0), 600) * 1000;
  for (;;) {
    const st = await leadGesehen(db, projektId, agentId);
    if (st.halter || st.bereich !== null || st.ohne_amt) return null;
    if (Date.now() >= ende) {
      return { bereitschaft: true, halter: st.halter_name ?? null, weiter: true,
        hinweis: `another lead service of this project holds the main loop; this one stands by and takes over when that one is silent for ${HALTER_STILLE_MINUTEN} minutes. To split the work instead, give this one an area (take_area).` };
    }
    await schlaf(Math.min(15_000, Math.max(0, ende - Date.now())));
  }
}

/** Eine beendete Runde: Der Runner meldet Dauer, Verbrauch, Anlass und Ausgang. */
export async function rundeMelden(db: Db, wer: Wer, projektId: number, r: { sekunden: number; tokens: number; anlass: string; ergebnis: string }) {
  await db.query(
    "insert into lead_runden(projekt_id, agent_id, beginn, sekunden, tokens, anlass, ergebnis) values ($1,$2, now() - make_interval(secs => $3), $3, $4, $5, $6)",
    [projektId, wer.agentId, r.sekunden, r.tokens, r.anlass.slice(0, 200), r.ergebnis.slice(0, 200)]);
  await db.query("update projekte set lead_runde = now(), lead_gesehen = now() where id = $1 and lead_halter = $2", [projektId, wer.agentId]);
  return "recorded";
}

/** Git-Zugang für das Repository des gebundenen Projekts: ein Token der GitHub App (eine Stunde, nur dieses Repo, keine
 *  Workflows) und die Autorenangaben. Lead und Visionär schreiben Inhalte, ein Bereichs-Lead liest nur. Andere Git-Hosts:
 *  Token-Datei beim Runner. */
export async function leadGitZugang(db: Db, menschId: number, projektId: number) {
  const p = (await db.query("select slug, repo_url, repo_git, github_installation from projekte where id = $1", [projektId])).rows[0];
  if (!p.repo_git && !p.repo_url) throw new Abgelehnt("the project has no repository connected yet");
  // Der Lead-Dienst holt und schiebt über http(s) mit Token (ssh-Adressen, etwa eines Gitea im Heimnetz: die Web-Adresse)
  const web = (u: unknown) => typeof u === "string" && /^https?:\/\//.test(u);
  const repo_git = web(p.repo_git) ? String(p.repo_git) : web(p.repo_url) ? `${String(p.repo_url).replace(/\/+$/, "")}.git` : String(p.repo_git);
  const m = (await db.query("select konto_provider, konto_id, login from menschen where id = $1", [menschId])).rows[0];
  const autor = { name: m.login, email: m.konto_provider === "github" ? `${m.konto_id}+${m.login}@users.noreply.github.com` : `${m.login}@users.noreply.swarvex.ai` };
  const rollen = await rollenVon(db, menschId, projektId);
  const schreiben = rollen.includes("lead") || rollen.includes("visionaer");
  if (!p.github_installation || !appBereit() || !repoAus(String(p.repo_url ?? repo_git)))
    return { repo_git, autor, schreiben, token: null, hinweis: "not a GitHub repository with the swarvex app: pass your own token with swarvex project lead setup --git-token-file" };
  const token = await repoToken(Number(p.github_installation), repo_git, schreiben);
  return { repo_git, autor, schreiben, token, gueltig_bis: new Date(Date.now() + 50 * 60_000).toISOString() };
}

/** Anwesenheit für die Projektseite; null, solange kein Lead-Dienst eingerichtet ist. */
export async function leadAnwesenheit(db: Db, projektId: number) {
  const r = (await db.query(
    `select p.lead_gesehen, p.lead_runde,
            exists (select 1 from agenten a where a.lead_projekt_id = p.id and a.abgemeldet is null) as dienst,
            (select count(*)::int from agenten a where a.lead_projekt_id = p.id and a.abgemeldet is null) as dienste
     from projekte p where p.id = $1`, [projektId])).rows[0];
  if (!r?.dienst) return null;
  const minuten = r.lead_gesehen ? (Date.now() - new Date(r.lead_gesehen).getTime()) / 60_000 : Infinity;
  return { aktiv: minuten <= AKTIV_MINUTEN, gesehen: r.lead_gesehen, letzte_runde: r.lead_runde, dienste: r.dienste };
}

/** Lead-Dienste, die seit STILLE_MINUTEN schweigen: eine Nachricht an den Visionär je Ausfall. */
export async function leadAlarme(db: Db) {
  const still = (await db.query(
    `select p.id, p.slug, p.lead_gesehen from projekte p
     where not p.gesperrt and p.lead_gesehen < now() - make_interval(mins => $1)
       and (p.lead_alarm is null or p.lead_alarm < p.lead_gesehen)
       and exists (select 1 from agenten a where a.lead_projekt_id = p.id and a.abgemeldet is null)`, [STILLE_MINUTEN])).rows;
  for (const p of still) {
    await transaktion(db, async (tx) => {
      await tx.query(
        `insert into nachrichten(projekt_id, von_mensch, von_agent, an, art, betreff, inhalt, aufgaben, zustand)
         values ($1, null, null, 'visionaer', 'info', $2, $3, '{}', 'erledigt')`,
        [p.id, "Lead service not reachable", `The lead service of ${p.slug} has not been heard from since ${new Date(p.lead_gesehen).toISOString()}. ` +
          "The swarm keeps working (test pipeline, merge train, cross-checks, peer reviews); only decisions of the lead wait. " +
          "On the machine that runs it: swarvex project lead status " + p.slug + " (and swarvex project lead start " + p.slug + ")."]);
      await tx.query("update projekte set lead_alarm = now() where id = $1", [p.id]);
    });
  }
  return still.length;
}
