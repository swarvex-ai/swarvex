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
	auftrag := auftragstext(aufgabe{Titel: "t", Pfade: []string{"docs/test-triage/x.md"}, BasisCommit: "77db3e1"}, u)
	if !strings.Contains(auftrag, "/work/docs/test-triage/x.md") || !strings.Contains(auftrag, "you start in /work/game") {
		t.Errorf("Auftrag nennt keine vollen Pfade:\n%s", auftrag)
	}
	if !strings.Contains(auftrag, "at commit 77db3e1") {
		t.Errorf("Auftrag nennt den Basis-Commit nicht:\n%s", auftrag)
	}
}

// Ein Filter aus der Git-Konfiguration des Nutzers läuft nicht, auch wenn eine .gitattributes aus der Sandbox ihn
// auswählt (Sicherheitsaudit 12.11.2026).
func TestPatchOhneGitEinstellungenDesNutzers(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git fehlt")
	}
	ordner := t.TempDir()
	home := filepath.Join(ordner, "home")
	os.MkdirAll(home, 0o755)
	marke := filepath.Join(ordner, "filter-ran")
	os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[filter \"boese\"]\n\tclean = touch "+marke+" && cat\n"), 0o644)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	baum, gitDir := filepath.Join(ordner, "tree"), filepath.Join(ordner, "git")
	os.MkdirAll(filepath.Join(baum, "docs"), 0o755)
	os.WriteFile(filepath.Join(baum, "docs", "a.md"), []byte("alt\n"), 0o644)
	if err := gitBasis(gitDir, baum, nil); err != nil {
		t.Fatal(err)
	}
	// die Sandbox schreibt eine .gitattributes und ändert eine erlaubte Datei
	os.WriteFile(filepath.Join(baum, ".gitattributes"), []byte("*.md filter=boese\n"), 0o644)
	os.WriteFile(filepath.Join(baum, "docs", "a.md"), []byte("neu\n"), 0o644)
	patch, _, err := patchBilden(gitDir, baum, []string{"docs/**"}, umgebung.Umgebung{})
	if err != nil {
		t.Fatalf("Patch: %v", err)
	}
	if _, err := os.Stat(marke); err == nil {
		t.Fatal("a filter from the user's git config ran on the sandbox tree")
	}
	if !strings.Contains(patch, "+neu") {
		t.Errorf("patch lost the change:\n%s", patch)
	}
}

// Muster wie bei der Plattform (matchesGlob): Platzhalter vor "/**" gelten, "**" steht für beliebig viele Ordner.
func TestPasst(t *testing.T) {
	m := "asset_library/comic/intro/panel_01*/**"
	for _, f := range []struct {
		muster, pfad string
		ok           bool
	}{
		{m, "asset_library/comic/intro/panel_01_v2/PROVENANCE.md", true},
		{m, "asset_library/comic/intro/panel_01_v2/src/render.py", true},
		{m, "asset_library/comic/intro/panel_02_v1/PROVENANCE.md", false},
		{m, "asset_library/comic/intro/PROVENANCE.md", false},
		{"tools/**", "tools/run_tests.sh", true},
		{"tools/**", "tools/a/b/c.sh", true},
		{"tools/**", "toolsx/a.sh", false},
		{"game/**/*.gd", "game/enemy.gd", true},
		{"game/**/*.gd", "game/a/b/enemy.gd", true},
		{"game/**/*.gd", "game/a/enemy.tscn", false},
		{"game/assets/comic/intro_panel_01_art.png", "game/assets/comic/intro_panel_01_art.png", true},
		{"docs/*.md", "docs/a/b.md", false},
		{"**", "irgendwo/datei.txt", true},
	} {
		if got := passt(f.muster, f.pfad); got != f.ok {
			t.Errorf("passt(%q, %q) = %v, erwartet %v", f.muster, f.pfad, got, f.ok)
		}
	}
}

// Fall Rend&Run 24.11.2026: Bild und Herkunftsordner mit Platzhalter landen beide im Patch.
func TestPatchMitPlatzhalterOrdner(t *testing.T) {
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
	schreiben("README.md", "x\n")
	if err := gitBasis(gitDir, baum, nil); err != nil {
		t.Fatal(err)
	}
	u, err := umgebung.Ermitteln(baum)
	if err != nil {
		t.Fatal(err)
	}
	schreiben("game/assets/comic/intro_panel_01_art.png", "png\n")
	schreiben("asset_library/comic/intro/panel_01_v2/PROVENANCE.md", "# Provenance\n")
	schreiben("asset_library/comic/intro/panel_01_v2/render_panel_01.py", "print(1)\n")
	erlaubt := []string{"game/assets/comic/intro_panel_01_art.png", "asset_library/comic/intro/panel_01*/**"}
	patch, verworfen, err := patchBilden(gitDir, baum, erlaubt, u)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"intro_panel_01_art.png", "panel_01_v2/PROVENANCE.md", "panel_01_v2/render_panel_01.py"} {
		if !strings.Contains(patch, p) {
			t.Errorf("%s fehlt im Patch", p)
		}
	}
	if len(verworfen) != 0 || len(patchVorpruefen(patch, erlaubt, u)) != 0 {
		t.Errorf("verworfen %v, Vorprüfung %v", verworfen, patchVorpruefen(patch, erlaubt, u))
	}
}
