package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// swarvex status nennt Modell, Verbindung und Agent, wie der Dienst sie startet (Wunsch von neVs, 12.11.2026).
func TestModellZeilen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, f := range []struct {
		args                      []string
		modell, verbindung, agent string
	}{
		{[]string{"--project", "formwerk", "--model", "google/gemini-3.8-flash", "--model-url", "https://openrouter.ai/api", "--model-key-file", "/home/x/.config/swarvex/key"},
			"google/gemini-3.8-flash", "OpenRouter (https://openrouter.ai/api)", "opencode"},
		{[]string{"--agent=claude", "--model=claude-sonnet-5-5"}, "claude-sonnet-5-5", "Anthropic", "claude"},
		{[]string{"--agent", "codex", "--model", "gpt-5.5", "--model-url", "https://chatgpt.com/backend-api/codex"}, "gpt-5.5", "ChatGPT subscription", "codex"},
		{[]string{"--model", "qwen3.8", "--model-url", "http://192.168.10.20:11434"}, "qwen3.8", "local server (http://192.168.10.20:11434)", "opencode"},
		{[]string{"--model", "x", "--model-url", "https://llm.example.org/api"}, "x", "llm.example.org (https://llm.example.org/api)", "opencode"},
		{[]string{"--project", "formwerk"}, "the agent's default", "the agent's default", "opencode"},
	} {
		want := [][2]string{{"Model", f.modell}, {"Connection", f.verbindung}, {"Agent", f.agent}}
		if got := modellZeilen(f.args); !reflect.DeepEqual(got, want) {
			t.Errorf("%v:\n got  %v\n want %v", f.args, got, want)
		}
	}
	// Ohne --model: das Standardmodell aus opencodes eigener Konfiguration
	os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".config", "opencode"), 0o755)
	os.WriteFile(filepath.Join(os.Getenv("HOME"), ".config", "opencode", "opencode.json"), []byte(`{"model": "openrouter/google/gemini-3.8-flash"}`), 0o644)
	if got := modellZeilen([]string{"--model-url", "https://openrouter.ai/api"})[0][1]; got != "openrouter/google/gemini-3.8-flash (opencode default)" {
		t.Errorf("opencode default: %s", got)
	}
}

func TestArbeitZeilen(t *testing.T) {
	got := arbeitZeilen([]string{"--project", "auto", "--kinds", "game,teaching", "--schedule", "mon-fri 18:00-23:00; sat 09:00-12:00", "--only-when-idle",
		"--continuous", "--peer-reviews=false", "--media", "claude:meshy,hermes:elevenlabs", "--media-daily", "30"})
	want := [][2]string{
		{"Project", "auto (wherever most tasks are open), kinds game, teaching"},
		{"Hours", "mon-fri 18:00-23:00, sat 09:00-12:00, only while nobody uses this computer"},
		{"Peer reviews", "off"},
		{"Media", "claude:meshy, hermes:elevenlabs, at most 30 briefs a day"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("\n got  %v\n want %v", got, want)
	}
	if got := arbeitZeilen([]string{"--project", "formwerk", "--cross-checks-only"}); !reflect.DeepEqual(got, [][2]string{
		{"Project", "formwerk"}, {"Hours", "always"}, {"Mode", "only cross-checks of other people's submissions (no model)"}}) {
		t.Errorf("cross-checks only: %v", got)
	}
	// Ein Schalter, dem ein anderes Argument folgt, nimmt dieses nicht als Wert
	if argGesetzt([]string{"--only-when-idle", "--project", "x"}, "only-when-idle") != true || argGesetzt([]string{"--only-when-idle=false"}, "only-when-idle") {
		t.Error("argGesetzt")
	}
}

func TestVerbrauchText(t *testing.T) {
	for _, f := range []struct {
		verbraucht, budget int64
		text               string
	}{
		{15644235, 2000000, "15,644,235 of 2,000,000 a day, used up"},
		{512000, 2000000, "512,000 of 2,000,000 a day (25 %)"},
		{999, 0, "999 (no daily budget)"},
		{1234567890, 0, "1,234,567,890 (no daily budget)"},
	} {
		if got := verbrauchText(f.verbraucht, f.budget); got != f.text {
			t.Errorf("%d/%d: %q", f.verbraucht, f.budget, got)
		}
	}
}

// Die letzten Meldungen mit Uhrzeit; Podman, Proxy und systemd-Statistik fallen weg, Start und Stopp stehen kurz da.
func TestJournalMeldungen(t *testing.T) {
	zeit := func(s string) int64 {
		z, _ := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
		return z.UnixMicro()
	}
	zeile := func(s, quelle, text string) string {
		return fmt.Sprintf(`{"__REALTIME_TIMESTAMP":"%d","SYSLOG_IDENTIFIER":%q,"MESSAGE":%q}`, zeit(s), quelle, text)
	}
	journal := strings.Join([]string{
		zeile("2026-11-12 15:30", "systemd", "Stopping swarvex.service - swarvex runner (works on open-source tasks within its time windows)..."),
		zeile("2026-11-12 15:30", "systemd", "Stopped swarvex.service - swarvex runner (works on open-source tasks within its time windows)."),
		zeile("2026-11-12 15:30", "systemd", "Started swarvex.service - swarvex runner (works on open-source tasks within its time windows)."),
		zeile("2026-11-12 15:31", "conmon", "conmon 4ed90a16b7f4542af8ec <nwarn>: Failed to open cgroups file"),
		zeile("2026-11-12 15:31", "podman", "container died dbdbac2aaa25 (image=localhost/swarvex-proxy:latest, name=swarvex-proxy-199)"),
		zeile("2026-11-12 15:31", "systemd", "swarvex.service: Consumed 14.265s CPU time."),
		`{"__REALTIME_TIMESTAMP":"1","MESSAGE":[1,2,3]}`,
		"kein json",
		zeile("2026-11-12 15:31", "swarvex", "Daily budget used up. Until Fri 13 Nov 00:00 only cross-checks of other people's submissions (no model, no tokens)."),
	}, "\n")
	got := journalMeldungen(journal, 6)
	var texte []string
	for _, m := range got {
		texte = append(texte, m.Zeit.Format("15:04")+" "+m.Text)
	}
	want := []string{"15:30 Service stopped", "15:30 Service started",
		"15:31 Daily budget used up. Until Fri 13 Nov 00:00 only cross-checks of other people's submissions (no model, no tokens)."}
	if !reflect.DeepEqual(texte, want) {
		t.Errorf("\n got  %q\n want %q", texte, want)
	}
	if len(journalMeldungen(journal, 1)) != 1 {
		t.Error("n")
	}
	jetzt := time.Date(2026, 11, 12, 18, 0, 0, 0, time.Local)
	if uhrzeit(got[0].Zeit, jetzt) != "15:30" || uhrzeit(got[0].Zeit, jetzt.AddDate(0, 0, 1)) != "Thu 12 Nov 15:30" {
		t.Errorf("uhrzeit: %s / %s", uhrzeit(got[0].Zeit, jetzt), uhrzeit(got[0].Zeit, jetzt.AddDate(0, 0, 1)))
	}
}

func TestUmbrechen(t *testing.T) {
	text := "  Waiting       until Fri 13 Nov 00:00: daily budget used up; meanwhile cross-checks (no model)"
	got := umbrechen(text, 60, 16)
	want := "  Waiting       until Fri 13 Nov 00:00: daily budget used\n                up; meanwhile cross-checks (no model)"
	if got != want {
		t.Errorf("\n got  %q\n want %q", got, want)
	}
	for _, z := range strings.Split(got, "\n") {
		if len(z) > 60 {
			t.Errorf("too long: %q", z)
		}
	}
	if umbrechen("kurz", 60, 15) != "kurz" {
		t.Error("short text changed")
	}
}
