package main

// swarvex update: holt die aktuelle Version von der Plattform, ohne den Einzeiler erneut einzufügen.
// Es läuft dasselbe install.sh wie beim Einzeiler (Signatur- und Prüfsummenprüfung inklusive), nur ohne
// die Einrichtungsfragen; die Einstellungen bleiben. Zusätzlich muss der Signaturschlüssel der Plattform
// derselbe sein wie bei der Installation (release.pub), sonst wird nichts installiert.

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// version setzt release.sh beim Bauen aus runner/VERSION (-ldflags -X main.version=…), z. B. 0.9.0.
var version = "dev"

// versionText: so steht die Version vor Menschen, z. B. „v0.9.0“.
func versionText(v string) string {
	if v != "" && v[0] >= '0' && v[0] <= '9' {
		return "v" + v
	}
	return v
}

func versionBefehl() { fmt.Println("swarvex runner", versionText(version)) }

// istNeuer vergleicht Versionen Teil für Teil als Zahlen (0.10.0 > 0.9.3 > 0.1.202611041358);
// eine eigene Entwicklerversion ("dev") gilt als älter als jede veröffentlichte.
func istNeuer(neu, alt string) bool {
	zahlen := func(v string) ([]int, bool) {
		teile := strings.Split(strings.TrimPrefix(v, "v"), ".")
		r := make([]int, len(teile))
		for i, t := range teile {
			n, err := strconv.Atoi(t)
			if err != nil {
				return nil, false
			}
			r[i] = n
		}
		return r, true
	}
	n, okN := zahlen(neu)
	a, okA := zahlen(alt)
	switch {
	case !okN:
		return false
	case !okA:
		return true
	}
	for i := 0; i < max(len(n), len(a)); i++ {
		x, y := 0, 0
		if i < len(n) {
			x = n[i]
		}
		if i < len(a) {
			y = a[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

var installWert = regexp.MustCompile(`(?m)^(VERSION|SIGNING_KEY)="([^"\n]*)"`)

// installerLesen liefert Version und Signaturschlüssel aus dem install.sh der Plattform.
func installerLesen(skript string) (ver, schluessel string) {
	for _, m := range installWert.FindAllStringSubmatch(skript, -1) {
		if m[1] == "VERSION" {
			ver = m[2]
		} else {
			schluessel = m[2]
		}
	}
	return
}

func pinDatei() string { return filepath.Join(konfigOrdner(), "release.pub") }

// schluesselPasst: ohne gemerkten Schlüssel (Installation vor dieser Version) gilt der aktuelle.
func schluesselPasst(gemerkt, neu string) bool {
	gemerkt = strings.TrimSpace(gemerkt)
	return gemerkt == "" || gemerkt == strings.TrimSpace(neu)
}

func installerHolen(plattform, ca string) (string, error) {
	c := httpClient(ca)
	c.Timeout = 15 * time.Second
	r, err := c.Get(strings.TrimRight(plattform, "/") + "/install.sh")
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", r.StatusCode)
	}
	roh, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	return string(roh), err
}

// plattformUndCA: wo der Runner angemeldet ist (oder --platform).
func plattformUndCA(vorgabe string) (string, string) {
	plattform, ca := vorgabe, ""
	if z, err := zugangLaden(); err == nil {
		if plattform == "" {
			plattform = z.Plattform
		}
		ca = z.CAPfad
	}
	return plattform, ca
}

// neuesteVersion liest install.sh der Plattform: Skript, Version und ob der Signaturschlüssel der gemerkte ist.
func neuesteVersion(plattform, ca string) (skript, neu string, err error) {
	skript, err = installerHolen(plattform, ca)
	if err != nil {
		return "", "", fmt.Errorf("could not fetch %s/install.sh: %w", plattform, err)
	}
	neu, schluessel := installerLesen(skript)
	if neu == "" || schluessel == "" {
		return "", "", fmt.Errorf("the platform's install.sh has no version or signing key")
	}
	gemerkt, _ := os.ReadFile(pinDatei())
	if !schluesselPasst(string(gemerkt), schluessel) {
		return "", neu, fmt.Errorf("the platform's release signing key is not the one you installed with; " +
			"if the project announced a new key, install again with the one-liner from the website")
	}
	return skript, neu, nil
}

// installieren führt install.sh ohne Einrichtungsfragen aus (dieselbe Signatur- und Prüfsummenprüfung wie der Einzeiler).
func installieren(skript, ca string, ausgabe io.Writer) error {
	datei, err := os.CreateTemp("", "swarvex-install-*.sh")
	if err != nil {
		return err
	}
	defer os.Remove(datei.Name())
	_, err = datei.WriteString(skript)
	datei.Close()
	if err != nil {
		return err
	}
	install := exec.Command("sh", datei.Name())
	install.Stdout, install.Stderr = ausgabe, ausgabe
	install.Env = append(os.Environ(), "SWARVEX_NO_SETUP=1")
	if ca != "" {
		install.Env = append(install.Env, "SWARVEX_CA="+ca)
	}
	return install.Run()
}

func updateBefehl(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	nurSehen := fs.Bool("check", false, "only show whether a newer version exists")
	erzwingen := fs.Bool("force", false, "install even if this version is already installed")
	sofort := fs.Bool("now", false, "restart the service right away, even during a task (the task is handed back)")
	plattformWert := fs.String("platform", "", "platform address (default: where you are signed in)")
	fs.Parse(args)

	plattform, ca := plattformUndCA(*plattformWert)
	if plattform == "" {
		fehlerAus("Not signed in: pass --platform https://swarvex.ai or run the one-liner from the website.")
	}
	skript, neu, err := neuesteVersion(plattform, ca)
	if neu != "" {
		fmt.Printf("Installed: %s · on %s: %s\n", versionText(version), plattform, versionText(neu))
	}
	if err != nil {
		fehlerAus(err.Error() + "; nothing was installed.")
	}
	if !istNeuer(neu, version) && !*erzwingen {
		fmt.Println("✓ Up to date.")
		return
	}
	if *nurSehen {
		fmt.Println("Update with: swarvex update")
		return
	}
	if installieren(skript, ca, os.Stdout) != nil {
		fehlerAus("The update did not finish; the previous version keeps running.")
	}
	dienstNeuStarten(*sofort, "the new version")
}

// autoUpdate: im Dauerbetrieb höchstens einmal am Tag, zwischen zwei Aufgaben. Danach lädt sich die Schleife neu.
func autoUpdate() {
	plattform, ca := plattformUndCA("")
	if plattform == "" {
		return
	}
	skript, neu, err := neuesteVersion(plattform, ca)
	switch {
	case err != nil:
		fmt.Println("Automatic update skipped:", err)
	case istNeuer(neu, version):
		fmt.Printf("Automatic update: %s → %s\n", versionText(version), versionText(neu))
		if err := installieren(skript, ca, os.Stdout); err != nil {
			fmt.Println("Automatic update did not finish; continuing with", versionText(version))
		}
	}
}

// dienstNeuStarten: Ohne laufende Aufgabe startet der Dienst gleich neu. Mit Aufgabe läuft sie zu Ende; danach
// lädt sich der Dauerbetrieb selbst neu (neues Programm oder geänderte Dienst-Einheit, siehe dauerbetrieb).
func dienstNeuStarten(sofort bool, was string) {
	if _, err := os.Stat(dienstDatei()); err != nil {
		return
	}
	if zustand, _ := systemctl("is-active", dienstName); zustand != "active" {
		return
	}
	if aufgabeLaeuft() && !sofort {
		fmt.Printf("A task is running: it finishes as it is, the service then continues with %s.\n", was)
		fmt.Println("  To switch right away (the task is handed back), add --now")
		return
	}
	if aus, err := systemctl("restart", dienstName); err != nil {
		fehlerAus("restarting the service: " + aus)
	}
	fmt.Printf("Service restarted with %s.\n", was)
}

// aufgabeLaeuft: Jede Aufgabe hat ihren eigenen Schlüssel-Proxy-Container.
func aufgabeLaeuft() bool {
	aus, err := exec.Command("podman", "ps", "-q", "--filter", "name=^swarvex-proxy-").Output()
	return err == nil && strings.TrimSpace(string(aus)) != ""
}

// neueVersionInstalliert: Hat `swarvex update` das Programm seit dem Start ersetzt?
func neueVersionInstalliert(programm string, start os.FileInfo) bool {
	jetzt, err := os.Stat(programm)
	return err == nil && start != nil && !os.SameFile(start, jetzt)
}
