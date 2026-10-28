package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillsKopieren(t *testing.T) {
	eigen, projekt, ziel := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "skills")
	datei(t, filepath.Join(eigen, "tdd", "SKILL.md"), "eigen")
	datei(t, filepath.Join(eigen, "tdd", "scripts", "run.sh"), "echo")
	datei(t, filepath.Join(eigen, "kein-skill", "README.md"), "x")
	datei(t, filepath.Join(eigen, "tdd", ".git", "config"), "geheim")
	datei(t, filepath.Join(projekt, "tdd", "SKILL.md"), "projekt")
	extern := filepath.Join(t.TempDir(), "debug")
	datei(t, filepath.Join(extern, "SKILL.md"), "debug")
	os.Symlink(extern, filepath.Join(eigen, "debug"))

	n, err := skillsKopieren(ziel, []string{eigen, filepath.Join(t.TempDir(), "gibt-es-nicht"), projekt})
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	lies := func(p string) string { roh, _ := os.ReadFile(filepath.Join(ziel, p)); return string(roh) }
	// Der Projekt-Skill ersetzt den gleichnamigen eigenen ganz (auch dessen Skripte).
	if lies("tdd/SKILL.md") != "projekt" || lies("debug/SKILL.md") != "debug" || lies("tdd/scripts/run.sh") != "" {
		t.Errorf("Inhalt falsch: %q %q %q", lies("tdd/SKILL.md"), lies("debug/SKILL.md"), lies("tdd/scripts/run.sh"))
	}
	if _, err := os.Stat(filepath.Join(ziel, "kein-skill")); err == nil {
		t.Error("Ordner ohne SKILL.md kopiert")
	}
	if _, err := os.Stat(filepath.Join(ziel, "tdd", ".git")); err == nil {
		t.Error(".git kopiert")
	}
}

func TestAgentStart(t *testing.T) {
	tok := strings.Repeat("t", 40)
	for _, name := range []string{"opencode", "claude", "codex"} {
		ordner := t.TempDir()
		baum := t.TempDir()
		datei(t, filepath.Join(baum, ".agents", "skills", "projekt", "SKILL.md"), "p")
		args, befehl, err := agentStart(agenten[name], ordner, "http://swarvex-proxy-1:8080", tok, "Mein Modell", "AUFTRAG", nil, baum, "")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		alles := strings.Join(args, " ")
		if befehl[0] != name || befehl[len(befehl)-1] != "AUFTRAG" {
			t.Errorf("%s: Befehl %v", name, befehl)
		}
		if !strings.Contains(alles, "SWARVEX_TOKEN="+tok) || !strings.Contains(alles, "HOME=/home/agent") {
			t.Errorf("%s: Grundumgebung fehlt: %s", name, alles)
		}
		switch name {
		case "claude":
			for _, soll := range []string{"ANTHROPIC_BASE_URL=http://swarvex-proxy-1:8080", "ANTHROPIC_AUTH_TOKEN=" + tok,
				"ANTHROPIC_DEFAULT_HAIKU_MODEL=Mein Modell", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "DISABLE_AUTOUPDATER=1",
				":/home/agent/.claude/skills:ro", "/home/agent/.claude:rw,exec,size=256m,mode=1777"} {
				if !strings.Contains(alles, soll) {
					t.Errorf("claude ohne %q", soll)
				}
			}
		case "codex":
			for _, soll := range []string{`model_providers.swarvex={name="swarvex proxy", base_url="http://swarvex-proxy-1:8080/v1", env_key="SWARVEX_TOKEN", wire_api="responses"}`,
				`model_provider="swarvex"`} {
				if !strings.Contains(strings.Join(befehl, " "), soll) {
					t.Errorf("codex ohne %q: %v", soll, befehl)
				}
			}
			if strings.Contains(alles, "/skills") {
				t.Error("codex: Projekt-Skills liest Codex selbst, keine Kopie nötig")
			}
		case "opencode":
			roh, _ := os.ReadFile(filepath.Join(ordner, "opencode.json"))
			if !strings.Contains(string(roh), "http://swarvex-proxy-1:8080/v1") {
				t.Errorf("opencode.json: %s", roh)
			}
		}
	}
	if _, _, err := agentStart(agentArt{Name: "unbekannt"}, t.TempDir(), "", tok, "m", "a", nil, t.TempDir(), ""); err == nil {
		t.Error("unbekannter Agent ohne Fehler")
	}
	// Projekt im Unterordner: Projekt-Skills aus der Wurzel bekommt auch Codex als Kopie.
	ordner, baum := t.TempDir(), t.TempDir()
	datei(t, filepath.Join(baum, ".agents", "skills", "wurzel", "SKILL.md"), "w")
	args, _, err := agentStart(agenten["codex"], ordner, "http://p:8080", tok, "m", "a", nil, baum, "game")
	if err != nil || !strings.Contains(strings.Join(args, " "), ":/home/agent/.agents/skills:ro") {
		t.Errorf("Unterordner: %v %v", args, err)
	}
	if _, err := os.Stat(filepath.Join(ordner, "skills", "wurzel", "SKILL.md")); err != nil {
		t.Error("Wurzel-Skill nicht kopiert")
	}
}
