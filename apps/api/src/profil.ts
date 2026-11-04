// Öffentliches Beitragsprofil (/api/people/<login>, Website /mitstreiter/<login>): nur, wenn die Person es im Konto
// einschaltet (menschen.profil_oeffentlich, Standard aus). Es zeigt, was ohnehin im Protokoll steht, an einer Stelle:
// angenommene Beiträge je Projekt, Arten der Arbeit, Fremdprüfungen, Rollen und Abzeichen. Keine Agenten-Namen, keine Zeiten.
import type { Db } from "./db.ts";
import { sorteVon } from "./beitraege.ts";

/** Abzeichen: Kennung und Bedingung; die Website beschriftet sie. Reihenfolge = Anzeige. */
const ABZEICHEN: [string, (z: Zahlen) => boolean][] = [
  ["first_accepted", (z) => z.angenommen >= 1],
  ["ten_accepted", (z) => z.angenommen >= 10],
  ["hundred_accepted", (z) => z.angenommen >= 100],
  ["peer_reviewer", (z) => z.fremdpruefungen >= 5],
  ["three_projects", (z) => z.projekte >= 3],
  ["lead", (z) => z.lead],
  ["area_lead", (z) => z.officer],
  ["clean_room", (z) => z.verfahren.includes("clean_room")],
  ["continuation", (z) => z.verfahren.includes("continuation")],
  ["research", (z) => z.forschung],
];
type Zahlen = { angenommen: number; fremdpruefungen: number; projekte: number; lead: boolean; officer: boolean; verfahren: string[]; forschung: boolean };

export async function profilSetzen(db: Db, menschId: number, oeffentlich: boolean) {
  await db.query("update menschen set profil_oeffentlich = $2 where id = $1", [menschId, oeffentlich]);
  return { oeffentlich };
}

export async function profil(db: Db, login: string) {
  const m = (await db.query("select id, login, erstellt, vertrauensstufe from menschen where lower(login) = lower($1) and profil_oeffentlich and not gesperrt", [login])).rows[0];
  if (!m) return null;
  const projekte = (await db.query(
    `select p.slug as projekt, p.name, p.verfahren, p.projektart, count(*)::int as angenommen
     from einreichungen e join agenten ag on ag.id = e.agent_id join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id
     join bewertungen b on b.einreichung_id = e.id and b.urteil = 'annehmen'
     where ag.mensch_id = $1 and not p.gesperrt and not p.entwurf group by p.slug, p.name, p.verfahren, p.projektart order by angenommen desc, p.name`, [m.id])).rows;
  // Arten der Arbeit aus den Pfaden der angenommenen Patches (wie in „Mein Konto“)
  const patches = (await db.query(
    `select e.patch from einreichungen e join agenten ag on ag.id = e.agent_id join bewertungen b on b.einreichung_id = e.id and b.urteil = 'annehmen'
     join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id where ag.mensch_id = $1 and not p.gesperrt order by e.id desc limit 500`, [m.id])).rows;
  const sorten = new Map<string, number>();
  for (const { patch } of patches) {
    for (const s of new Set([...String(patch).matchAll(/^diff --git a\/\S+ b\/(\S+)$/gm)].map((x) => sorteVon(x[1])))) sorten.set(s, (sorten.get(s) ?? 0) + 1);
  }
  const fremd = Number((await db.query("select count(*) from fremdpruefungen where mensch_id = $1", [m.id])).rows[0].count);
  const rollen = (await db.query(
    "select distinct r.rolle from projekt_rollen r join projekte p on p.id = r.projekt_id where r.mensch_id = $1 and not p.entwurf and not p.gesperrt", [m.id])).rows.map((r) => r.rolle);
  const officer = Number((await db.query("select count(*) from officers where mensch_id = $1", [m.id])).rows[0].count) > 0;
  const z: Zahlen = {
    angenommen: projekte.reduce((s, p) => s + p.angenommen, 0), fremdpruefungen: fremd, projekte: projekte.length,
    lead: rollen.includes("lead"), officer, verfahren: projekte.map((p) => p.verfahren), forschung: projekte.some((p) => p.projektart === "forschung"),
  };
  return {
    login: m.login, seit: String(m.erstellt.toISOString()).slice(0, 7), vertrauensstufe: m.vertrauensstufe,
    angenommen: z.angenommen, fremdpruefungen: fremd,
    projekte: projekte.map(({ projekt, name, angenommen }) => ({ projekt, name, angenommen })),
    sorten: [...sorten].sort((a, b) => b[1] - a[1]).map(([sorte, anzahl]) => ({ sorte, anzahl })),
    abzeichen: ABZEICHEN.filter(([, wenn]) => wenn(z)).map(([k]) => k),
  };
}
