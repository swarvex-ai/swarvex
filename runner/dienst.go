package main

// Dienstbetrieb und Aufräumen:
//
//	swarvex service <Angaben wie bei work>      Runner als Dienst einrichten (Dauerbetrieb, startet mit dem Rechner)
//	swarvex start | stop | status               Dienst starten, anhalten (laufende Aufgabe geht zurück), Zustand
//	swarvex uninstall [--yes]                    alles entfernen: Dienst, Container, Images, Daten, Anmeldung
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
var pfadFlags = map[string]bool{"model-key-file": true, "skills": true}

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
		teile := strings.Split(wert, ":") // --skills: mehrere Ordner; "auto" bleibt
		for j, t := range teile {
			if abs, err := filepath.Abs(t); err == nil && t != "auto" {
				teile[j] = abs
			}
		}
		wert = strings.Join(teile, ":")
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
	teile := []string{systemdQuote(programm), "work", "--continuous"}
	for _, a := range args {
		teile = append(teile, systemdQuote(a))
	}
	return `# Set up by "swarvex service". Change: swarvex service <new options>. Remove: swarvex uninstall.
[Unit]
Description=swarvex runner (works on open-source tasks within its time windows)
Documentation=https://swarvex.ai/docs/runner
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=` + strings.Join(teile, " ") + `
Restart=on-failure
RestartSec=60
# On stop the running task also gets SIGTERM: it stops the sandbox and hands itself back.
KillMode=control-group
TimeoutStopSec=120
Nice=10

[Install]
WantedBy=default.target
`
}

func nurLinux() {
	if runtime.GOOS != "linux" {
		fehlerAus("Running as a service is currently only available on Linux (macOS and Windows will follow). " +
			"Until then, run the runner in the foreground: swarvex work --continuous …")
	}
}

func systemctl(args ...string) (string, error) {
	aus, err := exec.Command("systemctl", append([]string{"--user"}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(aus)), err
}

func dienstBefehl(args []string) {
	nurLinux()
	if len(args) == 0 {
		fehlerAus(`the same options as for "work" are required, e.g.:
  swarvex service --project my-game --model <name> --model-url <url> --schedule "mon-fri 18:00-23:00" --daily-budget-tokens 2000000`)
	}
	args = absolutePfade(args)
	programm, err := os.Executable()
	pruefen(err)
	if programm, err = filepath.EvalSymlinks(programm); err != nil {
		fehlerAus(err.Error())
	}
	// Angaben vorher prüfen, damit der Dienst nicht in einer Neustart-Schleife landet.
	pruefung := exec.Command(programm, append([]string{"work", "--check-only"}, ohneDauerbetrieb(args)...)...)
	pruefung.Stdout, pruefung.Stderr = os.Stdout, os.Stderr
	if pruefung.Run() != nil {
		os.Exit(2)
	}
	if _, err := zugangLaden(); err != nil {
		fehlerAus("Not signed in yet: run `swarvex login` first.")
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
		fehlerAus("starting the service: " + aus)
	}
	fmt.Println("Service set up and started:", datei)
	fmt.Println("  State: swarvex status · Stop: swarvex stop · Log: journalctl --user -u swarvex -f")
	if aus, _ := exec.Command("loginctl", "show-user", os.Getenv("USER"), "-p", "Linger", "--value").Output(); strings.TrimSpace(string(aus)) != "yes" {
		fmt.Println("Note: the service only runs while you are logged in. To keep it working after a reboot")
		fmt.Println("without a login, run once: loginctl enable-linger")
	}
}

func startBefehl() {
	nurLinux()
	if _, err := os.Stat(dienstDatei()); err != nil {
		fehlerAus("No service set up: run `swarvex service …` first.")
	}
	if aus, err := systemctl("start", dienstName); err != nil {
		fehlerAus(aus)
	}
	fmt.Println("Service is running.")
}

func stopBefehl() {
	nurLinux()
	if _, err := os.Stat(dienstDatei()); err != nil {
		fmt.Println("No service set up. A runner in the foreground stops with Ctrl+C.")
		return
	}
	fmt.Println("Stopping … a running task is ended and handed back.")
	if aus, err := systemctl("stop", dienstName); err != nil {
		fehlerAus(aus)
	}
	fmt.Println("Stopped. Continue with: swarvex start")
}

func statusBefehl() {
	fmt.Printf("Usage today: %d tokens\n", verbrauchAmTag(verbrauchsDatei(), time.Now()))
	if z, err := zugangLaden(); err == nil {
		fmt.Println("Signed in at:", z.Plattform)
	} else {
		fmt.Println("Not signed in.")
	}
	if runtime.GOOS != "linux" {
		return
	}
	if _, err := os.Stat(dienstDatei()); err != nil {
		fmt.Println("Service: not set up")
		return
	}
	zustand, _ := systemctl("is-active", dienstName)
	fmt.Println("Service:", zustand)
	if aus, err := exec.Command("journalctl", "--user", "-u", "swarvex", "-n", "5", "--no-pager", "-o", "cat").Output(); err == nil && len(aus) > 0 {
		fmt.Println("Latest messages:")
		fmt.Print(string(aus))
	}
}

// uninstallBefehl entfernt alles, was der Runner angelegt hat, und meldet das Gerät bei der Plattform ab.
func uninstallBefehl(args []string) {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	ja := fs.Bool("yes", false, "do not ask")
	fs.Parse(args)
	if !*ja {
		fmt.Print("Remove the runner, service, containers, images, work data and sign-in? [y/N] ")
		antwort, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(antwort)); a != "y" && a != "yes" && a != "j" && a != "ja" {
			fmt.Println("Cancelled, nothing removed.")
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
			schritt("service stopped and removed", err)
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
		schritt("sign-in at "+z.Plattform+" revoked", err)
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
		schritt(fmt.Sprintf("containers, %d images and network removed", len(images)), err)
	}
	// 4. Arbeitsdaten (Snapshots, Caches, Protokolle, Verbrauch) und Konfiguration mit Anmeldung
	schritt("work data removed ("+datenOrdner()+")", os.RemoveAll(datenOrdner()))
	schritt("configuration and sign-in removed ("+konfigOrdner()+")", os.RemoveAll(konfigOrdner()))
	// Ordner unter dem alten Namen (vor der Umbenennung in swarvex), falls noch vorhanden
	for _, alt := range []string{filepath.Join(filepath.Dir(konfigOrdner()), "schwarm-runner"), filepath.Join(filepath.Dir(datenOrdner()), "schwarm-runner")} {
		if _, err := os.Stat(alt); err == nil {
			schritt("old folder removed ("+alt+")", os.RemoveAll(alt))
		}
	}
	// 5. Das Programm selbst
	if programm, err := os.Executable(); err == nil {
		programm, _ = filepath.EvalSymlinks(programm)
		schritt("program removed ("+programm+")", os.Remove(programm))
	}
	fmt.Println("Done. The runner never copied your model key (--model-key-file); that file stays where it is.")
}
