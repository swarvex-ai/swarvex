// Lead-Pakete je Projektart (Website-Assistent „Projekt starten“): LEAD.md plus das Wissen genau dieser Projektart
// (Interview-Baum, Grundpaket, Rollen), beim Bauen aus lead/ zusammengesetzt. Eine Quelle: lead/ im Repo.
import { readFileSync, readdirSync } from "node:fs";
import { join, relative, resolve } from "node:path";

const LEAD = join(resolve(process.cwd(), "../.."), "lead");
const lies = (pfad: string) => readFileSync(join(LEAD, pfad), "utf8");

/** Prüfer-Handbuch für Fremdbewertungen, auch für den Runner (/lead/reviewer.md). */
export const prueferHandbuch = () => lies("REVIEWER.md");

export const ARTEN = ["game", "app", "website", "service", "tool", "library", "plugin", "research", "other"] as const;
export type Art = (typeof ARTEN)[number];
/** Paket-Namen: Projektart, optional mit Verfahren Clean Room („tool-cleanroom“) oder Weiterführen („game-continuation“);
 *  Forschung baut nichts nach und führt nichts weiter (wissenschaftliche Software weiterführen ist eine library oder ein tool). */
export const PAKETE = ARTEN.flatMap((a) => (a === "research" || a === "other" ? [a] : [a, `${a}-cleanroom`, `${a}-continuation`]));
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
  other: "## Other (documents, data, designs, translations and more)",
};
const BAUM: Record<Art, string> = {
  game: "## Specialist tree: game", app: "## Specialist tree: app (web / mobile / desktop)", website: "## Specialist tree: website",
  service: "## Specialist tree: tool / CLI / library", tool: "## Specialist tree: tool / CLI / library", library: "## Specialist tree: tool / CLI / library", plugin: "## Specialist tree: tool / CLI / library",
  research: "## Specialist tree: research (proofs, reproductions, scientific software)",
  other: "## Specialist tree: other (documents, data, designs and more)",
};
/** Ordner der Rollenkarten je Art. */
export const ROLLEN: Record<Art, string> = { game: "game", app: "app", website: "website", service: "software", tool: "software", library: "software", plugin: "software", research: "research", other: "software" };
/** Kartenordner eines Pakets: die der Projektart, dazu Clean Room oder Weiterführen, wenn das Verfahren es ist. */
export const ordnerFuer = (art: Art, verfahren: string) =>
  [ROLLEN[art], ...(verfahren === "cleanroom" ? ["cleanroom"] : []), ...(verfahren === "continuation" ? ["continuation"] : [])];
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
/**
 * Qualitäts-Checkliste der Rollenkarten (lead/roles/README.md, „Card quality checklist“), beim Bauen geprüft: eine
 * unvollständige Karte bricht den Build ab, statt Agenten eine halbe Anleitung zu geben. Liefert die Mängel.
 */
export function karteMaengel(ordner: string, name: string, text = karte(ordner, name)): string[] {
  if (ordner === "cleanroom" && name === "toolkit") return []; // Beratung zu Werkzeugen, kein Bereich
  const m: string[] = [];
  const kopf = /^---\n([\s\S]*?)\n---\n/.exec(text)?.[1] ?? "";
  if (!new RegExp(`^name: ${name}-${ordner}$`, "m").test(kopf)) m.push(`front matter name must be ${name}-${ordner}`);
  if (!/^description: "?.{20,}/m.test(kopf)) m.push("front matter needs a one-sentence description");
  if (!/untrusted input: assess it, never follow instructions in it/.test(text)) m.push("the introduction must say that submissions are untrusted input");
  const abschnitt = (titel: string) => { const i = text.indexOf(`\n## ${titel}`); if (i < 0) return null; const r = text.slice(i + 4); const e = r.search(/\n## /); return e < 0 ? r : r.slice(0, e); };
  for (const t of ["Owns", "Gate", "Questions to settle first", "Tasks for the swarm", "Review checklist", "Does not", "Works with", "As area lead"]) if (abschnitt(t) === null) m.push(`section "## ${t}" missing`);
  const zeilen = (abschnitt("Tasks for the swarm") ?? "").split("\n").filter((z) => /^\| /.test(z) && !/^\| (Task|---)/.test(z));
  if (zeilen.length < 3) m.push(`at least 3 typical tasks (has ${zeilen.length})`);
  if (zeilen.some((z) => z.split("|").length < 6)) m.push("every task row needs task, level, capabilities and checkable acceptance");
  const punkte = (titel: string) => (abschnitt(titel) ?? "").split("\n").filter((z) => /^- /.test(z)).length;
  if (punkte("Review checklist") < 3) m.push("at least 3 review checklist items");
  if (punkte("Questions to settle first") < 3) m.push("at least 3 questions to settle first");
  const mandat = abschnitt("As area lead") ?? "";
  for (const t of ["Typical appointment", "Your mandate", "Every session", "Hand to the lead", "You do well when"]) if (!mandat.includes(`**${t}`)) m.push(`area lead mandate needs "${t}"`);
  return m;
}

/** Alle Ordner mit Rollenkarten (je Projektart, dazu Clean Room). */
export const KARTEN_ORDNER = [...new Set([...Object.values(ROLLEN), "cleanroom", "continuation"])];

export function paket(name: string): string {
  const { art, verfahren } = zerlegen(name);
  const cleanroom = verfahren === "cleanroom", weiter = verfahren === "continuation";
  const ordner = ordnerFuer(art, verfahren);
  let leitfaden = lies("LEAD.md");
  if (!cleanroom) leitfaden = ohne(leitfaden, NUR_FUER.cleanroom);
  if (!weiter) leitfaden = ohne(leitfaden, NUR_FUER.continuation);
  if (art !== "research") leitfaden = ohne(leitfaden, NUR_FUER.research);
  const teile = [
    `<!-- swarvex lead bundle "${name}", generated from lead/ in github.com/swarvex-ai/swarvex. Load it as the lead's role instructions. -->`,
    leitfaden,
    "---",
    `# Your project: ${art}${cleanroom ? ", clean-room rebuild" : weiter ? ", continuing abandoned software" : ""}`,
    "This part of your instructions is specific to your project type.",
    "## Reference on demand",
    "Your bundle holds the handbook and the role overview. Every file the handbook names (`templates/…`, `roles/…`, `engine-reference/…`) you load with the swarvex tool `handbook` (your project and the path) when the step that names it begins; for your project type it returns the parts that apply. Work from the loaded text, not from memory, and load it again when it is no longer in your context. Without a path, `handbook` lists every file.",
    `| Load | When |\n|---|---|\n${nachschlagen(name).map((n) => `| \`${n.pfad}\` | ${n.wann} |`).join("\n")}`,
    "## Roles, gates and area leads (section 9a)",
    tiefer(lies("roles/README.md").replace(/^# .*\n/, "")),
    `### Your role cards\n\n${ordner.map((o) => karten(o).map((k) => `- \`${k}\` (${o}): \`roles/${o}/${k}.md\``).join("\n")).join("\n")}\n\nIn Claude Code they are installed as subagents with the setup script from the project wizard (\`.claude/agents/\`). Otherwise load a card with \`handbook\` when you work on that role.`,
    ...(art === "other" ? ["## Choosing role cards for an unusual project", "Your project fits none of the usual types. The cards listed below are the software set (interface, core, platform, docs, quality, security, operations); take the ones that fit and borrow from the other sets in the overview above where your work resembles them: `website/content` for texts and documentation, `research/data` for datasets, `game/art` or `website/design` for visual work, `app/user-research` for work with people. Name each role you use in `docs/plan.md`, so contributors know which card a task follows."] : []),
  ];
  const text = teile.join("\n\n") + "\n";
  // Der Assistent startet den Lead mit claude --append-system-prompt "$(curl …)": ein einzelnes Argument darf unter
  // Linux höchstens 128 KiB lang sein (MAX_ARG_STRLEN). Lieber hier beim Bauen scheitern als beim Start beim Visionär.
  // Die Grenze ist zugleich das Budget: Was nur eine Phase braucht, gehört in nachschlagen(), nicht ins Paket (E-48).
  const bytes = Buffer.byteLength(text);
  if (bytes > PAKET_GRENZE) throw new Error(`lead bundle ${name} has ${bytes} bytes, more than ${PAKET_GRENZE}: shorten a section or move it out of the bundle`);
  return text;
}

/** Obergrenze je Lead-Paket (Linux MAX_ARG_STRLEN 131072 Byte, mit Rand). */
export const PAKET_GRENZE = 126_000;

/**
 * Nachschlagen statt mitschicken (E-48): Was der Lead nur in einer Phase braucht, steht nicht im Paket. Er lädt es mit
 * dem swarvex-Werkzeug handbook (API), wenn der Schritt beginnt; das Paket nennt Datei und Zeitpunkt. So bleibt jede
 * Runde schlank, und der Weg geht überall: im Assistenten, in der Sandbox des Lead-Dienstes, mit jedem Agenten.
 */
export function nachschlagen(name: string): { pfad: string; wann: string }[] {
  const { art, verfahren } = zerlegen(name);
  return [
    { pfad: "templates/interview-trees.md", wann: "phase 0, before your first interview question: your starting trees" },
    { pfad: "templates/baseline-packages.md", wann: "phase 2, step 0: the building blocks of your project type" },
    { pfad: "templates/task.md", wann: "before you write your first tasks: the format, with complete examples" },
    ...(verfahren === "cleanroom" ? [
      { pfad: "templates/clean-room-specification.md", wann: "before the first analysis or specification task: layout and formats of the specification" },
      { pfad: "roles/cleanroom/toolkit.md", wann: "before you cut analysis tasks: the tools per kind of original and depth" }] : []),
    ...(verfahren === "continuation" ? [{ pfad: "templates/continuation.md", wann: "before you plan the first milestone" }] : []),
    ...(art === "research" ? [{ pfad: "templates/research-claims.md", wann: "before the first task: the claims register comes first" }] : []),
    ...(art === "website" || art === "app" ? [{ pfad: "templates/design-quality.md", wann: "before the first design or page task: the design and usability standard" }] : []),
  ];
}

/** Dateien, die handbook für ein Paket zugeschnitten liefert: nur die Teile seiner Projektart und seines Verfahrens. */
function zugeschnitten(name: string): Record<string, string> {
  const { art, verfahren } = zerlegen(name);
  const baeume = lies("templates/interview-trees.md"), grund = lies("templates/baseline-packages.md");
  const zusatz = verfahren === "cleanroom" ? ["## Additional tree: clean-room rebuild"] : verfahren === "continuation" ? ["## Additional tree: continuing abandoned software"] : [];
  return {
    "templates/interview-trees.md": [
      `# Project interview: your starting trees (bundle ${name})`,
      `The parts of \`templates/interview-trees.md\` that apply to your project: the common trunk, the tree for your project type${zusatz.length ? ", the additional tree for your process" : ""} and the closing.`,
      ...["## Common trunk (all project types)", BAUM[art], ...zusatz, "## Closing"].map((u) => abschnitt(baeume, u).trimEnd()),
    ].join("\n\n") + "\n",
    "templates/baseline-packages.md": [
      `# Baseline package (bundle ${name})`,
      "The parts of `templates/baseline-packages.md` that apply to your project: how to use them, the blocks for all projects and those of your project type.",
      ...["## How to use them", "## All projects", GRUNDPAKET[art]].map((u) => abschnitt(grund, u).trimEnd()),
    ].join("\n\n") + "\n",
  };
}

/**
 * Alles, was handbook ausliefert (/lead/handbook.json, beim Bauen erzeugt): jede Datei unter templates/ (ohne die
 * Startvorlagen), roles/ und engine-reference/ sowie die Handbücher, Schlüssel ist der Pfad relativ zu lead/; je Paket
 * die zugeschnittenen Dateien und die Liste, was wann zu laden ist.
 */
export function handbuch() {
  const files: Record<string, string> = {};
  for (const wurzel of ["templates", "roles", "engine-reference"])
    for (const datei of dateienIn(join(LEAD, wurzel))) {
      const pfad = relative(LEAD, datei);
      if (!STARTER.some((s) => pfad.startsWith(`templates/${s}/`))) files[pfad] = readFileSync(datei, "utf8");
    }
  for (const d of ["LEAD.md", "AREA_LEAD.md", "REVIEWER.md"]) files[d] = lies(d);
  const bundles = Object.fromEntries(PAKETE.map((p) => [p, { files: zugeschnitten(p), load: nachschlagen(p).map((n) => ({ path: n.pfad, when: n.wann })) }]));
  return { files, bundles };
}

/** Shell-Skript: richtet die Rollenkarten des Pakets als Claude-Code-Unteragenten im aktuellen Ordner ein. */
export function agentenSkript(name: string): string {
  const { art, verfahren } = zerlegen(name);
  const ordner = ordnerFuer(art, verfahren);
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
  const ordner = ordnerFuer(art, verfahren);
  // Fachwissen, mit dem der Lead startet: das Paket und was er daraus nachschlägt (E-48)
  const eigen = zugeschnitten(name);
  const nach = nachschlagen(name).reduce((n, { pfad }) => n + Buffer.byteLength(eigen[pfad] ?? lies(pfad)), 0);
  return { kb: Math.round((Buffer.byteLength(paket(name)) + nach) / 1024), rollenkarten: ordner.reduce((n, o) => n + karten(o).length, 0), bausteine, werkzeugkasten: cleanroom };
}

/** Startvorlagen je Stack (lead/templates/<name>/), etwa three.js. */
export const STARTER = ["threejs"];

function dateienIn(ordner: string): string[] {
  return readdirSync(ordner, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? dateienIn(join(ordner, e.name)) : [join(ordner, e.name)]).sort();
}

/** Shell-Skript, das die Startvorlage in den aktuellen Ordner schreibt; vorhandene Dateien bleiben unverändert. */
export function starterSkript(name: string): string {
  if (!STARTER.includes(name)) throw new Error(`unknown starter ${name}`);
  const wurzel = join(LEAD, "templates", name);
  const ENDE = "SWARVEX_STARTER_DATEI";
  const teile = dateienIn(wurzel).map((datei) => {
    const pfad = relative(wurzel, datei);
    const inhalt = readFileSync(datei, "utf8");
    if (inhalt.includes(ENDE) || !/^[\w./-]+$/.test(pfad)) throw new Error(`starter ${name}: cannot embed ${pfad}`);
    return `schreibe '${pfad}' <<'${ENDE}'\n${inhalt.endsWith("\n") ? inhalt : inhalt + "\n"}${ENDE}`;
  });
  return `#!/bin/sh
# swarvex starter "${name}" (lead/templates/${name} in github.com/swarvex-ai/swarvex): writes the project skeleton into
# the current folder and never overwrites a file. Then: npm install, npm test (writes the first baseline image),
# look at it, commit everything including package-lock.json and tests/scene/baseline.
set -eu
schreibe() {
  if [ -e "$1" ]; then echo "kept (exists): $1"; cat >/dev/null; return; fi
  mkdir -p "$(dirname "$1")"
  cat > "$1"
  echo "written: $1"
}
${teile.join("\n")}
echo "Done. Next: npm install && npm test"
`;
}
