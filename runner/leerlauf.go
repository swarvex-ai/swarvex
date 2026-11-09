package main

// Nur im Leerlauf und nur bestimmte Anliegen (0.13.4, neVs 09.11.2026, angeregt von Folding@home).
//
// Leerlauf (--only-when-idle): Im Dauerbetrieb beginnt der Runner Arbeit nur, solange niemand den Rechner benutzt.
// Erkannt über systemd-logind: jede grafische Sitzung dieses Benutzers ist gesperrt (LockedHint) oder untätig
// (IdleHint, setzt die Arbeitsumgebung nach ihrer eigenen Leerlaufzeit). Ein Rechner ohne grafische Sitzung (Server)
// gilt als frei, ebenso einer ohne logind. Eine laufende Aufgabe wird fertig; neue warten.
// Sinnvoll nur, wenn das Modell auf diesem Rechner rechnet (neVs 09.11.): Bei einem Modell in der Cloud (API,
// OpenRouter, Abo) laufen hier nur Sandbox, Builds und Tests; der Leerlauf kostete dann vor allem Agentenzeit. Das Setup
// fragt deshalb nur bei einem Modell auf diesem Rechner, `swarvex budget --when-idle on` sagt es sonst dazu.
//
// Anliegen (--kinds): Bei „irgendein Projekt“ nur Projekte dieser Arten, etwa Spiele oder Forschung.

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// nurLeerlauf: --only-when-idle (gesetzt in `swarvex work`, gilt im Dauerbetrieb).
var nurLeerlauf bool

// loginctl: im Test ersetzt.
var loginctl = func(args ...string) ([]byte, error) { return exec.Command("loginctl", args...).Output() }

// rechnerFrei: benutzt gerade niemand diesen Rechner? Sonst false (eine aktive, nicht gesperrte, nicht untätige
// grafische Sitzung dieses Benutzers).
func rechnerFrei() bool {
	liste, err := loginctl("list-sessions", "--no-legend")
	if err != nil {
		return true
	}
	uid := strconv.Itoa(os.Getuid())
	for _, zeile := range strings.Split(strings.TrimSpace(string(liste)), "\n") {
		f := strings.Fields(zeile)
		if len(f) < 2 || f[1] != uid {
			continue
		}
		aus, err := loginctl("show-session", f[0], "-p", "Type", "-p", "Active", "-p", "IdleHint", "-p", "LockedHint")
		if err != nil {
			continue
		}
		w := map[string]string{}
		for _, z := range strings.Split(string(aus), "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(z), "="); ok {
				w[k] = v
			}
		}
		if (w["Type"] == "x11" || w["Type"] == "wayland" || w["Type"] == "mir") && w["Active"] == "yes" && w["LockedHint"] != "yes" && w["IdleHint"] != "yes" {
			return false
		}
	}
	return true
}

// modellHier: rechnet das Modell auf diesem Rechner (Ollama, llama.cpp, LM Studio …, auch unter Windows neben WSL)?
// args: gespeicherte Angaben mit --model-url. Ohne Adresse (Abo, Claude Code, Codex) rechnet es in der Cloud.
func modellHier(args []string) bool {
	adresse, ok := argWert(args, "model-url")
	if !ok || adresse == "" {
		return false
	}
	u, err := url.Parse(adresse)
	if err != nil {
		return false
	}
	if h := u.Hostname(); h == "host.containers.internal" || enthaelt(windowsAdressen(), h) {
		return true
	}
	_, hier := sandboxAdresse(adresse)
	return hier
}

// grafischeSitzung: läuft auf diesem Rechner ein Desktop dieses Benutzers? (Nur dann fragt das Setup nach dem Leerlauf.)
func grafischeSitzung() bool {
	if os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("DISPLAY") != "" {
		return true
	}
	liste, err := loginctl("list-sessions", "--no-legend")
	if err != nil {
		return false
	}
	uid := strconv.Itoa(os.Getuid())
	for _, zeile := range strings.Split(strings.TrimSpace(string(liste)), "\n") {
		if f := strings.Fields(zeile); len(f) >= 2 && f[1] == uid {
			if aus, err := loginctl("show-session", f[0], "-p", "Type"); err == nil {
				t := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(aus)), "Type="))
				if t == "x11" || t == "wayland" || t == "mir" {
					return true
				}
			}
		}
	}
	return false
}

// ---- Anliegen ----

// anliegenArten: Anliegen → Projektarten der Plattform (project_type).
var anliegenArten = []struct {
	Name, Titel string
	Arten       []string
}{
	{"games", "Games", []string{"game"}},
	{"apps", "Apps and tools", []string{"app", "website", "service", "tool", "plugin"}},
	{"libraries", "Libraries and asset catalogues", []string{"library"}},
	{"research", "Research and science", []string{"research"}},
	{"other", "Other", []string{"other"}},
}

// bevorzugteAnliegen: --kinds (leer: alle).
var bevorzugteAnliegen []string

// anliegenLesen: "games, research" → ["games", "research"]; Fehler bei Unbekanntem.
func anliegenLesen(text string) ([]string, error) {
	var r []string
	for _, t := range strings.Split(text, ",") {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || t == "all" {
			continue
		}
		bekannt := false
		for _, a := range anliegenArten {
			bekannt = bekannt || a.Name == t
		}
		if !bekannt {
			return nil, fmt.Errorf("unknown kind %q (known: %s, or all)", t, anliegenNamen())
		}
		if !enthaelt(r, t) {
			r = append(r, t)
		}
	}
	return r, nil
}

func anliegenNamen() string {
	n := make([]string, len(anliegenArten))
	for i, a := range anliegenArten {
		n[i] = a.Name
	}
	return strings.Join(n, ", ")
}

// passtZuAnliegen: gehört ein Projekt dieser Art zu den gewählten Anliegen? Ohne Wahl immer.
func passtZuAnliegen(projektArt string, anliegen []string) bool {
	if len(anliegen) == 0 {
		return true
	}
	for _, a := range anliegenArten {
		if enthaelt(anliegen, a.Name) && enthaelt(a.Arten, projektArt) {
			return true
		}
	}
	return false
}

// anliegenFragen: im Setup unter „jede Einstellung selbst“; Nummern durch Komma, leer = alle.
func anliegenFragen(f *frager) string {
	fmt.Println("  Kinds of projects:")
	for i, a := range anliegenArten {
		fmt.Printf("    %d) %s\n", i+1, a.Titel)
	}
	for {
		eingabe := strings.TrimSpace(f.frage("Which ones? Numbers separated by commas", "1"))
		var namen []string
		ok := eingabe != ""
		for _, t := range strings.Split(eingabe, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(t))
			if err != nil || n < 1 || n > len(anliegenArten) {
				ok = false
				break
			}
			if !enthaelt(namen, anliegenArten[n-1].Name) {
				namen = append(namen, anliegenArten[n-1].Name)
			}
		}
		if ok {
			return strings.Join(namen, ",")
		}
		fmt.Printf("  Please enter numbers from 1 to %d, e.g. 1,4\n", len(anliegenArten))
	}
}
