// Credit-Spenden, Weg „Auftrag + Upload“: Zusagen von Spendern, Aufträge des Leads, Upload im Browser.
// Kein Schlüssel verlässt das Konto des Spenders; die Plattform speichert nur die hochgeladene Datei.
import { createHash, randomBytes } from "node:crypto";
import { createReadStream, createWriteStream } from "node:fs";
import { mkdir, open, rename, rm, stat } from "node:fs/promises";
import type { IncomingMessage, ServerResponse } from "node:http";
import { extname, join } from "node:path";
import { transaktion, type Db, type Tx } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { vorGenerierung } from "./katalog.ts";
import { Abgelehnt, officersVon, projektPerSlug, rollenVon } from "./aufgaben.ts";
import { inhaltPasst, RUNNER_TYPEN } from "./medien.ts";

/** Upload-Speicher (Docker-Volume spenden); beim Aufruf gelesen, damit Tests ihn umlenken können. */
export const speicher = () => process.env.SPENDEN_DIR ?? "/data/spenden";
/** Häufige Dienste mit verfallenden Credits; „andere“ für alles Weitere (Name in der Notiz). */
export const DIENSTE = ["meshy", "higgsfield", "elevenlabs", "suno", "runway", "midjourney", "leonardo", "kling", "andere"] as const;
/** Dateitypen, die hochgeladen werden dürfen (Endungen ohne Punkt). */
export const DATEITYPEN = ["glb", "gltf", "fbx", "obj", "zip", "png", "jpg", "jpeg", "webp", "svg", "mp4", "webm", "mov", "wav", "mp3", "ogg", "flac", "txt", "md"] as const;
export const ZUSAGEN_JE_MENSCH = 10;
export const UPLOADS_JE_TAG = 30;
/** Zusage über den Runner: Medien-Aufträge je Tag (über alle Dienste des Agenten), Standard und Höchstwert. */
export const MEDIEN_JE_TAG = 10;
export const MEDIEN_JE_TAG_MAX = 30;
/** So lange gilt ein begonnener Auftrag als in Arbeit; danach darf der Runner ihn neu beginnen. */
const BEGONNEN_SPERRE = "3 hours";
/** Pause nach einem Fehlschlag (zurückgegeben ohne Ergebnis), damit ein Auftrag, der immer scheitert, keine Credits verbrennt */
const ABBRUCH_PAUSE = "24 hours";
const offen = ["offen", "hochgeladen", "nacharbeit", "angenommen"];

async function darfSteuern(db: Db, menschId: number, projektId: number) {
  const rollen = await rollenVon(db, menschId, projektId);
  if (rollen.includes("lead") || rollen.includes("visionaer")) return;
  if ((await officersVon(db, menschId, projektId)).length) return;
  throw new Abgelehnt("only the project's lead, visionary or area leads manage credit donations");
}

export async function zusageAnlegen(db: Db, menschId: number, z: { projekt: string; dienst: string; credits: number; verfaellt: string; notiz?: string }) {
  if (!DIENSTE.includes(z.dienst as never)) throw new Abgelehnt(`service: ${DIENSTE.join(", ")}`);
  if (!Number.isInteger(z.credits) || z.credits < 1 || z.credits > 1_000_000) throw new Abgelehnt("credits: a whole number from 1 to 1,000,000");
  const bis = new Date(`${z.verfaellt}T23:59:59Z`);
  const heute = new Date(new Date().toISOString().slice(0, 10) + "T00:00:00Z");
  if (Number.isNaN(bis.getTime()) || bis < heute) throw new Abgelehnt("expiry date must be today or later");
  if (bis.getTime() - heute.getTime() > 400 * 86_400_000) throw new Abgelehnt("expiry date at most 400 days ahead");
  return transaktion(db, async (tx) => {
    const p = await projektPerSlug(tx, z.projekt);
    const n = Number((await tx.query("select count(*) from zusagen where mensch_id = $1 and zustand = 'offen'", [menschId])).rows[0].count);
    if (n >= ZUSAGEN_JE_MENSCH) throw new Abgelehnt(`at most ${ZUSAGEN_JE_MENSCH} open pledges per person`);
    const r = (await tx.query("insert into zusagen(mensch_id, projekt_id, dienst, credits, verfaellt, notiz) values ($1,$2,$3,$4,$5,$6) returning id",
      [menschId, p.id, z.dienst, z.credits, z.verfaellt, (z.notiz ?? "").slice(0, 500)])).rows[0];
    await protokollieren(tx, { projekt_id: Number(p.id), akteur: `mensch:${menschId}`, art: "zusage_eingegangen", bezug: `zusage:${r.id}`, daten: { dienst: z.dienst, credits: z.credits, verfaellt: z.verfaellt } });
    return { zusage_id: Number(r.id) };
  });
}

export async function zusageBeenden(db: Db, menschId: number, zusageId: number) {
  return transaktion(db, async (tx) => {
    const z = (await tx.query("select * from zusagen where id = $1 and mensch_id = $2 for update", [zusageId, menschId])).rows[0];
    if (!z) throw new Abgelehnt("unknown pledge");
    if (z.zustand !== "offen") throw new Abgelehnt("the pledge has already ended");
    await tx.query("update zusagen set zustand = 'beendet' where id = $1", [zusageId]);
    await tx.query("update spendenauftraege set zustand = 'zurueckgezogen', entschieden = now() where zusage_id = $1 and zustand in ('offen', 'nacharbeit')", [zusageId]);
    await protokollieren(tx, { projekt_id: Number(z.projekt_id), akteur: `mensch:${menschId}`, art: "zusage_beendet", bezug: `zusage:${zusageId}`, daten: { dienst: z.dienst } });
  });
}

/** Für „Mein Konto“: eigene Zusagen mit ihren Aufträgen. */
export async function meineZusagen(db: Db, menschId: number) {
  const z = (await db.query(
    `select z.id as zusage_id, p.slug as projekt, p.name as projekt_name, z.dienst, z.credits, z.verfaellt, z.notiz, z.zustand, z.erstellt, z.weg, z.tageslimit
     from zusagen z left join projekte p on p.id = z.projekt_id where z.mensch_id = $1 order by z.zustand, z.erstellt desc limit 50`, [menschId])).rows;
  const a = (await db.query(
    `select a.id as auftrag_id, a.zusage_id, a.titel, a.anleitung, a.zielpfad, a.dateitypen, a.max_mb, a.credits, a.zustand, a.dateiname, a.hochgeladen, a.begruendung,
            a.katalog_treffer, a.katalog_begruendung, a.erstellt
     from spendenauftraege a join zusagen z on z.id = a.zusage_id where z.mensch_id = $1 order by a.erstellt`, [menschId])).rows;
  return z.map((x) => ({ ...x, zusage_id: Number(x.zusage_id), verfaellt: datumText(x.verfaellt),
    auftraege: a.filter((y) => Number(y.zusage_id) === Number(x.zusage_id)).map((y) => ({ ...y, auftrag_id: Number(y.auftrag_id), zusage_id: Number(y.zusage_id) })) }));
}

/** Für Lead, Visionär und Bereichs-Leads: offene Zusagen mit verbleibenden Credits. */
export async function zusagenFuerLead(db: Db, menschId: number, projektSlug: string) {
  const p = await projektPerSlug(db, projektSlug);
  await darfSteuern(db, menschId, Number(p.id));
  const r = await db.query(
    `select z.id as zusage_id, m.login as spender, z.dienst, z.credits, z.verfaellt, z.notiz, z.erstellt, z.weg, z.agent_id, z.tageslimit,
            (z.projekt_id is null) as alle_projekte,
            coalesce((select sum(a.credits) from spendenauftraege a where a.zusage_id = z.id and a.zustand = any($2)), 0)::int as verplant
     from zusagen z join menschen m on m.id = z.mensch_id
     where (z.projekt_id = $1 or (z.weg = 'runner' and z.projekt_id is null)) and z.zustand = 'offen'
       and (z.verfaellt is null or z.verfaellt >= current_date)
     order by z.weg = 'runner' desc, z.verfaellt nulls first`, [p.id, offen]);
  const heute = new Map<number, number>();
  for (const x of r.rows) if (x.weg === "runner" && !heute.has(Number(x.agent_id))) heute.set(Number(x.agent_id), await heuteBegonnen(db, Number(x.agent_id)));
  return r.rows.map(({ agent_id, verplant, ...x }) => x.weg === "runner"
    ? { ...x, zusage_id: Number(x.zusage_id), verfaellt: null, credits: null, heute_frei: Math.max(0, x.tageslimit - (heute.get(Number(agent_id)) ?? 0)) }
    : { ...x, zusage_id: Number(x.zusage_id), verfaellt: datumText(x.verfaellt), tageslimit: undefined, alle_projekte: undefined, verplant, frei: x.credits - verplant });
}

const datumText = (d: unknown) => d === null || d === undefined ? null : d instanceof Date ? d.toISOString().slice(0, 10) : String(d);

/** Wie viele Medien-Aufträge der Runner-Agent heute (UTC) begonnen hat. */
async function heuteBegonnen(db: Db | Tx, agentId: number): Promise<number> {
  return Number((await db.query(
    "select count(*) from ereignisse where akteur = $1 and art = 'spendenauftrag_begonnen' and zeit >= date_trunc('day', now())", [`agent:${agentId}`])).rows[0].count);
}

export async function auftragAnlegen(db: Db, menschId: number, a: { zusage_id: number; titel: string; anleitung: string; zielpfad: string; dateitypen: string[]; max_mb?: number; credits: number; projekt?: string; katalog_begruendung?: string }) {
  const typen = a.dateitypen.map((t) => t.toLowerCase().replace(/^\./, ""));
  if (!typen.length || typen.some((t) => !DATEITYPEN.includes(t as never))) throw new Abgelehnt(`file types: ${DATEITYPEN.join(", ")}`);
  if (a.zielpfad.includes("..") || a.zielpfad.startsWith("/")) throw new Abgelehnt("target path: relative path inside the repository");
  if (!Number.isInteger(a.credits) || a.credits < 1) throw new Abgelehnt("credits: estimated cost, a whole number of at least 1");
  // Erst in Formwerks Katalog nachsehen: Credits nur für Assets, die es dort nicht schon gibt (src/katalog.ts)
  const katalog = await vorGenerierung(db, `${a.titel}\n${a.anleitung}`, a.katalog_begruendung, "brief");
  return transaktion(db, async (tx) => {
    const z = (await tx.query("select * from zusagen where id = $1 for update", [a.zusage_id])).rows[0];
    if (!z) throw new Abgelehnt("unknown pledge");
    // Eine Runner-Zusage ohne Projekt gilt für jedes Projekt: der Auftrag nennt es
    let projektId = z.projekt_id === null ? null : Number(z.projekt_id);
    if (a.projekt) {
      const p = await projektPerSlug(tx, a.projekt);
      if (projektId !== null && projektId !== Number(p.id)) throw new Abgelehnt(`this pledge is for another project`);
      projektId = Number(p.id);
    }
    if (projektId === null) throw new Abgelehnt("project: this pledge is open to every project, so name yours");
    await darfSteuern(tx as unknown as Db, menschId, projektId);
    if (z.zustand !== "offen" || (z.verfaellt && new Date(z.verfaellt) < new Date(new Date().toISOString().slice(0, 10)))) throw new Abgelehnt("the pledge is no longer open");
    if (z.weg === "runner" && typen.some((t) => !RUNNER_TYPEN.includes(t)))
      throw new Abgelehnt(`file types via the runner: ${RUNNER_TYPEN.join(", ")} (their content is checked; texts and archives come through the website)`);
    if (z.weg === "website") {
      const verplant = Number((await tx.query("select coalesce(sum(credits), 0) as s from spendenauftraege where zusage_id = $1 and zustand = any($2)", [z.id, offen])).rows[0].s);
      if (verplant + a.credits > z.credits) throw new Abgelehnt(`the pledge has ${z.credits - verplant} credits left`);
    }
    const r = (await tx.query(
      `insert into spendenauftraege(zusage_id, projekt_id, von_mensch, titel, anleitung, zielpfad, dateitypen, max_mb, credits, katalog_treffer, katalog_begruendung) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) returning id`,
      [z.id, projektId, menschId, a.titel, a.anleitung, a.zielpfad, typen, Math.min(Math.max(a.max_mb ?? 200, 1), 500), a.credits, katalog.treffer, katalog.begruendung])).rows[0];
    await protokollieren(tx, { projekt_id: projektId, akteur: `mensch:${menschId}`, art: "spendenauftrag_angelegt", bezug: `spendenauftrag:${r.id}`, daten: { zusage: Number(z.id), titel: a.titel, credits: a.credits, katalog_treffer: katalog.treffer } });
    return { auftrag_id: Number(r.id) };
  });
}

export async function auftraegeFuerLead(db: Db, menschId: number, projektSlug: string, nurOffen = false) {
  const p = await projektPerSlug(db, projektSlug);
  await darfSteuern(db, menschId, Number(p.id));
  const r = await db.query(
    `select a.id as auftrag_id, a.zusage_id, m.login as spender, z.dienst, a.titel, a.zielpfad, a.dateitypen, a.credits, a.zustand,
            a.dateiname, a.sha256, a.bytes, a.hochgeladen, a.begruendung, a.abgebrochen, a.abbruchgrund, a.katalog_treffer, a.katalog_begruendung, a.erstellt
     from spendenauftraege a join zusagen z on z.id = a.zusage_id join menschen m on m.id = z.mensch_id
     where a.projekt_id = $1 and ($2::boolean is false or a.zustand in ('offen', 'hochgeladen', 'nacharbeit')) order by (a.zustand = 'hochgeladen') desc, a.erstellt`, [p.id, nurOffen]);
  return r.rows.map((x) => ({ ...x, auftrag_id: Number(x.auftrag_id), zusage_id: Number(x.zusage_id), bytes: x.bytes === null ? null : Number(x.bytes),
    datei_url: x.zustand === "hochgeladen" || x.zustand === "angenommen" ? `/donations/${x.auftrag_id}/file` : null }));
}

/** Upload vom Spender (Website, eigene Herkunft und Sitzung geprüft): als Strom auf die Platte, mit SHA-256 und Größenlimit. */
export async function hochladen(db: Db, menschId: number, auftragId: number, req: IncomingMessage, dateiname: string, runnerAgent?: number) {
  const a = (await db.query(
    `select a.*, z.mensch_id as spender, z.weg, z.agent_id as zusage_agent, p.slug
     from spendenauftraege a join zusagen z on z.id = a.zusage_id join projekte p on p.id = a.projekt_id where a.id = $1`, [auftragId])).rows[0];
  if (!a || Number(a.spender) !== menschId) throw new Abgelehnt("unknown brief");
  if (!["offen", "nacharbeit"].includes(a.zustand)) throw new Abgelehnt("this brief takes no upload right now");
  // Der Runner lädt nur zu Aufträgen seiner eigenen Zusage hoch, die er begonnen hat (das Tageslimit zählt beim Beginnen)
  if (runnerAgent !== undefined && (a.weg !== "runner" || Number(a.zusage_agent) !== runnerAgent || !a.begonnen)) throw new Abgelehnt("start the brief first (start_media_brief)");
  const endung = extname(dateiname).toLowerCase().replace(/^\./, "");
  if (!a.dateitypen.includes(endung)) throw new Abgelehnt(`file type .${endung} is not allowed here (allowed: ${a.dateitypen.join(", ")})`);
  if (runnerAgent !== undefined && !RUNNER_TYPEN.includes(endung)) throw new Abgelehnt(`file type .${endung} does not come through the runner (${RUNNER_TYPEN.join(", ")})`);
  const max = a.max_mb * 1024 * 1024;
  const laenge = Number(req.headers["content-length"] ?? 0);
  if (!laenge || laenge > max) throw new Abgelehnt(`file too large or size unknown (at most ${a.max_mb} MB)`);
  const heute = Number((await db.query("select count(*) from spendenauftraege a join zusagen z on z.id = a.zusage_id where z.mensch_id = $1 and a.hochgeladen > now() - interval '1 day'", [menschId])).rows[0].count);
  if (runnerAgent === undefined && heute >= UPLOADS_JE_TAG) throw new Abgelehnt(`at most ${UPLOADS_JE_TAG} uploads per day`);
  const ordner = join(speicher(), a.slug);
  await mkdir(ordner, { recursive: true });
  const tmp = join(ordner, `.${auftragId}-${randomBytes(6).toString("hex")}.part`);
  const hash = createHash("sha256");
  let bytes = 0;
  await new Promise<void>((ok, fehler) => {
    const aus = createWriteStream(tmp, { flags: "wx" });
    req.on("data", (teil: Buffer) => {
      bytes += teil.length;
      if (bytes > max) { req.destroy(); aus.destroy(); fehler(new Abgelehnt(`file larger than ${a.max_mb} MB`)); return; }
      hash.update(teil);
    });
    req.pipe(aus);
    aus.on("finish", ok);
    aus.on("error", fehler);
    req.on("error", fehler);
  }).catch(async (e) => { await rm(tmp, { force: true }); throw e; });
  // Passt der Inhalt zur Endung (medien.ts)? Sonst bleibt nichts liegen.
  const f = await open(tmp, "r");
  const kopf = Buffer.alloc(Math.min(bytes, 65536));
  await f.read(kopf, 0, kopf.length, 0).finally(() => f.close());
  if (inhaltPasst(endung, kopf) === false) {
    await rm(tmp, { force: true });
    throw new Abgelehnt(`the file's content is not a .${endung} file`);
  }
  const sha = hash.digest("hex");
  const ziel = join(ordner, `${auftragId}-${sha.slice(0, 16)}.${endung}`);
  // Zustand und Tageslimit noch einmal unter Sperre prüfen: Gleichzeitige Uploads kamen sonst alle an den Prüfungen
  // oben vorbei, überschritten das Limit und ließen Dateien liegen, die das Aufräumen nie löscht (Sicherheitsaudit
  // 12.11.2026). Nur der Gewinner bekommt seinen Dateinamen; ein Verlierer löscht seine Datei.
  let alt: string | null = null;
  try {
    await transaktion(db, async (tx) => {
      await tx.query("select pg_advisory_xact_lock(hashtext('spende-upload:' || $1::text))", [menschId]);
      const jetzt = (await tx.query("select zustand, datei from spendenauftraege where id = $1 for update", [auftragId])).rows[0];
      if (!["offen", "nacharbeit"].includes(jetzt?.zustand)) throw new Abgelehnt("this brief takes no upload right now");
      const n = Number((await tx.query("select count(*) from spendenauftraege a join zusagen z on z.id = a.zusage_id where z.mensch_id = $1 and a.hochgeladen > now() - interval '1 day'", [menschId])).rows[0].count);
      if (runnerAgent === undefined && n >= UPLOADS_JE_TAG) throw new Abgelehnt(`at most ${UPLOADS_JE_TAG} uploads per day`);
      await rename(tmp, ziel);
      alt = jetzt.datei;
      await tx.query("update spendenauftraege set zustand = 'hochgeladen', datei = $2, dateiname = $3, sha256 = $4, bytes = $5, hochgeladen = now(), begonnen = null, abgebrochen = null, abbruchgrund = null where id = $1",
        [auftragId, ziel, dateiname.slice(0, 200), sha, bytes]);
      await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: runnerAgent !== undefined ? `agent:${runnerAgent}` : `mensch:${menschId}`, art: "spende_hochgeladen",
        bezug: `spendenauftrag:${auftragId}`, daten: { sha256: sha, bytes, ...(runnerAgent !== undefined ? { weg: "runner" } : {}) } });
    });
  } catch (e) {
    await rm(tmp, { force: true });
    throw e;
  }
  if (alt && alt !== ziel) await rm(alt, { force: true });
  return { auftrag_id: auftragId, sha256: sha, bytes };
}

export async function spendeBewerten(db: Db, menschId: number, agentId: number, auftragId: number, urteil: "annehmen" | "nacharbeit" | "ablehnen", begruendung: string) {
  if (!begruendung.trim()) throw new Abgelehnt("a reason is required");
  return transaktion(db, async (tx) => {
    const a = (await tx.query("select a.*, z.mensch_id as spender from spendenauftraege a join zusagen z on z.id = a.zusage_id where a.id = $1 for update", [auftragId])).rows[0];
    if (!a) throw new Abgelehnt("unknown brief");
    await darfSteuern(tx as unknown as Db, menschId, Number(a.projekt_id));
    if (Number(a.spender) === menschId) throw new Abgelehnt("nobody reviews their own donation");
    if (a.zustand !== "hochgeladen") throw new Abgelehnt("only uploaded results can be reviewed");
    const neu = { annehmen: "angenommen", nacharbeit: "nacharbeit", ablehnen: "abgelehnt" }[urteil];
    await tx.query("update spendenauftraege set zustand = $2, begruendung = $3, entschieden = now() where id = $1", [auftragId, neu, begruendung]);
    await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: `agent:${agentId}`, art: "spende_bewertet", bezug: `spendenauftrag:${auftragId}`, daten: { urteil, credits: a.credits } });
    return { auftrag_id: auftragId, zustand: neu };
  });
}

/** Datei an Lead, Visionär oder Bereichs-Lead des Projekts (Agenten-Token), immer als Download. */
export async function dateiSenden(db: Db, menschId: number, auftragId: number, res: ServerResponse) {
  const a = (await db.query("select * from spendenauftraege where id = $1", [auftragId])).rows[0];
  if (!a || !a.datei) throw new Abgelehnt("no file for this brief");
  await darfSteuern(db, menschId, Number(a.projekt_id));
  const s = await stat(a.datei);
  res.writeHead(200, { "content-type": "application/octet-stream", "content-length": String(s.size), "x-content-type-options": "nosniff",
    "content-disposition": `attachment; filename="${String(a.dateiname ?? "donation").replace(/[^A-Za-z0-9._-]/g, "_")}"`, "x-sha256": a.sha256 });
  createReadStream(a.datei).pipe(res);
  await new Promise<void>((ok) => res.on("close", () => ok()));
}

/** Täglich: Dateien entschiedener Aufträge nach 30 Tagen löschen (angenommene liegen dann im Repo). */
export async function aufraeumen(db: Db) {
  const r = await db.query("select id, datei from spendenauftraege where datei is not null and zustand in ('angenommen', 'abgelehnt', 'zurueckgezogen') and entschieden < now() - interval '30 days'");
  for (const z of r.rows) {
    await rm(z.datei, { force: true });
    await db.query("update spendenauftraege set datei = null where id = $1", [z.id]);
  }
  return r.rows.length;
}

/** Öffentliche Zahlen je Projekt (Projektseite, metrics): zugesagte Credits je Dienst und angenommene Spenden. */
export async function creditSpenden(db: Db, projektId: number) {
  const zugesagt = (await db.query(
    "select dienst, sum(credits)::int as credits, count(distinct mensch_id)::int as spender from zusagen where projekt_id = $1 and weg = 'website' and zustand = 'offen' and verfaellt >= current_date group by dienst order by 2 desc", [projektId])).rows;
  const angenommen = (await db.query("select count(*)::int as n, coalesce(sum(credits), 0)::int as credits from spendenauftraege where projekt_id = $1 and zustand = 'angenommen'", [projektId])).rows[0];
  return { zugesagt, angenommen: angenommen.n, credits_angenommen: angenommen.credits };
}

// ── Zusage über den Runner ────────────────────────────────────────────────────────────────────────────────────────
// Der Runner meldet, welche Medien-Dienste seine Agenten haben (swarvex setup), holt Aufträge, beginnt sie (zählt gegen
// das Tageslimit), erzeugt in der Sandbox und lädt hoch (PUT /donations/<id>/upload). Bewertet wird wie jede Spende.

type RunnerWer = { menschId: number; agentId: number; runner?: boolean | null };
function nurRunner(wer: RunnerWer) {
  if (wer.runner !== true) throw new Abgelehnt("only the swarvex runner offers media tools (swarvex setup)");
}

/** Gleicht die Runner-Zusagen des Agenten ab: je Dienst eine offene Zusage; fehlende Dienste beenden ihre Zusage. */
export async function runnerZusagenSetzen(db: Db, wer: RunnerWer, e: { dienste: string[]; tageslimit?: number; projekt?: string | null }) {
  nurRunner(wer);
  const dienste = [...new Set(e.dienste)];
  if (dienste.some((d) => !DIENSTE.includes(d as never) || d === "andere")) throw new Abgelehnt(`services: ${DIENSTE.filter((d) => d !== "andere").join(", ")}`);
  const limit = e.tageslimit ?? MEDIEN_JE_TAG;
  if (!Number.isInteger(limit) || limit < 1 || limit > MEDIEN_JE_TAG_MAX) throw new Abgelehnt(`daily limit: 1 to ${MEDIEN_JE_TAG_MAX} briefs`);
  return transaktion(db, async (tx) => {
    const projektId = e.projekt ? Number((await projektPerSlug(tx, e.projekt)).id) : null;
    const alt = (await tx.query("select * from zusagen where agent_id = $1 and weg = 'runner' and zustand = 'offen' for update", [wer.agentId])).rows;
    for (const z of alt) {
      if (dienste.includes(z.dienst) && (z.projekt_id === null ? null : Number(z.projekt_id)) === projektId) {
        if (z.tageslimit !== limit) {
          await tx.query("update zusagen set tageslimit = $2 where id = $1", [z.id, limit]);
          await protokollieren(tx, { projekt_id: projektId, akteur: `agent:${wer.agentId}`, art: "zusage_geaendert", bezug: `zusage:${z.id}`, daten: { dienst: z.dienst, tageslimit: limit } });
        }
        continue;
      }
      await tx.query("update zusagen set zustand = 'beendet' where id = $1", [z.id]);
      await tx.query("update spendenauftraege set zustand = 'zurueckgezogen', entschieden = now() where zusage_id = $1 and zustand in ('offen', 'nacharbeit')", [z.id]);
      await protokollieren(tx, { projekt_id: z.projekt_id === null ? null : Number(z.projekt_id), akteur: `agent:${wer.agentId}`, art: "zusage_beendet", bezug: `zusage:${z.id}`, daten: { dienst: z.dienst, weg: "runner" } });
    }
    const bleiben = new Set(alt.filter((z) => dienste.includes(z.dienst) && (z.projekt_id === null ? null : Number(z.projekt_id)) === projektId).map((z) => z.dienst));
    for (const d of dienste.filter((x) => !bleiben.has(x))) {
      const r = (await tx.query("insert into zusagen(mensch_id, projekt_id, dienst, weg, agent_id, tageslimit) values ($1,$2,$3,'runner',$4,$5) returning id",
        [wer.menschId, projektId, d, wer.agentId, limit])).rows[0];
      await protokollieren(tx, { projekt_id: projektId, akteur: `agent:${wer.agentId}`, art: "zusage_eingegangen", bezug: `zusage:${r.id}`, daten: { dienst: d, weg: "runner", tageslimit: limit } });
    }
    return { dienste, tageslimit: limit, projekt: e.projekt ?? null };
  });
}

/** Aufträge für die Runner-Zusagen des Agenten, die er jetzt beginnen kann (Nacharbeit zuerst, mit Begründung). */
export async function runnerAuftraege(db: Db, wer: RunnerWer) {
  nurRunner(wer);
  const limit = Number((await db.query("select max(tageslimit) as l from zusagen where agent_id = $1 and weg = 'runner' and zustand = 'offen'", [wer.agentId])).rows[0].l ?? 0);
  const heute = await heuteBegonnen(db, wer.agentId);
  const r = limit - heute <= 0 ? [] : (await db.query(
    `select a.id as auftrag_id, p.slug as projekt, z.dienst, a.titel, a.anleitung, a.zielpfad, a.dateitypen, a.max_mb, a.zustand,
            case when a.zustand = 'nacharbeit' then a.begruendung end as begruendung
     from spendenauftraege a join zusagen z on z.id = a.zusage_id join projekte p on p.id = a.projekt_id
     where z.agent_id = $1 and z.weg = 'runner' and z.zustand = 'offen' and a.zustand in ('offen', 'nacharbeit')
       and (a.begonnen is null or a.begonnen < now() - interval '${BEGONNEN_SPERRE}')
       and (a.abgebrochen is null or a.abgebrochen < now() - interval '${ABBRUCH_PAUSE}')
     order by a.zustand = 'nacharbeit' desc, a.erstellt limit $2`, [wer.agentId, limit - heute])).rows;
  return { tageslimit: limit, heute, auftraege: r.map((x) => ({ ...x, auftrag_id: Number(x.auftrag_id) })) };
}

export async function runnerAuftragBeginnen(db: Db, wer: RunnerWer, auftragId: number) {
  nurRunner(wer);
  return transaktion(db, async (tx) => {
    const a = (await tx.query(
      `select a.*, z.agent_id as zusage_agent, z.weg, z.zustand as zusage_zustand, z.tageslimit
       from spendenauftraege a join zusagen z on z.id = a.zusage_id where a.id = $1 for update of a`, [auftragId])).rows[0];
    if (!a || a.weg !== "runner" || Number(a.zusage_agent) !== wer.agentId) throw new Abgelehnt("unknown brief");
    if (a.zusage_zustand !== "offen" || !["offen", "nacharbeit"].includes(a.zustand)) throw new Abgelehnt("this brief takes no work right now");
    if (a.begonnen && (await tx.query(`select $1::timestamptz > now() - interval '${BEGONNEN_SPERRE}' as s`, [a.begonnen])).rows[0].s) throw new Abgelehnt("already started");
    if (a.abgebrochen && (await tx.query(`select $1::timestamptz > now() - interval '${ABBRUCH_PAUSE}' as s`, [a.abgebrochen])).rows[0].s) throw new Abgelehnt("handed back less than 24 hours ago");
    // Sperre je Agent, damit zwei gleichzeitige Beginne das Tageslimit nicht überholen
    await tx.query("select pg_advisory_xact_lock(hashtext('medien:' || $1::text))", [wer.agentId]);
    const limit = Number((await tx.query("select max(tageslimit) as l from zusagen where agent_id = $1 and weg = 'runner' and zustand = 'offen'", [wer.agentId])).rows[0].l ?? 0);
    if (await heuteBegonnen(tx, wer.agentId) >= limit) throw new Abgelehnt(`daily limit reached (${limit} briefs); change it with swarvex budget --media`);
    await tx.query("update spendenauftraege set begonnen = now() where id = $1", [auftragId]);
    await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: `agent:${wer.agentId}`, art: "spendenauftrag_begonnen", bezug: `spendenauftrag:${auftragId}`, daten: { nacharbeit: a.zustand === "nacharbeit" } });
    return { auftrag_id: auftragId };
  });
}

/** Zurückgeben ohne Ergebnis. gestoppt: nur angehalten (kein Fehlschlag), dann gleich wieder frei, sonst 24 Stunden Pause. */
export async function runnerAuftragAbbrechen(db: Db, wer: RunnerWer, auftragId: number, grund: string, gestoppt = false) {
  nurRunner(wer);
  return transaktion(db, async (tx) => {
    const a = (await tx.query("select a.*, z.agent_id as zusage_agent from spendenauftraege a join zusagen z on z.id = a.zusage_id where a.id = $1 for update of a", [auftragId])).rows[0];
    if (!a || Number(a.zusage_agent) !== wer.agentId) throw new Abgelehnt("unknown brief");
    if (!a.begonnen) throw new Abgelehnt("this brief was not started");
    if (gestoppt) await tx.query("update spendenauftraege set begonnen = null where id = $1", [auftragId]);
    else await tx.query("update spendenauftraege set begonnen = null, abgebrochen = now(), abbruchgrund = $2 where id = $1", [auftragId, grund.slice(0, 500)]);
    await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: `agent:${wer.agentId}`, art: "spendenauftrag_abgebrochen", bezug: `spendenauftrag:${auftragId}`, daten: { grund: grund.slice(0, 500), gestoppt } });
    return { auftrag_id: auftragId, zustand: a.zustand };
  });
}
