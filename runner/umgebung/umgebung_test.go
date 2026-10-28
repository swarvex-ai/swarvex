package umgebung

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func projekt(t *testing.T, dateien map[string]string) string {
	t.Helper()
	w := t.TempDir()
	for name, inhalt := range dateien {
		pfad := filepath.Join(w, name)
		os.MkdirAll(filepath.Dir(pfad), 0o755)
		if err := os.WriteFile(pfad, []byte(inhalt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return w
}

func TestErkennung(t *testing.T) {
	faelle := []struct {
		dateien map[string]string
		vorlage string
		testen  bool
	}{
		{map[string]string{"game/project.godot": `config/features=PackedStringArray("4.7", "Forward Plus")`}, "godot-4.7", true},
		{map[string]string{"package.json": `{"scripts":{"test":"vitest run"}}`, "package-lock.json": "{}"}, "node", true},
		{map[string]string{"package.json": `{"scripts":{"test":"echo \"Error: no test specified\" && exit 1"}}`}, "node", false},
		{map[string]string{"pyproject.toml": "[project]\nname='x'"}, "python", true},
		{map[string]string{"Cargo.toml": "[package]"}, "rust", true},
		{map[string]string{"go.mod": "module x"}, "go", true},
		{map[string]string{"App.csproj": "<Project/>"}, "dotnet", true},
		{map[string]string{"pom.xml": "<project/>"}, "java-maven", true},
		{map[string]string{"README.md": "nur Text"}, "default", false},
	}
	for _, f := range faelle {
		u, err := Ermitteln(projekt(t, f.dateien))
		if err != nil {
			t.Fatal(err)
		}
		if u.Vorlage != f.vorlage || (u.Testen != "") != f.testen {
			t.Errorf("%v: Vorlage %q (Tests %v), erwartet %q (Tests %v)", f.dateien, u.Vorlage, u.Testen != "", f.vorlage, f.testen)
		}
		if u.Image == "" && u.Basis == "" {
			t.Errorf("%v: keine Basis", f.dateien)
		}
	}
	u, _ := Ermitteln(projekt(t, map[string]string{"game/project.godot": `config/features=PackedStringArray("4.7")`}))
	if u.Ordner != "game" || !strings.Contains(u.Basis, "Godot_v4.7.2-stable") {
		t.Errorf("Godot: Ordner %q, Basis %q", u.Ordner, u.Basis)
	}
}

func TestSwarvexYaml(t *testing.T) {
	w := projekt(t, map[string]string{"package.json": `{}`, "swarvex.yaml": "# comment\ndir: web/\ntest: |\n  npm run lint\n  npm test\nnote: \"Bitte Prettier\"  # style\n"})
	u, err := Ermitteln(w)
	if err != nil {
		t.Fatal(err)
	}
	if u.Vorlage != "node" || u.Quelle != "swarvex.yaml" || u.Ordner != "web" || u.Testen != "npm run lint\nnpm test" || u.Hinweis != "Bitte Prettier" {
		t.Errorf("unerwartet: %+v", u)
	}
	u, err = Ermitteln(projekt(t, map[string]string{"swarvex.yaml": "image: ghcr.io/beispiel/unity:2023\ntest: ./test.sh\n"}))
	if err != nil || u.Image != "ghcr.io/beispiel/unity:2023" || u.Vorlage != "custom-image" {
		t.Errorf("eigenes Image: %+v %v", u, err)
	}
	for _, kaputt := range []string{"template: cobol\n", "unknown: 1\n", "  indented: 1\n", "testen: echo\n"} {
		if _, err := Ermitteln(projekt(t, map[string]string{"swarvex.yaml": kaputt})); err == nil {
			t.Errorf("%q hätte abgelehnt werden müssen", kaputt)
		}
	}
}

func TestDevcontainer(t *testing.T) {
	w := projekt(t, map[string]string{".devcontainer/devcontainer.json": `{
  // Kommentar
  "build": { "dockerfile": "Dockerfile", "context": ".." },
  "postCreateCommand": ["npm", "ci"],
  "onCreateCommand": { "a": "echo eins", "b": "echo zwei" },
}`, ".devcontainer/Dockerfile": "FROM node:24", "package.json": `{"scripts":{"test":"node --test"}}`})
	u, err := Ermitteln(w)
	if err != nil {
		t.Fatal(err)
	}
	if u.Dockerfile != ".devcontainer/Dockerfile" || u.Kontext != "." || u.Einrichten != "echo eins\necho zwei\n'npm' 'ci'" || u.Testen != "npm test" {
		t.Errorf("unerwartet: %+v", u)
	}
	if !strings.HasPrefix(u.Containerfile("swarvex-basis-x", "swarvex-agent"), "FROM swarvex-basis-x\n") {
		t.Error("Containerfile nutzt das gebaute Basis-Image nicht")
	}
}

func TestYamlUeberDevcontainer(t *testing.T) {
	u, err := Ermitteln(projekt(t, map[string]string{".devcontainer/devcontainer.json": `{"image":"docker.io/library/debian:trixie-slim","postCreateCommand":"echo ja"}`, "swarvex.yaml": "test: echo test\n"}))
	if err != nil || u.Image != "docker.io/library/debian:trixie-slim" || u.Einrichten != "echo ja" || u.Testen != "echo test" || u.Quelle != "swarvex.yaml" {
		t.Errorf("swarvex.yaml soll nur Befehle überschreiben: %+v %v", u, err)
	}
}

func TestGeschuetzt(t *testing.T) {
	for _, p := range []string{"swarvex.yaml", ".devcontainer/Dockerfile", "AGENTS.md", ".github/workflows/ci.yml"} {
		if !IstGeschuetzt(p) {
			t.Errorf("%s müsste geschützt sein", p)
		}
	}
	for _, p := range []string{"src/swarvex.yaml", "docs/AGENTS.md.bak", ".github/CODEOWNERS"} {
		if IstGeschuetzt(p) {
			t.Errorf("%s dürfte nicht geschützt sein", p)
		}
	}
	if !IstGeschuetzt("LICENSE.md") {
		t.Error("Lizenzdateien müssen geschützt sein")
	}
	u, err := Ermitteln(projekt(t, map[string]string{"go.mod": "module x", "swarvex.yaml": "protected: |\n  - tests/vertraege/**\n  - tools/**\n"}))
	if err != nil || !u.Gesperrt("tests/vertraege/a_test.go") || !u.Gesperrt("tools/x.sh") || u.Gesperrt("src/a.go") || !u.Gesperrt("AGENTS.md") {
		t.Errorf("projekteigene Sperren: %+v %v", u.Geschuetzt, err)
	}
}

func TestProprietaereEngines(t *testing.T) {
	u, err := Ermitteln(projekt(t, map[string]string{"client/ProjectSettings/ProjectVersion.txt": "m_EditorVersion: 6000.0.32f1\nm_EditorVersionWithRevision: 6000.0.32f1 (abc)\n"}))
	if err != nil {
		t.Fatal(err)
	}
	if u.Vorlage != "unity" || u.Ordner != "client" || u.Engine == nil || u.Engine.Version != "6000.0.32f1" || u.Testen == "" {
		t.Errorf("Unity: %+v", u)
	}
	if !u.Engine.Passt("unity", "6000.0.32f1") || u.Engine.Passt("unity", "6000.0.33f1") || u.Engine.Passt("unreal", "6000.0.32f1") {
		t.Error("Unity: Versionen müssen exakt passen")
	}

	u, err = Ermitteln(projekt(t, map[string]string{"Robo.uproject": `{"FileVersion": 3, "EngineAssociation": "5.5"}`, "Source/Robo/Robo.Build.cs": "x"}))
	if err != nil {
		t.Fatal(err)
	}
	if u.Vorlage != "unreal" || u.Ordner != "" || u.Engine == nil || u.Engine.Version != "5.5" || u.Bauen == "" || !strings.Contains(u.Testen, "RunTests Robo.") {
		t.Errorf("Unreal: %+v", u)
	}
	if !u.Engine.Passt("unreal", "5.5.4") || u.Engine.Passt("unreal", "5.4.4") {
		t.Error("Unreal: major.minor muss passen")
	}
	// Eigene Engine-Builds tragen eine GUID statt einer Version: dann passt jede Unreal-Installation.
	u, _ = Ermitteln(projekt(t, map[string]string{"g/Robo.uproject": `{"EngineAssociation": "{A1B2C3}"}`}))
	if u.Ordner != "g" || u.Engine.Version != "" || u.Bauen != "" || !u.Engine.Passt("unreal", "5.6.0") {
		t.Errorf("Unreal ohne Version: %+v", u)
	}

	// Vorlage über swarvex.yaml.
	u, err = Ermitteln(projekt(t, map[string]string{"ProjectSettings/ProjectVersion.txt": "m_EditorVersion: 2022.3.50f1\n", "swarvex.yaml": "template: unity\n"}))
	if err != nil || u.Engine == nil || u.Engine.Version != "2022.3.50f1" {
		t.Errorf("Unity per Vorlage: %+v, %v", u, err)
	}
	if IstProprietaer("godot") || !IstProprietaer("Unity 6") || !IstProprietaer("unreal") {
		t.Error("IstProprietaer")
	}
}
