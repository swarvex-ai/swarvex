import { describe, expect, it } from "vitest";
import { geraetAus, pfadNormal, quelleAus, zaehlbar } from "../src/besuche.ts";

describe("Besucherstatistik", () => {
  it("normalisiert Pfade und verwirft Ungewöhnliches", () => {
    expect(pfadNormal("/projekte/rend-and-run/?x=1#a")).toBe("/projekte/rend-and-run");
    expect(pfadNormal("/")).toBe("/");
    expect(pfadNormal("/en/docs/runner")).toBe("/en/docs/runner");
    expect(pfadNormal("/docs/agent-w%C3%A4hlen")).toBe("/docs/agent-wählen");
    expect(pfadNormal("https://evil.example/")).toBeNull();
    expect(pfadNormal("/<script>")).toBeNull();
    expect(pfadNormal("/" + "a".repeat(200))).toBeNull();
    expect(pfadNormal(42)).toBeNull();
  });
  it("nimmt von der Herkunft nur die Domain", () => {
    expect(quelleAus("https://www.google.com/search?q=swarvex", "swarvex.ai")).toBe("google.com");
    expect(quelleAus("https://swarvex.ai/docs", "swarvex.ai")).toBe("direkt");
    expect(quelleAus("", "swarvex.ai")).toBe("direkt");
    expect(quelleAus("kaputt", "swarvex.ai")).toBe("direkt");
  });
  it("erkennt Gerätetyp, Bots und Datenschutz-Signale", () => {
    expect(geraetAus("Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) Mobile")).toBe("mobil");
    expect(geraetAus("Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X)")).toBe("tablet");
    expect(geraetAus("Mozilla/5.0 (X11; Linux x86_64) Firefox/140.0")).toBe("desktop");
    expect(zaehlbar({ "user-agent": "Mozilla/5.0 (X11; Linux x86_64) Firefox/140.0" })).toBe(true);
    expect(zaehlbar({ "user-agent": "Googlebot/2.1" })).toBe(false);
    expect(zaehlbar({ "user-agent": "Mozilla/5.0 Firefox", "sec-gpc": "1" })).toBe(false);
    expect(zaehlbar({ "user-agent": "Mozilla/5.0 Firefox", dnt: "1" })).toBe(false);
    expect(zaehlbar({})).toBe(false);
  });
});
