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
	"strings"
	"time"
)

// version setzt release.sh beim Bauen (-ldflags -X main.version=…).
var version = "dev"

func versionBefehl() { fmt.Println("swarvex runner", version) }

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

func updateBefehl(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	nurSehen := fs.Bool("check", false, "only show whether a newer version exists")
	erzwingen := fs.Bool("force", false, "install even if this version is already installed")
	sofort := fs.Bool("now", false, "restart the service right away, even during a task (the task is handed back)")
	plattformWert := fs.String("platform", "", "platform address (default: where you are signed in)")
	fs.Parse(args)

	plattform, ca := *plattformWert, ""
	if z, err := zugangLaden(); err == nil {
		if plattform == "" {
			plattform = z.Plattform
		}
		ca = z.CAPfad
	}
	if plattform == "" {
		fehlerAus("Not signed in: pass --platform https://swarvex.ai or run the one-liner from the website.")
	}
	skript, err := installerHolen(plattform, ca)
	if err != nil {
		fehlerAus("Could not fetch " + plattform + "/install.sh: " + err.Error())
	}
	neu, schluessel := installerLesen(skript)
	if neu == "" || schluessel == "" {
		fehlerAus("The platform's install.sh has no version or signing key; nothing was installed.")
	}
	fmt.Printf("Installed: %s · on %s: %s\n", version, plattform, neu)
	if neu == version && !*erzwingen {
		fmt.Println("✓ Up to date.")
		return
	}
	if *nurSehen {
		fmt.Println("Update with: swarvex update")
		return
	}
	gemerkt, _ := os.ReadFile(pinDatei())
	if !schluesselPasst(string(gemerkt), schluessel) {
		fehlerAus("The platform's release signing key is not the one you installed with; nothing was installed.\n" +
			"  If the project announced a new key, install again with the one-liner from the website.")
	}

	datei, err := os.CreateTemp("", "swarvex-install-*.sh")
	pruefen(err)
	defer os.Remove(datei.Name())
	_, err = datei.WriteString(skript)
	datei.Close()
	pruefen(err)
	install := exec.Command("sh", datei.Name())
	install.Stdin, install.Stdout, install.Stderr = os.Stdin, os.Stdout, os.Stderr
	install.Env = append(os.Environ(), "SWARVEX_NO_SETUP=1")
	if ca != "" {
		install.Env = append(install.Env, "SWARVEX_CA="+ca)
	}
	if install.Run() != nil {
		fehlerAus("The update did not finish; the previous version keeps running.")
	}
	dienstNachUpdate(*sofort)
}

// dienstNachUpdate: Ohne laufende Aufgabe startet der Dienst gleich neu. Mit Aufgabe läuft sie zu Ende;
// die nächste startet schon mit der neuen Version, und der Dauerbetrieb lädt sich danach selbst neu.
func dienstNachUpdate(sofort bool) {
	if _, err := os.Stat(dienstDatei()); err != nil {
		return
	}
	if zustand, _ := systemctl("is-active", dienstName); zustand != "active" {
		return
	}
	if aufgabeLaeuft() && !sofort {
		fmt.Println("A task is running: it finishes with the current version, the next one starts with the new one.")
		fmt.Println("  To switch right away (the task is handed back): swarvex update --now")
		return
	}
	if aus, err := systemctl("restart", dienstName); err != nil {
		fehlerAus("restarting the service: " + aus)
	}
	fmt.Println("Service restarted with the new version.")
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
