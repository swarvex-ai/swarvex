// Schwarm-Bilanz je Projekt und Meilenstein, nur aus Messungen: Wie lange hat der Schwarm gebraucht, wie viele Agenten
// haben gleichzeitig gearbeitet, und wie lange hätte ein einzelner Agent für dieselbe Arbeit gebraucht?
// Keine geschätzten Menschenstunden und kein Vergleich mit menschlichen Teams (Entscheidung neVs, 02.11.2026): dafür gibt es
// keine belastbare Messung. Ein Vergleich mit einem früheren Stand (z. B. Rend&Run, Solo-Bau) läuft über Aufgaben-Einheiten
// (Stufen-Gewichte wie beim Fortschritt), und die Einordnung dieses Stands muss öffentlich im Repo belegt sein.
// Methode in der Doku: „Warum swarvex“, Abschnitt „Wie wir den Vorteil messen“.
import type { Db } from "./db.ts";
import { STUFEN_GEWICHT } from "./aufgaben.ts";

type Zeile = {
  einreichung: number; aufgabe: number; stufe: string; meilenstein: string | null; mensch: number; agent: number; modell: string | null;
  sekunden: number; urteil: string | null; bewertet: Date | null; bewerter: number | null; pruefung: string | null; zustand: string;
};

const r1 = (x: number) => Math.round(x * 10) / 10;

/** Bilanz einer Gruppe von Einreichungen (ganzes Projekt oder ein Meilenstein). start: erste Reservierung der Gruppe. */
export function gruppenBilanz(zeilen: Zeile[], start: Date | null) {
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
      agentenstunden_je_tag: kalenderStunden > 0 ? r1(agentenStunden / (kalenderStunden / 24)) : null,
      einheiten_je_tag: kalenderStunden > 0 ? r1(einheiten / (kalenderStunden / 24)) : null,
    },
    ein_agent: { stunden: r1(einAgentStunden) },
    // > 1: der Schwarm war schneller, als ein einzelner Agent rund um die Uhr gewesen wäre (eine Untergrenze für den Vorteil)
    beschleunigung: kalenderStunden > 0 && einAgentStunden > 0 ? r1(einAgentStunden / kalenderStunden) : null,
    // jeder angenommene Beitrag von einem anderen Menschen bewertet als dem, der ihn eingereicht hat
    vier_augen_prozent: angenommen.length ? Math.round((100 * angenommen.filter((z) => z.bewerter !== null && z.bewerter !== z.mensch).length) / angenommen.length) : null,
    pruefstrasse_prozent: angenommen.length ? Math.round((100 * angenommen.filter((z) => z.pruefung === "ok").length) / angenommen.length) : null,
  };
}

export async function schwarmBilanz(db: Db, projektId: number) {
  const zeilen: Zeile[] = (await db.query(
    `select e.id::int as einreichung, a.id::int as aufgabe, a.stufe, m.titel as meilenstein, ag.mensch_id::int as mensch, e.agent_id::int as agent, e.modell,
       coalesce((e.verbrauch->>'sekunden')::numeric, (e.verbrauch->>'seconds')::numeric, 0)::float as sekunden,
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
  const gesamt = gruppenBilanz(zeilen, gesamtStart);
  // Vergleich mit einem früheren Stand über den Durchsatz in Einheiten je Kalendertag (nur mit öffentlichem Beleg der Einordnung)
  const vergleich = (await db.query("select vergleich from projekte where id = $1", [projektId])).rows[0]?.vergleich ?? null;
  if (vergleich?.einheiten && vergleich?.kalender_tage && vergleich?.beleg) {
    vergleich.einheiten_je_tag = r1(Number(vergleich.einheiten) / Number(vergleich.kalender_tage));
    if (gesamt?.schwarm.einheiten_je_tag) vergleich.durchsatz_faktor = r1(gesamt.schwarm.einheiten_je_tag / vergleich.einheiten_je_tag);
  }
  return {
    gesamt,
    meilensteine: titel.map((t) => ({ titel: t, abgeschlossen: (offen.get(t) ?? 0) === 0, ...gruppenBilanz(zeilen.filter((z) => z.meilenstein === t), startVon(t)) }))
      .filter((m) => m.aufgaben_fertig),
    vergleich,
  };
}
