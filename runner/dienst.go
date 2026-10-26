package main

// Dienstbetrieb und Aufräumen:
//
//	swarvex dienst <Angaben wie bei arbeiten>   Runner als Dienst einrichten (Dauerbetrieb, startet mit dem Rechner)
//	swarvex start | stop | status               Dienst starten, anhalten (laufende Aufgabe geht zurück), Zustand
//	swarvex uninstall [--ja]                    alles entfernen: Dienst, Container, Images, Daten, Anmeldung
//
// Linux: systemd-Nutzerdienst (kein root). macOS (launchd) und Windows folgen.

import (
	"bufio"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const dienstName = "swarvex.service"

func dienstDatei() string {
	basis := os.Getenv("XDG_CONFIG_HOME")
	if basis == "" {
		h, _ := os.UserHomeDir()
		basis = filepath.Join(h, ".config")
	}
	return filepath.Join(basis, "systemd", "user", dienstName)
}

// pfadFlags enthalten Dateien; im Dienst gilt ein anderes Arbeitsverzeichnis, deshalb absolut machen.
var pfadFlags = map[string]bool{"modell-key-datei": true}

// absolutePfade macht Dateiangaben absolut (--x wert und --x=wert).
func absolutePfade(args []string) []string {
	r := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, wert, mitGleich := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if !strings.HasPrefix(a, "-") || !pfadFlags[name] {
			r = append(r, a)
			continue
		}
		if !mitGleich {
			if i+1 >= len(args) {
				r = append(r, a)
				continue
			}
			i++
			wert = args[i]
		}
		if abs, err := filepath.Abs(wert); err == nil {
			wert = abs
		}
		r = append(r, "--"+name+"="+wert)
	}
	return r
}

// systemdQuote setzt ein Argument für ExecStart in Anführungszeichen (systemd.syntax).
func systemdQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"'\\$%;") {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", "$$", "%", "%%")
	return `"` + r.Replace(s) + `"`
}

func dienstEinheit(programm string, args []string) string {
	teile := []string{systemdQuote(programm), "arbeiten", "--dauerbetrieb"}
	for _, a := range args {
		teile = append(teile, systemdQuote(a))
	}
	return `# Eingerichtet von „swarvex dienst“. Ändern: swarvex dienst <neue Angaben>. Entfernen: swarvex uninstall.
[Unit]
Description=swarvex Runner (arbeitet in den Zeitfenstern an Open-Source-Aufgaben)
Documentation=https://swarvex.ai/docs/runner
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=` + strings.Join(teile, " ") + `
Restart=on-failure
RestartSec=60
# Beim Anhalten bekommt auch die laufende Aufgabe SIGTERM: Sie stoppt die Sandbox und gibt sich zurück.
KillMode=control-group
TimeoutStopSec=120
Nice=10

[Install]
WantedBy=default.target
`
}

func nurLinux() {
	if runtime.GOOS != "linux" {
		fehlerAus("Der Dienstbetrieb ist im Moment nur unter Linux verfügbar (macOS und Windows folgen). " +
			"Starte den Runner bis dahin im Vordergrund: swarvex arbeiten --dauerbetrieb …")
	}
}

func systemctl(args ...string) (string, error) {
	aus, err := exec.Command("systemctl", append([]string{"--user"}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(aus)), err
}

func dienstBefehl(args []string) {
	nurLinux()
	if len(args) == 0 {
		fehlerAus(`Angaben wie bei „arbeiten“ nötig, z. B.:
  swarvex dienst --projekt mein-spiel --modell <name> --modell-url <url> --zeitfenster "mo-fr 18:00-23:00" --tagesbudget-tokens 2000000`)
	}
	args = absolutePfade(args)
	programm, err := os.Executable()
	pruefen(err)
	if programm, err = filepath.EvalSymlinks(programm); err != nil {
		fehlerAus(err.Error())
	}
	// Angaben vorher prüfen, damit der Dienst nicht in einer Neustart-Schleife landet.
	pruefung := exec.Command(programm, append([]string{"arbeiten", "--nur-pruefen"}, ohneDauerbetrieb(args)...)...)
	pruefung.Stdout, pruefung.Stderr = os.Stdout, os.Stderr
	if pruefung.Run() != nil {
		os.Exit(2)
	}
	if _, err := zugangLaden(); err != nil {
		fehlerAus("Noch nicht angemeldet: zuerst `swarvex login`.")
	}
	datei := dienstDatei()
	pruefen(os.MkdirAll(filepath.Dir(datei), 0o755))
	pruefen(os.WriteFile(datei, []byte(dienstEinheit(programm, ohneDauerbetrieb(args))), 0o644))
	if aus, err := systemctl("daemon-reload"); err != nil {
		fehlerAus("systemctl --user daemon-reload: " + aus)
	}
	if aus, err := systemctl("enable", dienstName); err != nil {
		fehlerAus("systemctl --user enable: " + aus)
	}
	if aus, err := systemctl("restart", dienstName); err != nil {
		fehlerAus("Dienst starten: " + aus)
	}
	fmt.Println("Dienst eingerichtet und gestartet:", datei)
	fmt.Println("  Zustand: swarvex status · Anhalten: swarvex stop · Protokoll: journalctl --user -u swarvex -f")
	if aus, _ := exec.Command("loginctl", "show-user", os.Getenv("USER"), "-p", "Linger", "--value").Output(); strings.TrimSpace(string(aus)) != "yes" {
		fmt.Println("Hinweis: Der Dienst läuft nur, solange du angemeldet bist. Damit er auch nach einem Neustart")
		fmt.Println("ohne Anmeldung arbeitet, einmal ausführen: loginctl enable-linger")
	}
}

func startBefehl() {
	nurLinux()
	if _, err := os.Stat(dienstDatei()); err != nil {
		fehlerAus("Kein Dienst eingerichtet: zuerst `swarvex dienst …`.")
	}
	if aus, err := systemctl("start", dienstName); err != nil {
		fehlerAus(aus)
	}
	fmt.Println("Dienst läuft.")
}

func stopBefehl() {
	nurLinux()
	if _, err := os.Stat(dienstDatei()); err != nil {
		fmt.Println("Kein Dienst eingerichtet. Ein Runner im Vordergrund stoppt mit Strg+C.")
		return
	}
	fmt.Println("Halte an … eine laufende Aufgabe wird beendet und zurückgegeben.")
	if aus, err := systemctl("stop", dienstName); err != nil {
		fehlerAus(aus)
	}
	fmt.Println("Angehalten. Weiter mit: swarvex start")
}

func statusBefehl() {
	fmt.Printf("Verbrauch heute: %d Tokens\n", verbrauchAmTag(verbrauchsDatei(), time.Now()))
	if z, err := zugangLaden(); err == nil {
		fmt.Println("Angemeldet bei:", z.Plattform)
	} else {
		fmt.Println("Nicht angemeldet.")
	}
	if runtime.GOOS != "linux" {
		return
	}
	if _, err := os.Stat(dienstDatei()); err != nil {
		fmt.Println("Dienst: nicht eingerichtet")
		return
	}
	zustand, _ := systemctl("is-active", dienstName)
	fmt.Println("Dienst:", zustand)
	if aus, err := exec.Command("journalctl", "--user", "-u", "swarvex", "-n", "5", "--no-pager", "-o", "cat").Output(); err == nil && len(aus) > 0 {
		fmt.Println("Letzte Meldungen:")
		fmt.Print(string(aus))
	}
}

// uninstallBefehl entfernt alles, was der Runner angelegt hat, und meldet das Gerät bei der Plattform ab.
func uninstallBefehl(args []string) {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	ja := fs.Bool("ja", false, "ohne Rückfrage")
	fs.Parse(args)
	if !*ja {
		fmt.Print("Runner, Dienst, Container, Images, Arbeitsdaten und Anmeldung entfernen? [j/N] ")
		antwort, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(antwort)); a != "j" && a != "ja" && a != "y" && a != "yes" {
			fmt.Println("Abgebrochen, nichts entfernt.")
			return
		}
	}
	schritt := func(text string, err error) {
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", text, err)
		} else {
			fmt.Printf("  ✓ %s\n", text)
		}
	}
	// 1. Dienst anhalten (gibt eine laufende Aufgabe zurück) und entfernen
	if runtime.GOOS == "linux" {
		if _, err := os.Stat(dienstDatei()); err == nil {
			systemctl("disable", "--now", dienstName)
			err := os.Remove(dienstDatei())
			systemctl("daemon-reload")
			schritt("Dienst angehalten und entfernt", err)
		}
	}
	// 2. Gerät bei der Plattform abmelden: Refresh-Token widerrufen (danach gilt auch kein Access-Token mehr lange)
	if z, err := zugangLaden(); err == nil {
		r, err := httpClient(z.CAPfad).PostForm(z.Plattform+"/token/revocation", url.Values{
			"token": {z.RefreshToken}, "token_type_hint": {"refresh_token"}, "client_id": {z.ClientID},
		})
		if err == nil {
			r.Body.Close()
			if r.StatusCode != 200 {
				err = fmt.Errorf("HTTP %d", r.StatusCode)
			}
		}
		schritt("Anmeldung bei "+z.Plattform+" widerrufen", err)
	}
	// 3. Container, Images und Netz in Podman
	if _, err := exec.LookPath("podman"); err == nil {
		if ids, _ := exec.Command("podman", "ps", "-aq", "--filter", "name=^swarvex-").Output(); len(strings.Fields(string(ids))) > 0 {
			exec.Command("podman", append([]string{"rm", "-f"}, strings.Fields(string(ids))...)...).Run()
		}
		var images []string
		if aus, err := exec.Command("podman", "images", "--format", "{{.Repository}}:{{.Tag}}").Output(); err == nil {
			for _, name := range strings.Fields(string(aus)) {
				if strings.HasPrefix(strings.TrimPrefix(name, "localhost/"), "swarvex-") {
					images = append(images, name)
				}
			}
		}
		var err error
		if len(images) > 0 {
			if aus, e := exec.Command("podman", append([]string{"rmi", "-f"}, images...)...).CombinedOutput(); e != nil {
				err = fmt.Errorf("%s", strings.TrimSpace(string(aus)))
			}
		}
		exec.Command("podman", "network", "rm", "-f", "swarvex-intern").Run()
		schritt(fmt.Sprintf("Container, %d Images und Netz entfernt", len(images)), err)
	}
	// 4. Arbeitsdaten (Snapshots, Caches, Protokolle, Verbrauch) und Konfiguration mit Anmeldung
	schritt("Arbeitsdaten entfernt ("+datenOrdner()+")", os.RemoveAll(datenOrdner()))
	schritt("Konfiguration und Anmeldung entfernt ("+konfigOrdner()+")", os.RemoveAll(konfigOrdner()))
	// Ordner unter dem alten Namen (vor der Umbenennung in swarvex), falls noch vorhanden
	for _, alt := range []string{filepath.Join(filepath.Dir(konfigOrdner()), "schwarm-runner"), filepath.Join(filepath.Dir(datenOrdner()), "schwarm-runner")} {
		if _, err := os.Stat(alt); err == nil {
			schritt("Alter Ordner entfernt ("+alt+")", os.RemoveAll(alt))
		}
	}
	// 5. Das Programm selbst
	if programm, err := os.Executable(); err == nil {
		programm, _ = filepath.EvalSymlinks(programm)
		schritt("Programm entfernt ("+programm+")", os.Remove(programm))
	}
	fmt.Println("Fertig. Deinen Modell-Schlüssel (--modell-key-datei) hat der Runner nie kopiert; die Datei bleibt, wo sie ist.")
}
