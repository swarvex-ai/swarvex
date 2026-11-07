package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Nur geänderte oder neue Szenen gehen an die Plattform; die Referenz kommt aus dem Basis-Commit, nicht aus dem Beitrag.
func TestSzenenBilder(t *testing.T) {
	baum := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", baum, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		if aus, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, aus)
		}
	}
	png := func(x byte) []byte { return []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', x} }
	schreibe := func(name string, inhalt []byte) {
		os.MkdirAll(filepath.Dir(filepath.Join(baum, name)), 0o755)
		os.WriteFile(filepath.Join(baum, name), inhalt, 0o644)
	}
	git("init", "-q")
	schreibe("tests/scene/baseline/gleich.png", png(1))
	schreibe("tests/scene/baseline/anders.png", png(2))
	git("add", ".")
	git("commit", "-q", "-m", "basis")
	basis, _ := exec.Command("git", "-C", baum, "rev-parse", "HEAD").Output()
	// Der Beitrag ändert die Referenz von "anders" mit: maßgeblich bleibt der Basis-Commit
	schreibe("tests/scene/baseline/anders.png", png(3))
	schreibe("tests/scene/out/gleich.png", png(1))
	schreibe("tests/scene/out/anders.png", png(3))
	schreibe("tests/scene/out/anders.diff.png", png(9))
	schreibe("tests/scene/out/neu.png", png(4))
	schreibe("tests/scene/out/Unsinn!.png", png(5))
	bilder, err := szenenBilder(baum, "", "tests/scene/out", "tests/scene/baseline", string(basis[:40]))
	if err != nil {
		t.Fatal(err)
	}
	if len(bilder) != 2 || bilder[0].Name != "anders" || bilder[1].Name != "neu" {
		t.Fatalf("changed and new scenes only: %+v", bilder)
	}
	if string(bilder[0].Alt) != string(png(2)) || string(bilder[0].Diff) != string(png(9)) || bilder[1].Alt != nil {
		t.Errorf("old from the base commit, diff from the run: %+v", bilder)
	}
	if bilder, _ := szenenBilder(baum, "", "", "", string(basis[:40])); bilder != nil {
		t.Error("without images in swarvex.yaml: none")
	}
}
