// Der Schwarm lernt: Gründe für Nacharbeit und Ablehnung (vom Lead, von Officers und aus Fremdprüfungen) je Rolle gebündelt,
// damit der Lead wiederkehrende Fehler in die Projektregeln (AGENTS.md) schreibt und spätere Agenten sie nicht wiederholen.
import type { Db } from "./db.ts";
import { Abgelehnt, projektPerSlug, rollenVon, type Wer } from "./aufgaben.ts";

export async function nacharbeitMuster(db: Db, wer: Wer, slug: string, tage: number) {
  const p = await projektPerSlug(db, slug);
  const rollen = await rollenVon(db, wer.menschId, Number(p.id));
  const officer = (await db.query("select 1 from officers where projekt_id = $1 and mensch_id = $2 and beendet is null", [p.id, wer.menschId])).rowCount;
  if (!rollen.includes("lead") && !rollen.includes("visionaer") && !officer) throw new Abgelehnt("only the project's lead, visionary or an officer may do this");
  const r = (await db.query(
    `select coalesce(a.rolle, '–') as rolle, x.quelle, x.urteil, x.begruendung, x.erstellt, a.id::int as aufgabe
     from (select e.aufgabe_id, 'lead' as quelle, b.urteil, b.begruendung, b.erstellt from bewertungen b join einreichungen e on e.id = b.einreichung_id
             where b.urteil in ('nacharbeit', 'ablehnen')
           union all
           select e.aufgabe_id, 'fremdpruefung', f.urteil, f.begruendung, f.erstellt from fremdpruefungen f join einreichungen e on e.id = f.einreichung_id
             where f.urteil = 'aenderungen') x
     join aufgaben a on a.id = x.aufgabe_id
     where a.projekt_id = $1 and x.erstellt > now() - make_interval(days => $2)
     order by x.erstellt desc`, [p.id, Math.min(Math.max(tage, 1), 180)])).rows;
  type Grund = { aufgabe: number; quelle: string; urteil: string; begruendung: string };
  const je = new Map<string, { rolle: string; anzahl: number; gruende: Grund[] }>();
  for (const z of r) {
    const g = je.get(z.rolle) ?? { rolle: z.rolle, anzahl: 0, gruende: [] as Grund[] };
    g.anzahl++;
    if (g.gruende.length < 8) g.gruende.push({ aufgabe: z.aufgabe, quelle: z.quelle, urteil: z.urteil, begruendung: String(z.begruendung).slice(0, 600) });
    je.set(z.rolle, g);
  }
  return {
    tage, gesamt: r.length, je_rolle: [...je.values()].sort((x, y) => y.anzahl - x.anzahl),
    hinweis: "Look for reasons that repeat. Write each one as a short rule into AGENTS.md (you change it directly) or into the role's document, then cut the next tasks with it in the acceptance criteria.",
  };
}
