package main

import (
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
