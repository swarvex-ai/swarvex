// Tests ohne Datenbank: Captcha des Meldeformulars.
import { createHash } from "node:crypto";
import { describe, expect, it } from "vitest";
import { captchaAufgabe, captchaPruefen } from "../src/captcha.ts";

const G = "test-geheimnis";
const sha256 = (s: string) => createHash("sha256").update(s).digest("hex");

function loesen(a: ReturnType<typeof captchaAufgabe>, salt = a.salt) {
  for (let n = 0; n <= a.maxnumber; n++) {
    if (sha256(a.salt + n) === a.challenge) return Buffer.from(JSON.stringify({ challenge: a.challenge, salt, number: n, algorithm: a.algorithm })).toString("base64");
  }
  throw new Error("keine Lösung");
}

describe("Captcha", () => {
  it("nimmt eine gelöste Aufgabe genau einmal an", () => {
    const l = loesen(captchaAufgabe(G));
    expect(captchaPruefen(G, l)).toBe(true);
    expect(captchaPruefen(G, l)).toBe(false);
  });
  it("lehnt fehlende, kaputte und falsche Lösungen ab", () => {
    const a = captchaAufgabe(G);
    for (const x of [undefined, "", "kein-base64", 42]) expect(captchaPruefen(G, x)).toBe(false);
    const falsch = Buffer.from(JSON.stringify({ challenge: a.challenge, salt: a.salt, number: -1, algorithm: "SHA-256" })).toString("base64");
    expect(captchaPruefen(G, falsch)).toBe(false);
  });
  it("lehnt fremde und abgelaufene Aufgaben ab", () => {
    expect(captchaPruefen("anderes-geheimnis", loesen(captchaAufgabe(G)))).toBe(false);
    const alt = captchaAufgabe(G, Date.now() - 11 * 60 * 1000);
    expect(captchaPruefen(G, loesen(alt))).toBe(false);
  });
  it("lehnt selbst gebaute Aufgaben ohne gültige Signatur ab", () => {
    const salt = `abc.${Date.now() + 60_000}.gefaelscht.`;
    const l = Buffer.from(JSON.stringify({ challenge: sha256(salt + 1), salt, number: 1, algorithm: "SHA-256" })).toString("base64");
    expect(captchaPruefen(G, l)).toBe(false);
  });
});
