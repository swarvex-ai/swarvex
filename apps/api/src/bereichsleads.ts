// Bereichs-Leads automatisch (E-35). Mit wachsender Last empfiehlt die Lead-Bilanz einen Bereich (kennzahlen.ts,
// leadBilanz). Dann sucht die Plattform selbst einen Kandidaten: Vertrauensstufe 2, die meiste gemergte Arbeit in dieser
// Rolle im Projekt, in den letzten 14 Tagen aktiv. Pfade und Fähigkeiten kommen aus den echten Aufgaben der Rolle.
// Die Person bekommt eine Einladung (Mein Konto) und sagt zu oder ab; danach gibt der Visionär mit einem Klick frei,
// oder die Ernennung geschieht sofort, wenn er das für sein Projekt eingeschaltet hat. Ohne Aktivität endet eine
// Amtszeit nach 21 Tagen von selbst. Jeder Schritt steht im öffentlichen Protokoll.
import { Abgelehnt, officerBeenden, officerErnennen } from "./aufgaben.ts";
import { gesuchBeenden } from "./leadkraft.ts";
import { transaktion, type Db } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { leadBilanz } from "./kennzahlen.ts";

export const BEREICHS_LEAD = { aktivTage: 14, inaktivEndeTage: 21, vorschlagGueltigTage: 30, rueckblickTage: 180, mindestStufe: 2 } as const;

/** Mögliche Bereichs-Leads einer Rolle, beste zuerst. Ohne Stufe 2 niemand; Lead, Visionär und amtierende Bereichs-Leads nicht. */
export async function kandidaten(db: Db, projektId: number, rolle: string, anzahl = 3) {
  return (await db.query(
    `with gem as (
       select ag.mensch_id, count(*)::int as gemerged, max(x.zeit) as zuletzt_gemerged
       from ereignisse x join einreichungen e on e.id = (x.daten->>'einreichung')::bigint join aufgaben a on a.id = e.aufgabe_id join agenten ag on ag.id = e.agent_id
       where x.art = 'gemerged' and a.projekt_id = $1 and a.rolle = $2 and x.zeit > now() - make_interval(days => $3)
       group by ag.mensch_id),
     aktiv as (
       select ag.mensch_id, max(greatest(l.erstellt, coalesce(l.beendet, l.erstellt))) as zuletzt
       from leases l join agenten ag on ag.id = l.agent_id join aufgaben a on a.id = l.aufgabe_id where a.projekt_id = $1 group by ag.mensch_id)
     select m.id::int as mensch_id, m.login, m.konto_provider as anbieter, m.vertrauensstufe, g.gemerged, aktiv.zuletzt
     from gem g join menschen m on m.id = g.mensch_id join aktiv on aktiv.mensch_id = m.id
     where not m.gesperrt and m.vertrauensstufe >= $4 and aktiv.zuletzt > now() - make_interval(days => $5)
       and not exists (select 1 from projekt_rollen r where r.projekt_id = $1 and r.mensch_id = m.id and r.rolle in ('lead', 'visionaer'))
       and not exists (select 1 from officers o where o.projekt_id = $1 and o.mensch_id = m.id and o.beendet is null)
       and not exists (select 1 from bereichs_vorschlaege v where v.projekt_id = $1 and v.bereich = $2 and v.mensch_id = m.id and v.zustand = 'abgelehnt')
     order by g.gemerged desc, aktiv.zuletzt desc limit $6`,
    [projektId, rolle, BEREICHS_LEAD.rueckblickTage, BEREICHS_LEAD.mindestStufe, BEREICHS_LEAD.aktivTage, anzahl])).rows;
}

/** Der Bereich, wie er wirklich bearbeitet wird: Pfade und Fähigkeiten der Aufgaben dieser Rolle (ohne „ganzes Repo“). */
export async function bereichAusAufgaben(db: Db, projektId: number, rolle: string) {
  const r = (await db.query(
    `select coalesce(array_agg(distinct p) filter (where p not in ('**', '*') and p not like '/%' and p not like '%..%'), '{}') as pfade,
            coalesce((select array_agg(distinct f) from aufgaben b, unnest(b.faehigkeiten) f where b.projekt_id = $1 and b.rolle = $2 and b.erstellt > now() - make_interval(days => $3)), '{}') as faehigkeiten
     from aufgaben a, unnest(a.pfade) p where a.projekt_id = $1 and a.rolle = $2 and a.erstellt > now() - make_interval(days => $3)`,
    [projektId, rolle, BEREICHS_LEAD.rueckblickTage])).rows[0];
  return { pfade: (r.pfade as string[]).sort().slice(0, 30), faehigkeiten: (r.faehigkeiten as string[]).sort().slice(0, 20) };
}

/** Täglich: für jede empfohlene Rolle ohne Bereichs-Lead einen Kandidaten einladen. Clean-Room-Projekte (Räume) ernennt der Visionär selbst. */
export async function vorschlaegeErzeugen(db: Db) {
  const projekte = (await db.query("select id::int, slug from projekte where not entwurf and not gesperrt and verfahren is distinct from 'clean_room'")).rows;
  let neu = 0;
  for (const p of projekte) {
    const bilanz = await leadBilanz(db, p.slug);
    for (const { rolle, gruende } of bilanz.officer_empfehlung) {
      const k = (await kandidaten(db, p.id, rolle, 1))[0];
      if (!k) continue;
      const b = await bereichAusAufgaben(db, p.id, rolle);
      if (!b.pfade.length || !b.faehigkeiten.length) continue;
      const angelegt = await transaktion(db, async (tx) => {
        const r = await tx.query(
          `insert into bereichs_vorschlaege(projekt_id, bereich, mensch_id, pfade, faehigkeiten, gruende) values ($1,$2,$3,$4,$5,$6)
           on conflict (projekt_id, bereich) where zustand in ('offen', 'zugesagt') and gesuch_id is null do nothing returning id`,
          [p.id, rolle, k.mensch_id, b.pfade, b.faehigkeiten, JSON.stringify(gruende)]);
        if (!r.rowCount) return false;
        await protokollieren(tx, { projekt_id: p.id, akteur: "system", art: "bereichs_lead_vorgeschlagen", bezug: `mensch:${k.mensch_id}`,
          daten: { bereich: rolle, login: k.login, gemerged: k.gemerged, gruende } });
        return true;
      });
      if (angelegt) neu++;
    }
  }
  return neu;
}

async function ernennen(db: Db, v: { id: number; projekt_id: number; bereich: string; mensch_id: number; pfade: string[]; faehigkeiten: string[]; gesuch_id?: number | null }, akteur: string) {
  const m = (await db.query("select login, konto_provider, vertrauensstufe from menschen where id = $1", [v.mensch_id])).rows[0];
  // Bewerbung auf ein Lead-Gesuch (leadkraft.ts): Der Visionär wählt die Person selbst aus, ohne Vertrauensstufe 2
  const gesuch = v.gesuch_id ? (await db.query("select raum from lead_gesuche where id = $1", [v.gesuch_id])).rows[0] : null;
  if (!gesuch && Number(m.vertrauensstufe) < BEREICHS_LEAD.mindestStufe) throw new Abgelehnt(`${m.login} is no longer at trust level ${BEREICHS_LEAD.mindestStufe}`);
  const slug = (await db.query("select slug from projekte where id = $1", [v.projekt_id])).rows[0].slug;
  await officerErnennen(db, "betrieb", slug, `${m.konto_provider}:${m.login}`, v.bereich, v.pfade, v.faehigkeiten, gesuch?.raum ?? undefined, akteur);
  await db.query("update bereichs_vorschlaege set zustand = 'ernannt', entschieden = now() where id = $1", [v.id]);
  if (v.gesuch_id) await gesuchBeenden(db, Number(v.gesuch_id));
}

/** Die eingeladene Person sagt zu oder ab (Mein Konto). Bei „automatisch“ im Projekt ist sie damit ernannt. */
export async function einladungBeantworten(db: Db, menschId: number, id: number, zusage: boolean) {
  const v = (await db.query(
    "select v.*, p.bereichs_leads_automatisch as automatisch from bereichs_vorschlaege v join projekte p on p.id = v.projekt_id where v.id = $1 and v.mensch_id = $2", [id, menschId])).rows[0];
  if (!v || v.zustand !== "offen") throw new Abgelehnt("this invitation is no longer open");
  await transaktion(db, async (tx) => {
    await tx.query("update bereichs_vorschlaege set zustand = $2, entschieden = now() where id = $1", [id, zusage ? "zugesagt" : "abgelehnt"]);
    await protokollieren(tx, { projekt_id: Number(v.projekt_id), akteur: `mensch:${menschId}`, art: zusage ? "bereichs_lead_zugesagt" : "bereichs_lead_abgesagt", bezug: `mensch:${menschId}`, daten: { bereich: v.bereich } });
  });
  if (zusage && v.automatisch) await ernennen(db, v, "system");
  return { state: zusage ? (v.automatisch ? "appointed" : "accepted, waiting for the visionary") : "declined" };
}

/** Der Visionär gibt frei oder verwirft (Mein Konto). */
export async function vorschlagEntscheiden(db: Db, visionaerId: number, id: number, freigeben: boolean, grund = "") {
  const v = (await db.query(
    `select v.* from bereichs_vorschlaege v where v.id = $1
     and exists (select 1 from projekt_rollen r where r.projekt_id = v.projekt_id and r.mensch_id = $2 and r.rolle = 'visionaer')`, [id, visionaerId])).rows[0];
  if (!v || v.zustand !== "zugesagt") throw new Abgelehnt("nothing to decide for this proposal");
  if (freigeben) await ernennen(db, v, `mensch:${visionaerId}`);
  else await transaktion(db, async (tx) => {
    await tx.query("update bereichs_vorschlaege set zustand = 'verworfen', entschieden = now() where id = $1", [id]);
    await protokollieren(tx, { projekt_id: Number(v.projekt_id), akteur: `mensch:${visionaerId}`, art: "bereichs_lead_verworfen", bezug: `mensch:${v.mensch_id}`, daten: { bereich: v.bereich, grund: grund.trim().slice(0, 500) } });
  });
  return { state: freigeben ? "appointed" : "declined" };
}

/** Visionär: Ernennung ohne eigene Freigabe, sobald die Person zusagt. */
export async function automatikSetzen(db: Db, visionaerId: number, slug: string, an: boolean) {
  const r = await db.query(
    `update projekte p set bereichs_leads_automatisch = $3 where p.slug = $2
     and exists (select 1 from projekt_rollen r where r.projekt_id = p.id and r.mensch_id = $1 and r.rolle = 'visionaer') returning p.id`, [visionaerId, slug, an]);
  if (!r.rowCount) throw new Abgelehnt("only the project's visionary changes this");
  return { projekt: slug, automatisch: an };
}

/** Täglich: Vorschläge verfallen nach 30 Tagen, Amtszeiten ohne Aktivität enden nach 21 Tagen. */
export async function aufraeumen(db: Db) {
  await db.query("update bereichs_vorschlaege set zustand = 'abgelaufen', entschieden = now() where zustand in ('offen', 'zugesagt') and erstellt < now() - make_interval(days => $1)",
    [BEREICHS_LEAD.vorschlagGueltigTage]);
  const ruhig = (await db.query(
    `select p.slug, o.bereich from officers o join projekte p on p.id = o.projekt_id
     where o.beendet is null and o.ernannt < now() - make_interval(days => $1)
       and coalesce((select max(x.zeit) from ereignisse x where x.projekt_id = o.projekt_id
                     and x.akteur in (select 'agent:' || a.id from agenten a where a.mensch_id = o.mensch_id)), o.ernannt) < now() - make_interval(days => $1)`,
    [BEREICHS_LEAD.inaktivEndeTage])).rows;
  for (const o of ruhig) await officerBeenden(db, "betrieb", o.slug, o.bereich, `no activity for ${BEREICHS_LEAD.inaktivEndeTage} days (ended automatically)`);
  return ruhig.length;
}

/** Für Mein Konto: eigene Einladungen, Vorschläge zur Freigabe, Automatik je eigenem Projekt. */
export async function bereichsLeadsFuer(db: Db, menschId: number) {
  const einladungen = (await db.query(
    `select v.id::int, p.slug as projekt, p.name as projekt_name, v.bereich, v.pfade, v.faehigkeiten, v.gruende, v.erstellt
     from bereichs_vorschlaege v join projekte p on p.id = v.projekt_id where v.mensch_id = $1 and v.zustand = 'offen' order by v.erstellt`, [menschId])).rows;
  const freigaben = (await db.query(
    `select v.id::int, p.slug as projekt, p.name as projekt_name, v.bereich, v.pfade, v.faehigkeiten, v.gruende, v.erstellt, m.login, m.vertrauensstufe, v.gesuch_id is not null as bewerbung
     from bereichs_vorschlaege v join projekte p on p.id = v.projekt_id join menschen m on m.id = v.mensch_id
     where v.zustand = 'zugesagt' and exists (select 1 from projekt_rollen r where r.projekt_id = v.projekt_id and r.mensch_id = $1 and r.rolle = 'visionaer')
     order by v.erstellt`, [menschId])).rows;
  const automatik = (await db.query(
    `select p.slug as projekt, p.name as projekt_name, p.bereichs_leads_automatisch as automatisch from projekte p
     where exists (select 1 from projekt_rollen r where r.projekt_id = p.id and r.mensch_id = $1 and r.rolle = 'visionaer') and not p.entwurf order by p.name`, [menschId])).rows;
  return { einladungen, freigaben, automatik };
}
