// Nachschlagen statt mitschicken (E-48): Das Lead-Paket enthält das Handbuch und die Rollenübersicht. Was der Lead nur
// in einer Phase braucht (Interview-Bäume, Grundpaket, Clean-Room-Formate, Werkzeugkasten, Design-Standard) und jede
// Vorlage, Rollenkarte und Engine-Referenz holt er mit dem Werkzeug handbook, wenn der Schritt beginnt. Der Weg geht
// überall: im Assistenten wie in der Sandbox des Lead-Dienstes (die nur Modell, Plattform und Git-Host erreicht) und mit
// jedem Agenten, der swarvex über MCP angebunden hat.
//
// Eine Quelle: lead/ im Repo. Die Website erzeugt beim Bauen /lead/handbook.json (apps/web/src/lib/server/lead.ts,
// handbuch()); die API holt es intern (WEB_INTERNAL) und hält es zehn Minuten. Inhalte sind öffentlich, der Pfad ist nur
// ein Schlüssel in dieser Liste, nie ein Dateipfad.
import type { Db } from "./db.ts";
import { Abgelehnt } from "./aufgaben.ts";
import { WERTE } from "./aussen.ts";

type Handbuch = {
  files: Record<string, string>;
  bundles: Record<string, { files: Record<string, string>; load: { path: string; when: string }[] }>;
};

const HALTEN_MS = 10 * 60_000;
let vorrat: { zeit: number; quelle: string; daten: Handbuch } | null = null;

async function handbuchLaden(): Promise<Handbuch> {
  const quelle = `${process.env.WEB_INTERNAL ?? process.env.BASE_URL ?? "http://web:3000"}/lead/handbook.json`;
  if (vorrat && vorrat.quelle === quelle && Date.now() - vorrat.zeit < HALTEN_MS) return vorrat.daten;
  try {
    const r = await fetch(quelle, { signal: AbortSignal.timeout(10_000) });
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    const daten = (await r.json()) as Handbuch;
    if (!daten?.files || !daten?.bundles) throw new Error("not a handbook");
    vorrat = { zeit: Date.now(), quelle, daten };
    return daten;
  } catch (e) {
    console.error(`handbook: ${quelle}: ${(e as Error).message}`);
    if (vorrat?.quelle === quelle) return vorrat.daten; // lieber der letzte Stand als keiner
    throw new Abgelehnt("the handbook cannot be loaded right now; try again in a minute");
  }
}

/** Name des Lead-Pakets aus Projektart und Verfahren, wie der Runner (runner/lead.go, leadPaket). */
export function paketName(projektart: string | null, verfahren: string | null): string {
  const art = WERTE.projektart[projektart ?? ""] ?? "other";
  if ((verfahren === "clean_room" || verfahren === "continuation") && art !== "research" && art !== "other")
    return `${art}-${verfahren === "clean_room" ? "cleanroom" : "continuation"}`;
  return art;
}

/** Eine Datei des Handbuchs, für dieses Projekt zugeschnitten; ein Ordner listet seine Dateien; ohne Pfad: was wann, und alles. */
export async function handbuchHolen(db: Db, slug: string, pfad?: string): Promise<string> {
  const p = (await db.query("select projektart, verfahren from projekte where slug = $1", [slug])).rows[0];
  if (!p) throw new Abgelehnt(`unknown project ${slug}`);
  const paket = paketName(p.projektart, p.verfahren);
  const h = await handbuchLaden();
  const eigen = h.bundles[paket] ?? { files: {}, load: [] };
  const alle = Object.keys(h.files).sort();
  const sauber = (pfad ?? "").trim().replace(/^(\.?\/)+/, "").replace(/^lead\//, "");
  if (!sauber) {
    return [
      `# Handbook files for ${slug} (lead bundle ${paket})`,
      "",
      "Load these when the step begins:",
      ...eigen.load.map((l) => `- \`${l.path}\`: ${l.when}`),
      "",
      "All files (pass one as path; a folder lists its files):",
      ...alle.map((d) => `- \`${d}\``),
    ].join("\n") + "\n";
  }
  const text = eigen.files[sauber] ?? h.files[sauber];
  if (text !== undefined) return text;
  const ordner = sauber.endsWith("/") ? sauber : `${sauber}/`;
  const darin = alle.filter((d) => d.startsWith(ordner));
  if (darin.length) return [`# ${ordner}`, "", ...darin.map((d) => `- \`${d}\``)].join("\n") + "\n";
  const name = sauber.split("/").pop()!;
  const aehnlich = alle.filter((d) => d === name || d.endsWith(`/${name}`));
  throw new Abgelehnt(`no handbook file ${sauber}${aehnlich.length ? `; did you mean ${aehnlich.join(", ")}?` : ""} Call handbook without path for the list.`);
}
