package main

import (
	"bytes"
	"compress/zlib"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// base85Kodieren: wie git (base85.c) eine Datenzeile kodiert, für Patches im Test.
func base85Kodieren(roh []byte) string {
	var s strings.Builder
	for len(roh) > 0 {
		n := min(len(roh), 52)
		if n <= 26 {
			s.WriteByte(byte('A' + n - 1))
		} else {
			s.WriteByte(byte('a' + n - 27))
		}
		teil := append([]byte(nil), roh[:n]...)
		for len(teil)%4 != 0 {
			teil = append(teil, 0)
		}
		for i := 0; i < len(teil); i += 4 {
			v := uint32(teil[i])<<24 | uint32(teil[i+1])<<16 | uint32(teil[i+2])<<8 | uint32(teil[i+3])
			var gruppe [5]byte
			for j := 4; j >= 0; j-- {
				gruppe[j] = base85Zeichen[v%85]
				v /= 85
			}
			s.Write(gruppe[:])
		}
		s.WriteByte('\n')
		roh = roh[n:]
	}
	return s.String()
}

func literalTeil(inhalt []byte, angabe int) string {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	w.Write(inhalt)
	w.Close()
	return "literal " + strconv.Itoa(angabe) + "\n" + base85Kodieren(z.Bytes()) + "\n"
}

func zufallsBytes(n int, saat int64) []byte {
	b := make([]byte, n)
	rand.New(rand.NewSource(saat)).Read(b)
	return b
}

// Mit echtem git: Text bleibt, neue und geänderte Bilder werden je eine Zeile, der neue Inhalt liegt unter files/.
func TestBinaerPatchKuerzenMitGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git fehlt")
	}
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = repo
		aus, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, aus)
		}
		return string(aus)
	}
	schreiben := func(p string, inhalt []byte) {
		t.Helper()
		voll := filepath.Join(repo, p)
		os.MkdirAll(filepath.Dir(voll), 0o755)
		if err := os.WriteFile(voll, inhalt, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	schreiben("tests/baseline/alt.png", zufallsBytes(40000, 1))
	schreiben("src/a.ts", []byte("export const a = 1;\n"))
	git("add", ".")
	git("commit", "-qm", "basis")
	neuBild, geaendert := zufallsBytes(30000, 2), zufallsBytes(40000, 3)
	schreiben("assets/neu bild.png", neuBild)
	schreiben("tests/baseline/alt.png", geaendert)
	schreiben("src/a.ts", []byte("export const a = 2;\n"))
	git("add", ".")
	patch := git("diff", "--cached", "--binary")
	if !strings.Contains(patch, "GIT binary patch") {
		t.Fatal("kein Binärpatch erzeugt")
	}

	arbeit := t.TempDir()
	gekuerzt, dateien, weg := binaerPatchKuerzen(patch, arbeit)
	if strings.Count(gekuerzt, "GIT binary patch (left out by the swarvex runner") != 2 {
		t.Fatalf("zwei Notizzeilen erwartet:\n%s", gekuerzt)
	}
	if !strings.Contains(gekuerzt, "-export const a = 1;\n+export const a = 2;\n") || !strings.Contains(gekuerzt, "diff --git a/src/a.ts b/src/a.ts") {
		t.Fatalf("Textteil verändert:\n%s", gekuerzt)
	}
	if weg < len(patch)*9/10 || len(gekuerzt) > 3000 {
		t.Fatalf("zu wenig gekürzt: %d von %d Bytes übrig", len(gekuerzt), len(patch))
	}
	if len(dateien) != 2 {
		t.Fatalf("zwei ausgeschriebene Dateien erwartet: %v\n%s", dateien, gekuerzt)
	}
	for p, soll := range map[string][]byte{"assets/neu bild.png": neuBild, "tests/baseline/alt.png": geaendert} {
		ist, err := os.ReadFile(filepath.Join(arbeit, "files", filepath.FromSlash(p)))
		if err != nil || !bytes.Equal(ist, soll) {
			t.Fatalf("%s falsch entpackt (%v)", p, err)
		}
		if !strings.Contains(gekuerzt, "decoded to /work/files/"+p+")") {
			t.Fatalf("Notiz nennt %s nicht:\n%s", p, gekuerzt)
		}
	}
}

func TestBinaerPatchKuerzenOhneBinaer(t *testing.T) {
	patch := "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+GIT binary patch\n"
	if g, d, w := binaerPatchKuerzen(patch, t.TempDir()); g != patch || d != nil || w != 0 {
		t.Fatalf("Textpatch verändert: %q %v %d", g, d, w)
	}
}

// Fremder Patch: Pfade bleiben unter files/, falsche Größen und Daten werden nur vermerkt, Deltas ebenso.
func TestBinaerPatchKuerzenFremdeEingaben(t *testing.T) {
	inhalt := zufallsBytes(500, 4)
	patch := "diff --git a/../../ausbruch.png b/../../ausbruch.png\nnew file mode 100644\nGIT binary patch\n" + literalTeil(inhalt, 500) + "literal 0\nHcmV?d00001\n\n" +
		"diff --git a/bombe.bin b/bombe.bin\nGIT binary patch\n" + literalTeil(make([]byte, 1<<20), 10) +
		"diff --git a/kaputt.bin b/kaputt.bin\nGIT binary patch\nliteral 8\nzzzzzzz\n\n" +
		"diff --git a/d.png b/d.png\nindex 1..2 100644\nGIT binary patch\ndelta 1234\nzcmV?d00001\n\ndelta 99\nzcmV?d00001\n\n" +
		"diff --git a/weg.png b/weg.png\ndeleted file mode 100644\nGIT binary patch\nliteral 0\nHcmV?d00001\n\nliteral 500\n" + strings.Repeat("z", 10) + "\n\n" +
		"diff --git a/t.txt b/t.txt\n--- a/t.txt\n+++ b/t.txt\n@@ -1 +1 @@\n-x\n+y\n"
	ordner := t.TempDir()
	arbeit := filepath.Join(ordner, "work")
	os.MkdirAll(arbeit, 0o755)
	gekuerzt, dateien, _ := binaerPatchKuerzen(patch, arbeit)
	if len(dateien) != 1 || filepath.ToSlash(dateien[0]) != "files/ausbruch.png" {
		t.Fatalf("nur files/ausbruch.png erwartet: %v\n%s", dateien, gekuerzt)
	}
	if ist, _ := os.ReadFile(filepath.Join(arbeit, "files", "ausbruch.png")); !bytes.Equal(ist, inhalt) {
		t.Fatal("Inhalt falsch")
	}
	if eintraege, _ := os.ReadDir(ordner); len(eintraege) != 1 {
		t.Fatalf("außerhalb von work geschrieben: %v", eintraege)
	}
	for _, soll := range []string{
		"new content 10 bytes, could not be decoded: size", // mehr entpackt als angekündigt
		"new content 8 bytes, could not be decoded",
		"binary delta against the old file, 1234 bytes after the change",
		"the file is empty after the change (or deleted)",
		"diff --git a/t.txt b/t.txt\n--- a/t.txt\n+++ b/t.txt\n@@ -1 +1 @@\n-x\n+y\n",
	} {
		if !strings.Contains(gekuerzt, soll) {
			t.Fatalf("fehlt %q in:\n%s", soll, gekuerzt)
		}
	}
	if strings.Contains(gekuerzt, "zcmV") || strings.Contains(gekuerzt, "HcmV") || strings.Contains(gekuerzt, "zzzz") {
		t.Fatalf("Binärdaten übrig:\n%s", gekuerzt)
	}
	if _, err := os.Stat(filepath.Join(arbeit, "files", "bombe.bin")); err == nil {
		t.Fatal("zu große Entpackung geschrieben")
	}
}

func TestDiffPfad(t *testing.T) {
	for zeile, soll := range map[string]string{
		"diff --git a/x.png b/x.png":                   "x.png",
		"diff --git a/a b/c b/a b/c":                   "a b/c",
		`diff --git "a/\303\244.png" "b/\303\244.png"`: "ä.png",
		"diff --git a/alt.png b/neu.png":               "",
	} {
		if ist := diffPfad(zeile); ist != soll {
			t.Errorf("%s: %q statt %q", zeile, ist, soll)
		}
	}
}
