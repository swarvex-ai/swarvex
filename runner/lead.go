package main

// swarvex project lead: der Lead eines Projekts als Dienst in einer eigenen Sandbox (E-38, Entscheidung neVs 2026-11-06).
// Bisher lief der Lead als Sitzung mit allen Rechten auf dem Rechner des Visionärs und führte dort fremden Code aus
// (Tests, Editor, das Spiel). Jetzt:
//
//   - Ein Container aus der Projekt-Umgebung (wie die Sandbox einer Aufgabe) mit Agent, Git und virtuellem Bildschirm.
//     Dauerhaft sind nur der Klon (/work), das Zuhause des Agenten (/home/agent) und Caches; nichts vom Rechner.
//   - Netz nur über den Lead-Proxy (leadproxy.go): das Modell (der Schlüssel bleibt im Proxy), die Plattform (/mcp; das
//     Token bleibt im Proxy) und der Git-Host des Projekts. Git meldet sich mit einem Token an, das nur dieses Repo
//     betrifft und eine Stunde gilt (GitHub App, ohne Workflow-Rechte).
//   - Eine eigene Anmeldung, an das Projekt gebunden: nur dessen Lead-Werkzeuge, keine Freigaben (apps/api leaddienst.ts).
//   - Dauerbetrieb als systemd-Dienst: Der Runner wartet draußen auf Arbeit (wait_for_work, kostet keine Tokens) und
//     startet erst dann eine Runde des Agenten. Jede Runde meldet er der Plattform; die Projektseite zeigt die Anwesenheit.
//
// Ablage: ~/.local/share/swarvex/lead/<projekt>/ (config.json, work, home, cache, run, proxy, log) und die Anmeldung
// in ~/.config/swarvex/lead-<projekt>.json.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

type leadKonfig struct {
	Projekt       string `json:"project"`
	Plattform     string `json:"platform"`
	CAPfad        string `json:"ca_path,omitempty"`
	Agent         string `json:"agent"`
	Modell        string `json:"model"`
	ModellURL     string `json:"model_url,omitempty"`
	KeyDatei      string `json:"model_key_file,omitempty"`
	GitTokenDatei string `json:"git_token_file,omitempty"`
	GitNutzer     string `json:"git_user,omitempty"`
	Tagesbudget   int64  `json:"daily_budget_tokens"`
	Ruhe          string `json:"idle_cycle"`
	RundeMax      string `json:"cycle_time_limit"`
	Paket         string `json:"bundle"`
	RepoGit       string `json:"repo_git"`
	AutorName     string `json:"author_name"`
	AutorEmail    string `json:"author_email"`
	// Mehrere Lead-Dienste je Projekt (E-39): Name des Dienstes auf diesem Rechner (<projekt>.<bereich>; leer: das
	// Projekt), sein Bereich (take_area) und ob er einem Bereichs-Lead gehört (liest das Repository nur, pusht nicht).
	Name         string   `json:"name,omitempty"`
	Bereich      string   `json:"area,omitempty"`
	Pfade        []string `json:"paths,omitempty"`
	Faehigkeiten []string `json:"capabilities,omitempty"`
	BereichsLead bool     `json:"area_lead,omitempty"`
}

// dienst: Name dieses Lead-Dienstes auf dem Rechner (Ordner, Anmeldung, systemd-Einheit, Container).
func (l *leadKonfig) dienst() string {
	if l.Name != "" {
		return l.Name
	}
	return l.Projekt
}

var leadSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// leadName: ein Lead-Dienst auf diesem Rechner, das Projekt oder <projekt>.<bereich> (setup --area)
var leadName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}(\.[a-z][a-z0-9-]{1,30})?$`)

func leadOrdner(p string) string      { return filepath.Join(datenOrdner(), "lead", p) }
func leadUnter(p, teil string) string { return filepath.Join(leadOrdner(p), teil) }
func leadZugangDatei(p string) string { return filepath.Join(konfigOrdner(), "lead-"+p+".json") }
func leadDienstName(p string) string  { return "swarvex-lead-" + p + ".service" }
func leadDienstDatei(p string) string {
	return filepath.Join(filepath.Dir(dienstDatei()), leadDienstName(p))
}
func leadRundenDatei(p string) string    { return leadUnter(p, "cycles.jsonl") }
func leadVerbrauchDatei(p string) string { return leadUnter(p, "usage.jsonl") }

// Nach so langer Ruhe läuft eine Runde auch ohne Anlass (Plan und Status); einmal je Stunde höchstens für dasselbe.
const (
	leadRuheStandard     = 6 * time.Hour
	leadRundeMaxStandard = 60 * time.Minute
	leadWiederholung     = 60 * time.Minute
	leadAbstand          = 2 * time.Minute
)

func projektBefehl(args []string) {
	if len(args) == 0 || args[0] != "lead" {
		fmt.Println("Usage: swarvex project lead <setup|run|cycle|start|stop|status|logs|attach|exec|remove> <project> [options]")
		fmt.Println("Runs the lead of a project you lead as a service in its own sandbox. Docs: " + standardPlattform + "/en/docs/lead-service")
		os.Exit(2)
	}
	args = args[1:]
	if len(args) == 0 {
		leadUebersicht()
		return
	}
	befehl := args[0]
	if len(args) < 2 || strings.HasPrefix(args[1], "-") {
		fehlerAus("which project? swarvex project lead " + befehl + " <project>")
	}
	p := args[1]
	if !leadName.MatchString(p) || (befehl == "setup" && !leadSlug.MatchString(p)) {
		fehlerAus("not a project slug: " + p + " (a lead service for an area is named <project>.<area>; set it up with --area)")
	}
	rest := args[2:]
	switch befehl {
	case "setup":
		leadEinrichten(p, rest)
	case "run":
		leadBetrieb(p)
	case "cycle":
		leadEineRunde(p, rest)
	case "start":
		leadSystemctl(p, "start")
	case "stop":
		leadSystemctl(p, "stop")
	case "status":
		leadStatus(p)
	case "logs":
		c := exec.Command("journalctl", "--user", "-u", leadDienstName(p), "-n", "80", "-f", "--no-pager", "-o", "short-iso")
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		c.Run()
	case "attach":
		leadAnschliessen(p)
	case "exec":
		leadBefehlAusfuehren(p, rest)
	case "remove":
		leadEntfernen(p)
	default:
		fehlerAus("unknown: swarvex project lead " + befehl + " (setup, run, cycle, start, stop, status, logs, attach, exec, remove)")
	}
}

func leadKonfigLaden(p string) (*leadKonfig, error) {
	roh, err := os.ReadFile(leadUnter(p, "config.json"))
	if err != nil {
		return nil, fmt.Errorf("no lead service for %s here: run `swarvex project lead setup %s` first", p, p)
	}
	var l leadKonfig
	return &l, json.Unmarshal(roh, &l)
}

func (l *leadKonfig) speichern() error {
	roh, _ := json.MarshalIndent(l, "", "  ")
	return os.WriteFile(leadUnter(l.dienst(), "config.json"), roh, 0o600)
}

func (l *leadKonfig) dauer(wert string, standard time.Duration) time.Duration {
	if d, err := time.ParseDuration(wert); err == nil && d > 0 {
		return d
	}
	return standard
}

// ---- Plattform: öffentliche Projektdaten, Bindung, Git-Zugang ----

type leadProjektInfo struct {
	Art       string `json:"project_type"`
	Verfahren string `json:"procedure"`
	RepoURL   string `json:"repo_url"`
	Name      string `json:"name"`
}

func leadProjektLesen(plattform, ca, p string) (leadProjektInfo, error) {
	var info leadProjektInfo
	r, err := httpClient(ca).Get(plattform + "/api/projects/" + p)
	if err != nil {
		return info, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return info, fmt.Errorf("project %s not found at %s (HTTP %d)", p, plattform, r.StatusCode)
	}
	return info, json.NewDecoder(r.Body).Decode(&info)
}

// leadPaket: Name des Lead-Pakets (lead/<paket>.md) aus Projektart und Verfahren.
func leadPaket(info leadProjektInfo) string {
	art := info.Art
	if art == "" {
		art = "other"
	}
	switch info.Verfahren {
	case "clean_room":
		if art != "research" && art != "other" {
			return art + "-cleanroom"
		}
	case "continuation":
		if art != "research" && art != "other" {
			return art + "-continuation"
		}
	}
	return art
}

type leadGit struct {
	RepoGit      string  `json:"repo_git"`
	Token        *string `json:"token"`
	Hinweis      string  `json:"hint"`
	Fehlt        string  `json:"missing"`     // app_not_installed, private, no_write_access (GitHub ohne App-Verbindung)
	Installieren string  `json:"install_url"` // bei app_not_installed: die Installation der swarvex-App
	Autor        struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
}

func leadGitHolen(l *leadKonfig, zugriff string) (leadGit, error) {
	var g leadGit
	req, _ := http.NewRequest(http.MethodPost, l.Plattform+"/lead-service/git", nil)
	req.Header.Set("authorization", "Bearer "+zugriff)
	r, err := httpClient(l.CAPfad).Do(req)
	if err != nil {
		return g, err
	}
	defer r.Body.Close()
	roh, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if r.StatusCode != http.StatusOK {
		return g, fmt.Errorf("git access: HTTP %d: %s", r.StatusCode, strings.TrimSpace(string(roh)))
	}
	return g, json.Unmarshal(roh, &g)
}

// leadAnmeldung: die Anmeldung des Lead-Dienstes mit zwischengespeichertem Zugriffstoken (gilt eine Stunde; jede
// Erneuerung tauscht auch das Refresh-Token aus, darum nicht bei jedem Aufruf neu).
type leadAnmeldung struct {
	z    *zugangsdaten
	tok  string
	zeit time.Time
}

func (a *leadAnmeldung) token() (string, error) {
	if a.tok != "" && time.Since(a.zeit) < 40*time.Minute {
		return a.tok, nil
	}
	tok, err := zugriffstoken(a.z)
	if err != nil {
		return "", err
	}
	a.tok, a.zeit = tok, time.Now()
	return tok, nil
}

// leadMcp: MCP-Client mit der Anmeldung des Lead-Dienstes; ohne kurze Frist, weil wait_for_work bis zu 5 Minuten wartet.
func leadMcp(a *leadAnmeldung) (*mcpClient, string, error) {
	z := a.z
	tok, err := a.token()
	if err != nil {
		return nil, "", err
	}
	c := httpClient(z.CAPfad)
	c.Timeout = 11 * time.Minute
	m := &mcpClient{basis: z.Plattform, token: tok, http: c}
	return m, tok, m.initialisieren()
}

// gitHosts: Hosts, die der Lead-Proxy für Git öffnet. GitHub liefert Git-LFS-Dateien von eigenen Hosts aus.
func gitHosts(repoGit string) (string, error) {
	u, err := url.Parse(repoGit)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" {
		return "", fmt.Errorf("the lead service needs an https repository address (http only inside your own network), not %q", repoGit)
	}
	if u.Scheme == "http" {
		if ip := net.ParseIP(u.Hostname()); ip == nil || oeffentlicheAdresse(ip) {
			return "", fmt.Errorf("plain http only for a repository inside your own network, not %q", repoGit)
		}
	}
	h := strings.ToLower(u.Hostname())
	if h == "github.com" {
		return "github.com,*.githubusercontent.com,github-cloud.s3.amazonaws.com", nil
	}
	return h, nil
}

// ---- Einrichten ----

func leadEinrichten(p string, args []string) {
	nurLinux()
	if _, err := exec.LookPath("podman"); err != nil {
		fehlerAus("the lead service needs Podman (rootless), like the runner's sandbox")
	}
	gespeichert := gespeicherteArgs()
	vorgabe := func(name string) string { v, _ := argWert(gespeichert, name); return v }
	fs := flag.NewFlagSet("project lead setup", flag.ExitOnError)
	plattformWert := fs.String("platform", "", "platform address (default: where the runner is signed in)")
	agentName := fs.String("agent", envOderLeer(vorgabe("agent"), "claude"), "agent in the lead sandbox: "+agentNamen())
	modell := fs.String("model", vorgabe("model"), "model for the lead (default: the runner's model)")
	modellURL := fs.String("model-url", vorgabe("model-url"), "base URL of the model API (default: the runner's)")
	keyDatei := fs.String("model-key-file", vorgabe("model-key-file"), "key or subscription token file (default: the runner's)")
	gitToken := fs.String("git-token-file", "", "repositories outside GitHub: a file with a token that may push to this repository only")
	gitNutzer := fs.String("git-user", "x-access-token", "user name for --git-token-file (e.g. your login on Gitea or Codeberg)")
	tages := fs.Int64("daily-budget-tokens", 0, "at most this many tokens per day for the lead (0 = no limit; a subscription has its own)")
	ruhe := fs.Duration("idle-cycle", leadRuheStandard, "run a cycle after this long without work (plan and status)")
	rundeMax := fs.Duration("cycle-time-limit", leadRundeMaxStandard, "maximum duration of one cycle")
	ohneDienst := fs.Bool("no-service", false, "only prepare; do not set up the background service")
	bereich := fs.String("area", "", "a second lead service for one area of the project (e.g. combat), next to the main one; ideally with another agent or model")
	pfade := fs.String("paths", "", "with --area: the area's paths, comma-separated globs (e.g. game/combat/**,game/tests/combat/**)")
	faehigkeiten := fs.String("capabilities", "code.*,test.*", "with --area: the area's capabilities, comma-separated")
	fs.Parse(args)
	n := p // Name des Dienstes auf diesem Rechner
	liste := func(x string) []string {
		var r []string
		for _, t := range strings.Split(x, ",") {
			if t = strings.TrimSpace(t); t != "" {
				r = append(r, t)
			}
		}
		return r
	}
	if *bereich != "" {
		if !regexp.MustCompile(`^[a-z][a-z0-9-]{1,30}$`).MatchString(*bereich) {
			fehlerAus("--area: lowercase letters, digits and dashes, e.g. combat or ui")
		}
		if len(liste(*pfade)) == 0 {
			fehlerAus("--area needs --paths: the area's globs inside the repository, e.g. game/combat/**")
		}
		n = p + "." + *bereich
	}
	gesetzt := map[string]bool{}
	fs.Visit(func(fl *flag.Flag) { gesetzt[fl.Name] = true })
	if *modell == "" {
		// Kein Runner eingerichtet (der Einzeiler `install.sh | sh -s -- lead <projekt>`): das Modell hier wählen
		f, ok := neuerFrager()
		if !ok {
			fehlerAus("--model is required (the runner has no saved model yet)")
		}
		fmt.Println("== Model for the lead\nThe lead works with a model of your choice. Your key stays on this machine; the sandbox never sees it.")
		w, _ := modellWaehlen(f, "Which model should your lead use?")
		if !gesetzt["agent"] {
			*agentName = w.Agent
		}
		*modell, *modellURL = w.Modell, w.URL
		if !gesetzt["model-key-file"] {
			*keyDatei = w.Key
		}
	}
	agent := agentPruefen(*agentName)
	if *modellURL == "" && !codexAboDatei(agent.Name, *keyDatei) {
		fehlerAus("--model-url is required (not with Codex and a ChatGPT sign-in)")
	}

	basis, ca := plattformUndCA(*plattformWert)
	if basis == "" {
		basis = standardPlattform
	}
	if eigene := filepath.Join(konfigOrdner(), "ca.crt"); ca == "" {
		if _, err := os.Stat(eigene); err == nil { // Labor ohne Runner-Anmeldung: die CA, die install.sh abgelegt hat
			ca = eigene
		}
	}
	info, err := leadProjektLesen(basis, ca, p)
	pruefen(err)
	fmt.Printf("== Lead service for %s (%s) on %s\n", info.Name, p, basis)
	pruefen(os.MkdirAll(leadOrdner(n), 0o700))
	for _, teil := range []string{"work", "home", "cache", "log"} {
		pruefen(os.MkdirAll(leadUnter(n, teil), 0o700))
	}
	for _, teil := range []string{"run", "proxy"} { // für den Container lesbar (nur lesend eingebunden), der Ordner darüber bleibt 0700
		pruefen(os.MkdirAll(leadUnter(n, teil), 0o755))
	}

	// 1. Eigene Anmeldung, an das Projekt gebunden
	z, err := zugangLadenAus(leadZugangDatei(n))
	if err == nil && z.Plattform == basis {
		if _, err := zugriffstoken(z); err != nil {
			z = nil
		}
	} else {
		z = nil
	}
	if z == nil {
		host, _ := os.Hostname()
		fmt.Println("== Sign-in for the lead service (its own device, bound to this project)")
		z, err = geraetAnmelden(basis, ca, fmt.Sprintf("swarvex project lead: %s (%s)", n, host), leadZugangDatei(n))
		pruefen(err)
	}
	an := &leadAnmeldung{z: z}
	m, tok, err := leadMcp(an)
	pruefen(err)
	var bindung struct {
		Rolle        string   `json:"role"`
		Bereich      string   `json:"area"`
		Pfade        []string `json:"paths"`
		Faehigkeiten []string `json:"capabilities"`
	}
	if err := m.werkzeug("bind_lead_service", map[string]any{"project": p}, &bindung); err != nil {
		fehlerAus("binding the sign-in to " + p + ": " + err.Error())
	}
	bereichsLead := bindung.Rolle == "area_lead"
	if bereichsLead && *bereich != "" {
		fehlerAus("you are an area lead of " + p + ": your service runs your area " + bindung.Bereich + " by itself; leave out --area")
	}
	if *bereich != "" {
		if err := m.werkzeug("take_area", map[string]any{"project": p, "area": *bereich, "paths": liste(*pfade), "capabilities": liste(*faehigkeiten)}, nil); err != nil {
			fehlerAus("taking the area " + *bereich + ": " + err.Error())
		}
	}
	if bereichsLead {
		fmt.Printf("   ✓ signed in as area lead of %s; this service is bound to it\n", info.Name)
	} else {
		fmt.Printf("   ✓ signed in as lead of %s; this service is bound to it (only its lead tools; approvals stay with you)\n", info.Name)
	}
	if bereichsLead {
		fmt.Printf("   as area lead of %q: reviews and cuts tasks in your area; the repository is read-only for it\n", bindung.Bereich)
	} else if *bereich != "" {
		fmt.Printf("   runs the area %q; the main lead service keeps the rest\n", *bereich)
	}

	// 2. Git-Zugang
	g, err := leadGitHolen(&leadKonfig{Plattform: basis, CAPfad: ca}, tok)
	pruefen(err)
	if g.Token == nil && *gitToken == "" && g.Fehlt == "app_not_installed" && g.Installieren != "" {
		g = leadAufAppWarten(g, func() (leadGit, error) {
			t, err := an.token()
			if err != nil {
				return leadGit{}, err
			}
			return leadGitHolen(&leadKonfig{Plattform: basis, CAPfad: ca}, t)
		}, 5*time.Second, 30*time.Minute)
	}
	if g.Token == nil && *gitToken == "" {
		fehlerAus(g.Hinweis)
	}
	if *gitToken != "" {
		abs, err := filepath.Abs(*gitToken)
		pruefen(err)
		*gitToken = abs
	}
	l := &leadKonfig{Projekt: p, BereichsLead: bereichsLead, Plattform: basis, CAPfad: ca, Agent: agent.Name, Modell: *modell, ModellURL: *modellURL, KeyDatei: *keyDatei,
		GitTokenDatei: *gitToken, GitNutzer: *gitNutzer, Tagesbudget: *tages, Ruhe: ruhe.String(), RundeMax: rundeMax.String(),
		Paket: leadPaket(info), RepoGit: g.RepoGit, AutorName: g.Autor.Name, AutorEmail: g.Autor.Email}
	switch {
	case bereichsLead:
		l.Bereich, l.Pfade, l.Faehigkeiten = bindung.Bereich, bindung.Pfade, bindung.Faehigkeiten
	case *bereich != "":
		l.Name, l.Bereich, l.Pfade, l.Faehigkeiten = n, *bereich, liste(*pfade), liste(*faehigkeiten)
	}
	if _, err := gitHosts(l.RepoGit); err != nil {
		fehlerAus(err.Error())
	}
	if l.KeyDatei != "" {
		if abs, err := filepath.Abs(l.KeyDatei); err == nil {
			l.KeyDatei = abs
		}
	}
	pruefen(l.speichern())

	// 3. Klon, Umgebung, Image
	ctx := context.Background()
	pruefen(leadTokensSchreiben(l, an))
	if _, err := os.Stat(filepath.Join(leadUnter(n, "work"), ".git")); err != nil {
		fmt.Println("== Cloning", l.RepoGit, "inside the sandbox")
		if aus, err := leadGitAusfuehren(ctx, l, "git clone --quiet \"$SWARVEX_REPO\" /work"); err != nil {
			fehlerAus("cloning: " + strings.TrimSpace(letzteZeilen(aus, 15)+"\n"+err.Error()))
		}
	}
	fmt.Println("== Preparing the lead sandbox (the first time takes a few minutes)")
	if _, _, err := leadVorbereiten(ctx, l); err != nil {
		fehlerAus(err.Error())
	}
	fmt.Println("   project environment, agent, git and a virtual display ready")

	if *ohneDienst {
		fmt.Printf("\nReady. One cycle now: swarvex project lead cycle %s · as a service: swarvex project lead setup %s (same options, without --no-service)\n", n, p)
		return
	}
	// 4. Dienst
	programm, err := os.Executable()
	pruefen(err)
	pruefen(os.MkdirAll(filepath.Dir(leadDienstDatei(n)), 0o755))
	pruefen(os.WriteFile(leadDienstDatei(n), []byte(leadDienstEinheit(programm, n)), 0o644))
	for _, a := range [][]string{{"daemon-reload"}, {"enable", leadDienstName(n)}, {"restart", leadDienstName(n)}} {
		if aus, err := systemctl(a...); err != nil {
			fehlerAus("systemctl --user " + strings.Join(a, " ") + ": " + aus)
		}
	}
	if bereichsLead || *bereich != "" {
		fmt.Printf("\n✓ The lead service %s runs as a service in its sandbox.\n", n)
	} else {
		fmt.Printf("\n✓ You are the lead of %s. Your lead service runs in its sandbox and starts its first cycle now.\n", info.Name)
		fmt.Printf("  The project page shows it as active within a minute: %s/en/projects/%s\n", basis, p)
	}
	fmt.Printf("  Talk to it (e.g. the project interview): swarvex project lead attach %s\n", n)
	fmt.Printf("  State: swarvex project lead status %s · Log: swarvex project lead logs %s\n", n, n)
	if n == p && !bereichsLead {
		fmt.Println("  If a lead session (Claude Code, Codex …) led this project before, end it: from now on the service is the lead.")
	}
	if aus, _ := exec.Command("loginctl", "show-user", os.Getenv("USER"), "-p", "Linger", "--value").Output(); strings.TrimSpace(string(aus)) != "yes" && !istWSL() {
		fmt.Println("  Note: the service only runs while you are logged in. To keep it working without a login, run once: loginctl enable-linger")
	}
}

func envOderLeer(wert, standard string) string {
	if wert != "" {
		return wert
	}
	return standard
}

func leadDienstEinheit(programm, p string) string {
	return `# Set up by "swarvex project lead setup ` + p + `". Remove: swarvex project lead remove ` + p + `.
[Unit]
Description=swarvex lead service for ` + p + ` (the project's lead in its own sandbox)
Documentation=https://swarvex.ai/en/docs/lead-service
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=` + systemdQuote(programm) + ` project lead run ` + p + `
Restart=on-failure
RestartSec=60
KillMode=control-group
TimeoutStopSec=120
Nice=5

[Install]
WantedBy=default.target
`
}

func leadSystemctl(p, was string) {
	nurLinux()
	if _, err := os.Stat(leadDienstDatei(p)); err != nil {
		fehlerAus("no lead service set up for " + p + ": swarvex project lead setup " + p)
	}
	if aus, err := systemctl(was, leadDienstName(p)); err != nil {
		fehlerAus(aus)
	}
	fmt.Printf("Lead service for %s: %s\n", p, map[string]string{"start": "running", "stop": "stopped (a running cycle ended)"}[was])
}

// ---- Bilder und Container ----

const leadGitImage = "swarvex-lead-git"

// Was der Lead über die Projekt-Umgebung hinaus braucht: Git mit LFS und einen virtuellen Bildschirm mit
// Software-Rendering (Screenshots, Editor, das Spiel). Danach wieder ohne Setuid-Programme.
const leadSchicht = "RUN if ! command -v apt-get >/dev/null; then echo 'the lead sandbox needs a Debian or Ubuntu based project image' >&2; exit 1; fi \\\n" +
	" && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends git git-lfs ca-certificates xvfb xauth \\\n" +
	"    libgl1 libgl1-mesa-dri libglx-mesa0 libegl1 libx11-6 libxcursor1 libxinerama1 libxrandr2 libxi6 libxext6 libxrender1 libxkbcommon0 fontconfig \\\n" +
	" && rm -rf /var/lib/apt/lists/* && git lfs install --system >/dev/null \\\n" +
	" && (find / -xdev -perm -4000 -type f -exec chmod u-s {} + 2>/dev/null; true)\n"

func leadGitImageBereit(ctx context.Context) error {
	if _, err := podmanAus(ctx, "", "image", "exists", leadGitImage); err == nil {
		return nil
	}
	leer, err := os.MkdirTemp("", "swarvex-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(leer)
	datei := "FROM docker.io/library/debian:trixie-slim\n" +
		"RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends git git-lfs ca-certificates && rm -rf /var/lib/apt/lists/* \\\n" +
		" && git lfs install --system >/dev/null && (find / -xdev -perm -4000 -type f -exec chmod u-s {} + 2>/dev/null; true)\nUSER 1000:1000\n"
	if aus, err := podmanAus(ctx, datei, "build", "-q", "-t", leadGitImage, "-f", "-", leer); err != nil {
		return fmt.Errorf("building the git image: %s", letzteZeilen(aus, 15))
	}
	return nil
}

// leadVorbereiten: Umgebung des Projekts im Klon ermitteln, Lead-Image bauen (einmal je Umgebung und Agent-Version)
// und die Abhängigkeiten einrichten, wenn sich der Stand seit dem letzten Mal geändert hat.
func leadVorbereiten(ctx context.Context, l *leadKonfig) (umgebung.Umgebung, string, error) {
	baum := leadUnter(l.dienst(), "work")
	u, err := umgebungErmitteln(baum)
	if err != nil {
		return u, "", fmt.Errorf("project environment: %v", err)
	}
	agent := agenten[l.Agent]
	if err := agentImageBereit(ctx, agent); err != nil {
		return u, "", err
	}
	env, err := umgebungsImage(ctx, u, baum, agent.Image)
	if err != nil {
		return u, "", err
	}
	h := sha256.Sum256([]byte(env + "\n" + leadSchicht))
	image := "swarvex-lead-" + hex.EncodeToString(h[:])[:16]
	if _, err := podmanAus(ctx, "", "image", "exists", image); err != nil {
		leer, err := os.MkdirTemp("", "swarvex-build-")
		if err != nil {
			return u, "", err
		}
		defer os.RemoveAll(leer)
		if aus, err := podmanAus(ctx, "FROM "+env+"\nUSER root\n"+leadSchicht+"USER 1000:1000\n", "build", "-q", "-t", image, "-f", "-", leer); err != nil {
			return u, "", fmt.Errorf("building the lead image: %s", letzteZeilen(aus, 20))
		}
	}
	// Abhängigkeiten (mit Netz über den Ausgangs-Proxy, ohne Agent, ohne Schlüssel), nur wenn sich der Stand geändert hat
	stand := gitStand(baum)
	if alt, _ := os.ReadFile(leadUnter(l.dienst(), "setup-head")); stand == "" || strings.TrimSpace(string(alt)) != stand {
		if err := einrichten(ctx, u, env, baum, leadUnter(l.dienst(), "cache"), leadUnter(l.dienst(), filepath.Join("log", "setup.log"))); err != nil {
			fmt.Println("   setting up dependencies failed (the lead can still work; details in log/setup.log):", letzteZeilen(err.Error(), 3))
		} else if stand != "" {
			os.WriteFile(leadUnter(l.dienst(), "setup-head"), []byte(stand+"\n"), 0o600)
		}
	}
	return u, image, nil
}

// gitStand liest den Commit von HEAD aus dem Klon (nur Dateien lesen, kein git auf dem Rechner).
func gitStand(baum string) string {
	kopf, err := os.ReadFile(filepath.Join(baum, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	ref, ok := strings.CutPrefix(strings.TrimSpace(string(kopf)), "ref: ")
	if !ok {
		return strings.TrimSpace(string(kopf))
	}
	if roh, err := os.ReadFile(filepath.Join(baum, ".git", filepath.FromSlash(ref))); err == nil {
		return strings.TrimSpace(string(roh))
	}
	gepackt, _ := os.ReadFile(filepath.Join(baum, ".git", "packed-refs"))
	for _, z := range strings.Split(string(gepackt), "\n") {
		if f := strings.Fields(z); len(f) == 2 && f[1] == ref {
			return f[0]
		}
	}
	return ""
}

// leadGitUmgebung: Git im Container geht über den Proxy und meldet sich mit dem Token aus /run/swarvex an.
func leadGitUmgebung(l *leadKonfig, proxy string) []string {
	nutzer := l.GitNutzer
	if nutzer == "" || l.GitTokenDatei == "" {
		nutzer = "x-access-token"
	}
	werte := [][2]string{
		{"http.proxy", "http://" + proxy + ":3128"},
		{"credential.helper", `!f() { test "$1" = get || exit 0; echo username=` + nutzer + `; printf 'password=%s\n' "$(cat /run/swarvex/git-token)"; }; f`},
		{"user.name", l.AutorName}, {"user.email", l.AutorEmail},
		{"safe.directory", "*"}, {"pull.rebase", "true"},
	}
	args := []string{"-e", fmt.Sprintf("GIT_CONFIG_COUNT=%d", len(werte))}
	for i, w := range werte {
		args = append(args, "-e", fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", i, w[0]), "-e", fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", i, w[1]))
	}
	return args
}

// leadContainerArgs: gemeinsame, abgeschottete Einstellungen des Lead-Containers (Git, Runde, Gespräch).
func leadContainerArgs(l *leadKonfig, name, netz string, hosts []string, sekunden int) []string {
	p := l.dienst()
	args := []string{"run", "--rm", "--name", name, "--timeout", fmt.Sprint(sekunden),
		"--cap-drop=ALL", "--security-opt", "no-new-privileges", "--read-only",
		"--tmpfs", "/tmp:rw,exec,size=4g,mode=1777", "--shm-size", "512m",
		"--userns=keep-id:uid=1000,gid=1000", "--user", "1000:1000", "--memory", "6g", "--cpus", "4", "--pids-limit", "2048",
		"-v", leadUnter(p, "work") + ":/work:Z", "-v", leadUnter(p, "home") + ":/home/agent:Z", "-v", leadUnter(p, "cache") + ":/cache:Z",
		"-v", leadUnter(p, "run") + ":/run/swarvex:ro,Z", "-w", "/work", "--network", netz, "-e", "HOME=/home/agent"}
	args = append(args, hosts...)
	for _, v := range umgebung.CacheVariablen {
		args = append(args, "-e", v)
	}
	return args
}

// leadProxyBetrieb: der Lead-Proxy im Sandbox-Netz (und nach außen). Liefert das Wegwerf-Token des Containers und
// den /etc/hosts-Eintrag, unter dem er ihn erreicht. Aufräumen: podman rm -f name.
func leadProxyBetrieb(ctx context.Context, l *leadKonfig, name, netz string, zugang modellZugang, budget int64) (string, []string, error) {
	agent := agenten[l.Agent]
	hostsGit, err := gitHosts(l.RepoGit)
	if err != nil {
		return "", nil, err
	}
	tok := zufall(36)
	if zugang.chatgpt() {
		tok = codexSandboxToken(tok)
	}
	exec.CommandContext(ctx, "podman", "rm", "-f", name).Run()
	args := []string{"run", "-d", "--name", name, "--network", netz, "--network", "podman", "--timeout", "86400",
		"--read-only", "--cap-drop=ALL", "--security-opt", "no-new-privileges", "--memory", "256m", "--pids-limit", "128",
		"-v", leadUnter(l.dienst(), "proxy") + ":/run/lead:ro,Z",
		"-e", "SWARVEX_UPSTREAM=" + zugang.URL, "-e", "SWARVEX_API=" + agent.API, "-e", "SWARVEX_MODEL=" + l.Modell, "-e", "SWARVEX_TASK_TOKEN=" + tok,
		"-e", fmt.Sprintf("SWARVEX_BUDGET_TOKENS=%d", budget), "-e", "SWARVEX_PLATFORM=" + l.Plattform, "-e", "SWARVEX_GIT_HOSTS=" + hostsGit}
	if zugang.Key != "" {
		args = append(args, "-e", "SWARVEX_UPSTREAM_KEY="+zugang.Key)
	}
	if zugang.chatgpt() {
		args = append(args, "-e", "SWARVEX_ACCOUNT_ID="+zugang.Konto)
	}
	if l.CAPfad != "" {
		args = append(args, "-v", l.CAPfad+":/run/lead-ca.pem:ro,z", "-e", "SWARVEX_PLATFORM_CA=/run/lead-ca.pem")
	}
	if aus, err := exec.CommandContext(ctx, "podman", append(args, "swarvex-proxy", "lead")...).CombinedOutput(); err != nil {
		return "", nil, fmt.Errorf("starting the lead proxy: %s", strings.TrimSpace(string(aus)))
	}
	hosts, err := proxyHost(ctx, name)
	return tok, hosts, err
}

// leadGitAusfuehren: ein Git-Befehl im Klon, im kleinen Git-Image, ins Netz nur über den Lead-Proxy.
func leadGitAusfuehren(ctx context.Context, l *leadKonfig, befehl string) (string, error) {
	if err := leadGitImageBereit(ctx); err != nil {
		return "", err
	}
	netz, err := internesNetz(ctx)
	if err != nil {
		return "", err
	}
	proxy := "swarvex-leadproxy-" + l.dienst()
	zugang, err := zugangLesen(agenten[l.Agent], l.KeyDatei, l.ModellURL, time.Hour)
	if err != nil {
		return "", err
	}
	_, hosts, err := leadProxyBetrieb(ctx, l, proxy, netz, zugang, 1)
	defer exec.Command("podman", "rm", "-f", proxy).Run()
	if err != nil {
		return "", err
	}
	name := "swarvex-leadgit-" + l.dienst()
	exec.Command("podman", "rm", "-f", name).Run()
	args := append(leadContainerArgs(l, name, netz, hosts, 1800), leadGitUmgebung(l, proxy)...)
	args = append(args, "-e", "SWARVEX_REPO="+l.RepoGit, leadGitImage, "sh", "-ec", befehl)
	ctx, stopp := context.WithTimeout(ctx, 30*time.Minute)
	defer stopp()
	aus, err := exec.CommandContext(ctx, "podman", args...).CombinedOutput()
	if err != nil {
		// Git schweigt manchmal (etwa wenn die Verbindung über den Proxy abbricht): dann wenigstens den Grund und
		// was der Proxy gesehen hat
		aus = append(aus, []byte("\n[swarvex] "+err.Error())...)
		if log, _ := exec.Command("podman", "logs", proxy).CombinedOutput(); strings.Contains(string(log), "egress_blocked") {
			aus = append(aus, []byte("\n[swarvex] blocked hosts: "+gesperrteHosts(string(log)))...)
		} else if z := letzteZeilen(string(log), 5); z != "" {
			aus = append(aus, []byte("\n[swarvex] lead proxy: "+z)...)
		}
	}
	return string(aus), err
}

func gesperrteHosts(protokoll string) string {
	var hosts []string
	for _, z := range strings.Split(protokoll, "\n") {
		var e struct {
			Art  string `json:"type"`
			Host string `json:"host"`
		}
		if json.Unmarshal([]byte(z), &e) == nil && e.Art == "egress_blocked" && !strings.Contains(strings.Join(hosts, ","), e.Host) {
			hosts = append(hosts, e.Host)
		}
	}
	return strings.Join(hosts, ", ")
}

// leadTokensSchreiben: frische Zugänge für den Proxy (Plattform) und für Git, als Dateien (atomar ersetzt).
func leadTokensSchreiben(l *leadKonfig, a *leadAnmeldung) error {
	tok, err := a.token()
	if err != nil {
		return err
	}
	if err := atomarSchreiben(leadUnter(l.dienst(), filepath.Join("proxy", "access-token")), tok); err != nil {
		return err
	}
	git := ""
	if l.GitTokenDatei != "" {
		roh, err := os.ReadFile(l.GitTokenDatei)
		if err != nil {
			return fmt.Errorf("--git-token-file: %v", err)
		}
		git = strings.TrimSpace(string(roh))
	} else {
		g, err := leadGitHolen(l, tok)
		if err != nil {
			return err
		}
		if g.Token == nil {
			return errors.New(g.Hinweis)
		}
		git = *g.Token
	}
	return atomarSchreiben(leadUnter(l.dienst(), filepath.Join("run", "git-token")), git)
}

// leadTokensFrischHalten: solange gearbeitet wird, alle 5 Minuten frische Zugänge (das Git-Token der GitHub App
// kann beim Abholen schon zur Hälfte verbraucht sein). Aufhören: die gelieferte Funktion aufrufen.
func leadTokensFrischHalten(l *leadKonfig, a *leadAnmeldung) func() {
	fertig := make(chan struct{})
	go func() {
		for {
			select {
			case <-fertig:
				return
			case <-time.After(5 * time.Minute):
				if err := leadTokensSchreiben(l, a); err != nil {
					fmt.Println("Refreshing the lead's access:", err)
				}
			}
		}
	}()
	return func() { close(fertig) }
}

func atomarSchreiben(datei, inhalt string) error {
	if err := os.WriteFile(datei+".neu", []byte(inhalt+"\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(datei+".neu", datei)
}

// ---- Runden ----

type leadAnlass struct {
	Reviews     []int64
	Nachrichten int
	Nachschub   bool
	Frei, Aktiv int
	// Bereitschaft: ein anderer Lead-Dienst des Projekts hält die Hauptschleife (E-39); dieser startet keine Runde
	Bereitschaft bool
	Halter       string
}

func (a leadAnlass) arbeit() bool { return len(a.Reviews) > 0 || a.Nachrichten > 0 }

func (a leadAnlass) signatur() string {
	r := append([]int64{}, a.Reviews...)
	sort.Slice(r, func(i, j int) bool { return r[i] < r[j] })
	return fmt.Sprint(r, a.Nachrichten)
}

func (a leadAnlass) text() string {
	var t []string
	if len(a.Reviews) > 0 {
		t = append(t, fmt.Sprintf("%d checked submission(s) wait for your review", len(a.Reviews)))
	}
	if a.Nachrichten > 0 {
		t = append(t, fmt.Sprintf("%d open message(s) to you", a.Nachrichten))
	}
	if a.Nachschub {
		t = append(t, fmt.Sprintf("only %d free task(s) for %d active agent(s): cut more work", a.Frei, a.Aktiv))
	}
	return strings.Join(t, "; ")
}

// leadWarten: wait_for_work der Plattform, ohne Agent und ohne Tokens. Hält zugleich die Anwesenheit aktuell.
func leadWarten(m *mcpClient, p string, sekunden int) (leadAnlass, error) {
	var w struct {
		Reviews []struct {
			ID int64 `json:"submission_id"`
		} `json:"to_review"`
		Nachrichten int  `json:"messages_open"`
		Nachschub   bool `json:"backlog_low"`
		Angebot     struct {
			Frei  int `json:"free"`
			Aktiv int `json:"active"`
		} `json:"supply"`
		Bereitschaft bool   `json:"standby"`
		Halter       string `json:"holder"`
	}
	err := m.werkzeug("wait_for_work", map[string]any{"project": p, "max_seconds": sekunden, "wake_on": []string{"reviews", "messages", "backlog"}}, &w)
	a := leadAnlass{Nachrichten: w.Nachrichten, Nachschub: w.Nachschub, Frei: w.Angebot.Frei, Aktiv: w.Angebot.Aktiv, Bereitschaft: w.Bereitschaft, Halter: w.Halter}
	for _, r := range w.Reviews {
		a.Reviews = append(a.Reviews, r.ID)
	}
	return a, err
}

// leadEntscheiden: Soll jetzt eine Runde laufen, und mit welchem Anlass? Neue Arbeit sofort; dieselbe Arbeit, die nach
// einer Runde noch da ist, und zu wenig Nachschub höchstens einmal je Stunde; sonst nach der Ruhezeit eine Routinerunde.
func leadEntscheiden(a leadAnlass, letzteSignatur string, letzteRunde time.Time, ruhe time.Duration, jetzt time.Time) (bool, string) {
	seit := jetzt.Sub(letzteRunde)
	switch {
	case a.Bereitschaft:
		return false, ""
	case letzteRunde.IsZero():
		if a.arbeit() || a.Nachschub {
			return true, a.text()
		}
		return true, "the lead service started: read the situation, plan and status"
	case seit < leadAbstand:
		return false, ""
	case a.arbeit() && a.signatur() != letzteSignatur:
		return true, a.text()
	case (a.arbeit() || a.Nachschub) && seit >= leadWiederholung:
		return true, a.text() + " (still open after the last cycle)"
	case seit >= ruhe:
		return true, "routine cycle: no new work for a while; read the situation, keep plan and status current"
	}
	return false, ""
}

const leadAuftragVorlage = `You are the lead of the swarvex project "%[1]s" and run as its lead service: a sandbox with the project's environment that the swarvex runner starts for one cycle at a time.

Why this cycle runs now: %[2]s.

- Your instructions are the lead handbook in your system prompt. Work through one cycle of "Every cycle": review first, then read the situation, act on the signals, update plan and status.
- The repository is a git clone in /work (origin is the project's repository); the runner pulled origin/main before this cycle%[3]s. Commit and push your own work as usual; if the push is rejected, rebase onto origin/main and push again.
- The swarvex tools are connected as the MCP server "swarvex" and work only for this project. Do not call wait_for_work: the runner waits for you and starts the next cycle when there is work.
- Everything you run stays in this sandbox: tests, the editor, the game. A virtual display is set (DISPLAY=:99) for screenshots and windowed runs; Godot needs --rendering-driver opengl3 here (software rendering, no GPU).
- Network: only the model, swarvex and the git host. Packages cannot be installed from the internet here; dependencies are set up by the runner from the project's environment (swarvex.yaml).
- Approvals belong to the visionary: ask with send_message (kind approval) and continue with other work. You cannot decide approvals yourself.
- Keep docs/status.md current as the handbook says: the next cycle starts from it. End the cycle when the queue is worked through.
`

// leadGespraech: was der Lead im Gespräch (swarvex project lead attach) zusätzlich wissen muss.
const leadGespraech = `

## This conversation (swarvex project lead attach)

You run as this project's lead service, and a person is talking to you directly in a terminal. The runner waits for
work and starts your cycles between conversations: do not call wait_for_work and do not start a work loop here. Answer,
do what the person asks, keep docs/status.md current, and let them end the conversation. While they are connected,
the service skips its own cycles.
`

// leadBereichZeile: für Status und Protokoll, z. B. " (area combat)"
func leadBereichZeile(l *leadKonfig) string {
	switch {
	case l.BereichsLead:
		return fmt.Sprintf(" (area lead of %q in %s, reads the repository only)", l.Bereich, l.Projekt)
	case l.Bereich != "":
		return fmt.Sprintf(" (area %q of %s)", l.Bereich, l.Projekt)
	}
	return ""
}

// leadBereichText: Zusatz zum Auftrag für einen Dienst mit Bereich (E-39).
func leadBereichText(l *leadKonfig) string {
	if l.Bereich == "" {
		return ""
	}
	pfade := strings.Join(l.Pfade, ", ")
	if l.BereichsLead {
		return fmt.Sprintf("- You are the area lead for %q (paths %s), not the project's lead: create, review and close tasks only inside your area (levels A and B, C when the lead released it); level D and everything outside belongs to the lead. The handbook's area-lead rules apply (lead/AREA_LEAD.md). This clone is read-only for you: never push; your own code goes through tasks like everyone else's.\n", l.Bereich, pfade)
	}
	return fmt.Sprintf("- This lead service runs the area %q (paths %s) of the project: pending_submissions and wait_for_work show you only that area. The main lead service handles everything else; coordinate with it through send_message (to: lead) instead of working outside your area.\n", l.Bereich, pfade)
}

// leadRunde: eine Runde des Agenten im Lead-Container. Liefert Verbrauch und Ausgang.
func leadRunde(ctx context.Context, l *leadKonfig, a *leadAnmeldung, anlass string, budget int64) (int64, string, error) {
	tokens, ergebnis, _, err := leadImContainer(ctx, l, a, anlass, budget, "")
	return tokens, ergebnis, err
}

// leadImContainer: eine Runde des Agenten, oder statt seiner ein Befehl (swarvex project lead exec). Liefert auch die Ausgabe.
func leadImContainer(ctx context.Context, l *leadKonfig, a *leadAnmeldung, anlass string, budget int64, statt string) (int64, string, string, error) {
	p := l.dienst()
	agent := agenten[l.Agent]
	if err := leadTokensSchreiben(l, a); err != nil {
		return 0, "", "", err
	}
	defer leadTokensFrischHalten(l, a)()
	// Vor der Runde den neuesten Stand holen (nur vorspulen; eigene, noch nicht gepushte Arbeit bleibt)
	hinweis := ""
	zug, err := leadGitAusfuehren(ctx, l, "git fetch --quiet origin; if ! git merge --ff-only --quiet '@{upstream}' >/dev/null 2>&1; then echo SWARVEX_DIVERGED; fi; git lfs pull >/dev/null 2>&1 || true")
	switch {
	case err != nil:
		hinweis = " (the pull failed: " + strings.TrimSpace(letzteZeilen(zug, 3)+" "+err.Error()) + ")"
	case strings.Contains(zug, "SWARVEX_DIVERGED"):
		hinweis = " (it could not fast-forward: your local commits or changes diverge from origin/main; rebase them)"
	}
	_, image, err := leadVorbereiten(ctx, l)
	if err != nil {
		return 0, "", "", err
	}
	paket, err := leadPaketHolen(l)
	if err != nil {
		return 0, "", "", err
	}
	if err := os.WriteFile(leadUnter(p, filepath.Join("run", "LEAD.md")), []byte(paket), 0o644); err != nil {
		return 0, "", "", err
	}
	if err := os.WriteFile(leadUnter(p, filepath.Join("run", "cycle.md")), []byte(fmt.Sprintf(leadAuftragVorlage, l.Projekt, anlass, hinweis)+leadBereichText(l)), 0o644); err != nil {
		return 0, "", "", err
	}
	netz, err := internesNetz(ctx)
	if err != nil {
		return 0, "", "", err
	}
	zugang, err := zugangLesen(agent, l.KeyDatei, l.ModellURL, l.dauer(l.RundeMax, leadRundeMaxStandard))
	if err != nil {
		return 0, "", "", err
	}
	proxy := "swarvex-leadproxy-" + p
	tok, hosts, err := leadProxyBetrieb(ctx, l, proxy, netz, zugang, budget)
	defer exec.Command("podman", "rm", "-f", proxy).Run()
	if err != nil {
		return 0, "", "", err
	}
	agentArgs, befehl, err := leadAgent(l, agent, proxy, tok, zugang, false)
	if err != nil {
		return 0, "", "", err
	}
	if statt != "" {
		befehl = statt
	}
	name := "swarvex-lead-" + p
	exec.Command("podman", "rm", "-f", name).Run()
	frist := l.dauer(l.RundeMax, leadRundeMaxStandard)
	args := append(leadContainerArgs(l, name, netz, hosts, int(frist.Seconds())+60), leadGitUmgebung(l, proxy)...)
	args = append(append(args, agentArgs...), image, "sh", "-c", leadBildschirm+befehl)
	rctx, stopp := context.WithTimeout(ctx, frist)
	defer stopp()
	cmd := exec.CommandContext(rctx, "podman", args...)
	cmd.Cancel = func() error { exec.Command("podman", "kill", name).Run(); return cmd.Process.Kill() }
	cmd.WaitDelay = 15 * time.Second
	aus, err := cmd.CombinedOutput()
	exec.Command("podman", "rm", "-f", name).Run()
	protokoll, _ := exec.Command("podman", "logs", proxy).CombinedOutput()
	verbrauch := verbrauchAusProxy(string(protokoll))
	stempel := time.Now().Format("20060102-150405")
	os.WriteFile(leadUnter(p, filepath.Join("log", stempel+".log")), aus, 0o600)
	os.WriteFile(leadUnter(p, filepath.Join("log", stempel+".proxy.log")), protokoll, 0o600)
	logsAufraeumen(leadUnter(p, "log"), 60)
	tokens := int64(verbrauch["budget_tokens"])
	switch {
	case rctx.Err() != nil && ctx.Err() == nil:
		return tokens, "time limit reached", string(aus), nil
	case ctx.Err() != nil:
		return tokens, "stopped", string(aus), ctx.Err()
	case err != nil:
		return tokens, "agent error: " + letzteZeilen(string(aus), 2), string(aus), nil
	}
	return tokens, "ok", string(aus), nil
}

// Ein virtueller Bildschirm für Screenshots und Fenster (Godot, Browser); fehlt Xvfb, geht es ohne weiter.
const leadBildschirm = "if command -v Xvfb >/dev/null; then Xvfb :99 -screen 0 1920x1080x24 -nolisten tcp >/tmp/xvfb.log 2>&1 & export DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1; fi; "

// leadAgent: Umgebung und Befehl des Agenten im Lead-Container. Anleitung (Lead-Paket) und Auftrag liegen in
// /run/swarvex; die swarvex-Werkzeuge kommen über den Proxy. gespraech: im Terminal, statt einer Runde ohne Rückfragen.
func leadAgent(l *leadKonfig, agent agentArt, proxy, tok string, zugang modellZugang, gespraech bool) ([]string, string, error) {
	proxyURL := "http://" + proxy + ":8080"
	args := []string{"-e", "SWARVEX_TOKEN=" + tok, "-e", "SWARVEX_PROXY=" + proxyURL, "-e", "SWARVEX_MODEL=" + l.Modell}
	switch agent.Name {
	case "claude":
		mcp, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"swarvex": map[string]any{
			"type": "http", "url": proxyURL + "/mcp", "headers": map[string]string{"Authorization": "Bearer " + tok}}}})
		if err := os.WriteFile(leadUnter(l.dienst(), filepath.Join("run", "mcp.json")), mcp, 0o644); err != nil {
			return nil, "", err
		}
		for _, v := range []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL", "ANTHROPIC_SMALL_FAST_MODEL", "CLAUDE_CODE_SUBAGENT_MODEL"} {
			args = append(args, "-e", v+"="+l.Modell)
		}
		args = append(args, "-e", "ANTHROPIC_BASE_URL="+proxyURL, "-e", "ANTHROPIC_AUTH_TOKEN="+tok,
			"-e", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "-e", "DISABLE_AUTOUPDATER=1", "-e", "DISABLE_TELEMETRY=1",
			"-e", "DISABLE_ERROR_REPORTING=1", "-e", "IS_SANDBOX=1")
		gemeinsam := `claude --dangerously-skip-permissions --model "$SWARVEX_MODEL" --mcp-config /run/swarvex/mcp.json --strict-mcp-config --append-system-prompt "$(cat /run/swarvex/LEAD.md)"`
		if gespraech {
			return args, gemeinsam + " --continue || exec " + gemeinsam, nil
		}
		return args, "exec " + gemeinsam + ` -p --output-format text "$(cat /run/swarvex/cycle.md)"`, nil

	case "codex":
		// Codex liest ~/.codex/AGENTS.md als eigene Anleitung: dorthin das Lead-Paket (das AGENTS.md im Repo bleibt die
		// Regel für Mitstreiter). Anbieter und MCP-Server per -c, das Token der Werkzeuge aus SWARVEX_TOKEN.
		codexOrdner := leadUnter(l.dienst(), filepath.Join("home", ".codex"))
		if err := os.MkdirAll(codexOrdner, 0o700); err != nil {
			return nil, "", err
		}
		anbieter := fmt.Sprintf("{name=%s, base_url=%s, env_key=\"SWARVEX_TOKEN\", wire_api=\"responses\"}", tomlText("swarvex proxy"), tomlText(proxyURL+"/v1"))
		if zugang.chatgpt() {
			pfad := leadUnter(l.dienst(), filepath.Join("run", "codex-auth.json"))
			if err := os.WriteFile(pfad, codexSandboxAnmeldung(tok, zugang.Plan), 0o644); err != nil {
				return nil, "", err
			}
			args = append(args, "-v", pfad+":/home/agent/.codex/auth.json:ro,z")
			anbieter = fmt.Sprintf("{name=%s, base_url=%s, requires_openai_auth=true, wire_api=\"responses\"}", tomlText("swarvex proxy"), tomlText(proxyURL+"/v1"))
		}
		einstellungen := fmt.Sprintf(`-c %s -c 'model_provider="swarvex"' -c check_for_update_on_startup=false -c features.apps=false -c %s -c 'mcp_servers.swarvex.bearer_token_env_var="SWARVEX_TOKEN"' --model "$SWARVEX_MODEL"`,
			shellText("model_providers.swarvex="+anbieter), shellText("mcp_servers.swarvex.url="+tomlText(proxyURL+"/mcp")))
		vorher := "cp /run/swarvex/LEAD.md ~/.codex/AGENTS.md && "
		if gespraech {
			return args, vorher + "exec codex --dangerously-bypass-approvals-and-sandbox " + einstellungen, nil
		}
		return args, vorher + `exec codex exec --dangerously-bypass-approvals-and-sandbox --color never ` + einstellungen + ` "$(cat /run/swarvex/cycle.md)"`, nil

	case "opencode":
		konfig := map[string]any{
			"$schema": "https://opencode.ai/config.json", "autoupdate": false, "share": "disabled", "model": "swarvex/" + l.Modell,
			"instructions": []string{"/run/swarvex/LEAD.md"},
			"provider": map[string]any{"swarvex": map[string]any{
				"npm": "@ai-sdk/openai-compatible", "name": "swarvex proxy",
				"options": map[string]any{"baseURL": proxyURL + "/v1", "apiKey": "{env:SWARVEX_TOKEN}"},
				"models":  map[string]any{l.Modell: map[string]any{"name": l.Modell, "tool_call": true}},
			}},
			"mcp": map[string]any{"swarvex": map[string]any{"type": "remote", "url": proxyURL + "/mcp", "headers": map[string]string{"Authorization": "Bearer {env:SWARVEX_TOKEN}"}}},
		}
		roh, _ := json.MarshalIndent(konfig, "", "  ")
		if err := os.WriteFile(leadUnter(l.dienst(), filepath.Join("run", "opencode.json")), roh, 0o644); err != nil {
			return nil, "", err
		}
		args = append(args, "-e", "OPENCODE_CONFIG=/run/swarvex/opencode.json", "-e", "OPENCODE_DISABLE_AUTOUPDATE=1", "-e", "OPENCODE_DISABLE_MODELS_FETCH=1")
		if gespraech {
			return args, `exec opencode -m "swarvex/$SWARVEX_MODEL"`, nil
		}
		return args, `exec opencode run --pure --auto -m "swarvex/$SWARVEX_MODEL" "$(cat /run/swarvex/cycle.md)"`, nil
	}
	return nil, "", fmt.Errorf("unknown agent %q (available: %s)", agent.Name, agentNamen())
}

func shellText(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// leadPaketHolen: das Lead-Paket der Projektart von der Plattform (aktuell zu jeder Runde; ohne Netz das letzte).
func leadPaketHolen(l *leadKonfig) (string, error) {
	r, err := httpClient(l.CAPfad).Get(l.Plattform + "/lead/" + l.Paket + ".md")
	if err == nil {
		defer r.Body.Close()
		if r.StatusCode == http.StatusOK {
			roh, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err == nil && len(roh) > 1000 {
				os.WriteFile(leadUnter(l.dienst(), "LEAD.cache.md"), roh, 0o600)
				return string(roh), nil
			}
		}
	}
	if roh, err := os.ReadFile(leadUnter(l.dienst(), "LEAD.cache.md")); err == nil {
		return string(roh), nil
	}
	return "", fmt.Errorf("could not load the lead handbook %s/lead/%s.md", l.Plattform, l.Paket)
}

func logsAufraeumen(ordner string, behalten int) {
	eintraege, _ := os.ReadDir(ordner)
	var namen []string
	for _, e := range eintraege {
		if strings.HasSuffix(e.Name(), ".log") && e.Name() != "setup.log" {
			namen = append(namen, e.Name())
		}
	}
	sort.Strings(namen)
	for len(namen) > behalten {
		os.Remove(filepath.Join(ordner, namen[0]))
		namen = namen[1:]
	}
}

type leadRundeEintrag struct {
	Zeit     time.Time `json:"time"`
	Sekunden int       `json:"seconds"`
	Tokens   int64     `json:"tokens"`
	Anlass   string    `json:"reason"`
	Ergebnis string    `json:"outcome"`
}

func leadRundeMerken(p string, e leadRundeEintrag) {
	roh, _ := json.Marshal(e)
	f, err := os.OpenFile(leadRundenDatei(p), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		f.Write(append(roh, '\n'))
		f.Close()
	}
}

func leadLetzteRunden(p string, n int) []leadRundeEintrag {
	roh, _ := os.ReadFile(leadRundenDatei(p))
	var r []leadRundeEintrag
	for _, z := range strings.Split(strings.TrimSpace(string(roh)), "\n") {
		var e leadRundeEintrag
		if json.Unmarshal([]byte(z), &e) == nil && !e.Zeit.IsZero() {
			r = append(r, e)
		}
	}
	if len(r) > n {
		r = r[len(r)-n:]
	}
	return r
}

// leadSperre: Runde und Gespräch (attach) schließen sich aus; wer zuerst kommt, arbeitet, der andere wartet.
func leadSperre(p string, warten bool) (func(), bool) {
	f, err := os.OpenFile(leadUnter(p, "cycle.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return func() {}, true
	}
	modus := syscall.LOCK_EX
	if !warten {
		modus |= syscall.LOCK_NB
	}
	if syscall.Flock(int(f.Fd()), modus) != nil {
		f.Close()
		return func() {}, false
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, true
}

// rundeAusfuehren: Sperre, Budget, Runde, Meldung an die Plattform, eigenes Protokoll.
func rundeAusfuehren(ctx context.Context, l *leadKonfig, a *leadAnmeldung, anlass string) (string, error) {
	frei, ok := leadSperre(l.dienst(), false)
	if !ok {
		return "skipped", nil // jemand spricht gerade mit dem Lead (attach)
	}
	defer frei()
	budget := int64(0)
	if l.Tagesbudget > 0 {
		rest := l.Tagesbudget - verbrauchAmTag(leadVerbrauchDatei(l.dienst()), time.Now())
		if rest <= 0 {
			return "daily budget used up", nil
		}
		budget = rest
	}
	start := time.Now()
	fmt.Printf("Cycle: %s\n", anlass)
	tokens, ergebnis, err := leadRunde(ctx, l, a, anlass, budget)
	if err != nil && ergebnis == "" {
		ergebnis = "error: " + err.Error()
	}
	sekunden := int(time.Since(start).Seconds())
	verbrauchMerken(leadVerbrauchDatei(l.dienst()), 0, tokens, time.Now())
	leadRundeMerken(l.dienst(), leadRundeEintrag{Zeit: start, Sekunden: sekunden, Tokens: tokens, Anlass: anlass, Ergebnis: ergebnis})
	fmt.Printf("Cycle done after %s: %s, %d tokens\n", (time.Duration(sekunden) * time.Second).String(), ergebnis, tokens)
	// Mit frischem Token melden: Die Runde kann länger gedauert haben, als das alte gilt
	if m, _, e := leadMcp(a); e == nil {
		if e := m.werkzeug("report_lead_cycle", map[string]any{"project": l.Projekt, "seconds": sekunden, "tokens": tokens,
			"reason": abschneiden(anlass, 200), "outcome": abschneiden(ergebnis, 200)}, nil); e != nil {
			fmt.Println("Could not report the cycle:", e)
		}
	}
	return ergebnis, err
}

func abschneiden(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// ---- Dauerbetrieb (der Dienst) ----

func leadBetrieb(p string) {
	l, err := leadKonfigLaden(p)
	pruefen(err)
	z, err := zugangLadenAus(leadZugangDatei(p))
	if err != nil {
		fehlerAus("the lead service has no sign-in: swarvex project lead setup " + p)
	}
	an := &leadAnmeldung{z: z}
	ctx, stopp := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopp()
	programmBeimStart, _ := os.Stat(os.Args[0])
	ruhe := l.dauer(l.Ruhe, leadRuheStandard)
	fmt.Printf("Lead service %s: %s with %s, waits for work on %s%s\n", p, l.Agent, l.Modell, l.Plattform, leadBereichZeile(l))
	var m *mcpClient
	var tokenZeit time.Time
	var letzteRunde time.Time
	letzteSignatur := ""
	if r := leadLetzteRunden(p, 1); len(r) == 1 {
		letzteRunde = r[0].Zeit.Add(time.Duration(r[0].Sekunden) * time.Second)
	}
	fehlerPause := time.Minute
	bereichGenommen, bereitschaft := false, false
	for ctx.Err() == nil {
		// Nach `swarvex update` zwischen zwei Runden mit der neuen Version weiter (gleiche PID, der Dienst merkt nichts)
		if neueVersionInstalliert(os.Args[0], programmBeimStart) {
			fmt.Println("New runner version installed; continuing with it.")
			if err := syscall.Exec(os.Args[0], os.Args, os.Environ()); err != nil {
				programmBeimStart, _ = os.Stat(os.Args[0])
			}
		}
		if m == nil || time.Since(tokenZeit) > 30*time.Minute {
			neu, _, err := leadMcp(an)
			if err != nil {
				if errors.Is(err, errAbgemeldet) {
					fehlerAus("the lead service was signed out (My account) or its sign-in expired: swarvex project lead setup " + p)
				}
				fmt.Println("Platform not reachable:", err, "- trying again in", fehlerPause)
				leadSchlafen(ctx, fehlerPause)
				fehlerPause = min(fehlerPause*2, 15*time.Minute)
				continue
			}
			m, tokenZeit, fehlerPause = neu, time.Now(), time.Minute
		}
		if l.Bereich != "" && !l.BereichsLead && !bereichGenommen {
			// Ein zweiter Lead-Dienst des Leads übernimmt seinen Bereich (wieder), etwa nach einem Neustart
			if err := m.werkzeug("take_area", map[string]any{"project": l.Projekt, "area": l.Bereich, "paths": l.Pfade, "capabilities": l.Faehigkeiten}, nil); err != nil {
				fmt.Println("take_area:", err)
			} else {
				bereichGenommen = true
			}
		}
		a, err := leadWarten(m, l.Projekt, 300)
		if a.Bereitschaft != bereitschaft {
			bereitschaft = a.Bereitschaft
			if bereitschaft {
				fmt.Printf("Standby: the lead service %q holds the main loop of %s. This one takes over when that one is silent for 15 minutes.\n", a.Halter, l.Projekt)
			} else {
				fmt.Println("This lead service now holds the main loop.")
			}
		}
		if err != nil {
			fmt.Println("wait_for_work:", err)
			m = nil
			leadSchlafen(ctx, fehlerPause)
			continue
		}
		starten, anlass := leadEntscheiden(a, letzteSignatur, letzteRunde, ruhe, time.Now())
		if !starten {
			if a.arbeit() || a.Nachschub {
				leadSchlafen(ctx, time.Minute) // wait_for_work kehrte sofort zurück: nicht im Kreis fragen
			}
			continue
		}
		ergebnis, err := rundeAusfuehren(ctx, l, an, anlass)
		if ergebnis == "skipped" {
			leadSchlafen(ctx, time.Minute)
			continue
		}
		letzteRunde, letzteSignatur = time.Now(), a.signatur()
		if ergebnis == "daily budget used up" {
			n := mitternacht(time.Now())
			fmt.Printf("Daily budget used up. Next cycle after %s (the service stays reachable).\n", zeitpunkt(n))
			for time.Now().Before(n) && ctx.Err() == nil {
				if _, err := leadWarten(m, l.Projekt, 300); err != nil {
					m = nil
					break
				}
			}
		}
		if err != nil && ctx.Err() == nil {
			fmt.Println("Cycle failed:", err)
			leadSchlafen(ctx, 10*time.Minute)
		}
	}
	fmt.Println("Lead service stopped.")
}

func leadSchlafen(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// leadEineRunde: eine Runde jetzt, im Vordergrund (zum Ausprobieren und für Tests).
func leadEineRunde(p string, args []string) {
	fs := flag.NewFlagSet("project lead cycle", flag.ExitOnError)
	anlass := fs.String("reason", "started by hand: read the situation, review what waits, keep plan and status current", "why this cycle runs (the lead reads it)")
	fs.Parse(args)
	l, err := leadKonfigLaden(p)
	pruefen(err)
	z, err := zugangLadenAus(leadZugangDatei(p))
	pruefen(err)
	ctx, stopp := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopp()
	ergebnis, err := rundeAusfuehren(ctx, l, &leadAnmeldung{z: z}, *anlass)
	if ergebnis == "skipped" {
		fehlerAus("someone is talking to the lead right now (swarvex project lead attach); try again afterwards")
	}
	pruefen(err)
	fmt.Println("Output of the cycle:", leadUnter(p, "log"))
}

// leadAnschliessen: mit dem Lead sprechen, im selben Container wie seine Runden (Terminal, Rückfragen möglich).
func leadAnschliessen(p string) {
	l, err := leadKonfigLaden(p)
	pruefen(err)
	z, err := zugangLadenAus(leadZugangDatei(p))
	pruefen(err)
	frei, ok := leadSperre(p, false)
	if !ok {
		fmt.Println("A cycle is running; you can talk to the lead as soon as it ends …")
		frei, _ = leadSperre(p, true)
	}
	defer frei()
	ctx := context.Background()
	an := &leadAnmeldung{z: z}
	pruefen(leadTokensSchreiben(l, an))
	_, image, err := leadVorbereiten(ctx, l)
	pruefen(err)
	paket, err := leadPaketHolen(l)
	pruefen(err)
	// Im Gespräch kein eigenes Warten auf Arbeit: das macht der Runner zwischen den Gesprächen (die Runden schreiben
	// LEAD.md wieder ohne diesen Abschnitt)
	pruefen(os.WriteFile(leadUnter(p, filepath.Join("run", "LEAD.md")), []byte(paket+leadGespraech), 0o644))
	netz, err := internesNetz(ctx)
	pruefen(err)
	agent := agenten[l.Agent]
	zugang, err := zugangLesen(agent, l.KeyDatei, l.ModellURL, 8*time.Hour)
	pruefen(err)
	proxy := "swarvex-leadproxy-" + p
	tok, hosts, err := leadProxyBetrieb(ctx, l, proxy, netz, zugang, 0)
	defer exec.Command("podman", "rm", "-f", proxy).Run()
	pruefen(err)
	agentArgs, befehl, err := leadAgent(l, agent, proxy, tok, zugang, true)
	pruefen(err)
	aufhoeren := leadTokensFrischHalten(l, an) // solange das Gespräch läuft
	name := "swarvex-lead-" + p
	args := append(leadContainerArgs(l, name, netz, hosts, 12*3600), leadGitUmgebung(l, proxy)...)
	args = append(append(append([]string{args[0], "-it"}, args[1:]...), agentArgs...), image, "sh", "-c", leadBildschirm+befehl)
	c := exec.Command("podman", args...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Run()
	aufhoeren()
}

// leadBefehlAusfuehren: ein Befehl in der Lead-Sandbox statt einer Runde, mit Netz und Werkzeugen wie dort (prüfen,
// ob die Umgebung stimmt: swarvex project lead exec <projekt> -- godot --version). Zählt nicht als Runde.
func leadBefehlAusfuehren(p string, args []string) {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fehlerAus("which command? swarvex project lead exec " + p + " -- <command>")
	}
	l, err := leadKonfigLaden(p)
	pruefen(err)
	z, err := zugangLadenAus(leadZugangDatei(p))
	pruefen(err)
	frei, ok := leadSperre(p, false)
	if !ok {
		fehlerAus("a cycle or a conversation is running; try again afterwards")
	}
	defer frei()
	ctx, stopp := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopp()
	befehl := strings.Join(args, " ")
	if len(args) > 1 {
		teile := make([]string, len(args))
		for i, a := range args {
			teile[i] = shellText(a)
		}
		befehl = strings.Join(teile, " ")
	}
	_, ergebnis, aus, err := leadImContainer(ctx, l, &leadAnmeldung{z: z}, "command by hand", 1, befehl)
	fmt.Print(aus)
	pruefen(err)
	if ergebnis != "ok" {
		os.Exit(1)
	}
}

// ---- Status, Übersicht, Entfernen ----

func leadStatus(p string) {
	l, err := leadKonfigLaden(p)
	pruefen(err)
	fmt.Printf("Lead service %s%s\n", p, leadBereichZeile(l))
	fmt.Printf("Agent: %s with %s · repository %s\n", l.Agent, l.Modell, l.RepoGit)
	zustand := "not set up as a service"
	if _, err := os.Stat(leadDienstDatei(p)); err == nil {
		zustand, _ = systemctl("is-active", leadDienstName(p))
		if h := dienstHinweis(zustand); h != "" {
			zustand += " (" + strings.Replace(h, "swarvex start", "swarvex project lead start "+p, 1) + ")"
		}
	}
	fmt.Println("Service:", zustand)
	budget := "no daily limit"
	if l.Tagesbudget > 0 {
		budget = fmt.Sprintf("daily limit %d", l.Tagesbudget)
	}
	fmt.Printf("Usage today: %d tokens (%s)\n", verbrauchAmTag(leadVerbrauchDatei(p), time.Now()), budget)
	runden := leadLetzteRunden(p, 5)
	if len(runden) == 0 {
		fmt.Println("No cycle yet.")
		return
	}
	fmt.Println("Latest cycles:")
	for _, r := range runden {
		fmt.Printf("  %s  %4ds  %8d tokens  %s — %s\n", r.Zeit.Format("2006-01-02 15:04"), r.Sekunden, r.Tokens, r.Ergebnis, abschneiden(r.Anlass, 70))
	}
}

func leadProjekte() []string {
	eintraege, _ := os.ReadDir(filepath.Join(datenOrdner(), "lead"))
	var p []string
	for _, e := range eintraege {
		if _, err := os.Stat(filepath.Join(datenOrdner(), "lead", e.Name(), "config.json")); err == nil && e.IsDir() {
			p = append(p, e.Name())
		}
	}
	return p
}

func leadUebersicht() {
	projekte := leadProjekte()
	if len(projekte) == 0 {
		fmt.Println("No lead service on this machine. Set one up for a project you lead: swarvex project lead setup <project>")
		return
	}
	for _, p := range projekte {
		zustand := "not a service"
		if _, err := os.Stat(leadDienstDatei(p)); err == nil {
			zustand, _ = systemctl("is-active", leadDienstName(p))
		}
		letzte := "no cycle yet"
		if r := leadLetzteRunden(p, 1); len(r) == 1 {
			letzte = "last cycle " + r[0].Zeit.Format("2006-01-02 15:04") + ": " + r[0].Ergebnis
		}
		fmt.Printf("%-28s %-10s %s\n", p, zustand, letzte)
	}
}

// leadDienstEntfernen: Dienst anhalten und entfernen, Anmeldung widerrufen (auch für swarvex uninstall).
func leadDienstEntfernen(p string) []string {
	var schritte []string
	if _, err := os.Stat(leadDienstDatei(p)); err == nil {
		systemctl("disable", "--now", leadDienstName(p))
		os.Remove(leadDienstDatei(p))
		systemctl("daemon-reload")
		schritte = append(schritte, "service stopped and removed")
	}
	if z, err := zugangLadenAus(leadZugangDatei(p)); err == nil {
		if r, err := httpClient(z.CAPfad).PostForm(z.Plattform+"/token/revocation", url.Values{
			"token": {z.RefreshToken}, "token_type_hint": {"refresh_token"}, "client_id": {z.ClientID}}); err == nil {
			r.Body.Close()
			schritte = append(schritte, "sign-in revoked")
		}
		os.Remove(leadZugangDatei(p))
	}
	exec.Command("podman", "rm", "-f", "swarvex-lead-"+p, "swarvex-leadproxy-"+p, "swarvex-leadgit-"+p).Run()
	return schritte
}

func leadEntfernen(p string) {
	if _, err := os.Stat(leadOrdner(p)); err != nil {
		fehlerAus("no lead service for " + p + " on this machine")
	}
	for _, s := range leadDienstEntfernen(p) {
		fmt.Println("  ✓", s)
	}
	// Der Klon kann ungepushte Arbeit des Leads enthalten: nicht stillschweigend löschen
	fmt.Printf("  ✓ the clone and the lead's notes stay in %s (delete the folder when you no longer need them)\n", leadOrdner(p))
	os.Remove(leadUnter(p, "config.json"))
}

// leadAufAppWarten: Das Repo liegt auf GitHub, die swarvex-App ist dort aber noch nicht installiert. Statt abzubrechen:
// den Link zeigen (und öffnen) und warten, bis die Plattform das Repo über die App verbinden kann.
func leadAufAppWarten(g leadGit, holen func() (leadGit, error), abstand, hoechstens time.Duration) leadGit {
	fmt.Println("== Connect the repository: install the swarvex app on it (choose this repository)")
	fmt.Println("\n    " + g.Installieren + "\n")
	browserOeffnen(g.Installieren)
	fmt.Print("   waiting for the installation (Ctrl+C to stop) …")
	bis := time.Now().Add(hoechstens)
	for time.Now().Before(bis) {
		time.Sleep(abstand)
		neu, err := holen()
		if err != nil {
			continue // Netz weg: weiter warten
		}
		if neu.Token != nil {
			fmt.Println(" connected")
			return neu
		}
		if neu.Fehlt != "app_not_installed" {
			fmt.Println()
			return neu // anderer Grund (privat, keine Schreibrechte): der Hinweis sagt ihn
		}
		fmt.Print(".")
	}
	fmt.Println()
	return g
}
