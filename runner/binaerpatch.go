package main

// Binärteile im Patch für Gutachter (16.11.2026): Ein Bild im Patch steht dort als „GIT binary patch“, Base85 über
// zlib, oft 100 bis 600 KB Text. Das Modell liest patch.diff ganz und schickt es in der Agentenschleife mit jedem
// Schritt erneut: 120 von 153 Gemini-Prüfungen hatten solche Teile, das Guthaben war in zwei Tagen weg. Lesen kann ein
// Modell die Zeichen nicht; die Bilder bekommt es als Szenenbilder (alt, neu, Differenz). Darum: In patch.diff steht
// je Binärdatei eine Zeile, und der neue Inhalt liegt entpackt unter /work/files/<Pfad>, falls jemand ihn ansehen will
// (ein Gutachter dekodierte am 16.11. ein Comic-Panel, um die Palette zu prüfen).

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	binaerDateiMax  = 16 << 20 // größer wird nicht ausgeschrieben, nur vermerkt
	binaerGesamtMax = 64 << 20
)

// Das Base85-Alphabet von git (base85.c), nicht das von encoding/ascii85.
const base85Zeichen = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz!#$%&()*+-;<=>?@^_`{|}~"

var base85Wert = func() (w [256]int16) {
	for i := range w {
		w[i] = -1
	}
	for i := 0; i < len(base85Zeichen); i++ {
		w[base85Zeichen[i]] = int16(i)
	}
	return
}()

// base85Zeile: eine Datenzeile eines Binärpatches. Das erste Zeichen nennt die Länge (A–Z: 1–26, a–z: 27–52 Bytes).
func base85Zeile(z string) ([]byte, error) {
	if z == "" {
		return nil, fmt.Errorf("empty line")
	}
	var n int
	switch c := z[0]; {
	case c >= 'A' && c <= 'Z':
		n = int(c-'A') + 1
	case c >= 'a' && c <= 'z':
		n = int(c-'a') + 27
	default:
		return nil, fmt.Errorf("bad length character %q", c)
	}
	daten := z[1:]
	if len(daten) != (n+3)/4*5 {
		return nil, fmt.Errorf("line length %d does not fit %d bytes", len(daten), n)
	}
	aus := make([]byte, 0, (n+3)/4*4)
	for i := 0; i < len(daten); i += 5 {
		var summe uint32
		for j := 0; j < 5; j++ {
			v := base85Wert[daten[i+j]]
			if v < 0 {
				return nil, fmt.Errorf("bad character %q", daten[i+j])
			}
			if summe > (0xffffffff-uint32(v))/85 {
				return nil, fmt.Errorf("value out of range")
			}
			summe = summe*85 + uint32(v)
		}
		aus = append(aus, byte(summe>>24), byte(summe>>16), byte(summe>>8), byte(summe))
	}
	return aus[:n], nil
}

// binaerInhalt: der Inhalt eines „literal“-Teils, entpackt und auf die angegebene Größe geprüft.
func binaerInhalt(zeilen []string, groesse int) ([]byte, error) {
	var gepackt bytes.Buffer
	for _, z := range zeilen {
		teil, err := base85Zeile(z)
		if err != nil {
			return nil, err
		}
		gepackt.Write(teil)
	}
	r, err := zlib.NewReader(&gepackt)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	inhalt, err := io.ReadAll(io.LimitReader(r, int64(groesse)+1)) // nicht mehr entpacken als angekündigt
	if err != nil {
		return nil, err
	}
	if len(inhalt) != groesse {
		return nil, fmt.Errorf("size %d instead of %d", len(inhalt), groesse)
	}
	return inhalt, nil
}

// diffPfad: der neue Pfad aus „diff --git a/P b/P“ (auch in Anführungszeichen); leer, wenn er nicht sicher zu lesen ist.
func diffPfad(zeile string) string {
	rest := strings.TrimPrefix(zeile, "diff --git ")
	if strings.HasPrefix(rest, `"`) || strings.HasSuffix(rest, `"`) {
		if i := strings.LastIndex(rest, ` "b/`); i >= 0 {
			if p, err := strconv.Unquote(rest[i+1:]); err == nil {
				return strings.TrimPrefix(p, "b/")
			}
		}
		if i := strings.LastIndex(rest, " b/"); i >= 0 && !strings.HasSuffix(rest, `"`) {
			return rest[i+3:]
		}
		return ""
	}
	// a/P b/P: beide Hälften gleich lang (bei Umbenennungen setzt „rename to“ den Pfad)
	if len(rest)%2 == 1 {
		h := (len(rest) - 1) / 2
		if a, b := rest[:h], rest[h+1:]; strings.HasPrefix(a, "a/") && strings.HasPrefix(b, "b/") && a[2:] == b[2:] {
			return b[2:]
		}
	}
	return ""
}

// sichererPfad: ein Pfad aus dem (fremden) Patch, der unter files/ bleibt; sonst leer.
func sichererPfad(p string) string {
	if p == "" || strings.ContainsRune(p, 0) || strings.Contains(p, `\`) {
		return ""
	}
	p = path.Clean("/" + p)[1:]
	if p == "" || p == "." {
		return ""
	}
	return p
}

// binaerPatchKuerzen: der Patch ohne Binärdaten, mit einer Zeile je Binärdatei. Den neuen Inhalt einer Datei („literal“)
// schreibt es nach <arbeit>/files/<Pfad>, wenn arbeit nicht leer ist. Liefert die geschriebenen Dateien (relativ zu
// arbeit) und wie viele Bytes aus dem Patch wegfielen.
func binaerPatchKuerzen(patch, arbeit string) (string, []string, int) {
	if !strings.Contains(patch, "\nGIT binary patch\n") && !strings.HasPrefix(patch, "GIT binary patch\n") {
		return patch, nil, 0
	}
	zeilen := strings.SplitAfter(patch, "\n")
	var aus strings.Builder
	var dateien []string
	pfad, gesamt, unbenannt := "", 0, 0
	for i := 0; i < len(zeilen); i++ {
		z := zeilen[i]
		ohne := strings.TrimRight(z, "\r\n")
		switch {
		case strings.HasPrefix(ohne, "diff --git "):
			pfad = diffPfad(ohne)
		case strings.HasPrefix(ohne, "rename to ") || strings.HasPrefix(ohne, "copy to "):
			pfad = ohne[strings.Index(ohne, " to ")+4:]
		}
		if ohne != "GIT binary patch" {
			aus.WriteString(z)
			continue
		}
		// Kopf des ersten Teils (der neue Stand), dann Datenzeilen bis zur Leerzeile; der Rest bis zum nächsten
		// „diff --git“ ist der Rückweg und fällt ganz weg.
		art, groesse := "", -1
		var daten []string
		j := i + 1
		if j < len(zeilen) {
			if f := strings.Fields(zeilen[j]); len(f) == 2 && (f[0] == "literal" || f[0] == "delta") {
				if n, err := strconv.Atoi(f[1]); err == nil && n >= 0 {
					art, groesse = f[0], n
					j++
					for ; j < len(zeilen) && strings.TrimRight(zeilen[j], "\r\n") != ""; j++ {
						daten = append(daten, strings.TrimRight(zeilen[j], "\r\n"))
					}
				}
			}
		}
		for j < len(zeilen) && !strings.HasPrefix(zeilen[j], "diff --git ") {
			j++
		}
		weg := 0
		for _, x := range zeilen[i:j] {
			weg += len(x)
		}
		notiz := ""
		switch {
		case art == "literal" && groesse == 0:
			notiz = "the file is empty after the change (or deleted)"
		case art == "literal":
			ziel := sichererPfad(pfad)
			if ziel == "" {
				unbenannt++
				ziel = fmt.Sprintf("unnamed-%d", unbenannt)
			}
			rel := filepath.Join("files", filepath.FromSlash(ziel))
			switch {
			case groesse > binaerDateiMax || gesamt+groesse > binaerGesamtMax:
				notiz = fmt.Sprintf("new content %d bytes, too large to write out", groesse)
			case arbeit == "":
				notiz = fmt.Sprintf("new content %d bytes", groesse)
			default:
				inhalt, err := binaerInhalt(daten, groesse)
				if err == nil {
					voll := filepath.Join(arbeit, rel)
					if err = os.MkdirAll(filepath.Dir(voll), 0o755); err == nil {
						err = os.WriteFile(voll, inhalt, 0o644)
					}
				}
				if err != nil {
					notiz = fmt.Sprintf("new content %d bytes, could not be decoded: %v", groesse, err)
				} else {
					gesamt += groesse
					dateien = append(dateien, rel)
					notiz = fmt.Sprintf("new content %d bytes, decoded to /work/%s", groesse, filepath.ToSlash(rel))
				}
			}
		case art == "delta":
			notiz = fmt.Sprintf("binary delta against the old file, %d bytes after the change; the new content is not available here", groesse)
		default:
			notiz = "binary data in an unknown form"
		}
		fmt.Fprintf(&aus, "GIT binary patch (left out by the swarvex runner, %d bytes of encoded data: %s)\n", weg, notiz)
		i = j - 1
	}
	gekuerzt := aus.String()
	return gekuerzt, dateien, len(patch) - len(gekuerzt)
}
