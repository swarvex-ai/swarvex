// Statusboard-Werte eines Projekts. Dieselbe Funktion liefert das Board
// und das MCP-Werkzeug `kennzahlen`, damit Menschen und Agenten denselben Stand sehen.
import { nachweis } from "./cleanroom.ts";
import type { Db } from "./db.ts";
import { STUFEN_GEWICHT, projektPerSlug } from "./aufgaben.ts";
import { ketteVerifizieren } from "./ereignisse.ts";

const ERLEDIGT = new Set(["angenommen", "gemerged"]);

export async function kennzahlen(db: Db, slug: string) {
  const p = await projektPerSlug(db, slug);
  const aufgaben = (await db.query("select a.id, a.stufe, a.zustand, a.faehigkeiten, m.titel as meilenstein from aufgaben a left join meilensteine m on m.id = a.meilenstein_id where a.projekt_id = $1 and a.zustand <> 'geschlossen'", [p.id])).rows;
  const gruppen = new Map<string, { gesamt: number; fertig: number }>();
  for (const a of aufgaben) {
    const g = gruppen.get(a.meilenstein ?? "") ?? { gesamt: 0, fertig: 0 };
    g.gesamt += STUFEN_GEWICHT[a.stufe];
    if (ERLEDIGT.has(a.zustand)) g.fertig += STUFEN_GEWICHT[a.stufe];
    gruppen.set(a.meilenstein ?? "", g);
  }
  const meilensteine = [...gruppen].map(([titel, g]) => ({ titel: titel || null, fortschritt_prozent: g.gesamt ? Math.round((100 * g.fertig) / g.gesamt) : 0 }));
  const gesamt = meilensteine.length ? Math.round(meilensteine.reduce((s, m) => s + m.fortschritt_prozent, 0) / meilensteine.length) : 0;
  const aktiv = (await db.query(
    "select a.faehigkeiten from leases l join aufgaben a on a.id = l.aufgabe_id where a.projekt_id = $1 and l.beendet is null and l.laeuft_ab > now()", [p.id])).rows;
  const nachArt: Record<string, number> = {};
  for (const l of aktiv) { const art = (l.faehigkeiten[0] ?? "other").split(".")[0]; nachArt[art] = (nachArt[art] ?? 0) + 1; }
  const mitstreiter = (await db.query(
    "select count(distinct r.mensch_id)::int as menschen, (select count(distinct l.agent_id)::int from leases l join aufgaben a on a.id = l.aufgabe_id where a.projekt_id = $1) as agenten from projekt_rollen r where r.projekt_id = $1", [p.id])).rows[0];
  const vision = (await db.query("select zeit from ereignisse where projekt_id = $1 and art = 'vision_veroeffentlicht' order by seq limit 1", [p.id])).rows[0];
  const zuletzt = (await db.query("select zeit, art, bezug, daten from ereignisse where projekt_id = $1 order by seq desc limit 1", [p.id])).rows[0];
  const offen = aufgaben.filter((a) => a.zustand === "offen").length;
  // Gespendete Rechenleistung im laufenden Monat (Verbrauchsmeldungen der Runner).
  const spenden = (await db.query(
    `select coalesce(sum((e.verbrauch->>'tokens')::numeric), 0)::bigint as tokens,
            coalesce(sum((e.verbrauch->>'anfragen')::numeric), 0)::bigint as anfragen,
            coalesce(round(sum((e.verbrauch->>'sekunden')::numeric)), 0)::bigint as sekunden
     from einreichungen e join aufgaben a on a.id = e.aufgabe_id
     where a.projekt_id = $1 and e.eingereicht >= date_trunc('month', now())`, [p.id])).rows[0];
  return {
    projekt: p.slug, name: p.name, vision_kurz: p.vision_kurz, vision_veroeffentlicht: vision?.zeit ?? null,
    lead_modell: p.lead_modell, engine: p.engine, lizenz: `${p.lizenz_code} / ${p.lizenz_assets}`, ziel: p.ziel,
    fortschritt_prozent: gesamt, meilensteine,
    fortschritt_berechnung: "accepted tasks ÷ all tasks per milestone, weighted A=1 B=2 C=3 D=5; total = mean of the milestones",
    agenten_aktiv: aktiv.length, agenten_aktiv_nach_art: nachArt,
    mitstreiter_menschen: mitstreiter.menschen, mitstreiter_agenten: mitstreiter.agenten,
    aufgaben_offen: offen, spenden_monat: spenden, zuletzt: zuletzt ? { zeit: zuletzt.zeit, art: zuletzt.art, bezug: zuletzt.bezug } : null,
  };
}

/** Alle Projekte mit Kennzahlen und Aktivität der letzten 7 Tage, für die Rangliste der Website. */
export async function projektListe(db: Db) {
  const liste = (await db.query("select slug from projekte where not gesperrt order by id")).rows;
  const projekte = await Promise.all(liste.map(async ({ slug }) => {
    const k = await kennzahlen(db, slug);
    const woche = (await db.query(
      `select
         (select coalesce(sum((e.verbrauch->>'tokens')::numeric), 0)::bigint from einreichungen e join aufgaben a on a.id = e.aufgabe_id
            where a.projekt_id = p.id and e.eingereicht >= now() - interval '7 days') as tokens,
         (select count(*)::int from einreichungen e join aufgaben a on a.id = e.aufgabe_id
            where a.projekt_id = p.id and e.eingereicht >= now() - interval '7 days') as einreichungen,
         (select count(*)::int from bewertungen b join einreichungen e on e.id = b.einreichung_id join aufgaben a on a.id = e.aufgabe_id
            where a.projekt_id = p.id and b.urteil = 'annehmen' and b.erstellt >= now() - interval '7 days') as angenommen,
         (select count(distinct l.agent_id)::int from leases l join aufgaben a on a.id = l.aufgabe_id
            where a.projekt_id = p.id and l.erstellt >= now() - interval '7 days') as agenten
       from projekte p where p.slug = $1`, [slug])).rows[0];
    // welche Fähigkeiten die offenen Aufgaben verlangen („sucht: code.gdscript, asset.3d“)
    const sucht = (await db.query(
      `select f, count(*)::int as n from aufgaben a join projekte p on p.id = a.projekt_id, unnest(a.faehigkeiten) f
       where p.slug = $1 and a.zustand = 'offen' group by f order by n desc limit 5`, [slug])).rows.map((r) => r.f as string);
    return { ...k, sucht, woche: { tokens: Number(woche.tokens), einreichungen: woche.einreichungen, angenommen: woche.angenommen, agenten: woche.agenten } };
  }));
  return {
    stand: new Date().toISOString(),
    summe: {
      projekte: projekte.length,
      agenten_aktiv: projekte.reduce((s, p) => s + p.agenten_aktiv, 0),
      tokens_woche: projekte.reduce((s, p) => s + p.woche.tokens, 0),
      angenommen_woche: projekte.reduce((s, p) => s + p.woche.angenommen, 0),
    },
    projekte,
  };
}

/** Alles für die Projektseite der Website: Kennzahlen, Vision, Aufgaben, Beitragende, Protokoll. */
export async function projektDetail(db: Db, slug: string) {
  const k = await kennzahlen(db, slug);
  const p = (await db.query("select id, vision_md, repo_url, verfahren, vorbild from projekte where slug = $1", [slug])).rows[0];
  const aufgaben = (await db.query(
    `select a.id, a.titel, a.stufe, a.zustand, a.faehigkeiten, a.raum, m.titel as meilenstein,
            (select ag.name || ' · ' || m2.login from leases l join agenten ag on ag.id = l.agent_id join menschen m2 on m2.id = ag.mensch_id
               where l.aufgabe_id = a.id and l.beendet is null and l.laeuft_ab > now() limit 1) as arbeitet,
            (select ag.name || ' · ' || m3.login from einreichungen e join agenten ag on ag.id = e.agent_id join menschen m3 on m3.id = ag.mensch_id
               where e.aufgabe_id = a.id and a.zustand in ('angenommen', 'gemerged') order by e.id desc limit 1) as beitrag_von
     from aufgaben a left join meilensteine m on m.id = a.meilenstein_id
     where a.projekt_id = $1 and a.zustand <> 'geschlossen' order by a.id desc limit 50`, [p.id])).rows;
  // Beitragende: pro Mensch eingereicht und angenommen, mit den genutzten Agenten
  const beitragende = (await db.query(
    `select m.login as mensch, count(e.id)::int as eingereicht,
            count(b.id) filter (where b.urteil = 'annehmen')::int as angenommen,
            array_agg(distinct ag.name) as agenten
     from einreichungen e join aufgaben a on a.id = e.aufgabe_id join agenten ag on ag.id = e.agent_id join menschen m on m.id = ag.mensch_id
     left join bewertungen b on b.einreichung_id = e.id
     where a.projekt_id = $1 group by m.login order by angenommen desc, eingereicht desc`, [p.id])).rows;
  const namen = new Map<string, string>();
  for (const z of (await db.query("select 'agent:' || a.id as k, a.name || ' · ' || m.login as v from agenten a join menschen m on m.id = a.mensch_id union all select 'mensch:' || id, login from menschen")).rows) namen.set(z.k, z.v);
  const ereignisse = (await db.query("select seq, zeit, akteur, art, bezug from ereignisse where projekt_id = $1 order by seq desc limit 20", [p.id])).rows
    .map((e) => ({ seq: Number(e.seq), zeit: e.zeit, wer: namen.get(e.akteur) ?? e.akteur, art: e.art, bezug: e.bezug }));
  const kette = await ketteVerifizieren(db);
  const kopf = (await db.query("select seq, zeit from koepfe order by seq desc limit 1")).rows[0];
  const clean_room = p.verfahren === "clean_room" ? { vorbild: p.vorbild, ...(await nachweis(db, Number(p.id))) } : null;
  return { ...k, vision_md: p.vision_md, repo_url: p.repo_url, verfahren: p.verfahren, clean_room, aufgaben, beitragende, ereignisse,
           protokoll: { ok: kette.ok, eintraege: kette.anzahl, kopf: kette.kopf,
                        veroeffentlicht: kopf ? { seq: Number(kopf.seq), zeit: kopf.zeit } : null } };
}
