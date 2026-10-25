// Doku-Engine: Markdown-Seiten aus src/lib/inhalt/doku/<sprache>, optional mit `quelle:` auf eine Datei
// im Repo (z. B. lead/LEAD.md), damit es nur eine Quelle gibt. Seiten beider Sprachen sind über
// `schluessel` verbunden (für hreflang und den Sprachumschalter). Wird beim Bauen vorgerendert.
import { readFileSync, readdirSync } from "node:fs";
import { join, resolve } from "node:path";
import GithubSlugger from "github-slugger";
import { Marked, type Tokens } from "marked";
import { createHighlighter, type Highlighter } from "shiki";
import type { Sprache } from "$lib/texte";

const REPO = resolve(process.cwd(), "../..");
const ORDNER = join(REPO, "apps/web/src/lib/inhalt/doku");
import { QUELLCODE } from "$lib/seo";
export const REPO_WEB = `${QUELLCODE}/blob/main`;

/** Interne Hinweise in Quelldateien (z. B. Entwurfsvermerke) erscheinen nicht auf der Website. */
const ohneIntern = (md: string) => md.replace(/<!-- intern -->[\s\S]*?<!-- \/intern -->\n?/g, "");

export const GRUPPEN: Record<Sprache, string[]> = {
  de: ["Erste Schritte", "Konzepte", "Runner", "Lead", "Schnittstellen", "Sicherheit", "Betrieb", "Referenz"],
  en: ["Getting started", "Concepts", "Runner", "Lead", "Interfaces", "Security", "Operations", "Reference"],
};
export const STARTSEITE: Record<Sprache, string> = { de: "ueberblick", en: "overview" };
export const docsPfad = (s: Sprache, slug: string) => `${s === "en" ? "/en" : ""}/docs/${slug}`;

export type Seite = {
  sprache: Sprache; slug: string; schluessel: string; titel: string; gruppe: string; reihenfolge: number;
  beschreibung: string; quelle?: string; markdown: string; datei: string;
};
export type Ueberschrift = { id: string; text: string; ebene: number };

// HTML einer Überschrift als reiner Text (Svelte maskiert ihn beim Ausgeben selbst)
const klartext = (html: string) => html.replace(/<[^>]+>/g, "").replace(/&#39;/g, "'").replace(/&quot;/g, '"').replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&amp;/g, "&");

function frontmatter(roh: string) {
  const m = /^---\n([\s\S]*?)\n---\n?/.exec(roh);
  const meta: Record<string, string> = {};
  if (m) for (const zeile of m[1].split("\n")) { const i = zeile.indexOf(":"); if (i > 0) meta[zeile.slice(0, i).trim()] = zeile.slice(i + 1).trim(); }
  return { meta, text: m ? roh.slice(m[0].length) : roh };
}

const cache = new Map<Sprache, Seite[]>();
export function seiten(sprache: Sprache): Seite[] {
  if (cache.has(sprache)) return cache.get(sprache)!;
  const ordner = join(ORDNER, sprache);
  const liste = readdirSync(ordner).filter((d) => d.endsWith(".md")).map((d) => {
    const { meta, text } = frontmatter(readFileSync(join(ordner, d), "utf8"));
    const slug = d.replace(/\.md$/, "");
    // eingebundene Repo-Datei: erste H1 entfällt (der Titel kommt aus der Doku)
    const markdown = ohneIntern(meta.quelle ? readFileSync(join(REPO, meta.quelle), "utf8").replace(/^# .*\n+/, "") : text);
    return { sprache, slug, schluessel: meta.schluessel ?? slug, titel: meta.titel, gruppe: meta.gruppe, reihenfolge: Number(meta.reihenfolge ?? 99),
             beschreibung: meta.beschreibung ?? "", quelle: meta.quelle, markdown,
             datei: meta.quelle ?? `apps/web/src/lib/inhalt/doku/${sprache}/${d}` };
  }).sort((a, b) => GRUPPEN[sprache].indexOf(a.gruppe) - GRUPPEN[sprache].indexOf(b.gruppe) || a.reihenfolge - b.reihenfolge);
  cache.set(sprache, liste);
  return liste;
}

/** Pfade derselben Seite in beiden Sprachen (für hreflang und Sprachumschalter). */
export function alternativen(seite: Seite): Record<string, string> {
  const r: Record<string, string> = {};
  for (const s of ["de", "en"] as Sprache[]) {
    const gegen = seiten(s).find((x) => x.schluessel === seite.schluessel);
    if (gegen) r[s] = docsPfad(s, gegen.slug);
  }
  return r;
}

let hl: Promise<Highlighter> | undefined;
const highlighter = () => (hl ??= createHighlighter({
  themes: ["github-dark-dimmed", "github-light"],
  langs: ["bash", "sh", "yaml", "json", "typescript", "go", "text", "markdown", "toml", "python", "gdscript"],
}));

/** Rendert eine Seite zu HTML und liefert die Überschriften fürs Inhaltsverzeichnis. */
export async function rendern(seite: Seite) {
  const h = await highlighter();
  const slugger = new GithubSlugger();
  const ueberschriften: Ueberschrift[] = [];
  const geladen = h.getLoadedLanguages();
  const anker = seite.sprache === "en" ? "Link to this section" : "Link zu diesem Abschnitt";
  const kopieren = seite.sprache === "en" ? ["Copy", "Copy code"] : ["Kopieren", "Code kopieren"];
  const md = new Marked({
    async: true,
    gfm: true,
    renderer: {
      heading({ tokens, depth, text }: Tokens.Heading) {
        const id = slugger.slug(text.replace(/<[^>]+>/g, ""));
        const inhalt = this.parser.parseInline(tokens);
        if (depth === 2 || depth === 3) ueberschriften.push({ id, text: klartext(inhalt), ebene: depth });
        return `<h${depth} id="${id}"><a class="anker" href="#${id}" aria-label="${anker}">#</a>${inhalt}</h${depth}>\n`;
      },
      code({ text, lang }: Tokens.Code) {
        const sprache = lang && geladen.includes(lang) ? lang : "text";
        const html = h.codeToHtml(text, { lang: sprache, themes: { dark: "github-dark-dimmed", light: "github-light" }, defaultColor: false });
        return `<div class="code">${html}<button class="kopieren" type="button" data-kopieren aria-label="${kopieren[1]}">${kopieren[0]}</button></div>`;
      },
      link({ href, title, tokens }: Tokens.Link) {
        const text = this.parser.parseInline(tokens);
        let ziel = href;
        // relative Repo-Links (z. B. vorlagen/aufgabe.md) zeigen auf Gitea
        if (!/^(https?:|mailto:|#|\/)/.test(href)) {
          const basis = seite.quelle ? seite.quelle.replace(/[^/]+$/, "") : "docs/";
          ziel = `${REPO_WEB}/${join(basis, href)}`;
        }
        const extern = /^https?:/.test(ziel);
        return `<a href="${ziel}"${title ? ` title="${title}"` : ""}${extern ? ' rel="noopener"' : ""}>${text}</a>`;
      },
      blockquote({ tokens }: Tokens.Blockquote) {
        const inhalt = this.parser.parse(tokens);
        const art = /^<p><strong>(Hinweis|Stand|Achtung|Wichtig|Note|Status|Warning|Important)/.exec(inhalt)?.[1]?.toLowerCase() ?? "hinweis";
        const klasse = ["achtung", "wichtig", "warning", "important"].includes(art) ? "achtung" : "hinweis";
        return `<aside class="hinweis ${klasse}">${inhalt}</aside>\n`;
      },
      table(token: Tokens.Table) {
        const kopf = token.header.map((c) => `<th>${this.parser.parseInline(c.tokens)}</th>`).join("");
        const zeilen = token.rows.map((r) => `<tr>${r.map((c) => `<td>${this.parser.parseInline(c.tokens)}</td>`).join("")}</tr>`).join("");
        return `<div class="tabelle"><table><thead><tr>${kopf}</tr></thead><tbody>${zeilen}</tbody></table></div>\n`;
      },
    },
  });
  const html = await md.parse(seite.markdown);
  return { html, ueberschriften };
}

/** Navigation: Gruppen mit Seiten. */
export function navigation(sprache: Sprache) {
  return GRUPPEN[sprache].map((g) => ({ gruppe: g, seiten: seiten(sprache).filter((s) => s.gruppe === g).map(({ slug, titel }) => ({ slug, titel })) }))
    .filter((g) => g.seiten.length);
}

/** Suchindex: pro Seite die Abschnitte mit Klartext. */
export function suchindex(sprache: Sprache) {
  return seiten(sprache).map((s) => {
    const slugger = new GithubSlugger();
    const abschnitte: { id: string; titel: string; text: string }[] = [{ id: "", titel: s.titel, text: s.beschreibung }];
    for (const zeile of s.markdown.split("\n")) {
      const m = /^(#{2,3})\s+(.*)$/.exec(zeile);
      if (m) abschnitte.push({ id: slugger.slug(m[2]), titel: m[2].replace(/[*`]/g, ""), text: "" });
      else abschnitte[abschnitte.length - 1].text += " " + zeile.replace(/[#*`|>_\[\]()]/g, " ");
    }
    return { slug: s.slug, titel: s.titel, gruppe: s.gruppe,
             abschnitte: abschnitte.map((a) => ({ ...a, text: a.text.replace(/\s+/g, " ").trim().slice(0, 600) })) };
  });
}
