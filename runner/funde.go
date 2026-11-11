package main

// Fundmeldungen (E-46): Was dem Agenten außerhalb seiner Aufgabe auffällt (ein Fehler daneben, ein fehlender Test, eine
// Datei außerhalb seiner Pfade, die sich ändern sollte), schreibt er nach /work/.swarvex-findings.json, statt es zu tun.
// Der Runner schickt die Meldungen mit Einreichung oder Rückgabe an die Plattform; der Lead macht daraus eine Aufgabe oder
// verwirft sie. Änderungen außerhalb der erlaubten Pfade, die der Runner verwirft, gehen nicht mehr verloren: Ihre Pfade
// (nicht ihr Inhalt) werden zu einer Meldung, ohne erzeugte Dateien wie Import-Caches und Lockfiles.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	fundDatei   = ".swarvex-findings.json"
	fundeMax    = 3
	fundePfade  = 10
	fundTitel   = 120
	fundText    = 1000
	sandboxMaxB = 256 << 10 // so viel liest der Runner höchstens aus einer Datei, die der Agent geschrieben hat
)

// fund: eine Meldung, wie sie an die Plattform geht (submit_result / abandon_task: findings).
type fund struct {
	Titel string   `json:"title"`
	Text  string   `json:"text,omitempty"`
	Pfade []string `json:"paths,omitempty"`
	Stufe string   `json:"level,omitempty"`
}

// sandboxDateiLesen liest eine Datei, die der Agent in der Sandbox geschrieben hat. Der Ordner liegt auf dem Rechner des
// Betreibers: Ein symbolischer Link (etwa auf ~/.ssh/id_ed25519) würde hier, außerhalb der Sandbox, aufgelöst. Deshalb nur
// eine echte Datei, ohne Links zu folgen, und höchstens sandboxMaxB Bytes.
func sandboxDateiLesen(pfad string) ([]byte, error) {
	info, err := os.Lstat(pfad)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	if info.Size() > sandboxMaxB {
		return nil, fmt.Errorf("larger than %d KiB", sandboxMaxB>>10)
	}
	f, err := os.OpenFile(pfad, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, sandboxMaxB+1))
}

// maskiert: dieselben Masken wie für die öffentliche Rückgabe (ursache.go), aber mit Zeilenumbrüchen. Meldungen sind
// nicht öffentlich, verlassen aber den Rechner des Betreibers: keine Adressen, Schlüssel, Kennungen, Home-Ordner.
func maskiert(text string) string {
	text = maskeMail.ReplaceAllString(text, "[address]")
	text = maskeGeheim.ReplaceAllString(text, "[hidden]")
	text = maskeKenn.ReplaceAllStringFunc(text, func(s string) string {
		if strings.ContainsAny(s, "0123456789") {
			return "[hidden]"
		}
		return s
	})
	return maskeHeim.ReplaceAllString(text, "~")
}

func runenKuerzen(text string, n int) string {
	if r := []rune(text); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return text
}

// fundeLesen liest die Meldungen des Agenten und entfernt die Datei aus dem Arbeitsbaum, bevor der Patch entsteht: Sie
// gehört nicht ins Projekt. Hat das Projekt selbst eine Datei dieses Namens (in Git), bleibt sie unberührt. Was nicht
// passt, fällt weg; die Plattform prüft noch einmal.
func fundeLesen(gitDir, baum string) []fund {
	if drin, _ := git(gitDir, baum, "ls-files", "--", fundDatei); strings.TrimSpace(drin) != "" {
		return nil
	}
	pfad := filepath.Join(baum, fundDatei)
	roh, err := sandboxDateiLesen(pfad)
	os.Remove(pfad) // auch ein Link oder eine zu große Datei: weg damit, nur der Link selbst, nie sein Ziel
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Printf("Findings of the agent ignored (%s: %v).\n", fundDatei, err)
		}
		return nil
	}
	var liste []fund
	if json.Unmarshal(roh, &liste) != nil {
		var einer fund // ein einzelnes Objekt statt einer Liste
		if json.Unmarshal(roh, &einer) != nil {
			fmt.Printf("Findings of the agent ignored (%s is not valid JSON).\n", fundDatei)
			return nil
		}
		liste = []fund{einer}
	}
	var r []fund
	for _, f := range liste {
		f.Titel = runenKuerzen(strings.Join(strings.Fields(maskiert(f.Titel)), " "), fundTitel)
		if len([]rune(f.Titel)) < 5 {
			continue
		}
		f.Text = runenKuerzen(strings.TrimSpace(maskiert(f.Text)), fundText)
		var pfade []string
		for _, p := range f.Pfade {
			p = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(p), "/work/"), "./")
			if p != "" && !strings.HasPrefix(p, "/") && !enthaelt(strings.Split(p, "/"), "..") && len(pfade) < fundePfade {
				pfade = append(pfade, maskiert(p))
			}
		}
		f.Pfade = pfade
		f.Stufe = strings.ToUpper(strings.TrimSpace(f.Stufe))
		if !enthaelt([]string{"A", "B", "C", "D"}, f.Stufe) {
			f.Stufe = ""
		}
		r = append(r, f)
		if len(r) == fundeMax {
			break
		}
	}
	return r
}

// erzeugteDatei: Was beim Bauen, Importieren oder Installieren von selbst entsteht, sagt nichts über nötige Arbeit.
func erzeugteDatei(p string) bool {
	for _, endung := range []string{".import", ".uid", ".pyc", ".log", ".tmp", "~", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
		"go.sum", "Cargo.lock", "poetry.lock", "Gemfile.lock", "composer.lock", "packages.lock.json", ".DS_Store"} {
		if strings.HasSuffix(p, endung) {
			return true
		}
	}
	for _, ordner := range []string{".godot/", "node_modules/", "__pycache__/", ".venv/", "Library/", "Temp/", "obj/", ".gradle/", "target/"} {
		if strings.HasPrefix(p, ordner) || strings.Contains(p, "/"+ordner) {
			return true
		}
	}
	return false
}

// verworfenFund: Die verworfenen Änderungen außerhalb der erlaubten Pfade als Meldung an den Lead (nur die Pfade), oder
// nil, wenn nur Erzeugtes darunter war.
func verworfenFund(id int64, titel string, verworfen []string) *fund {
	var echt []string
	for _, p := range verworfen {
		if !erzeugteDatei(p) && p != fundDatei {
			echt = append(echt, p)
		}
	}
	if len(echt) == 0 {
		return nil
	}
	f := &fund{Titel: fmt.Sprintf("Changes outside the allowed paths of task #%d", id), Pfade: echt[:min(len(echt), fundePfade)]}
	mehr := ""
	if len(echt) > fundePfade {
		mehr = fmt.Sprintf(" and %d more", len(echt)-fundePfade)
	}
	f.Text = runenKuerzen(fmt.Sprintf("While working on task #%d (%s), the agent also changed these files outside the task's allowed paths: %s%s. "+
		"The runner discarded the changes (their content stayed on the contributor's machine). They may point to work the project needs.",
		id, titel, strings.Join(f.Pfade, ", "), mehr), fundText)
	return f
}

// fundeZusammen: die Meldungen des Agenten und, falls es eine gibt, die über verworfene Änderungen, zusammen höchstens
// fundeMax (die des Runners zuletzt, aber sicher dabei).
func fundeZusammen(agent []fund, verworfen *fund) []fund {
	if verworfen == nil {
		return agent
	}
	return append(agent[:min(len(agent), fundeMax-1):min(len(agent), fundeMax-1)], *verworfen)
}

// fundeNennen schreibt ins Journal, was an den Lead geht.
func fundeNennen(funde []fund) {
	if len(funde) == 0 {
		return
	}
	titel := make([]string, len(funde))
	for i, f := range funde {
		titel[i] = f.Titel
	}
	fmt.Printf("Findings for the lead (%d): %s\n", len(funde), strings.Join(titel, "; "))
}
