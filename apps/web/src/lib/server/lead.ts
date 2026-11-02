// Lead-Pakete je Projektart (Website-Assistent „Projekt starten“): LEAD.md plus das Wissen genau dieser Projektart
// (Interview-Baum, Grundpaket, Rollen), beim Bauen aus lead/ zusammengesetzt. Eine Quelle: lead/ im Repo.
import { readFileSync, readdirSync } from "node:fs";
import { join, resolve } from "node:path";

const LEAD = join(resolve(process.cwd(), "../.."), "lead");
const lies = (pfad: string) => readFileSync(join(LEAD, pfad), "utf8");
const ROH = "https://raw.githubusercontent.com/swarvex-ai/swarvex/main/lead";

export const ARTEN = ["game", "app", "website", "service", "tool", "library", "plugin"] as const;
export type Art = (typeof ARTEN)[number];
/** Paket-Namen: Projektart, optional mit Clean-Room-Verfahren („tool-cleanroom“). */
export const PAKETE = ARTEN.flatMap((a) => [a, `${a}-cleanroom`]);

const GRUNDPAKET: Record<Art, string> = {
  game: "## Game", app: "## App (web, mobile, desktop)", website: "## Website (landing page, documentation, content)",
  service: "## Web service or API (server, backend)", tool: "## Tool (command line)", library: "## Library or SDK", plugin: "## Plugin or extension (for a host program)",
};
const BAUM: Record<Art, string> = {
  game: "## Specialist tree: game", app: "## Specialist tree: app (web / mobile / desktop)", website: "## Specialist tree: app (web / mobile / desktop)",
  service: "## Specialist tree: tool / CLI / library", tool: "## Specialist tree: tool / CLI / library", library: "## Specialist tree: tool / CLI / library", plugin: "## Specialist tree: tool / CLI / library",
};
/** Ordner der Rollenkarten je Art. */
export const ROLLEN: Record<Art, string> = { game: "game", app: "app", website: "app", service: "software", tool: "software", library: "software", plugin: "software" };

/** Ein Abschnitt (## …) bis zum nächsten gleich hohen. */
function abschnitt(md: string, ueberschrift: string): string {
  const i = md.indexOf(`\n${ueberschrift}\n`);
  if (i < 0) throw new Error(`section not found: ${ueberschrift}`);
  const rest = md.slice(i + 1);
  const ende = rest.slice(3).search(/\n## /);
  return ende < 0 ? rest : rest.slice(0, ende + 4);
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

export function paket(name: string): string {
  const cleanroom = name.endsWith("-cleanroom");
  const art = name.replace(/-cleanroom$/, "") as Art;
  if (!ARTEN.includes(art)) throw new Error(`unknown bundle ${name}`);
  const baeume = lies("templates/interview-trees.md"), grund = lies("templates/baseline-packages.md");
  const ordner = [ROLLEN[art], ...(cleanroom ? ["cleanroom"] : [])];
  const teile = [
    `<!-- swarvex lead bundle "${name}", generated from lead/ in github.com/swarvex-ai/swarvex. Load it as the lead's role instructions. -->`,
    lies("LEAD.md"),
    "---",
    `# Your project: ${art}${cleanroom ? ", clean-room rebuild" : ""}`,
    `This part of your instructions is specific to your project type. The handbook above refers to templates; they are in this bundle where you need them first, and all of them are at ${ROH}/templates/ (task.md, project-brief.md, asset-brief.md, review-report.md, decision.md, status-report.md, AGENTS.md, swarvex.yaml, skills/).`,
    "## Project interview (phase 0): your starting trees",
    abschnitt(baeume, "## Common trunk (all project types)").replace(/^## /, "### "),
    abschnitt(baeume, BAUM[art]).replace(/^## /, "### "),
    ...(cleanroom ? [abschnitt(baeume, "## Additional tree: clean-room rebuild").replace(/^## /, "### ")] : []),
    abschnitt(baeume, "## Closing").replace(/^## /, "### "),
    "## Baseline package (phase 2, step 0)",
    abschnitt(grund, "## How to use them").replace(/^## /, "### "),
    abschnitt(grund, "## All projects").replace(/^## /, "### "),
    abschnitt(grund, GRUNDPAKET[art]).replace(/^## /, "### "),
    "## Roles, gates and officers (section 9a)",
    tiefer(lies("roles/README.md").replace(/^# .*\n/, "")),
    `### Your role cards\n\n${ordner.map((o) => karten(o).map((k) => `- \`${k}\` (${o}): ${ROH}/roles/${o}/${k}.md`).join("\n")).join("\n")}\n\nIn Claude Code they are installed as subagents with the setup script from the project wizard (\`.claude/agents/\`). Other agents read a card when they work on that role.`,
    ...(cleanroom ? ["## Clean-room specification format", tiefer(lies("templates/clean-room-specification.md").replace(/^# .*\n/, ""))] : []),
  ];
  return teile.join("\n\n") + "\n";
}

/** Shell-Skript: richtet die Rollenkarten des Pakets als Claude-Code-Unteragenten im aktuellen Ordner ein. */
export function agentenSkript(name: string): string {
  const cleanroom = name.endsWith("-cleanroom");
  const art = name.replace(/-cleanroom$/, "") as Art;
  const ordner = [ROLLEN[art], ...(cleanroom ? ["cleanroom"] : [])];
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
