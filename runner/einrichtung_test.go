package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGespeicherteAngaben(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if gespeicherteArgs() != nil {
		t.Fatal("expected no saved settings")
	}
	if err := argsSpeichern([]string{"--project", "auto", "--model", "m", "--model-url", "http://x"}); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(einstellungenDatei()); st.Mode().Perm() != 0o600 {
		t.Fatalf("work.json must be 0600, is %v", st.Mode().Perm())
	}
	// ohne --project: gespeicherte Angaben vorn, eigene dahinter (spätere Flags gewinnen)
	got := mitGespeicherten([]string{"--continuous"})
	want := []string{"--project", "auto", "--model", "m", "--model-url", "http://x", "--continuous"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	// mit --project: unverändert
	eigen := []string{"--project=mein-spiel", "--model", "x", "--model-url", "http://y"}
	if got := mitGespeicherten(eigen); !reflect.DeepEqual(got, eigen) {
		t.Fatalf("got %v", got)
	}
}

func TestProjekteAuto(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"projects": []map[string]any{
			{"project": "leer", "name": "Leer", "tasks_open": 0},
			{"project": "klein", "name": "Klein", "tasks_open": 2},
			{"project": "gross", "name": "Groß", "tasks_open": 9},
		}})
	}))
	defer srv.Close()
	z := &zugangsdaten{Plattform: srv.URL}
	if got := projekteFuer("auto", z); !reflect.DeepEqual(got, []string{"gross", "klein"}) {
		t.Fatalf("auto: got %v", got)
	}
	if got := projekteFuer("mein-spiel", z); !reflect.DeepEqual(got, []string{"mein-spiel"}) {
		t.Fatalf("fixed: got %v", got)
	}
}

func TestOpencodeEintragen(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	pfad := filepath.Join(h, ".config", "opencode", "opencode.json")
	os.MkdirAll(filepath.Dir(pfad), 0o755)
	os.WriteFile(pfad, []byte(`{"theme":"dark","mcp":{"anderer":{"type":"local"}}}`), 0o644)
	if err := opencodeEintragen("https://swarvex.ai/mcp"); err != nil {
		t.Fatal(err)
	}
	var c map[string]any
	roh, _ := os.ReadFile(pfad)
	json.Unmarshal(roh, &c)
	mcp := c["mcp"].(map[string]any)
	if c["theme"] != "dark" || mcp["anderer"] == nil || mcp["swarvex"].(map[string]any)["url"] != "https://swarvex.ai/mcp" {
		t.Fatalf("unexpected config: %s", roh)
	}
	// JSON mit Kommentaren wird nicht angefasst
	os.WriteFile(pfad, []byte("// mein Kommentar\n{}"), 0o644)
	if err := opencodeEintragen("https://swarvex.ai/mcp"); err == nil {
		t.Fatal("expected an error for JSONC")
	}
	if roh, _ := os.ReadFile(pfad); string(roh) != "// mein Kommentar\n{}" {
		t.Fatal("JSONC file was changed")
	}
}

func TestMCPBefehle(t *testing.T) {
	for _, a := range mcpAgenten {
		if a.Hinzufuegen == nil {
			continue
		}
		befehl := a.Hinzufuegen("https://swarvex.ai/mcp")
		gefunden := false
		for _, x := range befehl {
			gefunden = gefunden || x == "https://swarvex.ai/mcp"
		}
		if !gefunden || befehl[0] != "mcp" {
			t.Fatalf("%s: %v", a.Titel, befehl)
		}
	}
}

// Ein Modell-Server auf diesem Rechner ist aus der Sandbox host.containers.internal, egal ob als localhost,
// 127.0.0.1 oder mit der eigenen Adresse im Heimnetz eingetragen; andere Geräte bleiben, wie sie sind.
func TestSandboxAdresse(t *testing.T) {
	alt := eigeneAdressen
	defer func() { eigeneAdressen = alt }()
	eigeneAdressen = func() []net.IP { return []net.IP{net.ParseIP("192.168.1.112"), net.ParseIP("fd00::12")} }
	for ein, aus := range map[string]string{
		"http://localhost:11434":                "http://host.containers.internal:11434",
		"http://127.0.0.1:8080/api":             "http://host.containers.internal:8080/api",
		"http://[::1]:1234":                     "http://host.containers.internal:1234",
		"http://192.168.1.112:11434":            "http://host.containers.internal:11434",
		"http://[fd00::12]:8000":                "http://host.containers.internal:8000",
		"http://192.168.1.50:11434":             "http://192.168.1.50:11434",
		"https://192.168.1.112:8443":            "https://192.168.1.112:8443",
		"https://openrouter.ai/api":             "https://openrouter.ai/api",
		"http://host.containers.internal:11434": "http://host.containers.internal:11434",
		"":                                      "",
	} {
		if neu, _ := sandboxAdresse(ein); neu != aus {
			t.Errorf("%q: got %q, want %q", ein, neu, aus)
		}
	}
}
