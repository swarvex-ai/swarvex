// „Das hat dein Agent gebaut“ (Kontoseite): je Einreichung, was entstanden ist (Dateien, Zeilen, Art), wie es ausging
// (Prüfung, Bewertung mit Begründung, Merge-Commit), dazu Summen und ein Vergleich der eigenen Agenten und Modelle.
// Vorschauen von Bildern und Audio holt der Server aus dem öffentlichen Projekt-Repo und reicht sie durch,
// damit der Browser der Besucher nicht direkt bei GitHub anfragt.
import type { ServerResponse } from "node:http";
import type { Db } from "./db.ts";
import { Abgelehnt } from "./aufgaben.ts";

export type Datei = { pfad: string; plus: number; minus: number; sorte: Sorte; lfs: boolean; binaer: boolean };
export type Sorte = "code" | "test" | "bild" | "audio" | "video" | "3d" | "text" | "daten" | "sonstiges";

const ENDUNGEN: [RegExp, Sorte][] = [
  [/\.(png|jpe?g|webp|gif|svg|avif|bmp|tga|psd|kra|aseprite)$/i, "bild"],
  [/\.(ogg|wav|mp3|flac|opus|m4a)$/i, "audio"],
  [/\.(mp4|webm|mov|mkv)$/i, "video"],
  [/\.(glb|gltf|fbx|obj|blend|stl|dae|usdz?)$/i, "3d"],
  [/(^|\/)(tests?|spec)\/|_test\.\w+$|\.test\.\w+$|\.spec\.\w+$|_spec\.\w+$/i, "test"],
  [/\.(md|txt|po|pot|rst|adoc)$/i, "text"],
  [/\.(json|ya?ml|toml|csv|tsv|xml|tres|tscn|ini|cfg)$/i, "daten"],
  [/\.(gd|cs|ts|tsx|js|jsx|mjs|svelte|vue|py|go|rs|c|cc|cpp|h|hpp|java|kt|swift|rb|php|lua|sh|gdshader|shader|glsl|hlsl|wgsl|sql|html|css|scss)$/i, "code"],
];
export const sorteVon = (pfad: string): Sorte => ENDUNGEN.find(([r]) => r.test(pfad))?.[1] ?? "sonstiges";

/** Dateien eines Patches mit hinzugefügten und entfernten Zeilen; Binärdateien und Git-LFS-Zeiger markiert. */
export function patchDateien(patch: string): Datei[] {
  const dateien: Datei[] = [];
  let d: Datei | null = null;
  for (const zeile of patch.split("\n")) {
    const kopf = /^diff --git a\/(.+?) b\/(.+)$/.exec(zeile);
    if (kopf) { d = { pfad: kopf[2], plus: 0, minus: 0, sorte: sorteVon(kopf[2]), lfs: false, binaer: false }; dateien.push(d); continue; }
    if (!d) continue;
    if (zeile.startsWith("Binary files ") || zeile === "GIT binary patch") d.binaer = true;
    else if (zeile.startsWith("+version https://git-lfs.github.com/spec/")) d.lfs = true;
    else if (zeile.startsWith("+") && !zeile.startsWith("+++ ")) d.plus++;
    else if (zeile.startsWith("-") && !zeile.startsWith("--- ")) d.minus++;
  }
  // LFS-Zeiger sind drei Zeilen Text; gezählt wird die Datei, nicht der Zeiger
  for (const x of dateien) if (x.lfs) { x.plus = 0; x.minus = 0; }
  return dateien;
}

const VORSCHAU: Partial<Record<Sorte, boolean>> = { bild: true, audio: true };
const TYPEN: Record<string, string> = { png: "image/png", jpg: "image/jpeg", jpeg: "image/jpeg", webp: "image/webp", gif: "image/gif", avif: "image/avif",
  svg: "image/svg+xml", ogg: "audio/ogg", opus: "audio/ogg", wav: "audio/wav", mp3: "audio/mpeg", flac: "audio/flac", m4a: "audio/mp4" };
const typVon = (pfad: string) => TYPEN[pfad.split(".").pop()!.toLowerCase()];

/** owner/repo eines öffentlichen GitHub-Repos, sonst null. */
function github(repoUrl: string | null) {
  const m = /^https:\/\/github\.com\/([\w.-]+)\/([\w.-]+?)(?:\.git)?\/?$/.exec(repoUrl ?? "");
  return m ? `${m[1]}/${m[2]}` : null;
}

export async function meineBeitraege(db: Db, menschId: number) {
  const zeilen = (await db.query(
    `select e.id::int as einreichung, a.id::int as aufgabe, a.titel, a.faehigkeiten, a.rolle, p.slug as projekt, p.name as projekt_name, p.repo_url,
       ag.name as agent, e.agent_programm, e.modell, e.eingereicht, a.zustand, e.patch, e.zusammenfassung,
       coalesce((e.verbrauch->>'tokens')::bigint, 0)::bigint as tokens,
       (select pr.ergebnis from pruefungen pr where pr.einreichung_id = e.id and pr.quelle = 'projekt' order by pr.id desc limit 1) as pruefung,
       (select b.urteil from bewertungen b where b.einreichung_id = e.id order by b.id desc limit 1) as urteil,
       (select b.begruendung from bewertungen b where b.einreichung_id = e.id order by b.id desc limit 1) as begruendung,
       (select x.daten->>'commit' from ereignisse x where x.art = 'gemerged' and x.bezug = 'aufgabe:' || a.id and x.daten->>'einreichung' = e.id::text
          order by x.seq desc limit 1) as commit
     from einreichungen e join agenten ag on ag.id = e.agent_id join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id
     where ag.mensch_id = $1 order by e.eingereicht desc limit 500`, [menschId])).rows;

  const beitraege = zeilen.map(({ patch, repo_url, ...b }) => {
    const dateien = patchDateien(patch);
    const gh = b.commit ? github(repo_url) : null;
    const sorten = [...new Set(dateien.map((d) => d.sorte))];
    return {
      ...b, tokens: Number(b.tokens),
      zusammenfassung: String(b.zusammenfassung ?? "").slice(0, 600),
      plus: dateien.reduce((n, d) => n + d.plus, 0), minus: dateien.reduce((n, d) => n + d.minus, 0),
      sorten: sorten.length ? sorten : ["sonstiges"],
      dateien: dateien.slice(0, 40).map((d) => ({ ...d, vorschau: !!(gh && VORSCHAU[d.sorte] && typVon(d.pfad)) })),
      dateien_gesamt: dateien.length,
      commit_url: gh ? `https://github.com/${gh}/commit/${b.commit}` : null,
    };
  });

  const angenommen = beitraege.filter((b) => b.urteil === "annehmen");
  const zaehle = (sorte: Sorte) => angenommen.reduce((n, b) => n + b.dateien.filter((d: Datei) => d.sorte === sorte).length, 0);
  const summe = {
    einreichungen: beitraege.length,
    angenommen: angenommen.length,
    gemerged: beitraege.filter((b) => b.commit).length,
    projekte: new Set(angenommen.map((b) => b.projekt)).size,
    tokens: beitraege.reduce((n, b) => n + b.tokens, 0),
    // nur angenommene Beiträge zählen als „gebaut“
    zeilen_code: angenommen.reduce((n, b) => n + b.dateien.filter((d: Datei) => d.sorte === "code" || d.sorte === "test").reduce((m: number, d: Datei) => m + d.plus, 0), 0),
    bilder: zaehle("bild"), audio: zaehle("audio"), modelle_3d: zaehle("3d"), videos: zaehle("video"),
    texte: zaehle("text"), reviews: angenommen.filter((b) => (b.faehigkeiten as string[]).includes("review")).length,
  };

  // eigene Agenten und Modelle im Vergleich: was bringt welches Modell?
  const gruppen = new Map<string, { agent_programm: string; modell: string; einreichungen: number; angenommen: number; tokens: number }>();
  for (const b of beitraege) {
    const k = `${b.agent_programm ?? "?"}|${b.modell ?? "?"}`;
    const g = gruppen.get(k) ?? { agent_programm: b.agent_programm ?? "", modell: b.modell ?? "", einreichungen: 0, angenommen: 0, tokens: 0 };
    g.einreichungen++; g.tokens += b.tokens; if (b.urteil === "annehmen") g.angenommen++;
    gruppen.set(k, g);
  }
  const modelle = [...gruppen.values()].map((g) => ({ ...g, annahme_prozent: Math.round((100 * g.angenommen) / g.einreichungen),
    tokens_je_annahme: g.angenommen ? Math.round(g.tokens / g.angenommen) : null })).sort((x, y) => y.einreichungen - x.einreichungen);

  return { beitraege, summe, modelle };
}

/** Vorschau einer Datei aus einem eigenen, gemergten Beitrag: aus dem öffentlichen GitHub-Repo, höchstens 8 MB. */
export async function vorschauSenden(db: Db, menschId: number, einreichungId: number, pfad: string, res: ServerResponse) {
  const r = (await db.query(
    `select e.patch, p.repo_url,
       (select x.daten->>'commit' from ereignisse x where x.art = 'gemerged' and x.bezug = 'aufgabe:' || a.id and x.daten->>'einreichung' = e.id::text
          order by x.seq desc limit 1) as commit
     from einreichungen e join agenten ag on ag.id = e.agent_id join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id
     where e.id = $1 and ag.mensch_id = $2`, [einreichungId, menschId])).rows[0];
  if (!r) throw new Abgelehnt("not your contribution");
  const d = patchDateien(r.patch).find((x) => x.pfad === pfad);
  const typ = d && VORSCHAU[d.sorte] ? typVon(pfad) : undefined;
  const gh = r.commit ? github(r.repo_url) : null;
  if (!d || !typ || !gh || !/^[0-9a-f]{7,40}$/.test(r.commit)) throw new Abgelehnt("no preview for this file");
  const weg = pfad.split("/").map(encodeURIComponent).join("/");
  const quelle = d.lfs ? `https://media.githubusercontent.com/media/${gh}/${r.commit}/${weg}` : `https://raw.githubusercontent.com/${gh}/${r.commit}/${weg}`;
  const antwort = await fetch(quelle, { signal: AbortSignal.timeout(15_000), redirect: "follow" });
  const laenge = Number(antwort.headers.get("content-length") ?? 0);
  if (!antwort.ok || laenge > 8_000_000) throw new Abgelehnt(`preview not available (HTTP ${antwort.status})`);
  const daten = Buffer.from(await antwort.arrayBuffer());
  if (daten.length > 8_000_000) throw new Abgelehnt("preview too large");
  // SVG nur als Bild: ohne Skripte und ohne Einbettung in fremde Seiten
  res.writeHead(200, { "content-type": typ, "content-length": daten.length, "cache-control": "private, max-age=86400",
    "x-content-type-options": "nosniff", "content-security-policy": "default-src 'none'; style-src 'unsafe-inline'; sandbox" });
  res.end(daten);
}
