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
