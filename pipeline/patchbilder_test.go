package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testPNG(t *testing.T, faerben func(x, y int) color.NRGBA) []byte {
	t.Helper()
	b := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			b.SetNRGBA(x, y, faerben(x, y))
		}
	}
	var puffer bytes.Buffer
	png.Encode(&puffer, b)
	return puffer.Bytes()
}

// Szenenbilder aus dem Patch: alt aus dem Basis-Commit, neu aus dem Patch, Differenz hier gerechnet; ein Zeiger auf
// Git-LFS statt des Bildes heißt „unvollständig“, ein Bild außerhalb eines Referenzordners zählt nicht.
func TestPatchBilder(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	arbeit = t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t", "GIT_CONFIG_NOSYSTEM=1")
		aus, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, aus)
		}
		return strings.TrimSpace(string(aus))
	}
	rot := func(x, y int) color.NRGBA { return color.NRGBA{R: 200, A: 255} }
	ursprung := filepath.Join(t.TempDir(), "projekt.git")
	git("", "init", "-q", "--bare", "-b", "main", ursprung)
	baum := t.TempDir()
	git(baum, "init", "-q", "-b", "main")
	schreiben := func(p string, inhalt []byte) {
		os.MkdirAll(filepath.Dir(filepath.Join(baum, p)), 0o755)
		os.WriteFile(filepath.Join(baum, p), inhalt, 0o644)
	}
	alt := testPNG(t, rot)
	schreiben("tests/scene/baseline/wagen.png", alt)
	schreiben("src/wagen.ts", []byte("export const wagen = 1;\n"))
	git(baum, "add", "-A")
	git(baum, "commit", "-q", "-m", "Basis")
	git(baum, "push", "-q", ursprung, "main")
	basis := git(baum, "rev-parse", "HEAD")

	patch := func(dateien map[string][]byte) einreichung {
		for p, inhalt := range dateien {
			schreiben(p, inhalt)
		}
		git(baum, "add", "-A")
		// ungekürzt: Ein Binärpatch endet mit einer Leerzeile, ohne sie wendet git die letzte Datei nicht an
		cmd := exec.Command("git", "diff", "--cached", "--binary", "HEAD")
		cmd.Dir = baum
		roh, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		d := string(roh)
		git(baum, "reset", "-q", "--hard", "HEAD")
		git(baum, "clean", "-qfd")
		summe := sha256.Sum256([]byte(d))
		return einreichung{EinreichungID: 1, Patch: d, PatchSHA: hex.EncodeToString(summe[:]), BasisCommit: basis, Slug: "bilder", RepoGit: ursprung}
	}
	// Ein Pixel wird blau, eine Szene kommt neu dazu, ein Icon außerhalb des Referenzordners ändert sich auch
	neu := testPNG(t, func(x, y int) color.NRGBA {
		if x == 3 && y == 4 {
			return color.NRGBA{B: 220, A: 255}
		}
		return rot(x, y)
	})
	karre := testPNG(t, func(x, y int) color.NRGBA { return color.NRGBA{G: 180, A: 255} })
	e := patch(map[string][]byte{"tests/scene/baseline/wagen.png": neu, "tests/scene/baseline/karre.png": karre,
		"src/icons/wagen.png": karre, "src/wagen.ts": []byte("export const wagen = 2;\n")})
	bilder, vollstaendig := patchBilder(e)
	if !vollstaendig || len(bilder) != 2 {
		for _, b := range bilder {
			t.Logf("scene %s old %d new %d diff %d", b.Name, len(b.Alt), len(b.Neu), len(b.Diff))
		}
		t.Fatalf("complete %v, %d scenes", vollstaendig, len(bilder))
	}
	namen := map[string]szenenBild{}
	for _, b := range bilder {
		namen[b.Name] = b
	}
	w, k := namen["wagen"], namen["karre"]
	if !bytes.Equal(w.Alt, alt) || !bytes.Equal(w.Neu, neu) || w.Diff == nil {
		t.Errorf("wagen: old %v new %v diff %v", bytes.Equal(w.Alt, alt), bytes.Equal(w.Neu, neu), w.Diff != nil)
	}
	if k.Alt != nil || !bytes.Equal(k.Neu, karre) || k.Diff != nil {
		t.Errorf("karre (new scene): %+v", k.Name)
	}
	// Die Differenz markiert genau das geänderte Pixel
	d, err := png.Decode(bytes.NewReader(w.Diff))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct {
		x, y int
		rot  bool
	}{{3, 4, true}, {0, 0, false}, {7, 7, false}} {
		r, _, _, a := d.At(f.x, f.y).RGBA()
		if (r>>8 > 200 && a>>8 == 255) != f.rot {
			t.Errorf("pixel %d,%d: red %v, want %v", f.x, f.y, !f.rot, f.rot)
		}
	}
	// Referenzbild als Zeiger auf Git-LFS: nicht lieferbar, die Prüfstraße rendert
	lfs := patch(map[string][]byte{"tests/scene/baseline/wagen.png": []byte("version https://git-lfs.github.com/spec/v1\noid sha256:" + strings.Repeat("a", 64) + "\nsize 1234\n")})
	if b, vollstaendig := patchBilder(lfs); vollstaendig || len(b) != 0 {
		t.Errorf("LFS pointer: complete %v, %d scenes", vollstaendig, len(b))
	}
	// Basis-Commit, der kein Hash ist, geht nie an git (Audit 12.11.2026: "--index-output=…" schrieb sonst eine Datei als root)
	ziel := filepath.Join(t.TempDir(), "probe")
	os.WriteFile(ziel, []byte("unverändert"), 0o644)
	option := e
	option.BasisCommit = "--index-output=" + ziel
	if _, vollstaendig := patchBilder(option); vollstaendig {
		t.Error("option-shaped base commit counted as complete")
	}
	if roh, _ := os.ReadFile(ziel); string(roh) != "unverändert" {
		t.Errorf("option-shaped base commit reached git: %q", roh)
	}
	// Prüfsumme passt nicht: nichts
	e.PatchSHA = strings.Repeat("0", 64)
	if _, vollstaendig := patchBilder(e); vollstaendig {
		t.Error("wrong checksum counted as complete")
	}
}
