package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

// Der Agent testet nur, was er in seinen Pfaden geändert hat (Hebel B, 14.11.2026): Das Skript findet die Dateien,
// die nach dem Start des Agenten geändert wurden, und reicht sie wie der eigene Testlauf als SWARVEX_CHANGED weiter.
func TestTestSkriptNurGeaendertes(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	wurzel := t.TempDir()
	work := filepath.Join(wurzel, "work")
	schreiben := func(p, inhalt string) {
		voll := filepath.Join(work, p)
		os.MkdirAll(filepath.Dir(voll), 0o755)
		os.WriteFile(voll, []byte(inhalt), 0o644)
	}
	// vor dem Start: Snapshot und Einrichten
	schreiben("src/assets/props/barrel.ts", "alt")
	schreiben("src/assets/props/chair.ts", "alt")
	schreiben("node_modules/x/index.js", "alt")
	alt := time.Now().Add(-time.Hour)
	filepath.Walk(work, func(p string, _ os.FileInfo, _ error) error { os.Chtimes(p, alt, alt); return nil })
	u := umgebung.Umgebung{Ordner: "", Bauen: "echo gebaut > " + wurzel + "/gebaut", Testen: "printf '%s\\n' \"$SWARVEX_CHANGED\" > " + wurzel + "/geaendert"}
	a := aufgabe{Pfade: []string{"src/assets/props/barrel.ts", "docs/it's here/**", "tests/scene/baseline"}}
	ordner := filepath.Join(wurzel, "aufgabe")
	pfad, err := testHilfeAnlegen(ordner, a, u)
	if err != nil {
		t.Fatal(err)
	}
	// das Skript an die Testordner anpassen (in der Sandbox: /work und /run/swarvex-test)
	roh, _ := os.ReadFile(filepath.Join(pfad, "test.sh"))
	skript := strings.ReplaceAll(strings.ReplaceAll(string(roh), testHilfePfad, pfad), "/work", work)
	os.WriteFile(filepath.Join(pfad, "test.sh"), []byte(skript), 0o755)
	// nichts geändert: kein Testlauf (ohne SWARVEX_CHANGED liefe sonst die ganze Suite)
	if aus, err := exec.Command("sh", filepath.Join(pfad, "test.sh")).CombinedOutput(); err != nil || !strings.Contains(string(aus), "nothing changed") {
		t.Fatalf("unchanged: %v %s", err, aus)
	}
	time.Sleep(20 * time.Millisecond)
	// der Agent ändert eine erlaubte Datei, legt eine in einem Glob-Ordner und ein Referenzbild an und fasst Fremdes an
	schreiben("src/assets/props/barrel.ts", "neu")
	schreiben("docs/it's here/a.md", "neu")
	schreiben("tests/scene/baseline/barrel.png", "neu")
	schreiben("src/assets/props/chair.ts", "neu")
	schreiben("dist/catalog.json", "neu")
	if aus, err := exec.Command("sh", filepath.Join(pfad, "test.sh")).CombinedOutput(); err != nil {
		t.Fatalf("run: %v %s", err, aus)
	}
	geaendert, _ := os.ReadFile(filepath.Join(wurzel, "geaendert"))
	want := "docs/it's here/a.md\nsrc/assets/props/barrel.ts\ntests/scene/baseline/barrel.png\n"
	if string(geaendert) != want {
		t.Fatalf("SWARVEX_CHANGED = %q, want %q", geaendert, want)
	}
	if _, err := os.Stat(filepath.Join(wurzel, "gebaut")); err != nil {
		t.Fatal("build step did not run before the tests")
	}
	if _, err := testHilfeAnlegen(filepath.Join(wurzel, "ohne"), a, umgebung.Umgebung{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wurzel, "ohne", "swarvex-test")); err == nil {
		t.Fatal("helper created for a project without a test command")
	}
}
