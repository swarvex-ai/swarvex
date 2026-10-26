// Rechtliche Seiten (Impressum, Datenschutz, Nutzungsbedingungen) aus src/lib/inhalt/recht/<sprache>.
// Betreiberangaben stehen einmal in angaben.json und werden als {{feld}} eingesetzt; {{?feld}} ist optional
// (Zeile entfällt, wenn leer). Fehlt eine Pflichtangabe, steht sichtbar „fehlt: …“ auf der Seite.
import { Marked } from "marked";
import type { Sprache } from "$lib/texte";
import angaben from "$lib/inhalt/recht/angaben.json";

// Beim Bauen eingebunden: Der Web-Container enthält keine Quelldateien.
const DATEIEN = import.meta.glob("/src/lib/inhalt/recht/*/*.md", { query: "?raw", import: "default", eager: true }) as Record<string, string>;

/** Adressen je Seite und Sprache; der Schlüssel verbindet beide Sprachen. */
export const RECHT_SEITEN = {
  impressum: { de: "impressum", en: "legal-notice" },
  datenschutz: { de: "datenschutz", en: "privacy" },
  nutzungsbedingungen: { de: "nutzungsbedingungen", en: "terms" },
} as const;
export type RechtSchluessel = keyof typeof RECHT_SEITEN;

export const rechtPfad = (s: Sprache, k: RechtSchluessel) => `${s === "en" ? "/en" : ""}/${RECHT_SEITEN[k][s]}`;

const marked = new Marked({ breaks: true });

function wert(angaben: Record<string, unknown>, pfad: string): string {
  const w = pfad.split(".").reduce<unknown>((o, k) => (o && typeof o === "object" ? (o as Record<string, unknown>)[k] : undefined), angaben);
  return typeof w === "string" ? w.trim() : "";
}

export function einsetzen(md: string, angaben: Record<string, unknown>): { md: string; fehlt: string[] } {
  const fehlt: string[] = [];
  const zeilen = md.split("\n").filter((z) => [...z.matchAll(/\{\{\?([\w.]+)\}\}/g)].every((m) => wert(angaben, m[1]) !== ""));
  const ergebnis = zeilen.join("\n").replace(/\{\{\??([\w.]+)\}\}/g, (_, pfad: string) => {
    const w = wert(angaben, pfad);
    if (!w) { fehlt.push(pfad); return `<mark class="fehlt">fehlt: ${pfad}</mark>`; }
    return w.replace(/[<>&]/g, (z) => `&#${z.charCodeAt(0)};`);
  });
  return { md: ergebnis, fehlt };
}

export function rechtSeite(s: Sprache, slug: string) {
  const schluessel = (Object.keys(RECHT_SEITEN) as RechtSchluessel[]).find((k) => RECHT_SEITEN[k][s] === slug);
  if (!schluessel) return null;
  const roh = DATEIEN[`/src/lib/inhalt/recht/${s}/${slug}.md`];
  if (roh === undefined) return null;
  const m = /^---\n([\s\S]*?)\n---\n?/.exec(roh);
  const meta: Record<string, string> = {};
  if (m) for (const z of m[1].split("\n")) { const i = z.indexOf(":"); if (i > 0) meta[z.slice(0, i).trim()] = z.slice(i + 1).trim(); }
  const { md, fehlt } = einsetzen(m ? roh.slice(m[0].length) : roh, angaben);
  return {
    schluessel, titel: meta.titel, beschreibung: meta.beschreibung ?? "", html: marked.parse(md) as string, fehlt,
    alternativen: { de: rechtPfad("de", schluessel), en: rechtPfad("en", schluessel) },
  };
}
