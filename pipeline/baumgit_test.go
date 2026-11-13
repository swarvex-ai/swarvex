package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Ein Container ersetzt die .git-Datei des Worktrees durch ein eigenes Repository mit fsmonitor. Git auf dem Host liest
// dann trotzdem das Git-Verzeichnis im Spiegel und führt nichts aus dem Baum aus (Sicherheitsaudit 12.11.2026; ohne
// festes Git-Verzeichnis lief der fsmonitor bei einer Prüfstraße ohne root).
func TestBaumGitIgnoriertUntergeschobenesRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git fehlt")
	}
	ordner := t.TempDir()
	t.Setenv("HOME", ordner)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	g := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if aus, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, aus)
		}
	}
	sp := filepath.Join(ordner, "spiegel")
	os.MkdirAll(sp, 0o755)
	g(sp, "init", "-q")
	os.WriteFile(filepath.Join(sp, "a.txt"), []byte("alt\n"), 0o644)
	g(sp, "add", "a.txt")
	g(sp, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "basis")
	baum := filepath.Join(ordner, "check", "tree")
	g(sp, "worktree", "add", "-q", "--detach", "--force", baum, "HEAD")
	gitDir, err := baumGitDir(baum)
	if err != nil || !strings.HasPrefix(gitDir, sp+string(filepath.Separator)) {
		t.Fatalf("git dir %q (%v), want one inside the mirror", gitDir, err)
	}

	marke := filepath.Join(ordner, "fsmonitor-ran")
	boese := filepath.Join(ordner, "boese")
	g(ordner, "init", "-q", boese)
	g(boese, "config", "core.fsmonitor", "touch "+marke+"; false")
	os.Remove(filepath.Join(baum, ".git"))
	if err := os.Rename(filepath.Join(boese, ".git"), filepath.Join(baum, ".git")); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(baum, "a.txt"), []byte("neu\n"), 0o644)

	pfade, err := geaendertePfade(gitDir, baum)
	if err != nil || len(pfade) != 1 || pfade[0] != "a.txt" {
		t.Fatalf("changed paths %v (%v), want [a.txt]", pfade, err)
	}
	if _, err := os.Stat(marke); err == nil {
		t.Fatal("host git ran the fsmonitor of the repository planted in the tree")
	}
	if alt, err := baumGitBefehl(gitDir, baum, "show", "HEAD:a.txt").Output(); err != nil || string(alt) != "alt\n" {
		t.Fatalf("show from the mirror: %q (%v)", alt, err)
	}
}
