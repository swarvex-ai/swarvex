// Besucherstatistik ohne Cookies (migrations/034_besuche.sql): der Browser meldet nur die Seite und, beim ersten
// Aufruf, die Herkunft. Gespeichert werden Summen; eindeutige Besucher je Tag über einen Hash mit Tagesgeheimnis,
// das samt Hashes nach Tagesende gelöscht wird. Bots, „Do Not Track“ und „Global Privacy Control“ zählen nicht.
import { createHash, randomBytes } from "node:crypto";
import type { Db } from "./db.ts";

const BOT = /bot|crawl|spider|slurp|scrape|preview|monitor|uptime|lighthouse|headless|phantom|curl|wget|python|java\/|go-http|axios|node-fetch/i;

export function geraetAus(ua: string): "mobil" | "tablet" | "desktop" {
  if (/ipad|tablet|kindle|silk/i.test(ua) || (/android/i.test(ua) && !/mobile/i.test(ua))) return "tablet";
  if (/mobi|iphone|ipod|android|phone/i.test(ua)) return "mobil";
  return "desktop";
}

/** Nur der Pfad, ohne Abfrage und Anker; ungewöhnliche Pfade (Spam, sehr lang) zählen nicht. */
export function pfadNormal(p: unknown): string | null {
  if (typeof p !== "string" || !p.startsWith("/") || p.length > 160) return null;
  let pfad = p.split(/[?#]/)[0];
  try { pfad = decodeURI(pfad); } catch { return null; }
  if (pfad.length > 1) pfad = pfad.replace(/\/+$/, "");
  return /^\/[\p{L}\p{N}\-._~/]*$/u.test(pfad) ? pfad : null;
}

/** Herkunft als Domain (ohne www.); eigene Seiten und Leeres zählen als „direkt“. */
export function quelleAus(referrer: unknown, eigenerHost: string): string {
  if (typeof referrer !== "string" || !referrer) return "direkt";
  try {
    const host = new URL(referrer).hostname.toLowerCase().replace(/^www\./, "");
    if (!host || host === eigenerHost.replace(/^www\./, "")) return "direkt";
    return host.slice(0, 80);
  } catch {
    return "direkt";
  }
}

export function zaehlbar(kopf: Record<string, string | string[] | undefined>): boolean {
  const ua = String(kopf["user-agent"] ?? "");
  if (!ua || BOT.test(ua)) return false;
  return kopf["dnt"] !== "1" && kopf["sec-gpc"] !== "1";
}

export async function besuchZaehlen(db: Db, eingabe: { p?: unknown; r?: unknown }, ip: string, ua: string, eigenerHost: string): Promise<boolean> {
  const pfad = pfadNormal(eingabe.p);
  if (!pfad) return false;
  const quelle = quelleAus(eingabe.r, eigenerHost);
  const geraet = geraetAus(ua);
  const tag = (await db.query("select (now() at time zone 'Europe/Berlin')::date::text as tag")).rows[0].tag as string;
  await db.query("insert into besuch_salz(tag, wert) values ($1, $2) on conflict do nothing", [tag, randomBytes(32)]);
  const salz = (await db.query("select wert from besuch_salz where tag = $1", [tag])).rows[0].wert as Buffer;
  const kennung = createHash("sha256").update(salz).update(ip).update("\0").update(ua).digest().subarray(0, 16);
  await db.query("insert into besucher_heute(tag, kennung) values ($1, $2) on conflict do nothing", [tag, kennung]);
  await db.query(
    `insert into besuche(tag, pfad, quelle, geraet, aufrufe) values ($1, $2, $3, $4, 1)
     on conflict (tag, pfad, quelle, geraet) do update set aufrufe = besuche.aufrufe + 1`, [tag, pfad, quelle, geraet]);
  return true;
}

/** Läuft periodisch: abgeschlossene Tage behalten nur die Besucherzahl; Geheimnis und Hashes werden gelöscht. */
export async function besucheAbschliessen(db: Db): Promise<number> {
  const heute = "(now() at time zone 'Europe/Berlin')::date";
  const n = (await db.query(
    `insert into besucher_tage(tag, besucher) select tag, count(*) from besucher_heute where tag < ${heute} group by tag
     on conflict (tag) do update set besucher = besucher_tage.besucher + excluded.besucher`)).rowCount ?? 0;
  await db.query(`delete from besucher_heute where tag < ${heute}`);
  await db.query(`delete from besuch_salz where tag < ${heute}`);
  return n;
}
