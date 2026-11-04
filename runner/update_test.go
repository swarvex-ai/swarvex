package main

import (
	"encoding/base64"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerLesen(t *testing.T) {
	skript := "#!/bin/sh\nset -eu\nPLATFORM=\"https://swarvex.ai\"\nVERSION=\"0.1.202611041235\"\nSIGNING_KEY=\"ssh-ed25519 AAAAC3Nza\"\nCA_SOURCE=\"${SWARVEX_CA:-}\"\n"
	ver, key, _ := installerLesen(skript)
	if ver != "0.1.202611041235" || key != "ssh-ed25519 AAAAC3Nza" {
		t.Fatalf("%q %q", ver, key)
	}
	if v, k, _ := installerLesen("<html>nope</html>"); v != "" || k != "" {
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

func TestIstNeuer(t *testing.T) {
	for _, f := range []struct {
		neu, alt string
		soll     bool
	}{
		{"0.9.0", "0.1.202611041358", true}, {"0.9.1", "0.9.0", true}, {"0.10.0", "0.9.3", true}, {"1.0.0", "0.99.0", true},
		{"0.9.0", "0.9.0", false}, {"0.9.0", "0.9.1", false}, {"0.9", "0.9.0", false}, {"0.9.0", "dev", true}, {"kaputt", "0.9.0", false},
	} {
		if istNeuer(f.neu, f.alt) != f.soll {
			t.Errorf("istNeuer(%q, %q) != %v", f.neu, f.alt, f.soll)
		}
	}
	if versionText("0.9.0") != "v0.9.0" || versionText("dev") != "dev" {
		t.Fatal(versionText("0.9.0"))
	}
}

func TestNeuigkeiten(t *testing.T) {
	text := "# Runner changelog\n\nIntro.\n\n## 0.9.2 (6 Nov)\n\n- c\n\n## 0.9.1 (5 Nov)\n\n- b\n  more\n\n## 0.9.0 (4 Nov)\n\n- a\n"
	got := strings.Join(neuigkeiten(text, "0.9.0", "0.9.2"), "|")
	if want := "0.9.2 (6 Nov)||- c||0.9.1 (5 Nov)||- b|  more"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := neuigkeiten(text, "0.9.1", "0.9.1"); len(got) != 0 {
		t.Fatalf("nothing new expected, got %q", got)
	}
	if got := strings.Join(neuigkeiten(text, "0.1.202611041358", "0.9.0"), "|"); got != "0.9.0 (4 Nov)||- a" {
		t.Fatalf("from a date build: %q", got)
	}
}

func TestVersionAus(t *testing.T) {
	for ein, want := range map[string]string{"swarvex runner v0.9.1\n": "0.9.1", "swarvex runner 0.1.202611041358": "0.1.202611041358",
		"swarvex runner dev": "dev", "Unknown command": "", "": ""} {
		if got := versionAus(ein); got != want {
			t.Errorf("versionAus(%q) = %q, want %q", ein, got, want)
		}
	}
}

func TestOffeneFragen(t *testing.T) {
	ohneDienst := &einrichtungStand{work: []string{"--project", "auto"}}
	if len(offeneFragen(ohneDienst, nil)) != 0 {
		t.Fatal("without a service there is no auto-update question")
	}
	mit := &einrichtungStand{mitDienst: true, dienst: []string{"--daily-budget-tokens", "0", "--auto-update"}}
	if len(offeneFragen(mit, nil)) != 0 {
		t.Fatal("auto-update already on")
	}
	aus := &einrichtungStand{mitDienst: true, dienst: []string{"--daily-budget-tokens", "0"}}
	if len(offeneFragen(aus, nil)) != 1 {
		t.Fatal("auto-update never asked: expected one open question")
	}
	datei := filepath.Join(t.TempDir(), "asked.json")
	gefragtMerken(datei, "auto-update")
	gefragtMerken(datei, "spaeter")
	g := gefragtLesen(datei)
	if !g["auto-update"] || !g["spaeter"] || len(offeneFragen(aus, g)) != 0 {
		t.Fatalf("answered (no) must not be asked again: %v", g)
	}
}

// Ein gespielter Installer: legt wie install.sh release/ und ~/.local/bin/swarvex an; das Programm meldet meldet.
func spielInstaller(meldet string) string {
	return `set -e
R="$XDG_DATA_HOME/swarvex/release"; rm -rf "$R"; mkdir -p "$R" "$HOME/.local/bin"
printf '#!/bin/sh\necho "swarvex runner ` + meldet + `"\n' > "$R/swarvex"; chmod 755 "$R/swarvex"
install -m 755 "$R/swarvex" "$HOME/.local/bin/swarvex"
`
}

func TestSicherInstallieren(t *testing.T) {
	heimat := t.TempDir()
	t.Setenv("HOME", heimat)
	t.Setenv("XDG_DATA_HOME", filepath.Join(heimat, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(heimat, "config"))
	gebaut := 0
	altBauen, altVersion := imagesBauen, version
	imagesBauen = func(string, io.Writer) error { gebaut++; return nil }
	defer func() { imagesBauen, version = altBauen, altVersion }()
	version = "0.9.0"
	var aus strings.Builder
	if err := installieren(spielInstaller("v0.9.0"), "", &aus); err != nil {
		t.Fatal(err, aus.String())
	}

	// Die neue Version startet nicht richtig: die alte kommt zurück, mit ihren Images.
	err := sicherInstallieren(spielInstaller("kaputt"), "", "0.9.1", &aus)
	if !errors.Is(err, errStartetNicht) {
		t.Fatalf("expected errStartetNicht, got %v", err)
	}
	if v, _ := programmVersion(installiertesProgramm()); v != "0.9.0" || gebaut != 1 {
		t.Fatalf("after a broken update: version %q, images built %d times\n%s", v, gebaut, aus.String())
	}
	// Der Download scheitert, bevor etwas ersetzt ist: zurück ohne Images neu zu bauen.
	if err := sicherInstallieren("exit 3", "", "0.9.1", &aus); err == nil || gebaut != 1 {
		t.Fatalf("failed download: err %v, images built %d times", err, gebaut)
	}
	if _, err := os.Stat(filepath.Join(releaseOrdner(), "swarvex")); err != nil {
		t.Fatal("release folder must be back after a failed download")
	}
	// Klappt: neue Version installiert, die alte liegt in release.prev.
	if err := sicherInstallieren(spielInstaller("v0.9.1"), "", "0.9.1", &aus); err != nil {
		t.Fatal(err, aus.String())
	}
	if v, _ := programmVersion(filepath.Join(releaseOrdner()+".prev", "swarvex")); v != "0.9.0" {
		t.Fatalf("previous version kept: %q", v)
	}
	// Von Hand zurück (--rollback ohne Dienst): wieder 0.9.0, Images neu.
	if err := zurueckholen(&aus, true); err != nil {
		t.Fatal(err)
	}
	if v, _ := programmVersion(installiertesProgramm()); v != "0.9.0" || gebaut != 2 {
		t.Fatalf("after rollback: %q, images built %d times", v, gebaut)
	}
	if zurueckholen(&aus, true) == nil {
		t.Fatal("a second rollback has nothing to go back to")
	}
}

// Schlüsselwechsel: Ein neuer Release-Schlüssel gilt nur, wenn der gemerkte ihn signiert hat.
func TestSchluesselwechsel(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not installed")
	}
	dir := t.TempDir()
	neuerSchluessel := func(name string) (string, string) {
		datei := filepath.Join(dir, name)
		if aus, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", name, "-f", datei).CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, aus)
		}
		pub, _ := os.ReadFile(datei + ".pub")
		felder := strings.Fields(string(pub))
		return datei, felder[0] + " " + felder[1]
	}
	altDatei, alt := neuerSchluessel("alt")
	_, neu := neuerSchluessel("neu")
	fremdDatei, _ := neuerSchluessel("fremd")
	signieren := func(schluessel, text string) string {
		cmd := exec.Command("ssh-keygen", "-Y", "sign", "-f", schluessel, "-n", "swarvex-key-rotation")
		cmd.Stdin = strings.NewReader(text)
		sig, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		return base64.StdEncoding.EncodeToString(sig)
	}
	if !schluesselwechselGeprueft(alt, neu, signieren(altDatei, neu)) {
		t.Fatal("a new key signed with the pinned key must be accepted")
	}
	if schluesselwechselGeprueft(alt, neu, signieren(fremdDatei, neu)) {
		t.Fatal("a new key signed by someone else must be refused")
	}
	if schluesselwechselGeprueft(alt, neu, signieren(altDatei, neu+"x")) {
		t.Fatal("a signature over different text must be refused")
	}
	if schluesselwechselGeprueft(alt, neu, "") || schluesselwechselGeprueft("", neu, signieren(altDatei, neu)) {
		t.Fatal("without a signature or a pinned key nothing changes")
	}
	_, _, w := installerLesen("VERSION=\"0.9.3\"\nSIGNING_KEY=\"" + neu + "\"\nKEY_ROTATION_SIGNATURE=\"abc\"\n")
	if w != "abc" {
		t.Fatalf("rotation signature not read: %q", w)
	}
}
