// Tests ohne Datenbank: SSRF-Schutz und Ratenlimits.
import { describe, expect, it } from "vitest";
import { oeffentlicheAdresse, privateAdresse } from "../src/auth/netz.ts";
import { grenzenErstellen } from "../src/grenzen.ts";

describe("SSRF-Schutz", () => {
  it("erkennt private und besondere Adressen", () => {
    for (const ip of ["10.1.2.3", "127.0.0.1", "169.254.169.254", "172.16.0.1", "192.168.1.10", "100.64.0.1", "0.0.0.0", "::1", "fd00::1", "fe80::1", "::ffff:192.168.1.1"]) {
      expect(privateAdresse(ip), ip).toBe(true);
    }
    for (const ip of ["1.1.1.1", "8.8.8.8", "140.82.112.3", "2606:4700:4700::1111"]) expect(privateAdresse(ip), ip).toBe(false);
  });
  it("lehnt IP-Literale im internen Netz ab", async () => {
    expect(await oeffentlicheAdresse("127.0.0.1")).toBe(false);
    expect(await oeffentlicheAdresse("[::1]")).toBe(false);
    expect(await oeffentlicheAdresse("gibt-es-nicht.invalid")).toBe(false);
  });
});

describe("Ratenlimits", () => {
  it("lässt die Grenze zu und füllt mit der Zeit nach", () => {
    let t = 0;
    const pruefen = grenzenErstellen({ test: { anzahl: 3, proSekunden: 60 } }, () => t);
    expect([pruefen("test", "a"), pruefen("test", "a"), pruefen("test", "a")]).toEqual([0, 0, 0]);
    expect(pruefen("test", "a")).toBeGreaterThan(0);
    expect(pruefen("test", "b")).toBe(0);  // anderer Schlüssel, eigener Eimer
    t += 20_000;                             // 20 s = ein Platz
    expect(pruefen("test", "a")).toBe(0);
    expect(pruefen("test", "a")).toBeGreaterThan(0);
  });
});
