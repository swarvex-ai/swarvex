// Schwarm-Bilanz je Projekt und Meilenstein: Wie lange hat der Schwarm gebraucht (gemessen), wie lange hätte ein
// einzelner Agent gebraucht (aus Messungen berechnet), und was hätte ein menschliches Team gebraucht (geschätzt)?
// Methode öffentlich in der Doku („Warum swarvex“, Abschnitt „So rechnen wir“). Schwarm-Werte sind nur gemessen;
// Menschen-Werte sind immer als Schätzung gekennzeichnet: Schätzung des Leads je Aufgabe und COCOMO als Gegenprobe.
import type { Db } from "./db.ts";
import { patchDateien } from "./beitraege.ts";

/** Ein menschliches Vergleichsteam: so viele Leute mit so vielen produktiven Stunden je Woche. */
export const TEAM = { menschen: 4, stunden_je_woche: 30 };
/** COCOMO erst ab dieser Codegröße; darunter ist das Modell nicht aussagekräftig. */
const COCOMO_AB_ZEILEN = 500;

type Zeile = {
  einreichung: number; aufgabe: number; meilenstein: string | null; mensch: number; agent: number; modell: string | null;
  sekunden: number; urteil: string | null; bewertet: Date | null; bewerter: number | null; pruefung: string | null;
  zustand: string; menschen_stunden: number | null; patch: string | null;
};

const r1 = (x: number) => Math.round(x * 10) / 10;

/** COCOMO (Basismodell, organische Projekte): Personenmonate, Monate Dauer und Teamgröße aus der Codegröße. */
export function cocomo(zeilen: number) {
  if (zeilen < COCOMO_AB_ZEILEN) return null;
  const kloc = zeilen / 1000;
  const personenmonate = 2.4 * kloc ** 1.05;
  const monate = 2.5 * personenmonate ** 0.38;
  return { zeilen, personenmonate: r1(personenmonate), monate: r1(monate), menschen: r1(personenmonate / monate) };
}

/** Bilanz einer Gruppe von Einreichungen (ganzes Projekt oder ein Meilenstein). start: erste Reservierung der Gruppe. */
export function gruppenBilanz(zeilen: Zeile[], start: Date | null) {
  const fertig = new Set(zeilen.filter((z) => z.zustand === "angenommen" || z.zustand === "gemerged").map((z) => z.aufgabe));
  if (fertig.size === 0) return null;
  const angenommen = zeilen.filter((z) => z.urteil === "annehmen" && fertig.has(z.aufgabe));
  // ein Agent allein: jede Arbeit an den erledigten Aufgaben, nacheinander, auch die Versuche vor der Annahme
  const einAgentStunden = zeilen.filter((z) => fertig.has(z.aufgabe)).reduce((n, z) => n + z.sekunden, 0) / 3600;
  const ende = angenommen.reduce<Date | null>((m, z) => (z.bewertet && (!m || z.bewertet > m) ? z.bewertet : m), null);
  const kalenderStunden = start && ende ? Math.max(0, (ende.getTime() - start.getTime()) / 3_600_000) : 0;
  const schaetzungen = new Map<number, number>();
  for (const z of zeilen) if (fertig.has(z.aufgabe) && z.menschen_stunden) schaetzungen.set(z.aufgabe, Number(z.menschen_stunden));
  const menschenStunden = [...schaetzungen.values()].reduce((n, h) => n + h, 0);
  const codeZeilen = angenommen.reduce((n, z) => n + patchDateien(z.patch ?? "").filter((d) => d.sorte === "code" || d.sorte === "test").reduce((m, d) => m + d.plus, 0), 0);
  return {
    aufgaben_fertig: fertig.size,
    schwarm: {
      kalender_stunden: r1(kalenderStunden),
      agenten_stunden: r1(zeilen.reduce((n, z) => n + z.sekunden, 0) / 3600),
      agenten: new Set(zeilen.map((z) => z.agent)).size,
      menschen: new Set(zeilen.map((z) => z.mensch)).size,
      modelle: new Set(zeilen.map((z) => z.modell).filter(Boolean)).size,
    },
    ein_agent: { stunden: r1(einAgentStunden) },
    // > 1: der Schwarm war schneller, als ein einzelner Agent rund um die Uhr gewesen wäre (eine Untergrenze für den Vorteil)
    beschleunigung: kalenderStunden > 0 && einAgentStunden > 0 ? r1(einAgentStunden / kalenderStunden) : null,
    menschen_team: {
      geschaetzte_stunden: schaetzungen.size ? r1(menschenStunden) : null,
      aufgaben_geschaetzt: schaetzungen.size,
      team: TEAM.menschen,
      wochen: schaetzungen.size ? r1(menschenStunden / (TEAM.menschen * TEAM.stunden_je_woche)) : null,
      // geschätzte Menschenstunden, die der Schwarm je Kalendertag erledigt: vergleichbar über verschieden große Umfänge
      durchsatz_je_tag: schaetzungen.size && kalenderStunden > 0 ? r1(menschenStunden / (kalenderStunden / 24)) : null,
      cocomo: cocomo(codeZeilen),
    },
    // jeder angenommene Beitrag von einem anderen Menschen bewertet als dem, der ihn eingereicht hat
    vier_augen_prozent: angenommen.length ? Math.round((100 * angenommen.filter((z) => z.bewerter !== null && z.bewerter !== z.mensch).length) / angenommen.length) : null,
    pruefstrasse_prozent: angenommen.length ? Math.round((100 * angenommen.filter((z) => z.pruefung === "ok").length) / angenommen.length) : null,
  };
}

export async function schwarmBilanz(db: Db, projektId: number) {
  const zeilen: Zeile[] = (await db.query(
    `select e.id::int as einreichung, a.id::int as aufgabe, m.titel as meilenstein, ag.mensch_id::int as mensch, e.agent_id::int as agent, e.modell,
       coalesce((e.verbrauch->>'sekunden')::numeric, (e.verbrauch->>'seconds')::numeric, 0)::float as sekunden,
       b.urteil, b.erstellt as bewertet, ba.mensch_id::int as bewerter, a.zustand, a.menschen_stunden,
       (select pr.ergebnis from pruefungen pr where pr.einreichung_id = e.id and pr.quelle = 'projekt' order by pr.id desc limit 1) as pruefung,
       case when b.urteil = 'annehmen' then e.patch end as patch
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
  const vergleich = (await db.query("select vergleich from projekte where id = $1", [projektId])).rows[0]?.vergleich ?? null;
  const gesamt = gruppenBilanz(zeilen, gesamtStart);
  // Vergleich über den Durchsatz (Menschenstunden je Kalendertag), wenn der Vergleichswert eine Schätzung seines Umfangs hat
  if (vergleich?.menschen_stunden && vergleich?.kalender_tage) {
    vergleich.durchsatz_je_tag = r1(Number(vergleich.menschen_stunden) / Number(vergleich.kalender_tage));
    if (gesamt?.menschen_team.durchsatz_je_tag) vergleich.durchsatz_faktor = r1(gesamt.menschen_team.durchsatz_je_tag / vergleich.durchsatz_je_tag);
  }
  return {
    gesamt,
    meilensteine: titel.map((t) => ({ titel: t, abgeschlossen: (offen.get(t) ?? 0) === 0, ...gruppenBilanz(zeilen.filter((z) => z.meilenstein === t), startVon(t)) }))
      .filter((m) => m.aufgaben_fertig),
    vergleich,
    methode: { team: TEAM, cocomo_ab_zeilen: COCOMO_AB_ZEILEN },
  };
}
