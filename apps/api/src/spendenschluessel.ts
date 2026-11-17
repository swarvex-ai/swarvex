// API-Schlüssel von Spendern (Zusage mit Schlüssel, migrations/067): Die API verschlüsselt mit dem öffentlichen
// Schlüssel und kann danach selbst nicht mehr lesen; nur das Spendenwerk (eigener Container) hat den privaten Schlüssel.
// Ein Einbruch in die API allein gibt also keine Spender-Schlüssel preis. RSA-OAEP mit SHA-256: API-Schlüssel sind kurz
// (unter 200 Zeichen), das passt direkt in einen Block.
import { constants, privateDecrypt, publicEncrypt } from "node:crypto";
import { readFileSync } from "node:fs";

const OEFFENTLICH = () => process.env.SPENDEN_PUBLIC_KEY ?? "/data/spenden-schluessel/public.pem";
const PRIVAT = () => process.env.SPENDEN_PRIVATE_KEY ?? "/data/spenden-schluessel/private.pem";

let oeffentlich: string | null | undefined;
let privat: string | null | undefined;
const lesen = (pfad: string) => { try { return readFileSync(pfad, "utf8"); } catch { return null; } };

/** Für Tests: gelesene Schlüsseldateien vergessen. */
export function schluesselVergessen() { oeffentlich = undefined; privat = undefined; }

export function verschluesseln(klartext: string): Buffer {
  oeffentlich ??= lesen(OEFFENTLICH());
  if (!oeffentlich) throw new Error("donation key encryption is not set up (public key missing)");
  return publicEncrypt({ key: oeffentlich, padding: constants.RSA_PKCS1_OAEP_PADDING, oaepHash: "sha256" }, Buffer.from(klartext, "utf8"));
}

/** Nur im Spendenwerk: dort liegt der private Schlüssel. */
export function entschluesseln(chiffre: Buffer): string {
  privat ??= lesen(PRIVAT());
  if (!privat) throw new Error("donation key decryption is not available here (private key missing)");
  return privateDecrypt({ key: privat, padding: constants.RSA_PKCS1_OAEP_PADDING, oaepHash: "sha256" }, chiffre).toString("utf8");
}

/** Letzte vier Zeichen zum Wiedererkennen (Konto, Lead sieht sie nicht). */
export const schluesselEnde = (s: string) => s.slice(-4).replace(/[^A-Za-z0-9_-]/g, "");
