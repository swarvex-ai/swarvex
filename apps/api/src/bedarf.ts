// Projektübergreifende Aufgaben (Bedarf, E-39): Projekte auf swarvex bauen aufeinander auf. Grundriss meldet bei
// Formwerk „brauche einen Ohrensessel“; der Lead von Formwerk nimmt an und verknüpft die Aufgabe, die das liefert. Die
// wartende Aufgabe in Grundriss ist so lange blockiert (abhaengig_von über Projektgrenzen) und wird frei, sobald die
// Formwerk-Aufgabe angenommen ist. Bedarf ist öffentlich sichtbar (Titel und Zustand): Er zeigt, wer wen braucht.
import { transaktion, type Db } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt, officersVon, projektPerSlug, rollenVon, type Wer } from "./aufgaben.ts";

export const BEDARF_OFFEN_MAX = 20;

async function darfFuehren(db: Db, menschId: number, projektId: number) {
  const rollen = await rollenVon(db, menschId, projektId);
  return rollen.includes("lead") || rollen.includes("visionaer") || (await officersVon(db, menschId, projektId)).length > 0;
}

/** Lead, Visionär oder Bereichs-Lead eines Projekts meldet Bedarf bei einem anderen Projekt. */
export async function bedarfMelden(db: Db, wer: Wer, vonSlug: string, anSlug: string, b: { titel: string; text: string; aufgabe?: number }) {
  const titel = b.titel.trim(), text = b.text.trim();
  if (titel.length < 5 || titel.length > 140) throw new Abgelehnt("title: 5 to 140 characters, e.g. \"Wing chair with high back\"");
  if (text.length < 20 || text.length > 4000) throw new Abgelehnt("text: 20 to 4000 characters: what you need, the parameters or interface, and how you will use it");
  return transaktion(db, async (tx) => {
    const von = await projektPerSlug(tx, vonSlug), an = await projektPerSlug(tx, anSlug);
    if (Number(von.id) === Number(an.id)) throw new Abgelehnt("a request goes to another project; inside your own project create a task");
    if (an.entwurf || an.gesperrt) throw new Abgelehnt(`${anSlug} takes no requests right now`);
    if (!(await darfFuehren(tx as unknown as Db, wer.menschId, Number(von.id)))) throw new Abgelehnt("only the lead, visionary or an area lead of the requesting project sends requests");
    const offen = Number((await tx.query("select count(*) from bedarf where von_projekt = $1 and zustand = 'offen'", [von.id])).rows[0].count);
    if (offen >= BEDARF_OFFEN_MAX) throw new Abgelehnt(`at most ${BEDARF_OFFEN_MAX} open requests per project`);
    if (b.aufgabe !== undefined) {
      const a = (await tx.query("select projekt_id, zustand from aufgaben where id = $1", [b.aufgabe])).rows[0];
      if (!a || Number(a.projekt_id) !== Number(von.id)) throw new Abgelehnt(`waiting_task_id: #${b.aufgabe} is not a task of ${vonSlug}`);
      if (["angenommen", "gemerged", "geschlossen"].includes(a.zustand)) throw new Abgelehnt(`waiting_task_id: #${b.aufgabe} is already finished`);
    }
    const id = Number((await tx.query(
      "insert into bedarf(von_projekt, an_projekt, von_aufgabe, titel, text, von_mensch) values ($1,$2,$3,$4,$5,$6) returning id",
      [von.id, an.id, b.aufgabe ?? null, titel, text, wer.menschId])).rows[0].id);
    const login = (await tx.query("select login from menschen where id = $1", [wer.menschId])).rows[0].login;
    await tx.query(
      `insert into nachrichten(projekt_id, von_mensch, von_agent, an, art, betreff, inhalt, aufgaben, zustand)
       values ($1, null, null, 'lead', 'frage', $2, $3, '{}', 'offen')`,
      [an.id, `Request from ${von.slug}: ${titel}`.slice(0, 140),
        `${von.name} (${von.slug}, sent by ${login}) needs something from this project (request #${id}):\n\n${text}\n\n` +
        "Accept it by creating a task for it and linking it: answer_work_request with accept true and the task_id. Decline with a reason if it does not fit the project."]);
    await protokollieren(tx, { projekt_id: Number(von.id), akteur: `mensch:${wer.menschId}`, art: "bedarf_gemeldet", bezug: `projekt:${an.id}`, daten: { bedarf: id, titel } });
    return { bedarf_id: id, an: an.slug, zustand: "offen" };
  });
}

/** Lead, Visionär oder Bereichs-Lead des Zielprojekts nimmt an (mit eigener Aufgabe) oder lehnt ab (mit Grund). */
export async function bedarfBeantworten(db: Db, wer: Wer, id: number, annehmen: boolean, aufgabeId: number | undefined, grund: string) {
  return transaktion(db, async (tx) => {
    const b = (await tx.query("select b.*, v.slug as von_slug, n.slug as an_slug from bedarf b join projekte v on v.id = b.von_projekt join projekte n on n.id = b.an_projekt where b.id = $1 for update of b", [id])).rows[0];
    if (!b) throw new Abgelehnt(`no request #${id}`);
    if (!(await darfFuehren(tx as unknown as Db, wer.menschId, Number(b.an_projekt)))) throw new Abgelehnt("only the lead, visionary or an area lead of the requested project answers");
    if (b.zustand !== "offen") throw new Abgelehnt(`request #${id} is ${b.zustand === "angenommen" ? "accepted" : b.zustand === "abgelehnt" ? "declined" : b.zustand === "erledigt" ? "done" : "withdrawn"}`);
    let antwort: string;
    if (annehmen) {
      if (aufgabeId === undefined) throw new Abgelehnt("task_id: the task of your project that delivers this (create it first)");
      const a = (await tx.query("select projekt_id, zustand from aufgaben where id = $1", [aufgabeId])).rows[0];
      if (!a || Number(a.projekt_id) !== Number(b.an_projekt) || a.zustand === "geschlossen") throw new Abgelehnt(`task_id: #${aufgabeId} is not an open task of ${b.an_slug}`);
      await tx.query("update bedarf set zustand = 'angenommen', aufgabe_id = $2, antwort = $3, entschieden = now() where id = $1", [id, aufgabeId, grund.trim().slice(0, 2000) || null]);
      // Die wartende Aufgabe des anfragenden Projekts wartet jetzt auf diese Aufgabe
      if (b.von_aufgabe) await tx.query("update aufgaben set abhaengig_von = array_append(coalesce(abhaengig_von, '{}'), $2::bigint) where id = $1 and not ($2::bigint = any(coalesce(abhaengig_von, '{}')))", [b.von_aufgabe, aufgabeId]);
      antwort = `${b.an_slug} accepted request #${id} ("${b.titel}") with task #${aufgabeId}.${b.von_aufgabe ? ` Your task #${b.von_aufgabe} waits for it and is released when it is accepted.` : ""}${grund.trim() ? `\n\n${grund.trim()}` : ""}`;
    } else {
      if (grund.trim().length < 10) throw new Abgelehnt("reason: why it does not fit (at least 10 characters)");
      await tx.query("update bedarf set zustand = 'abgelehnt', antwort = $2, entschieden = now() where id = $1", [id, grund.trim().slice(0, 2000)]);
      antwort = `${b.an_slug} declined request #${id} ("${b.titel}"): ${grund.trim()}`;
    }
    await tx.query(
      `insert into nachrichten(projekt_id, von_mensch, von_agent, an, art, betreff, inhalt, aufgaben, zustand)
       values ($1, null, null, 'lead', 'info', $2, $3, $4, 'offen')`,
      [b.von_projekt, `Request #${id} ${annehmen ? "accepted" : "declined"} by ${b.an_slug}`, antwort, b.von_aufgabe ? [b.von_aufgabe] : []]);
    await protokollieren(tx, { projekt_id: Number(b.an_projekt), akteur: `mensch:${wer.menschId}`, art: annehmen ? "bedarf_angenommen" : "bedarf_abgelehnt", bezug: `projekt:${b.von_projekt}`,
      daten: { bedarf: id, ...(annehmen ? { aufgabe: aufgabeId } : {}) } });
    return { bedarf_id: id, zustand: annehmen ? "angenommen" : "abgelehnt" };
  });
}

/** Das anfragende Projekt zieht seinen Bedarf zurück (die Blockade seiner Aufgabe fällt weg). */
export async function bedarfZurueckziehen(db: Db, wer: Wer, id: number) {
  return transaktion(db, async (tx) => {
    const b = (await tx.query("select * from bedarf where id = $1 for update", [id])).rows[0];
    if (!b) throw new Abgelehnt(`no request #${id}`);
    if (!(await darfFuehren(tx as unknown as Db, wer.menschId, Number(b.von_projekt)))) throw new Abgelehnt("only the requesting project withdraws its request");
    if (!["offen", "angenommen"].includes(b.zustand)) throw new Abgelehnt(`request #${id} is already closed`);
    await tx.query("update bedarf set zustand = 'zurueckgezogen', entschieden = now() where id = $1", [id]);
    if (b.von_aufgabe && b.aufgabe_id) await tx.query("update aufgaben set abhaengig_von = array_remove(abhaengig_von, $2::bigint) where id = $1", [b.von_aufgabe, b.aufgabe_id]);
    return { bedarf_id: id, zustand: "zurueckgezogen" };
  });
}

/** Bedarf eines Projekts: an es (zu beantworten) und von ihm (wartend); öffentlich ohne den Text. */
export async function bedarfVon(db: Db, projektId: number, mitText = false) {
  const zeile = (r: Record<string, unknown>) => ({ id: Number(r.id), titel: r.titel, zustand: r.zustand, projekt: r.slug, projekt_name: r.name, aufgabe_id: r.aufgabe_id === null ? null : Number(r.aufgabe_id),
    von_aufgabe: r.von_aufgabe === null ? null : Number(r.von_aufgabe), erstellt: r.erstellt, ...(mitText ? { text: r.text, antwort: r.antwort } : {}) });
  const an = (await db.query(
    `select b.*, p.slug, p.name from bedarf b join projekte p on p.id = b.von_projekt where b.an_projekt = $1 and b.zustand in ('offen', 'angenommen') order by b.id`, [projektId])).rows.map(zeile);
  const von = (await db.query(
    `select b.*, p.slug, p.name from bedarf b join projekte p on p.id = b.an_projekt where b.von_projekt = $1 and b.zustand in ('offen', 'angenommen') order by b.id`, [projektId])).rows.map(zeile);
  return { an_uns: an, von_uns: von };
}

/** Regelmäßig: angenommener Bedarf, dessen Aufgabe fertig ist, ist erledigt; geschlossene Aufgabe: Bedarf wieder offen. */
export async function bedarfAktualisieren(db: Db) {
  const erledigt = (await db.query(
    `update bedarf b set zustand = 'erledigt', entschieden = now() from aufgaben a
     where a.id = b.aufgabe_id and b.zustand = 'angenommen' and a.zustand in ('angenommen', 'gemerged') returning b.id`)).rowCount ?? 0;
  const zurueck = (await db.query(
    `select b.id, b.von_projekt, b.von_aufgabe, b.aufgabe_id, b.titel, n.slug as an_slug from bedarf b join aufgaben a on a.id = b.aufgabe_id join projekte n on n.id = b.an_projekt
     where b.zustand = 'angenommen' and a.zustand = 'geschlossen'`)).rows;
  for (const b of zurueck) {
    await transaktion(db, async (tx) => {
      await tx.query("update bedarf set zustand = 'offen', aufgabe_id = null, entschieden = null where id = $1", [b.id]);
      if (b.von_aufgabe) await tx.query("update aufgaben set abhaengig_von = array_remove(abhaengig_von, $2::bigint) where id = $1", [b.von_aufgabe, b.aufgabe_id]);
      await tx.query(
        `insert into nachrichten(projekt_id, von_mensch, von_agent, an, art, betreff, inhalt, aufgaben, zustand)
         values ($1, null, null, 'lead', 'info', $2, $3, '{}', 'offen')`,
        [b.von_projekt, `Request #${b.id} open again`, `${b.an_slug} closed task #${b.aufgabe_id} that was to deliver "${b.titel}". The request is open again and your task no longer waits for it.`]);
    });
  }
  return { erledigt, wieder_offen: zurueck.length };
}
