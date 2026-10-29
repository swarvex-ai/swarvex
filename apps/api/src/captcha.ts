// Captcha des Meldeformulars: Proof of Work im ALTCHA-Format, ohne Drittanbieter und ohne Cookies.
// Der Browser sucht n mit SHA-256(salt + n) === challenge und schickt base64(JSON {challenge, salt, number, algorithm}).
// Das Salt trägt Ablauf und HMAC selbst; Zustand gibt es nur für bereits benutzte Aufgaben (bis zum Ablauf).
import { createHash, createHmac, randomBytes, randomInt, timingSafeEqual } from "node:crypto";

// Im Schnitt die Hälfte davon an SHA-256-Runden im Browser: deutlich unter einer Sekunde
export const MAX_ZAHL = 250_000;
const GUELTIG_MS = 10 * 60 * 1000;
const benutzt = new Map<string, number>();

const sha256 = (s: string) => createHash("sha256").update(s).digest("hex");
const signieren = (geheimnis: string, wert: string) => createHmac("sha256", geheimnis).update(`captcha:${wert}`).digest("base64url");
function pruefen(geheimnis: string, wert: string, sig: string) {
  const a = Buffer.from(signieren(geheimnis, wert)), b = Buffer.from(sig);
  return a.length === b.length && timingSafeEqual(a, b);
}

export function captchaAufgabe(geheimnis: string, jetzt = Date.now()) {
  const basis = `${randomBytes(12).toString("hex")}.${jetzt + GUELTIG_MS}`;
  const salt = `${basis}.${signieren(geheimnis, basis)}.`;
  return { challenge: sha256(salt + randomInt(MAX_ZAHL)), salt, algorithm: "SHA-256", maxnumber: MAX_ZAHL };
}

/** Prüft die Lösung; jede Aufgabe gilt nur einmal. */
export function captchaPruefen(geheimnis: string, nutzlast: unknown, jetzt = Date.now()): boolean {
  if (typeof nutzlast !== "string" || nutzlast.length > 1000) return false;
  let d: { challenge?: unknown; salt?: unknown; number?: unknown };
  try {
    d = JSON.parse(Buffer.from(nutzlast, "base64").toString("utf8"));
  } catch {
    return false;
  }
  const { challenge, salt, number } = d;
  if (typeof challenge !== "string" || typeof salt !== "string" || typeof number !== "number") return false;
  const [zufall, ablauf, sig] = salt.split(".");
  if (!zufall || !ablauf || !sig || !pruefen(geheimnis, `${zufall}.${ablauf}`, sig)) return false;
  if (Number(ablauf) < jetzt || sha256(salt + number) !== challenge) return false;
  for (const [c, a] of benutzt) if (a < jetzt) benutzt.delete(c);
  if (benutzt.has(challenge)) return false;
  benutzt.set(challenge, Number(ablauf));
  return true;
}
