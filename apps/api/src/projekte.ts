// Projekte anlegen: von der Website (Assistent „Projekt starten“) und von der Betriebs-Kommandozeile, mit denselben Regeln.
// swarvex hat nur offene Projekte (E-27): Code unter einer OSI-Lizenz, Assets unter einer freien Lizenz.
import { ideePer } from "./ideen.ts";
import { transaktion, type Db } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt } from "./aufgaben.ts";
import { appBereit, repoPruefen } from "./github.ts";

export const OSI_LIZENZEN = new Set(["MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "MPL-2.0", "LGPL-2.1-or-later", "LGPL-3.0-or-later",
  "GPL-2.0-or-later", "GPL-3.0-only", "GPL-3.0-or-later", "AGPL-3.0-only", "AGPL-3.0-or-later", "EUPL-1.2", "0BSD", "Zlib", "Unlicense",
  // häufig bei älteren Projekten, die weitergeführt werden (Verfahren continuation erbt die Lizenz)
  "GPL-2.0-only", "LGPL-2.1-only", "LGPL-3.0-only", "MPL-1.1", "EPL-2.0", "Artistic-2.0", "BSL-1.0", "CDDL-1.0", "Python-2.0", "PostgreSQL", "MIT-0"]);
export const FREIE_ASSET_LIZENZEN = new Set(["CC BY 4.0", "CC BY-SA 4.0", "CC0 1.0", "CC-BY-4.0", "CC-BY-SA-4.0", "CC0-1.0", "OFL-1.1"]);
export const PROJEKTARTEN: Record<string, string> = { game: "spiel", app: "app", website: "website", service: "dienst", tool: "tool", library: "bibliothek", plugin: "plugin", research: "forschung", other: "sonstiges" };
export const PROJEKTE_JE_MENSCH = 3;
/** Adressen, die keine Projekte sein können: Subdomains der Vorschau-Domain (review.<Domain> für Einreichungen) und Plattform-Namen. */
export const RESERVIERT = new Set(["review", "www", "api", "docs", "play", "static", "assets", "mail", "admin", "swarvex"]);
/** Art des Originals bei Clean Room: wählt den Werkzeug-Pfad (lead/roles/cleanroom/toolkit.md). */
export const VORBILD_ARTEN = ["windows", "linux_macos", "android", "web", "game", "firmware", "format_protocol"];
export const SPRACHEN = ["de", "en"];
const VORBILD_FELDER = ["name", "version", "acquisition", "target", "depth", "inspect", "legal_basis", "models", "providers", "goal"] as const;
/** Art eines Forschungsprojekts (lead/LEAD.md 9): wählt die Rollen und Prüfungen, die zuerst zählen. */
export const FORSCHUNG_ARTEN = ["formal_proof", "reproduction", "scientific_software", "verification"];
const FORSCHUNG_FELDER = ["kind", "source", "success"] as const;
/** Zustand des Originals beim Weiterführen: archiviert, ohne Antwort, Maintainer stimmen zu, Maintainer übergeben. */
export const UPSTREAM_ZUSTAENDE = ["archived", "no_answer", "maintainers_agree", "handover"];
const UPSTREAM_FELDER = ["repo", "status", "contact"] as const;

export type NeuesProjekt = {
  slug: string; name: string; vision_md: string; vision_short: string; engine: string; license_code: string; license_assets: string;
  repo_url?: string | null; lead_model: string; goal: string; type?: string; procedure?: string; language?: string;
  original?: { name?: string; version?: string; acquisition?: string; target?: string; depth?: string; inspect?: string; legal_basis?: string; models?: string; providers?: string; goal?: string } | null;
  research?: { kind?: string; source?: string; success?: string } | null;
  upstream?: { repo?: string; status?: string; contact?: string } | null;
  /** Übernommene Projektidee „Lead gesucht“ (ideen.ts) */
  idea?: string | null;
};

/** Prüft ein neues Projekt; liefert die Fehler (leer = in Ordnung). */
export function projektPruefen(p: NeuesProjekt): string[] {
  const f: string[] = [];
  if (!/^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$/.test(p.slug ?? "")) f.push("slug: 3–40 lowercase letters, digits and dashes");
  else if (RESERVIERT.has(p.slug)) f.push(`slug: ${p.slug} is reserved`);
  if (!p.name?.trim() || p.name.length > 80) f.push("name: 1–80 characters");
  if (!p.vision_short?.trim() || p.vision_short.length > 200) f.push("vision_short: one sentence, at most 200 characters");
  if ((p.vision_md ?? "").length > 20000) f.push("vision_md: at most 20,000 characters");
  if (!p.engine?.trim() || p.engine.length > 60) f.push("engine: 1–60 characters, e.g. godot-4.7, sveltekit, rust");
  if (!OSI_LIZENZEN.has(p.license_code)) f.push(`license_code ${p.license_code} is not an OSI license (e.g. MIT, Apache-2.0, GPL-3.0-or-later, AGPL-3.0-or-later, MPL-2.0)`);
  // Weitergeführte Projekte erben die Lizenz des Originals, oft auch für ihre Assets (z. B. GPL für alte Spiele)
  if (!FREIE_ASSET_LIZENZEN.has(p.license_assets) && !(p.procedure === "continuation" && OSI_LIZENZEN.has(p.license_assets))) f.push(`license_assets ${p.license_assets} is not a free license (e.g. CC BY 4.0, CC BY-SA 4.0, CC0 1.0)`);
  if (p.type && !PROJEKTARTEN[p.type]) f.push(`type: ${Object.keys(PROJEKTARTEN).join(" | ")}`);
  if (p.repo_url && !/^https:\/\/(github\.com|codeberg\.org|gitlab\.com)\/[\w.-]+\/[\w.-]+\/?$/.test(p.repo_url)) f.push("repo_url: a public repository, e.g. https://github.com/owner/name");
  if (!p.lead_model?.trim() || p.lead_model.length > 120) f.push("lead_model: which agent and model leads the project");
  if (!p.goal?.trim() || p.goal.length > 200) f.push("goal: the first goal in one sentence");
  if (p.language && !SPRACHEN.includes(p.language)) f.push(`language: the language of vision and goal, ${SPRACHEN.join(" or ")}`);
  if (p.idea && !ideePer(p.idea)) f.push(`idea: unknown project idea ${p.idea}`);
  const verfahren = p.procedure ?? "standard";
  if (!["standard", "clean_room", "continuation"].includes(verfahren)) f.push("procedure: standard, clean_room or continuation");
  if (verfahren === "clean_room") {
    const o = p.original ?? {};
    if (!(o.name?.trim() && o.acquisition?.trim())) f.push("a clean-room project needs original.name and original.acquisition (how you lawfully obtained it)");
    if (o.target && !VORBILD_ARTEN.includes(o.target)) f.push(`original.target: ${VORBILD_ARTEN.join(" | ")}`);
    if (o.depth && !["observe", "inspect"].includes(o.depth)) f.push("original.depth: observe (only from outside, default) or inspect");
    if (o.depth === "inspect" && !(o.legal_basis?.trim() && o.inspect?.trim()))
      f.push("looking inside (original.depth inspect) needs original.inspect (which parts) and original.legal_basis");
    // Werkzeug-Ausgaben von innen (Disassembly, Pseudocode) erreichen den Modell-Anbieter des Agenten
    if (o.depth === "inspect" && !["local", "providers"].includes(o.models ?? ""))
      f.push("looking inside (original.depth inspect) needs original.models: local (only models on the analyst's own machine) or providers (only the providers named in original.providers)");
    if (o.depth === "inspect" && o.models === "providers" && !o.providers?.trim())
      f.push("original.models providers needs original.providers (which model providers may see tool output from inside, e.g. Anthropic, OpenAI)");
    if (VORBILD_FELDER.some((k) => String(o[k] ?? "").length > 300)) f.push("original: at most 300 characters per field");
  }
  if (verfahren === "continuation") {
    const u = p.upstream ?? {};
    if (!/^https:\/\/[\w.-]+\.[a-z]{2,}\/[\w./~-]+$/.test(u.repo ?? "")) f.push("upstream.repo: the address of the original repository (https://…)");
    if (!UPSTREAM_ZUSTAENDE.includes(u.status ?? "")) f.push(`upstream.status: ${UPSTREAM_ZUSTAENDE.join(" | ")}`);
    if (u.status === "no_answer" && !u.contact?.trim()) f.push("upstream.contact: where you asked the maintainers (e.g. the link to your issue) when they did not answer");
    if (UPSTREAM_FELDER.some((k) => String(u[k] ?? "").length > 300)) f.push("upstream: at most 300 characters per field");
    if (p.repo_url && u.repo && p.repo_url.replace(/\/$/, "") === u.repo.replace(/\/$/, "")) f.push("repo_url is the fork, not the original");
  }
  if (p.type === "research") {
    const r = p.research ?? {};
    if (verfahren !== "standard") f.push("a research project uses the standard procedure");
    if (!FORSCHUNG_ARTEN.includes(r.kind ?? "")) f.push(`research.kind: ${FORSCHUNG_ARTEN.join(" | ")}`);
    if (!(r.source?.trim() && r.success?.trim())) f.push("a research project needs research.source (paper, theorem or repository, exactly) and research.success (what counts as done, and who or what decides)");
    if (FORSCHUNG_FELDER.some((k) => String(r[k] ?? "").length > 500)) f.push("research: at most 500 characters per field");
  }
  return f;
}

/** Angaben zum Original beim Weiterführen: nur bekannte Felder, getrimmt. */
function upstreamNormal(u: NeuesProjekt["upstream"]) {
  return Object.fromEntries(UPSTREAM_FELDER.map((k) => [k, String(u?.[k] ?? "").trim()]).filter(([, v]) => v));
}

/** Eckdaten eines Forschungsprojekts: nur bekannte Felder, getrimmt. */
function forschungNormal(r: NeuesProjekt["research"]) {
  return Object.fromEntries(FORSCHUNG_FELDER.map((k) => [k, String(r?.[k] ?? "").trim()]).filter(([, v]) => v));
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
    if (p.idea) {
      const weg = (await tx.query("select slug from projekte where idee = $1", [p.idea])).rows[0];
      if (weg) throw new Abgelehnt(`this project idea has already been taken over (${weg.slug})`);
    }
    const verfahren = p.procedure ?? "standard";
    const id = Number((await tx.query(
      `insert into projekte(slug, name, vision_md, vision_kurz, engine, lizenz_code, lizenz_assets, repo_url, lead_modell, ziel, verfahren, vorbild, projektart, forschung, upstream, entwurf, angelegt_von, sprache, idee)
       values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19) returning id`,
      [p.slug, p.name.trim(), p.vision_md?.trim() || p.vision_short.trim(), p.vision_short.trim(), p.engine.trim(), p.license_code, p.license_assets, p.repo_url ?? null,
       p.lead_model.trim(), p.goal.trim(), verfahren, verfahren === "clean_room" ? vorbildNormal(p.original) : null, p.type ? PROJEKTARTEN[p.type] : null,
       p.type === "research" ? forschungNormal(p.research) : null, verfahren === "continuation" ? upstreamNormal(p.upstream) : null, von !== "betrieb", von === "betrieb" ? null : von, p.language ?? "en", p.idea || null])).rows[0].id);
    if (von !== "betrieb") {
      await tx.query("insert into projekt_rollen(projekt_id, mensch_id, rolle) values ($1,$2,'visionaer'), ($1,$2,'lead')", [id, von]);
    }
    await protokollieren(tx, { projekt_id: id, akteur: von === "betrieb" ? "system" : `mensch:${von}`, art: "vision_veroeffentlicht", bezug: `projekt:${id}`,
      daten: { slug: p.slug, name: p.name.trim(), lead_modell: p.lead_model.trim(), verfahren, ...(p.type ? { projektart: PROJEKTARTEN[p.type] } : {}), ...(verfahren === "clean_room" ? { vorbild: vorbildNormal(p.original) } : {}),
        ...(p.type === "research" ? { forschung: forschungNormal(p.research) } : {}),
        ...(verfahren === "continuation" ? { upstream: upstreamNormal(p.upstream) } : {}) } });
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

export type Uebersetzung = { vision_md?: string; vision_short?: string; goal?: string };

/** Übersetzung von Vision, Kurzvision und Ziel setzen: Lead oder Visionär des Projekts (oder der Betrieb).
 *  Das Original bleibt unverändert und maßgeblich; leere Felder entfernen die Übersetzung dieses Felds. */
export async function uebersetzungSetzen(db: Db, slug: string, von: number | "betrieb", sprache: string, u: Uebersetzung) {
  if (!SPRACHEN.includes(sprache)) throw new Abgelehnt(`language: ${SPRACHEN.join(" or ")}`);
  const p = (await db.query("select id, sprache, uebersetzungen from projekte where slug = $1 and not gesperrt", [slug])).rows[0];
  if (!p) throw new Abgelehnt(`project ${slug} does not exist`);
  if (von !== "betrieb") {
    const r = await db.query("select 1 from projekt_rollen where projekt_id = $1 and mensch_id = $2 and rolle in ('lead', 'visionaer')", [p.id, von]);
    if (!r.rowCount) throw new Abgelehnt("only the lead or the visionary of this project can set a translation");
  }
  if (sprache === p.sprache) throw new Abgelehnt(`the original is already in ${sprache}; change the original instead`);
  const grenzen: Record<keyof Uebersetzung, number> = { vision_md: 20000, vision_short: 200, goal: 200 };
  const alt: Uebersetzung = p.uebersetzungen?.[sprache] ?? {};
  const neu: Uebersetzung = { ...alt };
  for (const k of Object.keys(grenzen) as (keyof Uebersetzung)[]) {
    if (u[k] === undefined) continue;
    const v = u[k]!.trim();
    if (v.length > grenzen[k]) throw new Abgelehnt(`${k}: at most ${grenzen[k]} characters`);
    if (v) neu[k] = v; else delete neu[k];
  }
  await transaktion(db, async (tx) => {
    await tx.query("update projekte set uebersetzungen = jsonb_set(uebersetzungen, $2, $3::jsonb) where id = $1", [p.id, [sprache], JSON.stringify(neu)]);
    await protokollieren(tx, { projekt_id: Number(p.id), akteur: von === "betrieb" ? "system" : `mensch:${von}`, art: "uebersetzung_gesetzt", bezug: `projekt:${p.id}`,
      daten: { sprache, felder: Object.keys(neu) } });
  });
  return { project: slug, language: sprache, fields: Object.keys(neu) };
}
