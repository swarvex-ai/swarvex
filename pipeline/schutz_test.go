package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Die Prüfung nach dem Anwenden sieht, was sich wirklich geändert hat, auch wenn der Text des Patches
// den Pfad versteckt (Anführungszeichen, Umbenennung, Groß-/Kleinschreibung).
func TestGeschuetztGeaendert(t *testing.T) {
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "init.defaultBranch=main"}, args...)...)
		cmd.Dir = dir
		if aus, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, aus)
		}
	}
	faelle := []struct{ name, patch, erwartet string }{
		{"Anführungszeichen", "diff --git \"a/AGENTS.md\" \"b/AGENTS.md\"\nnew file mode 100644\n--- /dev/null\n+++ \"b/AGENTS.md\"\n@@ -0,0 +1 @@\n+x\n", "AGENTS.md"},
		{"Umbenennung hinein", "diff --git a/src/x.md b/src/x.md\nsimilarity index 100%\nrename from src/x.md\nrename to .github/workflows/x.yml\n", ".github/workflows/x.yml"},
		{"Umbenennung heraus", "diff --git a/CLAUDE.md b/CLAUDE.md\nsimilarity index 100%\nrename from CLAUDE.md\nrename to src/y.md\n", "CLAUDE.md"},
		{"klein geschrieben", "diff --git a/agents.md b/agents.md\nnew file mode 100644\n--- /dev/null\n+++ b/agents.md\n@@ -0,0 +1 @@\n+x\n", "agents.md"},
		{"harmlos", "diff --git a/src/x.md b/src/x.md\n--- a/src/x.md\n+++ b/src/x.md\n@@ -1 +1 @@\n-alt\n+neu\n", ""},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			dir := t.TempDir()
			os.MkdirAll(filepath.Join(dir, "src"), 0o755)
			os.WriteFile(filepath.Join(dir, "src", "x.md"), []byte("alt\n"), 0o644)
			os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("rules\n"), 0o644)
			git(dir, "init", "-q")
			git(dir, "add", "-A")
			git(dir, "commit", "-qm", "base")
			patch := filepath.Join(t.TempDir(), "p.patch")
			os.WriteFile(patch, []byte(f.patch), 0o644)
			git(dir, "apply", patch)
			p, err := geschuetztGeaendert(dir)
			if err != nil || p != f.erwartet {
				t.Fatalf("got %q (%v), want %q", p, err, f.erwartet)
			}
		})
	}
}
