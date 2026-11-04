// Credit-Spenden, Weg „Auftrag + Upload“: Zusagen von Spendern, Aufträge des Leads, Upload im Browser.
// Kein Schlüssel verlässt das Konto des Spenders; die Plattform speichert nur die hochgeladene Datei.
import { createHash, randomBytes } from "node:crypto";
import { createReadStream, createWriteStream } from "node:fs";
import { mkdir, rename, rm, stat } from "node:fs/promises";
import type { IncomingMessage, ServerResponse } from "node:http";
import { extname, join } from "node:path";
import { transaktion, type Db } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt, officersVon, projektPerSlug, rollenVon } from "./aufgaben.ts";

/** Upload-Speicher (Docker-Volume spenden); beim Aufruf gelesen, damit Tests ihn umlenken können. */
export const speicher = () => process.env.SPENDEN_DIR ?? "/data/spenden";
/** Häufige Dienste mit verfallenden Credits; „andere“ für alles Weitere (Name in der Notiz). */
export const DIENSTE = ["meshy", "higgsfield", "elevenlabs", "suno", "runway", "midjourney", "leonardo", "kling", "andere"] as const;
/** Dateitypen, die hochgeladen werden dürfen (Endungen ohne Punkt). */
export const DATEITYPEN = ["glb", "gltf", "fbx", "obj", "zip", "png", "jpg", "jpeg", "webp", "svg", "mp4", "webm", "mov", "wav", "mp3", "ogg", "flac", "txt", "md"] as const;
export const ZUSAGEN_JE_MENSCH = 10;
export const UPLOADS_JE_TAG = 30;
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
    `select z.id as zusage_id, p.slug as projekt, p.name as projekt_name, z.dienst, z.credits, z.verfaellt, z.notiz, z.zustand, z.erstellt
     from zusagen z join projekte p on p.id = z.projekt_id where z.mensch_id = $1 order by z.zustand, z.erstellt desc limit 50`, [menschId])).rows;
  const a = (await db.query(
    `select a.id as auftrag_id, a.zusage_id, a.titel, a.anleitung, a.zielpfad, a.dateitypen, a.max_mb, a.credits, a.zustand, a.dateiname, a.hochgeladen, a.begruendung, a.erstellt
     from spendenauftraege a join zusagen z on z.id = a.zusage_id where z.mensch_id = $1 order by a.erstellt`, [menschId])).rows;
  return z.map((x) => ({ ...x, zusage_id: Number(x.zusage_id), verfaellt: String(x.verfaellt instanceof Date ? x.verfaellt.toISOString().slice(0, 10) : x.verfaellt),
    auftraege: a.filter((y) => Number(y.zusage_id) === Number(x.zusage_id)).map((y) => ({ ...y, auftrag_id: Number(y.auftrag_id), zusage_id: Number(y.zusage_id) })) }));
}

/** Für Lead, Visionär und Bereichs-Leads: offene Zusagen mit verbleibenden Credits. */
export async function zusagenFuerLead(db: Db, menschId: number, projektSlug: string) {
  const p = await projektPerSlug(db, projektSlug);
  await darfSteuern(db, menschId, Number(p.id));
  const r = await db.query(
    `select z.id as zusage_id, m.login as spender, z.dienst, z.credits, z.verfaellt, z.notiz, z.erstellt,
            coalesce((select sum(a.credits) from spendenauftraege a where a.zusage_id = z.id and a.zustand = any($2)), 0)::int as verplant
     from zusagen z join menschen m on m.id = z.mensch_id
     where z.projekt_id = $1 and z.zustand = 'offen' and z.verfaellt >= current_date order by z.verfaellt`, [p.id, offen]);
  return r.rows.map((x) => ({ ...x, zusage_id: Number(x.zusage_id), verfaellt: x.verfaellt instanceof Date ? x.verfaellt.toISOString().slice(0, 10) : x.verfaellt, frei: x.credits - x.verplant }));
}

export async function auftragAnlegen(db: Db, menschId: number, a: { zusage_id: number; titel: string; anleitung: string; zielpfad: string; dateitypen: string[]; max_mb?: number; credits: number }) {
  const typen = a.dateitypen.map((t) => t.toLowerCase().replace(/^\./, ""));
  if (!typen.length || typen.some((t) => !DATEITYPEN.includes(t as never))) throw new Abgelehnt(`file types: ${DATEITYPEN.join(", ")}`);
  if (a.zielpfad.includes("..") || a.zielpfad.startsWith("/")) throw new Abgelehnt("target path: relative path inside the repository");
  if (!Number.isInteger(a.credits) || a.credits < 1) throw new Abgelehnt("credits: estimated cost, a whole number of at least 1");
  return transaktion(db, async (tx) => {
    const z = (await tx.query("select * from zusagen where id = $1 for update", [a.zusage_id])).rows[0];
    if (!z) throw new Abgelehnt("unknown pledge");
    await darfSteuern(tx as unknown as Db, menschId, Number(z.projekt_id));
    if (z.zustand !== "offen" || new Date(z.verfaellt) < new Date(new Date().toISOString().slice(0, 10))) throw new Abgelehnt("the pledge is no longer open");
    const verplant = Number((await tx.query("select coalesce(sum(credits), 0) as s from spendenauftraege where zusage_id = $1 and zustand = any($2)", [z.id, offen])).rows[0].s);
    if (verplant + a.credits > z.credits) throw new Abgelehnt(`the pledge has ${z.credits - verplant} credits left`);
    const r = (await tx.query(
      `insert into spendenauftraege(zusage_id, projekt_id, von_mensch, titel, anleitung, zielpfad, dateitypen, max_mb, credits) values ($1,$2,$3,$4,$5,$6,$7,$8,$9) returning id`,
      [z.id, z.projekt_id, menschId, a.titel, a.anleitung, a.zielpfad, typen, Math.min(Math.max(a.max_mb ?? 200, 1), 500), a.credits])).rows[0];
    await protokollieren(tx, { projekt_id: Number(z.projekt_id), akteur: `mensch:${menschId}`, art: "spendenauftrag_angelegt", bezug: `spendenauftrag:${r.id}`, daten: { zusage: Number(z.id), titel: a.titel, credits: a.credits } });
    return { auftrag_id: Number(r.id) };
  });
}

export async function auftraegeFuerLead(db: Db, menschId: number, projektSlug: string, nurOffen = false) {
  const p = await projektPerSlug(db, projektSlug);
  await darfSteuern(db, menschId, Number(p.id));
  const r = await db.query(
    `select a.id as auftrag_id, a.zusage_id, m.login as spender, z.dienst, a.titel, a.zielpfad, a.dateitypen, a.credits, a.zustand,
            a.dateiname, a.sha256, a.bytes, a.hochgeladen, a.begruendung, a.erstellt
     from spendenauftraege a join zusagen z on z.id = a.zusage_id join menschen m on m.id = z.mensch_id
     where a.projekt_id = $1 and ($2::boolean is false or a.zustand in ('offen', 'hochgeladen', 'nacharbeit')) order by (a.zustand = 'hochgeladen') desc, a.erstellt`, [p.id, nurOffen]);
  return r.rows.map((x) => ({ ...x, auftrag_id: Number(x.auftrag_id), zusage_id: Number(x.zusage_id), bytes: x.bytes === null ? null : Number(x.bytes),
    datei_url: x.zustand === "hochgeladen" || x.zustand === "angenommen" ? `/donations/${x.auftrag_id}/file` : null }));
}

/** Upload vom Spender (Website, eigene Herkunft und Sitzung geprüft): als Strom auf die Platte, mit SHA-256 und Größenlimit. */
export async function hochladen(db: Db, menschId: number, auftragId: number, req: IncomingMessage, dateiname: string) {
  const a = (await db.query(
    "select a.*, z.mensch_id as spender, p.slug from spendenauftraege a join zusagen z on z.id = a.zusage_id join projekte p on p.id = a.projekt_id where a.id = $1", [auftragId])).rows[0];
  if (!a || Number(a.spender) !== menschId) throw new Abgelehnt("unknown brief");
  if (!["offen", "nacharbeit"].includes(a.zustand)) throw new Abgelehnt("this brief takes no upload right now");
  const endung = extname(dateiname).toLowerCase().replace(/^\./, "");
  if (!a.dateitypen.includes(endung)) throw new Abgelehnt(`file type .${endung} is not allowed here (allowed: ${a.dateitypen.join(", ")})`);
  const max = a.max_mb * 1024 * 1024;
  const laenge = Number(req.headers["content-length"] ?? 0);
  if (!laenge || laenge > max) throw new Abgelehnt(`file too large or size unknown (at most ${a.max_mb} MB)`);
  const heute = Number((await db.query("select count(*) from spendenauftraege a join zusagen z on z.id = a.zusage_id where z.mensch_id = $1 and a.hochgeladen > now() - interval '1 day'", [menschId])).rows[0].count);
  if (heute >= UPLOADS_JE_TAG) throw new Abgelehnt(`at most ${UPLOADS_JE_TAG} uploads per day`);
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
  const sha = hash.digest("hex");
  const ziel = join(ordner, `${auftragId}-${sha.slice(0, 16)}.${endung}`);
  await rename(tmp, ziel);
  const alt = a.datei as string | null;
  await transaktion(db, async (tx) => {
    await tx.query("update spendenauftraege set zustand = 'hochgeladen', datei = $2, dateiname = $3, sha256 = $4, bytes = $5, hochgeladen = now() where id = $1",
      [auftragId, ziel, dateiname.slice(0, 200), sha, bytes]);
    await protokollieren(tx, { projekt_id: Number(a.projekt_id), akteur: `mensch:${menschId}`, art: "spende_hochgeladen", bezug: `spendenauftrag:${auftragId}`, daten: { sha256: sha, bytes } });
  });
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
    "select dienst, sum(credits)::int as credits, count(distinct mensch_id)::int as spender from zusagen where projekt_id = $1 and zustand = 'offen' and verfaellt >= current_date group by dienst order by 2 desc", [projektId])).rows;
  const angenommen = (await db.query("select count(*)::int as n, coalesce(sum(credits), 0)::int as credits from spendenauftraege where projekt_id = $1 and zustand = 'angenommen'", [projektId])).rows[0];
  return { zugesagt, angenommen: angenommen.n, credits_angenommen: angenommen.credits };
}
