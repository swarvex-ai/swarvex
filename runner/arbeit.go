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
	"regexp"
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
	// Der letzte Versuch ging an der Prüfstraße zurück in den Pool: deren Bericht (nur Testausgabe)
	LetztePruefung *struct {
		Bericht string `json:"report"`
	} `json:"last_check"`
	// Der Lead schickte den letzten Versuch zurück (Nacharbeit oder Ablehnung): seine Begründung
	LetzteBewertung *struct {
		Urteil      string `json:"verdict"`
		Begruendung string `json:"reason"`
	} `json:"last_review"`
}

func arbeitenBefehl(args []string) {
	args = mitGespeicherten(args)
	fs := flag.NewFlagSet("work", flag.ExitOnError)
	projekt := fs.String("project", "", `project slug, or "auto" (wherever most tasks are open)`)
	nur := fs.Int64("task", 0, "a specific task (otherwise the first matching one)")
	agentName := fs.String("agent", "opencode", "agent in the sandbox: "+agentNamen())
	modell := fs.String("model", "", "model name at the provider")
	upstream := fs.String("model-url", "", "base URL of the model API without /v1: OpenAI-compatible (opencode), Anthropic (claude) or OpenAI Responses (codex)")
	keyDatei := fs.String("model-key-file", "", "file containing the model's API key; claude: also a subscription token from `claude setup-token`; codex: also your ChatGPT sign-in ~/.codex/auth.json (optional)")
	skillsWert := fs.String("skills", "", `your own skills for the agent: "auto" (the usual folders) or folders separated by ":" (copied, read-only)`)
	budget := fs.Int("budget-tokens", 0, "token limit for one task (0 = none; the daily budget still caps every task)")
	tagesbudget := fs.Int64("daily-budget-tokens", 0, "at most this many tokens per calendar day across all tasks (0 = no daily limit)")
	zeit := fs.Duration("time-limit", 90*time.Minute, "maximum run time of the sandbox")
	faehigkeiten := fs.String("capabilities", "", "comma-separated, e.g. code.gdscript,code.javascript")
	fensterText := fs.String("schedule", "", `when work is allowed, e.g. "mon-fri 18:00-23:00; sat,sun 09:00-24:00" or "daily 22:00-07:00" (empty = always)`)
	amEnde := fs.String("at-window-end", "stop", "stop: the sandbox ends with the time window at the latest; finish: a running task may finish")
	mindest := fs.Duration("min-time", 15*time.Minute, "do not start a new task if the time window ends sooner (only with stop)")
	dauer := fs.Bool("continuous", false, "keep taking tasks; outside the time windows wait for the next window")
	autoAktualisieren := fs.Bool("auto-update", false, "with --continuous: install new runner versions by itself, at most once a day and only between tasks")
	pause := fs.Duration("pause", 5*time.Minute, "wait time in continuous mode when no matching task is open")
	nurPruefen := fs.Bool("check-only", false, "only check the options, reserve nothing (used by `swarvex service`)")
	rest := fs.Bool("leftover", false, "beta, subscriptions only: work only with what your subscription would let expire, shortly before its weekly reset")
	restReserve := fs.Float64("leftover-reserve", 10, "with --leftover: percent of the weekly window that always stays yours")
	restVor := fs.Duration("leftover-before-reset", 24*time.Hour, "with --leftover: start this long before the weekly window resets")
	restSitzung := fs.Float64("leftover-session-max", 80, "with --leftover: never fill the 5-hour window beyond this percent")
	restNacht := fs.String("leftover-night", "", "with --leftover: also work in these time windows before the reset, e.g. \"daily 01:00-06:00\" (default with a night share)")
	restNachtAnteil := fs.Float64("leftover-night-share", 0, "with --leftover: percent of the weekly window the agent may use per night (0 = no night work, 100 = no limit)")
	restNachtDecke := fs.Float64("leftover-night-max", 80, "with --leftover: no night work once this percent of the weekly window is used")
	medien := fs.String("media", "", "media tools of your agents for projects' media briefs (found by `swarvex setup`, e.g. claude:meshy,hermes:elevenlabs); empty or none = off")
	medienTag := fs.Int("media-daily", medienStandard, fmt.Sprintf("media briefs per day at most (1–%d; 0 = none)", medienHoechstens))
	medienAufrufe := fs.Int("media-max-calls", 8, "generating tool calls per media brief at most (status checks and downloads do not count)")
	fs.StringVar(&setupNetz, "setup-network", "", "network while installing dependencies: public (default, only public addresses through a filtering proxy), lan (also your local network), open (no proxy, like before 0.9.3)")
	fs.StringVar(&setupErlaubt, "setup-allow", "", "with public: exceptions for the setup, e.g. a package mirror in your network (IP, CIDR or host name, comma-separated)")
	fs.Parse(args)
	if *projekt == "" || *modell == "" || (*upstream == "" && !codexAboDatei(*agentName, *keyDatei)) {
		fehlerAus("--project, --model and --model-url are required (--model-url not with Codex and a ChatGPT sign-in)")
	}
	agent := agentPruefen(*agentName)
	skills := skillsLesen(*skillsWert)
	fenster, err := zeitfensterLesen(*fensterText)
	pruefen(err)
	if *amEnde != "stop" && *amEnde != "finish" {
		fehlerAus("--at-window-end accepts only stop or finish")
	}
	aboZugang := false
	if *keyDatei != "" {
		roh, err := os.ReadFile(*keyDatei)
		if err != nil {
			fehlerAus("--model-key-file: " + err.Error())
		}
		if istCodexAnmeldung(roh) && agent.Name != "codex" {
			fehlerAus("--model-key-file: a Codex sign-in (auth.json) only works with --agent codex")
		}
		aboZugang = istCodexAnmeldung(roh) || strings.HasPrefix(strings.TrimSpace(string(roh)), "sk-ant-oat")
	}
	if *rest {
		if !aboZugang {
			fehlerAus("--leftover works only with a subscription (Claude via `claude setup-token`, ChatGPT via the Codex sign-in): with an API key nothing expires")
		}
		r, err := restRegelnLesen(fmt.Sprintf("reserve=%g;before=%s;session=%g", *restReserve, *restVor, *restSitzung))
		pruefen(err)
		r.NachtText, r.NachtAnteil, r.NachtDecke = *restNacht, *restNachtAnteil, *restNachtDecke
		if r.NachtAnteil > 0 && r.NachtText == "" {
			r.NachtText = fairNacht
		}
		pruefen(r.gueltig())
		restModus = &r
	}
	if *nurPruefen {
		return
	}
	if *dauer {
		dauerbetrieb(fenster, *pause, ohneDauerbetrieb(args), *autoAktualisieren)
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
	// Restkontingent: nur kurz vor dem Wochen-Reset und nur bis zur Reserve (Exit-Code 7, kontingent.go).
	if restModus != nil {
		if k, ok := kontingentLesen(kontingentDatei()); ok {
			if erlaubt, grund, ab := restModus.fuer(k, time.Now()).pruefen(k.fortgeschrieben(time.Now()), time.Now()); !erlaubt {
				fmt.Printf("Leftover mode: %s. Next chance from %s.\n", grund, zeitpunkt(ab))
				os.Exit(7)
			}
		} else {
			fmt.Println("Leftover mode: the subscription's state is not known yet; the first request shows it, and the proxy stops if it is not leftover.")
		}
	}
	// Tagesbudget: keine Aufgabe mehr, wenn es verbraucht ist (Exit-Code 6); sonst Aufgabenbudget auf den Rest kürzen.
	if *tagesbudget > 0 {
		rest := *tagesbudget - verbrauchAmTag(verbrauchsDatei(), time.Now())
		grenze := *tagesbudget
		if *budget > 0 {
			grenze = min(int64(*budget), *tagesbudget)
		}
		if rest < grenze/10 {
			fmt.Printf("Daily budget of %d tokens is used up. Continuing from %s.\n", *tagesbudget, zeitpunkt(mitternacht(time.Now())))
			os.Exit(6)
		}
		if *budget == 0 || int64(*budget) > rest {
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

	// 0. Wartet die Einreichung eines anderen Menschen auf Gegenprüfung, kommt sie zuerst: ohne Modell, ohne Tokens,
	// und sie hält den Fluss für alle in Gang (Prüfen vor neuer Arbeit).
	// Danach endet dieser Aufruf; im Dauerbetrieb geht es mit der nächsten Runde weiter.
	// "auto": alle Projekte mit offenen Aufgaben, die meisten zuerst; das erste mit passender Aufgabe gilt.
	projekte := projekteFuer(*projekt, z)
	if *nur == 0 {
		for _, p := range projekte {
			if gegenpruefenWennMoeglich(basis, m, z, p) {
				return
			}
		}
	}

	// 0b. Medien-Aufträge (Zusage über den Runner, medien.go): Zusagen abgleichen; wartet ein Auftrag, kommt er vor den
	// Code-Aufgaben (es gibt weniger davon, und das Tageslimit begrenzt sie ohnehin).
	if *nur == 0 && (*medien != "" || lesen(filepath.Join(datenOrdner(), "media-pledges.json")) != "") {
		auswahl := medienAuswahl(*medien, medienServerFinden())
		medienZusagenAbgleichen(m, auswahl, *medienTag, *projekt)
		for _, s := range auswahl {
			if s.Fehlt != "" {
				fmt.Printf("Media tool %s is not usable right now: %s\n", s.Titel(), s.Fehlt)
			}
		}
		if len(auswahl) > 0 && *medienTag > 0 && medienRunde(basis, m, z, medienEinstellung{server: auswahl, agent: agent, modell: *modell,
			upstream: *upstream, keyDatei: *keyDatei, budget: *budget, zeit: *zeit, skills: skills, maxAufrufe: max(*medienAufrufe, 1)}) {
			return
		}
	}

	// 1. Aufgabe wählen und reservieren.
	id := *nur
	if id == 0 {
		var liste []struct {
			ID int64 `json:"id"`
		}
		for _, p := range projekte {
			filter := map[string]any{"project": p}
			if *faehigkeiten != "" {
				filter["capabilities"] = strings.Split(*faehigkeiten, ",")
			}
			pruefen(m.werkzeug("matching_tasks", filter, &liste))
			if len(liste) > 0 {
				*projekt = p
				break
			}
		}
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
	// Ab hier gibt jeder Fehler die Aufgabe sofort zurück in den Pool (und landet in `swarvex history`).
	var a aufgabe
	var verbrauch map[string]float64
	abbrechen := func(grund string) {
		m.werkzeug("abandon_task", map[string]any{"task_id": id, "reason": grund}, nil)
		verlaufMerken(verlaufsDatei(), verlaufEintragAus(id, a, "returned", grund, verbrauch))
		fehlerAus(grund)
	}
	gestoppt := func() {
		if basis.Err() != nil {
			abbrechen("runner was stopped")
		}
	}
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
		m.werkzeug("report_progress", map[string]any{"task_id": id, "note": "sandbox started", "agent_program": agent.Name, "model": *modell}, nil)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if neu, err := zugriffstoken(z); err == nil {
					m.token = neu
				}
				// bisheriger Verbrauch aus dem Proxy-Protokoll, damit die Plattform laufende Arbeit mitzählt
				proxyLog, _ := exec.Command("podman", "logs", fmt.Sprintf("swarvex-proxy-%d", id)).CombinedOutput()
				m.werkzeug("report_progress", map[string]any{"task_id": id, "note": "working", "usage": verbrauchAusProxy(string(proxyLog)), "agent_program": agent.Name, "model": *modell}, nil)
			}
		}
	}()

	// 4. Sandbox mit Schlüssel-Proxy.
	zugang, err := zugangLesen(agent, *keyDatei, *upstream, *zeit)
	if err != nil {
		abbrechen(err.Error())
	}
	var protokoll string
	verbrauch, protokoll, err = sandboxAusfuehren(ctx, a, u, image, arbeitsbaum, cache, ordner, agent, skills, zugang, *modell, *budget, nil)
	stopp()
	if err := verbrauchMerken(verbrauchsDatei(), id, budgetTokensAus(verbrauch), time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, "recording usage:", err)
	}
	gestoppt()
	// Token-Budget erreicht: was der Agent bis dahin erarbeitet hat, geht als Einreichung weiter (Prüfung und Bewertung
	// entscheiden), statt verworfen zu werden; ohne Ergebnis wird die Aufgabe mit klarem Grund zurückgegeben.
	amBudget := *budget > 0 && budgetTokensAus(verbrauch) >= int64(*budget)
	if amBudget {
		fmt.Printf("Token budget reached (%d of %d, cache hits count a tenth). Submitting what the agent produced so far; raise the budget with `swarvex setup`.\n", budgetTokensAus(verbrauch), *budget)
	} else if err != nil {
		abbrechen("sandbox: " + err.Error())
	}

	// 5. Patch bilden (Git-Verzeichnis liegt außerhalb der Sandbox) und selbst prüfen.
	patch, verworfen, err := patchBilden(gitDir, arbeitsbaum, a.Pfade, u)
	if len(verworfen) > 0 {
		fmt.Printf("Discarded %d side changes outside the allowed paths (e.g. dependencies, build or import files); they never leave this machine: %s\n", len(verworfen), beispiele(verworfen))
	}
	if err != nil || strings.TrimSpace(patch) == "" {
		if amBudget {
			abbrechen(fmt.Sprintf("token budget used up (%d of %d, cache hits count a tenth; %d tokens in all) before a result; raise it with `swarvex setup`", budgetTokensAus(verbrauch), *budget, tokensAus(verbrauch)))
		}
		switch {
		case err != nil:
			abbrechen("could not build the patch: " + kuerzen(err.Error(), 300))
		case len(verworfen) > 0:
			abbrechen(fmt.Sprintf("no change in the allowed paths (%s); %d changes elsewhere were discarded: %s", strings.Join(a.Pfade, ", "), len(verworfen), beispiele(verworfen)))
		default:
			abbrechen(fmt.Sprintf("the agent finished without changing the allowed paths (%s)", strings.Join(a.Pfade, ", ")))
		}
	}
	if fehler := patchVorpruefen(patch, a.Pfade, u); len(fehler) > 0 {
		abbrechen("patch violates the task: " + strings.Join(fehler, "; "))
	}
	if zuGross := patchUmfang(patch, a.Grenzen.Zeilen, a.Grenzen.Dateien); zuGross != "" {
		abbrechen("patch too large: " + zuGross + " (the lead may split the task)")
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
		"task_id": id, "patch": patch, "summary": budgetHinweis(amBudget, *budget) + zusammenfassungAus(protokoll),
		"own_tests": kuerzen(testBericht, 19000), "usage": verbrauch, "rights_confirmed": true,
		"agent_program": agent.Name, "model": *modell,
	}
	if testErgebnis != "" {
		einreichung["own_test_result"] = testErgebnis
	}
	var ergebnis map[string]any
	pruefen(m.werkzeug("submit_result", einreichung, &ergebnis))
	// Die Plattform weist ab, ohne die Reservierung zu beenden: sofort zurückgeben, statt die Aufgabe bis zum Ablauf zu blockieren
	if abgewiesen, ok := ergebnis["rejected"].([]any); ok && len(abgewiesen) > 0 {
		gruende := make([]string, len(abgewiesen))
		for i, g := range abgewiesen {
			gruende[i] = fmt.Sprint(g)
		}
		abbrechen("the platform rejected the submission: " + strings.Join(gruende, "; "))
	}
	fmt.Println("Submitted:", ergebnis)
	verlaufMerken(verlaufsDatei(), verlaufEintragAus(id, a, "submitted", "", verbrauch))
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
			modus := os.FileMode(0o644)
			if h.Mode&0o111 != 0 {
				modus = 0o755 // Ausführbare Skripte (z. B. tools/run_tests.sh) bleiben ausführbar.
			}
			f, err := os.OpenFile(pfad, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, modus)
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
	cmd.Dir = baum // Pfade in Ausgaben immer ab der Wurzel des Baums
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
func patchBilden(gitDir, baum string, erlaubt []string, u umgebung.Umgebung) (string, []string, error) {
	// Geänderte, gelöschte und neue Dateien selbst auflisten und mit denselben Regeln wie die Plattform
	// prüfen (erlaubte Pfade, geschützte Dateien); Git bekommt nur wörtliche Pfade. Git-Globs in
	// Pfadangaben nahmen neue Dateien in neuen Ordnern bei Git 2.43 (Ubuntu 24.04, Mint 22) nicht auf.
	liste, err := git(gitDir, baum, "ls-files", "-z", "--full-name", "--modified", "--deleted", "--others", "--exclude-standard")
	if err != nil {
		return "", nil, err
	}
	gesehen := map[string]bool{}
	var nehmen, verworfen []string
	for _, p := range strings.Split(liste, "\x00") {
		if p == "" || gesehen[p] {
			continue
		}
		gesehen[p] = true
		ok := !u.Gesperrt(p)
		if ok {
			ok = false
			for _, g := range erlaubt {
				if passt(g, p) {
					ok = true
				}
			}
		}
		if ok {
			nehmen = append(nehmen, p)
		} else {
			verworfen = append(verworfen, p)
		}
	}
	for start := 0; start < len(nehmen); start += 200 {
		args := []string{"add", "-A", "--"}
		for _, p := range nehmen[start:min(start+200, len(nehmen))] {
			args = append(args, ":(literal)"+p)
		}
		if _, err := git(gitDir, baum, args...); err != nil {
			return "", verworfen, err
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

// patchUmfang: dieselbe Zählung wie die Plattform (apps/api/src/aufgaben.ts, patchPruefen): geänderte Zeilen sind
// Zeilen mit + oder - am Anfang außer den Kopfzeilen „+++ “/„--- “, Dateien die Pfade der diff-Köpfe.
// Leer: innerhalb der Grenzen (oder die Plattform nannte keine).
func patchUmfang(patch string, maxZeilen, maxDateien int) string {
	zeilen, dateien := 0, map[string]bool{}
	for _, z := range strings.Split(patch, "\n") {
		if strings.HasPrefix(z, "diff --git a/") {
			if teile := strings.SplitN(strings.TrimPrefix(z, "diff --git a/"), " b/", 2); len(teile) == 2 {
				dateien[teile[1]] = true
			}
			continue
		}
		if z == "" || (z[0] != '+' && z[0] != '-') {
			continue
		}
		if len(z) >= 4 && strings.ContainsRune("+-", rune(z[1])) && strings.ContainsRune("+-", rune(z[2])) && z[3] == ' ' {
			continue
		}
		zeilen++
	}
	var zuViel []string
	if maxZeilen > 0 && zeilen > maxZeilen {
		zuViel = append(zuViel, fmt.Sprintf("%d changed lines > %d", zeilen, maxZeilen))
	}
	if maxDateien > 0 && len(dateien) > maxDateien {
		zuViel = append(zuViel, fmt.Sprintf("%d files > %d", len(dateien), maxDateien))
	}
	return strings.Join(zuViel, ", ")
}

// passt unterstützt die Glob-Formen der Plattform: exakt, *, und ** am Ende.
func passt(muster, pfad string) bool {
	if strings.HasSuffix(muster, "/**") {
		return strings.HasPrefix(pfad, strings.TrimSuffix(muster, "**"))
	}
	ok, _ := path.Match(muster, pfad)
	return ok
}

// beispiele: die ersten Pfade einer Liste für Meldungen.
func beispiele(pfade []string) string {
	if len(pfade) > 3 {
		return strings.Join(pfade[:3], ", ") + fmt.Sprintf(" and %d more", len(pfade)-3)
	}
	return strings.Join(pfade, ", ")
}

func zufall(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// terminalSteuerung: ANSI-Escape-Sequenzen (Farben, Cursor), die Agenten in ihre Ausgabe schreiben.
var terminalSteuerung = regexp.MustCompile(`\x1b(\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|[@-Z\\-_])`)

func zusammenfassungAus(protokoll string) string {
	protokoll = terminalSteuerung.ReplaceAllString(protokoll, "")
	if len(protokoll) > 4000 {
		protokoll = strings.ToValidUTF8(protokoll[len(protokoll)-4000:], "")
	}
	return "Worked on in the sandbox. Last output of the agent:\n" + protokoll
}

// sandboxAusfuehren startet Proxy und Sandbox (rootless Podman) und gibt Verbrauch und Agenten-Ausgabe zurück.
// befehl ersetzt den Agenten (nur für Angriffstests, siehe angriff.go); nil = der gewählte Agent mit dem Auftrag.
func sandboxAusfuehren(ctx context.Context, a aufgabe, u umgebung.Umgebung, image, baum, cache, ordner string, agent agentArt, skills []string,
	zugang modellZugang, modell string, budget int, befehl []string) (map[string]float64, string, error) {
	podman := func(args ...string) (string, error) {
		aus, err := exec.CommandContext(ctx, "podman", args...).CombinedOutput()
		return strings.TrimSpace(string(aus)), err
	}
	netz, err := internesNetz(ctx)
	if err != nil {
		return nil, "", err
	}
	proxyName := fmt.Sprintf("swarvex-proxy-%d", a.ID)
	tok, proxyHosts, err := schluesselProxyStarten(ctx, proxyName, netz, agent, zugang, modell, budget)
	defer exec.Command("podman", "rm", "-f", proxyName).Run()
	if err != nil {
		return nil, "", err
	}

	auftrag := auftragstext(a, u)
	agentArgs, agentBefehl, err := agentStart(agent, ordner, "http://"+proxyName+":8080", tok, modell, auftrag, skills, baum, u.Ordner, zugang)
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
	sandboxArgs := append(append(append(containerArgs(sandboxName, baum, cache, u, rest), "--network", netz), proxyHosts...), agentArgs...)
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

	verbrauch := proxyAuswerten(proxyName, ordner)
	if ctx.Err() != nil {
		return verbrauch, string(aus), fmt.Errorf("time limit reached")
	}
	if err != nil {
		return verbrauch, string(aus), fmt.Errorf("agent exited with an error: %v", err)
	}
	return verbrauch, string(aus), nil
}

// schluesselProxyStarten: der Schlüssel-Proxy einer Aufgabe im Sandbox-Netz (und nach außen). Liefert das Wegwerf-Token
// der Sandbox und den /etc/hosts-Eintrag, unter dem sie ihn erreicht. Aufräumen: podman rm -f name.
func schluesselProxyStarten(ctx context.Context, name, netz string, agent agentArt, zugang modellZugang, modell string, budget int) (string, []string, error) {
	tok := zufall(36)
	if zugang.chatgpt() {
		tok = codexSandboxToken(tok) // Codex prüft, dass sein Zugangstoken ein gültiges JWT ist
	}
	exec.CommandContext(ctx, "podman", "rm", "-f", name).Run()
	proxyArgs := []string{"run", "-d", "--name", name, "--network", netz, "--network", "podman", "--timeout", "86400",
		"--read-only", "--cap-drop=ALL", "--security-opt", "no-new-privileges", "--memory", "256m", "--pids-limit", "64",
		"-e", "SWARVEX_UPSTREAM=" + zugang.URL, "-e", "SWARVEX_API=" + agent.API, "-e", "SWARVEX_MODEL=" + modell, "-e", "SWARVEX_TASK_TOKEN=" + tok,
		"-e", fmt.Sprintf("SWARVEX_BUDGET_TOKENS=%d", budget)}
	if zugang.Key != "" {
		proxyArgs = append(proxyArgs, "-e", "SWARVEX_UPSTREAM_KEY="+zugang.Key)
	}
	if zugang.chatgpt() {
		proxyArgs = append(proxyArgs, "-e", "SWARVEX_ACCOUNT_ID="+zugang.Konto)
	}
	if restModus != nil {
		k, _ := kontingentLesen(kontingentDatei())
		proxyArgs = append(proxyArgs, "-e", "SWARVEX_LEFTOVER="+restModus.fuer(k, time.Now()).String())
	}
	if aus, err := exec.CommandContext(ctx, "podman", append(proxyArgs, "swarvex-proxy")...).CombinedOutput(); err != nil {
		return "", nil, fmt.Errorf("starting the proxy: %s", strings.TrimSpace(string(aus)))
	}
	hosts, err := proxyHost(ctx, name)
	return tok, hosts, err
}

// proxyAuswerten: Verbrauch aus dem Protokoll des Schlüssel-Proxys, das Protokoll in den Aufgabenordner, und der
// Stand der Abo-Fenster für die nächste Entscheidung im Restkontingent-Modus und für `swarvex status`.
func proxyAuswerten(name, ordner string) map[string]float64 {
	proxyLog, _ := exec.Command("podman", "logs", name).CombinedOutput()
	os.WriteFile(filepath.Join(ordner, "proxy.log"), proxyLog, 0o644)
	if k, ok := kontingentAusProtokoll(string(proxyLog)); ok {
		if alt, ok := kontingentLesen(kontingentDatei()); ok && k.Nacht == nil {
			k.Nacht = alt.Nacht // der Beginn der Nacht bleibt, auch wenn diese Aufgabe ihn nicht neu gesetzt hat
		}
		kontingentMerken(kontingentDatei(), k)
	}
	return verbrauchAusProxy(string(proxyLog))
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
	if a.BasisCommit != "" {
		// Der Snapshot hat kein .git; ohne diese Zeile nahmen Agenten einen Commit aus den Projektdokumenten.
		fmt.Fprintf(&b, "The files are the project at commit %s (the base commit of this task; there is no .git folder).\n", a.BasisCommit)
	}
	b.WriteString("Read AGENTS.md first (if present) and follow the project's rules.\n\n")
	fmt.Fprintf(&b, "TASK: %s\n\n%s\n\n", a.Titel, a.Beschreibung)
	b.WriteString("ACCEPTANCE CRITERIA:\n")
	for _, k := range a.Akzeptanz {
		fmt.Fprintf(&b, "- %s\n", k)
	}
	if a.LetztePruefung != nil && strings.TrimSpace(a.LetztePruefung.Bericht) != "" {
		b.WriteString("\nPREVIOUS ATTEMPT: an earlier submission for this task failed the project's test pipeline. The end of its report:\n")
		b.WriteString(einruecken(strings.TrimSpace(a.LetztePruefung.Bericht)))
		b.WriteString("\nFix what comes from the change. Tests that fail outside your paths are not yours to fix: name them in your summary.\n")
	}
	if a.LetzteBewertung != nil && strings.TrimSpace(a.LetzteBewertung.Begruendung) != "" {
		fmt.Fprintf(&b, "\nLEAD FEEDBACK ON THE PREVIOUS ATTEMPT (%s):\n%s\nDo exactly what it asks; the rest of the task stays as written.\n",
			a.LetzteBewertung.Urteil, einruecken(a.LetzteBewertung.Begruendung))
	}
	// Pfade sind relativ zur Wurzel des Repos; mit dir startet der Agent in einem Unterordner und
	// legte Dateien sonst dort an (game/docs/... statt docs/...). Deshalb volle Pfade nennen.
	volle := make([]string, len(a.Pfade))
	for i, p := range a.Pfade {
		volle[i] = "/work/" + strings.TrimPrefix(p, "/")
	}
	b.WriteString("\nRULES:\n- Change or create ONLY these paths (full paths; all task paths are relative to /work, not to your start folder): " + strings.Join(volle, ", ") + "\n")
	b.WriteString("- Do not create or change any other files (no notes, logs or helper scripts in the project either).\n")
	if a.Grenzen.Zeilen > 0 && a.Grenzen.Dateien > 0 {
		fmt.Fprintf(&b, "- Keep the change within %d changed lines (added plus removed) and %d files; the platform rejects larger patches. Do the core of the task compactly rather than everything at length.\n", a.Grenzen.Zeilen, a.Grenzen.Dateien)
	}
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
// 5 gestoppt vor der Reservierung, 6 Tagesbudget verbraucht, 7 Restkontingent: gerade nichts übrig.
func dauerbetrieb(fenster zeitfenster, pause time.Duration, args []string, autoAktualisieren bool) {
	fmt.Println("Continuous mode started. Stop with Ctrl+C or `swarvex stop`.")
	if t := offeneTitel(); t != "" {
		fmt.Println("New settings to choose: " + t + " (the defaults apply until then; run: swarvex setup --new)")
	}
	// Beim Anhalten bekommt die laufende Aufgabe das Signal ebenfalls, gibt sich zurück und endet; dann ist Schluss.
	basis, aufhoeren := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer aufhoeren()
	schlafen := func(d time.Duration) {
		select {
		case <-basis.Done():
		case <-time.After(d):
		}
	}
	programmBeimStart, _ := os.Stat(os.Args[0])
	var einheitBeimStart os.FileInfo
	if os.Getenv("INVOCATION_ID") != "" { // nur als systemd-Dienst, nicht im Vordergrund
		einheitBeimStart, _ = os.Stat(dienstDatei())
	}
	var letzterUpdateCheck time.Time
	for basis.Err() == nil {
		if autoAktualisieren && time.Since(letzterUpdateCheck) > 24*time.Hour {
			letzterUpdateCheck = time.Now()
			autoUpdate()
		}
		// `swarvex budget` hat die Dienst-Einheit geändert: neu starten, damit die neuen Angaben gelten.
		if jetzt, err := os.Stat(dienstDatei()); err == nil && einheitBeimStart != nil && !jetzt.ModTime().Equal(einheitBeimStart.ModTime()) {
			fmt.Println("Settings changed; restarting the service with them.")
			systemctl("restart", "--no-block", dienstName)
			schlafen(2 * time.Minute) // systemd beendet diesen Prozess gleich
			einheitBeimStart = jetzt
			continue
		}
		// Nach `swarvex update` lädt sich der Dauerbetrieb zwischen zwei Aufgaben selbst neu (gleiche PID, der Dienst merkt nichts).
		if neueVersionInstalliert(os.Args[0], programmBeimStart) {
			fmt.Println("New runner version installed; continuing with it.")
			if err := syscall.Exec(os.Args[0], os.Args, os.Environ()); err != nil {
				fmt.Println("Could not switch to the new version:", err)
				programmBeimStart, _ = os.Stat(os.Args[0])
			}
		}
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
		case 7:
			// Das Kind hat Grund und Zeitpunkt schon genannt; aus dem gemerkten Stand ergibt sich, wie lange.
			n := time.Now().Add(30 * time.Minute)
			if k, ok := kontingentLesen(kontingentDatei()); ok && restModus != nil {
				if _, _, ab := restModus.fuer(k, time.Now()).pruefen(k.fortgeschrieben(time.Now()), time.Now()); ab.After(time.Now()) {
					n = ab
				}
			}
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

// zugangLesen: Modell-Adresse und Anmeldung aus --model-url und --model-key-file. Bei Codex mit
// ChatGPT-Anmeldung (auth.json) wird das Token bei Bedarf erneuert, damit es die ganze Aufgabe hält.
func zugangLesen(agent agentArt, keyDatei, upstream string, dauer time.Duration) (modellZugang, error) {
	upstream, _ = sandboxAdresse(upstream) // ältere Einstellungen mit localhost oder der eigenen IP
	z := modellZugang{URL: upstream}
	if keyDatei == "" {
		return z, nil
	}
	roh, err := os.ReadFile(keyDatei)
	if err != nil {
		return z, err
	}
	if !istCodexAnmeldung(roh) {
		z.Key = strings.TrimSpace(string(roh))
		return z, nil
	}
	if agent.Name != "codex" {
		return z, fmt.Errorf("a Codex sign-in (auth.json) only works with --agent codex")
	}
	abo, err := codexAboLaden(keyDatei, dauer+15*time.Minute)
	if err != nil {
		return z, err
	}
	if upstream != "" {
		abo.URL = upstream
	}
	return abo, nil
}

// codexAboDatei: --agent codex mit einer Codex-Anmeldung als Schlüsseldatei?
func codexAboDatei(agent, keyDatei string) bool {
	if agent != "codex" || keyDatei == "" {
		return false
	}
	roh, err := os.ReadFile(keyDatei)
	return err == nil && istCodexAnmeldung(roh)
}

// Restkontingent-Modus dieses Aufrufs (nil: aus); im Dauerbetrieb bestimmt er auch, wie lange gewartet wird.
var restModus *restRegeln

// verbrauchAusProxy summiert die Anfragen im Protokoll des Schlüssel-Proxys (eine JSON-Zeile je Anfrage).
func verbrauchAusProxy(protokoll string) map[string]float64 {
	verbrauch := map[string]float64{"tokens": 0, "cached_tokens": 0, "budget_tokens": 0, "requests": 0, "seconds": 0}
	for _, zeile := range strings.Split(protokoll, "\n") {
		var e struct {
			Art         string   `json:"type"`
			Tokens      float64  `json:"tokens"`
			Cached      float64  `json:"cached"`
			FuersBudget *float64 `json:"budget_tokens"`
			Ms          float64  `json:"ms"`
		}
		if json.Unmarshal([]byte(zeile), &e) == nil && e.Art == "request" {
			verbrauch["tokens"] += e.Tokens
			verbrauch["cached_tokens"] += e.Cached
			if e.FuersBudget != nil {
				verbrauch["budget_tokens"] += *e.FuersBudget
			} else {
				verbrauch["budget_tokens"] += e.Tokens // Proxy einer älteren Version
			}
			verbrauch["requests"]++
			verbrauch["seconds"] += e.Ms / 1000
		}
	}
	return verbrauch
}

// budgetHinweis stellt einer Einreichung voran, dass der Agent am Token-Budget gestoppt wurde.
func budgetHinweis(amBudget bool, budget int) string {
	if !amBudget {
		return ""
	}
	return fmt.Sprintf("Note: the agent was stopped at the token budget (%d tokens); the result may be incomplete.\n\n", budget)
}
