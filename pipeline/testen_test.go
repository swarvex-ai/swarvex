package main

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestTestenZweiterLauf(t *testing.T) {
	for _, f := range []struct {
		name     string
		laeufe   []error
		ok       bool
		anzahl   int
		enthaelt string
	}{
		{"gleich gruen", []error{nil}, true, 1, "=== Test\n"},
		{"wackelig", []error{errors.New("1"), nil}, true, 2, "second run: passed"},
		{"wirklich kaputt", []error{errors.New("1"), errors.New("1")}, false, 2, "failed again"},
	} {
		n := 0
		var b strings.Builder
		ok := testen(&b, func() (string, error) { n++; return "lauf", f.laeufe[n-1] })
		if ok != f.ok || n != f.anzahl || !strings.Contains(b.String(), f.enthaelt) {
			t.Errorf("%s: ok=%v runs=%d report=%q", f.name, ok, n, b.String())
		}
	}
}

func TestWackeligeTestsAnVerschiedenenStellen(t *testing.T) {
	// Rend&Run, 5.11.2026: eine Doku-Seite, der erste Lauf scheitert an einem Test, der zweite an einem anderen
	laeufe := []string{
		"FAIL    headless   crouch_gait_integration\n---\n      1 FAIL headless\n     75 PASS headless\n",
		"FAIL    headless   arm_first_fight\n---\n      1 FAIL headless\n     75 PASS headless\n",
	}
	n := 0
	var b strings.Builder
	if !testen(&b, func() (string, error) { n++; return laeufe[n-1], errors.New("1") }) || !strings.Contains(b.String(), "failed on other tests") {
		t.Errorf("different tests in two runs are flaky, not the contribution: %q", b.String())
	}
	// Derselbe Test in beiden Läufen (dazu ein wackeliger): der Beitrag ist schuld
	laeufe = []string{"--- FAIL: TestA (0.01s)\n--- FAIL: TestB (0.20s)\nFAIL\n", "--- FAIL: TestA (0.02s)\nFAIL\n"}
	n = 0
	b.Reset()
	if testen(&b, func() (string, error) { n++; return laeufe[n-1], errors.New("1") }) || !strings.Contains(b.String(), "failed again") {
		t.Errorf("the same test failing twice counts: %q", b.String())
	}
	// Nichts Erkennbares in der Ausgabe: der strenge Weg
	n = 0
	b.Reset()
	if testen(&b, func() (string, error) { n++; return "Fehler", errors.New("1") }) {
		t.Errorf("unreadable output stays red: %q", b.String())
	}
}

func TestFehlschlaege(t *testing.T) {
	got := fehlschlaege("FAIL    headless   nav_test\n      3 FAIL headless\n--- FAIL: TestX (0.00s)\nFAIL\tpkg/x\t0.012s\nFAILED tests/a.py::test_b - assert 1\n=== 1 failed in 0.1s ===\nFAIL headless nav_test\n")
	want := []string{"FAIL headless nav_test", "--- FAIL: TestX", "FAIL pkg/x", "FAILED tests/a.py::test_b - assert 1"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("fehlschlaege = %q, want %q", got, want)
	}
}

func TestGegenprobe(t *testing.T) {
	for _, f := range []struct {
		name      string
		bestanden bool
		err       error
		rot       bool
		enthaelt  string
	}{
		{"Projekt schon rot", false, nil, true, "failed too"},
		{"nur mit Beitrag rot", true, nil, false, "fail only with this contribution"},
		{"Gegenprobe unmöglich", false, errors.New("patch -R"), false, "not possible"},
	} {
		var b strings.Builder
		if rot := gegenprobe(&b, func() (string, bool, error) { return "lauf", f.bestanden, f.err }); rot != f.rot || !strings.Contains(b.String(), f.enthaelt) {
			t.Errorf("%s: rot=%v report=%q", f.name, rot, b.String())
		}
	}
}

// Am Zeitlimit (timeout endet mit 124) kein zweiter Lauf: er kostete nur noch einmal so lange und hielte alle auf.
func TestTestenZeitlimitOhneZweitenLauf(t *testing.T) {
	limit := exec.Command("sh", "-c", "exit 124").Run()
	if !zeitUm(limit) || zeitUm(errors.New("1")) || zeitUm(nil) {
		t.Fatalf("zeitUm: %v", limit)
	}
	n := 0
	var b strings.Builder
	if testen(&b, func() (string, error) { n++; return "lauf", limit }) || n != 1 || !strings.Contains(b.String(), Zeitlimit) {
		t.Errorf("ok after the time limit or run again: runs=%d report=%q", n, b.String())
	}
}

func TestGeaendertEnv(t *testing.T) {
	if geaendertEnv(nil) != nil {
		t.Error("without changes: no variable, the project tests everything")
	}
	if got := geaendertEnv([]string{"src/a.ts", "tests/b.png"}); len(got) != 2 || got[0] != "-e" || got[1] != "SWARVEX_CHANGED=src/a.ts\ntests/b.png" {
		t.Errorf("geaendertEnv: %q", got)
	}
}
