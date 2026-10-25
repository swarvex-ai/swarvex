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
	"path"
	"path/filepath"
	"strings"
	"time"
)

type aufgabe struct {
	ID           int64    `json:"id"`
	Titel        string   `json:"titel"`
	Beschreibung string   `json:"beschreibung"`
	Stufe        string   `json:"stufe"`
	Pfade        []string `json:"pfade"`
	Akzeptanz    []string `json:"akzeptanz"`
	BasisCommit  string   `json:"basis_commit"`
	Projekt      string   `json:"projekt"`
	Engine       string   `json:"engine"`
	Grenzen      struct {
		Zeilen  int `json:"zeilen"`
		Dateien int `json:"dateien"`
	} `json:"grenzen"`
}

func arbeitenBefehl(args []string) {
	fs := flag.NewFlagSet("arbeiten", flag.ExitOnError)
	projekt := fs.String("projekt", "", "Projekt-Slug")
	nur := fs.Int64("aufgabe", 0, "bestimmte Aufgabe (sonst die erste passende)")
	modell := fs.String("modell", "", "Modellname beim Anbieter")
	upstream := fs.String("modell-url", "", "OpenAI-kompatible Basis-URL des Modells (ohne /v1)")
	keyDatei := fs.String("modell-key-datei", "", "Datei mit dem API-Schlüssel des Modells (optional)")
	budget := fs.Int("budget-tokens", 400000, "Token-Budget für diese Aufgabe")
	zeit := fs.Duration("zeitlimit", 90*time.Minute, "maximale Laufzeit der Sandbox")
	faehigkeiten := fs.String("faehigkeiten", "", "kommagetrennt, z. B. gdscript,godot-4,headless-test")
	fensterText := fs.String("zeitfenster", "", `wann gearbeitet werden darf, z. B. "mo-fr 18:00-23:00; sa,so 09:00-24:00" oder "täglich 22:00-07:00" (leer = immer)`)
	amEnde := fs.String("am-fensterende", "stopp", "stopp: Sandbox endet spätestens mit dem Zeitfenster; fertig: laufende Aufgabe darf fertig werden")
	mindest := fs.Duration("mindestzeit", 15*time.Minute, "keine neue Aufgabe beginnen, wenn das Zeitfenster vorher endet (nur bei stopp)")
	dauer := fs.Bool("dauerbetrieb", false, "laufend Aufgaben holen; außerhalb der Zeitfenster bis zum nächsten Fenster warten")
	pause := fs.Duration("pause", 5*time.Minute, "Wartezeit im Dauerbetrieb, wenn keine passende Aufgabe offen ist")
	fs.Parse(args)
	if *projekt == "" || *modell == "" || *upstream == "" {
		fehlerAus("--projekt, --modell und --modell-url sind nötig")
	}
	fenster, err := zeitfensterLesen(*fensterText)
	pruefen(err)
	if *amEnde != "stopp" && *amEnde != "fertig" {
		fehlerAus("--am-fensterende kennt nur stopp oder fertig")
	}
	if *dauer {
		dauerbetrieb(fenster, *pause, ohneDauerbetrieb(args))
		return
	}

	// Zeitfenster: außerhalb wird keine Aufgabe reserviert (Exit-Code 4). Bei „stopp“ endet die Sandbox
	// spätestens mit dem Fenster; reicht die Restzeit nicht für eine sinnvolle Aufgabe, beginnt keine neue.
	if !fenster.offen(time.Now()) {
		if n, ok := fenster.naechsterBeginn(time.Now()); ok {
			fmt.Printf("Außerhalb der Zeitfenster. Nächstes Fenster ab %s.\n", zeitpunkt(n))
		}
		os.Exit(4)
	}
	if *amEnde == "stopp" && len(fenster) > 0 {
		rest := fenster.verbleibend(time.Now())
		if rest < *mindest {
			fmt.Printf("Zeitfenster endet in %s, zu kurz für eine neue Aufgabe (--mindestzeit %s).\n", rest, *mindest)
			os.Exit(4)
		}
		if *zeit > rest {
			*zeit = rest
			fmt.Printf("Zeitlimit auf das Ende des Zeitfensters gekürzt: %s.\n", rest.Round(time.Minute))
		}
	}

	z, err := zugangLaden()
	pruefen(err)
	token, err := zugriffstoken(z)
	pruefen(err)
	m := &mcpClient{basis: z.Plattform, token: token, http: httpClient(z.CAPfad)}
	pruefen(m.initialisieren())

	// 1. Aufgabe wählen und reservieren.
	id := *nur
	if id == 0 {
		var liste []struct {
			ID int64 `json:"id"`
		}
		filter := map[string]any{"projekt": *projekt}
		if *faehigkeiten != "" {
			filter["faehigkeiten"] = strings.Split(*faehigkeiten, ",")
		}
		pruefen(m.werkzeug("aufgaben_passend", filter, &liste))
		if len(liste) == 0 {
			fmt.Println("Keine passende offene Aufgabe.")
			os.Exit(3) // Dauerbetrieb wartet dann und fragt später wieder
		}
		id = liste[0].ID
	}
	var lease map[string]any
	pruefen(m.werkzeug("aufgabe_reservieren", map[string]any{"aufgabe_id": id}, &lease))
	// Ab hier gibt jeder Fehler die Aufgabe sofort zurück in den Pool.
	abbrechen := func(grund string) {
		m.werkzeug("abbruch_melden", map[string]any{"aufgabe_id": id, "grund": grund}, nil)
		fehlerAus(grund)
	}
	var a aufgabe
	if err := m.werkzeug("aufgabe_details", map[string]any{"aufgabe_id": id}, &a); err != nil {
		abbrechen("Aufgabe lesen: " + err.Error())
	}
	fmt.Printf("Aufgabe %d reserviert: %s\n", a.ID, a.Titel)

	// 2. Snapshot holen und entpacken (ohne .git, ohne Symlinks, ohne Pfade nach außen).
	ordner := filepath.Join(datenOrdner(), "arbeit", fmt.Sprint(id))
	os.RemoveAll(ordner)
	arbeitsbaum := filepath.Join(ordner, "baum")
	gitDir := filepath.Join(ordner, "git") // außerhalb des Sandbox-Mounts
	pruefen(os.MkdirAll(arbeitsbaum, 0o755))
	if err := snapshotHolen(z, token, id, arbeitsbaum); err != nil {
		abbrechen("Snapshot: " + err.Error())
	}
	// LFS-Dateien aufgelöst, aber nicht in Git aufgenommen: Patches ändern keine Binärassets
	// (dafür gibt es Asset-Aufträge); so muss Git die großen Dateien auch nicht kopieren.
	lfsPfade, err := lfsAufloesen(z, token, id, arbeitsbaum)
	if err != nil {
		abbrechen("LFS: " + err.Error())
	}
	if err := gitBasis(gitDir, arbeitsbaum, lfsPfade); err != nil {
		abbrechen("Basis-Commit: " + err.Error())
	}

	// 3. Lebenszeichen, solange gearbeitet wird.
	ctx, stopp := context.WithTimeout(context.Background(), *zeit)
	defer stopp()
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		m.werkzeug("fortschritt_melden", map[string]any{"aufgabe_id": id, "notiz": "Sandbox gestartet"}, nil)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if neu, err := zugriffstoken(z); err == nil {
					m.token = neu
				}
				m.werkzeug("fortschritt_melden", map[string]any{"aufgabe_id": id, "notiz": "arbeitet"}, nil)
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
	verbrauch, protokoll, err := sandboxAusfuehren(ctx, a, arbeitsbaum, ordner, *upstream, key, *modell, *budget, nil)
	stopp()
	if err != nil {
		abbrechen("Sandbox: " + err.Error())
	}

	// 5. Patch bilden (Git-Verzeichnis liegt außerhalb der Sandbox) und selbst prüfen.
	patch, verworfen, err := patchBilden(gitDir, arbeitsbaum, a.Pfade)
	if verworfen > 0 {
		fmt.Printf("%d Nebenänderungen außerhalb der erlaubten Pfade verworfen (z. B. Godot-Importdateien); sie verlassen das Gerät nicht.\n", verworfen)
	}
	if err != nil || strings.TrimSpace(patch) == "" {
		abbrechen("kein verwertbares Ergebnis der Sandbox")
	}
	if fehler := patchVorpruefen(patch, a.Pfade); len(fehler) > 0 {
		abbrechen("Patch verletzt die Aufgabe: " + strings.Join(fehler, "; "))
	}

	// 6. Einreichen (mit Beitragserklärung).
	if neu, err := zugriffstoken(z); err == nil {
		m.token = neu
	}
	var ergebnis map[string]any
	pruefen(m.werkzeug("ergebnis_einreichen", map[string]any{
		"aufgabe_id": id, "patch": patch, "zusammenfassung": zusammenfassungAus(protokoll),
		"eigene_tests": "siehe Sandbox-Protokoll des Agenten", "verbrauch": verbrauch, "rechte_versichert": true,
	}, &ergebnis))
	fmt.Println("Eingereicht:", ergebnis)
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
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/aufgaben/%d/snapshot", z.Plattform, id), nil)
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
func patchBilden(gitDir, baum string, erlaubt []string) (string, int, error) {
	args := []string{"add", "-A", "--"}
	for _, g := range erlaubt {
		args = append(args, ":(glob)"+g)
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
func patchVorpruefen(patch string, erlaubt []string) []string {
	var fehler []string
	for _, zeile := range strings.Split(patch, "\n") {
		if !strings.HasPrefix(zeile, "diff --git a/") {
			continue
		}
		teile := strings.SplitN(strings.TrimPrefix(zeile, "diff --git a/"), " b/", 2)
		for _, p := range teile {
			ok := false
			for _, g := range erlaubt {
				if passt(g, p) {
					ok = true
				}
			}
			if !ok {
				fehler = append(fehler, "außerhalb der erlaubten Pfade: "+p)
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
	return "Bearbeitet in der Sandbox. Letzte Ausgabe des Agenten:\n" + protokoll
}

// sandboxAusfuehren startet Proxy und Sandbox (rootless Podman) und gibt Verbrauch und Agenten-Ausgabe zurück.
// befehl ersetzt den Agenten (nur für Angriffstests, siehe angriff.go); nil = opencode mit dem Auftrag.
func sandboxAusfuehren(ctx context.Context, a aufgabe, baum, ordner, upstream, key, modell string, budget int, befehl []string) (map[string]float64, string, error) {
	podman := func(args ...string) (string, error) {
		aus, err := exec.CommandContext(ctx, "podman", args...).CombinedOutput()
		return strings.TrimSpace(string(aus)), err
	}
	netz := "swarvex-intern"
	if _, err := podman("network", "exists", netz); err != nil {
		if aus, err := podman("network", "create", "--internal", netz); err != nil {
			return nil, "", fmt.Errorf("Netz anlegen: %s", aus)
		}
	}
	tok := zufall(36)
	proxyName := fmt.Sprintf("swarvex-proxy-%d", a.ID)
	podman("rm", "-f", proxyName)
	proxyArgs := []string{"run", "-d", "--name", proxyName, "--network", netz, "--network", "podman", "--timeout", "86400",
		"--read-only", "--cap-drop=ALL", "--security-opt", "no-new-privileges", "--memory", "256m", "--pids-limit", "64",
		"-e", "SWARVEX_UPSTREAM=" + upstream, "-e", "SWARVEX_MODELL=" + modell, "-e", "SWARVEX_AUFGABEN_TOKEN=" + tok,
		"-e", fmt.Sprintf("SWARVEX_BUDGET_TOKENS=%d", budget)}
	if key != "" {
		proxyArgs = append(proxyArgs, "-e", "SWARVEX_UPSTREAM_KEY="+key)
	}
	if aus, err := podman(append(proxyArgs, "swarvex-proxy")...); err != nil {
		return nil, "", fmt.Errorf("Proxy starten: %s", aus)
	}
	defer exec.Command("podman", "rm", "-f", proxyName).Run()

	konfig := map[string]any{
		"$schema": "https://opencode.ai/config.json", "autoupdate": false, "share": "disabled",
		"model": "swarvex/" + modell,
		"provider": map[string]any{"swarvex": map[string]any{
			"npm": "@ai-sdk/openai-compatible", "name": "Schwarm-Proxy",
			"options": map[string]any{"baseURL": fmt.Sprintf("http://%s:8080/v1", proxyName), "apiKey": "{env:SWARVEX_TOKEN}"},
			"models":  map[string]any{modell: map[string]any{"name": modell, "tool_call": true}},
		}},
	}
	konfigPfad := filepath.Join(ordner, "opencode.json")
	roh, _ := json.MarshalIndent(konfig, "", "  ")
	if err := os.WriteFile(konfigPfad, roh, 0o644); err != nil {
		return nil, "", err
	}

	auftrag := auftragstext(a)
	sandboxName := fmt.Sprintf("swarvex-sandbox-%d", a.ID)
	podman("rm", "-f", sandboxName)
	rest := 3600
	if frist, ok := ctx.Deadline(); ok {
		rest = max(int(time.Until(frist).Seconds()), 1)
	}
	sandboxArgs := []string{"run", "--rm", "--name", sandboxName, "--network", netz, "--timeout", fmt.Sprint(rest),
		"--cap-drop=ALL", "--security-opt", "no-new-privileges", "--read-only",
		"--tmpfs", "/tmp:rw,exec,size=1g,mode=1777", "--tmpfs", "/home/agent:rw,exec,size=1g,mode=1777",
		"--userns=keep-id:uid=1000,gid=1000", "--memory", "4g", "--cpus", "3", "--pids-limit", "512",
		"-v", baum + ":/work:Z", "-v", konfigPfad + ":/konfig/opencode.json:ro,Z",
		"-e", "OPENCODE_CONFIG=/konfig/opencode.json", "-e", "SWARVEX_TOKEN=" + tok,
		"-e", "SWARVEX_PROXY=http://" + proxyName + ":8080",
		"-e", "OPENCODE_DISABLE_AUTOUPDATE=1", "-e", "OPENCODE_DISABLE_MODELS_FETCH=1", "swarvex-sandbox"}
	if befehl == nil {
		befehl = []string{"opencode", "run", "--pure", "--auto", "-m", "swarvex/" + modell, auftrag}
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
	verbrauch := map[string]float64{"tokens": 0, "anfragen": 0, "sekunden": 0}
	for _, zeile := range strings.Split(string(proxyLog), "\n") {
		var e struct {
			Art    string  `json:"art"`
			Tokens float64 `json:"tokens"`
			Ms     float64 `json:"ms"`
		}
		if json.Unmarshal([]byte(zeile), &e) == nil && e.Art == "anfrage" {
			verbrauch["tokens"] += e.Tokens
			verbrauch["anfragen"]++
			verbrauch["sekunden"] += e.Ms / 1000
		}
	}
	if ctx.Err() != nil {
		return verbrauch, string(aus), fmt.Errorf("Zeitlimit erreicht")
	}
	if err != nil {
		return verbrauch, string(aus), fmt.Errorf("Agent beendet mit Fehler: %v", err)
	}
	return verbrauch, string(aus), nil
}

func auftragstext(a aufgabe) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Du arbeitest als Mitstreiter an einem Open-Source-Spiel (%s). Das Projekt liegt in /work (Godot-Projektwurzel: /work/game, `godot` ist installiert).\n\n", a.Engine)
	fmt.Fprintf(&b, "AUFGABE: %s\n\n%s\n\n", a.Titel, a.Beschreibung)
	b.WriteString("AKZEPTANZKRITERIEN:\n")
	for _, k := range a.Akzeptanz {
		fmt.Fprintf(&b, "- %s\n", k)
	}
	b.WriteString("\nREGELN:\n- Ändere oder erstelle NUR diese Pfade: " + strings.Join(a.Pfade, ", ") + "\n")
	b.WriteString("- Keine anderen Dateien anlegen oder ändern (auch keine Notizen, Logs oder Hilfsskripte im Projekt).\n")
	b.WriteString("- Führe die Tests aus den Akzeptanzkriterien selbst aus und behebe Fehler, bis sie grün sind.\n")
	b.WriteString("- Es gibt kein Internet. Beende die Arbeit mit einer kurzen Zusammenfassung, was du getan hast und wie die Tests ausgingen.\n")
	return b.String()
}

// dauerbetrieb holt in den offenen Zeitfenstern laufend Aufgaben. Jede Aufgabe läuft als eigener
// Kindprozess („arbeiten“ ohne --dauerbetrieb), damit ein Fehler nicht den ganzen Runner beendet.
// Exit-Codes des Kindes: 0 erledigt, 3 keine passende Aufgabe, 4 außerhalb bzw. zu kurz vor Fensterende.
func dauerbetrieb(fenster zeitfenster, pause time.Duration, args []string) {
	fmt.Println("Dauerbetrieb gestartet. Anhalten mit Strg+C.")
	for {
		jetzt := time.Now()
		if !fenster.offen(jetzt) {
			n, ok := fenster.naechsterBeginn(jetzt)
			if !ok {
				fehlerAus("Die Zeitfenster öffnen nie.")
			}
			fmt.Printf("Außerhalb der Zeitfenster. Warte bis %s.\n", zeitpunkt(n))
			// in Stücken schlafen, damit Uhrumstellung oder Ruhezustand nicht zu Fehlzeiten führen
			for time.Now().Before(n) {
				time.Sleep(min(time.Until(n), 10*time.Minute))
			}
			continue
		}
		kind := exec.Command(os.Args[0], append([]string{"arbeiten"}, args...)...)
		kind.Stdout, kind.Stderr = os.Stdout, os.Stderr
		code := 0
		if err := kind.Run(); err != nil {
			code = 1
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			}
		}
		switch code {
		case 0:
			continue // gleich die nächste Aufgabe
		case 3:
			fmt.Printf("Keine passende Aufgabe. Nächster Versuch in %s.\n", pause)
		case 4:
			// Fenster zu oder zu kurz: die Schleife wartet oben auf das nächste
		default:
			fmt.Printf("Aufgabe mit Fehler beendet (Code %d). Nächster Versuch in %s.\n", code, pause)
		}
		if code != 4 {
			time.Sleep(pause)
		} else {
			time.Sleep(time.Minute)
		}
	}
}

// ohneDauerbetrieb entfernt --dauerbetrieb aus den Argumenten für den Kindprozess.
func ohneDauerbetrieb(args []string) []string {
	var r []string
	for _, a := range args {
		switch strings.TrimLeft(a, "-") {
		case "dauerbetrieb", "dauerbetrieb=true", "dauerbetrieb=1":
			continue
		}
		r = append(r, a)
	}
	return r
}
