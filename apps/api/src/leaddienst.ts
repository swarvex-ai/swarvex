// Lead-Dienst (E-38): `swarvex project lead` lässt den Lead eines Projekts als Dienst in einer eigenen Sandbox laufen,
// statt mit allen Rechten auf dem Rechner des Visionärs. Er meldet sich als eigenes Gerät an (Device Flow) und bindet
// diese Anmeldung an genau ein Projekt. Eine gebundene Anmeldung
//   - sieht nur die Lead-Werkzeuge, und nur für dieses Projekt (mcp.ts, LEAD_DIENST_WERKZEUGE),
//   - entscheidet keine Freigaben: Die bleiben beim Menschen (Mein Konto oder seine eigene Sitzung),
//   - holt Git-Zugang nur für das Repository dieses Projekts (kurzlebiges Token der GitHub App, ohne Workflow-Rechte).
// Jeder Aufruf hält die Anwesenheit aktuell; schweigt der Dienst eine Stunde, bekommt der Visionär eine Nachricht.
import { transaktion, type Db } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt, projektPerSlug, rollenVon, type Wer } from "./aufgaben.ts";
import { appBereit, repoAus, repoToken } from "./github.ts";

/** Werkzeuge, die eine an ein Projekt gebundene Lead-Anmeldung sieht. Alles andere (Aufgaben übernehmen, einreichen,
 *  Gegenprüfungen, Peer-Reviews, Medien-Zusagen, Bereichs-Leads ernennen) bleibt Mitstreitern bzw. dem Menschen. */
export const LEAD_DIENST_WERKZEUGE = new Set([
  "project_context", "weekly_report", "metrics", "matching_tasks", "task_details",
  "create_task", "create_tasks", "close_task", "complete_own_task", "rework_patterns", "wait_for_work", "pending_submissions",
  "report_regression", "review_submission", "send_message", "inbox", "answer_message",
  "donation_pledges", "create_donation_brief", "donation_briefs", "review_donation", "preview_feedback", "mark_feedback", "set_translation",
  "take_area", "leave_area", "release_level_c", "report_lead_cycle",
]);

/** Nach so langer Stille gilt der Lead-Dienst als nicht erreichbar (Projektseite, Nachricht an den Visionär). */
export const STILLE_MINUTEN = 60;
/** So lange gilt er nach dem letzten Aufruf als aktiv (er wartet höchstens 10 Minuten am Stück auf Arbeit). */
export const AKTIV_MINUTEN = 15;

/** Bindet die Anmeldung eines Runners an ein Projekt, dessen Lead oder Visionär sein Mensch ist. Endgültig für diese
 *  Anmeldung: lösen heißt abmelden (Mein Konto). */
export async function leadDienstBinden(db: Db, wer: Wer, projektSlug: string) {
  if (wer.runner !== true) throw new Abgelehnt("only the swarvex runner sets up a lead service (swarvex project lead setup)");
  return transaktion(db, async (tx) => {
    const p = await projektPerSlug(tx, projektSlug);
    const rollen = await rollenVon(tx, wer.menschId, Number(p.id));
    if (!rollen.includes("lead") && !rollen.includes("visionaer")) throw new Abgelehnt("only the project's lead or visionary runs its lead service");
    const a = (await tx.query("select lead_projekt_id from agenten where id = $1 for update", [wer.agentId])).rows[0];
    if (a.lead_projekt_id !== null && Number(a.lead_projekt_id) !== Number(p.id)) throw new Abgelehnt("this sign-in already belongs to the lead service of another project");
    if (a.lead_projekt_id === null) {
      await tx.query("update agenten set lead_projekt_id = $2 where id = $1", [wer.agentId, p.id]);
      await protokollieren(tx, { projekt_id: Number(p.id), akteur: `mensch:${wer.menschId}`, art: "lead_dienst_verbunden", bezug: `agent:${wer.agentId}`, daten: {} });
    }
    await tx.query("update projekte set lead_gesehen = now() where id = $1", [p.id]);
    return { projekt: p.slug, gebunden: true };
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

/** Jeder Aufruf des Lead-Dienstes hält seine Anwesenheit aktuell (höchstens einmal je halbe Minute geschrieben). */
export async function leadGesehen(db: Db, projektId: number) {
  await db.query("update projekte set lead_gesehen = now() where id = $1 and (lead_gesehen is null or lead_gesehen < now() - interval '30 seconds')", [projektId]);
}

/** Eine beendete Runde: Der Runner meldet Dauer, Verbrauch, Anlass und Ausgang. */
export async function rundeMelden(db: Db, wer: Wer, projektId: number, r: { sekunden: number; tokens: number; anlass: string; ergebnis: string }) {
  await db.query(
    "insert into lead_runden(projekt_id, agent_id, beginn, sekunden, tokens, anlass, ergebnis) values ($1,$2, now() - make_interval(secs => $3), $3, $4, $5, $6)",
    [projektId, wer.agentId, r.sekunden, r.tokens, r.anlass.slice(0, 200), r.ergebnis.slice(0, 200)]);
  await db.query("update projekte set lead_runde = now(), lead_gesehen = now() where id = $1", [projektId]);
  return "recorded";
}

/** Git-Zugang für das Repository des gebundenen Projekts: ein Token der GitHub App (eine Stunde, nur dieses Repo, Inhalte
 *  schreiben, keine Workflows) und die Autorenangaben des Visionärs. Andere Git-Hosts: Token-Datei beim Runner. */
export async function leadGitZugang(db: Db, menschId: number, projektId: number) {
  const p = (await db.query("select slug, repo_url, repo_git, github_installation from projekte where id = $1", [projektId])).rows[0];
  if (!p.repo_git && !p.repo_url) throw new Abgelehnt("the project has no repository connected yet");
  // Der Lead-Dienst holt und schiebt über http(s) mit Token (ssh-Adressen, etwa eines Gitea im Heimnetz: die Web-Adresse)
  const web = (u: unknown) => typeof u === "string" && /^https?:\/\//.test(u);
  const repo_git = web(p.repo_git) ? String(p.repo_git) : web(p.repo_url) ? `${String(p.repo_url).replace(/\/+$/, "")}.git` : String(p.repo_git);
  const m = (await db.query("select konto_provider, konto_id, login from menschen where id = $1", [menschId])).rows[0];
  const autor = { name: m.login, email: m.konto_provider === "github" ? `${m.konto_id}+${m.login}@users.noreply.github.com` : `${m.login}@users.noreply.swarvex.ai` };
  if (!p.github_installation || !appBereit() || !repoAus(String(p.repo_url ?? repo_git)))
    return { repo_git, autor, token: null, hinweis: "not a GitHub repository with the swarvex app: pass your own token with swarvex project lead setup --git-token-file" };
  const token = await repoToken(Number(p.github_installation), repo_git, true);
  return { repo_git, autor, token, gueltig_bis: new Date(Date.now() + 50 * 60_000).toISOString() };
}

/** Anwesenheit für die Projektseite; null, solange kein Lead-Dienst eingerichtet ist. */
export async function leadAnwesenheit(db: Db, projektId: number) {
  const r = (await db.query(
    `select p.lead_gesehen, p.lead_runde,
            exists (select 1 from agenten a where a.lead_projekt_id = p.id and a.abgemeldet is null) as dienst
     from projekte p where p.id = $1`, [projektId])).rows[0];
  if (!r?.dienst) return null;
  const minuten = r.lead_gesehen ? (Date.now() - new Date(r.lead_gesehen).getTime()) / 60_000 : Infinity;
  return { aktiv: minuten <= AKTIV_MINUTEN, gesehen: r.lead_gesehen, letzte_runde: r.lead_runde };
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
