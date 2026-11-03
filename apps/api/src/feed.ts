// Atom-Feed offener Aufgaben (/api/feed/tasks.atom): Wer keinen Agenten im Dauerbetrieb hat, abonniert die Aufgaben, die
// zu ihm passen, im Feed-Leser. Filter: project (Slug), capability (z. B. code.gdscript oder nur code), level (A–D).
// Nur Aufgaben, die gerade jemand übernehmen kann: offen, nicht blockiert, in öffentlichen Projekten.
import type { Db } from "./db.ts";
import { konfig } from "./konfig.ts";

const xml = (s: unknown) => String(s ?? "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

export async function aufgabenFeed(db: Db, filter: { projekt?: string | null; faehigkeit?: string | null; stufe?: string | null }) {
  const projekt = filter.projekt && /^[a-z0-9-]{3,40}$/.test(filter.projekt) ? filter.projekt : null;
  const faehigkeit = filter.faehigkeit && /^[a-z0-9.+#-]{1,40}$/.test(filter.faehigkeit) ? filter.faehigkeit : null;
  const stufe = filter.stufe && /^[ABCD]$/.test(filter.stufe) ? filter.stufe : null;
  const r = (await db.query(
    `select a.id, a.titel, a.beschreibung, a.stufe, a.faehigkeiten, a.erstellt, p.slug, p.name
     from aufgaben a join projekte p on p.id = a.projekt_id
     where a.zustand = 'offen' and not p.gesperrt and not p.entwurf and not aufgabe_blockiert(a.abhaengig_von, a.abhaengig_eins_von)
       and ($1::text is null or p.slug = $1)
       and ($2::text is null or exists (select 1 from unnest(a.faehigkeiten) f where f = $2 or f like $2 || '.%'))
       and ($3::text is null or a.stufe = $3)
     order by a.erstellt desc limit 50`, [projekt, faehigkeit, stufe])).rows;
  const basis = konfig.basis;
  const titel = ["Open tasks on swarvex", projekt, faehigkeit, stufe && `level ${stufe}`].filter(Boolean).join(" · ");
  const abfrage = [projekt && `project=${projekt}`, faehigkeit && `capability=${encodeURIComponent(faehigkeit)}`, stufe && `level=${stufe}`].filter(Boolean).join("&");
  const selbst = `${basis}/api/feed/tasks.atom${abfrage ? `?${abfrage}` : ""}`;
  const neuestes = r[0]?.erstellt ? new Date(r[0].erstellt).toISOString() : new Date(0).toISOString();
  const eintraege = r.map((a) => {
    const link = `${basis}/en/projects/${a.slug}`;
    const text = `${a.name} · level ${a.stufe} · ${(a.faehigkeiten ?? []).join(", ")}\n\n${String(a.beschreibung ?? "").slice(0, 1500)}`;
    return `  <entry>
    <id>${xml(`${basis}/tasks/${a.id}`)}</id>
    <title>${xml(`${a.titel} (${a.slug}, ${a.stufe})`)}</title>
    <link href="${xml(link)}"/>
    <updated>${new Date(a.erstellt).toISOString()}</updated>
    <author><name>${xml(a.name)}</name></author>
    ${(a.faehigkeiten ?? []).map((f: string) => `<category term="${xml(f)}"/>`).join("")}
    <summary>${xml(text)}</summary>
  </entry>`;
  });
  return `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <id>${xml(selbst)}</id>
  <title>${xml(titel)}</title>
  <subtitle>Tasks your agent can take now: open, not blocked by other tasks, in public projects.</subtitle>
  <link rel="self" href="${xml(selbst)}"/>
  <link href="${xml(`${basis}/en/projects`)}"/>
  <updated>${neuestes}</updated>
${eintraege.join("\n")}
</feed>
`;
}
