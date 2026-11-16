package main

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hermesOrdner(t *testing.T, config, env string) {
	t.Helper()
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "config.yaml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if env != "" {
		if err := os.WriteFile(filepath.Join(d, ".env"), []byte(env), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HERMES_HOME", d)
	for _, k := range []string{"OPENROUTER_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		t.Setenv(k, "")
	}
}

func TestHermesOpenRouter(t *testing.T) {
	hermesOrdner(t, `_config_version: 52
database:
  journal_mode: "wal"
model:
  # Default model
  default: "anthropic/claude-opus-4.6"  # Kommentar
  provider: "auto"
  base_url: "https://openrouter.ai/api/v1"
terminal:
  backend: local
`, "# Keys\nOPENROUTER_API_KEY=sk-or-123\n")
	w := hermesModelle()
	if len(w) != 1 {
		t.Fatalf("eine Wahl erwartet, %v", w)
	}
	if w[0].Agent != "opencode" || w[0].URL != "https://openrouter.ai/api" || w[0].Modell != "anthropic/claude-opus-4.6" || w[0].Key != "wert:sk-or-123" {
		t.Fatalf("falsch übernommen: %+v", w[0])
	}
	if !strings.Contains(w[0].Titel, "OpenRouter") || strings.Contains(w[0].Titel, "sk-or") {
		t.Fatalf("Titel: %q", w[0].Titel)
	}
}

func TestHermesLokalUndOhneSchluessel(t *testing.T) {
	hermesOrdner(t, "model:\n  model: qwen3:8b\n  provider: ollama\n  base_url: http://localhost:11434/v1\n", "")
	w := hermesModelle()
	if len(w) != 1 || w[0].URL != "http://host.containers.internal:11434" || !w[0].Lokal || w[0].Key != "" {
		t.Fatalf("lokal: %+v", w)
	}
	// OpenRouter ohne Schlüssel und OAuth-Anbieter: nichts anbieten
	hermesOrdner(t, "model:\n  default: x/y\n  provider: openrouter\n", "")
	if w := hermesModelle(); len(w) != 0 {
		t.Fatalf("ohne Schlüssel: %+v", w)
	}
	hermesOrdner(t, "model:\n  default: hermes-4\n  provider: nous\n", "")
	if w := hermesModelle(); len(w) != 0 {
		t.Fatalf("OAuth: %+v", w)
	}
}

func TestHermesAnthropicMitKurzform(t *testing.T) {
	hermesOrdner(t, "model:\n  default: claude-sonnet-5-5\n  provider: anthropic\n  api_key: 'sk-ant-1'\n", "")
	w := hermesModelle()
	if len(w) != 1 || w[0].Agent != "claude" || w[0].URL != "https://api.anthropic.com" || w[0].Key != "wert:sk-ant-1" {
		t.Fatalf("anthropic: %+v", w)
	}
	if m := hermesModellAbschnitt("model: \"gpt-5\"\nother: 1\n"); m["model"] != "gpt-5" {
		t.Fatalf("Kurzform: %v", m)
	}
}

func TestHalbFertigeVerbindung(t *testing.T) {
	gescheitert := "✗ Failed to connect: Connecting to MCP server 'swarvex' timed out after 30s\n✓ Saved 'swarvex' to config (disabled)\n"
	if !halbFertig([]byte(gescheitert)) {
		t.Error("gescheiterte Hermes-Anmeldung nicht erkannt")
	}
	if halbFertig([]byte("✓ OAuth configured\n✓ Connected to 'swarvex' (12 tools)\n")) {
		t.Error("gelungene Verbindung als gescheitert erkannt")
	}
}

// Weg „Something else“ mit OpenRouter: Enter übernimmt den Schlüssel aus Hermes' .env, ein abgelehnter wird nicht
// wieder angeboten, ohne Schlüssel fragt setup nach.
func TestEigenerSchluessel(t *testing.T) {
	anbieter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/key" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-or-gut" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`{"data":{}}`))
	}))
	defer anbieter.Close()
	adresse := anbieter.URL + "/api"
	frag := func(antworten string) *frager { return &frager{r: bufio.NewReader(strings.NewReader(antworten))} }

	hermesOrdner(t, "model:\n  default: x/y\n", "OPENROUTER_API_KEY=sk-or-gut\n")
	if k := eigenerSchluessel(frag("\n"), adresse, "OpenRouter"); k != "sk-or-gut" {
		t.Fatalf("Enter sollte den Hermes-Schlüssel nehmen: %q", k)
	}
	// abgelehnter Hermes-Schlüssel: nicht behalten, dann einen eigenen eingeben
	hermesOrdner(t, "model:\n  default: x/y\n", "OPENROUTER_API_KEY=sk-or-alt\n")
	if k := eigenerSchluessel(frag("\nn\nsk-or-gut\n"), adresse, "OpenRouter"); k != "sk-or-gut" {
		t.Fatalf("nach Ablehnung: %q", k)
	}
	// nichts vorhanden, erst leer, dann ein Schlüssel; aus der Umgebung, wenn Hermes keinen hat
	hermesOrdner(t, "model:\n  default: x/y\n", "")
	if k := eigenerSchluessel(frag("\nsk-or-gut\n"), adresse, "OpenRouter"); k != "sk-or-gut" {
		t.Fatalf("ohne Vorschlag: %q", k)
	}
	t.Setenv("OPENROUTER_API_KEY", "sk-or-gut")
	if k := eigenerSchluessel(frag("\n"), adresse, "OpenRouter"); k != "sk-or-gut" {
		t.Fatalf("aus der Umgebung: %q", k)
	}
	// dreimal nichts: ohne Schlüssel weiter; lokale Server fragen wie bisher
	t.Setenv("OPENROUTER_API_KEY", "")
	if k := eigenerSchluessel(frag("\n\n\n"), adresse, "OpenRouter"); k != "" {
		t.Fatalf("leer erwartet: %q", k)
	}
	if k := eigenerSchluessel(frag("\n"), "http://host.containers.internal:11434", ""); k != "" {
		t.Fatalf("lokal: %q", k)
	}
	for adresse, soll := range map[string]string{"https://openrouter.ai/api": "OpenRouter", "https://api.anthropic.com": "Anthropic",
		"https://api.openai.com": "OpenAI", "http://host.containers.internal:11434": "", "https://api.deepseek.com": ""} {
		if ist := cloudAnbieter(adresse); ist != soll {
			t.Errorf("%s: %q statt %q", adresse, ist, soll)
		}
	}
}

func TestSchluesselAbgelehnt(t *testing.T) {
	anbieter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" && (r.Header.Get("x-api-key") == "gut" || r.Header.Get("Authorization") == "Bearer gut") {
			return
		}
		w.WriteHeader(401)
	}))
	defer anbieter.Close()
	for _, a := range []string{"Anthropic", "OpenAI"} {
		if ab, code := schluesselAbgelehnt(a, anbieter.URL, "gut"); ab || code != 200 {
			t.Errorf("%s gut: %v %d", a, ab, code)
		}
		if ab, code := schluesselAbgelehnt(a, anbieter.URL, "falsch"); !ab || code != 401 {
			t.Errorf("%s falsch: %v %d", a, ab, code)
		}
	}
	if ab, code := schluesselAbgelehnt("OpenAI", "http://127.0.0.1:1", "x"); ab || code != 0 {
		t.Errorf("ohne Antwort: %v %d", ab, code)
	}
}
