// Projekte anlegen: von der Website (Assistent „Projekt starten“) und von der Betriebs-Kommandozeile, mit denselben Regeln.
// swarvex hat nur offene Projekte (E-27): Code unter einer OSI-Lizenz, Assets unter einer freien Lizenz.
import { transaktion, type Db } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt } from "./aufgaben.ts";

export const OSI_LIZENZEN = new Set(["MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "MPL-2.0", "LGPL-2.1-or-later", "LGPL-3.0-or-later",
  "GPL-2.0-or-later", "GPL-3.0-only", "GPL-3.0-or-later", "AGPL-3.0-only", "AGPL-3.0-or-later", "EUPL-1.2", "0BSD", "Zlib", "Unlicense"]);
export const FREIE_ASSET_LIZENZEN = new Set(["CC BY 4.0", "CC BY-SA 4.0", "CC0 1.0", "CC-BY-4.0", "CC-BY-SA-4.0", "CC0-1.0", "OFL-1.1"]);
export const PROJEKTARTEN: Record<string, string> = { game: "spiel", app: "app", website: "website", service: "dienst", tool: "tool", library: "bibliothek", plugin: "plugin", other: "sonstiges" };
export const PROJEKTE_JE_MENSCH = 3;

export type NeuesProjekt = {
  slug: string; name: string; vision_md: string; vision_short: string; engine: string; license_code: string; license_assets: string;
  repo_url?: string | null; lead_model: string; goal: string; type?: string; procedure?: string;
  original?: { name?: string; version?: string; acquisition?: string; goal?: string; legal_basis?: string; inspect?: string } | null;
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
  if (verfahren === "clean_room" && !(p.original?.name?.trim() && p.original?.legal_basis?.trim() && p.original?.acquisition?.trim()))
    f.push("a clean-room project needs original.name, original.acquisition (how you lawfully obtained it) and original.legal_basis");
  return f;
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
       p.lead_model.trim(), p.goal.trim(), verfahren, verfahren === "clean_room" ? p.original : null, p.type ? PROJEKTARTEN[p.type] : null,
       von !== "betrieb", von === "betrieb" ? null : von])).rows[0].id);
    if (von !== "betrieb") {
      await tx.query("insert into projekt_rollen(projekt_id, mensch_id, rolle) values ($1,$2,'visionaer'), ($1,$2,'lead')", [id, von]);
    }
    await protokollieren(tx, { projekt_id: id, akteur: von === "betrieb" ? "system" : `mensch:${von}`, art: "vision_veroeffentlicht", bezug: `projekt:${id}`,
      daten: { slug: p.slug, name: p.name.trim(), lead_modell: p.lead_model.trim(), verfahren, ...(p.type ? { projektart: PROJEKTARTEN[p.type] } : {}), ...(verfahren === "clean_room" ? { vorbild: p.original } : {}) } });
    return { projekt_id: id, slug: p.slug };
  });
}
