// Statusboard-Werte eines Projekts. Dieselbe Funktion liefert das Board
// und das MCP-Werkzeug `kennzahlen`, damit Menschen und Agenten denselben Stand sehen.
import { leadAnwesenheit } from "./leaddienst.ts";
import { offeneGesuche } from "./leadkraft.ts";
import { bedarfVon } from "./bedarf.ts";
import { nachweis } from "./cleanroom.ts";
import type { Db } from "./db.ts";
import { schwarmBilanz } from "./bilanz.ts";
import { folgeaufgaben, projektPerSlug } from "./aufgaben.ts";
import { ketteVerifizierenLaufend } from "./ereignisse.ts";
import { creditSpenden } from "./spenden.ts";
import { einreichungsVorschauUrl, konfig, vorschauUrl } from "./konfig.ts";
import { feedbackZahlen } from "./feedback.ts";
import { einreichungsVorschau, KATALOG_PROJEKT, katalogLaden } from "./katalog.ts";

const ERLEDIGT = new Set(["angenommen", "gemerged"]);

/** Schwellen, ab denen ein Bereichs-Lead für einen Bereich sinnvoll ist (lead/roles/README.md, „Bereichs-Leads“). */
export const OFFICER_SCHWELLEN = { offen_je_rolle: 15, einreichungen_je_rolle_30t: 10, warten_auf_bewertung: 10, antwort_h_median: 24, nachrichten_offen: 5 };

/** Lead-Bilanz der letzten 30 Tage, gesamt und je Rolle, mit Empfehlung für Bereichs-Leads. Lead und Website sehen dasselbe. */
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
  // Laufende Vorschläge (bereichsleads.ts): eingeladen oder zugesagt, wartet auf den Visionär
  const vorschlaege = (await db.query(
    `select v.bereich, m.login, case v.zustand when 'offen' then 'invited' else 'accepted, waiting for the visionary' end as status
     from bereichs_vorschlaege v join menschen m on m.id = v.mensch_id join projekte p on p.id = v.projekt_id
     where p.slug = $1 and v.zustand in ('offen', 'zugesagt') order by v.bereich`, [slug])).rows;
  const empfehlung = [];
  for (const z of zeilen) {
    if (z.rolle === "ohne" || besetzt.has(z.rolle)) continue;
    const gruende = [];
    // Viele offene Aufgaben zählen nur, wenn in der Rolle auch gearbeitet wird; ohne Bewegung fehlen Mitstreiter, kein Bereichs-Lead
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
  // Nachrichten an den Lead (ohne Inhalte): offen, und Antwortzeit der letzten 30 Tage
  const nachr = (await db.query(
    `select count(*) filter (where zustand = 'offen')::int as offen,
            round((percentile_cont(0.5) within group (order by antwort_s) filter (where erstellt > now() - interval '30 days') / 3600)::numeric, 1) as antwort_h
     from analytik.nachrichten where projekt = $1 and an = 'lead' and art <> 'info'`, [slug])).rows[0];
  const leadLast = warten > OFFICER_SCHWELLEN.warten_auf_bewertung || (n(g.antwort_h_median) ?? 0) > OFFICER_SCHWELLEN.antwort_h_median
    || nachr.offen > OFFICER_SCHWELLEN.nachrichten_offen || (n(nachr.antwort_h) ?? 0) > OFFICER_SCHWELLEN.antwort_h_median;
  const engpass = leadLast ? "lead" : offenGesamt > OFFICER_SCHWELLEN.offen_je_rolle && einreichungen30 < OFFICER_SCHWELLEN.einreichungen_je_rolle_30t ? "mitstreiter" : null;
  return {
    engstellen: await engstellen(db, slug),
    bewertet: n(g.bewertet), annahme_prozent: n(g.annahme_prozent), nacharbeit_prozent: n(g.nacharbeit_prozent), antwort_h_median: n(g.antwort_h_median),
    warten_auf_bewertung: warten,
    unberuehrt: zeilen.reduce((s, z) => s + Number(z.unberuehrt), 0),
    reservierungen_ohne_ergebnis: zeilen.reduce((s, z) => s + Number(z.reservierungen_ohne_ergebnis), 0),
    je_rolle: zeilen.map((z) => ({ rolle: z.rolle, offen: Number(z.offen), unberuehrt: Number(z.unberuehrt), warten_auf_bewertung: Number(z.warten_auf_bewertung),
      reservierungen_ohne_ergebnis: Number(z.reservierungen_ohne_ergebnis), einreichungen_30t: Number(z.einreichungen_30t),
      annahme_prozent: n(z.annahme_prozent), nacharbeit_prozent: n(z.nacharbeit_prozent), antwort_h_median: n(z.antwort_h_median) })),
    officers: officers.map((o) => ({ bereich: o.bereich, login: o.login, seit: o.seit })),
    officer_empfehlung: empfehlung,
    bereichs_vorschlaege: vorschlaege,
    nachrichten_offen: nachr.offen, nachrichten_antwort_h_median: n(nachr.antwort_h),
    engpass,
  };
}

/**
 * Engstellen: Dateien, die viele Aufgaben brauchen. Je Pfad (wie in den Aufgaben genannt, auch Globs) die offenen und
 * laufenden Aufgaben und die Konflikte oder roten Merge-Züge der letzten 30 Tage. Ab zwei Aufgaben oder einem Konflikt ein
 * Kandidat zum Umbauen (Daten, Registries; LEAD.md 6.12), weil parallele Aufgaben dort nacheinander laufen müssen.
 */
async function engstellen(db: Db, slug: string) {
  return (await db.query(
    `with p as (select id from projekte where slug = $1),
     offen as (select unnest(a.pfade) as pfad, a.id from aufgaben a, p where a.projekt_id = p.id
               and a.zustand in ('offen', 'reserviert', 'in_arbeit', 'eingereicht', 'pruefung', 'geprueft_ok', 'angenommen')),
     reibung as (select unnest(a.pfade) as pfad, count(*) as n from ereignisse ev join aufgaben a on ev.bezug = 'aufgabe:' || a.id, p
                 where a.projekt_id = p.id and ev.art in ('merge_konflikt', 'merge_zug_rot') and ev.zeit > now() - interval '30 days' group by 1)
     select coalesce(o.pfad, r.pfad) as pfad, count(distinct o.id)::int as aufgaben, coalesce(max(r.n), 0)::int as konflikte
     from offen o full join reibung r on r.pfad = o.pfad
     group by 1 having count(distinct o.id) >= 2 or coalesce(max(r.n), 0) >= 1
     order by 3 desc, 2 desc, 1 limit 8`, [slug])).rows;
}

/** Schwarm gegenüber einem einzelnen Agenten (analytik.schwarm): öffentlich, damit der Vorteil messbar ist, nicht behauptet. */
async function schwarmZahlen(db: Db, slug: string) {
  const z = (await db.query("select * from analytik.schwarm where projekt = $1", [slug])).rows[0];
  if (!z) return null;
  const n = (x: unknown) => (x === null || x === undefined ? null : Number(x));
  return { agenten_stunden: n(z.agenten_stunden), einzel_agent_stunden: n(z.einzel_agent_stunden), kalender_stunden: n(z.kalender_stunden),
           schwarm_faktor: n(z.schwarm_faktor), max_parallel: n(z.max_parallel), token_effizienz_prozent: n(z.token_effizienz_prozent) };
}

export async function kennzahlen(db: Db, slug: string) {
  const p = await projektPerSlug(db, slug);
  const aufgaben = (await db.query("select a.id, a.stufe, a.zustand, a.faehigkeiten, a.meilenstein_id from aufgaben a where a.projekt_id = $1 and a.zustand <> 'geschlossen'", [p.id])).rows;
  // Fortschritt (E-41): Leads planen rollierend, angelegte Aufgaben sind nur ein Teil. Gezählt wird erledigt von geplant;
  // einen Prozentwert gibt es nur gegen die Schätzung des Leads, wie viele Aufgaben ein Meilenstein insgesamt braucht.
  const zaehlung = new Map<number | null, { geplant: number; fertig: number }>();
  for (const a of aufgaben) {
    const schluessel = a.meilenstein_id === null ? null : Number(a.meilenstein_id), z = zaehlung.get(schluessel) ?? { geplant: 0, fertig: 0 };
    z.geplant++;
    if (ERLEDIGT.has(a.zustand)) z.fertig++;
    zaehlung.set(schluessel, z);
  }
  const ms = (await db.query("select id, titel, ziel, schaetzung, geschaetzt from meilensteine where projekt_id = $1 order by reihenfolge, id", [p.id])).rows;
  const meilensteine = ms.filter((m) => zaehlung.has(Number(m.id)) || m.schaetzung).map((m) => {
    const z = zaehlung.get(Number(m.id)) ?? { geplant: 0, fertig: 0 }, umfang = m.schaetzung ? Math.max(m.schaetzung, z.geplant) : null;
    return { titel: m.titel as string | null, ziel: m.ziel ?? null, fertig: z.fertig, geplant: z.geplant, schaetzung: m.schaetzung ?? null,
      fortschritt_prozent: umfang ? Math.round((100 * z.fertig) / umfang) : null, geschaetzt_am: m.geschaetzt ?? null };
  });
  const ohne = zaehlung.get(null);
  if (ohne) meilensteine.push({ titel: null, ziel: null, ...ohne, schaetzung: null, fortschritt_prozent: null, geschaetzt_am: null });
  const benannt = meilensteine.filter((m) => m.titel !== null);
  const gesamt = benannt.length && benannt.every((m) => m.schaetzung)
    ? Math.round((100 * benannt.reduce((s, m) => s + m.fertig, 0)) / benannt.reduce((s, m) => s + Math.max(m.schaetzung!, m.geplant), 0)) : null;
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
    projekt: p.slug, name: p.name, vision_kurz: p.vision_kurz, sprache: p.sprache, uebersetzungen: p.uebersetzungen ?? {}, vision_veroeffentlicht: vision?.zeit ?? null,
    lead_modell: p.lead_modell, engine: p.engine, lizenz: `${p.lizenz_code} / ${p.lizenz_assets}`, ziel: p.ziel,
    projektart: p.projektart ?? null, verfahren: p.verfahren, ...(p.forschung ? { forschung: p.forschung } : {}), ...(p.upstream ? { upstream: p.upstream } : {}),
    fortschritt_prozent: gesamt, aufgaben_fertig: aufgaben.filter((a) => ERLEDIGT.has(a.zustand)).length, aufgaben_geplant: aufgaben.length, meilensteine,
    fortschritt_berechnung: "done tasks ÷ the lead's estimate of all tasks a milestone needs (never fewer than the tasks planned so far); the total only when every milestone has an estimate, otherwise null and only done of planned",
    agenten_aktiv: aktiv.length, agenten_aktiv_nach_art: nachArt,
    mitstreiter_menschen: mitstreiter.menschen, mitstreiter_agenten: mitstreiter.agenten,
    aufgaben_offen: offen, spenden_monat: spenden, zuletzt: zuletzt ? { zeit: zuletzt.zeit, art: zuletzt.art, bezug: zuletzt.bezug } : null,
    lead_bilanz: await leadBilanz(db, p.slug),
    // Schwarm-Bilanz mit Qualität, Kosten, Lead-Last und Erfolg je Modell und Stufe: für den Lead dieselben Zahlen wie auf der Projektseite
    bilanz: await schwarmBilanz(db, Number(p.id)),
    schwarm: await schwarmZahlen(db, p.slug),
    credit_spenden: await creditSpenden(db, Number(p.id)),
  };
}

/** Alle Projekte mit Kennzahlen und Aktivität der letzten 7 Tage, für die Rangliste der Website. */
export async function projektListe(db: Db) {
  const liste = (await db.query("select slug from projekte where not gesperrt and not entwurf order by id")).rows;
  const projekte = await Promise.all(liste.map(async ({ slug }) => {
    const k = await kennzahlen(db, slug);
    const woche = (await db.query(
      `select
         (select coalesce(sum((e.verbrauch->>'tokens')::numeric), 0)::bigint from einreichungen e join aufgaben a on a.id = e.aufgabe_id
            where a.projekt_id = p.id and e.eingereicht >= now() - interval '7 days')
         -- dazu laufende Arbeit: der zuletzt gemeldete Verbrauch jeder aktiven Reservierung (report_progress)
         + (select coalesce(sum(t.tokens), 0)::bigint from (
              select (select (ev.daten->'verbrauch'->>'tokens')::numeric from ereignisse ev
                       where ev.art = 'fortschritt' and ev.bezug = 'aufgabe:' || l.aufgabe_id and ev.akteur = 'agent:' || l.agent_id and ev.zeit >= l.erstellt
                       order by ev.seq desc limit 1) as tokens
              from leases l join aufgaben a on a.id = l.aufgabe_id
              where a.projekt_id = p.id and l.beendet is null and l.laeuft_ab > now()) t) as tokens,
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
    const lead_gesucht = Number((await db.query("select count(*)::int as n from lead_gesuche g join projekte p on p.id = g.projekt_id where p.slug = $1 and g.geschlossen is null", [slug])).rows[0].n);
    return { ...k, sucht, lead_gesucht, woche: { tokens: Number(woche.tokens), einreichungen: woche.einreichungen, angenommen: woche.angenommen, agenten: woche.agenten } };
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
  const p = (await db.query("select id, vision_md, repo_url, verfahren, vorbild, vorschau, fortlaufend from projekte where slug = $1", [slug])).rows[0];
  const aufgaben = await Promise.all((await db.query(
    `select a.id, a.titel, a.stufe, a.zustand, a.faehigkeiten, a.raum, a.rolle, m.titel as meilenstein,
            aufgabe_wartet_auf(a.abhaengig_von, a.abhaengig_eins_von) as wartet_auf, a.ohne_fremdbewertung,
            (select e.id::int from einreichungen e where e.aufgabe_id = a.id and a.zustand = 'geprueft_ok' and e.vorschau->>'status' = 'ok' order by e.id desc limit 1) as vorschau_einreichung,
            w.agent as arbeitet, w.login as arbeitet_login, w.profil as arbeitet_profil, b.agent as beitrag_von, b.login as beitrag_login, b.profil as beitrag_profil
     from aufgaben a left join meilensteine m on m.id = a.meilenstein_id
     -- Wer gerade arbeitet (laufende Reservierung) und wer eingereicht hat (ab der Einreichung bis zur Übernahme); profil:
     -- hat die Person ein öffentliches Profil (sonst verlinkt die Website den Namen nicht)
     left join lateral (select agent_oeffentlich(ag.name) || coalesce(' (' || nullif(concat_ws(', ', l.agent_programm, l.modell), '') || ')', '') as agent, m2.login, m2.profil_oeffentlich as profil
               from leases l join agenten ag on ag.id = l.agent_id join menschen m2 on m2.id = ag.mensch_id
               where l.aufgabe_id = a.id and l.beendet is null and l.laeuft_ab > now() limit 1) w on true
     left join lateral (select agent_oeffentlich(ag.name) || coalesce(' (' || nullif(concat_ws(', ', e.agent_programm, e.modell), '') || ')', '') as agent, m3.login, m3.profil_oeffentlich as profil
               from einreichungen e join agenten ag on ag.id = e.agent_id join menschen m3 on m3.id = ag.mensch_id
               where e.aufgabe_id = a.id and a.zustand in ('eingereicht', 'pruefung', 'geprueft_ok', 'angenommen', 'gemerged') order by e.id desc limit 1) b on true
     where a.projekt_id = $1 and a.zustand <> 'geschlossen' order by a.id desc limit 50`, [p.id])).rows
    .map(async ({ vorschau_einreichung, ...a }) => ({ ...a, ...(vorschau_einreichung && einreichungsVorschauUrl(1) ? { vorschau_url: await einreichungsVorschau(db, slug, vorschau_einreichung) } : {}) })));
  // Beitragende: pro Mensch eingereicht und angenommen, mit den genutzten Agenten
  const beitragende = (await db.query(
    `select m.login as mensch, count(e.id)::int as eingereicht,
            count(b.id) filter (where b.urteil = 'annehmen')::int as angenommen,
            array_agg(distinct agent_oeffentlich(ag.name) || coalesce(' (' || nullif(concat_ws(', ', e.agent_programm, e.modell), '') || ')', '')) as agenten
     from einreichungen e join aufgaben a on a.id = e.aufgabe_id join agenten ag on ag.id = e.agent_id join menschen m on m.id = ag.mensch_id
     left join bewertungen b on b.einreichung_id = e.id
     where a.projekt_id = $1 group by m.login order by angenommen desc, eingereicht desc`, [p.id])).rows;
  const namen = new Map<string, string>();
  for (const z of (await db.query("select 'agent:' || a.id as k, agent_oeffentlich(a.name) || ' · ' || m.login as v from agenten a join menschen m on m.id = a.mensch_id union all select 'mensch:' || id, login from menschen")).rows) namen.set(z.k, z.v);
  const ereignisse = (await db.query("select seq, zeit, akteur, art, bezug from ereignisse where projekt_id = $1 order by seq desc limit 20", [p.id])).rows
    .map((e) => ({ seq: Number(e.seq), zeit: e.zeit, wer: namen.get(e.akteur) ?? e.akteur, art: e.art, bezug: e.bezug }));
  // Kritischer Pfad hinter einer Vertrauensstufe: offene, freie Aufgaben, die anderes freischalten, aber nicht für Neue sind
  const folge = await folgeaufgaben(db, Number(p.id));
  const gesperrt = new Map<number, number>();
  for (const a of (await db.query(
    `select id, min_vertrauen from aufgaben where projekt_id = $1 and zustand = 'offen' and min_vertrauen between 1 and 2
       and not aufgabe_blockiert(abhaengig_von, abhaengig_eins_von)`, [p.id])).rows) {
    if ((folge.get(Number(a.id)) ?? 0) > 0) gesperrt.set(Number(a.min_vertrauen), (gesperrt.get(Number(a.min_vertrauen)) ?? 0) + 1);
  }
  const kritisch_vertrauen = [...gesperrt].sort((x, y) => x[0] - y[0]).map(([stufe, anzahl]) => ({ vertrauensstufe: stufe, anzahl }));
  const kette = await ketteVerifizierenLaufend(db);   // nur Neues, voll einmal die Stunde
  const kopf = (await db.query("select seq, zeit from koepfe order by seq desc limit 1")).rows[0];
  const clean_room = p.verfahren === "clean_room" ? { vorbild: p.vorbild, ...(await nachweis(db, Number(p.id))) } : null;
  // Spielbare Vorschau: die zuletzt gelungene, dazu der Stand des letzten Baus (z. B. fehlgeschlagen)
  const v = p.vorschau;
  const vorschau = vorschauUrl(slug) && v && v.status !== "none"
    ? { url: v.ok_commit && v.hat_vorschau !== false ? vorschauUrl(slug) : null, docs_url: v.ok_commit && v.hat_doku ? `${vorschauUrl(slug)}docs/` : null, ok_commit: v.ok_commit, ok_zeit: v.ok_zeit, commit: v.commit, status: v.status, zeit: v.zeit,
        rueckmeldungen: await feedbackZahlen(db, Number(p.id)) } : null;
  // Fortlaufend (migrations/057): Bestand, Anfragen und Galerie statt Fortschritt und Schwarm-Bilanz
  const laufend = p.fortlaufend ? await laufendeZahlen(db, Number(p.id), slug) : null;
  return { ...k, fortlaufend: p.fortlaufend, laufend, vision_md: p.vision_md, repo_url: p.repo_url, verfahren: p.verfahren, clean_room, vorschau, lead_dienst: await leadAnwesenheit(db, Number(p.id)), lead_gesuche: await offeneGesuche(db, Number(p.id)), bedarf: await bedarfVon(db, Number(p.id)), aufgaben, kritisch_vertrauen, beitragende, ereignisse, bilanz: await schwarmBilanz(db, Number(p.id)),
           protokoll: { ok: kette.ok, eintraege: kette.anzahl, kopf: kette.kopf,
                        veroeffentlicht: kopf ? { seq: Number(kopf.seq), zeit: kopf.zeit } : null } };
}

/** Für fortlaufende Projekte: was diese Woche dazukam, Anfragen anderer Projekte und beim Katalog sein Bestand. */
async function laufendeZahlen(db: Db, projektId: number, slug: string) {
  const diese_woche = Number((await db.query(
    "select count(*) from ereignisse where projekt_id = $1 and art = 'gemerged' and zeit > now() - interval '7 days'", [projektId])).rows[0].count);
  const anfragen = (await db.query(
    `select count(*) filter (where zustand in ('offen', 'angenommen'))::int as offen, count(*) filter (where zustand = 'erledigt')::int as beantwortet
     from bedarf where an_projekt = $1`, [projektId])).rows[0];
  let katalog: { assets: number; arten: number } | null = null;
  if (slug === KATALOG_PROJEKT) {
    const k = await katalogLaden(db);
    if (k.verfuegbar) katalog = { assets: k.eintraege.length, arten: new Set(k.eintraege.map((e) => e.art)).size };
  }
  return { katalog, diese_woche, anfragen };
}

/** Live-Ansicht der Startseite: die letzten öffentlichen Ereignisse aller Projekte, ohne Namen (nur Art, Projekt, Zeit). */
const LIVE_ARTEN = ["aufgabe_angelegt", "aufgabe_reserviert", "ergebnis_eingereicht", "fremdpruefung", "bewertet", "gemerged", "vorschau_gebaut"];
export async function schwarmLive(db: Db) {
  return (await db.query(
    `select e.seq::int, e.zeit, e.art, p.slug as projekt, p.name,
            case when e.art = 'bewertet' then e.daten->>'urteil' when e.art in ('fremdpruefung') then e.daten->>'urteil' else null end as urteil
     from ereignisse e join projekte p on p.id = e.projekt_id
     where e.art = any($1) and not p.gesperrt and not p.entwurf and e.zeit > now() - interval '7 days'
     order by e.seq desc limit 12`, [LIVE_ARTEN])).rows;
}

/** Abzeichen für die README eines Projekts: „▶ play“, wenn es eine Vorschau gibt, sonst der geschätzte Fortschritt oder erledigt von geplant. */
export async function abzeichen(db: Db, slug: string) {
  const p = (await db.query("select id, vorschau from projekte where slug = $1 and not gesperrt and not entwurf", [slug])).rows[0];
  const v = p?.vorschau, da = Boolean(v?.ok_commit && vorschauUrl(slug));
  let rechts: string;
  if (!p) rechts = "unknown";
  else if (da && v.hat_vorschau !== false) rechts = "▶ play";
  else if (da && v.hat_doku) rechts = "docs";
  else {
    // The lead's estimate when there is one; otherwise only done of planned, never a percentage of an unknown whole
    const z = await kennzahlen(db, slug);
    rechts = z.fortschritt_prozent !== null ? `≈ ${z.fortschritt_prozent} %` : `${z.aufgaben_fertig}/${z.aufgaben_geplant} tasks`;
  }
  const farbe = rechts === "▶ play" || rechts === "docs" ? "#047857" : rechts === "unknown" ? "#6b7280" : "#0369a1";
  const breite = (t: string) => Math.round([...t].length * 6.6 + 12);
  const l = "swarvex", bl = breite(l), br = breite(rechts);
  const esc = (t: string) => t.replace(/&/g, "&amp;").replace(/</g, "&lt;");
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${bl + br}" height="20" role="img" aria-label="swarvex: ${esc(rechts)}">
<title>swarvex: ${esc(rechts)}</title><linearGradient id="g" x2="0" y2="100%"><stop offset="0" stop-color="#fff" stop-opacity=".12"/><stop offset="1" stop-opacity=".1"/></linearGradient>
<clipPath id="r"><rect width="${bl + br}" height="20" rx="3"/></clipPath><g clip-path="url(#r)"><rect width="${bl}" height="20" fill="#1f2937"/><rect x="${bl}" width="${br}" height="20" fill="${farbe}"/><rect width="${bl + br}" height="20" fill="url(#g)"/></g>
<g fill="#fff" text-anchor="middle" font-family="Verdana,DejaVu Sans,sans-serif" font-size="11"><text x="${bl / 2}" y="14">${l}</text><text x="${bl + br / 2}" y="14">${esc(rechts)}</text></g></svg>`;
}

/** Kurzlink swarvex.run/<slug>: zur Vorschau, wenn es eine gibt, sonst zur Projektseite. */
export async function kurzlinkZiel(db: Db, slug: string, en: boolean) {
  const p = (await db.query("select vorschau from projekte where slug = $1 and not gesperrt and not entwurf", [slug])).rows[0];
  if (!p) return `${konfig.basis}${en ? "/en/projects" : "/projekte"}`;
  const v = p.vorschau;
  if (v?.ok_commit && vorschauUrl(slug)) return v.hat_vorschau !== false ? vorschauUrl(slug) : `${vorschauUrl(slug)}docs/`;
  return `${konfig.basis}${en ? "/en/projects" : "/projekte"}/${slug}`;
}
