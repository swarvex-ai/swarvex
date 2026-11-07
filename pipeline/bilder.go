package main

// Bildvergleich im Review (E-39): Nach dem Testlauf liest die Prüfstraße die Szenenbilder, die die Tests geschrieben
// haben (swarvex.yaml images, bei three.js tests/scene/out), und vergleicht sie mit den Referenzbildern des
// Basis-Commits (images_baseline). Nur Szenen, deren Bild sich ändert oder neu ist, gehen an die Plattform: alt, neu und
// die Differenz, die die Tests selbst erzeugen (<name>.diff.png).

import (
	"bytes"
	"encoding/base64"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	bilderMax      = 12
	bildGroesseMax = 400_000
)

var szenenName = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,60}$`)

type szenenBild struct {
	Name           string
	Alt, Neu, Diff []byte
}

func (s szenenBild) daten() map[string]any {
	d := map[string]any{"name": s.Name, "new": base64.StdEncoding.EncodeToString(s.Neu)}
	if s.Alt != nil {
		d["old"] = base64.StdEncoding.EncodeToString(s.Alt)
	}
	if s.Diff != nil {
		d["diff"] = base64.StdEncoding.EncodeToString(s.Diff)
	}
	return d
}

// szenenBilder: geänderte oder neue Szenenbilder im Arbeitsbaum, verglichen mit dem Referenzbild im Basis-Commit.
func szenenBilder(baum, ordner, bilder, referenzen, basis string) ([]szenenBild, error) {
	if bilder == "" || referenzen == "" {
		return nil, nil
	}
	eintraege, err := os.ReadDir(filepath.Join(baum, ordner, bilder))
	if err != nil {
		return nil, nil // keine Bilder in diesem Lauf
	}
	var namen []string
	for _, e := range eintraege {
		n := e.Name()
		if e.Type().IsRegular() && strings.HasSuffix(n, ".png") && !strings.HasSuffix(n, ".diff.png") && szenenName.MatchString(strings.TrimSuffix(n, ".png")) {
			namen = append(namen, strings.TrimSuffix(n, ".png"))
		}
	}
	sort.Strings(namen)
	lesen := func(datei string) []byte {
		info, err := os.Lstat(datei)
		if err != nil || !info.Mode().IsRegular() || info.Size() > bildGroesseMax {
			return nil
		}
		roh, _ := os.ReadFile(datei)
		return roh
	}
	var aus []szenenBild
	for _, n := range namen {
		neu := lesen(filepath.Join(baum, ordner, bilder, n+".png"))
		if neu == nil {
			continue
		}
		// Referenz vom Basis-Commit: ohne Beitrag, auch wenn der Beitrag die Referenz selbst ändert
		alt, err := exec.Command("git", "-C", baum, "show", basis+":"+path.Join(filepath.ToSlash(ordner), referenzen, n+".png")).Output()
		if err != nil || len(alt) == 0 || len(alt) > bildGroesseMax {
			alt = nil
		}
		if alt != nil && bytes.Equal(alt, neu) {
			continue
		}
		aus = append(aus, szenenBild{Name: n, Alt: alt, Neu: neu, Diff: lesen(filepath.Join(baum, ordner, bilder, n+".diff.png"))})
		if len(aus) == bilderMax {
			break
		}
	}
	return aus, nil
}
