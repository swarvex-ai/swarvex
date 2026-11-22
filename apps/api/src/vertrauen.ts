// Vertrauensstufen (E-35, Issue #28, docs/concept.md §6): verdient, automatisch, nachvollziehbar.
//   0 Neu       Start
//   1 Prüfer    3 gemergte Beiträge, nicht zu viele Ablehnungen in 30 Tagen, keine Regression in 90 Tagen → Peer-Reviews
//   2 Erfahren  10 gemergte Beiträge (3 davon ab Aufgabenstufe B), 5 Peer-Reviews, nicht zu viele Ablehnungen in 60
//               Tagen, keine Regression in 90 Tagen → kann Bereichs-Lead werden
//   3 Bereichs-Lead, solange die Person in einem Projekt einen Bereich führt und Stufe 2 verdient hat (docs/concept.md).
//     Ein Amt allein hebt nichts: Wer über ein Lead-Gesuch ohne Stufe 2 ernannt wird (E-39), führt seinen Bereich in
//     diesem Projekt, bekommt aber keine plattformweite Stufe (Sicherheitsaudit 12.11.2026: Zweitkonten im eigenen
//     Entwurfsprojekt zu Bereichs-Leads ernennen hätte sonst Fremdprüfungen in fremden Projekten freigeschaltet).
// Abstieg sofort: je gemeldeter Regression eine Stufe; zu viele Ablehnungen innerhalb von 30 Tagen eine Stufe.
// Zu viele heißt: mindestens 2 und mehr als 10 % der letzten 20 Urteile über die eigenen Einreichungen im Zeitfenster.
// Bis 22.11.2026 reichten 2 Ablehnungen, und Aufstieg verlangte 30 Tage ganz ohne: Bei 183 angenommenen und 3
// abgelehnten Einreichungen (1,6 %) stand der Visionär auf 0, der zweite aktive Mensch ebenso, und niemand Aktives
// durfte mehr fremdprüfen (21 Einreichungen warteten bis zu einer Woche).
// Jede Änderung steht mit Grund im öffentlichen Protokoll. Betreiber können die Stufe setzen (cli set-trust).
import { transaktion, type Db, type Tx } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";

export const VERTRAUEN_REGELN = {
  1: { gemerged: 3, ablehnungTage: 30 },
  2: { gemerged: 10, abStufeB: 3, fremdpruefungen: 5, ablehnungTage: 60 },
  ohneRegressionTage: 90,
  /** zu viele Ablehnungen: mindestens so viele und mehr als dieser Anteil der letzten Urteile (annehmen/ablehnen) */
  ablehnung: { mindestens: 2, anteil: 0.1, letzte: 20 },
  abstieg: { tage: 30 },
} as const;

export const STUFEN_NAMEN = ["new", "reviewer", "experienced", "area lead"] as const;

export type VertrauenZahlen = {
  gemerged: number; abStufeB: number; fremdpruefungen: number;
  /** Urteile über die eigenen Einreichungen (annehmen/ablehnen), neueste zuerst */
  urteile: { zeit: Date; abgelehnt: boolean }[]; regressionen: Date[]; bereichsLead: boolean;
};

const TAG = 86_400_000;
const seitTagen = (d: Date, jetzt: number) => (jetzt - d.getTime()) / TAG;

/** Ablehnungen unter den letzten Urteilen im Zeitfenster; zaehlt (wenn gegeben) wählt, welche Ablehnungen zählen. */
export function ablehnungsQuote(urteile: VertrauenZahlen["urteile"], tage: number, jetzt: number, zaehlt: (d: Date) => boolean = () => true) {
  const r = VERTRAUEN_REGELN.ablehnung;
  const fenster = urteile.filter((u) => seitTagen(u.zeit, jetzt) < tage).slice(0, r.letzte);
  const ablehnungen = fenster.filter((u) => u.abgelehnt && zaehlt(u.zeit)).length;
  return { ablehnungen, urteile: fenster.length, zuViele: ablehnungen >= r.mindestens && ablehnungen > r.anteil * fenster.length };
}
const zuVieleText = (q: { ablehnungen: number; urteile: number }, tage: number) =>
  `too many rejections: ${q.ablehnungen} of your last ${q.urteile} reviewed submissions within ${tage} days (at most 1, or ${VERTRAUEN_REGELN.ablehnung.anteil * 100} %)`;

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
  const q = ablehnungsQuote(z.urteile, r.ablehnungTage, jetzt);
  if (q.zuViele) fehlt.push(zuVieleText(q, r.ablehnungTage));
  const letzteRegression = z.regressionen.find((d) => seitTagen(d, jetzt) < VERTRAUEN_REGELN.ohneRegressionTage);
  if (letzteRegression) fehlt.push(`no regression for ${Math.ceil(VERTRAUEN_REGELN.ohneRegressionTage - seitTagen(letzteRegression, jetzt))} more days`);
  return fehlt;
}

/**
 * Neue Stufe aus der bisherigen und den Zahlen. Abstieg zählt nur Ereignisse nach der letzten Änderung (seit),
 * damit dieselbe Ablehnung nicht zweimal abstuft; Aufstieg prüft die Zeitfenster. Bereichs-Leads mit verdienter
 * Stufe 2 stehen auf 3; endet das Amt, bleibt die verdiente Stufe.
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
  const q = ablehnungsQuote(z.urteile, VERTRAUEN_REGELN.abstieg.tage, jetzt, nachSeit);
  if (q.zuViele && basis > 0) {
    basis -= 1;
    gruende.push(zuVieleText(q, VERTRAUEN_REGELN.abstieg.tage));
  }
  while (basis < 2 && fehltFuer((basis + 1) as 1 | 2, z, jetzt).length === 0) {
    basis += 1;
    gruende.push(basis === 1 ? `${z.gemerged} merged contributions` : `${z.gemerged} merged (${z.abStufeB} at level B+), ${z.fremdpruefungen} peer reviews`);
  }
  if (z.bereichsLead && basis >= 2) return { stufe: 3, grund: "leads an area of a project" };
  return { stufe: basis, grund: gruende.join("; ") };
}

export async function vertrauenZahlen(q: Db | Tx, menschId: number): Promise<VertrauenZahlen> {
  const r = (await q.query(
    `with meine as (select e.id, a.stufe from einreichungen e join agenten ag on ag.id = e.agent_id join aufgaben a on a.id = e.aufgabe_id where ag.mensch_id = $1),
          gem as (select m.* from meine m where exists (select 1 from ereignisse x where x.art = 'gemerged' and x.daten->>'einreichung' = m.id::text))
     select (select count(*) from gem)::int as gemerged,
            (select count(*) from gem where stufe in ('B', 'C', 'D'))::int as ab_b,
            (select count(*) from fremdpruefungen where mensch_id = $1)::int as fremd,
            coalesce((select jsonb_agg(jsonb_build_object('zeit', u.erstellt, 'abgelehnt', u.urteil = 'ablehnen') order by u.erstellt desc)
              from (select b.erstellt, b.urteil from bewertungen b join meine m on m.id = b.einreichung_id
                    where b.urteil in ('annehmen', 'ablehnen') order by b.erstellt desc limit ${VERTRAUEN_REGELN.ablehnung.letzte}) u), '[]') as urteile,
            coalesce((select array_agg(g.erstellt order by g.erstellt desc) from regressionen g join meine m on m.id = g.einreichung_id), '{}') as regressionen,
            exists (select 1 from officers o join projekte p on p.id = o.projekt_id where o.mensch_id = $1 and o.beendet is null and not p.gesperrt) as bereichs_lead`,
    [menschId])).rows[0];
  return { gemerged: r.gemerged, abStufeB: r.ab_b, fremdpruefungen: r.fremd, urteile: r.urteile.map((u: { zeit: string; abgelehnt: boolean }) => ({ zeit: new Date(u.zeit), abgelehnt: u.abgelehnt })), regressionen: r.regressionen, bereichsLead: r.bereichs_lead };
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
