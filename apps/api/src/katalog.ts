// Der Asset-Katalog vor jeder Generierung (Entscheidung neVs 2026-11-08): Bevor ein Projekt ein Asset mit gespendeten
// Credits generieren lässt (Spendenauftrag) oder Formwerk um ein neues bittet (Bedarf), schaut swarvex in Formwerks
// Katalog. Formwerk ist das Hausprojekt mit den Assets als Code; seine Vorschau veröffentlicht nach jedem Merge
// catalog.json (Modelle, Oberflächen, Sprites, Icons, Klänge, je mit Titel, Beschreibung und Presets auf Deutsch und
// Englisch). Gibt es passende Assets, lehnt das Werkzeug die erste Anfrage ab und nennt sie; der Lead nimmt eines
// davon oder fragt erneut mit Begründung, warum keines passt. Die Begründung steht am Auftrag (für Spender und
// Formwerk-Lead sichtbar), die Auswertung zählt, wie oft trotz Treffer generiert wurde.
// Die Suche ist bewusst einfach (Wörter, Wortanfänge, zusammengesetzte Wörter), ohne Modell: Synonyme wie
// „Ohrensessel“ und „Stuhl“ findet sie nicht, dafür bleibt sie vorhersagbar und kostet nichts.
import type { Db } from "./db.ts";
import { einreichungsVorschauUrl, vorschauUrl } from "./konfig.ts";
import { Abgelehnt } from "./aufgaben.ts";

/** Das Projekt, dessen Vorschau den Katalog veröffentlicht. */
export const KATALOG_PROJEKT = "formwerk";
const ARTEN = { models: "model", surfaces: "surface", sprites: "sprite", icons: "icon", sounds: "sound" } as const;
export type Art = (typeof ARTEN)[keyof typeof ARTEN];
export const KATALOG_ARTEN = Object.values(ARTEN);
const NEU_LADEN_MS = 10 * 60_000, ZEITLIMIT_MS = 5000;
export const BEGRUENDUNG = { min: 10, max: 500 };

type Text = { de?: string; en?: string };
type Roh = { id: string; category?: string; title?: Text; description?: Text; function?: string; gallery?: string; file?: string;
  files?: Record<string, string>; presets?: Record<string, { label?: Text }> };
// Wörter mit Herkunft: true = aus einem deutschen Feld (nur dort gelten zusammengesetzte Wörter)
type Eintrag = { art: Art; roh: Roh; stark: Map<string, boolean>; schwach: Map<string, boolean> };
export type Treffer = { kind: Art; id: string; function?: string; title: Text; category?: string; gallery?: string; files?: Record<string, string>;
  presets: { name: string; label?: Text }[]; strong: boolean; matched: string[] };

/** Austauschbar für Tests: holt catalog.json. */
export const quelle = { holen: async (url: string): Promise<unknown> => {
  const r = await fetch(url, { signal: AbortSignal.timeout(ZEITLIMIT_MS), headers: { accept: "application/json" } });
  if (!r.ok) throw new Error(`HTTP ${r.status}`);
  return r.json();
} };

// Füllwörter: sie sagen nichts darüber, welches Asset gemeint ist
const FUELL = new Set(("der die das den dem des ein eine einen einem einer und oder mit ohne fur von zu im in am an auf als wie wir ich aus " +
  "brauchen braucht brauche soll sollen fuer bitte neu neue neuen neues unser unsere unseren eigen eigene " +
  "the a an and or with without for of to in on at as by from is it our we need needs needed new please like own " +
  "asset assets datei file files png obj usdz 2d low poly lowpoly game spiel " +
  "projekt project style stil look version one ones some any thing things ding dinge etwas").split(" "));

// Wörter, die die Art des Assets nennen („a click sound“): dann zählen nur Assets dieser Art als starker Treffer
const ART_WORT: Record<string, Art> = Object.fromEntries([
  ...["sound", "sounds", "klang", "klange", "gerausch", "gerausche", "audio", "sfx", "wav", "ogg", "mp3"].map((w) => [w, "sound"]),
  ...["icon", "icons", "symbol", "svg"].map((w) => [w, "icon"]),
  ...["sprite", "sprites", "spritesheet", "pixelart"].map((w) => [w, "sprite"]),
  ...["texture", "textures", "textur", "texturen", "surface", "surfaces", "oberflache", "oberflachen", "material", "tileable", "kachelbar"].map((w) => [w, "surface"]),
  ...["model", "models", "modell", "modelle", "mesh", "glb", "3d", "rigged", "rig", "geriggt"].map((w) => [w, "model"]),
]) as Record<string, Art>;

const normal = (text: string) => text.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/ß/g, "ss").toLowerCase();
/** Die Arten, die eine Anfrage nennt. */
export const artenIn = (text: string) => new Set(normal(text).split(/[^a-z0-9]+/).map((w) => ART_WORT[w]).filter(Boolean));

export function woerter(text: string): string[] {
  return normal(text).split(/[^a-z0-9]+/).filter((w) => w.length >= 3 && !FUELL.has(w) && !ART_WORT[w] && !/^\d+$/.test(w));
}

function eintragAus(art: Art, roh: Roh): Eintrag {
  const presets = Object.entries(roh.presets ?? {});
  const menge = (en: string[], de: string[]) => {
    const m = new Map<string, boolean>();
    for (const w of woerter(en.join(" "))) m.set(w, false);
    for (const w of woerter(de.join(" "))) m.set(w, true);
    return m;
  };
  return { art, roh,
    stark: menge([roh.id, roh.id.replace(/-/g, " "), roh.function ?? "", roh.title?.en ?? "", ...presets.flatMap(([n, p]) => [n.replace(/-/g, " "), p.label?.en ?? ""])],
      [roh.title?.de ?? "", ...presets.map(([, p]) => p.label?.de ?? "")]),
    schwach: menge([roh.category ?? "", roh.description?.en ?? ""], [roh.description?.de ?? ""]) };
}

// Wie gut ein Wort der Anfrage zu einem Wort des Katalogs passt: gleich, Wortanfang (der den größten Teil des Worts
// ausmacht: „wood“ zu „wooden“, nicht „holz“ zu „holzdielen“), oder bei deutschen Wörtern das Ende eines
// zusammengesetzten Worts („Holzkiste“ zu „Kiste“, „Dielen“ zu „Holzdielen“). Im Englischen wären das Zufälle
// („password“ endet auf „sword“, „villager“ auf „lager“).
function passt(q: string, e: string, deutsch: boolean): number {
  if (q === e) return 1;
  const kurz = Math.min(q.length, e.length), lang = Math.max(q.length, e.length);
  if (kurz >= 4 && kurz >= 0.6 * lang && (e.startsWith(q) || q.startsWith(e))) return 0.8;
  if (!deutsch) return 0;
  if (e.length >= 4 && q.length > e.length && q.endsWith(e)) return 0.7;
  if (q.length >= 5 && e.length > q.length && e.endsWith(q)) return 0.6;
  return 0;
}

/**
 * Sucht im Katalog: stark heißt, ein Wort passt zum Namen, Titel oder einem Preset (nicht nur zur Beschreibung), und
 * wenn die Anfrage eine Art nennt („sound“, „icon“), ist es ein Asset dieser Art.
 */
export function suchen(eintraege: Eintrag[], text: string, art?: Art, max = 8): Treffer[] {
  const anfrage = [...new Set(woerter(text))], genannt = artenIn(text);
  if (!anfrage.length) return [];
  const ergebnis: (Treffer & { punkte: number })[] = [];
  for (const e of eintraege) {
    if (art && e.art !== art) continue;
    let punkte = 0, stark = false;
    const gefunden: string[] = [];
    const passendeArt = !genannt.size || genannt.has(e.art);
    for (const q of anfrage) {
      let s = 0, w = 0;
      for (const [x, de] of e.stark) s = Math.max(s, passt(q, x, de));
      for (const [x, de] of e.schwach) w = Math.max(w, passt(q, x, de));
      if (s >= 0.6 && passendeArt) { punkte += 3 * s; stark = true; gefunden.push(q); }
      else if (s >= 0.6 || w >= 0.8) { punkte += Math.max(s, w); gefunden.push(q); }
    }
    if (!punkte) continue;
    const r = e.roh;
    ergebnis.push({ kind: e.art, id: r.id, function: r.function, title: r.title ?? {}, category: r.category, gallery: r.gallery,
      files: r.files ?? (r.file ? { wav: r.file } : undefined), presets: Object.entries(r.presets ?? {}).map(([name, p]) => ({ name, label: p.label })),
      strong: stark, matched: gefunden, punkte });
  }
  return ergebnis.sort((a, b) => Number(b.strong) - Number(a.strong) || b.punkte - a.punkte).slice(0, max).map(({ punkte: _, ...t }) => t);
}

let zwischen: { url: string; commit: string; eintraege: Eintrag[]; zeit: number } | null = null;
/** Für Tests: vergisst den geladenen Katalog. */
export function katalogVergessen() { zwischen = null; }

/** Der Katalog aus Formwerks Vorschau, neu geladen nach jedem Merge (anderer Commit) oder nach zehn Minuten. */
export async function katalogLaden(db: Db): Promise<{ verfuegbar: true; url: string; eintraege: Eintrag[] } | { verfuegbar: false; grund: string }> {
  const p = (await db.query("select vorschau from projekte where slug = $1 and not gesperrt and not entwurf", [KATALOG_PROJEKT])).rows[0];
  const basis = vorschauUrl(KATALOG_PROJEKT), v = p?.vorschau;
  if (!p || !basis || !v?.ok_commit || v.hat_vorschau === false) return { verfuegbar: false, grund: `the ${KATALOG_PROJEKT} catalogue is not published yet` };
  const url = `${basis}catalog.json`;
  if (zwischen && zwischen.url === url && zwischen.commit === v.ok_commit && Date.now() - zwischen.zeit < NEU_LADEN_MS) return { verfuegbar: true, url, eintraege: zwischen.eintraege };
  try {
    const k = (await quelle.holen(url)) as Record<string, unknown>;
    const eintraege = Object.entries(ARTEN).flatMap(([feld, art]) =>
      (Array.isArray(k?.[feld]) ? (k[feld] as Roh[]) : []).filter((r) => r && typeof r.id === "string").map((r) => eintragAus(art, r)));
    if (!eintraege.length) throw new Error("no assets in it");
    zwischen = { url, commit: v.ok_commit, eintraege, zeit: Date.now() };
    return { verfuegbar: true, url, eintraege };
  } catch (e) {
    // Lieber den letzten Stand als gar keinen
    if (zwischen && zwischen.url === url) return { verfuegbar: true, url, eintraege: zwischen.eintraege };
    return { verfuegbar: false, grund: `the ${KATALOG_PROJEKT} catalogue could not be loaded (${(e as Error).message})` };
  }
}

/** Die Assets eines catalog.json (alle Arten) mit ihrem Ordner in Formwerk (src/<ordner>/…). */
export function katalogAssets(k: unknown): { id: string; ordner: string }[] {
  const o = (k ?? {}) as Record<string, unknown>;
  return Object.keys(ARTEN).flatMap((feld) => (Array.isArray(o[feld]) ? (o[feld] as Roh[]) : [])
    .filter((r) => r && typeof r.id === "string").map((r) => ({ id: r.id, ordner: feld === "models" ? "assets" : feld })));
}

/**
 * Sprungziel in der Vorschau einer Formwerk-Einreichung (neVs 09.11.): „ausprobieren“ soll das Asset der Einreichung
 * zeigen, nicht die Startseite der Galerie (die öffnet ein Asset über #<id>). Eine Datei im Patch passt zu einem Asset,
 * wenn sie so heißt und im Ordner seiner Art liegt (src/assets/<kategorie>/<id>.ts) oder unter tests/ (Referenzbild);
 * Registerdateien wie src/surfaces/list.ts treffen so kein gleichnamiges Icon. Reihenfolge: ein neues Asset mit eigener
 * Datei, sonst irgendein neues (etwa ein Icon-Satz in einer Datei), sonst ein geändertes. Es gilt nur, was im Katalog
 * der Einreichungs-Vorschau steht. Leer: keines gefunden (dann die Startseite).
 */
export function zielWaehlen(patch: string, vorschau: { id: string; ordner: string }[], idsMain: Set<string>): string {
  const pfade = [...patch.matchAll(/^\+\+\+ b\/(\S+)$/gm)].map((m) => m[1]);
  const passt = (a: { id: string; ordner: string }) => pfade.some((p) => {
    const teile = p.split("/");
    return teile.at(-1)!.replace(/\.[^.]+$/, "") === a.id && (teile[0] === "tests" || (teile[0] === "src" && teile[1] === a.ordner));
  });
  const ziel = vorschau.find((a) => !idsMain.has(a.id) && passt(a)) ?? vorschau.find((a) => !idsMain.has(a.id)) ?? vorschau.find(passt);
  return ziel ? `#${ziel.id}` : "";
}

/**
 * Adresse der Vorschau einer Einreichung zum Ausprobieren; bei Formwerk mit dem Asset als Sprungziel. Das Ziel wird
 * einmal ermittelt und steht dann an der Einreichung (vorschau.ziel); ließ sich der Katalog nicht laden, beim nächsten
 * Mal wieder.
 */
export async function einreichungsVorschau(db: Db, slug: string, einreichungId: number): Promise<string> {
  const url = einreichungsVorschauUrl(einreichungId);
  if (!url || slug !== KATALOG_PROJEKT) return url;
  // sprungziel (ab 09.11. abends; der erste Stand unter „ziel“ verwechselte Registerdateien mit Assets und wird ersetzt)
  const e = (await db.query("select patch, vorschau->>'sprungziel' as ziel from einreichungen where id = $1", [einreichungId])).rows[0];
  if (!e) return url;
  if (typeof e.ziel === "string") return url + e.ziel;
  let vorschau: { id: string; ordner: string }[];
  try { vorschau = katalogAssets(await quelle.holen(`${url}catalog.json`)); } catch { return url; }
  const main = await katalogLaden(db);
  const ziel = zielWaehlen(String(e.patch ?? ""), vorschau, new Set(main.verfuegbar ? main.eintraege.map((x) => x.roh.id) : []));
  await db.query("update einreichungen set vorschau = (vorschau - 'ziel') || jsonb_build_object('sprungziel', $2::text) where id = $1", [einreichungId, ziel]);
  return url + ziel;
}

/** Werkzeug find_assets: alle Treffer, starke zuerst. */
export async function assetsFinden(db: Db, text: string, art?: Art) {
  if (woerter(text).length === 0) throw new Abgelehnt("query: say what the asset is (\"wooden crate with a lid\", \"warning icon\", \"door creak\")");
  const k = await katalogLaden(db);
  if (!k.verfuegbar) return { catalog_available: false, reason: k.grund, matches: [] };
  const treffer = suchen(k.eintraege, text, art);
  return { catalog_available: true, catalog: k.url, matches: treffer,
    hint: treffer.length ? "Every asset is code (MIT) with CC0 output: import the function or download the files; set its parameters or a preset to fit. Missing a parameter or a variant? Ask Formworks for it with request_work."
      : "Nothing in the catalogue fits these words. Try other words (English or German), or ask Formworks for it with request_work." };
}

export type Pruefung = { treffer: string[] | null; begruendung: string | null };

/**
 * Prüfung vor einer Generierung oder einer Anfrage an Formwerk: Gibt es starke Treffer und keine Begründung, wird
 * abgelehnt und die Treffer genannt. Ohne veröffentlichten Katalog geht die Anfrage durch (treffer null = ungeprüft).
 */
export async function vorGenerierung(db: Db, text: string, begruendung: string | undefined, wofuer: "brief" | "request"): Promise<Pruefung> {
  const grund = begruendung?.trim() || null;
  if (grund && (grund.length < BEGRUENDUNG.min || grund.length > BEGRUENDUNG.max))
    throw new Abgelehnt(`catalog_checked: ${BEGRUENDUNG.min} to ${BEGRUENDUNG.max} characters on why none of the catalogue's assets fits`);
  const k = await katalogLaden(db);
  if (!k.verfuegbar) return { treffer: null, begruendung: grund };
  const treffer = suchen(k.eintraege, text).filter((t) => t.strong).slice(0, 5);
  if (treffer.length && !grund) {
    const liste = treffer.map((t) => `- ${t.function ?? t.id} (${t.kind}${t.category ? ", " + t.category : ""}): ${t.title.en ?? t.title.de ?? t.id}${t.presets.length ? `; presets ${t.presets.map((p) => p.name).join(", ")}` : ""}${t.gallery ? ` · ${t.gallery}` : ""}`).join("\n");
    throw new Abgelehnt(`Formworks already has assets that may fit:\n${liste}\n` +
      (wofuer === "brief"
        ? "Use one of them instead of spending credits (find_assets shows the code and the files; parameters and presets change it), or call again with catalog_checked: why none of them fits."
        : "If one is close, ask Formworks for a parameter or a preset of it instead of a new asset, or call again with catalog_checked: why none of them fits."));
  }
  return { treffer: treffer.map((t) => `${t.kind}:${t.id}`), begruendung: grund };
}
