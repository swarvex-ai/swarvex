// Sprungziel der Einreichungs-Vorschau bei Formwerk (katalog.ts): ohne Datenbank.
import { describe, expect, it } from "vitest";
import { katalogIds, zielWaehlen } from "../src/katalog.ts";

const patch = (...pfade: string[]) => pfade.map((p) => `diff --git a/${p} b/${p}\nnew file mode 100644\n--- /dev/null\n+++ b/${p}\n@@ -0,0 +1 @@\n+x\n`).join("");

describe("Sprungziel der Einreichungs-Vorschau", () => {
  it("liest die Kennungen aller Arten aus catalog.json", () => {
    expect(katalogIds({ models: [{ id: "chair" }, { id: "bridge" }], sounds: [{ id: "door-creak" }], fremd: [{ id: "nein" }], icons: [{ titel: "ohne id" }] }))
      .toEqual(["chair", "bridge", "door-creak"]);
    expect(katalogIds(null)).toEqual([]);
  });
  it("nimmt das Asset aus den Dateinamen des Patches", () => {
    const p = patch("src/assets/index.ts", "src/assets/structures/bridge.ts", "src/index.ts", "tests/scene/baseline/bridge.png");
    expect(zielWaehlen(p, ["chair", "bridge"], new Set(["chair"]))).toBe("#bridge");
  });
  it("findet auch ein geändertes Asset, das main schon hat", () => {
    expect(zielWaehlen(patch("src/assets/furniture/chair.ts"), ["chair", "sofa"], new Set(["chair", "sofa"]))).toBe("#chair");
  });
  it("sonst das Asset, das es auf main noch nicht gibt (etwa ein Icon-Satz in einer Datei)", () => {
    expect(zielWaehlen(patch("src/icons/status.ts"), ["chair", "warning", "info"], new Set(["chair"]))).toBe("#warning");
  });
  it("leer, wenn nichts passt (die Startseite)", () => {
    expect(zielWaehlen(patch("README.md"), ["chair"], new Set(["chair"]))).toBe("");
    expect(zielWaehlen(patch("src/assets/x/nicht-im-katalog.ts"), [], new Set())).toBe("");
  });
});
