package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

func datei(t *testing.T, pfad, inhalt string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(pfad), 0o755)
	if err := os.WriteFile(pfad, []byte(inhalt), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestEnginesFinden(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SWARVEX_UNITY", "")
	eigen := filepath.Join(t.TempDir(), "UE_5.5")
	t.Setenv("SWARVEX_UNREAL", eigen)
	datei(t, filepath.Join(home, "Unity/Hub/Editor/6000.0.32f1/Editor/Unity"), "")
	datei(t, filepath.Join(home, "Unity/Hub/Editor/2022.3.50f1/Editor/Unity"), "")
	datei(t, filepath.Join(home, "Unity/Hub/Editor/kaputt/README"), "") // ohne Editor: keine Installation
	datei(t, filepath.Join(home, ".local/share/unity3d/Unity/Unity_lic.ulf"), "<License/>")
	datei(t, filepath.Join(eigen, "Engine/Binaries/Linux/UnrealEditor-Cmd"), "")
	datei(t, filepath.Join(eigen, "Engine/Build/Build.version"), `{"MajorVersion": 5, "MinorVersion": 5, "PatchVersion": 4}`)

	alle := enginesFinden()
	if len(alle) != 3 || alle[0].Version != "2022.3.50f1" || alle[1].Version != "6000.0.32f1" || alle[2].Version != "5.5.4" {
		t.Fatalf("gefunden: %v", alle)
	}
	i, err := engineFuer(umgebung.Umgebung{Engine: &umgebung.Engine{Name: "unity", Version: "6000.0.32f1"}}, alle)
	if err != nil || i.Version != "6000.0.32f1" || len(i.Lizenz) != 1 {
		t.Fatalf("Unity: %v %v", i, err)
	}
	args := strings.Join(engineArgs(i), " ")
	for _, soll := range []string{":/opt/engine:ro", "SWARVEX_ENGINE=/opt/engine/Editor/Unity", "Unity_lic.ulf:/home/agent/.local/share/unity3d/Unity/Unity_lic.ulf:ro"} {
		if !strings.Contains(args, soll) {
			t.Errorf("Unity-Argumente ohne %q: %s", soll, args)
		}
	}
	i, err = engineFuer(umgebung.Umgebung{Engine: &umgebung.Engine{Name: "unreal", Version: "5.5"}}, alle)
	if err != nil || !strings.Contains(strings.Join(engineArgs(i), " "), "UE-LocalDataCachePath=/cache/ue-ddc") {
		t.Fatalf("Unreal: %v %v", i, err)
	}
	if _, err := engineFuer(umgebung.Umgebung{Engine: &umgebung.Engine{Name: "unity", Version: "6000.1.0f1"}}, alle); err == nil || !strings.Contains(err.Error(), "Unity Hub") {
		t.Errorf("fehlende Version: %v", err)
	}
	if i, err := engineFuer(umgebung.Umgebung{}, alle); err != nil || engineArgs(i) != nil {
		t.Errorf("Projekt ohne proprietäre Engine: %v %v", i, err)
	}
}
