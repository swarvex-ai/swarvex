package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Ungeprüfter Code bekommt je Lauf einen eigenen Cache, vorbelegt aus cache-main; was er dort ablegt, sieht weder der
// nächste Lauf noch die main-Vorschau (Sicherheitsaudit, Nachtrag 13.11.2026).
func TestWegwerfCache(t *testing.T) {
	alt := arbeit
	arbeit = t.TempDir()
	defer func() { arbeit = alt }()
	haupt := filepath.Join(arbeit, "cache-main", "spiel", "go", "build")
	os.MkdirAll(haupt, 0o755)
	os.WriteFile(filepath.Join(haupt, "eintrag"), []byte("von main"), 0o644)

	eins, wegEins, err := wegwerfCache("spiel")
	if err != nil {
		t.Fatal(err)
	}
	if roh, _ := os.ReadFile(filepath.Join(eins, "go", "build", "eintrag")); string(roh) != "von main" {
		t.Fatalf("not seeded from cache-main: %q", roh)
	}
	// der Lauf einer Einreichung verändert seinen Cache …
	os.WriteFile(filepath.Join(eins, "go", "build", "eintrag"), []byte("vergiftet"), 0o644)
	os.WriteFile(filepath.Join(eins, "neu"), []byte("vergiftet"), 0o644)
	// … der nächste Lauf und cache-main sehen davon nichts
	zwei, wegZwei, err := wegwerfCache("spiel")
	if err != nil {
		t.Fatal(err)
	}
	defer wegZwei()
	if roh, _ := os.ReadFile(filepath.Join(zwei, "go", "build", "eintrag")); string(roh) != "von main" {
		t.Fatalf("next run sees %q", roh)
	}
	if _, err := os.Stat(filepath.Join(zwei, "neu")); err == nil {
		t.Fatal("next run sees a file of the previous run")
	}
	if roh, _ := os.ReadFile(filepath.Join(haupt, "eintrag")); string(roh) != "von main" {
		t.Fatalf("cache-main changed: %q", roh)
	}
	wegEins()
	if _, err := os.Stat(eins); err == nil {
		t.Fatal("throwaway cache still there after cleanup")
	}
	// ohne cache-main: leer
	drei, wegDrei, err := wegwerfCache("neues-projekt")
	if err != nil {
		t.Fatal(err)
	}
	defer wegDrei()
	if eintraege, _ := os.ReadDir(drei); len(eintraege) != 0 {
		t.Fatalf("cache of a project without main cache not empty: %v", eintraege)
	}
}
