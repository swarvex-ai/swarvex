package main

// Medien-Aufträge (Zusage über den Runner): Der Lead oder ein Bereichs-Lead bestellt ein Asset (3D-Modell, Bild,
// Klang, Video) bei den Medien-Werkzeugen, die ein Mitstreiter in seinen Agenten eingetragen hat (werkzeuge.go).
//
//	Auftrag beginnen (zählt gegen das Tageslimit) → Werkzeug-Container je Server (Schlüssel darin, schreibt nur nach
//	/out, ins Internet nur über den Ausgangs-Proxy) → Agent in der Sandbox (Modell über den Schlüssel-Proxy, das
//	Werkzeug über die Brücke mit Wegwerf-Token) → Datei aus /out einsammeln, Typ, Inhalt und Größe prüfen → hochladen.
//
// Scheitert etwas, gibt der Runner den Auftrag zurück; gezählt bleibt er, damit ein Fehler keine Schleife wird.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

// medienBrief: ein Auftrag aus media_briefs (apps/api/src/spenden.ts, runnerAuftraege).
type medienBrief struct {
	ID          int64    `json:"brief_id"`
	Projekt     string   `json:"project"`
	Dienst      string   `json:"service"`
	Titel       string   `json:"title"`
	Anleitung   string   `json:"instructions"`
	Zielpfad    string   `json:"target_path"`
	Dateitypen  []string `json:"file_types"`
	MaxMB       int      `json:"max_mb"`
	Zustand     string   `json:"state"`
	Begruendung string   `json:"reason"`
}

// medienTypen: was über den Runner hochgeladen werden darf (wie RUNNER_TYPEN in apps/api/src/medien.ts).
var medienTypen = []string{"glb", "fbx", "obj", "png", "jpg", "jpeg", "webp", "mp4", "webm", "mov", "wav", "mp3", "ogg", "flac"}

// MedienStandard: Aufträge je Tag, wenn nichts anderes eingestellt ist; höchstens medienHoechstens (wie die Plattform).
const (
	medienStandard   = 10
	medienHoechstens = 30
	werkzeugImage    = "swarvex-tool"
	medienBasis      = werkzeugImage // die Agent-Schicht der Medien-Sandbox liegt auf dem Werkzeug-Image (glibc, sonst nichts Nötiges)
)

// Für den Durchlauf-Test (medien_e2e_test.go): ein anderer Befehl statt des Agenten, ein anderes Programm für die Brücke.
var (
	medienAgentBefehl []string
	eigenesProgramm   = os.Executable
)

// medienZusagenAbgleichen: die Zusagen über den Runner auf der Plattform an die nutzbaren Server anpassen. Nur wenn
// sich etwas geändert hat (gemerkt in media-pledges.json), damit nicht jede Runde ein Ereignis erzeugt.
func medienZusagenAbgleichen(m *mcpClient, server []medienServer, tag int, projekt string) {
	dienste := []string{}
	if tag > 0 {
		gesehen := map[string]bool{}
		for _, s := range server {
			if s.Fehlt == "" && !gesehen[s.Dienst] {
				gesehen[s.Dienst] = true
				dienste = append(dienste, s.Dienst)
			}
		}
		sort.Strings(dienste)
	}
	if projekt == "auto" {
		projekt = ""
	}
	stand, _ := json.Marshal(map[string]any{"services": dienste, "daily": tag, "project": projekt, "platform": m.basis})
	datei := filepath.Join(datenOrdner(), "media-pledges.json")
	if lesen(datei) == string(stand) {
		return
	}
	if len(dienste) == 0 && lesen(datei) == "" {
		return // nie eine Zusage gegeben: nichts zu beenden
	}
	argumente := map[string]any{"services": dienste}
	if tag > 0 {
		argumente["daily_limit"] = min(tag, medienHoechstens)
	}
	if projekt != "" {
		argumente["project"] = projekt
	}
	if err := m.werkzeug("set_media_pledges", argumente, nil); err != nil {
		fmt.Println("Media pledges not updated:", err)
		return
	}
	os.MkdirAll(filepath.Dir(datei), 0o755)
	os.WriteFile(datei, stand, 0o644)
	if len(dienste) == 0 {
		fmt.Println("Media pledges ended.")
	} else {
		fmt.Printf("Media pledges: %s, at most %d briefs a day.\n", strings.Join(dienste, ", "), min(tag, medienHoechstens))
	}
}

type medienEinstellung struct {
	server     []medienServer // ausgewählt und gerade in den Konfigurationen gefunden
	agent      agentArt
	modell     string
	upstream   string
	keyDatei   string
	budget     int
	zeit       time.Duration
	skills     []string
	maxAufrufe int
}

// medienRunde: wartet ein Medien-Auftrag, den einer der Server erledigen kann, kommt er vor den Code-Aufgaben.
// true: ein Auftrag wurde bearbeitet (dann endet dieser Aufruf).
func medienRunde(basis context.Context, m *mcpClient, z *zugangsdaten, e medienEinstellung) bool {
	var liste struct {
		Tageslimit int           `json:"daily_limit"`
		Heute      int           `json:"today"`
		Briefs     []medienBrief `json:"briefs"`
	}
	if err := m.werkzeug("media_briefs", map[string]any{}, &liste); err != nil {
		fmt.Println("Media briefs not readable:", err)
		return false
	}
	var b *medienBrief
	var server []medienServer
	for i := range liste.Briefs {
		for _, s := range e.server {
			if s.Dienst == liste.Briefs[i].Dienst && s.Fehlt == "" {
				server = append(server, s)
			}
		}
		if len(server) > 0 {
			b = &liste.Briefs[i]
			break
		}
	}
	if b == nil || basis.Err() != nil {
		return false
	}
	if err := m.werkzeug("start_media_brief", map[string]any{"brief_id": b.ID}, nil); err != nil {
		fmt.Println("Media brief not started:", err)
		return false
	}
	fmt.Printf("Media brief %d started (%s, %s): %s\n", b.ID, dienstTitel(b.Dienst), b.Projekt, b.Titel)
	var verbrauch map[string]float64
	eintrag := func(ergebnis, grund string) {
		verlaufMerken(verlaufsDatei(), verlaufEintrag{Zeit: time.Now(), Brief: b.ID, Projekt: b.Projekt, Titel: b.Titel,
			Ergebnis: ergebnis, Grund: kuerzen(grund, 400), Tokens: tokensAus(verbrauch), Budget: budgetTokensAus(verbrauch), Sekunden: int64(verbrauch["seconds"])})
	}
	// Zurückgeben: Nach einem Fehlschlag pausiert die Plattform den Auftrag 24 Stunden (sonst verbrauchte ein Auftrag, der
	// immer scheitert, bis zum Tageslimit Credits); nur angehalten (gestoppt) ist er gleich wieder frei.
	zurueck := func(grund string, gestoppt bool) {
		if neu, err := zugriffstoken(z); err == nil {
			m.token = neu
		}
		m.werkzeug("abandon_media_brief", map[string]any{"brief_id": b.ID, "reason": kuerzen(grund, 480), "stopped": gestoppt}, nil)
		eintrag("returned", grund)
		if !gestoppt {
			grund += " (the brief pauses for 24 hours; the lead sees the reason)"
		}
		fehlerAus("Media brief returned: " + grund)
	}
	abbrechen := func(grund string) { zurueck(grund, false) }
	ordner := filepath.Join(datenOrdner(), "media", fmt.Sprint(b.ID))
	os.RemoveAll(ordner)
	aus := filepath.Join(ordner, "out")
	arbeit := filepath.Join(ordner, "work")
	for _, d := range []string{aus, arbeit, filepath.Join(ordner, "cache")} {
		if err := os.MkdirAll(d, 0o755); err != nil { // Container laufen mit keep-id als dieser Benutzer
			abbrechen(err.Error())
		}
	}
	ctx, stopp := context.WithTimeout(basis, e.zeit)
	defer stopp()
	var err error
	verbrauch, _, err = medienSandbox(ctx, *b, server, e, ordner, aus, arbeit)
	verbrauchMerken(verbrauchsDatei(), -b.ID, budgetTokensAus(verbrauch), time.Now())
	if basis.Err() != nil {
		zurueck("runner was stopped", true)
	}
	if err != nil {
		abbrechen("sandbox: " + err.Error())
	}
	datei, grund := medienEinsammeln(aus, arbeit, *b)
	if datei == "" {
		abbrechen(grund)
	}
	fmt.Printf("Uploading %s (%s)\n", filepath.Base(datei), mbText(datei))
	if neu, err := zugriffstoken(z); err == nil {
		m.token = neu
	}
	if err := medienHochladen(z, m.token, b.ID, datei); err != nil {
		abbrechen("upload: " + err.Error())
	}
	fmt.Println("Media brief uploaded; the lead reviews it.")
	eintrag("uploaded", "")
	os.RemoveAll(aus)
	os.RemoveAll(arbeit)
	return true
}

// medienSandbox: Ausgangs-Proxy, Werkzeug-Container, Schlüssel-Proxy und die Sandbox mit dem Agenten.
func medienSandbox(ctx context.Context, b medienBrief, server []medienServer, e medienEinstellung, ordner, aus, arbeit string) (map[string]float64, string, error) {
	netz, err := internesNetz(ctx)
	if err != nil {
		return nil, "", err
	}
	if err := werkzeugImageBereit(ctx); err != nil {
		return nil, "", err
	}
	if err := agentImageBereit(ctx, e.agent); err != nil {
		return nil, "", err
	}
	image, err := medienSandboxImage(ctx, e.agent)
	if err != nil {
		return nil, "", err
	}
	// Das Programm selbst bringt die Brücke in den Werkzeug-Container (statisch gebaut, läuft in jedem Image).
	programm, err := eigenesProgramm()
	if err != nil {
		return nil, "", err
	}
	kopie := filepath.Join(ordner, "bin", "swarvex")
	os.MkdirAll(filepath.Dir(kopie), 0o755)
	if err := dateiKopieren(programm, kopie, 0o755); err != nil {
		return nil, "", err
	}
	sekunden := 3600
	if frist, ok := ctx.Deadline(); ok {
		sekunden = max(int(time.Until(frist).Seconds()), 1)
	}
	ausgangName := fmt.Sprintf("swarvex-egress-m%d", b.ID)
	exec.Command("podman", "rm", "-f", ausgangName).Run()
	ausgang, err := ausgangsProxyStarten(ctx, ausgangName, netz, "public", "", sekunden+300)
	defer func() { // gesperrte Ziele bleiben im Protokoll sichtbar
		if log, err := exec.Command("podman", "logs", ausgangName).CombinedOutput(); err == nil {
			os.WriteFile(filepath.Join(ordner, "egress.log"), log, 0o644)
		}
		exec.Command("podman", "rm", "-f", ausgangName).Run()
	}()
	if err != nil {
		return nil, "", fmt.Errorf("egress proxy for the media tools: %v", err)
	}
	var werkzeuge []sandboxWerkzeug
	var hosts []string
	for i, s := range server {
		name := fmt.Sprintf("swarvex-tool-%d-%d", b.ID, i+1)
		w, h, err := werkzeugStarten(ctx, name, netz, s, kopie, aus, ausgang, e.maxAufrufe, sekunden+300)
		defer func(name string) {
			if log, err := exec.Command("podman", "logs", name).CombinedOutput(); err == nil {
				os.WriteFile(filepath.Join(ordner, name+".log"), log, 0o644)
			}
			exec.Command("podman", "rm", "-f", name).Run()
		}(name)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %v", s.Titel(), err)
		}
		if i > 0 {
			w.Name = fmt.Sprintf("%s-%d", s.Dienst, i+1)
		}
		werkzeuge, hosts = append(werkzeuge, w), append(hosts, h...)
	}

	proxyName := fmt.Sprintf("swarvex-proxy-m%d", b.ID)
	zugang, err := zugangLesen(e.agent, e.keyDatei, e.upstream, e.zeit)
	if err != nil {
		return nil, "", err
	}
	tok, proxyHosts, err := schluesselProxyStarten(ctx, proxyName, netz, e.agent, zugang, e.modell, e.budget)
	defer exec.Command("podman", "rm", "-f", proxyName).Run()
	if err != nil {
		return nil, "", err
	}
	auftrag := medienAuftragstext(b, server, e.maxAufrufe)
	os.WriteFile(filepath.Join(arbeit, "BRIEF.md"), []byte(auftrag), 0o644)
	agentArgs, befehl, err := agentStart(e.agent, ordner, "http://"+proxyName+":8080", tok, e.modell, auftrag, e.skills, arbeit, "", zugang)
	if err != nil {
		return nil, "", err
	}
	agentArgs, befehl, err = agentWerkzeuge(e.agent, ordner, werkzeuge, agentArgs, befehl)
	if err != nil {
		return nil, "", err
	}
	sandboxName := fmt.Sprintf("swarvex-sandbox-m%d", b.ID)
	exec.Command("podman", "rm", "-f", sandboxName).Run()
	args := append(containerArgs(sandboxName, arbeit, filepath.Join(ordner, "cache"), umgebung.Umgebung{}, sekunden),
		"-v", aus+":/out:ro,z", "--network", netz)
	args = append(append(append(args, proxyHosts...), hosts...), agentArgs...)
	if medienAgentBefehl != nil {
		befehl = medienAgentBefehl
	}
	args = append(append(args, image), befehl...)
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Cancel = func() error {
		exec.Command("podman", "kill", sandboxName).Run()
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 10 * time.Second
	ausgabe, err := cmd.CombinedOutput()
	exec.Command("podman", "rm", "-f", sandboxName).Run()
	os.WriteFile(filepath.Join(ordner, "agent.log"), ausgabe, 0o644)
	verbrauch := proxyAuswerten(proxyName, ordner)
	if ctx.Err() != nil {
		return verbrauch, string(ausgabe), fmt.Errorf("time limit reached")
	}
	if err != nil {
		return verbrauch, string(ausgabe), fmt.Errorf("agent exited with an error: %v", err)
	}
	return verbrauch, string(ausgabe), nil
}

// sandboxWerkzeug: ein Werkzeug-Container, wie der Agent ihn sieht (Name, Adresse, Wegwerf-Token).
type sandboxWerkzeug struct{ Name, URL, Token string }

// werkzeugStarten: ein Werkzeug-Container für einen Server. Der Schlüssel steht nur in seiner Umgebung; schreiben
// darf er nur nach /out (und in seinen eigenen Paket-Cache), ins Internet nur über den Ausgangs-Proxy.
func werkzeugStarten(ctx context.Context, name, netz string, s medienServer, programm, aus string, ausgang []string, maxAufrufe, sekunden int) (sandboxWerkzeug, []string, error) {
	tok := zufall(36)
	cache := filepath.Join(datenOrdner(), "tool-cache", regexp.MustCompile(`[^A-Za-z0-9_-]`).ReplaceAllString(s.Kennung(), "_"))
	os.MkdirAll(cache, 0o755)
	args := []string{"run", "-d", "--name", name, "--network", netz, "--timeout", fmt.Sprint(sekunden),
		"--read-only", "--cap-drop=ALL", "--security-opt", "no-new-privileges", "--memory", "1g", "--pids-limit", "256",
		"--userns=keep-id:uid=1000,gid=1000", "--user", "1000:1000",
		"--tmpfs", "/tmp:rw,exec,size=1g,mode=1777", "--tmpfs", "/home/tool:rw,exec,size=1g,mode=1777",
		"-v", cache + ":/home/tool/.cache:Z", "-v", aus + ":/out:z", "-v", programm + ":/opt/swarvex/swarvex:ro,Z", "-w", "/out",
		"-e", "HOME=/home/tool", "-e", "npm_config_cache=/home/tool/.cache/npm", "-e", "UV_CACHE_DIR=/home/tool/.cache/uv",
		"-e", "NODE_USE_ENV_PROXY=1", "-e", "NPM_CONFIG_UPDATE_NOTIFIER=false",
		"-e", "SWARVEX_TOOL_TOKEN=" + tok, "-e", fmt.Sprintf("SWARVEX_TOOL_MAX_CALLS=%d", maxAufrufe)}
	args = append(args, ausgang...)
	if len(s.Befehl) > 0 {
		roh, _ := json.Marshal(s.Befehl)
		args = append(args, "-e", "SWARVEX_TOOL_COMMAND="+string(roh))
		for k, v := range s.Env {
			args = append(args, "-e", k+"="+v)
		}
	} else {
		roh, _ := json.Marshal(s.Kopf)
		args = append(args, "-e", "SWARVEX_TOOL_URL="+s.URL, "-e", "SWARVEX_TOOL_HEADERS="+string(roh))
	}
	args = append(args, werkzeugImage, "/opt/swarvex/swarvex", "proxy", "tool")
	exec.Command("podman", "rm", "-f", name).Run()
	if aus, err := exec.CommandContext(ctx, "podman", args...).CombinedOutput(); err != nil {
		return sandboxWerkzeug{}, nil, fmt.Errorf("starting the tool container: %s", strings.TrimSpace(string(aus)))
	}
	// Bereit, sobald die Brücke lauscht (den Server hat sie dann schon gestartet)
	for i := 0; i < 40; i++ {
		log, _ := exec.Command("podman", "logs", name).CombinedOutput()
		if bytes.Contains(log, []byte("tool: listening")) {
			break
		}
		if lauf, _ := exec.Command("podman", "inspect", "-f", "{{.State.Running}}", name).Output(); strings.TrimSpace(string(lauf)) == "false" {
			return sandboxWerkzeug{}, nil, fmt.Errorf("the tool container stopped: %s", letzteZeilen(string(log), 8))
		}
		time.Sleep(500 * time.Millisecond)
	}
	hosts, err := proxyHost(ctx, name)
	if err != nil {
		return sandboxWerkzeug{}, nil, err
	}
	// Name, wie der Agent ihn sieht: der Dienst (meshy, elevenlabs …); bei mehreren Servern nummeriert der Aufrufer
	return sandboxWerkzeug{Name: s.Dienst, URL: "http://" + name + ":8090/mcp", Token: tok}, hosts, nil
}

// werkzeugImageBereit baut das Werkzeug-Image (Node und uv für npx/uvx-Server) beim ersten Medien-Auftrag.
func werkzeugImageBereit(ctx context.Context) error {
	if _, err := podmanAus(ctx, "", "image", "exists", werkzeugImage); err == nil {
		return nil
	}
	kontext := filepath.Join(datenOrdner(), "release", "sandbox")
	if _, err := os.Stat(filepath.Join(kontext, "Containerfile")); err != nil {
		return fmt.Errorf("%s/Containerfile not found; reinstall the runner (install.sh)", kontext)
	}
	fmt.Println("Preparing the container for media tools (only the first time, takes a few minutes)")
	if aus, err := podmanAus(ctx, "", "build", "-q", "--target", "werkzeug", "-t", werkzeugImage, kontext); err != nil {
		return fmt.Errorf("building the media tool container: %s", letzteZeilen(aus, 20))
	}
	return nil
}

// medienSandboxImage: die Agent-Schicht auf dem Werkzeug-Image (einmal je Agent-Version).
func medienSandboxImage(ctx context.Context, a agentArt) (string, error) {
	id, err := podmanAus(ctx, "", "image", "inspect", "--format", "{{.Id}}", a.Image)
	if err != nil {
		return "", fmt.Errorf("agent image %s is missing; reinstall the runner (install.sh)", a.Image)
	}
	basis, err := podmanAus(ctx, "", "image", "inspect", "--format", "{{.Id}}", medienBasis)
	if err != nil {
		return "", err
	}
	name := "swarvex-media-" + a.Name + "-" + kurzKennung(id+basis)
	if _, err := podmanAus(ctx, "", "image", "exists", name); err == nil {
		return name, nil
	}
	leer, err := os.MkdirTemp("", "swarvex-build-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(leer)
	if aus, err := podmanAus(ctx, umgebung.Umgebung{Image: medienBasis}.Containerfile("", a.Image), "build", "-q", "-t", name, "-f", "-", leer); err != nil {
		return "", fmt.Errorf("building the media sandbox: %s", letzteZeilen(aus, 20))
	}
	return name, nil
}

func kurzKennung(s string) string {
	s = strings.ReplaceAll(s, "sha256:", "")
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// agentWerkzeuge trägt die Werkzeug-Container als MCP-Server beim Agenten ein (lange Zeitgrenzen: Erzeugen dauert).
func agentWerkzeuge(a agentArt, ordner string, werkzeuge []sandboxWerkzeug, args, befehl []string) ([]string, []string, error) {
	if len(werkzeuge) == 0 {
		return args, befehl, nil
	}
	auftrag := befehl[len(befehl)-1]
	vorne := befehl[:len(befehl)-1]
	switch a.Name {
	case "opencode":
		pfad := filepath.Join(ordner, "opencode.json")
		var konfig map[string]any
		if err := json.Unmarshal([]byte(lesen(pfad)), &konfig); err != nil {
			return nil, nil, fmt.Errorf("opencode config: %v", err)
		}
		mcp := map[string]any{}
		for _, w := range werkzeuge {
			mcp[w.Name] = map[string]any{"type": "remote", "url": w.URL, "enabled": true, "timeout": 120000,
				"headers": map[string]any{"Authorization": "Bearer " + w.Token}}
		}
		konfig["mcp"] = mcp
		if _, err := konfigSchreiben(ordner, "opencode.json", konfig); err != nil {
			return nil, nil, err
		}
		return args, befehl, nil
	case "claude":
		server := map[string]any{}
		for _, w := range werkzeuge {
			server[w.Name] = map[string]any{"type": "http", "url": w.URL, "headers": map[string]any{"Authorization": "Bearer " + w.Token}}
		}
		pfad, err := konfigSchreiben(ordner, "mcp.json", map[string]any{"mcpServers": server})
		if err != nil {
			return nil, nil, err
		}
		args = append(args, "-v", pfad+":/konfig/mcp.json:ro,Z", "-e", "MCP_TIMEOUT=120000", "-e", "MCP_TOOL_TIMEOUT=2700000")
		return args, append(append(append([]string{}, vorne...), "--mcp-config", "/konfig/mcp.json", "--strict-mcp-config"), auftrag), nil
	case "codex":
		neu := append([]string{}, vorne...)
		for i, w := range werkzeuge {
			variable := fmt.Sprintf("SWARVEX_TOOL_TOKEN_%d", i+1)
			args = append(args, "-e", variable+"="+w.Token)
			neu = append(neu, "-c", "mcp_servers."+w.Name+".url="+tomlText(w.URL),
				"-c", "mcp_servers."+w.Name+".bearer_token_env_var="+tomlText(variable),
				"-c", "mcp_servers."+w.Name+".startup_timeout_sec=120", "-c", "mcp_servers."+w.Name+".tool_timeout_sec=2700")
		}
		return args, append(neu, auftrag), nil
	}
	return nil, nil, fmt.Errorf("media tools are not available for the %s agent", a.Name)
}

// medienAuftragstext: der Auftrag an den Agenten in der Medien-Sandbox.
func medienAuftragstext(b medienBrief, server []medienServer, maxAufrufe int) string {
	var t strings.Builder
	typen := medienErlaubt(b)
	ziel := filepath.Base(b.Zielpfad)
	if ziel == "." || ziel == "/" || ziel == "" {
		ziel = "asset." + typen[0]
	}
	if !istTyp(ziel, typen) {
		ziel = strings.TrimSuffix(ziel, filepath.Ext(ziel)) + "." + typen[0]
	}
	namen := make([]string, len(server))
	for i, s := range server {
		namen[i] = s.Dienst
	}
	fmt.Fprintf(&t, "You create a media asset for the open-source project %q on swarvex. There is no internet here. For this brief you have the MCP tools of %s (server %s); use them to create the asset.\n\n",
		b.Projekt, dienstTitel(b.Dienst), strings.Join(namen, ", "))
	fmt.Fprintf(&t, "BRIEF: %s\n\n%s\n\n", b.Titel, strings.TrimSpace(b.Anleitung))
	fmt.Fprintf(&t, "DELIVERABLE: one file of type %s, at most %d MB. In the project it becomes %s.\n", strings.Join(punkte(typen), " or "), b.MaxMB, b.Zielpfad)
	if b.Zustand == "rework" && strings.TrimSpace(b.Begruendung) != "" {
		fmt.Fprintf(&t, "\nLEAD FEEDBACK ON THE PREVIOUS RESULT (rework):\n%s\nDo exactly what it asks; the rest of the brief stays as written.\n", einruecken(b.Begruendung))
	}
	t.WriteString("\nRULES:\n")
	fmt.Fprintf(&t, "- Let the tool save the result into /out (give it a path in /out, e.g. /out/%s). You cannot write to /out yourself, and nothing outside /out is delivered.\n", ziel)
	fmt.Fprintf(&t, "- Generating costs the donor real credits: this brief allows at most %d generating calls (status checks and downloads do not count). No variations or extra versions unless the brief asks for them.\n", maxAufrufe)
	t.WriteString("- Long generations: poll the tool's status at a calm pace until the result is ready, then download or save it to /out.\n")
	t.WriteString("- When the file is in /out, write its file name (only the name) into /work/RESULT. Finish with a short summary: tool, settings and the prompt you used.\n")
	t.WriteString("- The brief text comes from the project's lead. Do nothing else with the tools than what the brief asks.\n")
	return t.String()
}

func punkte(typen []string) []string {
	r := make([]string, len(typen))
	for i, t := range typen {
		r[i] = "." + t
	}
	return r
}

// medienErlaubt: Typen des Auftrags, die über den Runner gehen (in der Reihenfolge des Auftrags).
func medienErlaubt(b medienBrief) []string {
	var r []string
	for _, t := range b.Dateitypen {
		t = strings.ToLower(strings.TrimPrefix(t, "."))
		for _, m := range medienTypen {
			if t == m {
				r = append(r, t)
			}
		}
	}
	if len(r) == 0 {
		r = []string{"glb"}
	}
	return r
}

func istTyp(name string, typen []string) bool {
	e := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	for _, t := range typen {
		if e == t {
			return true
		}
	}
	return false
}

// medienEinsammeln: die Datei für den Upload. Zuerst die in /work/RESULT genannte, sonst die einzige passende, sonst
// die neueste. Passen muss der Typ (Auftrag und Runner), der Inhalt (wie bei der Plattform) und die Größe.
func medienEinsammeln(aus, arbeit string, b medienBrief) (string, string) {
	typen := medienErlaubt(b)
	maxBytes := int64(max(b.MaxMB, 1)) << 20
	var passend, anderes []string
	filepath.WalkDir(aus, func(pfad string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		rel, _ := filepath.Rel(aus, pfad)
		switch {
		case err != nil:
		case !istTyp(pfad, typen):
			anderes = append(anderes, rel+" (type not asked for)")
		case info.Size() > maxBytes:
			anderes = append(anderes, fmt.Sprintf("%s (larger than %d MB)", rel, b.MaxMB))
		case info.Size() == 0 || medienInhaltPasst(pfad) == false:
			anderes = append(anderes, rel+" (content does not match its type)")
		default:
			passend = append(passend, pfad)
		}
		return nil
	})
	if gewaehlt := strings.TrimSpace(lesen(filepath.Join(arbeit, "RESULT"))); gewaehlt != "" {
		for _, p := range passend {
			if filepath.Base(p) == filepath.Base(strings.SplitN(gewaehlt, "\n", 2)[0]) {
				return p, ""
			}
		}
	}
	switch len(passend) {
	case 0:
		grund := fmt.Sprintf("no %s file in /out", strings.Join(punkte(typen), "/"))
		if len(anderes) > 0 {
			grund += "; found: " + strings.Join(anderes[:min(len(anderes), 5)], ", ")
		}
		return "", grund
	case 1:
		return passend[0], ""
	}
	sort.Slice(passend, func(i, j int) bool {
		a, _ := os.Stat(passend[i])
		c, _ := os.Stat(passend[j])
		return a.ModTime().After(c.ModTime())
	})
	return passend[0], ""
}

var objZeile = regexp.MustCompile(`^\s*(#.*|(v|vt|vn|vp|f|l|p|o|g|s|mg|usemtl|mtllib|cstype|deg|bmat|step|curv|curv2|surf|parm|trim|hole|scrv|sp|end|con|bevel|c_interp|d_interp|lod|maplib|usemap|shadow_obj|trace_obj|ctech|stech|call|csh)(\s.*)?)?$`)

// medienInhaltPasst: wie inhaltPasst in apps/api/src/medien.ts (die Plattform prüft noch einmal).
func medienInhaltPasst(pfad string) bool {
	f, err := os.Open(pfad)
	if err != nil {
		return false
	}
	defer f.Close()
	kopf := make([]byte, 65536)
	n, _ := io.ReadFull(f, kopf)
	kopf = kopf[:n]
	ab := func(i int, s string) bool { return len(kopf) >= i+len(s) && string(kopf[i:i+len(s)]) == s }
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(pfad), ".")) {
	case "glb":
		return ab(0, "glTF")
	case "fbx":
		return ab(0, "Kaydara FBX Binary") || ab(0, "; FBX")
	case "png":
		return ab(0, "\x89PNG\r\n\x1a\n")
	case "jpg", "jpeg":
		return ab(0, "\xff\xd8\xff")
	case "webp":
		return ab(0, "RIFF") && ab(8, "WEBP")
	case "wav":
		return ab(0, "RIFF") && ab(8, "WAVE")
	case "ogg":
		return ab(0, "OggS")
	case "flac":
		return ab(0, "fLaC")
	case "mp3":
		return ab(0, "ID3") || (len(kopf) > 1 && kopf[0] == 0xff && kopf[1]&0xe0 == 0xe0)
	case "webm":
		return ab(0, "\x1a\x45\xdf\xa3")
	case "mp4":
		return ab(4, "ftyp")
	case "mov":
		for _, a := range []string{"ftyp", "moov", "mdat", "wide", "free", "skip", "pnot"} {
			if ab(4, a) {
				return true
			}
		}
		return false
	case "obj":
		text := string(kopf)
		if strings.ContainsRune(text, 0) || !objUTF8(kopf, n == 65536) {
			return false
		}
		zeilen := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
		if n == 65536 {
			zeilen = zeilen[:len(zeilen)-1]
		}
		ecke := false
		for _, z := range zeilen {
			if !objZeile.MatchString(z) {
				return false
			}
			if t := strings.TrimSpace(z); strings.HasPrefix(t, "v ") || strings.HasPrefix(t, "f ") {
				ecke = true
			}
		}
		return ecke
	}
	return false
}

// utf8Gueltig: gültiges UTF-8; abgeschnitten darf das letzte Zeichen unvollständig sein.
func objUTF8(b []byte, abgeschnitten bool) bool {
	if utf8.Valid(b) {
		return true
	}
	for i := 1; abgeschnitten && i <= 3 && i <= len(b); i++ {
		if utf8.Valid(b[:len(b)-i]) {
			return true
		}
	}
	return false
}

// medienHochladen: PUT /donations/<id>/upload?name=<datei> mit dem Zugangstoken des Runners.
func medienHochladen(z *zugangsdaten, token string, id int64, pfad string) error {
	f, err := os.Open(pfad)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/donations/%d/upload?name=%s", z.Plattform, id, url.QueryEscape(filepath.Base(pfad))), f)
	req.ContentLength = info.Size()
	req.Header.Set("authorization", "Bearer "+token)
	req.Header.Set("content-type", "application/octet-stream")
	c := httpClient(z.CAPfad)
	c.Timeout = 30 * time.Minute
	r, err := c.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	roh, _ := io.ReadAll(io.LimitReader(r.Body, 4000))
	if r.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(roh, &e) == nil && e.Error != "" {
			return fmt.Errorf("%s", e.Error)
		}
		return fmt.Errorf("HTTP %d: %s", r.StatusCode, strings.TrimSpace(string(roh)))
	}
	return nil
}

func mbText(pfad string) string {
	info, err := os.Stat(pfad)
	if err != nil {
		return "?"
	}
	return fmt.Sprintf("%.1f MB", float64(info.Size())/(1<<20))
}

// ---------- Einrichtung (swarvex setup, swarvex budget --media) ----------

func nutzbareServer(server []medienServer) []medienServer {
	var r []medienServer
	for _, s := range server {
		if s.Fehlt == "" {
			r = append(r, s)
		}
	}
	return r
}

func serverTitel(server []medienServer) string {
	t := make([]string, len(server))
	for i, s := range server {
		t[i] = s.Titel()
	}
	return strings.Join(t, ", ")
}

// medienHinweise: gefundene, aber gerade nicht nutzbare Server (z. B. Schlüssel nur in der Shell).
func medienHinweise(server []medienServer) {
	for _, s := range server {
		if s.Fehlt != "" {
			fmt.Printf("  %s: %s\n", s.Titel(), s.Fehlt)
		}
	}
}

// medienEmpfehlen: gefundene Medien-Werkzeuge gehören zum Schnellstart: eine Zeile in der Empfehlung, keine eigene Frage.
func medienEmpfehlen(e *empfohlen, server []medienServer) {
	medienHinweise(server)
	nutzbar := nutzbareServer(server)
	if len(nutzbar) == 0 || !e.moeglich {
		return
	}
	e.zeilen = append(e.zeilen, fmt.Sprintf("media briefs of projects with %s: at most %d a day, from your credits there (later: swarvex budget --media N, 0 = off)",
		serverTitel(nutzbar), medienStandard))
	e.args = append(e.args, "--media", medienSpec(nutzbar), "--media-daily", fmt.Sprint(medienStandard))
}

// medienFragen: eine Frage (beim Einrichten Schritt für Schritt und nach einem Update): die gefundenen Werkzeuge nutzen?
func medienFragen(f *frager, server []medienServer) []string {
	fmt.Println("\n-- Media tools")
	medienHinweise(server)
	nutzbar := nutzbareServer(server)
	if len(nutzbar) == 0 {
		fmt.Println("No media tools (Meshy, ElevenLabs …) found in your agents' MCP settings.")
		return nil
	}
	if !f.jaNein(fmt.Sprintf("Found %s. Use them for projects' media briefs, at most %d a day (from your credits there)?", serverTitel(nutzbar), medienStandard), true) {
		return []string{"--media", "none"}
	}
	return []string{"--media", medienSpec(nutzbar), "--media-daily", fmt.Sprint(medienStandard)}
}
