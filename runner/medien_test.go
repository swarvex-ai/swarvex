package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Konfigurationen der Agenten in einem eigenen HOME
func agentenKonfigurationen(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("MESHY_KEY_AUS_DER_SHELL", "")
	schreiben := func(pfad, inhalt string) {
		voll := filepath.Join(home, pfad)
		os.MkdirAll(filepath.Dir(voll), 0o755)
		if err := os.WriteFile(voll, []byte(inhalt), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	schreiben(".claude.json", `{"mcpServers": {
		"meshy": {"type": "stdio", "command": "npx", "args": ["-y", "@meshy-ai/meshy-mcp-server"], "env": {"MESHY_API_KEY": "msy-claude"}},
		"github": {"type": "http", "url": "https://api.githubcopilot.com/mcp/"}},
	  "projects": {"/home/x/spiel": {"mcpServers": {"suno-lokal": {"command": "uvx", "args": ["suno-mcp"], "env": {"SUNO_TOKEN": "${MESHY_KEY_AUS_DER_SHELL}"}}}}}}`)
	schreiben(".hermes/config.yaml", `model:
  default: x
mcp_servers:
  # ElevenLabs als lokaler Server
  elevenlabs:
    command: "uvx"
    args:
      - "elevenlabs-mcp"
    env:
      ELEVENLABS_API_KEY: ${ELEVEN_KEY}
  meshy:
    url: "https://mcp.meshy.ai/mcp"
    headers:
      Authorization: "Bearer msy-hermes"
  aus:
    command: npx
    args: ["-y", "runway-mcp"]
    enabled: false
`)
	schreiben(".hermes/.env", "ELEVEN_KEY=el-aus-env\n")
	schreiben(".codex/config.toml", `model = "gpt-5"
[mcp_servers.meshy]
command = "npx"
args = [
  "-y",   # Paket
  "@meshy-ai/meshy-mcp-server",
]
[mcp_servers.meshy.env]
MESHY_API_KEY = "msy-codex"

[mcp_servers."leonardo.ai"]
url = "https://leonardo.example/mcp"
bearer_token_env_var = "LEO_TOKEN"
`)
	schreiben(".config/opencode/opencode.jsonc", `{
  // Kommentar
  "mcp": {
    "kling": {"type": "remote", "url": "https://kling.example/mcp", "headers": {"X-Key": "{env:KLING_KEY}"},},
    "dateien": {"type": "local", "command": ["npx", "-y", "@modelcontextprotocol/server-filesystem"]}
  }
}`)
	schreiben(".gemini/settings.json", `{"mcpServers": {"higgsfield": {"url": "https://higgsfield.example/sse"}}}`)
	t.Setenv("KLING_KEY", "kl-1")
}

func TestMedienServerFinden(t *testing.T) {
	agentenKonfigurationen(t)
	alle := medienServerFinden()
	nach := map[string]medienServer{}
	for _, s := range alle {
		nach[s.Kennung()] = s
	}
	if len(alle) != 8 {
		t.Fatalf("found %d servers: %+v", len(alle), alle)
	}
	m := nach["claude:meshy"]
	if m.Dienst != "meshy" || strings.Join(m.Befehl, " ") != "npx -y @meshy-ai/meshy-mcp-server" || m.Env["MESHY_API_KEY"] != "msy-claude" || m.Fehlt != "" {
		t.Errorf("claude:meshy = %+v", m)
	}
	if s := nach["claude:suno-lokal"]; s.Dienst != "suno" || !strings.Contains(s.Fehlt, "MESHY_KEY_AUS_DER_SHELL") || !strings.Contains(s.Fehlt, "media.env") {
		t.Errorf("an unresolvable variable is named: %+v", s)
	}
	if s := nach["hermes:elevenlabs"]; s.Dienst != "elevenlabs" || strings.Join(s.Befehl, " ") != "uvx elevenlabs-mcp" || s.Env["ELEVENLABS_API_KEY"] != "el-aus-env" || s.Fehlt != "" {
		t.Errorf("hermes:elevenlabs (key from ~/.hermes/.env) = %+v", s)
	}
	if s := nach["hermes:meshy"]; s.URL != "https://mcp.meshy.ai/mcp" || s.Kopf["Authorization"] != "Bearer msy-hermes" || len(s.Befehl) != 0 {
		t.Errorf("hermes:meshy (remote) = %+v", s)
	}
	if _, ok := nach["hermes:aus"]; ok {
		t.Error("a disabled server is not offered")
	}
	if s := nach["codex:meshy"]; strings.Join(s.Befehl, " ") != "npx -y @meshy-ai/meshy-mcp-server" || s.Env["MESHY_API_KEY"] != "msy-codex" {
		t.Errorf("codex:meshy (multi-line list, env table) = %+v", s)
	}
	if s := nach["codex:leonardo.ai"]; s.Dienst != "leonardo" || !strings.Contains(s.Fehlt, "LEO_TOKEN") {
		t.Errorf("codex:leonardo.ai (quoted name, bearer from env) = %+v", s)
	}
	t.Setenv("LEO_TOKEN", "leo-1")
	for _, s := range medienServerFinden() {
		if s.Kennung() == "codex:leonardo.ai" && s.Kopf["Authorization"] != "Bearer leo-1" {
			t.Errorf("bearer_token_env_var becomes a bearer header: %+v", s)
		}
	}
	if s := nach["opencode:kling"]; s.URL != "https://kling.example/mcp" || s.Kopf["X-Key"] != "kl-1" {
		t.Errorf("opencode:kling (JSONC, {env:}) = %+v", s)
	}
	if _, ok := nach["opencode:dateien"]; ok {
		t.Error("servers without a media service are not offered")
	}
	if s := nach["gemini:higgsfield"]; !strings.Contains(s.Fehlt, "SSE") {
		t.Errorf("an SSE-only server is not usable: %+v", s)
	}
	// Auswahl aus --media in dieser Reihenfolge, Unbekanntes fällt weg
	auswahl := medienAuswahl("hermes:elevenlabs, claude:meshy,gibts:nicht", alle)
	if medienSpec(auswahl) != "hermes:elevenlabs,claude:meshy" || len(medienAuswahl("none", alle)) != 0 {
		t.Errorf("selection = %q", medienSpec(auswahl))
	}
}

func TestMedienEinrichtung(t *testing.T) {
	agentenKonfigurationen(t)
	e := empfohlen{moeglich: true, args: []string{"--project", "auto"}}
	medienEmpfehlen(&e, medienServerFinden())
	spec, _ := argWert(e.args, "media")
	if tag, _ := argWert(e.args, "media-daily"); tag != "30" || !strings.Contains(spec, "claude:meshy") || strings.Contains(spec, "suno") {
		t.Errorf("quick start takes the usable tools with 30 a day: %v", e.args)
	}
	if !strings.Contains(e.zeilen[len(e.zeilen)-1], "at most 30 a day") {
		t.Errorf("one line in the recommendation, no extra question: %v", e.zeilen)
	}
	if got := medienText([]string{"--media", "claude:meshy", "--media-daily", "3"}); got != "at most 3 briefs a day with Meshy (Claude Code)" {
		t.Errorf("medienText = %q", got)
	}
	if medienText([]string{"--media", "none"}) != "off" || medienText([]string{"--media", "claude:meshy", "--media-daily", "0"}) != "off" {
		t.Error("none or 0 is off")
	}
}

func TestIstAbfrage(t *testing.T) {
	for name, abfrage := range map[string]bool{"meshy_get_task_status": true, "meshy_download_model": true, "getVoices": true, "meshy_check_balance": true,
		"meshy_text_to_3d": false, "text_to_speech": false, "retarget_animation": false, "meshy_image_to_3d": false, "generate": false} {
		if istAbfrage(name) != abfrage {
			t.Errorf("istAbfrage(%q) = %v", name, !abfrage)
		}
	}
}

func TestBrueckeZaehltUndGrenztEin(t *testing.T) {
	var gesehen []string
	b := &bruecke{token: strings.Repeat("t", 32), maxErzeug: 2, maxAlle: 5, aufrufe: func(_ context.Context, n []byte, erwartet bool) ([]byte, error) {
		var r rpc
		json.Unmarshal(n, &r)
		gesehen = append(gesehen, r.Method)
		return []byte(`{"jsonrpc":"2.0","id":` + string(r.ID) + `,"result":{"ok":true}}`), nil
	}}
	srv := httptest.NewServer(http.HandlerFunc(b.behandeln))
	defer srv.Close()
	senden := func(tok, koerper string) (int, string) {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(koerper))
		req.Header.Set("Authorization", "Bearer "+tok)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		roh, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(roh)
	}
	if code, _ := senden("falsch", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`); code != 401 {
		t.Fatalf("wrong token: %d", code)
	}
	tok := strings.Repeat("t", 32)
	if code, aus := senden(tok, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`); code != 200 || !strings.Contains(aus, `"ok":true`) {
		t.Fatalf("initialize: %d %s", code, aus)
	}
	if code, _ := senden(tok, `{"jsonrpc":"2.0","method":"notifications/initialized"}`); code != 202 {
		t.Fatalf("notification: %d", code)
	}
	aufruf := func(id int, name string) string {
		_, aus := senden(tok, `{"jsonrpc":"2.0","id":`+string(rune('0'+id))+`,"method":"tools/call","params":{"name":"`+name+`","arguments":{}}}`)
		return aus
	}
	aufruf(2, "meshy_text_to_3d")
	aufruf(3, "meshy_get_task_status")
	aufruf(4, "meshy_text_to_3d_refine")
	if aus := aufruf(5, "meshy_text_to_3d"); !strings.Contains(aus, "at most 2 generating calls") || !strings.Contains(aus, `"isError":true`) {
		t.Errorf("third generating call is refused as a tool error: %s", aus)
	}
	if aus := aufruf(6, "meshy_download_model"); !strings.Contains(aus, `"ok":true`) {
		t.Errorf("downloads still pass: %s", aus)
	}
	aufruf(7, "meshy_list_models")
	if aus := aufruf(8, "meshy_get_task_status"); !strings.Contains(aus, "at most 5 tool calls") {
		t.Errorf("all calls are capped too: %s", aus)
	}
	if strings.Join(gesehen, ",") != "initialize,notifications/initialized,tools/call,tools/call,tools/call,tools/call,tools/call" {
		t.Errorf("forwarded: %v", gesehen)
	}
}

func TestBrueckeMitKindprozess(t *testing.T) {
	// Ein Server, der auf stdio antwortet und vorher den Client etwas fragt (roots/list)
	skript := filepath.Join(t.TempDir(), "server.sh")
	os.WriteFile(skript, []byte(`#!/bin/sh
read zeile
echo 'kein JSON: Startmeldung'
echo '{"jsonrpc":"2.0","id":"r1","method":"roots/list"}'
read antwort
echo "$antwort" >&2
echo '{"jsonrpc":"2.0","method":"notifications/progress","params":{}}'
echo '{"jsonrpc":"2.0","id":7,"result":{"serverInfo":{"name":"test"}}}'
read rest
`), 0o755)
	k, err := kindStarten([]string{"sh", skript})
	if err != nil {
		t.Fatal(err)
	}
	ctx, stopp := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopp()
	aus, err := k.senden(ctx, []byte(`{"jsonrpc":"2.0","id":7,"method":"initialize","params":{}}`), true)
	if err != nil || !strings.Contains(string(aus), `"serverInfo"`) {
		t.Fatalf("answer = %s, %v", aus, err)
	}
}

func TestMedienEinsammeln(t *testing.T) {
	aus, arbeit := t.TempDir(), t.TempDir()
	b := medienBrief{Dateitypen: []string{"glb", "txt"}, MaxMB: 1}
	if d, grund := medienEinsammeln(aus, arbeit, b); d != "" || !strings.Contains(grund, "no .glb file") {
		t.Errorf("empty: %q %q", d, grund)
	}
	os.WriteFile(filepath.Join(aus, "notiz.txt"), []byte("geheim"), 0o644)
	os.WriteFile(filepath.Join(aus, "falsch.glb"), []byte("-----BEGIN OPENSSH PRIVATE KEY-----"), 0o644)
	if d, grund := medienEinsammeln(aus, arbeit, b); d != "" || !strings.Contains(grund, "falsch.glb (content does not match") || !strings.Contains(grund, "notiz.txt (type not asked for)") {
		t.Errorf("only real models count, texts never come through the runner: %q %q", d, grund)
	}
	os.WriteFile(filepath.Join(aus, "a.glb"), append([]byte("glTF"), make([]byte, 50)...), 0o644)
	os.MkdirAll(filepath.Join(aus, "unter"), 0o755)
	os.WriteFile(filepath.Join(aus, "unter", "b.glb"), append([]byte("glTF"), make([]byte, 50)...), 0o644)
	os.WriteFile(filepath.Join(aus, "gross.glb"), append([]byte("glTF"), make([]byte, 2<<20)...), 0o644)
	os.WriteFile(filepath.Join(arbeit, "RESULT"), []byte("a.glb\n"), 0o644)
	if d, _ := medienEinsammeln(aus, arbeit, b); filepath.Base(d) != "a.glb" {
		t.Errorf("RESULT names the file: %q", d)
	}
	os.Remove(filepath.Join(arbeit, "RESULT"))
	jetzt := time.Now()
	os.Chtimes(filepath.Join(aus, "a.glb"), jetzt.Add(-time.Hour), jetzt.Add(-time.Hour))
	if d, _ := medienEinsammeln(aus, arbeit, b); filepath.Base(d) != "b.glb" {
		t.Errorf("otherwise the newest that fits (the 2 MB one is too large): %q", d)
	}
	// RESULT als Link oder FIFO wird übergangen, statt ihm zu folgen oder zu blockieren (Sicherheitsaudit 12.11.2026)
	fremd := filepath.Join(t.TempDir(), "fremd")
	os.WriteFile(fremd, []byte("a.glb\n"), 0o644)
	os.Symlink(fremd, filepath.Join(arbeit, "RESULT"))
	if d, _ := medienEinsammeln(aus, arbeit, b); filepath.Base(d) != "b.glb" {
		t.Errorf("RESULT as a link was followed: %q", d)
	}
	os.Remove(filepath.Join(arbeit, "RESULT"))
	if err := syscall.Mkfifo(filepath.Join(arbeit, "RESULT"), 0o644); err == nil {
		fertig := make(chan string, 1)
		go func() { d, _ := medienEinsammeln(aus, arbeit, b); fertig <- d }()
		select {
		case d := <-fertig:
			if filepath.Base(d) != "b.glb" {
				t.Errorf("RESULT as a FIFO: %q", d)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("RESULT as a FIFO blocks the runner")
		}
	}
}

func TestMedienInhalt(t *testing.T) {
	d := t.TempDir()
	for name, fall := range map[string]struct {
		inhalt string
		ok     bool
	}{
		"bild.png": {"\x89PNG\r\n\x1a\nrest", true}, "bild.jpg": {"\xff\xd8\xff\xe0", true}, "ton.wav": {"RIFF\x00\x00\x00\x00WAVEfmt ", true},
		"ton.ogg": {"OggS\x00", true}, "ton.mp3": {"ID3\x04", true}, "film.mp4": {"\x00\x00\x00\x18ftypmp42", true},
		"kiste.obj": {"# Kiste\nmtllib k.mtl\no K\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n", true},
		"env.obj":   {"OPENAI_API_KEY=sk-geheim\nv 1 2 3\n", false}, "bild.webp": {"RIFF\x00\x00\x00\x00WAVE", false}, "modell.glb": {"glTf", false},
	} {
		p := filepath.Join(d, name)
		os.WriteFile(p, []byte(fall.inhalt), 0o644)
		if medienInhaltPasst(p) != fall.ok {
			t.Errorf("%s: %v", name, !fall.ok)
		}
	}
}

func TestMedienAuftragstextUndAgent(t *testing.T) {
	b := medienBrief{ID: 4, Projekt: "rend-and-run", Dienst: "meshy", Titel: "Drohne", Anleitung: "low poly Drohne", Zielpfad: "asset_library/models/drohne.glb",
		Dateitypen: []string{"glb"}, MaxMB: 50, Zustand: "rework", Begruendung: "weniger Dreiecke"}
	text := medienAuftragstext(b, []medienServer{{Dienst: "meshy"}}, 8)
	for _, teil := range []string{"/out/drohne.glb", "at most 8 generating calls", "/work/RESULT", "weniger Dreiecke", "one file of type .glb, at most 50 MB"} {
		if !strings.Contains(text, teil) {
			t.Errorf("brief lacks %q:\n%s", teil, text)
		}
	}
	ordner := t.TempDir()
	w := []sandboxWerkzeug{{Name: "meshy", URL: "http://swarvex-tool-4-1:8090/mcp", Token: "wegwerf"}}
	konfigSchreiben(ordner, "opencode.json", map[string]any{"model": "swarvex/m"})
	if _, befehl, err := agentWerkzeuge(agenten["opencode"], ordner, w, nil, []string{"opencode", "run", "AUFTRAG"}); err != nil || befehl[len(befehl)-1] != "AUFTRAG" {
		t.Fatal(err)
	}
	if k := lesen(filepath.Join(ordner, "opencode.json")); !strings.Contains(k, `"Authorization": "Bearer wegwerf"`) || !strings.Contains(k, `"type": "remote"`) {
		t.Errorf("opencode gets a remote MCP server: %s", k)
	}
	args, befehl, _ := agentWerkzeuge(agenten["claude"], ordner, w, nil, []string{"claude", "-p", "AUFTRAG"})
	if strings.Join(befehl, " ") != "claude -p --mcp-config /konfig/mcp.json --strict-mcp-config AUFTRAG" || !strings.Contains(strings.Join(args, " "), "MCP_TOOL_TIMEOUT") {
		t.Errorf("claude: %v %v", args, befehl)
	}
	args, befehl, _ = agentWerkzeuge(agenten["codex"], ordner, w, nil, []string{"codex", "exec", "AUFTRAG"})
	if !strings.Contains(strings.Join(befehl, " "), `mcp_servers.meshy.bearer_token_env_var="SWARVEX_TOOL_TOKEN_1"`) || !strings.Contains(strings.Join(args, " "), "SWARVEX_TOOL_TOKEN_1=wegwerf") {
		t.Errorf("codex: %v %v", args, befehl)
	}
}

func TestMedienHochladen(t *testing.T) {
	var name, auth string
	var laenge int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, auth, laenge = r.URL.Query().Get("name"), r.Header.Get("authorization"), r.ContentLength
		if r.Method != http.MethodPut || r.URL.Path != "/donations/9/upload" {
			w.WriteHeader(404)
			return
		}
		if strings.Contains(name, "kaputt") {
			w.WriteHeader(403)
			io.WriteString(w, `{"error":"the file's content is not a .glb file"}`)
			return
		}
		io.WriteString(w, `{"brief_id":9}`)
	}))
	defer srv.Close()
	d := t.TempDir()
	p := filepath.Join(d, "drohne mit leer.glb")
	os.WriteFile(p, []byte("glTF1234"), 0o644)
	z := &zugangsdaten{Plattform: srv.URL}
	if err := medienHochladen(z, "tok", 9, p); err != nil || name != "drohne mit leer.glb" || auth != "Bearer tok" || laenge != 8 {
		t.Fatalf("upload: %v %q %q %d", err, name, auth, laenge)
	}
	k := filepath.Join(d, "kaputt.glb")
	os.WriteFile(k, []byte("glTF"), 0o644)
	if err := medienHochladen(z, "tok", 9, k); err == nil || err.Error() != "the file's content is not a .glb file" {
		t.Errorf("the platform's reason comes back: %v", err)
	}
}

// Der Name eines Werkzeug-Aufrufs gilt so, wie der Server ihn liest, und Werkzeuge jenseits der Medienerzeugung reicht
// die Brücke nicht weiter (Sicherheitsaudit, Nachtrag 13.11.2026).
func TestBrueckeNameUndSperre(t *testing.T) {
	var weiter []string
	b := &bruecke{token: strings.Repeat("t", 32), maxErzeug: 1, maxAlle: 50, aufrufe: func(_ context.Context, n []byte, erwartet bool) ([]byte, error) {
		weiter = append(weiter, string(n))
		var r rpc
		json.Unmarshal(n, &r)
		return []byte(`{"jsonrpc":"2.0","id":` + string(r.ID) + `,"result":{"ok":true}}`), nil
	}}
	srv := httptest.NewServer(http.HandlerFunc(b.behandeln))
	defer srv.Close()
	aufruf := func(params string) string {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":`+params+`}`))
		req.Header.Set("Authorization", "Bearer "+strings.Repeat("t", 32))
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		roh, _ := io.ReadAll(r.Body)
		return string(roh)
	}
	// zweiter Schlüssel, der für Go auch „name“ wäre: abgewiesen, nicht als Abfrage gezählt
	for _, p := range []string{`{"name":"meshy_text_to_3d","NAME":"meshy_get_task_status"}`, `{"Name":"meshy_get_task_status","name":"meshy_text_to_3d"}`,
		`{"name":"meshy_text_to_3d","name":"meshy_get_task_status"}`, `{"arguments":{}}`, `[]`} {
		if aus := aufruf(p); !strings.Contains(aus, `"isError":true`) {
			t.Errorf("%s: %s", p, aus)
		}
	}
	for _, name := range []string{"make_outbound_call", "create_agent", "deleteVoice", "meshy_send_to_slicer", "list_phone_numbers"} {
		if aus := aufruf(`{"name":"` + name + `","arguments":{}}`); !strings.Contains(aus, "does more than create or fetch media") {
			t.Errorf("%s not blocked: %s", name, aus)
		}
	}
	if len(weiter) != 0 {
		t.Fatalf("forwarded to the server: %v", weiter)
	}
	// normale Aufrufe gehen weiter, der erzeugende wird gezählt
	if aus := aufruf(`{"name":"text_to_speech","arguments":{"text":"hallo"}}`); !strings.Contains(aus, `"ok":true`) {
		t.Errorf("text_to_speech: %s", aus)
	}
	if aus := aufruf(`{"name":"text_to_speech","arguments":{}}`); !strings.Contains(aus, "at most 1 generating calls") {
		t.Errorf("second generating call: %s", aus)
	}
	for _, name := range []string{"meshy_get_task_status", "search_voice_library", "remove_background_status"} {
		if istGesperrt(name) {
			t.Errorf("%s blocked", name)
		}
	}
}
