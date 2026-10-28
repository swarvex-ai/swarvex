package main

// Installierte proprietäre Engines (Unity, Unreal) auf dem Rechner des Mitstreiters finden und
// nur lesend in die Sandbox einbinden. Die Lizenzdatei kommt ebenfalls nur lesend dazu und
// verlässt den Rechner nie; die Plattform erfährt nur Name und Version (E-31).
//
// Gesucht wird an den üblichen Orten; eigene Pfade über SWARVEX_UNITY bzw. SWARVEX_UNREAL
// (Ordner der Installation, mehrere mit ":" getrennt).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

// installation: eine gefundene Engine.
type installation struct {
	Name    string   // "unity" oder "unreal"
	Version string   // Unity: Editor-Version; Unreal: major.minor.patch
	Wurzel  string   // Installationsordner, wird nach /opt/engine eingebunden
	Binaer  string   // Programm relativ zur Wurzel
	Lizenz  []string // Lizenzdateien bzw. -ordner (Ziel im Container = gleicher Pfad unter /home/agent)
}

func (i installation) String() string { return fmt.Sprintf("%s %s (%s)", i.Name, i.Version, i.Wurzel) }

// enginesFinden: alle Installationen auf diesem Rechner, sortiert nach Name und Version.
func enginesFinden() []installation {
	home, _ := os.UserHomeDir()
	var gefunden []installation
	gesehen := map[string]bool{}
	dazu := func(i installation, ok bool) {
		if ok && !gesehen[i.Wurzel] {
			gesehen[i.Wurzel] = true
			gefunden = append(gefunden, i)
		}
	}
	for _, w := range unityOrte(home) {
		dazu(unityInstallation(home, w))
	}
	for _, w := range unrealOrte(home) {
		dazu(unrealInstallation(w))
	}
	sort.Slice(gefunden, func(a, b int) bool {
		if gefunden[a].Name != gefunden[b].Name {
			return gefunden[a].Name < gefunden[b].Name
		}
		return gefunden[a].Version < gefunden[b].Version
	})
	return gefunden
}

// engineFuer: die passende Installation für ein Projekt, sonst ein Fehler mit klarer Anleitung.
func engineFuer(u umgebung.Umgebung, alle []installation) (installation, error) {
	if u.Engine == nil {
		return installation{}, nil
	}
	for i := len(alle) - 1; i >= 0; i-- { // neueste passende zuerst
		if u.Engine.Passt(alle[i].Name, alle[i].Version) {
			return alle[i], nil
		}
	}
	wie := map[string]string{
		"unity":  "install it with the Unity Hub (a free Personal license is enough) or set SWARVEX_UNITY to the editor folder",
		"unreal": "install it (Epic Games launcher or the Linux build) or set SWARVEX_UNREAL to the engine folder",
	}[u.Engine.Name]
	return installation{}, fmt.Errorf("this project needs %s, which is not installed on this machine: %s", u.Engine, wie)
}

// engineArgs: Podman-Argumente, die Engine und Lizenz nur lesend einbinden.
func engineArgs(i installation) []string {
	if i.Wurzel == "" {
		return nil
	}
	args := []string{"-v", i.Wurzel + ":" + umgebung.EngineMount + ":ro",
		"-e", umgebung.EngineBinaerVariable + "=" + filepath.Join(umgebung.EngineMount, i.Binaer),
		"-e", umgebung.EngineWurzelVariable + "=" + umgebung.EngineMount}
	home, _ := os.UserHomeDir()
	for _, l := range i.Lizenz {
		rel, err := filepath.Rel(home, l)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		args = append(args, "-v", l+":"+filepath.Join("/home/agent", rel)+":ro")
	}
	if i.Name == "unreal" {
		// Die Engine ist schreibgeschützt: abgeleitete Daten in den Projekt-Cache statt in den Engine-Ordner.
		args = append(args, "-e", "UE-LocalDataCachePath=/cache/ue-ddc", "-e", "UE-SharedDataCachePath=None")
	}
	return args
}

// ---------- Unity ----------

func unityOrte(home string) []string {
	var orte []string
	orte = append(orte, strings.FieldsFunc(os.Getenv("SWARVEX_UNITY"), func(r rune) bool { return r == ':' })...)
	hubOrdner := []string{filepath.Join(home, "Unity", "Hub", "Editor")}
	// Eigener Installationsort im Unity Hub (JSON-Text mit dem Pfad).
	if roh, err := os.ReadFile(filepath.Join(home, ".config", "UnityHub", "secondaryInstallPath.json")); err == nil {
		var pfad string
		if json.Unmarshal(roh, &pfad) == nil && pfad != "" {
			hubOrdner = append(hubOrdner, pfad)
		}
	}
	for _, h := range hubOrdner {
		eintraege, _ := os.ReadDir(h)
		for _, e := range eintraege {
			if e.IsDir() {
				orte = append(orte, filepath.Join(h, e.Name()))
			}
		}
	}
	return orte
}

func unityInstallation(home, wurzel string) (installation, bool) {
	wurzel = strings.TrimSuffix(filepath.Clean(wurzel), "/Editor")
	if _, err := os.Stat(filepath.Join(wurzel, "Editor", "Unity")); err != nil {
		return installation{}, false
	}
	i := installation{Name: "unity", Version: filepath.Base(wurzel), Wurzel: wurzel, Binaer: "Editor/Unity"}
	// Lizenz: klassische Datei (.ulf) und die Ordner des Lizenz-Clients (Unity 2021 und neuer).
	for _, l := range []string{
		filepath.Join(home, ".local", "share", "unity3d", "Unity", "Unity_lic.ulf"),
		filepath.Join(home, ".config", "unity3d", "Unity", "licenses"),
		filepath.Join(home, ".local", "share", "unity3d", "Unity", "licenses"),
	} {
		if _, err := os.Stat(l); err == nil {
			i.Lizenz = append(i.Lizenz, l)
		}
	}
	return i, true
}

// ---------- Unreal ----------

func unrealOrte(home string) []string {
	var orte []string
	orte = append(orte, strings.FieldsFunc(os.Getenv("SWARVEX_UNREAL"), func(r rune) bool { return r == ':' })...)
	// Vom Editor registrierte Installationen: [Installations] GUID=/pfad
	if roh, err := os.ReadFile(filepath.Join(home, ".config", "Epic", "UnrealEngine", "Install.ini")); err == nil {
		for _, z := range strings.Split(string(roh), "\n") {
			if _, pfad, ok := strings.Cut(strings.TrimSpace(z), "="); ok && strings.HasPrefix(pfad, "/") {
				orte = append(orte, pfad)
			}
		}
	}
	for _, muster := range []string{filepath.Join(home, "UnrealEngine*"), filepath.Join(home, "Linux_Unreal_Engine_*"), "/opt/UnrealEngine*", "/opt/unreal*"} {
		treffer, _ := filepath.Glob(muster)
		orte = append(orte, treffer...)
	}
	return orte
}

var unrealBuildVersion = regexp.MustCompile(`"(Major|Minor|Patch)Version"\s*:\s*(\d+)`)

func unrealInstallation(wurzel string) (installation, bool) {
	wurzel = filepath.Clean(wurzel)
	binaer := "Engine/Binaries/Linux/UnrealEditor-Cmd"
	if _, err := os.Stat(filepath.Join(wurzel, binaer)); err != nil {
		return installation{}, false
	}
	version := ""
	if roh, err := os.ReadFile(filepath.Join(wurzel, "Engine", "Build", "Build.version")); err == nil {
		teile := map[string]string{}
		for _, m := range unrealBuildVersion.FindAllStringSubmatch(string(roh), -1) {
			teile[m[1]] = m[2]
		}
		if teile["Major"] != "" {
			version = teile["Major"] + "." + teile["Minor"] + "." + teile["Patch"]
		}
	}
	return installation{Name: "unreal", Version: version, Wurzel: wurzel, Binaer: binaer}, true
}
