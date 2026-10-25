// Statusboard: lesende Projektseiten ohne Login.
//   /projekte                         Übersicht aller Projekte
//   /projekte/<slug>                  Projektseite (aktualisiert sich alle 15 s)
//   /projekte/<slug>/kennzahlen.json  dieselben Werte wie das MCP-Werkzeug `kennzahlen`
// Nachweis-Version: serverseitig erzeugtes HTML; die spätere Website (M6) nutzt dieselben Daten.
import type { ServerResponse } from "node:http";
import type { Db } from "./db.ts";
import { kennzahlen } from "./kennzahlen.ts";
import { ketteVerifizieren } from "./ereignisse.ts";
import { Abgelehnt, projektPerSlug } from "./aufgaben.ts";

const esc = (w: unknown) => String(w ?? "").replace(/[&<>"']/g, (z) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[z]!);
const zahl = (n: number) => new Intl.NumberFormat("de-DE").format(n);
const datum = (d: Date | string | null) => (d ? new Intl.DateTimeFormat("de-DE", { dateStyle: "long", timeZone: "Europe/Berlin" }).format(new Date(d)) : "–");
const zeit = (d: Date | string) => new Intl.DateTimeFormat("de-DE", { dateStyle: "short", timeStyle: "short", timeZone: "Europe/Berlin" }).format(new Date(d));

const ZUSTAND: Record<string, string> = {
  offen: "offen", reserviert: "reserviert", in_arbeit: "in Arbeit", eingereicht: "eingereicht", pruefung: "in Prüfung",
  geprueft_ok: "geprüft ✓", geprueft_fehler: "Prüfung ✗", angenommen: "angenommen", gemerged: "übernommen", geschlossen: "geschlossen",
};
const ART: Record<string, string> = {
  vision_veroeffentlicht: "Vision veröffentlicht", mitstreiter_beigetreten: "Mitstreiter beigetreten", aufgabe_angelegt: "Aufgabe angelegt",
  aufgabe_reserviert: "Aufgabe reserviert", arbeit_begonnen: "Arbeit begonnen", fortschritt: "Fortschritt", abgebrochen: "zurückgegeben",
  reservierung_abgelaufen: "Reservierung abgelaufen", ergebnis_eingereicht: "Ergebnis eingereicht", einreichung_abgewiesen: "Einreichung abgewiesen",
  pruefung_gestartet: "Prüfung gestartet", pruefung_beendet: "Prüfung beendet", zurueck_in_pool: "zurück in den Pool", bewertet: "bewertet",
  gemerged: "übernommen", rolle_gesetzt: "Rolle gesetzt", repo_gesetzt: "Repo gesetzt",
};

const STIL = `
:root{--bg:#f6f7f8;--karte:#fff;--text:#1b2126;--leise:#5d6870;--linie:#dde2e6;--akzent:#c26a12;--gut:#2f7d4f;--schlecht:#b3412e;--balken:#e8ecef}
@media (prefers-color-scheme:dark){:root{--bg:#121518;--karte:#1b2024;--text:#e6eaed;--leise:#97a2aa;--linie:#2c343a;--akzent:#e59a45;--gut:#5fbf86;--schlecht:#e07a66;--balken:#2a3238}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:15px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif}
main{max-width:64rem;margin:0 auto;padding:1.5rem 1rem 3rem}a{color:var(--akzent)}h1{font-size:1.6rem;margin:.2rem 0}h2{font-size:1.05rem;margin:0 0 .6rem}
.leise{color:var(--leise)}.raster{display:grid;gap:.8rem;grid-template-columns:repeat(auto-fit,minmax(9.5rem,1fr));margin:1rem 0}
.karte{background:var(--karte);border:1px solid var(--linie);border-radius:.6rem;padding:.9rem 1rem;overflow-x:auto;min-width:0}
.wert{font-size:1.6rem;font-weight:650;font-variant-numeric:tabular-nums}.etikett{font-size:.8rem;color:var(--leise);text-transform:uppercase;letter-spacing:.03em}
.balken{height:.5rem;background:var(--balken);border-radius:1rem;overflow:hidden;margin:.4rem 0}.balken i{display:block;height:100%;background:var(--akzent)}
table{width:100%;border-collapse:collapse;font-variant-numeric:tabular-nums}td,th{text-align:left;padding:.35rem .4rem;border-bottom:1px solid var(--linie);vertical-align:top}th{font-size:.8rem;color:var(--leise);font-weight:600}
.gut{color:var(--gut)}.schlecht{color:var(--schlecht)}code{font-size:.85em}details{margin-top:.5rem}.zwei{display:grid;gap:.8rem;grid-template-columns:repeat(auto-fit,minmax(20rem,1fr))}
`;

function seite(titel: string, inhalt: string, aktualisieren = false) {
  return `<!doctype html><html lang="de"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>${esc(titel)}</title><style>${STIL}</style>${aktualisieren ? '<meta http-equiv="refresh" content="15">' : ""}</head><body><main>${inhalt}</main></body></html>`;
}

function senden(res: ServerResponse, status: number, html: string) {
  res.writeHead(status, { "content-type": "text/html; charset=utf-8", "cache-control": "no-store" });
  res.end(html);
}

export async function boardBehandeln(db: Db, res: ServerResponse, url: URL): Promise<boolean> {
  if (url.pathname === "/projekte" || url.pathname === "/") {
    const liste = (await db.query("select slug from projekte order by id")).rows;
    const karten = await Promise.all(liste.map(async ({ slug }) => {
      const k = await kennzahlen(db, slug);
      return `<a class="karte" href="/projekte/${esc(slug)}" style="text-decoration:none;color:inherit">
        <div class="etikett">${esc(k.engine)}</div><h2>${esc(k.name)}</h2><p class="leise">${esc(k.vision_kurz)}</p>
        <div class="balken"><i style="width:${k.fortschritt_prozent}%"></i></div>
        <div class="leise">${k.fortschritt_prozent} % · ${k.agenten_aktiv} Agenten aktiv · ${k.aufgaben_offen} offene Aufgaben</div></a>`;
    }));
    senden(res, 200, seite("swarvex – Projekte", `<h1>Projekte</h1><p class="leise">Statusboard von swarvex</p><div class="zwei">${karten.join("")}</div>`, true));
    return true;
  }
  const m = /^\/projekte\/([a-z0-9-]+)(\/kennzahlen\.json)?$/.exec(url.pathname);
  if (!m) return false;
  try { await projektPerSlug(db, m[1]); }
  catch (e) { if (e instanceof Abgelehnt) { senden(res, 404, seite("Nicht gefunden", "<h1>Projekt nicht gefunden</h1>")); return true; } throw e; }
  const k = await kennzahlen(db, m[1]);
  if (m[2]) {
    res.writeHead(200, { "content-type": "application/json", "cache-control": "no-store" });
    res.end(JSON.stringify(k));
    return true;
  }
  const p = (await db.query("select * from projekte where slug = $1", [m[1]])).rows[0];
  const aufgaben = (await db.query(
    `select a.id, a.titel, a.stufe, a.zustand, m.titel as meilenstein,
            coalesce(
              (select 'arbeitet: ' || m2.login from leases l join agenten ag on ag.id = l.agent_id join menschen m2 on m2.id = ag.mensch_id
                where l.aufgabe_id = a.id and l.beendet is null and l.laeuft_ab > now() limit 1),
              (select 'Beitrag von ' || m3.login from einreichungen e join agenten ag on ag.id = e.agent_id join menschen m3 on m3.id = ag.mensch_id
                where e.aufgabe_id = a.id and a.zustand in ('angenommen', 'gemerged') order by e.id desc limit 1)) as agent
     from aufgaben a left join meilensteine m on m.id = a.meilenstein_id where a.projekt_id = $1 order by a.id desc limit 30`, [p.id])).rows;
  const ereignisse = (await db.query("select seq, zeit, akteur, art, bezug from ereignisse where projekt_id = $1 order by seq desc limit 12", [p.id])).rows;
  const kette = await ketteVerifizieren(db);
  // Akteure lesbar machen: agent:2 → „swarvex-Runner (…) · mitstreiter-test“, mensch:1 → „molty“.
  const namen = new Map<string, string>();
  for (const z of (await db.query("select 'agent:' || a.id as k, a.name || ' · ' || m.login as v from agenten a join menschen m on m.id = a.mensch_id union all select 'mensch:' || id, login from menschen")).rows) namen.set(z.k, z.v);
  namen.set("system", "Plattform"); namen.set("pruefstrasse", "Prüfstraße");
  const meilen = k.meilensteine.map((ms) => `<div>${esc(ms.titel)}<div class="balken"><i style="width:${ms.fortschritt_prozent}%"></i></div><span class="leise">${ms.fortschritt_prozent} %</span></div>`).join("");
  const nachArt = Object.entries(k.agenten_aktiv_nach_art).map(([art, n]) => `${n} ${esc(art)}`).join(", ");
  const inhalt = `
<p class="leise"><a href="/projekte">Projekte</a> / ${esc(p.slug)}</p>
<h1>${esc(k.name)}</h1>
<p>${esc(k.vision_kurz)}</p>
<p class="leise">Vision veröffentlicht am ${datum(k.vision_veroeffentlicht)} · Ziel: ${esc(k.ziel)}</p>
<details><summary>Vision im Original</summary><pre style="white-space:pre-wrap;font:inherit">${esc(p.vision_md)}</pre></details>
<div class="raster">
  <div class="karte"><div class="etikett">Fortschritt</div><div class="wert">${k.fortschritt_prozent} %</div><div class="balken"><i style="width:${k.fortschritt_prozent}%"></i></div></div>
  <div class="karte"><div class="etikett">Agenten aktiv</div><div class="wert">${k.agenten_aktiv}</div><div class="leise">${nachArt || "gerade niemand"}</div></div>
  <div class="karte"><div class="etikett">Beteiligte</div><div class="wert">${k.mitstreiter_menschen}</div><div class="leise">Menschen · ${k.mitstreiter_agenten} Agenten</div></div>
  <div class="karte"><div class="etikett">Offene Aufgaben</div><div class="wert">${k.aufgaben_offen}</div></div>
  <div class="karte"><div class="etikett">Gespendet diesen Monat</div><div class="wert">${zahl(Number(k.spenden_monat.tokens))}</div><div class="leise">Tokens · ${zahl(Number(k.spenden_monat.anfragen))} Anfragen · ${zahl(Number(k.spenden_monat.sekunden))} s</div></div>
  <div class="karte"><div class="etikett">Lead-Modell</div><div style="font-weight:600">${esc(k.lead_modell)}</div><div class="leise">Lead-Test: noch nicht vorhanden</div></div>
</div>
<div class="zwei">
  <div class="karte"><h2>Meilensteine</h2>${meilen || '<span class="leise">noch keine</span>'}<p class="leise" style="font-size:.85rem">Berechnung: ${esc(k.fortschritt_berechnung)}</p></div>
  <div class="karte"><h2>Steckbrief</h2><table>
    <tr><th>Engine</th><td>${esc(k.engine)}</td></tr><tr><th>Lizenz</th><td>${esc(k.lizenz)}</td></tr>
    <tr><th>Zuletzt</th><td>${k.zuletzt ? `${esc(ART[k.zuletzt.art] ?? k.zuletzt.art)} (${esc(k.zuletzt.bezug)}), ${zeit(k.zuletzt.zeit)}` : "–"}</td></tr>
    <tr><th>Protokoll</th><td class="${kette.ok ? "gut" : "schlecht"}">${kette.ok ? "Kette geprüft ✓" : "Kette BESCHÄDIGT"} · ${kette.anzahl} Einträge<br><code>${esc(kette.kopf.slice(0, 16))}…</code></td></tr>
  </table></div>
</div>
<div class="karte" style="margin-top:.8rem"><h2>Aufgaben</h2><table><tr><th>#</th><th>Aufgabe</th><th>Stufe</th><th>Zustand</th><th>Agent</th></tr>
${aufgaben.map((a) => `<tr><td>${a.id}</td><td>${esc(a.titel)}<div class="leise">${esc(a.meilenstein ?? "")}</div></td><td>${esc(a.stufe)}</td><td>${esc(ZUSTAND[a.zustand] ?? a.zustand)}</td><td class="leise">${esc(a.agent ?? "")}</td></tr>`).join("") || '<tr><td colspan="5" class="leise">noch keine Aufgaben</td></tr>'}
</table></div>
<div class="karte" style="margin-top:.8rem"><h2>Protokoll (neueste zuerst)</h2><table><tr><th>#</th><th>Zeit</th><th>Wer</th><th>Was</th><th>Bezug</th></tr>
${ereignisse.map((e) => `<tr><td>${e.seq}</td><td class="leise">${zeit(e.zeit)}</td><td>${esc(namen.get(e.akteur) ?? e.akteur)}</td><td>${esc(ART[e.art] ?? e.art)}</td><td class="leise">${esc(e.bezug)}</td></tr>`).join("")}
</table></div>
<p class="leise" style="font-size:.85rem">Dieselben Werte erhalten Lead und Agenten über das MCP-Werkzeug <code>kennzahlen</code> · <a href="/projekte/${esc(p.slug)}/kennzahlen.json">JSON</a> · aktualisiert sich alle 15 s</p>`;
  senden(res, 200, seite(`${k.name} – Statusboard`, inhalt, true));
  return true;
}
