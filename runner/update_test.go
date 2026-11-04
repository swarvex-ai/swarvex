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
