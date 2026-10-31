// Statusboard-Werte eines Projekts. Dieselbe Funktion liefert das Board
// und das MCP-Werkzeug `kennzahlen`, damit Menschen und Agenten denselben Stand sehen.
import { nachweis } from "./cleanroom.ts";
import type { Db } from "./db.ts";
import { STUFEN_GEWICHT, projektPerSlug } from "./aufgaben.ts";
import { ketteVerifizieren } from "./ereignisse.ts";

const ERLEDIGT = new Set(["angenommen", "gemerged"]);

/** Schwellen, ab denen ein Officer für einen Bereich sinnvoll ist (lead/roles/README.md, „Officers“). */
export const OFFICER_SCHWELLEN = { offen_je_rolle: 15, einreichungen_je_rolle_30t: 10, warten_auf_bewertung: 10, antwort_h_median: 24 };

/** Lead-Bilanz der letzten 30 Tage, gesamt und je Rolle, mit Empfehlung für Officers. Lead und Website sehen dasselbe. */
export async function leadBilanz(db: Db, slug: string) {
  const zeilen = (await db.query("select * from analytik.lead_bilanz where projekt = $1 order by offen desc", [slug])).rows;
  const g = (await db.query(
    `select count(*) filter (where urteil is not null) as bewertet,
            round(100.0 * count(*) filter (where urteil = 'annehmen') / nullif(count(*) filter (where urteil is not null), 0)) as annahme_prozent,
            round(100.0 * count(*) filter (where urteil = 'nacharbeit') / nullif(count(*) filter (where urteil is not null), 0)) as nacharbeit_prozent,
            round((percentile_cont(0.5) within group (order by lead_reaktion_s) / 3600)::numeric, 1) as antwort_h_median
     from analytik.einreichungen where projekt = $1 and zeit > now() - interval '30 days'`, [slug])).rows[0];
  const n = (x: unknown) => (x === null || x === undefined ? null : Number(x));
  const warten = zeilen.reduce((s, z) => s + Number(z.warten_auf_bewertung), 0);
  const officers = (await db.query(
    "select o.bereich, o.ernannt as seit, m.login from officers o join menschen m on m.id = o.mensch_id join projekte p on p.id = o.projekt_id where p.slug = $1 and o.beendet is null order by o.bereich", [slug])).rows;
  const besetzt = new Set(officers.map((o) => o.bereich));
  const empfehlung = [];
  for (const z of zeilen) {
    if (z.rolle === "ohne" || besetzt.has(z.rolle)) continue;
    const gruende = [];
    // Viele offene Aufgaben zählen nur, wenn in der Rolle auch gearbeitet wird; ohne Bewegung fehlen Mitstreiter, kein Officer
    const inBewegung = Number(z.einreichungen_30t) >= OFFICER_SCHWELLEN.einreichungen_je_rolle_30t;
    if (Number(z.offen) > OFFICER_SCHWELLEN.offen_je_rolle && inBewegung) gruende.push(`${z.offen} open tasks and ${z.einreichungen_30t} submissions in 30 days in this role (thresholds ${OFFICER_SCHWELLEN.offen_je_rolle} and ${OFFICER_SCHWELLEN.einreichungen_je_rolle_30t})`);
    if (Number(z.warten_auf_bewertung) > OFFICER_SCHWELLEN.warten_auf_bewertung / 2) gruende.push(`${z.warten_auf_bewertung} submissions in this role await review`);
    if (n(z.antwort_h_median) !== null && n(z.antwort_h_median)! > OFFICER_SCHWELLEN.antwort_h_median) gruende.push(`median response ${z.antwort_h_median} h in this role (threshold ${OFFICER_SCHWELLEN.antwort_h_median} h)`);
    const gesamtLast = warten > OFFICER_SCHWELLEN.warten_auf_bewertung || (n(g.antwort_h_median) ?? 0) > OFFICER_SCHWELLEN.antwort_h_median;
    if (gruende.length && (gesamtLast || (Number(z.offen) > OFFICER_SCHWELLEN.offen_je_rolle && inBewegung))) empfehlung.push({ rolle: z.rolle, gruende });
  }
  // Der eigentliche Engpass, ehrlich benannt: Lead (Bewertungen stauen sich) oder Mitstreiter (Aufgaben liegen, niemand arbeitet)
  const offenGesamt = zeilen.reduce((s, z) => s + Number(z.offen), 0);
  const einreichungen30 = zeilen.reduce((s, z) => s + Number(z.einreichungen_30t), 0);
  const leadLast = warten > OFFICER_SCHWELLEN.warten_auf_bewertung || (n(g.antwort_h_median) ?? 0) > OFFICER_SCHWELLEN.antwort_h_median;
  const engpass = leadLast ? "lead" : offenGesamt > OFFICER_SCHWELLEN.offen_je_rolle && einreichungen30 < OFFICER_SCHWELLEN.einreichungen_je_rolle_30t ? "mitstreiter" : null;
  return {
    bewertet: n(g.bewertet), annahme_prozent: n(g.annahme_prozent), nacharbeit_prozent: n(g.nacharbeit_prozent), antwort_h_median: n(g.antwort_h_median),
    warten_auf_bewertung: warten,
    unberuehrt: zeilen.reduce((s, z) => s + Number(z.unberuehrt), 0),
    reservierungen_ohne_ergebnis: zeilen.reduce((s, z) => s + Number(z.reservierungen_ohne_ergebnis), 0),
    je_rolle: zeilen.map((z) => ({ rolle: z.rolle, offen: Number(z.offen), unberuehrt: Number(z.unberuehrt), warten_auf_bewertung: Number(z.warten_auf_bewertung),
      reservierungen_ohne_ergebnis: Number(z.reservierungen_ohne_ergebnis), einreichungen_30t: Number(z.einreichungen_30t),
      annahme_prozent: n(z.annahme_prozent), nacharbeit_prozent: n(z.nacharbeit_prozent), antwort_h_median: n(z.antwort_h_median) })),
    officers: officers.map((o) => ({ bereich: o.bereich, login: o.login, seit: o.seit })),
    officer_empfehlung: empfehlung,
    engpass,
  };
}

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
    lead_bilanz: await leadBilanz(db, p.slug),
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
    `select a.id, a.titel, a.stufe, a.zustand, a.faehigkeiten, a.raum, a.rolle, m.titel as meilenstein,
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
