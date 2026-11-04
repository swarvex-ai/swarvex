package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

// Ablauf wie bei Rend&Run (dir: game, geschützte Pfade mit „../“): die neue Datei im erlaubten
// Pfad landet im Patch, Änderungen an geschützten und fremden Pfaden nicht.
func TestPatchMitGeschuetztenPfadenAusDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git fehlt")
	}
	ordner := t.TempDir()
	baum, gitDir := filepath.Join(ordner, "tree"), filepath.Join(ordner, "git")
	schreiben := func(p, inhalt string) {
		t.Helper()
		voll := filepath.Join(baum, p)
		if err := os.MkdirAll(filepath.Dir(voll), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(voll, []byte(inhalt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	schreiben("swarvex.yaml", "template: godot\ndir: game\nprotected: |\n  - project.godot\n  - ../tools/run_tests.sh\n")
	schreiben("game/project.godot", "config_version=5\n")
	schreiben("tools/run_tests.sh", "#!/bin/sh\n")
	if err := gitBasis(gitDir, baum, nil); err != nil {
		t.Fatal(err)
	}
	u, err := umgebung.Ermitteln(baum)
	if err != nil {
		t.Fatal(err)
	}
	schreiben("docs/test-triage/x.md", "# Diagnose\n")
	schreiben("tools/run_tests.sh", "#!/bin/sh\nexit 0\n")
	schreiben("game/enemy.gd", "extends Node\n")

	patch, verworfen, err := patchBilden(gitDir, baum, []string{"docs/test-triage/x.md", "tools/**"}, u)
	if err != nil {
		t.Fatalf("Patch: %v", err)
	}
	if !strings.Contains(patch, "docs/test-triage/x.md") || strings.Contains(patch, "run_tests.sh") || strings.Contains(patch, "enemy.gd") {
		t.Errorf("Patch falsch:\n%s", patch)
	}
	if len(verworfen) != 2 {
		t.Errorf("verworfen = %v, erwartet 2", verworfen)
	}
	auftrag := auftragstext(aufgabe{Titel: "t", Pfade: []string{"docs/test-triage/x.md"}}, u)
	if !strings.Contains(auftrag, "/work/docs/test-triage/x.md") || !strings.Contains(auftrag, "you start in /work/game") {
		t.Errorf("Auftrag nennt keine vollen Pfade:\n%s", auftrag)
	}
}
