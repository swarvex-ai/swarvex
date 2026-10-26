// Package umgebung ermittelt, in welcher Umgebung ein Projekt gebaut und getestet wird.
// Gemeinsam genutzt von Runner (Mitstreiter) und Prüfstraße (Visionär), damit beide dasselbe sehen.
//
// Reihenfolge, die erste Quelle gewinnt:
//  1. swarvex.yaml im Projekt (alles frei wählbar: Image, Einrichten, Bauen, Testen, Ordner)
//  2. Dev Container (.devcontainer/devcontainer.json oder .devcontainer.json): Image bzw. Dockerfile
//     und onCreate-/updateContent-/postCreateCommand
//  3. Erkennung an typischen Dateien (Godot, Node, Python, Rust, Go, .NET, Java)
//  4. allgemeine Linux-Umgebung ohne automatische Tests
//
// Ablauf beim Arbeiten und Prüfen:
//   - Einrichten (Abhängigkeiten laden) läuft mit Netz, aber ohne Agent und ohne Schlüssel.
//   - Danach laufen Agent bzw. Tests ohne Netz; Abhängigkeiten liegen im Projektordner oder im Cache.
//
// Die Dateien, die die Umgebung bestimmen, sind für Beiträge gesperrt (Geschuetzt), damit niemand
// über einen Beitrag Befehle auf den Rechnern anderer Mitstreiter einschleusen kann.
package umgebung

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Umgebung beschreibt, wie ein Projekt eingerichtet, gebaut und getestet wird.
type Umgebung struct {
	Vorlage    string   // z. B. "node", "godot-4.7", "custom-image"
	Quelle     string   // "swarvex.yaml", "devcontainer", "detected", "default"
	Image      string   // fertiges Basis-Image (leer, wenn Containerfile gesetzt ist)
	Dockerfile string   // Pfad im Projekt zu einem eigenen Dockerfile (Dev Container)
	Kontext    string   // Build-Kontext dazu, relativ zum Projekt
	Basis      string   // Containerfile-Text für die Basis (Vorlagen ohne fertiges Image, z. B. Godot)
	Ordner     string   // Arbeitsordner relativ zur Projektwurzel ("" = Wurzel)
	Einrichten string   // Shell: Abhängigkeiten laden (mit Netz, ohne Agent)
	Bauen      string   // Shell: bauen (ohne Netz), optional
	Testen     string   // Shell: Tests (ohne Netz); leer = keine automatischen Tests
	Hinweis    string   // kurzer Satz für den Auftrag an den Agenten
	Geschuetzt []string // zusätzliche, projekteigene geschützte Pfade (z. B. tests/vertraege/**)
}

// Geschuetzt: Pfade, die Beiträge nie ändern dürfen (nur Lead bzw. Visionär direkt im Repo).
var Geschuetzt = []string{"swarvex.yaml", ".devcontainer/**", ".devcontainer.json", "AGENTS.md", ".github/workflows/**", ".gitlab-ci.yml", ".forgejo/workflows/**", ".gitea/workflows/**", "LICENSE*", "COPYING*"}

// IstGeschuetzt prüft einen Pfad gegen die festen geschützten Pfade.
func IstGeschuetzt(pfad string) bool { return trifft(Geschuetzt, pfad) }

// Gesperrt prüft einen Pfad gegen die festen und die projekteigenen geschützten Pfade.
func (u Umgebung) Gesperrt(pfad string) bool {
	return trifft(Geschuetzt, pfad) || trifft(u.Geschuetzt, pfad)
}

// trifft: Muster wie in den Aufgabenpfaden – exakt, mit * innerhalb eines Ordners, oder mit /** am Ende.
func trifft(muster []string, pfad string) bool {
	for _, g := range muster {
		if strings.HasSuffix(g, "/**") {
			if strings.HasPrefix(pfad, strings.TrimSuffix(g, "**")) {
				return true
			}
		} else if ok, _ := path.Match(g, pfad); ok {
			return true
		}
	}
	return false
}

// Umgebungsvariablen für Paketmanager-Caches unter /cache (bleibt pro Projekt erhalten, auch ohne Netz nutzbar).
var CacheVariablen = []string{
	"npm_config_cache=/cache/npm", "YARN_CACHE_FOLDER=/cache/yarn", "PNPM_HOME=/cache/pnpm", "COREPACK_HOME=/cache/corepack",
	"PIP_CACHE_DIR=/cache/pip", "UV_CACHE_DIR=/cache/uv", "CARGO_HOME=/cache/cargo", "GOMODCACHE=/cache/go/mod",
	"GOCACHE=/cache/go/build", "GOFLAGS=-modcacherw", "NUGET_PACKAGES=/cache/nuget", "MAVEN_OPTS=-Dmaven.repo.local=/cache/m2",
	"GRADLE_USER_HOME=/cache/gradle", "HOME=/home/agent", "XDG_CACHE_HOME=/cache/xdg",
}

// Ermitteln liest das Projekt unter wurzel und liefert seine Umgebung.
func Ermitteln(wurzel string) (Umgebung, error) {
	if roh, err := os.ReadFile(filepath.Join(wurzel, "swarvex.yaml")); err == nil {
		return ausYaml(wurzel, string(roh))
	}
	return ohneYaml(wurzel)
}

// ohneYaml: Dev Container, sonst Erkennung an typischen Dateien.
func ohneYaml(wurzel string) (Umgebung, error) {
	for _, p := range []string{".devcontainer/devcontainer.json", ".devcontainer.json"} {
		if roh, err := os.ReadFile(filepath.Join(wurzel, p)); err == nil {
			return ausDevcontainer(wurzel, p, roh)
		}
	}
	return erkennen(wurzel), nil
}

// Kennung: stabiler Name für das gebaute Umgebungs-Image (ändert sich, wenn sich die Basis ändert).
func (u Umgebung) Kennung(wurzel string) string {
	h := sha256.New()
	fmt.Fprintf(h, "v1\n%s\n%s\n%s\n", u.Image, u.Basis, u.Dockerfile)
	if u.Dockerfile != "" {
		if roh, err := os.ReadFile(filepath.Join(wurzel, u.Dockerfile)); err == nil {
			h.Write(roh)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Containerfile für die Umgebung (Basis + Agent-Schicht aus dem Image agentImage, falls angegeben).
// basisImage ersetzt Image/Basis, wenn vorher ein Dev-Container-Dockerfile gebaut wurde.
func (u Umgebung) Containerfile(basisImage, agentImage string) string {
	var b strings.Builder
	switch {
	case basisImage != "":
		fmt.Fprintf(&b, "FROM %s\n", basisImage)
	case u.Basis != "":
		b.WriteString(u.Basis + "\n")
	default:
		fmt.Fprintf(&b, "FROM %s\n", u.Image)
	}
	b.WriteString("USER root\n")
	if agentImage != "" {
		fmt.Fprintf(&b, "COPY --from=%s /opt/swarvex-agent /opt/swarvex-agent\n", agentImage)
	}
	// Keine Setuid-Programme (su, passwd …): Rechteaufstieg gar nicht erst anbieten.
	b.WriteString("RUN mkdir -p /work /cache /home/agent && chmod 1777 /work /cache /home/agent && (find / -xdev -perm -4000 -type f -exec chmod u-s {} + 2>/dev/null; true)\n")
	b.WriteString("ENV PATH=/opt/swarvex-agent/bin:$PATH\n")
	// Eigene Startskripte der Basis-Images (ENTRYPOINT) zurücksetzen: Es läuft genau unser Befehl.
	b.WriteString("ENTRYPOINT []\nCMD []\nWORKDIR /work\nUSER 1000:1000\n")
	return b.String()
}

// ---------- swarvex.yaml ----------

// ausYaml liest die einfache YAML-Untermenge von swarvex.yaml: „schluessel: wert“ und
// „schluessel: |“ mit eingerückten Folgezeilen. Unbekannte Schlüssel sind ein Fehler.
func ausYaml(wurzel, text string) (Umgebung, error) {
	werte, err := yamlLesen(text)
	if err != nil {
		return Umgebung{}, err
	}
	u := Umgebung{Quelle: "swarvex.yaml"}
	if v, ok := werte["template"]; ok {
		basis, gibt := vorlagen()[v]
		if !gibt {
			return u, fmt.Errorf("swarvex.yaml: unknown template %q (known: %s)", v, strings.Join(vorlagenNamen(), ", "))
		}
		u = basis(wurzel)
		u.Quelle = "swarvex.yaml"
	} else if _, ok := werte["image"]; !ok {
		// Nur Befehle angepasst: Basis aus Dev Container bzw. Erkennung, die Angaben hier gelten vorrangig.
		var err error
		if u, err = ohneYaml(wurzel); err != nil {
			return u, err
		}
		u.Quelle = "swarvex.yaml"
	}
	for k, v := range werte {
		switch k {
		case "template":
		case "image":
			u.Image, u.Basis, u.Vorlage = v, "", "custom-image"
		case "dir":
			u.Ordner = strings.Trim(v, "/")
		case "setup":
			u.Einrichten = v
		case "build":
			u.Bauen = v
		case "test":
			u.Testen = v
		case "note":
			u.Hinweis = v
		case "protected":
			for _, g := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == '\n' }) {
				if g = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(g), "- ")); g != "" {
					u.Geschuetzt = append(u.Geschuetzt, g)
				}
			}
		default:
			return u, fmt.Errorf("swarvex.yaml: unknown key %q (allowed: template, image, dir, setup, build, test, note, protected)", k)
		}
	}
	if u.Image == "" && u.Basis == "" && u.Dockerfile == "" {
		return u, fmt.Errorf("swarvex.yaml: neither template nor image given and no project type detected")
	}
	return u, nil
}

func yamlLesen(text string) (map[string]string, error) {
	werte := map[string]string{}
	zeilen := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i := 0; i < len(zeilen); i++ {
		z := zeilen[i]
		if strings.TrimSpace(z) == "" || strings.HasPrefix(strings.TrimSpace(z), "#") {
			continue
		}
		if strings.HasPrefix(z, " ") || strings.HasPrefix(z, "\t") {
			return nil, fmt.Errorf("swarvex.yaml line %d: unexpected indentation", i+1)
		}
		k, v, ok := strings.Cut(z, ":")
		if !ok {
			return nil, fmt.Errorf("swarvex.yaml line %d: expected \"key: value\"", i+1)
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if v == "|" || v == "|-" || v == ">" {
			var block []string
			for i+1 < len(zeilen) && (strings.HasPrefix(zeilen[i+1], " ") || strings.HasPrefix(zeilen[i+1], "\t") || strings.TrimSpace(zeilen[i+1]) == "") {
				i++
				block = append(block, zeilen[i])
			}
			v = einruecken(block, v == ">")
		} else if strings.Contains(v, " #") {
			v = strings.TrimSpace(v[:strings.Index(v, " #")])
		}
		werte[k] = strings.Trim(v, `"'`)
	}
	return werte, nil
}

func einruecken(block []string, gefaltet bool) string {
	min := -1
	for _, z := range block {
		if strings.TrimSpace(z) == "" {
			continue
		}
		n := len(z) - len(strings.TrimLeft(z, " \t"))
		if min < 0 || n < min {
			min = n
		}
	}
	var aus []string
	for _, z := range block {
		if len(z) >= min && min >= 0 {
			z = z[min:]
		}
		aus = append(aus, strings.TrimRight(z, " \t"))
	}
	text := strings.TrimRight(strings.Join(aus, "\n"), "\n")
	if gefaltet {
		text = strings.Join(strings.Fields(text), " ")
	}
	return text
}

// ---------- Dev Container ----------

var jsoncKommentar = regexp.MustCompile(`(?m)^\s*//.*$|/\*[\s\S]*?\*/`)

func ausDevcontainer(wurzel, pfad string, roh []byte) (Umgebung, error) {
	var d struct {
		Image string `json:"image"`
		Build struct {
			Dockerfile string `json:"dockerfile"`
			Context    string `json:"context"`
		} `json:"build"`
		DockerFile      string          `json:"dockerFile"`
		OnCreate        json.RawMessage `json:"onCreateCommand"`
		UpdateContent   json.RawMessage `json:"updateContentCommand"`
		PostCreate      json.RawMessage `json:"postCreateCommand"`
		WorkspaceFolder string          `json:"workspaceFolder"`
	}
	sauber := jsoncKommentar.ReplaceAll(roh, nil)
	sauber = regexp.MustCompile(`,(\s*[}\]])`).ReplaceAll(sauber, []byte("$1")) // JSONC erlaubt Kommas am Ende
	if err := json.Unmarshal(sauber, &d); err != nil {
		return Umgebung{}, fmt.Errorf("%s: %v", pfad, err)
	}
	u := erkennen(wurzel) // Testbefehle aus der Erkennung, falls vorhanden
	u.Quelle, u.Vorlage, u.Basis, u.Image = "devcontainer", "devcontainer", "", d.Image
	ordner := filepath.Dir(pfad)
	if df := firstNonEmpty(d.Build.Dockerfile, d.DockerFile); df != "" {
		u.Image = ""
		u.Dockerfile = filepath.ToSlash(filepath.Join(ordner, df))
		u.Kontext = filepath.ToSlash(filepath.Join(ordner, firstNonEmpty(d.Build.Context, ".")))
	}
	if u.Image == "" && u.Dockerfile == "" {
		return u, fmt.Errorf("%s: neither image nor build.dockerfile given", pfad)
	}
	var befehle []string
	for _, r := range []json.RawMessage{d.OnCreate, d.UpdateContent, d.PostCreate} {
		if b := befehlAus(r); b != "" {
			befehle = append(befehle, b)
		}
	}
	if len(befehle) > 0 {
		u.Einrichten = strings.Join(befehle, "\n")
	}
	return u, nil
}

// befehlAus: Dev-Container-Befehle sind Text, Liste oder Objekt mit benannten Befehlen.
func befehlAus(r json.RawMessage) string {
	if len(r) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(r, &text) == nil {
		return text
	}
	var liste []string
	if json.Unmarshal(r, &liste) == nil {
		return shellListe(liste)
	}
	var objekt map[string]json.RawMessage
	if json.Unmarshal(r, &objekt) == nil {
		namen := make([]string, 0, len(objekt))
		for n := range objekt {
			namen = append(namen, n)
		}
		sort.Strings(namen)
		var teile []string
		for _, n := range namen {
			if b := befehlAus(objekt[n]); b != "" {
				teile = append(teile, b)
			}
		}
		return strings.Join(teile, "\n")
	}
	return ""
}

func shellListe(teile []string) string {
	q := make([]string, len(teile))
	for i, t := range teile {
		q[i] = "'" + strings.ReplaceAll(t, "'", `'\''`) + "'"
	}
	return strings.Join(q, " ")
}

func firstNonEmpty(w ...string) string {
	for _, s := range w {
		if s != "" {
			return s
		}
	}
	return ""
}

// ---------- Erkennung und Vorlagen ----------

func vorlagen() map[string]func(string) Umgebung {
	return map[string]func(string) Umgebung{
		"godot": func(w string) Umgebung { return godot(w, "") }, "node": node, "python": python, "rust": rust,
		"go": golang, "dotnet": dotnet, "java-maven": maven, "java-gradle": gradle, "default": standard,
	}
}

func vorlagenNamen() []string {
	var n []string
	for k := range vorlagen() {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}

func gibt(w string, namen ...string) bool {
	for _, n := range namen {
		if treffer, _ := filepath.Glob(filepath.Join(w, n)); len(treffer) > 0 {
			return true
		}
	}
	return false
}

func erkennen(w string) Umgebung {
	// Godot liegt oft in einem Unterordner (z. B. game/project.godot)
	for _, kandidat := range append([]string{"project.godot"}, glob(w, "*/project.godot")...) {
		if gibt(w, kandidat) {
			return godot(w, filepath.Dir(kandidat))
		}
	}
	switch {
	case gibt(w, "package.json"):
		return node(w)
	case gibt(w, "pyproject.toml", "requirements.txt", "setup.py"):
		return python(w)
	case gibt(w, "Cargo.toml"):
		return rust(w)
	case gibt(w, "go.mod"):
		return golang(w)
	case gibt(w, "*.sln", "*.csproj", "*.fsproj"):
		return dotnet(w)
	case gibt(w, "pom.xml"):
		return maven(w)
	case gibt(w, "build.gradle", "build.gradle.kts"):
		return gradle(w)
	}
	return standard(w)
}

func glob(w, muster string) []string {
	treffer, _ := filepath.Glob(filepath.Join(w, muster))
	var rel []string
	for _, t := range treffer {
		if r, err := filepath.Rel(w, t); err == nil {
			rel = append(rel, filepath.ToSlash(r))
		}
	}
	sort.Strings(rel)
	return rel
}

// Bekannte Patch-Versionen; sonst wird die x.y-Version geladen (Godot veröffentlicht x.y als „x.y-stable“).
var godotVersionen = map[string]string{"4.7": "4.7.2", "4.4": "4.4.1", "4.2": "4.2.2"}

var godotFeature = regexp.MustCompile(`config/features=PackedStringArray\("(\d+\.\d+)"`)

func godot(w, ordner string) Umgebung {
	if ordner == "." {
		ordner = ""
	}
	version := "4.7"
	if roh, err := os.ReadFile(filepath.Join(w, ordner, "project.godot")); err == nil {
		if m := godotFeature.FindSubmatch(roh); m != nil {
			version = string(m[1])
		}
	}
	voll := version
	if v, ok := godotVersionen[version]; ok {
		voll = v
	}
	datei := fmt.Sprintf("Godot_v%s-stable_linux.x86_64", voll)
	basis := "FROM docker.io/library/debian:trixie-slim\n" +
		"RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl unzip git fontconfig libfontconfig1 && rm -rf /var/lib/apt/lists/*\n" +
		fmt.Sprintf("RUN curl -fsSL -o /tmp/godot.zip https://github.com/godotengine/godot/releases/download/%s-stable/%s.zip && unzip -q /tmp/godot.zip -d /opt && rm /tmp/godot.zip && ln -s /opt/%s /usr/local/bin/godot", voll, datei, datei)
	return Umgebung{
		Vorlage: "godot-" + version, Quelle: "detected", Basis: basis, Ordner: ordner,
		Einrichten: "godot --headless --import >/dev/null 2>&1 || true",
		// Jede Testdatei tests/*_test.gd läuft als eigenes Skript; ein Fehler macht die Prüfung rot.
		Testen: `ok=0; n=0; for t in $(find tests -name '*_test.gd' 2>/dev/null | sort); do n=$((n+1)); echo "=== $t"; ` +
			`aus=$(timeout 300 godot --headless --path . -s "res://$t" 2>&1); code=$?; echo "$aus"; ` +
			`if [ $code -ne 0 ] || echo "$aus" | grep -qE ' FAIL |SCRIPT ERROR|Parse Error'; then ok=1; fi; done; ` +
			`[ $n -gt 0 ] || { echo "No tests (tests/*_test.gd) found."; exit 3; }; exit $ok`,
		Hinweis: fmt.Sprintf("Godot %s project, `godot` is installed; tests are tests/*_test.gd and run with `godot --headless -s res://tests/…`.", version),
	}
}

func node(w string) Umgebung {
	einrichten := "npm install --no-audit --no-fund"
	switch {
	case gibt(w, "pnpm-lock.yaml"):
		einrichten = "corepack enable --install-directory /cache/bin && PATH=/cache/bin:$PATH pnpm install --frozen-lockfile"
	case gibt(w, "yarn.lock"):
		einrichten = "corepack enable --install-directory /cache/bin && PATH=/cache/bin:$PATH yarn install --immutable"
	case gibt(w, "package-lock.json", "npm-shrinkwrap.json"):
		einrichten = "npm ci --no-audit --no-fund"
	}
	u := Umgebung{Vorlage: "node", Quelle: "detected", Image: "docker.io/library/node:24-trixie-slim", Einrichten: einrichten,
		Hinweis: "Node.js project (Node 24); dependencies are installed."}
	var p struct {
		Scripts map[string]string `json:"scripts"`
	}
	if roh, err := os.ReadFile(filepath.Join(w, "package.json")); err == nil && json.Unmarshal(roh, &p) == nil {
		if t := p.Scripts["test"]; t != "" && !strings.Contains(t, "no test specified") {
			u.Testen = "npm test"
		}
		if p.Scripts["build"] != "" {
			u.Bauen = "npm run build"
		}
	}
	return u
}

func python(w string) Umgebung {
	einrichten := "python -m venv .venv && . .venv/bin/activate && pip install --upgrade pip"
	switch {
	case gibt(w, "requirements.txt"):
		einrichten += " && pip install -r requirements.txt"
		if gibt(w, "requirements-dev.txt") {
			einrichten += " -r requirements-dev.txt"
		}
	case gibt(w, "pyproject.toml", "setup.py"):
		einrichten += " && (pip install -e '.[test]' || pip install -e '.[dev]' || pip install -e .)"
	}
	einrichten += " && (pip install pytest >/dev/null 2>&1 || true)"
	return Umgebung{Vorlage: "python", Quelle: "detected", Image: "docker.io/library/python:3.13-slim-trixie", Einrichten: einrichten,
		Testen: ". .venv/bin/activate && python -m pytest -q", Hinweis: "Python project (3.13), virtual environment in .venv (`. .venv/bin/activate`)."}
}

func rust(w string) Umgebung {
	return Umgebung{Vorlage: "rust", Quelle: "detected", Image: "docker.io/library/rust:1-slim-trixie", Einrichten: "cargo fetch",
		Bauen: "cargo build --offline", Testen: "cargo test --offline", Hinweis: "Rust project; without network use `--offline`."}
}

func golang(w string) Umgebung {
	return Umgebung{Vorlage: "go", Quelle: "detected", Image: "docker.io/library/golang:1-trixie", Einrichten: "go mod download",
		Bauen: "go build ./...", Testen: "go test ./...", Hinweis: "Go project; modules are downloaded."}
}

func dotnet(w string) Umgebung {
	return Umgebung{Vorlage: "dotnet", Quelle: "detected", Image: "mcr.microsoft.com/dotnet/sdk:9.0", Einrichten: "dotnet restore",
		Bauen: "dotnet build --no-restore", Testen: "dotnet test --no-restore", Hinweis: ".NET project (SDK 9); packages are restored."}
}

func maven(w string) Umgebung {
	return Umgebung{Vorlage: "java-maven", Quelle: "detected", Image: "docker.io/library/maven:3-eclipse-temurin-21", Einrichten: "mvn -q -B dependency:go-offline",
		Testen: "mvn -o -q -B test", Hinweis: "Java project (Maven, JDK 21); without network use `-o`."}
}

func gradle(w string) Umgebung {
	return Umgebung{Vorlage: "java-gradle", Quelle: "detected", Image: "docker.io/library/gradle:8-jdk21", Einrichten: "gradle --no-daemon dependencies -q",
		Testen: "gradle --no-daemon --offline test", Hinweis: "Java project (Gradle, JDK 21); without network use `--offline`."}
}

func standard(w string) Umgebung {
	return Umgebung{Vorlage: "default", Quelle: "default",
		Basis:   "FROM docker.io/library/debian:trixie-slim\nRUN apt-get update && apt-get install -y --no-install-recommends ca-certificates git curl make python3 && rm -rf /var/lib/apt/lists/*",
		Hinweis: "General Linux environment (Debian 13); no automated tests configured."}
}
