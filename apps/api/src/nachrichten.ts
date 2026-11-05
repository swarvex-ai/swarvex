// Nachrichtenweg zwischen Lead, Visionär und Bereichs-Leads eines Projekts. Ersetzt das geplante request_approval:
// eine Freigabe ist eine Nachricht der Art „freigabe“ an den Visionär. Inhalte sind projektintern und für den
// Empfänger fremde Eingaben; Freigaben stehen mit Betreff und Entscheidung im öffentlichen Protokoll.
import { transaktion, type Db } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt, officersVon, projektPerSlug, rollenVon, type Wer } from "./aufgaben.ts";

export type Art = "frage" | "freigabe" | "info";
const AN = /^(lead|visionaer|bereich:[a-z][a-z0-9-]{1,30})$/;
export const NACHRICHTEN_JE_TAG = 50;

/** Adressen, an die ein Mensch in diesem Projekt schreibt und von denen er liest. Leer = keine Rolle im Projekt. */
async function adressenVon(db: Db, menschId: number, projektId: number) {
  const rollen = await rollenVon(db, menschId, projektId);
  const bereiche = (await officersVon(db, menschId, projektId)).map((o) => `bereich:${o.bereich}`);
  const eigene = [...(rollen.includes("lead") ? ["lead"] : []), ...(rollen.includes("visionaer") ? ["visionaer", "lead"] : []), ...bereiche];
  return { eigene: [...new Set(eigene)], visionaer: rollen.includes("visionaer"), beteiligt: rollen.includes("lead") || rollen.includes("visionaer") || bereiche.length > 0 };
}

export async function nachrichtSenden(db: Db, wer: Wer, projektSlug: string, n: { an: string; art: Art; betreff: string; inhalt: string; aufgaben?: number[] }) {
  if (!AN.test(n.an)) throw new Abgelehnt("to: lead, visionary or area:<name>");
  if (n.art === "freigabe" && n.an !== "visionaer") throw new Abgelehnt("approvals go to the visionary");
  return transaktion(db, async (tx) => {
    const p = await projektPerSlug(tx, projektSlug);
    const ich = await adressenVon(tx as unknown as Db, wer.menschId, Number(p.id));
    if (!ich.beteiligt) throw new Abgelehnt("only the project's lead, visionary and area leads use messages");
    if (n.an.startsWith("bereich:")) {
      const da = (await tx.query("select 1 from officers where projekt_id = $1 and bereich = $2 and beendet is null", [p.id, n.an.slice(8)])).rowCount;
      if (!da) throw new Abgelehnt(`no active area lead for ${n.an.slice(8)}`);
    }
    const heute = Number((await tx.query("select count(*) from nachrichten where von_mensch = $1 and projekt_id = $2 and erstellt > now() - interval '1 day'", [wer.menschId, p.id])).rows[0].count);
    if (heute >= NACHRICHTEN_JE_TAG) throw new Abgelehnt(`at most ${NACHRICHTEN_JE_TAG} messages per day and project`);
    const aufgaben = n.aufgaben ?? [];
    if (aufgaben.length) {
      const da = Number((await tx.query("select count(*) from aufgaben where projekt_id = $1 and id = any($2)", [p.id, aufgaben])).rows[0].count);
      if (da !== new Set(aufgaben).size) throw new Abgelehnt("tasks must belong to this project");
    }
    const r = (await tx.query(
      `insert into nachrichten(projekt_id, von_mensch, von_agent, an, art, betreff, inhalt, aufgaben, zustand)
       values ($1,$2,$3,$4,$5,$6,$7,$8,$9) returning id, erstellt`,
      [p.id, wer.menschId, wer.agentId, n.an, n.art, n.betreff.trim(), n.inhalt, aufgaben, n.art === "info" ? "erledigt" : "offen"])).rows[0];
    if (n.art === "freigabe") await protokollieren(tx, { projekt_id: Number(p.id), akteur: `agent:${wer.agentId}`, art: "freigabe_angefragt", bezug: `nachricht:${r.id}`, daten: { betreff: n.betreff.trim(), aufgaben } });
    return { nachricht_id: Number(r.id), erstellt: r.erstellt };
  });
}

/** Posteingang: an mich und von mir, offene zuerst; Infos und Erledigtes der letzten 30 Tage. */
export async function posteingang(db: Db, wer: Wer, projektSlug: string, nurOffen = false) {
  const p = await projektPerSlug(db, projektSlug);
  const ich = await adressenVon(db, wer.menschId, Number(p.id));
  if (!ich.beteiligt) throw new Abgelehnt("only the project's lead, visionary and area leads use messages");
  const r = await db.query(
    `select n.id as nachricht_id, n.an, n.art as nachricht_art, n.betreff, n.inhalt, n.aufgaben, n.zustand, n.antwort, n.entscheidung,
            n.erstellt, n.beantwortet_am, coalesce(m.login, 'swarvex') as von, b.login as beantwortet_von,
            n.an = any($3) as an_mich, n.von_mensch = $2 as von_mir
     from nachrichten n left join menschen m on m.id = n.von_mensch left join menschen b on b.id = n.beantwortet_von
     where n.projekt_id = $1 and (n.an = any($3) or n.von_mensch = $2)
       and ($4::boolean is false or n.zustand = 'offen')
       and (n.zustand = 'offen' or n.erstellt > now() - interval '30 days')
     order by (n.zustand = 'offen' and n.an = any($3)) desc, n.erstellt desc limit 100`, [p.id, wer.menschId, ich.eigene, nurOffen]);
  return r.rows.map((z) => ({ ...z, nachricht_id: Number(z.nachricht_id), aufgaben: z.aufgaben.map(Number), an: anAussen(z.an) }));
}

/** Offene Nachrichten an mich: Anzahl und Alter der ältesten (für project_context). */
export async function offeneFuerMich(db: Db, menschId: number, projektId: number) {
  const ich = await adressenVon(db, menschId, projektId);
  if (!ich.beteiligt) return null;
  const r = (await db.query(
    "select count(*)::int as offen, round(extract(epoch from now() - min(erstellt)) / 3600, 1) as aelteste_h from nachrichten where projekt_id = $1 and zustand = 'offen' and an = any($2)",
    [projektId, ich.eigene])).rows[0];
  return { offen: r.offen, aelteste_h: r.aelteste_h === null ? null : Number(r.aelteste_h) };
}

export async function nachrichtBeantworten(db: Db, menschId: number, agentId: number | null, nachrichtId: number, antwort: string, entscheidung?: "freigegeben" | "abgelehnt") {
  // Freigeben geht ohne Begründung; wer ablehnt oder antwortet, sagt etwas (sonst weiß der Lead nicht, was er ändern soll)
  if (!antwort.trim() && entscheidung !== "freigegeben")
    throw new Abgelehnt(entscheidung === "abgelehnt" ? "a decline needs a reason, so the lead knows what to change" : "an answer is required");
  return transaktion(db, async (tx) => {
    const n = (await tx.query("select * from nachrichten where id = $1 for update", [nachrichtId])).rows[0];
    if (!n) throw new Abgelehnt("unknown message");
    const ich = await adressenVon(tx as unknown as Db, menschId, Number(n.projekt_id));
    if (!ich.eigene.includes(n.an)) throw new Abgelehnt("only the recipient answers this message");
    // Kein Verbot der Selbstbeantwortung: Visionär und Lead sind oft derselbe Mensch, die Freigabe ist dann seine eigene Entscheidung.
    if (n.zustand !== "offen") throw new Abgelehnt("the message is already answered");
    if (n.art === "freigabe" && !entscheidung) throw new Abgelehnt("an approval needs a decision: approve or decline");
    if (n.art !== "freigabe" && entscheidung) throw new Abgelehnt("only approvals take a decision");
    await tx.query("update nachrichten set zustand = 'beantwortet', antwort = $2, entscheidung = $3, beantwortet_von = $4, beantwortet_am = now() where id = $1",
      [nachrichtId, antwort.trim() || null, entscheidung ?? null, menschId]);
    if (n.art === "freigabe") await protokollieren(tx, { projekt_id: Number(n.projekt_id), akteur: agentId ? `agent:${agentId}` : `mensch:${menschId}`,
      art: "freigabe_entschieden", bezug: `nachricht:${nachrichtId}`, daten: { betreff: n.betreff, entscheidung } });
    return { nachricht_id: nachrichtId, zustand: "beantwortet", entscheidung: entscheidung ?? null };
  });
}

/** Offene Freigaben für den Visionär (Website „Mein Konto“). */
export async function offeneFreigaben(db: Db, menschId: number) {
  const r = await db.query(
    `select n.id as nachricht_id, p.slug as projekt, p.name as projekt_name, n.betreff, n.inhalt, n.aufgaben, n.erstellt, coalesce(m.login, 'swarvex') as von
     from nachrichten n join projekte p on p.id = n.projekt_id left join menschen m on m.id = n.von_mensch
     where n.art = 'freigabe' and n.zustand = 'offen'
       and exists (select 1 from projekt_rollen r where r.projekt_id = n.projekt_id and r.mensch_id = $1 and r.rolle = 'visionaer')
     order by n.erstellt`, [menschId]);
  return r.rows.map((z) => ({ ...z, nachricht_id: Number(z.nachricht_id), aufgaben: z.aufgaben.map(Number) }));
}

export const anAussen = (an: string) => (an === "visionaer" ? "visionary" : an.startsWith("bereich:") ? `area:${an.slice(8)}` : an);
export const anRein = (an: string) => (an === "visionary" ? "visionaer" : an.startsWith("area:") ? `bereich:${an.slice(5)}` : an);
