package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallerLesen(t *testing.T) {
	skript := "#!/bin/sh\nset -eu\nPLATFORM=\"https://swarvex.ai\"\nVERSION=\"0.1.202611041235\"\nSIGNING_KEY=\"ssh-ed25519 AAAAC3Nza\"\nCA_SOURCE=\"${SWARVEX_CA:-}\"\n"
	ver, key := installerLesen(skript)
	if ver != "0.1.202611041235" || key != "ssh-ed25519 AAAAC3Nza" {
		t.Fatalf("%q %q", ver, key)
	}
	if v, k := installerLesen("<html>nope</html>"); v != "" || k != "" {
		t.Fatalf("aus HTML gelesen: %q %q", v, k)
	}
}

func TestSchluesselPasst(t *testing.T) {
	if !schluesselPasst("", "ssh-ed25519 A") || !schluesselPasst("ssh-ed25519 A\n", "ssh-ed25519 A") {
		t.Fatal("gleicher oder fehlender Schlüssel muss passen")
	}
	if schluesselPasst("ssh-ed25519 A\n", "ssh-ed25519 B") {
		t.Fatal("anderer Schlüssel darf nicht passen")
	}
}

// install ersetzt das Programm durch eine neue Datei; der Dauerbetrieb erkennt das und lädt sich neu.
func TestNeueVersionInstalliert(t *testing.T) {
	d := t.TempDir()
	programm := filepath.Join(d, "swarvex")
	os.WriteFile(programm, []byte("alt"), 0o755)
	start, _ := os.Stat(programm)
	if neueVersionInstalliert(programm, start) {
		t.Fatal("unverändert gilt als neu")
	}
	os.WriteFile(programm+".neu", []byte("neu"), 0o755)
	os.Rename(programm+".neu", programm)
	if !neueVersionInstalliert(programm, start) {
		t.Fatal("ersetzt, aber nicht erkannt")
	}
	if neueVersionInstalliert(programm, nil) {
		t.Fatal("ohne Startzustand nie neu laden")
	}
}

func TestEinheitArgsUndBudget(t *testing.T) {
	args := []string{"--project", "auto", "--model-key-file=/home/a b/key", "--budget-tokens", "1000000", "--schedule", "daily 22:00-07:00", "--daily-budget-tokens", "2000000", "--auto-update"}
	einheit := dienstEinheit("/home/u/.local/bin/swarvex", args)
	gelesen, ok := einheitArgsLesen(einheit)
	if !ok || len(gelesen) != len(args) {
		t.Fatalf("%v %q", ok, gelesen)
	}
	for i := range args {
		if gelesen[i] != args[i] {
			t.Fatalf("Argument %d: %q statt %q", i, gelesen[i], args[i])
		}
	}
	neu := argSetzen(argSetzen(gelesen, "budget-tokens", "0"), "daily-budget-tokens", "0")
	if w, _ := argWert(neu, "budget-tokens"); w != "0" {
		t.Fatalf("budget-tokens %q", w)
	}
	if w, _ := argWert(neu, "daily-budget-tokens"); w != "0" {
		t.Fatalf("daily %q", w)
	}
	if w, _ := argWert(neu, "schedule"); w != "daily 22:00-07:00" || !hatFlag(neu, "auto-update") {
		t.Fatalf("andere Angaben verändert: %q", neu)
	}
	if w, _ := argWert([]string{"--budget-tokens=5"}, "budget-tokens"); w != "5" {
		t.Fatal("--x=wert nicht gelesen")
	}
}

func TestVerlauf(t *testing.T) {
	datei := filepath.Join(t.TempDir(), "history.jsonl")
	for i := int64(1); i <= 5; i++ {
		verlaufMerken(datei, verlaufEintrag{Aufgabe: i, Ergebnis: "returned"})
	}
	l := verlaufLesen(datei, 3)
	if len(l) != 3 || l[0].Aufgabe != 5 || l[2].Aufgabe != 3 {
		t.Fatalf("%+v", l)
	}
	if tausender(1234567) != "1,234,567" || tausender(999) != "999" {
		t.Fatal(tausender(1234567))
	}
}
