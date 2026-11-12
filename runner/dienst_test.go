package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDienstEinheit(t *testing.T) {
	e := dienstEinheit("/home/a b/.local/bin/swarvex", []string{"--project", "game", "--model", "Qwen 3 $x", "--schedule", "mon-fri 18:00-23:00; sat 09:00-12:00"})
	for _, soll := range []string{
		`ExecStart="/home/a b/.local/bin/swarvex" work --continuous --project game --model "Qwen 3 $$x" --schedule "mon-fri 18:00-23:00; sat 09:00-12:00"`,
		"KillMode=control-group", "Restart=on-failure", "WantedBy=default.target",
	} {
		if !strings.Contains(e, soll) {
			t.Errorf("fehlt: %s\n%s", soll, e)
		}
	}
}

func TestDienstHinweis(t *testing.T) {
	for zustand, soll := range map[string]string{
		"active": "", "activating": "", "reloading": "",
		"inactive": "swarvex start", "failed": "swarvex logs",
	} {
		h := dienstHinweis(zustand)
		if (soll == "") != (h == "") || !strings.Contains(h, soll) {
			t.Errorf("%s: %q, erwartet mit %q", zustand, h, soll)
		}
	}
}

func TestAbsolutePfade(t *testing.T) {
	wd, _ := os.Getwd()
	r := absolutePfade([]string{"--project", "x", "--model-key-file", "key.txt", "--model-key-file=/abs/k"})
	soll := []string{"--project", "x", "--model-key-file=" + filepath.Join(wd, "key.txt"), "--model-key-file=/abs/k"}
	if strings.Join(r, "|") != strings.Join(soll, "|") {
		t.Errorf("%v != %v", r, soll)
	}
}

func TestTagesverbrauch(t *testing.T) {
	datei := filepath.Join(t.TempDir(), "verbrauch.jsonl")
	ort := time.FixedZone("CET", 3600)
	heute := time.Date(2026, 10, 26, 10, 0, 0, 0, ort)
	verbrauchMerken(datei, 1, 1000, heute.Add(-11*time.Hour)) // gestern 23:00
	verbrauchMerken(datei, 2, 2500, heute.Add(-9*time.Hour))  // heute 01:00
	verbrauchMerken(datei, 3, 500, heute)
	if v := verbrauchAmTag(datei, heute); v != 3000 {
		t.Errorf("heute %d, erwartet 3000", v)
	}
	if v := verbrauchAmTag(filepath.Join(t.TempDir(), "fehlt"), heute); v != 0 {
		t.Errorf("ohne Datei %d", v)
	}
	if m := mitternacht(heute); !m.Equal(time.Date(2026, 10, 27, 0, 0, 0, 0, ort)) {
		t.Errorf("mitternacht %v", m)
	}
}

// swarvex status nennt Modell und Verbindung, wie der Dienst sie startet (Wunsch von neVs, 12.11.2026).
func TestModellText(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, f := range []struct {
		args []string
		text string
	}{
		{[]string{"--project", "formwerk", "--model", "google/gemini-3.8-flash", "--model-url", "https://openrouter.ai/api", "--model-key-file", "/home/x/.config/swarvex/key"},
			"Model: google/gemini-3.8-flash via OpenRouter (https://openrouter.ai/api), agent opencode"},
		{[]string{"--agent=claude", "--model=claude-sonnet-5-5"}, "Model: claude-sonnet-5-5 via Anthropic, agent claude"},
		{[]string{"--agent", "codex", "--model", "gpt-5.5", "--model-url", "https://chatgpt.com/backend-api/codex"}, "Model: gpt-5.5 via ChatGPT subscription, agent codex"},
		{[]string{"--model", "qwen3.8", "--model-url", "http://192.168.10.20:11434"}, "Model: qwen3.8 via local server (http://192.168.10.20:11434), agent opencode"},
		{[]string{"--model", "x", "--model-url", "https://llm.example.org/api"}, "Model: x via llm.example.org (https://llm.example.org/api), agent opencode"},
		{[]string{"--project", "formwerk"}, "Model: the agent's default via the agent's default connection, agent opencode"},
	} {
		if got := modellText(f.args); got != f.text {
			t.Errorf("%v:\n got  %s\n want %s", f.args, got, f.text)
		}
	}
	// Ohne --model: das Standardmodell aus opencodes eigener Konfiguration
	os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".config", "opencode"), 0o755)
	os.WriteFile(filepath.Join(os.Getenv("HOME"), ".config", "opencode", "opencode.json"), []byte(`{"model": "openrouter/google/gemini-3.8-flash"}`), 0o644)
	if got := modellText([]string{"--model-url", "https://openrouter.ai/api"}); got != "Model: openrouter/google/gemini-3.8-flash (opencode default) via OpenRouter (https://openrouter.ai/api), agent opencode" {
		t.Errorf("opencode default: %s", got)
	}
}
