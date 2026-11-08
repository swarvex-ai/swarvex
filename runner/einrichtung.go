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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const standardPlattform = "https://swarvex.ai"

// ---- Fragen im Terminal (auch nach `curl … | sh`: dann über /dev/tty) ----

type frager struct {
	r   *bufio.Reader
	tty *os.File
}

// neuerFrager: Fragen im Terminal (im Test austauschbar).
var neuerFrager = terminalFrager

func terminalFrager() (*frager, bool) {
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

// wahl zeigt nummerierte Optionen; Enter nimmt die erste. Bei nur einer Option gibt es nichts zu wählen.
func (f *frager) wahl(text string, optionen []string) int {
	if len(optionen) == 1 {
		fmt.Println(text + " " + optionen[0])
		return 0
	}
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
		if a == "-h" || a == "--help" || a == "-help" {
			return args // nur die Optionen zeigen, auch ohne Einstellungen
		}
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

// lokaleModelle fragt die üblichen lokalen Server ab. Aus dem Container sind sie über host.containers.internal erreichbar;
// unter WSL zusätzlich Server, die unter Windows laufen, über die Windows-Adresse.
func lokaleModelle() []modellWahl {
	type ort struct{ abfrage, adresse, zusatz string }
	orte := []ort{{"127.0.0.1", "host.containers.internal", ""}}
	for _, a := range windowsAdressen() {
		orte = append(orte, ort{a, a, " (Windows)"})
	}
	var w []modellWahl
	gesehen := map[string]bool{}
	neu := func(titel, modell, port string, o ort) {
		if k := modell + "|" + port; !gesehen[k] {
			gesehen[k] = true
			w = append(w, modellWahl{Titel: titel + o.zusatz, Agent: "opencode", Modell: modell, URL: "http://" + o.adresse + ":" + port, Lokal: true})
		}
	}
	for _, o := range orte {
		var ollama struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if holeJSON("http://"+o.abfrage+":11434/api/tags", &ollama) {
			for _, m := range ollama.Models {
				neu("Ollama: "+m.Name, m.Name, "11434", o)
			}
		}
		for _, s := range []struct{ name, port string }{{"LM Studio", "1234"}, {"llama.cpp", "8080"}, {"vLLM", "8000"}} {
			var liste struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if holeJSON("http://"+o.abfrage+":"+s.port+"/v1/models", &liste) {
				for _, m := range liste.Data {
					neu(s.name+": "+m.ID, m.ID, s.port, o)
				}
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
func codexModell(ordner string) string {
	roh, err := os.ReadFile(filepath.Join(ordner, "config.toml"))
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
	// Codex-Anmeldung: hier und, unter WSL, im Windows-Benutzerordner
	codexOrdner := []string{heim(".codex")}
	if wh := windowsHeim(); wh != "" {
		codexOrdner = append(codexOrdner, filepath.Join(wh, ".codex"))
	}
	for _, o := range codexOrdner {
		datei := filepath.Join(o, "auth.json")
		if roh, err := os.ReadFile(datei); err == nil && istCodexAnmeldung(roh) {
			w = append(w, modellWahl{Titel: "Codex with your ChatGPT sign-in (" + datei + ")", Agent: "codex", Modell: codexModell(o), Key: datei})
		}
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
	w = append(w, hermesModelle()...)
	w = append(w, agentenSchluessel(w)...)
	return append(w, modellWahl{Titel: "Something else (enter address, model and key yourself)"})
}

// hermesModelle übernimmt das Modell, das Hermes Agent nutzt (~/.hermes/config.yaml, Schlüssel dort oder in ~/.hermes/.env).
// Nur Anbieter mit API-Schlüssel oder lokale Server; Anmeldungen per OAuth (Nous Portal, Codex) lassen sich nicht übertragen.
func hermesModelle() []modellWahl {
	ordner := os.Getenv("HERMES_HOME")
	if ordner == "" {
		ordner = heim(".hermes")
	}
	roh, err := os.ReadFile(filepath.Join(ordner, "config.yaml"))
	if err != nil {
		return nil
	}
	m := hermesModellAbschnitt(string(roh))
	modell := m["default"]
	if modell == "" {
		modell = m["model"]
	}
	if modell == "" {
		return nil
	}
	env := envDatei(filepath.Join(ordner, ".env"))
	schluessel := func(namen ...string) (string, string) {
		if m["api_key"] != "" {
			return m["api_key"], "config.yaml"
		}
		for _, n := range namen {
			if env[n] != "" {
				return env[n], ".env"
			}
			if v := os.Getenv(n); v != "" {
				return v, n
			}
		}
		return "", ""
	}
	anbieter, adresse := strings.ToLower(m["provider"]), strings.TrimSuffix(strings.TrimSuffix(m["base_url"], "/"), "/v1")
	w := modellWahl{Agent: "opencode", Modell: modell, URL: adresse}
	var key, quelle, name string
	cloud := false // Cloud-Anbieter ohne Schlüssel: nichts zu übernehmen
	switch {
	case anbieter == "anthropic" || strings.Contains(adresse, "api.anthropic.com"):
		w.Agent, w.URL, name, cloud = "claude", "https://api.anthropic.com", "Anthropic", true
		key, quelle = schluessel("ANTHROPIC_API_KEY")
	case anbieter == "openrouter" || strings.Contains(adresse, "openrouter.ai") || ((anbieter == "auto" || anbieter == "") && adresse == ""):
		w.URL, name, cloud = "https://openrouter.ai/api", "OpenRouter", true
		key, quelle = schluessel("OPENROUTER_API_KEY", "OPENAI_API_KEY")
	case adresse != "" && (anbieter == "custom" || anbieter == "auto" || anbieter == "" || anbieter == "ollama" || anbieter == "vllm" || anbieter == "llamacpp" || anbieter == "lmstudio"):
		name = adresse
		if neu, lokal := sandboxAdresse(adresse); lokal {
			w.URL, w.Lokal = neu, true // aus der Sandbox ist der eigene Rechner host.containers.internal
		}
		key, quelle = schluessel("OPENAI_API_KEY")
	default:
		return nil // OAuth-Anbieter oder solche, die wir nicht zuordnen können
	}
	if cloud && key == "" {
		return nil
	}
	w.Titel = "Hermes's model: " + modell + " via " + name
	if key != "" {
		w.Titel += " (key from Hermes " + quelle + ")"
		w.Key = "wert:" + key
	}
	return []modellWahl{w}
}

// hermesModellAbschnitt liest die einfachen Werte unter "model:" (oder "model: <name>") aus Hermes' config.yaml.
func hermesModellAbschnitt(yaml string) map[string]string {
	werte := map[string]string{}
	drin := false
	for _, zeile := range strings.Split(yaml, "\n") {
		ohne := strings.TrimRight(zeile, " \t\r")
		if ohne == "" || strings.HasPrefix(strings.TrimSpace(ohne), "#") {
			continue
		}
		eingerueckt := ohne[0] == ' ' || ohne[0] == '\t'
		if !eingerueckt {
			drin = false
			if strings.HasPrefix(ohne, "model:") {
				if v := yamlWert(strings.TrimPrefix(ohne, "model:")); v != "" {
					werte["model"] = v
				} else {
					drin = true
				}
			}
			continue
		}
		if drin {
			if k, v, ok := strings.Cut(strings.TrimSpace(ohne), ":"); ok && !strings.ContainsAny(k, " \t") {
				if wert := yamlWert(v); wert != "" {
					werte[k] = wert
				}
			}
		}
	}
	return werte
}

// yamlWert: Wert ohne Kommentar und Anführungszeichen.
func yamlWert(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 1 && (v[0] == '"' || v[0] == '\'') {
		if ende := strings.IndexByte(v[1:], v[0]); ende >= 0 {
			return v[1 : ende+1]
		}
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

// envDatei liest KEY=wert-Zeilen (wie Hermes' .env).
func envDatei(pfad string) map[string]string {
	werte := map[string]string{}
	roh, err := os.ReadFile(pfad)
	if err != nil {
		return werte
	}
	for _, zeile := range strings.Split(string(roh), "\n") {
		zeile = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(zeile), "export "))
		if k, v, ok := strings.Cut(zeile, "="); ok && !strings.HasPrefix(k, "#") {
			werte[strings.TrimSpace(k)] = yamlWert(v)
		}
	}
	return werte
}

// schluesselDatei legt einen eingegebenen oder aus der Umgebung übernommenen Schlüssel als Datei 0600 ab.
func schluesselDatei(name, wert string) string {
	pfad := filepath.Join(konfigOrdner(), name)
	pruefen(os.MkdirAll(konfigOrdner(), 0o700))
	pruefen(os.WriteFile(pfad, []byte(strings.TrimSpace(wert)+"\n"), 0o600))
	return pfad
}

// eigeneAdressen: die Adressen dieses Rechners (zum Test austauschbar).
var eigeneAdressen = func() []net.IP {
	var ips []net.IP
	if adr, err := net.InterfaceAddrs(); err == nil {
		for _, a := range adr {
			if n, ok := a.(*net.IPNet); ok {
				ips = append(ips, n.IP)
			}
		}
	}
	return ips
}

// sandboxAdresse: wie die Sandbox einen Modell-Server auf diesem Rechner erreicht. localhost und die eigene Adresse
// im Heimnetz zeigen aus dem Container nicht auf den Rechner (rootless Podman gibt dem Container die Adresse des
// Rechners mit), host.containers.internal schon. Server auf anderen Geräten bleiben unverändert. Bei https mit der
// eigenen IP bleibt die Adresse, weil das Zertifikat auf sie lautet.
func sandboxAdresse(adresse string) (string, bool) {
	u, err := url.Parse(adresse)
	if err != nil || u.Host == "" {
		return adresse, false
	}
	h := u.Hostname()
	eigen := h == "localhost"
	if ip := net.ParseIP(h); ip != nil {
		if ip.IsLoopback() {
			eigen = true
		} else if u.Scheme == "http" {
			for _, e := range eigeneAdressen() {
				if e.Equal(ip) {
					eigen = true
				}
			}
		}
	}
	if !eigen {
		return adresse, false
	}
	port := u.Port()
	u.Host = "host.containers.internal"
	if port != "" {
		u.Host += ":" + port
	}
	return u.String(), true
}

// erreichbar prüft im Proxy-Image (gleiches Netz wie der echte Proxy), ob die Modell-Adresse antwortet.
func erreichbar(adresse string) (bool, string) {
	if _, err := exec.LookPath("podman"); err != nil {
		return true, "podman missing, not checked"
	}
	adresse, _ = sandboxAdresse(adresse)
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
	Projekt string   `json:"project"`
	Name    string   `json:"name"`
	Offen   int      `json:"tasks_open"`
	Sucht   []string `json:"wants"` // Fähigkeiten, die offene Aufgaben am häufigsten verlangen
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

	fmt.Println("\n== 1/2 Model\nYour agent works with a model of your choice. Your key stays on this machine; the sandbox never sees it.")
	w, abo := modellWaehlen(f, "Which model should your agent use?")
	var plan aboPlan
	if abo {
		plan = aboErmitteln(f, w)
	}

	fmt.Println("\n== 2/2 Settings")
	var agenten []mcpAgent
	if !istWSL() {
		agenten = mcpAgentenGefunden()
	}
	e := empfehlung(w, abo, plan, agenten)
	medienEmpfehlen(&e, medienServerFinden())
	if !e.moeglich {
		fmt.Println(e.grund)
		return anpassen(f, z, w, abo, plan, agenten, ausfuehren)
	}
	fmt.Println("Recommended:")
	for _, zeile := range e.zeilen {
		fmt.Println("  • " + zeile)
	}
	if !f.jaNein("Use these settings? (no: choose each one yourself)", true) {
		return anpassen(f, z, w, abo, plan, agenten, ausfuehren)
	}
	pruefen(argsSpeichern(e.args))
	gefragtMerken(gefragtDatei(), "auto-update", "fair-use", "task-budget", "media")
	fmt.Println("Saved in", einstellungenDatei(), "(change any time with `swarvex setup`).")
	agentenVerbinden(z, agenten)
	if !ausfuehren {
		return e.args
	}
	dienstBefehl(append(append([]string{}, e.args...), e.dienstArgs...))
	return e.args
}

// modellWaehlen: Modell und Zugang im Terminal wählen (erkannte Agenten, Abo-Token, API-Schlüssel, eigener Server) und
// prüfen, ob die Sandbox es erreicht. abo: ein Abo statt API-Schlüssel (dann gibt es Kontingent, das verfällt).
// Für `swarvex setup` und für `swarvex project lead setup` auf einem Rechner ohne eingerichteten Runner.
func modellWaehlen(f *frager, frage string) (w modellWahl, abo bool) {
	optionen := modellAuswahl()
	titel := make([]string, len(optionen))
	for i, o := range optionen {
		titel[i] = o.Titel
	}
	w = optionen[f.wahl(frage, titel)]
	// Abo statt API-Schlüssel: nur dann gibt es Kontingent, das verfällt (Restkontingent-Modus)
	abo = w.Key == "claude-setup-token" || (w.Agent == "codex" && strings.HasSuffix(w.Key, "auth.json"))
	switch {
	case w.Key == "claude-setup-token":
		w.Key = schluesselDatei("claude-token", claudeTokenHolen(f))
	case strings.HasPrefix(w.Key, "env:"):
		w.Key = schluesselDatei(strings.ToLower(strings.TrimPrefix(w.Key, "env:")), os.Getenv(strings.TrimPrefix(w.Key, "env:")))
	case strings.HasPrefix(w.Key, "wert:"):
		roh := strings.TrimPrefix(w.Key, "wert:")
		w.Key = schluesselDatei(w.Agent+"-key", roh)
		// Modell unbekannt: die Modellliste des Anbieters anbieten, wenn sie überschaubar ist
		if w.Modell == "" && w.Agent == "opencode" {
			if liste := anbieterModelle(w.URL, roh); len(liste) > 0 && len(liste) <= 25 {
				w.Modell = liste[f.wahl("Which model?", liste)]
			}
		}
	case w.Titel == optionen[len(optionen)-1].Titel:
		if len(optionen) == 1 {
			fmt.Println("  (Nothing found automatically: no Ollama, LM Studio, Codex, Claude Code, Hermes or API key in the environment.)")
		}
		i := f.wahl("Which interface does your model offer?",
			[]string{"OpenAI-compatible (Ollama, OpenRouter, most providers) → agent opencode", "Anthropic → agent Claude Code", "OpenAI Responses → agent Codex"})
		w.Agent = []string{"opencode", "claude", "codex"}[i]
		vorschlag := []string{"https://openrouter.ai/api", "https://api.anthropic.com", "https://api.openai.com"}[i]
		for w.URL == "" {
			w.URL = strings.TrimSuffix(strings.TrimSuffix(f.frage("Address of the model API without /v1 (local server: http://host.containers.internal:<port>)", vorschlag), "/"), "/v1")
		}
		if neu, lokal := sandboxAdresse(w.URL); lokal {
			fmt.Println("  That server runs on this computer; the sandbox reaches it as " + neu)
			w.URL, w.Lokal = neu, true
		}
		if k := f.geheim("API key (empty if none)"); k != "" {
			w.Key = schluesselDatei(w.Agent+"-key", k)
		}
	}
	for w.Modell == "" { // erkannte Modelle gelten ohne Frage; ändern lässt es sich unter „Customize“
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
				fehlerAus("Cancelled; run the command again when the model is reachable.")
			}
		}
	}
	return w, abo
}

// empfohlen: der Schnellstart (eine Frage statt aller): beliebiges Projekt, Dienst, automatische Updates, Budget nach
// Art des Zugangs, bei Abos die faire Nutzung für die Stufe, gefundene Agenten verbunden.
type empfohlen struct {
	moeglich         bool
	grund            string
	zeilen           []string
	args, dienstArgs []string
}

func modellArgs(w modellWahl, projekt string) []string {
	args := []string{"--project", projekt, "--agent", w.Agent, "--model", w.Modell}
	if w.URL != "" {
		args = append(args, "--model-url", w.URL)
	}
	if w.Key != "" {
		args = append(args, "--model-key-file", w.Key)
	}
	return args
}

// setupProjekt: `swarvex setup --project <slug>` (Teilen-Link eines Projekts): der Schnellstart arbeitet dort statt überall.
var setupProjekt string

func empfehlung(w modellWahl, abo bool, plan aboPlan, agenten []mcpAgent) empfohlen {
	projekt, projektZeile := "auto", "project: any, wherever most is open"
	if setupProjekt != "" {
		projekt, projektZeile = setupProjekt, "project: "+setupProjekt
	}
	e := empfohlen{moeglich: true, args: modellArgs(w, projekt), dienstArgs: []string{"--auto-update"}}
	e.zeilen = []string{"model: " + w.Modell, projektZeile,
		"runs in the background (also after a restart) and installs signed updates itself"}
	switch {
	case abo && plan.Firma:
		e.moeglich, e.grund = false, plan.Name+": swarvex never uses a work account on its own. Choose each setting yourself:"
	case abo && plan.ZuKlein:
		e.moeglich, e.grund = false, plan.Name+" has too little quota for tasks (a local model or an API key works better). Choose each setting yourself:"
	case abo:
		e.zeilen = append(e.zeilen, plan.Name+", fair use: "+plan.fairText())
		e.dienstArgs = append(e.dienstArgs, plan.fairArgs()...)
	case w.Lokal:
		e.zeilen = append(e.zeilen, "local model: no token limits")
	default:
		e.zeilen = append(e.zeilen, "at most 2,000,000 tokens per day, however they split between tasks")
		e.dienstArgs = append(e.dienstArgs, "--daily-budget-tokens", "2000000")
	}
	if len(agenten) > 0 {
		namen := make([]string, len(agenten))
		for i, a := range agenten {
			namen[i] = a.Titel
		}
		e.zeilen = append(e.zeilen, "connects "+strings.Join(namen, ", ")+" for tasks without code (design, texts, reviews)")
	}
	return e
}

// claudeTokenHolen startet `claude setup-token` im selben Fenster (es öffnet den Browser) und nimmt den Token entgegen.
func claudeTokenHolen(f *frager) string {
	fmt.Println("Claude Code now creates a token for the runner (`claude setup-token`, opens your browser).")
	cmd := exec.Command("claude", "setup-token")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = f.tty, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("  That did not start (" + err.Error() + "). Run `claude setup-token` in another terminal.")
	}
	for i := 0; ; i++ {
		t := strings.TrimSpace(f.geheim("Paste the token it showed (starts with sk-ant-oat, input is hidden)"))
		if strings.HasPrefix(t, "sk-ant-oat") || i >= 2 {
			return t
		}
		fmt.Println("  That does not look like a Claude token (sk-ant-oat…).")
	}
}

// agentenVerbinden: gefundene Agenten für Aufgaben ohne Code eintragen (unter WSL macht das der Windows-Installer).
func agentenVerbinden(z *zugangsdaten, agenten []mcpAgent) {
	switch {
	case istWSL():
		fmt.Println("Agents installed on Windows (Claude Code, Codex …) are connected by the Windows installer, or later with: swarvex connect")
	case len(agenten) > 0:
		verbinden(z.Plattform, agenten)
	}
}

// anpassen: jede Einstellung einzeln (statt des Schnellstarts).
func anpassen(f *frager, z *zugangsdaten, w modellWahl, abo bool, plan aboPlan, agenten []mcpAgent, ausfuehren bool) []string {
	w.Modell = f.frage("Model name", w.Modell)
	for w.Modell == "" {
		w.Modell = f.frage("Model name (required)", "")
	}

	fmt.Println("\n-- Project")
	projekte := projekteLaden(z.Plattform, z.CAPfad)
	pTitel := []string{"Any project, wherever most is open (recommended)"}
	for _, p := range projekte {
		titel := fmt.Sprintf("%s (%s, %d open tasks)", p.Name, p.Projekt, p.Offen)
		if len(p.Sucht) > 0 {
			titel += ", wanted: " + strings.Join(p.Sucht, ", ")
		}
		pTitel = append(pTitel, titel)
	}
	projekt := "auto"
	if i := f.wahl("Where should your agent help?", pTitel); i > 0 {
		projekt = projekte[i-1].Projekt
	}
	args := modellArgs(w, projekt)

	fmt.Println("\n-- When")
	modus := f.wahl("How should it run?", []string{"As a service in the background (recommended)", "Try one task now", "Only save, I start it myself with `swarvex work`"})
	var dienstArgs []string
	if modus == 0 {
		titel := make([]string, len(zeitVorschlaege))
		for i, v := range zeitVorschlaege {
			titel[i] = v.titel
		}
		fenster := zeitVorschlaege[f.wahl("When may it work?", titel)].fenster
		if fenster == "?" {
			fenster = f.frage("Your time windows (e.g. mon-fri 18:00-23:00; sat,sun 09:00-24:00)", "daily 18:00-23:00")
			for _, err := zeitfensterLesen(fenster); err != nil; _, err = zeitfensterLesen(fenster) {
				fmt.Println("  " + err.Error())
				fenster = f.frage("Your time windows", "daily 18:00-23:00")
			}
		}
		if fenster != "" {
			dienstArgs = append(dienstArgs, "--schedule", fenster)
		}
		if abo {
			dienstArgs = append(dienstArgs, aboNutzungFragen(f, plan)...)
		} else {
			vorschlag := "2000000"
			if w.Lokal {
				vorschlag = "0"
			}
			dienstArgs = append(dienstArgs, "--daily-budget-tokens", f.frage("Tokens per day at most (0 = no limit)", vorschlag))
		}
		if f.jaNein(autoUpdateFrage, true) {
			dienstArgs = append(dienstArgs, "--auto-update")
		}
	}
	// Standard ohne Grenze je Aufgabe: Das Tagesbudget (bzw. der Restkontingent-Modus) deckelt jede Aufgabe ohnehin,
	// eine eigene Grenze bricht nur große Aufgaben ab. Wer eine will: mindestens 50.000, darunter kommt kein Agent weit.
	budgetVorschlag := "0"
	aufgabenBudget := f.frage("Tokens per task at most (0 = no limit; the daily budget still caps every task)", budgetVorschlag)
	for n, err := strconv.Atoi(aufgabenBudget); err != nil || (n != 0 && n < 50000); n, err = strconv.Atoi(aufgabenBudget) {
		aufgabenBudget = f.frage("Tokens per task at most (0 = no limit, otherwise at least 50000)", budgetVorschlag)
	}
	args = append(args, "--budget-tokens", aufgabenBudget)
	args = append(args, medienFragen(f, medienServerFinden())...)
	pruefen(argsSpeichern(args))
	gefragtMerken(gefragtDatei(), "auto-update", "fair-use", "task-budget", "media")
	fmt.Println("Saved in", einstellungenDatei(), "(change any time with `swarvex setup`).")

	fmt.Println("\n-- Tasks without code")
	if len(agenten) > 0 && !istWSL() {
		namen := make([]string, len(agenten))
		for i, a := range agenten {
			namen[i] = a.Titel
		}
		if f.jaNein("Also connect "+strings.Join(namen, ", ")+" directly (design, texts, reviews; no code runs on your machine)?", true) {
			agentenVerbinden(z, agenten)
		}
	} else if istWSL() {
		agentenVerbinden(z, nil)
	} else {
		fmt.Println("No other agent found (Claude Code, Codex, Hermes, opencode, OpenClaw, Gemini CLI). Later: swarvex connect")
	}

	if !ausfuehren {
		return args
	}
	switch modus {
	case 0:
		dienstBefehl(append(append([]string{}, args...), dienstArgs...))
	case 1:
		fmt.Println("\nStarting one task …")
		arbeitenBefehl(args)
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
		// unter WSL nicht die Windows-Programme aus dem angehängten Windows-PATH (/mnt/c/…): die verbindet install.ps1
		if pfad, err := exec.LookPath(a.Programm); err == nil && !strings.HasPrefix(pfad, "/mnt/") {
			r = append(r, a)
			continue
		}
		// Installer wie der von Hermes legen ihr Programm in ~/.local/bin ab, das oft (noch) nicht im PATH ist
		for _, ordner := range []string{heim(".local", "bin"), heim(".npm-global", "bin"), heim(".bun", "bin")} {
			if info, err := os.Stat(filepath.Join(ordner, a.Programm)); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
				a.Programm = filepath.Join(ordner, a.Programm)
				r = append(r, a)
				break
			}
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
			// Codex und Hermes fragen selbst nach (Hermes: "Enable all tools?", "Overwrite?") und starten die Anmeldung:
			// ihre Ausgabe deshalb sichtbar durchreichen und nur zusätzlich mitlesen
			var puffer bytes.Buffer
			c.Stdin = os.Stdin
			c.Stdout, c.Stderr = io.MultiWriter(os.Stdout, &puffer), io.MultiWriter(os.Stderr, &puffer)
			fmt.Printf("  %s asks a few things itself:\n", a.Titel)
			err = c.Run()
			aus = puffer.Bytes()
			if err != nil && strings.Contains(strings.ToLower(string(aus)), "already exists") {
				err = nil
			}
		}
		if err != nil {
			fmt.Printf("  ✗ %s: %s\n", a.Titel, letzteZeilen(strings.TrimSpace(string(aus))+" "+err.Error(), 3))
			continue
		}
		if halbFertig(aus) {
			// Hermes beendet sich auch dann mit 0, wenn die Anmeldung scheitert und es den Server deaktiviert speichert
			spaeter := a.Anmelden
			if i := strings.Index(spaeter, "later: "); i >= 0 {
				spaeter = spaeter[i+len("later: "):]
			}
			fmt.Printf("  ! %s: added, but the sign-in did not finish. Finish it in a terminal on this machine's desktop: %s\n", a.Titel, spaeter)
			continue
		}
		fmt.Printf("  ✓ %s connected. Sign in: %s\n", a.Titel, a.Anmelden)
	}
	fmt.Println("Directly connected agents only see tasks that run no code. Revoke access any time under My account on the website.")
}

// halbFertig erkennt in der Ausgabe eines Agenten, dass er den Server zwar gespeichert, die Verbindung
// oder Anmeldung aber nicht geschafft hat (Hermes: "Failed to connect", "Saved ... (disabled)").
func halbFertig(aus []byte) bool {
	t := strings.ToLower(string(aus))
	for _, m := range []string{"failed to connect", "(disabled)", "authorization failed", "oauth login failed"} {
		if strings.Contains(t, m) {
			return true
		}
	}
	return false
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

// zeitVorschlaege für die Einrichtung; "" heißt immer, "?" eigene Angabe. Über Mitternacht endet ein Fenster am Folgetag.
var zeitVorschlaege = []struct{ titel, fenster string }{
	{"Always", ""},
	{"At night (daily 00:00-06:00)", "daily 00:00-06:00"},
	{"Evenings and nights (daily 18:00-06:00)", "daily 18:00-06:00"},
	{"Outside working hours (mon-fri 18:00-08:00, all weekend)", "mon-fri 18:00-08:00; sat,sun 00:00-24:00"},
	{"Weekends only (sat, sun)", "sat,sun 00:00-24:00"},
	{"My own time windows", "?"},
}

// ---- Schlüssel aus anderen Agenten ----

// Anbieter, deren Schlüssel Agenten üblicherweise ablegen; URL ohne /v1 (der Agent in der Sandbox hängt es an).
// Gemini fehlt bewusst: seine OpenAI-kompatible Adresse endet nicht auf /v1.
var anbieterListe = []struct{ env, name, id, agent, url string }{
	{"OPENROUTER_API_KEY", "OpenRouter", "openrouter", "opencode", "https://openrouter.ai/api"},
	{"DEEPSEEK_API_KEY", "DeepSeek", "deepseek", "opencode", "https://api.deepseek.com"},
	{"ANTHROPIC_API_KEY", "Anthropic", "anthropic", "claude", "https://api.anthropic.com"},
	{"OPENAI_API_KEY", "OpenAI", "openai", "codex", "https://api.openai.com"},
	{"MOONSHOT_API_KEY", "Moonshot (Kimi)", "moonshot", "opencode", "https://api.moonshot.ai"},
	{"XAI_API_KEY", "xAI", "xai", "opencode", "https://api.x.ai"},
	{"MISTRAL_API_KEY", "Mistral", "mistral", "opencode", "https://api.mistral.ai"},
	{"GROQ_API_KEY", "Groq", "groq", "opencode", "https://api.groq.com/openai"},
}

// agentenOrte: wo installierte Agenten ihre Schlüssel ablegen (.env-Dateien, einfache api_key-Zeilen in TOML,
// opencodes auth.json). Unter WSL auch im Windows-Benutzerordner.
var agentenOrte = []struct {
	agent   string
	dateien []string
}{
	{"opencode", []string{".local/share/opencode/auth.json"}},
	{"Codewhale (DeepSeek TUI)", []string{".codewhale/.env", ".codewhale/config.toml", ".deepseek/.env", ".deepseek/config.toml"}},
	{"OpenClaw", []string{".openclaw/.env"}},
	{"Gemini CLI", []string{".gemini/.env"}},
	{"Qwen Code", []string{".qwen/.env"}},
	{"Hermes", []string{".hermes/.env"}},
	{"your ~/.env", []string{".env"}},
}

func agentenSchluessel(bisher []modellWahl) []modellWahl {
	gesehen := map[string]bool{}
	for _, b := range bisher {
		switch {
		case strings.HasPrefix(b.Key, "wert:"):
			gesehen[strings.TrimPrefix(b.Key, "wert:")] = true
		case strings.HasPrefix(b.Key, "env:"):
			gesehen[os.Getenv(strings.TrimPrefix(b.Key, "env:"))] = true
		}
	}
	heime := []string{heim()}
	if wh := windowsHeim(); wh != "" {
		heime = append(heime, wh)
	}
	opencodeModell := opencodeStandardModell()
	var w []modellWahl
	for _, h := range heime {
		for _, ort := range agentenOrte {
			for _, datei := range ort.dateien {
				pfad := filepath.Join(h, datei)
				for anbieterID, key := range schluesselIn(pfad) {
					if key == "" || gesehen[key] {
						continue
					}
					for _, a := range anbieterListe {
						if a.id != anbieterID {
							continue
						}
						gesehen[key] = true
						modell := ""
						if a.id == "anthropic" {
							modell = "claude-sonnet-5-5"
						} else if m, ok := strings.CutPrefix(opencodeModell, a.id+"/"); ok && ort.agent == "opencode" {
							modell = m
						}
						w = append(w, modellWahl{Titel: fmt.Sprintf("%s key from %s (%s)", a.name, ort.agent, pfad), Agent: a.agent, Modell: modell, URL: a.url, Key: "wert:" + key})
					}
				}
			}
		}
	}
	return w
}

// schluesselIn liefert Anbieter-ID → Schlüssel aus einer Datei: KEY=wert (.env), api_key = "…" (TOML, Anbieter aus
// [providers.<id>] oder provider = "<id>") oder opencodes auth.json ({"<id>": {"type": "api", "key": "…"}}).
func schluesselIn(pfad string) map[string]string {
	r := map[string]string{}
	roh, err := os.ReadFile(pfad)
	if err != nil {
		return r
	}
	switch {
	case strings.HasSuffix(pfad, ".json"):
		var auth map[string]struct {
			Type string `json:"type"`
			Key  string `json:"key"`
		}
		if json.Unmarshal(roh, &auth) == nil {
			for id, e := range auth {
				if e.Type == "api" {
					if id == "moonshotai" {
						id = "moonshot" // opencodes ID für Moonshot
					}
					r[id] = e.Key
				}
			}
		}
	case strings.HasSuffix(pfad, ".toml"):
		abschnitt, oben := "", ""
		for _, zeile := range strings.Split(string(roh), "\n") {
			z := strings.TrimSpace(zeile)
			if strings.HasPrefix(z, "[") {
				abschnitt = strings.TrimPrefix(strings.Trim(z, "[]"), "providers.")
				continue
			}
			k, v, ok := strings.Cut(z, "=")
			if !ok {
				continue
			}
			k, v = strings.TrimSpace(k), yamlWert(v)
			if k == "provider" && abschnitt == "" {
				oben = v
			}
			if k == "api_key" && v != "" {
				ziel := abschnitt
				if ziel == "" {
					ziel = oben
				}
				r[strings.TrimSuffix(ziel, "-cn")] = v
			}
		}
	default:
		for k, v := range envDatei(pfad) {
			for _, a := range anbieterListe {
				if k == a.env || (a.id == "moonshot" && k == "KIMI_API_KEY") {
					r[a.id] = v
				}
			}
		}
	}
	return r
}

// opencodeStandardModell: "model" aus ~/.config/opencode/opencode.json (z. B. "openrouter/deepseek/…").
func opencodeStandardModell() string {
	roh, err := os.ReadFile(heim(".config", "opencode", "opencode.json"))
	if err != nil {
		return ""
	}
	var k struct {
		Model string `json:"model"`
	}
	json.Unmarshal(roh, &k)
	return k.Model
}

// anbieterModelle fragt die Modellliste einer OpenAI-kompatiblen Schnittstelle ab (GET /v1/models).
func anbieterModelle(adresse, key string) []string {
	req, err := http.NewRequest("GET", strings.TrimRight(adresse, "/")+"/v1/models", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+key)
	c := &http.Client{Timeout: 8 * time.Second}
	r, err := c.Do(req)
	if err != nil {
		return nil
	}
	defer r.Body.Close()
	var liste struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if r.StatusCode != 200 || json.NewDecoder(r.Body).Decode(&liste) != nil {
		return nil
	}
	var ids []string
	for _, m := range liste.Data {
		ids = append(ids, m.ID)
	}
	return ids
}
