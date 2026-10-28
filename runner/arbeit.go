package main

// Eine Aufgabe bearbeiten: reservieren → Snapshot → Sandbox (Agent + Schlüssel-Proxy)
// → Patch bilden und prüfen → einreichen. Alles, was aus dem Projekt kommt, läuft nur in
// der Sandbox; der Runner selbst führt keinen Projektcode aus.

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

type aufgabe struct {
	ID           int64    `json:"id"`
	Titel        string   `json:"title"`
	Beschreibung string   `json:"description"`
	Stufe        string   `json:"level"`
	Pfade        []string `json:"paths"`
	Akzeptanz    []string `json:"acceptance"`
	BasisCommit  string   `json:"base_commit"`
	Projekt      string   `json:"project"`
	Engine       string   `json:"engine"`
	Grenzen      struct {
		Zeilen  int `json:"lines"`
		Dateien int `json:"files"`
	} `json:"limits"`
}

func arbeitenBefehl(args []string) {
	fs := flag.NewFlagSet("work", flag.ExitOnError)
	projekt := fs.String("project", "", "project slug")
	nur := fs.Int64("task", 0, "a specific task (otherwise the first matching one)")
	agentName := fs.String("agent", "opencode", "agent in the sandbox: "+agentNamen())
	modell := fs.String("model", "", "model name at the provider")
	upstream := fs.String("model-url", "", "base URL of the model API without /v1: OpenAI-compatible (opencode), Anthropic (claude) or OpenAI Responses (codex)")
	keyDatei := fs.String("model-key-file", "", "file containing the model's API key, for claude also a subscription token from `claude setup-token` (optional)")
	skillsWert := fs.String("skills", "", `your own skills for the agent: "auto" (the usual folders) or folders separated by ":" (copied, read-only)`)
	budget := fs.Int("budget-tokens", 400000, "token budget for this task")
	tagesbudget := fs.Int64("daily-budget-tokens", 0, "at most this many tokens per calendar day across all tasks (0 = no daily limit)")
	zeit := fs.Duration("time-limit", 90*time.Minute, "maximum run time of the sandbox")
	faehigkeiten := fs.String("capabilities", "", "comma-separated, e.g. code.gdscript,code.javascript")
	fensterText := fs.String("schedule", "", `when work is allowed, e.g. "mon-fri 18:00-23:00; sat,sun 09:00-24:00" or "daily 22:00-07:00" (empty = always)`)
	amEnde := fs.String("at-window-end", "stop", "stop: the sandbox ends with the time window at the latest; finish: a running task may finish")
	mindest := fs.Duration("min-time", 15*time.Minute, "do not start a new task if the time window ends sooner (only with stop)")
	dauer := fs.Bool("continuous", false, "keep taking tasks; outside the time windows wait for the next window")
	pause := fs.Duration("pause", 5*time.Minute, "wait time in continuous mode when no matching task is open")
	nurPruefen := fs.Bool("check-only", false, "only check the options, reserve nothing (used by `swarvex service`)")
	fs.Parse(args)
	if *projekt == "" || *modell == "" || *upstream == "" {
		fehlerAus("--project, --model and --model-url are required")
	}
	agent := agentPruefen(*agentName)
	skills := skillsLesen(*skillsWert)
	fenster, err := zeitfensterLesen(*fensterText)
	pruefen(err)
	if *amEnde != "stop" && *amEnde != "finish" {
		fehlerAus("--at-window-end accepts only stop or finish")
	}
	if *keyDatei != "" {
		roh, err := os.ReadFile(*keyDatei)
		if err != nil {
			fehlerAus("--model-key-file: " + err.Error())
		}
		if agent.Name == "codex" && strings.HasPrefix(strings.TrimSpace(string(roh)), "{") {
			fehlerAus("--model-key-file: Codex with a ChatGPT plan (auth.json) is not supported yet; use an OpenAI API key")
		}
	}
	if *nurPruefen {
		return
	}
	if *dauer {
		dauerbetrieb(fenster, *pause, ohneDauerbetrieb(args))
		return
	}

	// Zeitfenster: außerhalb wird keine Aufgabe reserviert (Exit-Code 4). Bei „stopp“ endet die Sandbox
	// spätestens mit dem Fenster; reicht die Restzeit nicht für eine sinnvolle Aufgabe, beginnt keine neue.
	if !fenster.offen(time.Now()) {
		if n, ok := fenster.naechsterBeginn(time.Now()); ok {
			fmt.Printf("Outside the time windows. Next window from %s.\n", zeitpunkt(n))
		}
		os.Exit(4)
	}
	// Tagesbudget: keine Aufgabe mehr, wenn es verbraucht ist (Exit-Code 6); sonst Aufgabenbudget auf den Rest kürzen.
	if *tagesbudget > 0 {
		rest := *tagesbudget - verbrauchAmTag(verbrauchsDatei(), time.Now())
		if rest < min(int64(*budget), *tagesbudget)/10 {
			fmt.Printf("Daily budget of %d tokens is used up. Continuing from %s.\n", *tagesbudget, zeitpunkt(mitternacht(time.Now())))
			os.Exit(6)
		}
		if int64(*budget) > rest {
			*budget = int(rest)
			fmt.Printf("Task budget reduced to what is left of the daily budget: %d tokens.\n", rest)
		}
	}
	if *amEnde == "stop" && len(fenster) > 0 {
		rest := fenster.verbleibend(time.Now())
		if rest < *mindest {
			fmt.Printf("Time window ends in %s, too short for a new task (--min-time %s).\n", rest, *mindest)
			os.Exit(4)
		}
		if *zeit > rest {
			*zeit = rest
			fmt.Printf("Time limit reduced to the end of the time window: %s.\n", rest.Round(time.Minute))
		}
	}

	// Strg+C bzw. `swarvex stop` (SIGTERM vom Dienst): laufende Arbeit beenden und die Aufgabe zurückgeben.
	basis, aufhoeren := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer aufhoeren()

	z, err := zugangLaden()
	pruefen(err)
	token, err := zugriffstoken(z)
	pruefen(err)
	m := &mcpClient{basis: z.Plattform, token: token, http: httpClient(z.CAPfad)}
	pruefen(m.initialisieren())

	// 0. Wartet eine Engine-Einreichung auf Gegenprüfung, kommt sie zuerst: schnell, ohne Modell, ohne Tokens.
	// Danach endet dieser Aufruf; im Dauerbetrieb geht es mit der nächsten Runde weiter.
	if *nur == 0 && gegenpruefenWennMoeglich(basis, m, z, *projekt) {
		return
	}

	// 1. Aufgabe wählen und reservieren.
	id := *nur
	if id == 0 {
		var liste []struct {
			ID int64 `json:"id"`
		}
		filter := map[string]any{"project": *projekt}
		if *faehigkeiten != "" {
			filter["capabilities"] = strings.Split(*faehigkeiten, ",")
		}
		pruefen(m.werkzeug("matching_tasks", filter, &liste))
		if len(liste) == 0 {
			fmt.Println("No matching open task.")
			os.Exit(3) // Dauerbetrieb wartet dann und fragt später wieder
		}
		id = liste[0].ID
	}
	if basis.Err() != nil {
		fmt.Println("Stopped, no task reserved.")
		os.Exit(5)
	}
	var lease map[string]any
	pruefen(m.werkzeug("reserve_task", map[string]any{"task_id": id}, &lease))
	// Ab hier gibt jeder Fehler die Aufgabe sofort zurück in den Pool.
	abbrechen := func(grund string) {
		m.werkzeug("abandon_task", map[string]any{"task_id": id, "reason": grund}, nil)
		fehlerAus(grund)
	}
	gestoppt := func() {
		if basis.Err() != nil {
			abbrechen("runner was stopped")
		}
	}
	var a aufgabe
	if err := m.werkzeug("task_details", map[string]any{"task_id": id}, &a); err != nil {
		abbrechen("reading the task: " + err.Error())
	}
	fmt.Printf("Task %d reserved: %s\n", a.ID, a.Titel)

	// 2. Snapshot holen und entpacken (ohne .git, ohne Symlinks, ohne Pfade nach außen).
	ordner := filepath.Join(datenOrdner(), "work", fmt.Sprint(id))
	os.RemoveAll(ordner)
	arbeitsbaum := filepath.Join(ordner, "tree")
	gitDir := filepath.Join(ordner, "git") // außerhalb des Sandbox-Mounts
	pruefen(os.MkdirAll(arbeitsbaum, 0o755))
	if err := snapshotHolen(z, token, id, arbeitsbaum); err != nil {
		abbrechen("snapshot: " + err.Error())
	}
	// LFS-Dateien aufgelöst, aber nicht in Git aufgenommen: Patches ändern keine Binärassets
	// (dafür gibt es Asset-Aufträge); so muss Git die großen Dateien auch nicht kopieren.
	lfsPfade, err := lfsAufloesen(z, token, id, arbeitsbaum)
	if err != nil {
		abbrechen("LFS: " + err.Error())
	}
	if err := gitBasis(gitDir, arbeitsbaum, lfsPfade); err != nil {
		abbrechen("base commit: " + err.Error())
	}
	gestoppt()

	// 2b. Umgebung des Projekts: erkennen, Image bauen, Abhängigkeiten laden (mit Netz, ohne Agent und Schlüssel).
	u, err := umgebungErmitteln(arbeitsbaum)
	if err != nil {
		abbrechen("environment: " + err.Error())
	}
	fmt.Printf("Environment: %s (%s)\n", u.Vorlage, u.Quelle)
	if err := agentImageBereit(basis, agent); err != nil {
		abbrechen(err.Error())
	}
	image, err := umgebungsImage(basis, u, arbeitsbaum, agent.Image)
	gestoppt()
	if err != nil {
		abbrechen(err.Error())
	}
	cache := cacheOrdner(*projekt)
	err = einrichten(basis, u, image, arbeitsbaum, cache, filepath.Join(ordner, "setup.log"))
	gestoppt()
	if err != nil {
		abbrechen(err.Error())
	}

	// 3. Lebenszeichen, solange gearbeitet wird.
	ctx, stopp := context.WithTimeout(basis, *zeit)
	defer stopp()
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		m.werkzeug("report_progress", map[string]any{"task_id": id, "note": "sandbox started"}, nil)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if neu, err := zugriffstoken(z); err == nil {
					m.token = neu
				}
				m.werkzeug("report_progress", map[string]any{"task_id": id, "note": "working"}, nil)
			}
		}
	}()

	// 4. Sandbox mit Schlüssel-Proxy.
	key := ""
	if *keyDatei != "" {
		roh, err := os.ReadFile(*keyDatei)
		pruefen(err)
		key = strings.TrimSpace(string(roh))
	}
	verbrauch, protokoll, err := sandboxAusfuehren(ctx, a, u, image, arbeitsbaum, cache, ordner, agent, skills, *upstream, key, *modell, *budget, nil)
	stopp()
	if err := verbrauchMerken(verbrauchsDatei(), id, tokensAus(verbrauch), time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, "recording usage:", err)
	}
	gestoppt()
	if err != nil {
		abbrechen("sandbox: " + err.Error())
	}

	// 5. Patch bilden (Git-Verzeichnis liegt außerhalb der Sandbox) und selbst prüfen.
	patch, verworfen, err := patchBilden(gitDir, arbeitsbaum, a.Pfade, u)
	if verworfen > 0 {
		fmt.Printf("Discarded %d side changes outside the allowed paths (e.g. dependencies, build or import files); they never leave this machine.\n", verworfen)
	}
	if err != nil || strings.TrimSpace(patch) == "" {
		abbrechen("no usable result from the sandbox")
	}
	if fehler := patchVorpruefen(patch, a.Pfade, u); len(fehler) > 0 {
		abbrechen("patch violates the task: " + strings.Join(fehler, "; "))
	}

	// 5b. Eigener Testlauf (ohne Agent, ohne Netz). Bei Unity/Unreal ist das die erste Prüfung.
	m.werkzeug("report_progress", map[string]any{"task_id": id, "note": "running the project's tests"}, nil)
	testErgebnis, testBericht := eigenerTestlauf(basis, u, image, arbeitsbaum, cache)
	gestoppt()
	if testErgebnis != "" {
		fmt.Printf("Own test run: %s\n", testErgebnis)
	}

	// 6. Einreichen (mit Beitragserklärung).
	if neu, err := zugriffstoken(z); err == nil {
		m.token = neu
	}
	einreichung := map[string]any{
		"task_id": id, "patch": patch, "summary": zusammenfassungAus(protokoll),
		"own_tests": kuerzen(testBericht, 19000), "usage": verbrauch, "rights_confirmed": true,
	}
	if testErgebnis != "" {
		einreichung["own_test_result"] = testErgebnis
	}
	var ergebnis map[string]any
	pruefen(m.werkzeug("submit_result", einreichung, &ergebnis))
	fmt.Println("Submitted:", ergebnis)
	// Arbeitsbaum (mit LFS-Kopien bis über 1 GB) aufräumen; Protokolle (agent.log, proxy.log) bleiben.
	os.RemoveAll(arbeitsbaum)
	os.RemoveAll(gitDir)
}

func datenOrdner() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "swarvex")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".local", "share", "swarvex")
}

func snapshotHolen(z *zugangsdaten, token string, id int64, ziel string) error {
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/tasks/%d/snapshot", z.Plattform, id), nil)
	req.Header.Set("authorization", "Bearer "+token)
	c := httpClient(z.CAPfad)
	c.Timeout = 10 * time.Minute
	r, err := c.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		roh, _ := io.ReadAll(r.Body)
		return fmt.Errorf("HTTP %d: %s", r.StatusCode, roh)
	}
	gz, err := gzip.NewReader(r.Body)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := path.Clean(h.Name)
		if name == "." || strings.HasPrefix(name, "..") || path.IsAbs(name) || strings.Contains("/"+name+"/", "/.git/") {
			continue
		}
		pfad := filepath.Join(ziel, filepath.FromSlash(name))
		switch h.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(pfad, 0o755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(pfad), 0o755)
			f, err := os.OpenFile(pfad, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, io.LimitReader(tr, 1<<30))
			f.Close()
			if err != nil {
				return err
			}
		} // Symlinks und Sonderdateien werden bewusst nicht angelegt.
	}
}

// git führt Git mit eigenem GIT_DIR außerhalb des Arbeitsbaums aus. Hooks, fsmonitor und
// Filter sind abgeschaltet, damit nichts aus dem (von der Sandbox beschriebenen) Baum
// beim Diff ausgeführt werden kann.
func git(gitDir, baum string, args ...string) (string, error) {
	sicher := []string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "core.autocrlf=false",
		"-c", "user.name=swarvex", "-c", "user.email=runner@swarvex.invalid", "-c", "commit.gpgsign=false"}
	cmd := exec.Command("git", append(sicher, args...)...)
	cmd.Env = append(os.Environ(), "GIT_DIR="+gitDir, "GIT_WORK_TREE="+baum, "GIT_CONFIG_NOSYSTEM=1", "GIT_ATTR_NOSYSTEM=1")
	aus, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", args[0], err, aus)
	}
	return string(aus), nil
}

func gitBasis(gitDir, baum string, ausschliessen []string) error {
	if _, err := git(gitDir, baum, "init", "-q"); err != nil {
		return err
	}
	if len(ausschliessen) > 0 {
		liste := make([]string, len(ausschliessen))
		for i, p := range ausschliessen {
			liste[i] = "/" + p
		}
		os.MkdirAll(filepath.Join(gitDir, "info"), 0o755)
		if err := os.WriteFile(filepath.Join(gitDir, "info", "exclude"), []byte(strings.Join(liste, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}
	if _, err := git(gitDir, baum, "add", "-A"); err != nil {
		return err
	}
	_, err := git(gitDir, baum, "commit", "-q", "-m", "basis")
	return err
}

// patchBilden nimmt nur Änderungen in den erlaubten Pfaden auf. Alles andere – etwa Dateien,
// die Werkzeuge beim Testen neu schreiben – bleibt auf dem Gerät und wird nur gezählt.
func patchBilden(gitDir, baum string, erlaubt []string, u umgebung.Umgebung) (string, int, error) {
	args := []string{"add", "-A", "--"}
	for _, g := range erlaubt {
		args = append(args, ":(glob)"+g)
	}
	// Geschützte Dateien (Umgebung, Projektregeln, CI) nie aufnehmen, auch wenn ein Muster sie trifft.
	for _, g := range append(append([]string{}, umgebung.Geschuetzt...), u.Geschuetzt...) {
		args = append(args, ":(exclude,glob)"+g)
	}
	if _, err := git(gitDir, baum, args...); err != nil {
		return "", 0, err
	}
	rest, _ := git(gitDir, baum, "status", "--porcelain", "--untracked-files=all")
	verworfen := 0
	for _, zeile := range strings.Split(rest, "\n") {
		if len(zeile) > 3 && zeile[1] != ' ' { // im Arbeitsbaum geändert, aber nicht aufgenommen
			verworfen++
		}
	}
	patch, err := git(gitDir, baum, "diff", "--cached", "--binary", "--no-color", "--no-ext-diff", "--no-textconv")
	return patch, verworfen, err
}

// patchVorpruefen: dieselbe Regel wie die Plattform (erlaubte Pfade), damit nichts Verbotenes das Gerät verlässt.
func patchVorpruefen(patch string, erlaubt []string, u umgebung.Umgebung) []string {
	var fehler []string
	for _, zeile := range strings.Split(patch, "\n") {
		if !strings.HasPrefix(zeile, "diff --git a/") {
			continue
		}
		teile := strings.SplitN(strings.TrimPrefix(zeile, "diff --git a/"), " b/", 2)
		for _, p := range teile {
			if u.Gesperrt(p) {
				fehler = append(fehler, "protected file (environment, project rules or CI): "+p)
				continue
			}
			ok := false
			for _, g := range erlaubt {
				if passt(g, p) {
					ok = true
				}
			}
			if !ok {
				fehler = append(fehler, "outside the allowed paths: "+p)
			}
		}
	}
	return fehler
}

// passt unterstützt die Glob-Formen der Plattform: exakt, *, und ** am Ende.
func passt(muster, pfad string) bool {
	if strings.HasSuffix(muster, "/**") {
		return strings.HasPrefix(pfad, strings.TrimSuffix(muster, "**"))
	}
	ok, _ := path.Match(muster, pfad)
	return ok
}

func zufall(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func zusammenfassungAus(protokoll string) string {
	if len(protokoll) > 4000 {
		protokoll = protokoll[len(protokoll)-4000:]
	}
	return "Worked on in the sandbox. Last output of the agent:\n" + protokoll
}

// sandboxAusfuehren startet Proxy und Sandbox (rootless Podman) und gibt Verbrauch und Agenten-Ausgabe zurück.
// befehl ersetzt den Agenten (nur für Angriffstests, siehe angriff.go); nil = der gewählte Agent mit dem Auftrag.
func sandboxAusfuehren(ctx context.Context, a aufgabe, u umgebung.Umgebung, image, baum, cache, ordner string, agent agentArt, skills []string,
	upstream, key, modell string, budget int, befehl []string) (map[string]float64, string, error) {
	podman := func(args ...string) (string, error) {
		aus, err := exec.CommandContext(ctx, "podman", args...).CombinedOutput()
		return strings.TrimSpace(string(aus)), err
	}
	netz := "swarvex-intern"
	if _, err := podman("network", "exists", netz); err != nil {
		if aus, err := podman("network", "create", "--internal", netz); err != nil {
			return nil, "", fmt.Errorf("creating the network: %s", aus)
		}
	}
	tok := zufall(36)
	proxyName := fmt.Sprintf("swarvex-proxy-%d", a.ID)
	podman("rm", "-f", proxyName)
	proxyArgs := []string{"run", "-d", "--name", proxyName, "--network", netz, "--network", "podman", "--timeout", "86400",
		"--read-only", "--cap-drop=ALL", "--security-opt", "no-new-privileges", "--memory", "256m", "--pids-limit", "64",
		"-e", "SWARVEX_UPSTREAM=" + upstream, "-e", "SWARVEX_API=" + agent.API, "-e", "SWARVEX_MODEL=" + modell, "-e", "SWARVEX_TASK_TOKEN=" + tok,
		"-e", fmt.Sprintf("SWARVEX_BUDGET_TOKENS=%d", budget)}
	if key != "" {
		proxyArgs = append(proxyArgs, "-e", "SWARVEX_UPSTREAM_KEY="+key)
	}
	if aus, err := podman(append(proxyArgs, "swarvex-proxy")...); err != nil {
		return nil, "", fmt.Errorf("starting the proxy: %s", aus)
	}
	defer exec.Command("podman", "rm", "-f", proxyName).Run()

	auftrag := auftragstext(a, u)
	agentArgs, agentBefehl, err := agentStart(agent, ordner, "http://"+proxyName+":8080", tok, modell, auftrag, skills, baum, u.Ordner)
	if err != nil {
		return nil, "", err
	}
	sandboxName := fmt.Sprintf("swarvex-sandbox-%d", a.ID)
	podman("rm", "-f", sandboxName)
	rest := 3600
	if frist, ok := ctx.Deadline(); ok {
		rest = max(int(time.Until(frist).Seconds()), 1)
	}
	// Agent im Projekt-Image: kein Netz außer dem Schlüssel-Proxy, Projekt unter /work, Caches unter /cache.
	sandboxArgs := append(append(containerArgs(sandboxName, baum, cache, u, rest), "--network", netz), agentArgs...)
	sandboxArgs = append(sandboxArgs, image)
	if befehl == nil {
		befehl = agentBefehl
	}
	cmd := exec.CommandContext(ctx, "podman", append(sandboxArgs, befehl...)...)
	cmd.Cancel = func() error {
		exec.Command("podman", "kill", sandboxName).Run()
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 10 * time.Second
	aus, err := cmd.CombinedOutput()
	exec.Command("podman", "rm", "-f", sandboxName).Run()
	os.WriteFile(filepath.Join(ordner, "agent.log"), aus, 0o644)

	// Verbrauch aus dem Proxy-Protokoll.
	proxyLog, _ := exec.Command("podman", "logs", proxyName).CombinedOutput()
	os.WriteFile(filepath.Join(ordner, "proxy.log"), proxyLog, 0o644)
	verbrauch := map[string]float64{"tokens": 0, "requests": 0, "seconds": 0}
	for _, zeile := range strings.Split(string(proxyLog), "\n") {
		var e struct {
			Art    string  `json:"type"`
			Tokens float64 `json:"tokens"`
			Ms     float64 `json:"ms"`
		}
		if json.Unmarshal([]byte(zeile), &e) == nil && e.Art == "request" {
			verbrauch["tokens"] += e.Tokens
			verbrauch["requests"]++
			verbrauch["seconds"] += e.Ms / 1000
		}
	}
	if ctx.Err() != nil {
		return verbrauch, string(aus), fmt.Errorf("time limit reached")
	}
	if err != nil {
		return verbrauch, string(aus), fmt.Errorf("agent exited with an error: %v", err)
	}
	return verbrauch, string(aus), nil
}

func auftragstext(a aufgabe, u umgebung.Umgebung) string {
	var b strings.Builder
	wurzel := "/work"
	if u.Ordner != "" {
		wurzel += "/" + u.Ordner
	}
	fmt.Fprintf(&b, "You are working as a contributor on an open-source project. The project is in /work, you start in %s.\n", wurzel)
	if u.Hinweis != "" {
		fmt.Fprintf(&b, "Environment: %s\n", u.Hinweis)
	}
	b.WriteString("Read AGENTS.md first (if present) and follow the project's rules.\n\n")
	fmt.Fprintf(&b, "TASK: %s\n\n%s\n\n", a.Titel, a.Beschreibung)
	b.WriteString("ACCEPTANCE CRITERIA:\n")
	for _, k := range a.Akzeptanz {
		fmt.Fprintf(&b, "- %s\n", k)
	}
	b.WriteString("\nRULES:\n- Change or create ONLY these paths: " + strings.Join(a.Pfade, ", ") + "\n")
	b.WriteString("- Do not create or change any other files (no notes, logs or helper scripts in the project either).\n")
	if u.Testen != "" {
		fmt.Fprintf(&b, "- The project's tests run with:\n%s\n  Run them and fix failures until they pass.\n", einruecken(u.Testen))
	} else {
		b.WriteString("- Run the checks from the acceptance criteria yourself and fix failures.\n")
	}
	b.WriteString("- There is no internet; all dependencies are already installed. Finish with a short summary of what you did and how the tests went.\n")
	return b.String()
}

func einruecken(text string) string {
	return "    " + strings.ReplaceAll(strings.TrimSpace(text), "\n", "\n    ")
}

// dauerbetrieb holt in den offenen Zeitfenstern laufend Aufgaben. Jede Aufgabe läuft als eigener
// Kindprozess („work“ ohne --continuous), damit ein Fehler nicht den ganzen Runner beendet.
// Exit-Codes des Kindes: 0 erledigt, 3 keine passende Aufgabe, 4 außerhalb bzw. zu kurz vor Fensterende,
// 5 gestoppt vor der Reservierung, 6 Tagesbudget verbraucht.
func dauerbetrieb(fenster zeitfenster, pause time.Duration, args []string) {
	fmt.Println("Continuous mode started. Stop with Ctrl+C or `swarvex stop`.")
	// Beim Anhalten bekommt die laufende Aufgabe das Signal ebenfalls, gibt sich zurück und endet; dann ist Schluss.
	basis, aufhoeren := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer aufhoeren()
	schlafen := func(d time.Duration) {
		select {
		case <-basis.Done():
		case <-time.After(d):
		}
	}
	for basis.Err() == nil {
		jetzt := time.Now()
		if !fenster.offen(jetzt) {
			n, ok := fenster.naechsterBeginn(jetzt)
			if !ok {
				fehlerAus("The time windows never open.")
			}
			fmt.Printf("Outside the time windows. Waiting until %s.\n", zeitpunkt(n))
			// in Stücken schlafen, damit Uhrumstellung oder Ruhezustand nicht zu Fehlzeiten führen
			for time.Now().Before(n) && basis.Err() == nil {
				schlafen(min(time.Until(n), 10*time.Minute))
			}
			continue
		}
		kind := exec.Command(os.Args[0], append([]string{"work"}, args...)...)
		kind.Stdout, kind.Stderr = os.Stdout, os.Stderr
		code := 0
		if err := kind.Start(); err != nil {
			fehlerAus("starting the task: " + err.Error())
		}
		fertig := make(chan struct{})
		go func() {
			select {
			case <-basis.Done(): // Signal weiterreichen, falls es nur den Dauerbetrieb erreicht hat
				kind.Process.Signal(syscall.SIGTERM)
			case <-fertig:
			}
		}()
		err := kind.Wait()
		close(fertig)
		if basis.Err() != nil {
			break
		}
		if err != nil {
			code = 1
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			}
		}
		switch code {
		case 0:
			continue // gleich die nächste Aufgabe
		case 3:
			fmt.Printf("No matching task. Next attempt in %s.\n", pause)
		case 4:
			// Fenster zu oder zu kurz: die Schleife wartet oben auf das nächste
		case 6:
			n := mitternacht(time.Now())
			fmt.Printf("Daily budget used up. Waiting until %s.\n", zeitpunkt(n))
			for time.Now().Before(n) && basis.Err() == nil {
				schlafen(min(time.Until(n), 10*time.Minute))
			}
			continue
		default:
			fmt.Printf("Task ended with an error (code %d). Next attempt in %s.\n", code, pause)
		}
		if code != 4 {
			schlafen(pause)
		} else {
			schlafen(time.Minute)
		}
	}
	fmt.Println("Continuous mode ended.")
}

// ohneDauerbetrieb entfernt --continuous aus den Argumenten für den Kindprozess.
func ohneDauerbetrieb(args []string) []string {
	var r []string
	for _, a := range args {
		switch strings.TrimLeft(a, "-") {
		case "continuous", "continuous=true", "continuous=1":
			continue
		}
		r = append(r, a)
	}
	return r
}
