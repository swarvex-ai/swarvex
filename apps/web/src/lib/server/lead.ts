// Lead-Pakete je Projektart (Website-Assistent „Projekt starten“): LEAD.md plus das Wissen genau dieser Projektart
// (Interview-Baum, Grundpaket, Rollen), beim Bauen aus lead/ zusammengesetzt. Eine Quelle: lead/ im Repo.
import { readFileSync, readdirSync } from "node:fs";
import { join, resolve } from "node:path";

const LEAD = join(resolve(process.cwd(), "../.."), "lead");
const lies = (pfad: string) => readFileSync(join(LEAD, pfad), "utf8");
const ROH = "https://raw.githubusercontent.com/swarvex-ai/swarvex/main/lead";

export const ARTEN = ["game", "app", "website", "service", "tool", "library", "plugin", "research"] as const;
export type Art = (typeof ARTEN)[number];
/** Paket-Namen: Projektart, optional mit Verfahren Clean Room („tool-cleanroom“) oder Weiterführen („game-continuation“);
 *  Forschung baut nichts nach und führt nichts weiter (wissenschaftliche Software weiterführen ist eine library oder ein tool). */
export const PAKETE = ARTEN.flatMap((a) => (a === "research" ? [a] : [a, `${a}-cleanroom`, `${a}-continuation`]));
type Verfahren = "standard" | "cleanroom" | "continuation";
function zerlegen(name: string): { art: Art; verfahren: Verfahren } {
  if (!PAKETE.includes(name)) throw new Error(`unknown bundle ${name}`);
  const m = /^(.*?)(?:-(cleanroom|continuation))?$/.exec(name)!;
  return { art: m[1] as Art, verfahren: (m[2] ?? "standard") as Verfahren };
}

const GRUNDPAKET: Record<Art, string> = {
  game: "## Game", app: "## App (web, mobile, desktop)", website: "## Website (landing page, documentation, content)",
  service: "## Web service or API (server, backend)", tool: "## Tool (command line)", library: "## Library or SDK", plugin: "## Plugin or extension (for a host program)",
  research: "## Research (formal proofs, reproductions, scientific software)",
};
const BAUM: Record<Art, string> = {
  game: "## Specialist tree: game", app: "## Specialist tree: app (web / mobile / desktop)", website: "## Specialist tree: website",
  service: "## Specialist tree: tool / CLI / library", tool: "## Specialist tree: tool / CLI / library", library: "## Specialist tree: tool / CLI / library", plugin: "## Specialist tree: tool / CLI / library",
  research: "## Specialist tree: research (proofs, reproductions, scientific software)",
};
/** Ordner der Rollenkarten je Art. */
export const ROLLEN: Record<Art, string> = { game: "game", app: "app", website: "website", service: "software", tool: "software", library: "software", plugin: "software", research: "research" };
/** Abschnitte in LEAD.md (9. Projektarten), die nur ihr eigenes Paket braucht; die übrigen Pakete bleiben so unter der Grenze für Befehlszeilen. */
const NUR_FUER = { cleanroom: "### Clean-room rebuild (process `clean_room`)", continuation: "### Continuing abandoned software (process `continuation`)",
  research: "### Research and verification (type `research`)" };

/** Ein Abschnitt (## …) bis zum nächsten gleich hohen. */
function abschnitt(md: string, ueberschrift: string): string {
  const i = md.indexOf(`\n${ueberschrift}\n`);
  if (i < 0) throw new Error(`section not found: ${ueberschrift}`);
  const rest = md.slice(i + 1);
  const ende = rest.slice(3).search(/\n## /);
  return ende < 0 ? rest : rest.slice(0, ende + 4);
}

/** Einen ###-Abschnitt weglassen, bis zum nächsten Abschnitt oder Trennstrich. */
function ohne(md: string, ueberschrift: string): string {
  const i = md.indexOf(`\n${ueberschrift}\n`);
  if (i < 0) throw new Error(`section not found: ${ueberschrift}`);
  const rest = md.slice(i + 1);
  const ende = rest.slice(4).search(/\n(#{2,3} |---\n)/);
  return md.slice(0, i + 1) + (ende < 0 ? "" : rest.slice(ende + 5));
}

/** Überschriften eine Ebene tiefer (außerhalb von Code-Blöcken), damit eingebettete Dokumente in die Gliederung passen. */
function tiefer(md: string, stufen = 1): string {
  let code = false;
  return md.split("\n").map((z) => {
    if (z.startsWith("```")) code = !code;
    return !code && /^#{1,5} /.test(z) ? "#".repeat(stufen) + z : z;
  }).join("\n");
}

export const karten = (ordner: string) => readdirSync(join(LEAD, "roles", ordner)).filter((d) => d.endsWith(".md")).map((d) => d.replace(/\.md$/, "")).sort();
export const karte = (ordner: string, name: string) => lies(`roles/${ordner}/${name}.md`);
/** Alle Ordner mit Rollenkarten (je Projektart, dazu Clean Room). */
export const KARTEN_ORDNER = [...new Set([...Object.values(ROLLEN), "cleanroom"])];

export function paket(name: string): string {
  const { art, verfahren } = zerlegen(name);
  const cleanroom = verfahren === "cleanroom", weiter = verfahren === "continuation";
  const baeume = lies("templates/interview-trees.md"), grund = lies("templates/baseline-packages.md");
  const ordner = [ROLLEN[art], ...(cleanroom ? ["cleanroom"] : [])];
  let leitfaden = lies("LEAD.md");
  if (!cleanroom) leitfaden = ohne(leitfaden, NUR_FUER.cleanroom);
  if (!weiter) leitfaden = ohne(leitfaden, NUR_FUER.continuation);
  if (art !== "research") leitfaden = ohne(leitfaden, NUR_FUER.research);
  const teile = [
    `<!-- swarvex lead bundle "${name}", generated from lead/ in github.com/swarvex-ai/swarvex. Load it as the lead's role instructions. -->`,
    leitfaden,
    "---",
    `# Your project: ${art}${cleanroom ? ", clean-room rebuild" : weiter ? ", continuing abandoned software" : ""}`,
    `This part of your instructions is specific to your project type. The handbook above refers to templates; they are in this bundle where you need them first, and all of them are at ${ROH}/templates/ (task.md, project-brief.md, asset-brief.md, review-report.md, decision.md, status-report.md, AGENTS.md, swarvex.yaml, skills/).`,
    "## Project interview (phase 0): your starting trees",
    abschnitt(baeume, "## Common trunk (all project types)").replace(/^## /, "### "),
    abschnitt(baeume, BAUM[art]).replace(/^## /, "### "),
    ...(cleanroom ? [abschnitt(baeume, "## Additional tree: clean-room rebuild").replace(/^## /, "### ")] : []),
    ...(weiter ? [abschnitt(baeume, "## Additional tree: continuing abandoned software").replace(/^## /, "### ")] : []),
    abschnitt(baeume, "## Closing").replace(/^## /, "### "),
    "## Baseline package (phase 2, step 0)",
    abschnitt(grund, "## How to use them").replace(/^## /, "### "),
    abschnitt(grund, "## All projects").replace(/^## /, "### "),
    abschnitt(grund, GRUNDPAKET[art]).replace(/^## /, "### "),
    "## Roles, gates and area leads (section 9a)",
    tiefer(lies("roles/README.md").replace(/^# .*\n/, "")),
    `### Your role cards\n\n${ordner.map((o) => karten(o).map((k) => `- \`${k}\` (${o}): ${ROH}/roles/${o}/${k}.md`).join("\n")).join("\n")}\n\nIn Claude Code they are installed as subagents with the setup script from the project wizard (\`.claude/agents/\`). Other agents read a card when they work on that role.`,
    ...(cleanroom ? ["## Clean-room specification format", tiefer(lies("templates/clean-room-specification.md").replace(/^# .*\n/, "")),
      // Werkzeuge je Art des Originals und Tiefe: der Lead braucht sie schon beim Schneiden der ersten Analyse-Aufgaben
      "## Analysis toolkit", tiefer(karte("cleanroom", "toolkit").replace(/^---\n[\s\S]*?\n---\n/, "").replace(/^\s*# .*\n/, ""))] : []),
    ...(weiter ? ["## First milestone of a continuation", tiefer(lies("templates/continuation.md").replace(/^# .*\n/, ""))] : []),
    ...(art === "research" ? ["## Claims register format", tiefer(lies("templates/research-claims.md").replace(/^# .*\n/, ""))] : []),
    // Websites und Apps: der Design- und Usability-Standard, bevor die ersten Design- und Seitenaufgaben geschnitten werden
    ...(art === "website" || art === "app" ? ["## Design and usability standard", tiefer(lies("templates/design-quality.md").replace(/^# .*\n/, ""))] : []),
  ];
  const text = teile.join("\n\n") + "\n";
  // Der Assistent startet den Lead mit claude --append-system-prompt "$(curl …)": ein einzelnes Argument darf unter
  // Linux höchstens 128 KiB lang sein (MAX_ARG_STRLEN). Lieber hier beim Bauen scheitern als beim Start beim Visionär.
  const bytes = Buffer.byteLength(text);
  if (bytes > PAKET_GRENZE) throw new Error(`lead bundle ${name} has ${bytes} bytes, more than ${PAKET_GRENZE}: shorten a section or move it out of the bundle`);
  return text;
}

/** Obergrenze je Lead-Paket (Linux MAX_ARG_STRLEN 131072 Byte, mit Rand). */
export const PAKET_GRENZE = 126_000;

/** Shell-Skript: richtet die Rollenkarten des Pakets als Claude-Code-Unteragenten im aktuellen Ordner ein. */
export function agentenSkript(name: string): string {
  const { art, verfahren } = zerlegen(name);
  const ordner = [ROLLEN[art], ...(verfahren === "cleanroom" ? ["cleanroom"] : [])];
  const zeilen = ordner.flatMap((o) => karten(o).map((k) => `hole ${o} ${k}`));
  return `#!/bin/sh
# swarvex: installs the role cards for a ${name} project as Claude Code subagents in ./.claude/agents
# (lead/roles in github.com/swarvex-ai/swarvex). Run it in the lead's working folder.
set -eu
BASIS="\${SWARVEX:-https://swarvex.ai}"
mkdir -p .claude/agents
hole() { curl -fsSL "$BASIS/lead/roles/$1/$2.md" -o ".claude/agents/$2-$1.md"; }
${zeilen.join("\n")}
echo "${zeilen.length} role cards installed in .claude/agents"
`;
}

/** Kennzahlen eines Pakets für die Schwarm-Bilanz der Projektseite („Wissensvorsprung“). */
export function paketStatistik(name: string) {
  const { art, verfahren } = zerlegen(name);
  const cleanroom = verfahren === "cleanroom";
  const grund = lies("templates/baseline-packages.md");
  const bausteine = [abschnitt(grund, "## All projects"), abschnitt(grund, GRUNDPAKET[art])].join("\n").split("\n").filter((z) => /^\| [A-Z]+-\d+ \|/.test(z)).length;
  const ordner = [ROLLEN[art], ...(cleanroom ? ["cleanroom"] : [])];
  return { kb: Math.round(Buffer.byteLength(paket(name)) / 1024), rollenkarten: ordner.reduce((n, o) => n + karten(o).length, 0), bausteine, werkzeugkasten: cleanroom };
}
