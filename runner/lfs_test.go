package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func zeigerText(oid string, groesse string) string {
	return "version https://git-lfs.github.com/spec/v1\noid sha256:" + oid + "\nsize " + groesse + "\n"
}

// Nur eine oid aus 64 Hex-Zeichen ist ein Zeiger (Sicherheitsaudit 12.11.2026): "../" in der oid hätte eine Datei des
// Rechners in den Arbeitsbaum kopiert.
func TestZeigerNurMitHexOid(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []struct {
		name, inhalt string
		zeiger       bool
	}{
		{"gut", zeigerText(strings.Repeat("a", 64), "12"), true},
		{"traversal", zeigerText("../../.."+strings.Repeat("/", 32)+".config/swarvex/dummy-key", "35"), false},
		{"grossbuchstaben", zeigerText(strings.Repeat("A", 64), "12"), false},
		{"negativ", zeigerText(strings.Repeat("b", 64), "-1"), false},
	} {
		p := filepath.Join(dir, f.name)
		os.WriteFile(p, []byte(f.inhalt), 0o644)
		if _, ok := zeigerLesen(p); ok != f.zeiger {
			t.Errorf("%s: pointer %v, want %v", f.name, ok, f.zeiger)
		}
	}
}

func TestLfsAufloesenLiestKeineFremdeDatei(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	geheim := filepath.Join(home, ".config", "swarvex", "dummy-key")
	os.MkdirAll(filepath.Dir(geheim), 0o700)
	os.WriteFile(geheim, []byte("sk-DUMMY-NOT-A-KEY\n"), 0o600)
	os.MkdirAll(lfsCache(), 0o755)
	// eine oid, die aus dem Cache bis zur Datei oben reicht und genau 64 Zeichen lang ist
	rel, _ := filepath.Rel(lfsCache(), geheim)
	oid := rel
	for len(oid) < 64 {
		oid = strings.Replace(oid, "/", "//", 1)
	}
	if len(oid) != 64 {
		t.Fatalf("test setup: oid length %d", len(oid))
	}
	baum := t.TempDir()
	zeiger := zeigerText(oid, "19")
	os.WriteFile(filepath.Join(baum, "model.glb"), []byte(zeiger), 0o644)
	var anfragen atomic.Int32
	plattform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		anfragen.Add(1)
		http.NotFound(w, r)
	}))
	defer plattform.Close()
	pfade, err := lfsAufloesen(&zugangsdaten{Plattform: plattform.URL}, "dummy-token", 7, baum)
	if err != nil || len(pfade) != 0 {
		t.Fatalf("paths %v, err %v; want none", pfade, err)
	}
	if roh, _ := os.ReadFile(filepath.Join(baum, "model.glb")); string(roh) != zeiger {
		t.Fatalf("work tree file changed: %q", roh)
	}
	if n := anfragen.Load(); n != 0 {
		t.Fatalf("%d requests to the platform with an invalid oid", n)
	}
}
