package umgebung

// Proprietäre Engines (Unity, Unreal): Sie dürfen nicht in Container-Images, und die Prüfstraße
// hat keine Lizenz dafür. Deshalb bringt jeder Mitstreiter seine eigene, installierte und
// lizenzierte Engine mit. Der Runner bindet sie nur lesend in die Sandbox ein (/opt/engine),
// die Lizenzdatei ebenso; beides verlässt den Rechner nie (E-31).
//
// Hier steht nur, was das Projekt verlangt und wie getestet wird. Wo die Engine auf dem
// Rechner liegt, findet der Runner selbst (runner/engines.go).

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Engine: eine proprietäre Engine, die das Projekt verlangt.
type Engine struct {
	Name    string // "unity" oder "unreal"
	Version string // Unity: exakte Editor-Version (z. B. 6000.0.32f1); Unreal: major.minor (z. B. 5.5)
}

func (e Engine) String() string { return e.Name + " " + e.Version }

// Passt: Unity braucht genau dieselbe Editor-Version, Unreal dieselbe major.minor-Version.
func (e Engine) Passt(name, version string) bool {
	if e.Name != name {
		return false
	}
	if e.Version == "" {
		return true
	}
	if name == "unreal" {
		return majorMinor(version) == majorMinor(e.Version)
	}
	return version == e.Version
}

func majorMinor(v string) string {
	teile := strings.Split(v, ".")
	if len(teile) >= 2 {
		return teile[0] + "." + teile[1]
	}
	return v
}

// IstProprietaer: Name einer Engine, die nur beim Mitstreiter laufen kann (Projektfeld engine, Vorlage).
func IstProprietaer(engine string) bool {
	e := strings.ToLower(engine)
	return strings.Contains(e, "unity") || strings.Contains(e, "unreal")
}

// EngineBinaerVariable: Pfad der Engine im Container; Testbefehle rufen "$SWARVEX_ENGINE" auf.
// EngineWurzelVariable: Installationsordner (/opt/engine), z. B. für Unreals Build-Skripte.
const (
	EngineBinaerVariable = "SWARVEX_ENGINE"
	EngineWurzelVariable = "SWARVEX_ENGINE_ROOT"
	EngineMount          = "/opt/engine"
)

// Basis für Engine-Projekte: schlankes Debian mit den Bibliotheken, die Unity im Batchmode und
// Unreal mit -nullrhi brauchen. Die Engine selbst kommt erst beim Start als Mount dazu.
const engineBasis = "FROM docker.io/library/debian:trixie-slim\n" +
	"RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates git curl python3 xz-utils " +
	"libglib2.0-0 libgtk-3-0 libnss3 libgbm1 libasound2 libxcursor1 libxrandr2 libxi6 libxinerama1 libxss1 libglu1-mesa " +
	"libvulkan1 libsdl2-2.0-0 libicu76 libssl3 libcurl4 && rm -rf /var/lib/apt/lists/*\n"

var unityVersion = regexp.MustCompile(`(?m)^m_EditorVersion:\s*(\S+)`)

// unityProjekt: ProjectSettings/ProjectVersion.txt im Wurzel- oder einem Unterordner.
func unityProjekt(w string) (string, bool) {
	for _, kandidat := range append([]string{"ProjectSettings/ProjectVersion.txt"}, glob(w, "*/ProjectSettings/ProjectVersion.txt")...) {
		if gibt(w, kandidat) {
			return filepath.Dir(filepath.Dir(kandidat)), true
		}
	}
	return "", false
}

func unity(w, ordner string) Umgebung {
	if ordner == "." {
		ordner = ""
	}
	version := ""
	if roh, err := os.ReadFile(filepath.Join(w, ordner, "ProjectSettings", "ProjectVersion.txt")); err == nil {
		if m := unityVersion.FindSubmatch(roh); m != nil {
			version = string(m[1])
		}
	}
	// EditMode- und PlayMode-Tests des Unity Test Framework; Ergebnis als NUnit-XML.
	testen := `"$` + EngineBinaerVariable + `" -batchmode -nographics -projectPath . -runTests -testPlatform EditMode ` +
		`-testResults /tmp/editmode.xml -logFile - ; code=$?; ` +
		`grep -q 'result="Passed"' /tmp/editmode.xml 2>/dev/null || code=1; exit $code`
	return Umgebung{
		Vorlage: "unity", Quelle: "detected", Basis: engineBasis, Ordner: ordner,
		Engine: &Engine{Name: "unity", Version: version},
		// Erster Import (Library/) mit Netz für Pakete aus dem Unity-Register.
		Einrichten: `"$` + EngineBinaerVariable + `" -batchmode -nographics -projectPath . -quit -logFile - >/tmp/import.log 2>&1 || (tail -40 /tmp/import.log; exit 1)`,
		Testen:     testen,
		Hinweis: fmt.Sprintf("Unity %s project. The editor is at $%s (batchmode only, no graphics); tests use the Unity Test Framework (EditMode).",
			version, EngineBinaerVariable),
	}
}

// unrealProjekt: genau eine *.uproject-Datei im Wurzel- oder einem Unterordner.
func unrealProjekt(w string) (string, bool) {
	treffer := append(glob(w, "*.uproject"), glob(w, "*/*.uproject")...)
	if len(treffer) == 0 {
		return "", false
	}
	return treffer[0], true
}

var unrealVersion = regexp.MustCompile(`"EngineAssociation"\s*:\s*"([^"]*)"`)

func unreal(w, uproject string) Umgebung {
	ordner := filepath.Dir(uproject)
	if ordner == "." {
		ordner = ""
	}
	name := strings.TrimSuffix(filepath.Base(uproject), ".uproject")
	version := ""
	if roh, err := os.ReadFile(filepath.Join(w, uproject)); err == nil {
		if m := unrealVersion.FindSubmatch(roh); m != nil && regexp.MustCompile(`^\d+\.\d+`).MatchString(string(m[1])) {
			version = majorMinor(string(m[1]))
		}
	}
	// Automation-Tests des Projekts (Namen beginnen mit dem Projektnamen), ohne Grafik.
	testen := fmt.Sprintf(`"$%s" "$PWD/%s.uproject" -ExecCmds="Automation RunTests %s.;Quit" -unattended -nullrhi -nosplash -nosound -stdout -FullStdOutLogOutput -log > /tmp/test.log 2>&1; code=$?; `+
		`tail -60 /tmp/test.log; grep -qE 'Test Completed\. Result=\{Fail' /tmp/test.log && code=1; `+
		`grep -q 'Test Completed. Result={Success}' /tmp/test.log || code=1; exit $code`, EngineBinaerVariable, name, name)
	bauen := ""
	if gibt(w, filepath.Join(ordner, "Source")) {
		// C++-Projekt: Editor-Module mit dem Unreal Build Tool der Engine bauen (ohne Netz).
		bauen = fmt.Sprintf(`"$%s/Engine/Build/BatchFiles/Linux/Build.sh" %sEditor Linux Development -Project="$PWD/%s.uproject" -waitmutex`,
			EngineWurzelVariable, name, name)
	}
	return Umgebung{
		Vorlage: "unreal", Quelle: "detected", Basis: engineBasis, Ordner: ordner,
		Engine: &Engine{Name: "unreal", Version: version},
		Bauen:  bauen,
		Testen: testen,
		Hinweis: fmt.Sprintf("Unreal Engine %s project %s. The editor command line is at $%s (UnrealEditor-Cmd, -nullrhi); tests are Automation tests named %s.*.",
			version, name, EngineBinaerVariable, name),
	}
}
