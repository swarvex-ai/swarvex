// Vertrauensstufen (E-35, Issue #28, docs/concept.md §6): verdient, automatisch, nachvollziehbar.
//   0 Neu       Start
//   1 Prüfer    3 gemergte Beiträge, keine Ablehnung in 30 Tagen, keine Regression in 90 Tagen → Peer-Reviews
//   2 Erfahren  10 gemergte Beiträge (3 davon ab Aufgabenstufe B), 5 Peer-Reviews, keine Ablehnung in 60 Tagen,
//               keine Regression in 90 Tagen → kann Bereichs-Lead werden
//   3 Bereichs-Lead, solange die Person in einem Projekt einen Bereich führt (Ernennung, nicht berechnet)
// Abstieg sofort: je gemeldeter Regression eine Stufe; zwei Ablehnungen innerhalb von 30 Tagen eine Stufe.
// Jede Änderung steht mit Grund im öffentlichen Protokoll. Betreiber können die Stufe setzen (cli set-trust).
import { transaktion, type Db, type Tx } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";

export const VERTRAUEN_REGELN = {
  1: { gemerged: 3, ohneAblehnungTage: 30 },
  2: { gemerged: 10, abStufeB: 3, fremdpruefungen: 5, ohneAblehnungTage: 60 },
  ohneRegressionTage: 90,
  abstieg: { ablehnungen: 2, tage: 30 },
} as const;

export const STUFEN_NAMEN = ["new", "reviewer", "experienced", "area lead"] as const;

export type VertrauenZahlen = {
  gemerged: number; abStufeB: number; fremdpruefungen: number;
  ablehnungen: Date[]; regressionen: Date[]; bereichsLead: boolean;
};

const TAG = 86_400_000;
const seitTagen = (d: Date, jetzt: number) => (jetzt - d.getTime()) / TAG;

/** Was für eine Stufe noch fehlt (leer = erfüllt). Englisch, weil es Agenten und die Website so zeigen. */
export function fehltFuer(stufe: 1 | 2, z: VertrauenZahlen, jetzt = Date.now()): string[] {
  const r = VERTRAUEN_REGELN[stufe];
  const fehlt: string[] = [];
  if (z.gemerged < r.gemerged) fehlt.push(`${r.gemerged - z.gemerged} more merged contribution${r.gemerged - z.gemerged === 1 ? "" : "s"}`);
  if (stufe === 2) {
    const r2 = VERTRAUEN_REGELN[2];
    if (z.abStufeB < r2.abStufeB) fehlt.push(`${r2.abStufeB - z.abStufeB} more merged at task level B or higher`);
    if (z.fremdpruefungen < r2.fremdpruefungen) fehlt.push(`${r2.fremdpruefungen - z.fremdpruefungen} more peer review${r2.fremdpruefungen - z.fremdpruefungen === 1 ? "" : "s"}`);
  }
  const letzteAblehnung = z.ablehnungen.find((d) => seitTagen(d, jetzt) < r.ohneAblehnungTage);
  if (letzteAblehnung) fehlt.push(`no rejection for ${Math.ceil(r.ohneAblehnungTage - seitTagen(letzteAblehnung, jetzt))} more days`);
  const letzteRegression = z.regressionen.find((d) => seitTagen(d, jetzt) < VERTRAUEN_REGELN.ohneRegressionTage);
  if (letzteRegression) fehlt.push(`no regression for ${Math.ceil(VERTRAUEN_REGELN.ohneRegressionTage - seitTagen(letzteRegression, jetzt))} more days`);
  return fehlt;
}

/**
 * Neue Stufe aus der bisherigen und den Zahlen. Abstieg zählt nur Ereignisse nach der letzten Änderung (seit),
 * damit dieselbe Ablehnung nicht zweimal abstuft; Aufstieg prüft die Zeitfenster. Bereichs-Leads stehen auf 3.
 */
export function stufeBerechnen(bisher: number, seit: Date | null, z: VertrauenZahlen, jetzt = Date.now()): { stufe: number; grund: string } {
  let basis = Math.min(Math.max(bisher, 0), 2);
  const gruende: string[] = [];
  const nachSeit = (d: Date) => !seit || d.getTime() > seit.getTime();
  const neueRegressionen = z.regressionen.filter(nachSeit).length;
  if (neueRegressionen) {
    basis = Math.max(0, basis - neueRegressionen);
    gruende.push(`${neueRegressionen} regression${neueRegressionen === 1 ? "" : "s"} reported`);
  }
  const neueAblehnungen = z.ablehnungen.filter((d) => nachSeit(d) && seitTagen(d, jetzt) < VERTRAUEN_REGELN.abstieg.tage).length;
  if (neueAblehnungen >= VERTRAUEN_REGELN.abstieg.ablehnungen && basis > 0) {
    basis -= 1;
    gruende.push(`${neueAblehnungen} rejections within ${VERTRAUEN_REGELN.abstieg.tage} days`);
  }
  while (basis < 2 && fehltFuer((basis + 1) as 1 | 2, z, jetzt).length === 0) {
    basis += 1;
    gruende.push(basis === 1 ? `${z.gemerged} merged contributions` : `${z.gemerged} merged (${z.abStufeB} at level B+), ${z.fremdpruefungen} peer reviews`);
  }
  if (z.bereichsLead) return { stufe: 3, grund: "leads an area of a project" };
  return { stufe: basis, grund: gruende.join("; ") };
}

export async function vertrauenZahlen(q: Db | Tx, menschId: number): Promise<VertrauenZahlen> {
  const r = (await q.query(
    `with meine as (select e.id, a.stufe from einreichungen e join agenten ag on ag.id = e.agent_id join aufgaben a on a.id = e.aufgabe_id where ag.mensch_id = $1),
          gem as (select m.* from meine m where exists (select 1 from ereignisse x where x.art = 'gemerged' and x.daten->>'einreichung' = m.id::text))
     select (select count(*) from gem)::int as gemerged,
            (select count(*) from gem where stufe in ('B', 'C', 'D'))::int as ab_b,
            (select count(*) from fremdpruefungen where mensch_id = $1)::int as fremd,
            coalesce((select array_agg(b.erstellt order by b.erstellt desc) from bewertungen b join meine m on m.id = b.einreichung_id where b.urteil = 'ablehnen'), '{}') as ablehnungen,
            coalesce((select array_agg(g.erstellt order by g.erstellt desc) from regressionen g join meine m on m.id = g.einreichung_id), '{}') as regressionen,
            exists (select 1 from officers o join projekte p on p.id = o.projekt_id where o.mensch_id = $1 and o.beendet is null and not p.gesperrt) as bereichs_lead`,
    [menschId])).rows[0];
  return { gemerged: r.gemerged, abStufeB: r.ab_b, fremdpruefungen: r.fremd, ablehnungen: r.ablehnungen, regressionen: r.regressionen, bereichsLead: r.bereichs_lead };
}

/** Stufe neu berechnen und, wenn sie sich ändert, speichern und protokollieren. anlass steht im Protokoll. */
export async function vertrauenAktualisieren(tx: Tx, menschId: number, anlass: string, jetzt = Date.now()) {
  const m = (await tx.query("select vertrauensstufe, vertrauen_seit from menschen where id = $1 for update", [menschId])).rows[0];
  if (!m) return null;
  const { stufe, grund } = stufeBerechnen(Number(m.vertrauensstufe), m.vertrauen_seit, await vertrauenZahlen(tx, menschId), jetzt);
  if (stufe === Number(m.vertrauensstufe)) return null;
  await tx.query("update menschen set vertrauensstufe = $2, vertrauen_seit = to_timestamp($3 / 1000.0) where id = $1", [menschId, stufe, jetzt]);
  await protokollieren(tx, { projekt_id: null, akteur: "system", art: "vertrauen_geaendert", bezug: `mensch:${menschId}`,
    daten: { von: Number(m.vertrauensstufe), nach: stufe, grund, anlass } });
  return { von: Number(m.vertrauensstufe), nach: stufe, grund };
}

/** Für die Konto-Seite und Agenten: Stufe, Zahlen und was bis zur nächsten fehlt. */
export async function vertrauenStand(db: Db, menschId: number) {
  const m = (await db.query("select vertrauensstufe from menschen where id = $1", [menschId])).rows[0];
  const z = await vertrauenZahlen(db, menschId);
  const stufe = Number(m?.vertrauensstufe ?? 0);
  const naechste = stufe < 2 ? { stufe: stufe + 1, name: STUFEN_NAMEN[stufe + 1], fehlt: fehltFuer((stufe + 1) as 1 | 2, z) } : null;
  return { stufe, name: STUFEN_NAMEN[stufe], gemerged: z.gemerged, gemerged_ab_b: z.abStufeB, fremdpruefungen: z.fremdpruefungen, naechste };
}

/** Nachts für alle: Zeitfenster laufen ab (eine alte Ablehnung blockiert nicht mehr). */
export async function alleVertrauenAktualisieren(db: Db) {
  const ids = (await db.query("select id from menschen where not gesperrt")).rows.map((r) => Number(r.id));
  let geaendert = 0;
  for (const id of ids) if (await transaktion(db, (tx) => vertrauenAktualisieren(tx, id, "daily"))) geaendert++;
  return geaendert;
}
