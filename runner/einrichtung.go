package main

// Einrichtung ohne Doku:
//
//	swarvex setup     fragt Modell, Projekt und Betriebsart ab, mit Vorschlägen aus dem, was schon da ist
//	                  (lokale Modell-Server, Codex-Anmeldung, Claude Code, API-Schlüssel in der Umgebung),
//	                  und speichert die Angaben (work.json); danach reichen `swarvex work` und `swarvex service`.
//	swarvex connect   trägt swarvex bei allen gefundenen Agenten als MCP-Server ein (Aufgaben ohne Code).
//	swarvex probe <url>  intern: prüft im Proxy-Image, ob eine Adresse aus dem Container erreichbar ist.

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const standardPlattform = "https://swarvex.ai"

// ---- Fragen im Terminal (auch nach `curl … | sh`: dann über /dev/tty) ----

type frager struct {
	r   *bufio.Reader
	tty *os.File
}

func neuerFrager() (*frager, bool) {
	if st, err := os.Stdin.Stat(); err == nil && st.Mode()&os.ModeCharDevice != 0 {
		return &frager{r: bufio.NewReader(os.Stdin), tty: os.Stdin}, true
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, false
	}
	return &frager{r: bufio.NewReader(tty), tty: tty}, true
}

func (f *frager) zeile() string {
	s, _ := f.r.ReadString('\n')
	return strings.TrimSpace(s)
}

func (f *frager) frage(text, vorschlag string) string {
	if vorschlag != "" {
		fmt.Printf("%s [%s]: ", text, vorschlag)
	} else {
		fmt.Printf("%s: ", text)
	}
	if a := f.zeile(); a != "" {
		return a
	}
	return vorschlag
}

func (f *frager) jaNein(text string, standard bool) bool {
	hinweis := "Y/n"
	if !standard {
		hinweis = "y/N"
	}
	fmt.Printf("%s [%s] ", text, hinweis)
	switch strings.ToLower(f.zeile()) {
	case "":
		return standard
	case "y", "yes", "j", "ja":
		return true
	}
	return false
}

// wahl zeigt nummerierte Optionen; Enter nimmt die erste.
func (f *frager) wahl(text string, optionen []string) int {
	fmt.Println(text)
	for i, o := range optionen {
		fmt.Printf("  %d) %s\n", i+1, o)
	}
	for {
		a := f.frage("Choose", "1")
		var n int
		if _, err := fmt.Sscan(a, &n); err == nil && n >= 1 && n <= len(optionen) {
			return n - 1
		}
		fmt.Println("  Please enter a number from the list.")
	}
}

// geheim liest ohne Echo (Schlüssel, Token).
func (f *frager) geheim(text string) string {
	fmt.Print(text + ": ")
	aus := exec.Command("stty", "-echo")
	aus.Stdin = f.tty
	if aus.Run() == nil {
		defer func() { an := exec.Command("stty", "echo"); an.Stdin = f.tty; an.Run(); fmt.Println() }()
	}
	return f.zeile()
}

// ---- gespeicherte Angaben ----

func einstellungenDatei() string { return filepath.Join(konfigOrdner(), "work.json") }

func gespeicherteArgs() []string {
	roh, err := os.ReadFile(einstellungenDatei())
	if err != nil {
		return nil
	}
	var e struct {
		Args []string `json:"args"`
	}
	if json.Unmarshal(roh, &e) != nil {
		return nil
	}
	return e.Args
}

func argsSpeichern(args []string) error {
	roh, _ := json.MarshalIndent(map[string]any{"args": args, "saved": time.Now().Format(time.RFC3339)}, "", "  ")
	if err := os.MkdirAll(konfigOrdner(), 0o700); err != nil {
		return err
	}
	return os.WriteFile(einstellungenDatei(), roh, 0o600)
}

// mitGespeicherten ergänzt Aufrufe ohne --project um die gespeicherten Angaben (z. B. `swarvex work --continuous`).
// Ohne gespeicherte Angaben startet im Terminal die Einrichtung.
func mitGespeicherten(args []string) []string {
	for _, a := range args {
		if n, _, _ := strings.Cut(strings.TrimLeft(a, "-"), "="); n == "project" && strings.HasPrefix(a, "-") {
			return args
		}
	}
	if g := gespeicherteArgs(); g != nil {
		return append(append([]string{}, g...), args...)
	}
	if _, ok := neuerFrager(); ok && len(args) == 0 {
		fmt.Println("No settings yet, starting the setup.")
		return setupDialog(false)
	}
	fehlerAus("No settings yet: run `swarvex setup` (or pass --project, --model and --model-url).")
	return nil
}

// ---- Modelle erkennen ----

type modellWahl struct {
	Titel  string
	Agent  string
	Modell string
	URL    string
	Key    string // Datei
	Lokal  bool
}

func holeJSON(adresse string, ziel any) bool {
	c := &http.Client{Timeout: 700 * time.Millisecond}
	r, err := c.Get(adresse)
	if err != nil {
		return false
	}
	defer r.Body.Close()
	return r.StatusCode == 200 && json.NewDecoder(r.Body).Decode(ziel) == nil
}

// lokaleModelle fragt die üblichen lokalen Server ab. Aus dem Container sind sie über host.containers.internal erreichbar.
func lokaleModelle() []modellWahl {
	var w []modellWahl
	var ollama struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if holeJSON("http://127.0.0.1:11434/api/tags", &ollama) {
		for _, m := range ollama.Models {
			w = append(w, modellWahl{Titel: "Ollama: " + m.Name, Agent: "opencode", Modell: m.Name, URL: "http://host.containers.internal:11434", Lokal: true})
		}
	}
	for _, s := range []struct{ name, port string }{{"LM Studio", "1234"}, {"llama.cpp", "8080"}, {"vLLM", "8000"}} {
		var liste struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if holeJSON("http://127.0.0.1:"+s.port+"/v1/models", &liste) {
			for _, m := range liste.Data {
				w = append(w, modellWahl{Titel: s.name + ": " + m.ID, Agent: "opencode", Modell: m.ID, URL: "http://host.containers.internal:" + s.port, Lokal: true})
			}
		}
	}
	return w
}

func heim(teile ...string) string {
	h, _ := os.UserHomeDir()
	return filepath.Join(append([]string{h}, teile...)...)
}

// codexModell liest das Modell aus der Codex-Konfiguration (model = "…"), falls gesetzt.
func codexModell() string {
	roh, err := os.ReadFile(heim(".codex", "config.toml"))
	if err != nil {
		return ""
	}
	if m := regexp.MustCompile(`(?m)^\s*model\s*=\s*"([^"]+)"`).FindSubmatch(roh); m != nil {
		return string(m[1])
	}
	return ""
}

func modellAuswahl() []modellWahl {
	w := lokaleModelle()
	if roh, err := os.ReadFile(heim(".codex", "auth.json")); err == nil && istCodexAnmeldung(roh) {
		w = append(w, modellWahl{Titel: "Codex with your ChatGPT sign-in (~/.codex/auth.json)", Agent: "codex", Modell: codexModell(), Key: heim(".codex", "auth.json")})
	}
	if _, err := exec.LookPath("claude"); err == nil {
		w = append(w, modellWahl{Titel: "Claude Code with your Claude subscription (token from `claude setup-token`)", Agent: "claude", Modell: "claude-sonnet-5-5", URL: "https://api.anthropic.com", Key: "claude-setup-token"})
	}
	for _, e := range []struct{ env, titel, agent, modell, adresse string }{
		{"ANTHROPIC_API_KEY", "Anthropic API key from ANTHROPIC_API_KEY", "claude", "claude-sonnet-5-5", "https://api.anthropic.com"},
		{"OPENAI_API_KEY", "OpenAI API key from OPENAI_API_KEY", "codex", "", "https://api.openai.com"},
		{"OPENROUTER_API_KEY", "OpenRouter key from OPENROUTER_API_KEY", "opencode", "", "https://openrouter.ai/api"},
	} {
		if os.Getenv(e.env) != "" {
			w = append(w, modellWahl{Titel: e.titel, Agent: e.agent, Modell: e.modell, URL: e.adresse, Key: "env:" + e.env})
		}
	}
	return append(w, modellWahl{Titel: "Something else (enter address, model and key yourself)"})
}

// schluesselDatei legt einen eingegebenen oder aus der Umgebung übernommenen Schlüssel als Datei 0600 ab.
func schluesselDatei(name, wert string) string {
	pfad := filepath.Join(konfigOrdner(), name)
	pruefen(os.MkdirAll(konfigOrdner(), 0o700))
	pruefen(os.WriteFile(pfad, []byte(strings.TrimSpace(wert)+"\n"), 0o600))
	return pfad
}

// erreichbar prüft im Proxy-Image (gleiches Netz wie der echte Proxy), ob die Modell-Adresse antwortet.
func erreichbar(adresse string) (bool, string) {
	if _, err := exec.LookPath("podman"); err != nil {
		return true, "podman missing, not checked"
	}
	aus, err := exec.Command("podman", "run", "--rm", "--network", "podman", "--entrypoint", "/swarvex", "swarvex-proxy", "probe", adresse).CombinedOutput()
	return err == nil, strings.TrimSpace(string(aus))
}

func probeBefehl(args []string) {
	if len(args) != 1 {
		fehlerAus("usage: swarvex probe <url>")
	}
	c := &http.Client{Timeout: 5 * time.Second}
	r, err := c.Get(args[0])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	r.Body.Close()
	fmt.Println("HTTP", r.StatusCode)
}

// ---- Projekte ----

type projektInfo struct {
	Projekt string `json:"project"`
	Name    string `json:"name"`
	Offen   int    `json:"tasks_open"`
}

func projekteLaden(plattform, ca string) []projektInfo {
	r, err := httpClient(ca).Get(plattform + "/api/projects")
	if err != nil {
		return nil
	}
	defer r.Body.Close()
	var l struct {
		Projekte []projektInfo `json:"projects"`
	}
	json.NewDecoder(r.Body).Decode(&l)
	sort.SliceStable(l.Projekte, func(i, j int) bool { return l.Projekte[i].Offen > l.Projekte[j].Offen })
	return l.Projekte
}

// projekteFuer: "auto" heißt alle Projekte mit offenen Aufgaben, die meisten zuerst.
func projekteFuer(projekt string, z *zugangsdaten) []string {
	if projekt != "auto" {
		return []string{projekt}
	}
	var r []string
	for _, p := range projekteLaden(z.Plattform, z.CAPfad) {
		if p.Offen > 0 {
			r = append(r, p.Projekt)
		}
	}
	return r
}

// ---- swarvex setup ----

func setupBefehl(args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	fs.Parse(args)
	setupDialog(true)
}

// setupDialog fragt alles ab, speichert und liefert die Angaben für `work`. ausfuehren: danach gleich starten oder Dienst einrichten.
func setupDialog(ausfuehren bool) []string {
	f, ok := neuerFrager()
	if !ok {
		fehlerAus("The setup needs a terminal. Run `swarvex setup` in a terminal.")
	}
	z, err := zugangLaden()
	if err != nil {
		fmt.Println("First sign in at swarvex (opens your browser).")
		loginBefehl([]string{"--platform", standardPlattform})
		z, err = zugangLaden()
		pruefen(err)
	}

	fmt.Println("\n== 1/4 Model\nYour agent works with a model of your choice. Your key stays on this machine; the sandbox never sees it.")
	optionen := modellAuswahl()
	titel := make([]string, len(optionen))
	for i, o := range optionen {
		titel[i] = o.Titel
	}
	w := optionen[f.wahl("Which model should your agent use?", titel)]
	switch {
	case w.Key == "claude-setup-token":
		fmt.Println("Run `claude setup-token` in another terminal and paste the token here (input is hidden).")
		w.Key = schluesselDatei("claude-token", f.geheim("Token"))
	case strings.HasPrefix(w.Key, "env:"):
		w.Key = schluesselDatei(strings.ToLower(strings.TrimPrefix(w.Key, "env:")), os.Getenv(strings.TrimPrefix(w.Key, "env:")))
	case w.Titel == optionen[len(optionen)-1].Titel:
		w.Agent = []string{"opencode", "claude", "codex"}[f.wahl("Which interface does your model offer?",
			[]string{"OpenAI-compatible (Ollama, OpenRouter, most providers) → agent opencode", "Anthropic → agent Claude Code", "OpenAI Responses → agent Codex"})]
		w.URL = f.frage("Address of the model API without /v1 (local server: http://host.containers.internal:<port>)", "")
		if k := f.geheim("API key (empty if none)"); k != "" {
			w.Key = schluesselDatei(w.Agent+"-key", k)
		}
	}
	w.Modell = f.frage("Model name", w.Modell)
	for w.Modell == "" {
		w.Modell = f.frage("Model name (required)", "")
	}
	if w.URL != "" {
		fmt.Print("Checking that the sandbox can reach the model … ")
		if ok, aus := erreichbar(w.URL); ok {
			fmt.Println("ok")
		} else {
			fmt.Println("NOT reachable (" + aus + ")")
			if w.Lokal {
				fmt.Println("  Your model server probably only listens on localhost. The sandbox reaches it through the container")
				fmt.Println("  network, so it has to listen there too (Ollama: OLLAMA_HOST=0.0.0.0, best behind your firewall).")
			}
			if !f.jaNein("Keep this model anyway?", false) {
				fehlerAus("Setup cancelled; run `swarvex setup` again when the model is reachable.")
			}
		}
	}

	fmt.Println("\n== 2/4 Project")
	projekte := projekteLaden(z.Plattform, z.CAPfad)
	pTitel := []string{"Any project, wherever most is open (recommended)"}
	for _, p := range projekte {
		pTitel = append(pTitel, fmt.Sprintf("%s (%s, %d open tasks)", p.Name, p.Projekt, p.Offen))
	}
	projekt := "auto"
	if i := f.wahl("Where should your agent help?", pTitel); i > 0 {
		projekt = projekte[i-1].Projekt
	}

	args := []string{"--project", projekt, "--agent", w.Agent, "--model", w.Modell}
	if w.URL != "" {
		args = append(args, "--model-url", w.URL)
	}
	if w.Key != "" {
		args = append(args, "--model-key-file", w.Key)
	}

	fmt.Println("\n== 3/4 When")
	modus := f.wahl("How should it run?", []string{"Try one task now", "As a service in the background, within time windows", "Only save, I start it myself with `swarvex work`"})
	var dienstArgs []string
	if modus == 1 {
		fenster := f.frage("When may it work? (e.g. daily 18:00-23:00, mon-fri 09:00-17:00, always)", "daily 18:00-23:00")
		if fenster != "always" {
			_, err := zeitfensterLesen(fenster)
			for err != nil {
				fmt.Println("  " + err.Error())
				fenster = f.frage("When may it work?", "daily 18:00-23:00")
				_, err = zeitfensterLesen(fenster)
			}
			dienstArgs = append(dienstArgs, "--schedule", fenster)
		}
		vorschlag := "2000000"
		if w.Lokal {
			vorschlag = "0"
		}
		dienstArgs = append(dienstArgs, "--daily-budget-tokens", f.frage("Tokens per day at most (0 = no limit)", vorschlag))
	}
	pruefen(argsSpeichern(args))
	fmt.Println("Saved in", einstellungenDatei(), "(change any time with `swarvex setup`).")

	fmt.Println("\n== 4/4 Tasks without code")
	if gefunden := mcpAgentenGefunden(); len(gefunden) > 0 {
		namen := make([]string, len(gefunden))
		for i, a := range gefunden {
			namen[i] = a.Titel
		}
		if f.jaNein("Also connect "+strings.Join(namen, ", ")+" directly (design, texts, reviews; no code runs on your machine)?", true) {
			verbinden(z.Plattform, gefunden)
		}
	} else {
		fmt.Println("No other agent found (Claude Code, Codex, Hermes, opencode, OpenClaw, Gemini CLI). Later: swarvex connect")
	}

	if !ausfuehren {
		return args
	}
	switch modus {
	case 0:
		fmt.Println("\nStarting one task …")
		arbeitenBefehl(args)
	case 1:
		dienstBefehl(append(append([]string{}, args...), dienstArgs...))
	default:
		fmt.Println("\nDone. Start with:  swarvex work   (one task)  or  swarvex service  (in the background)")
	}
	return args
}

// ---- swarvex connect ----

type mcpAgent struct {
	Titel, Programm string
	Hinzufuegen     func(mcpURL string) []string // Befehl ohne Programmnamen; nil = Konfigurationsdatei (opencode)
	Anmelden        string                       // was danach zu tun ist
}

var mcpAgenten = []mcpAgent{
	{"Claude Code", "claude", func(u string) []string {
		return []string{"mcp", "add", "--scope", "user", "--transport", "http", "swarvex", u}
	}, "in a session: /mcp → swarvex → Authenticate"},
	{"Codex", "codex", func(u string) []string { return []string{"mcp", "add", "swarvex", "--url", u} }, "sign-in starts right away; later: codex mcp login swarvex"},
	{"Hermes", "hermes", func(u string) []string { return []string{"mcp", "add", "swarvex", "--url", u, "--auth", "oauth"} }, "sign-in starts right away; later: hermes mcp login swarvex"},
	{"opencode", "opencode", nil, "opencode mcp auth swarvex"},
	{"OpenClaw", "openclaw", func(u string) []string {
		return []string{"mcp", "add", "swarvex", "--url", u, "--transport", "streamable-http", "--auth", "oauth"}
	}, "openclaw mcp login swarvex"},
	{"Gemini CLI", "gemini", func(u string) []string {
		return []string{"mcp", "add", "--scope", "user", "--transport", "http", "swarvex", u}
	}, "in a session: /mcp auth swarvex"},
}

func mcpAgentenGefunden() []mcpAgent {
	var r []mcpAgent
	for _, a := range mcpAgenten {
		if _, err := exec.LookPath(a.Programm); err == nil {
			r = append(r, a)
		}
	}
	return r
}

func connectBefehl(args []string) {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	plattform := fs.String("platform", "", "address of the platform (default: where the runner is signed in, else "+standardPlattform+")")
	fs.Parse(args)
	if *plattform == "" {
		*plattform = standardPlattform
		if z, err := zugangLaden(); err == nil {
			*plattform = z.Plattform
		}
	}
	gefunden := mcpAgentenGefunden()
	if len(gefunden) == 0 {
		fmt.Println("No supported agent found in your PATH (Claude Code, Codex, Hermes, opencode, OpenClaw, Gemini CLI).")
		fmt.Println("Other agents: add the MCP server " + strings.TrimRight(*plattform, "/") + "/mcp (Streamable HTTP with OAuth).")
		return
	}
	verbinden(*plattform, gefunden)
}

func verbinden(plattform string, agenten []mcpAgent) {
	mcpURL := strings.TrimRight(plattform, "/") + "/mcp"
	for _, a := range agenten {
		var err error
		var aus []byte
		if a.Hinzufuegen == nil {
			err = opencodeEintragen(mcpURL)
		} else {
			c := exec.Command(a.Programm, a.Hinzufuegen(mcpURL)...)
			c.Stdin = os.Stdin // Codex und Hermes starten die Anmeldung gleich mit
			aus, err = c.CombinedOutput()
			if err != nil && strings.Contains(strings.ToLower(string(aus)), "already exists") {
				err = nil
			}
		}
		if err != nil {
			fmt.Printf("  ✗ %s: %s\n", a.Titel, letzteZeilen(strings.TrimSpace(string(aus))+" "+err.Error(), 3))
			continue
		}
		fmt.Printf("  ✓ %s connected. Sign in: %s\n", a.Titel, a.Anmelden)
	}
	fmt.Println("Directly connected agents only see tasks that run no code. Revoke access any time under My account on the website.")
}

// opencodeEintragen ergänzt ~/.config/opencode/opencode.json um den MCP-Server (nur reines JSON; sonst Hinweis).
func opencodeEintragen(mcpURL string) error {
	pfad := heim(".config", "opencode", "opencode.json")
	inhalt := map[string]any{}
	if roh, err := os.ReadFile(pfad); err == nil {
		if json.Unmarshal(roh, &inhalt) != nil {
			return fmt.Errorf("%s is not plain JSON; add {\"mcp\": {\"swarvex\": {\"type\": \"remote\", \"url\": %q}}} yourself", pfad, mcpURL)
		}
	}
	mcp, _ := inhalt["mcp"].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
	}
	mcp["swarvex"] = map[string]any{"type": "remote", "url": mcpURL}
	inhalt["mcp"] = mcp
	roh, _ := json.MarshalIndent(inhalt, "", "  ")
	if err := os.MkdirAll(filepath.Dir(pfad), 0o755); err != nil {
		return err
	}
	return os.WriteFile(pfad, append(roh, '\n'), 0o644)
}
