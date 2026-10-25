// Tests ohne Datenbank: Patch-Regeln, insbesondere geschützte Dateien.
import { describe, expect, it } from "vitest";
import { patchPruefen } from "../src/aufgaben.ts";

const patch = (pfad: string) => `diff --git a/${pfad} b/${pfad}
--- a/${pfad}
+++ b/${pfad}
@@ -1 +1 @@
-alt
+neu
`;

describe("Patch-Regeln", () => {
  it("weist geschützte Dateien ab, auch wenn das Muster sie erlaubt", () => {
    for (const p of ["swarvex.yaml", "AGENTS.md", ".devcontainer/devcontainer.json", ".github/workflows/ci.yml"]) {
      expect(patchPruefen(patch(p), ["**"]).join(), p).toMatch(/geschützte Datei/);
    }
  });
  it("lässt normale Dateien in erlaubten Pfaden durch", () => {
    expect(patchPruefen(patch("src/app.ts"), ["src/**"])).toEqual([]);
    expect(patchPruefen(patch("docs/AGENTS.md"), ["docs/**"])).toEqual([]);
  });
});
