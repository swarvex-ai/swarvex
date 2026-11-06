// Schwarm-Bilanz je Projekt und Meilenstein, nur aus Messungen: Wie lange hat der Schwarm gebraucht, wie viele Agenten
// haben gleichzeitig gearbeitet, und wie lange hätte ein einzelner Agent für dieselbe Arbeit gebraucht?
// Keine geschätzten Menschenstunden und kein Vergleich mit menschlichen Teams (Entscheidung neVs, 02.11.2026): dafür gibt es
// keine belastbare Messung. Ein Vergleich mit einem früheren Stand (z. B. Rend&Run, Solo-Bau) läuft über Aufgaben-Einheiten
// (Stufen-Gewichte wie beim Fortschritt), und die Einordnung dieses Stands muss öffentlich im Repo belegt sein.
// Methode in der Doku: „Warum swarvex“, Abschnitt „Wie wir den Vorteil messen“.
import type { Db } from "./db.ts";
import { STUFEN_GEWICHT } from "./aufgaben.ts";
import { angebot } from "./fluss.ts";

type Zeile = {
  einreichung: number; aufgabe: number; stufe: string; meilenstein: string | null; mensch: number; agent: number; modell: string | null;
  sekunden: number; tokens: number; urteil: string | null; bewertet: Date | null; bewerter: number | null; pruefung: string | null; zustand: string;
};

/** Je Aufgabe: Regressionen und rote Merge-Züge (Qualität) sowie Aktionen des Leads (Last), für die Gruppen-Bilanz. */
type JeAufgabe = Map<number, { regressionen: number; zug_rot: number; lead_aktionen: number }>;
/** Was der Lead selbst erledigt hat (Stufe D, ohne Einreichung): zählt nicht als Schwarm-Arbeit, aber als Lead-Last. */
type LeadEigen = { meilenstein: string | null; stufe: string }[];

const r1 = (x: number) => Math.round(x * 10) / 10;
/** Werte je Kalendertag erst ab dieser Messdauer: kürzere Zeiten auf einen Tag hochzurechnen ergäbe Zahlen ohne Aussage. */
export const MIN_TAGE = 3;

/** Bilanz einer Gruppe von Einreichungen (ganzes Projekt oder ein Meilenstein). start: erste Reservierung der Gruppe. */
export function gruppenBilanz(zeilen: Zeile[], start: Date | null, jeAufgabe: JeAufgabe = new Map(), leadEigen: LeadEigen = []) {
  const fertig = new Map(zeilen.filter((z) => z.zustand === "angenommen" || z.zustand === "gemerged").map((z) => [z.aufgabe, z.stufe]));
  if (fertig.size === 0) return null;
  const angenommen = zeilen.filter((z) => z.urteil === "annehmen" && fertig.has(z.aufgabe));
  const agentenStunden = zeilen.reduce((n, z) => n + z.sekunden, 0) / 3600;
  // ein Agent allein: jede Arbeit an den erledigten Aufgaben, nacheinander, auch die Versuche vor der Annahme
  const einAgentStunden = zeilen.filter((z) => fertig.has(z.aufgabe)).reduce((n, z) => n + z.sekunden, 0) / 3600;
  const ende = angenommen.reduce<Date | null>((m, z) => (z.bewertet && (!m || z.bewertet > m) ? z.bewertet : m), null);
  const kalenderStunden = start && ende ? Math.max(0, (ende.getTime() - start.getTime()) / 3_600_000) : 0;
  const einheiten = [...fertig.values()].reduce((n, s) => n + (STUFEN_GEWICHT[s] ?? 1), 0);
  return {
    aufgaben_fertig: fertig.size,
    einheiten,
    schwarm: {
      kalender_stunden: r1(kalenderStunden),
      agenten_stunden: r1(agentenStunden),
      agenten: new Set(zeilen.map((z) => z.agent)).size,
      menschen: new Set(zeilen.map((z) => z.mensch)).size,
      modelle: new Set(zeilen.map((z) => z.modell).filter(Boolean)).size,
      // im Mittel gleichzeitig arbeitende Agenten; ein einzelner Agent kommt höchstens auf 1 (24 Agentenstunden je Tag)
      gleichzeitig: kalenderStunden > 0 ? r1(agentenStunden / kalenderStunden) : null,
      agentenstunden_je_tag: kalenderStunden >= MIN_TAGE * 24 ? r1(agentenStunden / (kalenderStunden / 24)) : null,
      einheiten_je_tag: kalenderStunden >= MIN_TAGE * 24 ? r1(einheiten / (kalenderStunden / 24)) : null,
    },
    ein_agent: { stunden: r1(einAgentStunden) },
    // > 1: der Schwarm war schneller, als ein einzelner Agent rund um die Uhr gewesen wäre (eine Untergrenze für den Vorteil)
    beschleunigung: kalenderStunden > 0 && einAgentStunden > 0 ? r1(einAgentStunden / kalenderStunden) : null,
    // jeder angenommene Beitrag von einem anderen Menschen bewertet als dem, der ihn eingereicht hat
    vier_augen_prozent: angenommen.length ? Math.round((100 * angenommen.filter((z) => z.bewerter !== null && z.bewerter !== z.mensch).length) / angenommen.length) : null,
    pruefstrasse_prozent: angenommen.length ? Math.round((100 * angenommen.filter((z) => z.pruefung === "ok").length) / angenommen.length) : null,
    // Geschwindigkeit nur mit gleicher Qualität: zurückgeschickte Bewertungen, Regressionen nach dem Merge, rote Merge-Züge
    qualitaet: qualitaetVon(zeilen, fertig, jeAufgabe),
    // Was eine fertige Einheit kostet: alle Tokens der Versuche an den fertigen Aufgaben (auch der verworfenen)
    kosten: { tokens_je_einheit: einheiten ? Math.round(zeilen.filter((z) => fertig.has(z.aufgabe)).reduce((n, z) => n + z.tokens, 0) / einheiten) : null },
    // Der Engpass der Koordination: Wie viel tut der Lead je Einheit des Schwarms, und wie viel baut er selbst?
    lead_last: {
      eigene_einheiten: leadEigen.reduce((n, l) => n + (STUFEN_GEWICHT[l.stufe] ?? 1), 0),
      aktionen_je_einheit: einheiten ? r1([...new Set(zeilen.map((z) => z.aufgabe))].reduce((n, a) => n + (jeAufgabe.get(a)?.lead_aktionen ?? 0), 0) / einheiten) : null,
    },
  };
}

function qualitaetVon(zeilen: Zeile[], fertig: Map<number, string>, jeAufgabe: JeAufgabe) {
  const bewertet = zeilen.filter((z) => z.urteil);
  const aufgaben = [...new Set(zeilen.map((z) => z.aufgabe))];
  return {
    nacharbeit_prozent: bewertet.length ? Math.round((100 * bewertet.filter((z) => z.urteil !== "annehmen").length) / bewertet.length) : null,
    regressionen: [...fertig.keys()].reduce((n, a) => n + (jeAufgabe.get(a)?.regressionen ?? 0), 0),
    zug_rot: aufgaben.reduce((n, a) => n + (jeAufgabe.get(a)?.zug_rot ?? 0), 0),
  };
}

/** Erfolg je Modell und Stufe (leistung_versuche): entschiedene Versuche, angenommen und wie die anderen ausgingen. */
async function leistung(db: Db, projektId: number) {
  return (await db.query(
    `select coalesce(modell, 'unknown') as modell, stufe, count(*)::int as versuche,
       count(*) filter (where ausgang = 'angenommen')::int as angenommen,
       count(*) filter (where ausgang = 'zurueck')::int as zurueck,
       count(*) filter (where ausgang in ('pruefung_rot', 'zug_rot'))::int as pruefung_rot,
       count(*) filter (where ausgang = 'abgebrochen')::int as abgebrochen
     from leistung_versuche where projekt_id = $1 and ausgang not in ('laeuft', 'offen')
     group by 1, 2 order by 1, 2`, [projektId])).rows
    .map((r) => ({ ...r, erfolg_prozent: r.versuche ? Math.round((100 * r.angenommen) / r.versuche) : null }));
}

const median = (xs: number[]) => {
  const s = xs.filter((x) => Number.isFinite(x) && x >= 0).sort((a, b) => a - b);
  return s.length ? r1(s.length % 2 ? s[(s.length - 1) / 2] : (s[s.length / 2 - 1] + s[s.length / 2]) / 2) : null;
};
const stunden = (von: Date | null, bis: Date | null) => (von && bis ? (bis.getTime() - von.getTime()) / 3_600_000 : NaN);

/** Wo die Zeit hingeht: Median je Stufe über die angenommenen Beiträge (gemessen, in Stunden). */
async function zeitfluss(db: Db, projektId: number) {
  const r = (await db.query(
    `select a.erstellt, (select min(l2.erstellt) from leases l2 where l2.aufgabe_id = a.id) as erste_reservierung,
       l.erstellt as reserviert, e.eingereicht, pr.erstellt as geprueft, b.erstellt as bewertet,
       (select min(x.zeit) from ereignisse x where x.bezug = 'aufgabe:' || a.id and x.art = 'gemerged') as gemerged
     from einreichungen e join aufgaben a on a.id = e.aufgabe_id join leases l on l.id = e.lease_id
     join lateral (select urteil, erstellt from bewertungen where einreichung_id = e.id order by id desc limit 1) b on b.urteil = 'annehmen'
     left join lateral (select erstellt from pruefungen where einreichung_id = e.id and erstellt <= b.erstellt order by id desc limit 1) pr on true
     where a.projekt_id = $1`, [projektId])).rows;
  if (!r.length) return null;
  return {
    warten_auf_agent: median(r.map((z) => stunden(z.erstellt, z.erste_reservierung))),
    bearbeitung: median(r.map((z) => stunden(z.reserviert, z.eingereicht))),
    pruefung: median(r.map((z) => stunden(z.eingereicht, z.geprueft))),
    bewertung: median(r.map((z) => stunden(z.geprueft ?? z.eingereicht, z.bewertet))),
    merge: median(r.map((z) => stunden(z.bewertet, z.gemerged))),
  };
}

/** Wo es gerade bremst: Angebot, Prüfung und Bewertung jetzt; engpass benennt die Stufe, an der Arbeit am längsten steht. */
async function jetzt(db: Db, projektId: number, fluss: Awaited<ReturnType<typeof zeitfluss>>) {
  const a = await angebot(db, projektId);
  const w = (await db.query(
    `select count(*) filter (where a.zustand in ('eingereicht', 'pruefung'))::int as warten_auf_pruefung,
       round(max(extract(epoch from now() - e.eingereicht)) filter (where a.zustand in ('eingereicht', 'pruefung')) / 3600, 1) as aelteste_pruefung_h,
       count(*) filter (where a.zustand = 'geprueft_ok')::int as warten_auf_bewertung,
       round(max(extract(epoch from now() - coalesce(pr.erstellt, e.eingereicht))) filter (where a.zustand = 'geprueft_ok') / 3600, 1) as aelteste_bewertung_h,
       count(*) filter (where a.zustand = 'angenommen')::int as warten_auf_merge,
       round(max(extract(epoch from now() - b.erstellt)) filter (where a.zustand = 'angenommen') / 3600, 1) as aelteste_merge_h
     from aufgaben a join lateral (select id, eingereicht from einreichungen where aufgabe_id = a.id order by id desc limit 1) e on true
     left join lateral (select erstellt from pruefungen where einreichung_id = e.id order by id desc limit 1) pr on true
     left join lateral (select erstellt from bewertungen where einreichung_id = e.id order by id desc limit 1) b on true
     where a.projekt_id = $1 and a.zustand in ('eingereicht', 'pruefung', 'geprueft_ok', 'angenommen')`, [projektId])).rows[0];
  const n = (x: unknown) => (x === null || x === undefined ? null : Number(x));
  const bewertungH = n(w.aelteste_bewertung_h), pruefungH = n(w.aelteste_pruefung_h), mergeH = n(w.aelteste_merge_h);
  const engpass = w.warten_auf_bewertung > 0 && (bewertungH ?? 0) >= Math.max(2, fluss?.bearbeitung ?? 0) ? "bewertung"
    : w.warten_auf_pruefung > 0 && (pruefungH ?? 0) >= 1 ? "pruefung"
    : w.warten_auf_merge > 0 && (mergeH ?? 0) >= 1 ? "merge"
    : a.aktiv > 0 && a.frei < a.aktiv ? "nachschub"
    : a.frei === 0 && a.blockiert > 0 ? "abhaengigkeiten"
    : a.frei > 0 && a.aktiv === 0 ? "mitstreiter" : null;
  return { ...a, warten_auf_pruefung: w.warten_auf_pruefung, aelteste_pruefung_h: pruefungH, warten_auf_bewertung: w.warten_auf_bewertung,
    aelteste_bewertung_h: bewertungH, warten_auf_merge: w.warten_auf_merge, aelteste_merge_h: mergeH, engpass };
}

export async function schwarmBilanz(db: Db, projektId: number) {
  const zeilen: Zeile[] = (await db.query(
    `select e.id::int as einreichung, a.id::int as aufgabe, a.stufe, m.titel as meilenstein, ag.mensch_id::int as mensch, e.agent_id::int as agent, e.modell,
       coalesce((e.verbrauch->>'sekunden')::numeric, (e.verbrauch->>'seconds')::numeric, 0)::float as sekunden,
       coalesce((e.verbrauch->>'tokens')::numeric, 0)::float as tokens,
       b.urteil, b.erstellt as bewertet, ba.mensch_id::int as bewerter, a.zustand,
       (select pr.ergebnis from pruefungen pr where pr.einreichung_id = e.id and pr.quelle = 'projekt' order by pr.id desc limit 1) as pruefung
     from einreichungen e join aufgaben a on a.id = e.aufgabe_id join agenten ag on ag.id = e.agent_id
     left join meilensteine m on m.id = a.meilenstein_id
     left join lateral (select urteil, erstellt, agent_id from bewertungen where einreichung_id = e.id order by id desc limit 1) b on true
     left join agenten ba on ba.id = b.agent_id
     where a.projekt_id = $1`, [projektId])).rows;
  const starts = (await db.query(
    `select coalesce(m.titel, '') as meilenstein, min(l.erstellt) as start from leases l join aufgaben a on a.id = l.aufgabe_id
     left join meilensteine m on m.id = a.meilenstein_id where a.projekt_id = $1 group by 1`, [projektId])).rows;
  const startVon = (titel: string | null) => starts.find((s) => s.meilenstein === (titel ?? ""))?.start ?? null;
  const gesamtStart = starts.reduce<Date | null>((m, s) => (s.start && (!m || s.start < m) ? s.start : m), null);
  const titel = [...new Set(zeilen.map((z) => z.meilenstein).filter((t): t is string => !!t))];
  const offen = new Map((await db.query(
    `select m.titel, count(*) filter (where a.zustand not in ('angenommen', 'gemerged', 'geschlossen'))::int as offen
     from aufgaben a join meilensteine m on m.id = a.meilenstein_id where a.projekt_id = $1 group by m.titel`, [projektId])).rows.map((r) => [r.titel, r.offen]));
  // Je Aufgabe: Regressionen, rote Merge-Züge und Aktionen des Leads (Bewerten, Anlegen, Schließen, selbst Erledigen)
  const jeAufgabe: JeAufgabe = new Map((await db.query(
    `select a.id::int as aufgabe,
       count(*) filter (where ev.art = 'regression_gemeldet')::int as regressionen,
       count(*) filter (where ev.art = 'merge_zug_rot')::int as zug_rot,
       count(*) filter (where ev.art in ('bewertet', 'aufgabe_angelegt', 'aufgabe_geschlossen', 'vom_lead_erledigt')
         and ev.akteur in (select 'agent:' || ag.id from agenten ag join projekt_rollen r on r.mensch_id = ag.mensch_id where r.projekt_id = $1 and r.rolle = 'lead'))::int as lead_aktionen
     from aufgaben a join ereignisse ev on ev.bezug = 'aufgabe:' || a.id where a.projekt_id = $1 group by a.id`, [projektId])).rows
    .map((r) => [r.aufgabe, { regressionen: r.regressionen, zug_rot: r.zug_rot, lead_aktionen: r.lead_aktionen }]));
  const leadEigen: LeadEigen = (await db.query(
    `select m.titel as meilenstein, a.stufe from aufgaben a left join meilensteine m on m.id = a.meilenstein_id
     where a.projekt_id = $1 and exists (select 1 from ereignisse ev where ev.bezug = 'aufgabe:' || a.id and ev.art = 'vom_lead_erledigt')`, [projektId])).rows;
  const gesamt = gruppenBilanz(zeilen, gesamtStart, jeAufgabe, leadEigen);
  // Vergleich mit einem früheren Stand über den Durchsatz in Einheiten je Kalendertag (nur mit öffentlichem Beleg der Einordnung)
  const vergleich = (await db.query("select vergleich from projekte where id = $1", [projektId])).rows[0]?.vergleich ?? null;
  if (vergleich?.einheiten && vergleich?.kalender_tage && vergleich?.beleg) {
    vergleich.einheiten_je_tag = r1(Number(vergleich.einheiten) / Number(vergleich.kalender_tage));
    if (gesamt?.schwarm.einheiten_je_tag) vergleich.durchsatz_faktor = r1(gesamt.schwarm.einheiten_je_tag / vergleich.einheiten_je_tag);
  }
  const fluss = await zeitfluss(db, projektId);
  return {
    gesamt,
    zeitfluss: fluss,
    jetzt: await jetzt(db, projektId, fluss),
    meilensteine: titel.map((t) => ({ titel: t, abgeschlossen: (offen.get(t) ?? 0) === 0,
      ...gruppenBilanz(zeilen.filter((z) => z.meilenstein === t), startVon(t), jeAufgabe, leadEigen.filter((l) => l.meilenstein === t)) }))
      .filter((m) => m.aufgaben_fertig),
    leistung: await leistung(db, projektId),
    vergleich,
  };
}
