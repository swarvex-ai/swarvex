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
		{map[string]string{"README.md": "nur Text"}, "standard", false},
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
	w := projekt(t, map[string]string{"package.json": `{}`, "swarvex.yaml": "# Kommentar\nordner: web/\ntesten: |\n  npm run lint\n  npm test\nhinweis: \"Bitte Prettier\"  # Stil\n"})
	u, err := Ermitteln(w)
	if err != nil {
		t.Fatal(err)
	}
	if u.Vorlage != "node" || u.Quelle != "swarvex.yaml" || u.Ordner != "web" || u.Testen != "npm run lint\nnpm test" || u.Hinweis != "Bitte Prettier" {
		t.Errorf("unerwartet: %+v", u)
	}
	u, err = Ermitteln(projekt(t, map[string]string{"swarvex.yaml": "image: ghcr.io/beispiel/unity:2023\ntesten: ./test.sh\n"}))
	if err != nil || u.Image != "ghcr.io/beispiel/unity:2023" || u.Vorlage != "eigenes-image" {
		t.Errorf("eigenes Image: %+v %v", u, err)
	}
	for _, kaputt := range []string{"vorlage: cobol\n", "unbekannt: 1\n", "  eingerueckt: 1\n"} {
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
	u, err := Ermitteln(projekt(t, map[string]string{".devcontainer/devcontainer.json": `{"image":"docker.io/library/debian:trixie-slim","postCreateCommand":"echo ja"}`, "swarvex.yaml": "testen: echo test\n"}))
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
	u, err := Ermitteln(projekt(t, map[string]string{"go.mod": "module x", "swarvex.yaml": "geschuetzt: |\n  - tests/vertraege/**\n  - tools/**\n"}))
	if err != nil || !u.Gesperrt("tests/vertraege/a_test.go") || !u.Gesperrt("tools/x.sh") || u.Gesperrt("src/a.go") || !u.Gesperrt("AGENTS.md") {
		t.Errorf("projekteigene Sperren: %+v %v", u.Geschuetzt, err)
	}
}
