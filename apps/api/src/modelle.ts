// Öffentliche Modell-Rangliste (/api/models, Website /modelle): Wie oft wird die Arbeit eines Modells angenommen, gemessen an
// echten Aufgaben statt an Benchmarks. Keine personenbezogenen Daten: Ein Modell erscheint erst ab MIN_BEWERTET bewerteten
// Einreichungen von mindestens MIN_MITSTREITER verschiedenen Menschen, Aufschlüsselungen erst ab MIN_ZEILE Bewertungen.
// Sortiert wird nach der unteren Grenze des 95-%-Wilson-Intervalls, damit wenige Glückstreffer nicht vorne landen.
import type { Db } from "./db.ts";

export const MODELL_REGELN = { zeitraum_tage: 90, mindestens_bewertet: 5, mindestens_mitstreiter: 3, mindestens_je_zeile: 3 };
const CACHE_MS = 5 * 60_000;
let zwischenspeicher: { zeit: number; daten: unknown } | null = null;

/** Untere Grenze des Wilson-Intervalls (95 %) für k Erfolge in n Versuchen, in Prozent. */
export function wilsonUnten(k: number, n: number, z = 1.96): number {
  if (n === 0) return 0;
  const p = k / n, z2 = z * z;
  const mitte = p + z2 / (2 * n), breite = z * Math.sqrt((p * (1 - p)) / n + z2 / (4 * n * n));
  return Math.round((1000 * (mitte - breite)) / (1 + z2 / n)) / 10;
}

/** Schreibweisen eines Modells zusammenführen: „Claude Opus 5.5“, „claude-opus-5.5“ und „claude_opus_5_5“ sind eins. */
export const modellSchluessel = (m: string) => m.trim().toLowerCase().replace(/[\s_.]+/g, "-");

type Zeile = { schluessel: string; roh: string; programm: string | null; mensch: number; projekt: number; stufe: string; familie: string;
  tokens: number | null; urteil: string | null; pruefung: string | null };

const prozent = (k: number, n: number) => (n ? Math.round((100 * k) / n) : null);

export async function modellRangliste(db: Db) {
  if (zwischenspeicher && Date.now() - zwischenspeicher.zeit < CACHE_MS) return zwischenspeicher.daten;
  const R = MODELL_REGELN;
  const zeilen = (await db.query(
    `select e.modell as roh, e.agent_programm as programm, ag.mensch_id::int as mensch, a.projekt_id::int as projekt, a.stufe,
            analytik.familie(a.faehigkeiten) as familie, nullif((e.verbrauch->>'tokens')::numeric, 0)::float as tokens, b.urteil, pr.ergebnis as pruefung
     from einreichungen e join agenten ag on ag.id = e.agent_id join aufgaben a on a.id = e.aufgabe_id
     join projekte p on p.id = a.projekt_id and not p.gesperrt and not p.entwurf
     left join lateral (select urteil from bewertungen where einreichung_id = e.id order by id desc limit 1) b on true
     left join lateral (select ergebnis from pruefungen where einreichung_id = e.id and quelle = 'projekt' order by id desc limit 1) pr on true
     where nullif(trim(e.modell), '') is not null and e.eingereicht > now() - make_interval(days => $1)`, [R.zeitraum_tage])).rows
    .map((z) => ({ ...z, schluessel: modellSchluessel(z.roh) })) as Zeile[];

  const gruppen = new Map<string, Zeile[]>();
  for (const z of zeilen) gruppen.set(z.schluessel, [...(gruppen.get(z.schluessel) ?? []), z]);
  const modelle = [];
  let ausgeblendet = 0;
  for (const [schluessel, g] of gruppen) {
    const bewertet = g.filter((z) => z.urteil);
    const menschen = new Set(g.map((z) => z.mensch)).size;
    if (bewertet.length < R.mindestens_bewertet || menschen < R.mindestens_mitstreiter) { ausgeblendet++; continue; }
    const angenommen = bewertet.filter((z) => z.urteil === "annehmen").length;
    const geprueft = g.filter((z) => z.pruefung === "ok" || z.pruefung === "fehler");
    const tokens = g.filter((z) => z.urteil === "annehmen" && z.tokens).map((z) => z.tokens!).sort((x, y) => x - y);
    // häufigste Schreibweise als Anzeigename
    const namen = new Map<string, number>();
    for (const z of g) namen.set(z.roh.trim(), (namen.get(z.roh.trim()) ?? 0) + 1);
    const aufteilen = (feld: "stufe" | "familie") => {
      const m = new Map<string, { bewertet: number; angenommen: number }>();
      for (const z of bewertet) {
        const w = m.get(z[feld]) ?? { bewertet: 0, angenommen: 0 };
        w.bewertet++; if (z.urteil === "annehmen") w.angenommen++;
        m.set(z[feld], w);
      }
      return [...m].filter(([, w]) => w.bewertet >= R.mindestens_je_zeile).sort(([a], [b]) => a.localeCompare(b))
        .map(([k, w]) => ({ [feld]: k, ...w, annahme_prozent: prozent(w.angenommen, w.bewertet) }));
    };
    const programme = new Map<string, number>();
    for (const z of g) programme.set(z.programm ?? "unbekannt", (programme.get(z.programm ?? "unbekannt") ?? 0) + 1);
    modelle.push({
      modell: [...namen].sort((a, b) => b[1] - a[1])[0][0], schluessel,
      einreichungen: g.length, bewertet: bewertet.length, angenommen,
      nacharbeit: bewertet.filter((z) => z.urteil === "nacharbeit").length, abgelehnt: bewertet.filter((z) => z.urteil === "ablehnen").length,
      annahme_prozent: prozent(angenommen, bewertet.length), untere_grenze_prozent: wilsonUnten(angenommen, bewertet.length),
      pruefung_ok_prozent: prozent(geprueft.filter((z) => z.pruefung === "ok").length, geprueft.length),
      tokens_median_je_annahme: tokens.length >= R.mindestens_je_zeile ? Math.round(tokens[Math.floor(tokens.length / 2)]) : null,
      menschen, projekte: new Set(g.map((z) => z.projekt)).size,
      stufen: aufteilen("stufe"), familien: aufteilen("familie"),
      programme: [...programme].sort((a, b) => b[1] - a[1]).map(([programm, n]) => ({ programm, anteil_prozent: prozent(n, g.length) })),
    });
  }
  modelle.sort((a, b) => b.untere_grenze_prozent - a.untere_grenze_prozent || b.bewertet - a.bewertet);
  const daten = {
    stand: new Date().toISOString(), regeln: R, modelle, ausgeblendet,
    methode: "Accepted ÷ reviewed submissions per model over the last 90 days, from tasks of real projects. Ranked by the lower bound of the 95 % Wilson interval. " +
      "A model is listed from 5 reviewed submissions by at least 3 different people; breakdowns from 3 reviews. Tasks differ between models, so compare levels and kinds, not only the total.",
  };
  zwischenspeicher = { zeit: Date.now(), daten };
  return daten;
}

/** Für Tests: Zwischenspeicher leeren. */
export const modellCacheLeeren = () => { zwischenspeicher = null; };
