// Lead-Kraft (E-39, Entscheidung neVs 2026-11-07): mehr Lead für Projekte, die es brauchen, immer einer Person zugeordnet.
//   - Teilen-Link je Projekt (/join/<projekt>): gibt keine Rechte, zeigt Vision, offene Arbeit und offene Lead-Gesuche.
//   - Lead-Gesuch: Lead oder Visionär (oder sein Lead-Dienst) sucht einen Bereichs-Lead für einen Bereich. Bewerben kann
//     sich jeder mit eigenem Konto, auch ohne Vertrauensstufe 2 und mit jedem Modell; der Visionär entscheidet unter Mein
//     Konto. Die Bewerbung ist ein Bereichs-Lead-Vorschlag (bereichsleads.ts), den die Person selbst anstößt.
//   - Lead-Kraft-Hinweis: Halten die drei Signale drei Tage an (Backlog leer, während Agenten da sind; geprüfte
//     Einreichungen warten auf den Lead; D-Aufgaben stauen sich oder Runden stoßen ans Zeitlimit), schlägt die Plattform
//     Lead und Visionär vor, jemanden einzuladen oder selbst einen zweiten Lead-Agenten (anderes Modell) anzubinden.
import { transaktion, type Db } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { Abgelehnt, officersVon, projektPerSlug, rollenVon, type Wer } from "./aufgaben.ts";
import { angebot } from "./fluss.ts";
import { konfig } from "./konfig.ts";

const NAME = /^[a-z][a-z0-9-]{1,30}$/;
const RAEUME = ["analyse", "pruefung", "bau"] as const;
type Raum = (typeof RAEUME)[number];

export const LEAD_KRAFT = { tage: 3, abstandTage: 7, wartenStunden: 24, dStau: 3, dAlterTage: 3, zeitlimitRunden: 2 } as const;

/** Der Teilen-Link eines Projekts. Er gibt keine Rechte: Wer mitmacht, meldet sich mit eigenem Konto an. */
export const teilenLink = (slug: string) => `${konfig.basis}/join/${slug}`;

async function mussLeadSein(db: Db, menschId: number, projektId: number) {
  const rollen = await rollenVon(db, menschId, projektId);
  if (!rollen.includes("lead") && !rollen.includes("visionaer")) throw new Abgelehnt("only the project's lead or visionary opens or closes lead positions");
}

/** Lead oder Visionär: sucht einen Bereichs-Lead für einen Bereich (ersetzt ein offenes Gesuch desselben Bereichs). */
export async function gesuchOeffnen(db: Db, wer: Wer, slug: string, g: { bereich: string; pfade: string[]; faehigkeiten: string[]; raum?: string; text: string }) {
  if (!NAME.test(g.bereich)) throw new Abgelehnt("area: lowercase letters, digits and dashes, e.g. netcode or ui");
  if (!g.pfade.length || g.pfade.some((x) => x.includes("..") || x.startsWith("/") || !x.trim() || x === "**" || x === "*"))
    throw new Abgelehnt("paths: relative globs of the area inside the repository, not the whole repository");
  if (!g.faehigkeiten.length) throw new Abgelehnt("capabilities: at least one, e.g. code.gdscript or test.*");
  const text = g.text.trim();
  if (text.length < 20 || text.length > 2000) throw new Abgelehnt("text: 20 to 2000 characters on what the area needs and what the area lead decides");
  return transaktion(db, async (tx) => {
    const p = await projektPerSlug(tx, slug);
    await mussLeadSein(tx as unknown as Db, wer.menschId, Number(p.id));
    if (p.verfahren === "clean_room") {
      if (!RAEUME.includes(g.raum as Raum)) throw new Abgelehnt("clean-room project: the position needs a room (analysis, review or build)");
    } else if (g.raum) throw new Abgelehnt("rooms only exist in clean-room projects");
    const besetzt = (await tx.query(
      "select m.login from officers o join menschen m on m.id = o.mensch_id where o.projekt_id = $1 and o.bereich = $2 and o.beendet is null", [p.id, g.bereich])).rows[0];
    if (besetzt) throw new Abgelehnt(`${g.bereich} already has an area lead (${besetzt.login}); end that term first`);
    await tx.query("update lead_gesuche set geschlossen = now() where projekt_id = $1 and bereich = $2 and geschlossen is null", [p.id, g.bereich]);
    const id = Number((await tx.query(
      "insert into lead_gesuche(projekt_id, bereich, pfade, faehigkeiten, raum, text, von_mensch) values ($1,$2,$3,$4,$5,$6,$7) returning id",
      [p.id, g.bereich, g.pfade, g.faehigkeiten, g.raum ?? null, text, wer.menschId])).rows[0].id);
    await protokollieren(tx, { projekt_id: Number(p.id), akteur: `mensch:${wer.menschId}`, art: "lead_gesuch_geoeffnet", bezug: `projekt:${p.id}`,
      daten: { bereich: g.bereich, pfade: g.pfade, faehigkeiten: g.faehigkeiten } });
    return { gesuch_id: id, bereich: g.bereich, link: teilenLink(p.slug),
      hinweis: "Share the link. Applicants use their own account; the visionary decides under My account." };
  });
}

/** Schließt das offene Gesuch eines Bereichs; laufende Bewerbungen darauf verfallen. */
export async function gesuchSchliessen(db: Db, wer: Wer, slug: string, bereich: string) {
  return transaktion(db, async (tx) => {
    const p = await projektPerSlug(tx, slug);
    await mussLeadSein(tx as unknown as Db, wer.menschId, Number(p.id));
    const g = (await tx.query("update lead_gesuche set geschlossen = now() where projekt_id = $1 and bereich = $2 and geschlossen is null returning id", [p.id, bereich])).rows[0];
    if (!g) throw new Abgelehnt(`no open lead position for ${bereich}`);
    await gesuchBeenden(tx as unknown as Db, Number(g.id));
    await protokollieren(tx, { projekt_id: Number(p.id), akteur: `mensch:${wer.menschId}`, art: "lead_gesuch_geschlossen", bezug: `projekt:${p.id}`, daten: { bereich } });
    return { bereich, geschlossen: true };
  });
}

/** Laufende Bewerbungen eines geschlossenen oder besetzten Gesuchs verfallen. */
export async function gesuchBeenden(db: Db, gesuchId: number) {
  await db.query("update lead_gesuche set geschlossen = coalesce(geschlossen, now()) where id = $1", [gesuchId]);
  await db.query("update bereichs_vorschlaege set zustand = 'abgelaufen', entschieden = now() where gesuch_id = $1 and zustand in ('offen', 'zugesagt')", [gesuchId]);
}

/** Offene Gesuche eines Projekts (öffentlich: Projektseite und Teilen-Link). */
export async function offeneGesuche(db: Db, projektId: number) {
  return (await db.query(
    `select g.id::int, g.bereich, g.pfade, g.faehigkeiten, g.raum, g.text, g.erstellt,
            (select count(*)::int from bereichs_vorschlaege v where v.gesuch_id = g.id and v.zustand in ('offen', 'zugesagt')) as bewerbungen
     from lead_gesuche g where g.projekt_id = $1 and g.geschlossen is null order by g.erstellt`, [projektId])).rows;
}

/** Jemand bewirbt sich mit eigenem Konto als Bereichs-Lead auf ein offenes Gesuch. */
export async function bewerben(db: Db, menschId: number, gesuchId: number, b: { text: string; modelle: string }) {
  const text = b.text.trim(), modelle = b.modelle.trim();
  if (text.length < 20 || text.length > 2000) throw new Abgelehnt("text: 20 to 2000 characters on why you and how you would lead the area");
  if (modelle.length > 200) throw new Abgelehnt("models: at most 200 characters");
  return transaktion(db, async (tx) => {
    const g = (await tx.query("select g.*, p.slug, p.gesperrt from lead_gesuche g join projekte p on p.id = g.projekt_id where g.id = $1", [gesuchId])).rows[0];
    if (!g || g.geschlossen || g.gesperrt) throw new Abgelehnt("this lead position is no longer open");
    const m = (await tx.query("select login, gesperrt from menschen where id = $1", [menschId])).rows[0];
    if (m.gesperrt) throw new Abgelehnt("your account is blocked");
    const rollen = await rollenVon(tx, menschId, Number(g.projekt_id));
    if (rollen.includes("lead") || rollen.includes("visionaer")) throw new Abgelehnt("you already lead this project: take the area with a second lead session instead (take_area)");
    if ((await officersVon(tx, menschId, Number(g.projekt_id))).some((o) => o.bereich === g.bereich)) throw new Abgelehnt("you already lead this area");
    let id: number;
    try {
      id = Number((await tx.query(
        `insert into bereichs_vorschlaege(projekt_id, bereich, mensch_id, pfade, faehigkeiten, gruende, zustand, gesuch_id)
         values ($1,$2,$3,$4,$5,$6,'zugesagt',$7) returning id`,
        [g.projekt_id, g.bereich, menschId, g.pfade, g.faehigkeiten, JSON.stringify([{ bewerbung: text, modelle }]), gesuchId])).rows[0].id);
    } catch (e) {
      if ((e as { code?: string }).code === "23505") throw new Abgelehnt("you have already applied for this position");
      throw e;
    }
    await protokollieren(tx, { projekt_id: Number(g.projekt_id), akteur: `mensch:${menschId}`, art: "bereichs_lead_beworben", bezug: `mensch:${menschId}`, daten: { bereich: g.bereich } });
    await tx.query(
      `insert into nachrichten(projekt_id, von_mensch, von_agent, an, art, betreff, inhalt, aufgaben, zustand)
       values ($1, null, null, 'visionaer', 'info', $2, $3, '{}', 'erledigt')`,
      [g.projekt_id, `Application as area lead for ${g.bereich}: ${m.login}`,
        `${m.login} applies to lead the area ${g.bereich} of ${g.slug}${modelle ? ` (brings: ${modelle})` : ""}:\n\n${text}\n\nDecide under My account on the website.`]);
    return { bewerbung_id: id, state: "applied, waiting for the visionary" };
  });
}

/** Eigene Bewerbungen (Mein Konto). */
export async function meineBewerbungen(db: Db, menschId: number) {
  return (await db.query(
    `select v.id::int, p.slug as projekt, p.name as projekt_name, v.bereich, v.erstellt, v.entschieden,
            case v.zustand when 'zugesagt' then 'waiting' when 'ernannt' then 'appointed' when 'verworfen' then 'declined' else 'expired' end as status
     from bereichs_vorschlaege v join projekte p on p.id = v.projekt_id
     where v.mensch_id = $1 and v.gesuch_id is not null order by v.erstellt desc limit 20`, [menschId])).rows;
}

/** Signale eines Projekts heute: Backlog leer, während Agenten da sind; geprüfte Einreichungen warten seit einem Tag auf
 *  den Lead; D-Aufgaben stauen sich oder Runden des Lead-Dienstes stoßen ans Zeitlimit. */
export async function leadKraftSignale(db: Db, projektId: number) {
  const a = await angebot(db, projektId);
  const r = (await db.query(
    `select
       (select count(distinct l.agent_id)::int from leases l join aufgaben a on a.id = l.aufgabe_id
          where a.projekt_id = $1 and l.erstellt > now() - interval '3 days') as agenten,
       exists (select 1 from aufgaben a join einreichungen e on e.id = (select max(id) from einreichungen where aufgabe_id = a.id)
          where a.projekt_id = $1 and a.zustand = 'geprueft_ok' and e.eingereicht < now() - make_interval(hours => $2)) as wartet,
       (select count(*)::int from aufgaben a where a.projekt_id = $1 and a.stufe = 'D' and a.zustand = 'offen'
          and a.erstellt < now() - make_interval(days => $3)) as d_offen,
       (select count(*)::int from lead_runden r where r.projekt_id = $1 and r.erstellt > now() - interval '1 day'
          and r.ergebnis = 'time limit reached') as zeitlimit`,
    [projektId, LEAD_KRAFT.wartenStunden, LEAD_KRAFT.dAlterTage])).rows[0];
  return {
    backlog_leer: a.frei === 0 && (a.aktiv > 0 || r.agenten > 0),
    wartet_auf_lead: Boolean(r.wartet),
    d_stau: r.d_offen >= LEAD_KRAFT.dStau || r.zeitlimit >= LEAD_KRAFT.zeitlimitRunden,
  };
}

const SIGNAL_TEXT = {
  backlog_leer: "the backlog ran dry while agents were there",
  wartet_auf_lead: "checked submissions waited more than a day for a lead decision",
  d_stau: "level D tasks piled up or the lead service's cycles hit their time limit",
} as const;

/** Signale anhaltend (LEAD_KRAFT.tage in Folge) für ein Projekt, aus dem Tagesstand. */
export async function anhaltendeSignale(db: Db, projektId: number) {
  const r = (await db.query(
    `select bool_and(backlog_leer) as backlog_leer, bool_and(wartet_auf_lead) as wartet_auf_lead, bool_and(d_stau) as d_stau, count(*)::int as tage
     from lead_kraft_tage where projekt_id = $1 and tag > current_date - $2::int`, [projektId, LEAD_KRAFT.tage])).rows[0];
  if (r.tage < LEAD_KRAFT.tage) return [];
  return (Object.keys(SIGNAL_TEXT) as (keyof typeof SIGNAL_TEXT)[]).filter((k) => r[k]);
}

/** Täglich: Tagesstand schreiben und, wo Signale anhalten, Lead und Visionär den Lead-Kraft-Hinweis schicken. */
export async function leadKraftPruefen(db: Db) {
  const projekte = (await db.query("select id, slug, lead_kraft_hinweis from projekte where not gesperrt and not entwurf")).rows;
  let hinweise = 0;
  for (const p of projekte) {
    const s = await leadKraftSignale(db, Number(p.id));
    await db.query(
      `insert into lead_kraft_tage(projekt_id, tag, backlog_leer, wartet_auf_lead, d_stau) values ($1, current_date, $2, $3, $4)
       on conflict (projekt_id, tag) do update set backlog_leer = $2, wartet_auf_lead = $3, d_stau = $4`, [p.id, s.backlog_leer, s.wartet_auf_lead, s.d_stau]);
    const an = await anhaltendeSignale(db, Number(p.id));
    if (!an.length) continue;
    if (p.lead_kraft_hinweis && Date.now() - new Date(p.lead_kraft_hinweis).getTime() < LEAD_KRAFT.abstandTage * 86_400_000) continue;
    const inhalt = `For ${LEAD_KRAFT.tage} days ${an.map((k) => SIGNAL_TEXT[k]).join("; ")}. The project would move faster with more lead power. Two ways:\n\n` +
      `1. Invite someone: open a lead position for an area (open_lead_position, or My account on the website) and share ${teilenLink(p.slug)}. ` +
      "Whoever applies uses their own account and their own model; the visionary decides.\n" +
      `2. Run a second lead agent yourself for one area, ideally with another model: swarvex project lead setup ${p.slug} --area <name> --paths <globs> --agent <another agent>. ` +
      "It takes that area (take_area); your main lead keeps the rest.\n\nThe swarm keeps working meanwhile.";
    await transaktion(db, async (tx) => {
      for (const [an_, zustand] of [["lead", "offen"], ["visionaer", "erledigt"]] as const) {
        await tx.query(
          `insert into nachrichten(projekt_id, von_mensch, von_agent, an, art, betreff, inhalt, aufgaben, zustand)
           values ($1, null, null, $2, 'info', 'More lead power would help', $3, '{}', $4)`, [p.id, an_, inhalt, zustand]);
      }
      await tx.query("update projekte set lead_kraft_hinweis = now() where id = $1", [p.id]);
    });
    hinweise++;
  }
  await db.query("delete from lead_kraft_tage where tag < current_date - 30");
  return hinweise;
}
