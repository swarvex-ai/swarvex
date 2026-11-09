// Sprungziel der Einreichungs-Vorschau bei Formwerk (katalog.ts): ohne Datenbank.
import { describe, expect, it } from "vitest";
import { katalogAssets, zielWaehlen } from "../src/katalog.ts";

const patch = (...pfade: string[]) => pfade.map((p) => `diff --git a/${p} b/${p}\n--- a/${p}\n+++ b/${p}\n@@ -1 +1 @@\n-x\n+y\n`).join("");
const katalog = katalogAssets({
  models: [{ id: "chair" }, { id: "bridge" }], surfaces: [{ id: "planks" }, { id: "shingles" }], icons: [{ id: "list" }, { id: "warning" }],
});

describe("Sprungziel der Einreichungs-Vorschau", () => {
  it("liest die Assets aller Arten mit ihrem Ordner aus catalog.json", () => {
    expect(katalogAssets({ models: [{ id: "chair" }], sounds: [{ id: "door-creak" }], fremd: [{ id: "nein" }], icons: [{ titel: "ohne id" }] }))
      .toEqual([{ id: "chair", ordner: "assets" }, { id: "door-creak", ordner: "sounds" }]);
    expect(katalogAssets(null)).toEqual([]);
  });
  it("nimmt das neue Asset mit eigener Datei", () => {
    const p = patch("src/assets/index.ts", "src/assets/structures/bridge.ts", "src/index.ts", "tests/scene/baseline/bridge.png");
    expect(zielWaehlen(p, katalog, new Set(["chair", "planks", "shingles", "list", "warning"]))).toBe("#bridge");
  });
  it("verwechselt eine Registerdatei nicht mit einem gleichnamigen Asset einer anderen Art", () => {
    // Oberfläche shingles: src/surfaces/list.ts ist das Register, „list“ auch ein Icon
    const p = patch("src/surfaces/list.ts", "src/surfaces/stone/shingles.ts", "tests/surfaces/baseline/shingles.png");
    expect(zielWaehlen(p, katalog, new Set(["chair", "bridge", "planks", "list", "warning"]))).toBe("#shingles");
    expect(zielWaehlen(patch("src/surfaces/list.ts"), katalog, new Set(katalog.map((a) => a.id)))).toBe("");
  });
  it("findet auch ein geändertes Asset, das main schon hat", () => {
    expect(zielWaehlen(patch("src/assets/furniture/chair.ts"), katalog, new Set(katalog.map((a) => a.id)))).toBe("#chair");
  });
  it("sonst das Asset, das es auf main noch nicht gibt (etwa ein Icon-Satz in einer Datei)", () => {
    expect(zielWaehlen(patch("src/icons/status/set.ts"), katalog, new Set(["chair", "bridge", "planks", "shingles", "list"]))).toBe("#warning");
  });
  it("leer, wenn nichts passt (die Startseite)", () => {
    expect(zielWaehlen(patch("README.md"), katalog, new Set(katalog.map((a) => a.id)))).toBe("");
    expect(zielWaehlen(patch("src/assets/x/nicht-im-katalog.ts"), [], new Set())).toBe("");
  });
});
