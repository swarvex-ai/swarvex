// Projekte anlegen: von der Website (Assistent „Projekt starten“) und von der Betriebs-Kommandozeile, mit denselben Regeln.
// swarvex hat nur offene Projekte (E-27): Code unter einer OSI-Lizenz, Assets unter einer freien Lizenz.
import { transaktion, type Db } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt } from "./aufgaben.ts";
import { appBereit, repoPruefen } from "./github.ts";

export const OSI_LIZENZEN = new Set(["MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "MPL-2.0", "LGPL-2.1-or-later", "LGPL-3.0-or-later",
  "GPL-2.0-or-later", "GPL-3.0-only", "GPL-3.0-or-later", "AGPL-3.0-only", "AGPL-3.0-or-later", "EUPL-1.2", "0BSD", "Zlib", "Unlicense"]);
export const FREIE_ASSET_LIZENZEN = new Set(["CC BY 4.0", "CC BY-SA 4.0", "CC0 1.0", "CC-BY-4.0", "CC-BY-SA-4.0", "CC0-1.0", "OFL-1.1"]);
export const PROJEKTARTEN: Record<string, string> = { game: "spiel", app: "app", website: "website", service: "dienst", tool: "tool", library: "bibliothek", plugin: "plugin", other: "sonstiges" };
export const PROJEKTE_JE_MENSCH = 3;
/** Art des Originals bei Clean Room: wählt den Werkzeug-Pfad (lead/roles/cleanroom/toolkit.md). */
export const VORBILD_ARTEN = ["windows", "linux_macos", "android", "web", "game", "firmware", "format_protocol"];
const VORBILD_FELDER = ["name", "version", "acquisition", "target", "depth", "inspect", "legal_basis", "goal"] as const;

export type NeuesProjekt = {
  slug: string; name: string; vision_md: string; vision_short: string; engine: string; license_code: string; license_assets: string;
  repo_url?: string | null; lead_model: string; goal: string; type?: string; procedure?: string;
  original?: { name?: string; version?: string; acquisition?: string; target?: string; depth?: string; inspect?: string; legal_basis?: string; goal?: string } | null;
};

/** Prüft ein neues Projekt; liefert die Fehler (leer = in Ordnung). */
export function projektPruefen(p: NeuesProjekt): string[] {
  const f: string[] = [];
  if (!/^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$/.test(p.slug ?? "")) f.push("slug: 3–40 lowercase letters, digits and dashes");
  if (!p.name?.trim() || p.name.length > 80) f.push("name: 1–80 characters");
  if (!p.vision_short?.trim() || p.vision_short.length > 200) f.push("vision_short: one sentence, at most 200 characters");
  if ((p.vision_md ?? "").length > 20000) f.push("vision_md: at most 20,000 characters");
  if (!p.engine?.trim() || p.engine.length > 60) f.push("engine: 1–60 characters, e.g. godot-4.7, sveltekit, rust");
  if (!OSI_LIZENZEN.has(p.license_code)) f.push(`license_code ${p.license_code} is not an OSI license (e.g. MIT, Apache-2.0, GPL-3.0-or-later, AGPL-3.0-or-later, MPL-2.0)`);
  if (!FREIE_ASSET_LIZENZEN.has(p.license_assets)) f.push(`license_assets ${p.license_assets} is not a free license (e.g. CC BY 4.0, CC BY-SA 4.0, CC0 1.0)`);
  if (p.type && !PROJEKTARTEN[p.type]) f.push(`type: ${Object.keys(PROJEKTARTEN).join(" | ")}`);
  if (p.repo_url && !/^https:\/\/(github\.com|codeberg\.org|gitlab\.com)\/[\w.-]+\/[\w.-]+\/?$/.test(p.repo_url)) f.push("repo_url: a public repository, e.g. https://github.com/owner/name");
  if (!p.lead_model?.trim() || p.lead_model.length > 120) f.push("lead_model: which agent and model leads the project");
  if (!p.goal?.trim() || p.goal.length > 200) f.push("goal: the first goal in one sentence");
  const verfahren = p.procedure ?? "standard";
  if (!["standard", "clean_room"].includes(verfahren)) f.push("procedure: standard or clean_room");
  if (verfahren === "clean_room") {
    const o = p.original ?? {};
    if (!(o.name?.trim() && o.acquisition?.trim())) f.push("a clean-room project needs original.name and original.acquisition (how you lawfully obtained it)");
    if (o.target && !VORBILD_ARTEN.includes(o.target)) f.push(`original.target: ${VORBILD_ARTEN.join(" | ")}`);
    if (o.depth && !["observe", "inspect"].includes(o.depth)) f.push("original.depth: observe (only from outside, default) or inspect");
    if (o.depth === "inspect" && !(o.legal_basis?.trim() && o.inspect?.trim()))
      f.push("looking inside (original.depth inspect) needs original.inspect (which parts) and original.legal_basis");
    if (VORBILD_FELDER.some((k) => String(o[k] ?? "").length > 300)) f.push("original: at most 300 characters per field");
  }
  return f;
}

/** Nur bekannte Felder, getrimmt; ohne Angabe gilt die Tiefe observe (nur von außen). */
function vorbildNormal(o: NeuesProjekt["original"]) {
  const r: Record<string, string> = { depth: "observe" };
  for (const k of VORBILD_FELDER) { const v = String(o?.[k] ?? "").trim(); if (v) r[k] = v; }
  return r;
}

export async function projektAnlegen(db: Db, p: NeuesProjekt, von: number | "betrieb") {
  const fehler = projektPruefen(p);
  if (fehler.length) throw new Abgelehnt(fehler.join("; "));
  return transaktion(db, async (tx) => {
    if (von !== "betrieb") {
      const eigene = Number((await tx.query("select count(*) from projekte where angelegt_von = $1 and not gesperrt", [von])).rows[0].count);
      if (eigene >= PROJEKTE_JE_MENSCH) throw new Abgelehnt(`at most ${PROJEKTE_JE_MENSCH} projects per person; write to contact@swarvex.ai for more`);
      const heute = Number((await tx.query("select count(*) from projekte where angelegt_von = $1 and id in (select projekt_id from ereignisse where art = 'vision_veroeffentlicht' and zeit > now() - interval '1 day')", [von])).rows[0].count);
      if (heute >= 1) throw new Abgelehnt("one new project per day");
    }
    if ((await tx.query("select 1 from projekte where slug = $1", [p.slug])).rowCount) throw new Abgelehnt(`the address ${p.slug} is taken`);
    const verfahren = p.procedure ?? "standard";
    const id = Number((await tx.query(
      `insert into projekte(slug, name, vision_md, vision_kurz, engine, lizenz_code, lizenz_assets, repo_url, lead_modell, ziel, verfahren, vorbild, projektart, entwurf, angelegt_von)
       values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) returning id`,
      [p.slug, p.name.trim(), p.vision_md?.trim() || p.vision_short.trim(), p.vision_short.trim(), p.engine.trim(), p.license_code, p.license_assets, p.repo_url ?? null,
       p.lead_model.trim(), p.goal.trim(), verfahren, verfahren === "clean_room" ? vorbildNormal(p.original) : null, p.type ? PROJEKTARTEN[p.type] : null,
       von !== "betrieb", von === "betrieb" ? null : von])).rows[0].id);
    if (von !== "betrieb") {
      await tx.query("insert into projekt_rollen(projekt_id, mensch_id, rolle) values ($1,$2,'visionaer'), ($1,$2,'lead')", [id, von]);
    }
    await protokollieren(tx, { projekt_id: id, akteur: von === "betrieb" ? "system" : `mensch:${von}`, art: "vision_veroeffentlicht", bezug: `projekt:${id}`,
      daten: { slug: p.slug, name: p.name.trim(), lead_modell: p.lead_model.trim(), verfahren, ...(p.type ? { projektart: PROJEKTARTEN[p.type] } : {}), ...(verfahren === "clean_room" ? { vorbild: vorbildNormal(p.original) } : {}) } });
    return { projekt_id: id, slug: p.slug };
  });
}

/** Repo über die GitHub App verbinden: nur der Visionär, nur ein öffentliches Repo, auf dem sein GitHub-Konto schreiben darf. */
export async function repoVerbinden(db: Db, slug: string, menschId: number, adresse: string) {
  const p = (await db.query(
    `select p.id, m.konto_provider, m.login from projekte p join projekt_rollen r on r.projekt_id = p.id and r.rolle = 'visionaer' and r.mensch_id = $2
     join menschen m on m.id = r.mensch_id where p.slug = $1 and not p.gesperrt`, [slug, menschId])).rows[0];
  if (!p) throw new Abgelehnt("only the visionary of this project can connect its repository");
  if (p.konto_provider !== "github") throw new Abgelehnt("sign in with GitHub to connect a GitHub repository");
  if (!appBereit()) throw new Abgelehnt("app_not_configured");
  const r = await repoPruefen(adresse, p.login);
  if ("fehler" in r) throw new Abgelehnt(r.fehler);
  await transaktion(db, async (tx) => {
    await tx.query("update projekte set repo_url = $2, repo_git = $3, github_installation = $4 where id = $1", [p.id, r.repo_url, r.repo_git, r.installation]);
    await protokollieren(tx, { projekt_id: Number(p.id), akteur: `mensch:${menschId}`, art: "repo_gesetzt", bezug: `projekt:${p.id}`, daten: { repo_git: r.repo_git, ueber: "github-app" } });
  });
  return { repo_url: r.repo_url };
}
