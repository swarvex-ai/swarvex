package main

// Bildvergleich im Review (E-39): Nach dem Testlauf liest die Prüfstraße die Szenenbilder, die die Tests geschrieben
// haben (swarvex.yaml images, bei three.js tests/scene/out), und vergleicht sie mit den Referenzbildern des
// Basis-Commits (images_baseline). Nur Szenen, deren Bild sich ändert oder neu ist, gehen an die Plattform: alt, neu und
// die Differenz, die die Tests selbst erzeugen (<name>.diff.png).

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
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

// --- Szenenbilder aus dem Patch (12.11.2026) ---
//
// Ändert eine Einreichung Referenzbilder, liest der Prüfserver vor jeder Prüfrunde das alte Bild aus dem Basis-Commit
// und das neue aus dem Patch, ohne Testlauf und ohne Checkout (der Patch geht nur in einen eigenen Index am Spiegel),
// und rechnet die Differenz selbst aus. Sind alle geänderten Szenen da, darf auch ein fremder Runner gegenprüfen; die
// Prüfer sehen genau das Bild, das gemergt würde. Fehlt eine (Git-LFS, zu groß, kein PNG), meldet er „unvollständig“,
// und die Prüfstraße prüft und rendert wie bisher.

var referenzPfad = regexp.MustCompile(`(^|/)baselines?/[^/]+\.png$`)

func bilderAusPatch() {
	var liste []einreichung
	if err := signiert("GET", "/pipeline/images-pending", nil, &liste); err != nil {
		fmt.Fprintln(os.Stderr, "scene images from patches:", err)
		return
	}
	for _, e := range liste {
		bilder, vollstaendig := patchBilder(e)
		for _, b := range bilder {
			if err := signiert("POST", fmt.Sprintf("/pipeline/%d/images", e.EinreichungID), b.daten(), nil); err != nil {
				fmt.Fprintf(os.Stderr, "submission %d, scene %s: %v\n", e.EinreichungID, b.Name, err)
				vollstaendig = false
			}
		}
		if err := signiert("POST", fmt.Sprintf("/pipeline/%d/images-done", e.EinreichungID), map[string]any{"complete": vollstaendig}, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		fmt.Printf("submission %d: %d scene images from the patch (complete: %v)\n", e.EinreichungID, len(bilder), vollstaendig)
	}
}

// patchBilder: alt, neu und Differenz je Referenzbild, das der Patch ändert oder anlegt. false, wenn nicht jede
// geänderte Szene geliefert werden kann.
func patchBilder(e einreichung) ([]szenenBild, bool) {
	summe := sha256.Sum256([]byte(e.Patch))
	if hex.EncodeToString(summe[:]) != e.PatchSHA || !commitForm.MatchString(e.BasisCommit) {
		return nil, false
	}
	sp, err := spiegel(e)
	if err != nil {
		return nil, false
	}
	ordner, err := os.MkdirTemp(arbeit, "images-")
	if err != nil {
		return nil, false
	}
	defer os.RemoveAll(ordner)
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(ordner, "index")}
	patchDatei := filepath.Join(ordner, "contribution.patch")
	os.WriteFile(patchDatei, []byte(e.Patch), 0o644)
	if _, err := befehl(sp, env, "git", "read-tree", e.BasisCommit); err != nil {
		return nil, false
	}
	if _, err := befehl(sp, env, "git", "apply", "--cached", patchDatei); err != nil {
		return nil, false
	}
	vollstaendig := true
	var aus []szenenBild
	for _, p := range patchPfade(e.Patch) {
		if !referenzPfad.MatchString(p) {
			continue
		}
		name := strings.TrimSuffix(path.Base(p), ".png")
		neu, err := gitBlob(sp, env, ":"+p)
		if err != nil {
			continue // gelöscht oder umbenannt: keine Szene mehr
		}
		if !szenenName.MatchString(name) || len(neu) > bildGroesseMax || !istPNG(neu) || len(aus) == bilderMax {
			vollstaendig = false // etwa ein Zeiger auf Git-LFS statt des Bildes
			continue
		}
		alt, err := gitBlob(sp, nil, e.BasisCommit+":"+p)
		if err != nil || !istPNG(alt) || len(alt) > bildGroesseMax {
			alt = nil // neue Szene
		}
		if alt != nil && bytes.Equal(alt, neu) {
			continue
		}
		aus = append(aus, szenenBild{Name: name, Alt: alt, Neu: neu, Diff: differenzBild(alt, neu)})
	}
	return aus, vollstaendig
}

// patchPfade: die Pfade aus den „diff --git a/… b/…“-Zeilen (alt und neu), ohne doppelte.
func patchPfade(patch string) []string {
	var pfade []string
	gesehen := map[string]bool{}
	for _, zeile := range strings.Split(patch, "\n") {
		if !strings.HasPrefix(zeile, "diff --git a/") {
			continue
		}
		for _, p := range strings.SplitN(strings.TrimPrefix(zeile, "diff --git a/"), " b/", 2) {
			if p != "" && !gesehen[p] {
				gesehen[p] = true
				pfade = append(pfade, p)
			}
		}
	}
	return pfade
}

func gitBlob(sp string, env []string, spec string) ([]byte, error) {
	cmd := exec.Command("git", "cat-file", "blob", spec)
	cmd.Dir = sp
	cmd.Env = append(os.Environ(), env...)
	return cmd.Output()
}

func istPNG(b []byte) bool {
	return bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n"))
}

// differenzBild: geänderte Pixel kräftig rot, der Rest das neue Bild blass in Grau. nil, wenn sich nichts vergleichen
// lässt (neue Szene, andere Größe, kein lesbares PNG) oder das Ergebnis zu groß wäre.
func differenzBild(alt, neu []byte) []byte {
	if alt == nil {
		return nil
	}
	a, err1 := png.Decode(bytes.NewReader(alt))
	b, err2 := png.Decode(bytes.NewReader(neu))
	if err1 != nil || err2 != nil || a.Bounds() != b.Bounds() || a.Bounds().Dx()*a.Bounds().Dy() > 4_000_000 {
		return nil
	}
	r := b.Bounds()
	aus := image.NewNRGBA(r)
	betrag := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			ca := color.NRGBAModel.Convert(a.At(x, y)).(color.NRGBA)
			cb := color.NRGBAModel.Convert(b.At(x, y)).(color.NRGBA)
			if betrag(ca.R, cb.R)+betrag(ca.G, cb.G)+betrag(ca.B, cb.B)+betrag(ca.A, cb.A) > 24 {
				aus.SetNRGBA(x, y, color.NRGBA{R: 230, G: 0, B: 60, A: 255})
				continue
			}
			grau := uint8((uint32(cb.R)*30 + uint32(cb.G)*59 + uint32(cb.B)*11) / 100)
			aus.SetNRGBA(x, y, color.NRGBA{R: grau, G: grau, B: grau, A: cb.A / 4})
		}
	}
	var puffer bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&puffer, aus); err != nil || puffer.Len() > bildGroesseMax {
		return nil
	}
	return puffer.Bytes()
}
