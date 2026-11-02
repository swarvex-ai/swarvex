// Beitragsübersicht (src/beitraege.ts): Dateien und Zeilen aus Patches, Art des Beitrags; ohne Datenbank.
import { describe, expect, it } from "vitest";
import { patchDateien, sorteVon } from "../src/beitraege.ts";

const PATCH = `diff --git a/src/spieler.gd b/src/spieler.gd
--- a/src/spieler.gd
+++ b/src/spieler.gd
@@ -1,3 +1,4 @@
 extends Node
-var tempo = 1
+var tempo = 2
+var sprung = 3
diff --git a/assets/held.png b/assets/held.png
new file mode 100644
--- /dev/null
+++ b/assets/held.png
@@ -0,0 +1,3 @@
+version https://git-lfs.github.com/spec/v1
+oid sha256:${"a".repeat(64)}
+size 1234
diff --git a/assets/klick.ogg b/assets/klick.ogg
new file mode 100644
Binary files /dev/null and b/assets/klick.ogg differ
diff --git a/tests/spieler_test.gd b/tests/spieler_test.gd
--- /dev/null
+++ b/tests/spieler_test.gd
@@ -0,0 +1 @@
+func test_tempo(): pass
`;

describe("Beitragsübersicht", () => {
  it("zählt Zeilen je Datei und erkennt LFS-Zeiger und Binärdateien", () => {
    const d = patchDateien(PATCH);
    expect(d.map((x) => [x.pfad, x.sorte, x.plus, x.minus, x.lfs, x.binaer])).toEqual([
      ["src/spieler.gd", "code", 2, 1, false, false],
      ["assets/held.png", "bild", 0, 0, true, false],
      ["assets/klick.ogg", "audio", 0, 0, false, true],
      ["tests/spieler_test.gd", "test", 1, 0, false, false],
    ]);
  });
  it("ordnet Dateien nach Endung ein", () => {
    expect(["a.glb", "b.wav", "c.md", "d.tscn", "e.svelte", "f.bin"].map(sorteVon)).toEqual(["3d", "audio", "text", "daten", "code", "sonstiges"]);
  });
});
