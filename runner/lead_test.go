package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Git aus dem Lead-Container: nur der Git-Host des Projekts (auch im eigenen Netz), Endungen nur öffentlich, sonst nichts.
func TestAusgangNurGitHosts(t *testing.T) {
	r, err := ausgangRegelnLesen("only", "gitea.lan, *.githubusercontent.com")
	if err != nil {
		t.Fatal(err)
	}
	pruefe := func(host, ip string, erwartet bool) {
		t.Helper()
		if got := r.erlaubt(host, net.ParseIP(ip)); got != erwartet {
			t.Errorf("%s %s: got %v, want %v", host, ip, got, erwartet)
		}
	}
	pruefe("gitea.lan", "192.168.10.100", true) // genau genannt: auch privat
	pruefe("gitea.lan", "127.0.0.1", false)     // nie der Proxy selbst
	pruefe("media.githubusercontent.com", "185.199.1.1", true)
	pruefe("media.githubusercontent.com", "10.0.0.5", false) // Endung: nur öffentlich
	pruefe("registry.npmjs.org", "104.16.0.1", false)        // alles andere: zu
	pruefe("githubusercontent.com.evil.example", "1.2.3.4", false)
	if _, err := ausgangRegelnLesen("only", "10.0.0.0/8"); err == nil {
		t.Fatal("only takes host names, no address ranges")
	}
}

func TestGitHosts(t *testing.T) {
	if h, _ := gitHosts("https://github.com/swarvex-ai/rend-and-run.git"); !strings.Contains(h, "*.githubusercontent.com") || !strings.HasPrefix(h, "github.com,") {
		t.Errorf("GitHub needs its LFS hosts too: %s", h)
	}
	if h, _ := gitHosts("https://192.168.10.100:3051/neVs/spiel.git"); h != "192.168.10.100" {
		t.Errorf("other hosts: just the host, got %s", h)
	}
	if h, err := gitHosts("http://192.168.10.100:3051/claudia/spiel.git"); err != nil || h != "192.168.10.100" {
		t.Errorf("http inside your own network: %s %v", h, err)
	}
	if _, err := gitHosts("http://example.org/x.git"); err == nil {
		t.Error("plain http to a public host would send the token unencrypted")
	}
	if _, err := gitHosts("git@github.com:x/y.git"); err == nil {
		t.Error("ssh addresses are not supported (the token goes over https)")
	}
}

func TestLeadPaket(t *testing.T) {
	for _, f := range []struct{ art, verfahren, paket string }{
		{"game", "standard", "game"}, {"tool", "clean_room", "tool-cleanroom"}, {"library", "continuation", "library-continuation"},
		{"research", "clean_room", "research"}, {"", "", "other"},
	} {
		if p := leadPaket(leadProjektInfo{Art: f.art, Verfahren: f.verfahren}); p != f.paket {
			t.Errorf("%s/%s: %s, want %s", f.art, f.verfahren, p, f.paket)
		}
	}
}

// Wann läuft eine Runde? Neue Arbeit gleich, dieselbe nach einer Stunde wieder, sonst nach der Ruhezeit.
func TestLeadEntscheiden(t *testing.T) {
	jetzt := time.Date(2026, 11, 6, 22, 0, 0, 0, time.UTC)
	ruhe := 6 * time.Hour
	zwei := leadAnlass{Reviews: []int64{12, 11}}
	if ok, grund := leadEntscheiden(leadAnlass{}, "", time.Time{}, ruhe, jetzt); !ok || !strings.Contains(grund, "started") {
		t.Errorf("first start: a cycle (%v %q)", ok, grund)
	}
	if ok, _ := leadEntscheiden(zwei, "", jetzt.Add(-30*time.Second), ruhe, jetzt); ok {
		t.Error("never two cycles within two minutes")
	}
	if ok, grund := leadEntscheiden(zwei, "[5] 0", jetzt.Add(-10*time.Minute), ruhe, jetzt); !ok || !strings.Contains(grund, "2 checked submission") {
		t.Errorf("new submissions: a cycle at once (%v %q)", ok, grund)
	}
	if ok, _ := leadEntscheiden(zwei, zwei.signatur(), jetzt.Add(-10*time.Minute), ruhe, jetzt); ok {
		t.Error("the same work right after a cycle: wait")
	}
	if ok, grund := leadEntscheiden(zwei, zwei.signatur(), jetzt.Add(-61*time.Minute), ruhe, jetzt); !ok || !strings.Contains(grund, "still open") {
		t.Errorf("the same work after an hour: again (%v %q)", ok, grund)
	}
	nachschub := leadAnlass{Nachschub: true, Frei: 1, Aktiv: 4}
	if ok, _ := leadEntscheiden(nachschub, "[] 0", jetzt.Add(-20*time.Minute), ruhe, jetzt); ok {
		t.Error("low backlog at most once an hour")
	}
	if ok, grund := leadEntscheiden(nachschub, "[] 0", jetzt.Add(-90*time.Minute), ruhe, jetzt); !ok || !strings.Contains(grund, "cut more work") {
		t.Errorf("low backlog after an hour (%v %q)", ok, grund)
	}
	if ok, grund := leadEntscheiden(leadAnlass{}, "[] 0", jetzt.Add(-7*time.Hour), ruhe, jetzt); !ok || !strings.Contains(grund, "routine") {
		t.Errorf("quiet for longer than the idle time: routine cycle (%v %q)", ok, grund)
	}
	if ok, _ := leadEntscheiden(leadAnlass{}, "[] 0", jetzt.Add(-2*time.Hour), ruhe, jetzt); ok {
		t.Error("nothing to do: no cycle")
	}
}

// Die Plattform sieht das Token des Lead-Dienstes, der Container nur sein Wegwerf-Token; nur /mcp und Spenden-Dateien.
func TestPlattformWeiter(t *testing.T) {
	var gesehen string
	plattform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gesehen = r.Method + " " + r.URL.Path + " " + r.Header.Get("Authorization")
		io.WriteString(w, `{"ok":true}`)
	}))
	defer plattform.Close()
	datei := filepath.Join(t.TempDir(), "access-token")
	os.WriteFile(datei, []byte("echtes-token\n"), 0o644)
	ziel, _ := url.Parse(plattform.URL)
	tok := strings.Repeat("w", 36)
	h := plattformWeiter(ziel, datei, tok, plattform.Client())
	rufe := func(methode, pfad, auth string) int {
		req := httptest.NewRequest(methode, pfad, strings.NewReader(`{}`))
		req.Header.Set("Authorization", auth)
		w := httptest.NewRecorder()
		h(w, req)
		return w.Code
	}
	if c := rufe("POST", "/mcp", "Bearer falsch"); c != http.StatusUnauthorized {
		t.Errorf("wrong token: %d", c)
	}
	if c := rufe("GET", "/api/account", "Bearer "+tok); c != http.StatusForbidden {
		t.Errorf("other platform paths are closed: %d", c)
	}
	if c := rufe("POST", "/mcp", "Bearer "+tok); c != http.StatusOK || gesehen != "POST /mcp Bearer echtes-token" {
		t.Errorf("forwarded with the real token: %d %q", c, gesehen)
	}
	os.WriteFile(datei, []byte("neues-token"), 0o644) // der Runner hat erneuert
	if c := rufe("GET", "/donations/7/file", "Bearer "+tok); c != http.StatusOK || gesehen != "GET /donations/7/file Bearer neues-token" {
		t.Errorf("donation file with the renewed token: %d %q", c, gesehen)
	}
}

func TestLeadGitUmgebung(t *testing.T) {
	l := &leadKonfig{AutorName: "nevs84", AutorEmail: "1+nevs84@users.noreply.github.com"}
	args := strings.Join(leadGitUmgebung(l, "swarvex-leadproxy-x"), " ")
	for _, soll := range []string{"http.proxy", "http://swarvex-leadproxy-x:3128", "username=x-access-token", "/run/swarvex/git-token", "user.name", "nevs84"} {
		if !strings.Contains(args, soll) {
			t.Errorf("missing %q in %s", soll, args)
		}
	}
	l.GitTokenDatei, l.GitNutzer = "/x", "neVs"
	if !strings.Contains(strings.Join(leadGitUmgebung(l, "p"), " "), "username=neVs") {
		t.Error("own token file: own user name")
	}
}

// Der Agent bekommt das Lead-Paket als Anleitung und die swarvex-Werkzeuge über den Proxy, nie einen echten Schlüssel.
func TestLeadAgent(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	l := &leadKonfig{Projekt: "spiel", Modell: "claude-opus-5-5"}
	for _, teil := range []string{"run", "home"} {
		os.MkdirAll(leadUnter("spiel", teil), 0o755)
	}
	tok := strings.Repeat("t", 36)
	args, befehl, err := leadAgent(l, agenten["claude"], "swarvex-leadproxy-spiel", tok, modellZugang{Key: "sk-ant-oat-geheim"}, false)
	if err != nil {
		t.Fatal(err)
	}
	alles := strings.Join(args, " ") + " " + befehl
	for _, soll := range []string{"--append-system-prompt", "/run/swarvex/LEAD.md", "--mcp-config /run/swarvex/mcp.json", "--strict-mcp-config", " -p ", "ANTHROPIC_BASE_URL=http://swarvex-leadproxy-spiel:8080"} {
		if !strings.Contains(alles, soll) {
			t.Errorf("claude: missing %q", soll)
		}
	}
	if strings.Contains(alles, "geheim") {
		t.Fatal("the real key must stay in the proxy")
	}
	var mcp struct {
		Server map[string]struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	roh, _ := os.ReadFile(leadUnter("spiel", "run/mcp.json"))
	if json.Unmarshal(roh, &mcp) != nil || mcp.Server["swarvex"].URL != "http://swarvex-leadproxy-spiel:8080/mcp" || mcp.Server["swarvex"].Headers["Authorization"] != "Bearer "+tok {
		t.Errorf("claude MCP config: %s", roh)
	}
	_, gespraech, _ := leadAgent(l, agenten["claude"], "p", tok, modellZugang{}, true)
	if strings.Contains(gespraech, " -p ") || !strings.Contains(gespraech, "--continue") {
		t.Errorf("attach: interactive, continuing the last conversation: %s", gespraech)
	}
	_, codex, _ := leadAgent(l, agenten["codex"], "p", tok, modellZugang{}, false)
	for _, soll := range []string{"~/.codex/AGENTS.md", "mcp_servers.swarvex.url", "bearer_token_env_var", "codex exec"} {
		if !strings.Contains(codex, soll) {
			t.Errorf("codex: missing %q in %s", soll, codex)
		}
	}
	if _, _, err := leadAgent(l, agenten["opencode"], "p", tok, modellZugang{}, false); err != nil {
		t.Fatal(err)
	}
	roh, _ = os.ReadFile(leadUnter("spiel", "run/opencode.json"))
	if !strings.Contains(string(roh), `"/run/swarvex/LEAD.md"`) || !strings.Contains(string(roh), `"http://p:8080/mcp"`) {
		t.Errorf("opencode config: %s", roh)
	}
}

// Ein Lead-Dienst hat seine eigene Anmeldung und Sperre neben der des Runners.
func TestLeadAnmeldungEigeneDatei(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	haupt := &zugangsdaten{Plattform: "https://p", ClientID: "runner", RefreshToken: "r1"}
	lead := &zugangsdaten{Plattform: "https://p", ClientID: "lead", RefreshToken: "l1", datei: leadZugangDatei("spiel")}
	if err := zugangSpeichern(haupt); err != nil {
		t.Fatal(err)
	}
	if err := zugangSpeichern(lead); err != nil {
		t.Fatal(err)
	}
	h, _ := zugangLaden()
	l, err := zugangLadenAus(leadZugangDatei("spiel"))
	if err != nil || h.ClientID != "runner" || l.ClientID != "lead" || l.pfad() != leadZugangDatei("spiel") {
		t.Fatalf("separate files: %+v %+v %v", h, l, err)
	}
	frei := zugangSperren(l) // die Sperre des Leads hält den Runner nicht auf
	defer frei()
	fertig := make(chan struct{})
	go func() { zugangSperren(h)(); close(fertig) }()
	select {
	case <-fertig:
	case <-time.After(2 * time.Second):
		t.Fatal("the runner's lock waited for the lead's")
	}
}

// Mehrere Lead-Dienste je Projekt (E-39): Bereitschaft startet keine Runde, Dienste mit Bereich heißen <projekt>.<bereich>
func TestLeadKraft(t *testing.T) {
	jetzt := time.Date(2026, 11, 7, 2, 0, 0, 0, time.UTC)
	bereit := leadAnlass{Reviews: []int64{3}, Bereitschaft: true, Halter: "swarvex project lead: kraft (box)"}
	for _, letzte := range []time.Time{{}, jetzt.Add(-7 * time.Hour)} {
		if ok, _ := leadEntscheiden(bereit, "", letzte, 6*time.Hour, jetzt); ok {
			t.Errorf("standby: never a cycle (last %v)", letzte)
		}
	}
	for name, ok := range map[string]bool{"rend-and-run": true, "rend-and-run.combat": true, "rend-and-run.Combat": false, "a.b.c": false, "rend-and-run.": false} {
		if leadName.MatchString(name) != ok {
			t.Errorf("leadName(%q) = %v, want %v", name, !ok, ok)
		}
	}
	l := &leadKonfig{Projekt: "kraft"}
	if l.dienst() != "kraft" || leadBereichText(l) != "" {
		t.Error("main service: named like the project, no area text")
	}
	l = &leadKonfig{Projekt: "kraft", Name: "kraft.combat", Bereich: "combat", Pfade: []string{"game/combat/**"}}
	if l.dienst() != "kraft.combat" || !strings.Contains(leadBereichText(l), "only that area") {
		t.Errorf("area service: %q %q", l.dienst(), leadBereichText(l))
	}
	l.BereichsLead = true
	if !strings.Contains(leadBereichText(l), "never push") {
		t.Errorf("area lead's service: read-only, %q", leadBereichText(l))
	}
}
