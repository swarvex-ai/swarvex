package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func codexAnmeldungSchreiben(t *testing.T, ablauf time.Time) string {
	t.Helper()
	pfad := filepath.Join(t.TempDir(), "auth.json")
	id := jwtAttrappe(map[string]any{"https://api.openai.com/auth": map[string]any{"chatgpt_plan_type": "pro"}})
	roh, _ := json.Marshal(map[string]any{
		"auth_mode": "chatgpt", "OPENAI_API_KEY": nil, "last_refresh": "2026-10-01T00:00:00Z", "eigenes_feld": 42,
		"tokens": map[string]any{"id_token": id, "access_token": jwtAttrappe(map[string]any{"exp": ablauf.Unix()}),
			"refresh_token": "alter-refresh", "account_id": "konto-123"},
	})
	if err := os.WriteFile(pfad, roh, 0o600); err != nil {
		t.Fatal(err)
	}
	return pfad
}

func TestCodexAboOhneErneuerung(t *testing.T) {
	aufrufe := atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { aufrufe.Add(1) }))
	defer srv.Close()
	codexTokenURL = srv.URL
	pfad := codexAnmeldungSchreiben(t, time.Now().Add(5*24*time.Hour))
	z, err := codexAboLaden(pfad, 2*time.Hour)
	if err != nil || !z.chatgpt() || z.Konto != "konto-123" || z.Plan != "pro" || z.URL != codexAboURL || z.Key == "" {
		t.Fatalf("%+v %v", z, err)
	}
	if aufrufe.Load() != 0 {
		t.Error("gültiges Token wurde erneuert")
	}
}

func TestCodexAboErneuerung(t *testing.T) {
	var anfrage map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&anfrage)
		io.WriteString(w, `{"access_token":"neuer-zugang","refresh_token":"neuer-refresh","id_token":"`+
			jwtAttrappe(map[string]any{"https://api.openai.com/auth": map[string]any{"chatgpt_plan_type": "plus"}})+`"}`)
	}))
	defer srv.Close()
	codexTokenURL = srv.URL
	pfad := codexAnmeldungSchreiben(t, time.Now().Add(30*time.Minute)) // läuft während der Aufgabe ab
	z, err := codexAboLaden(pfad, 2*time.Hour)
	if err != nil || z.Key != "neuer-zugang" || z.Plan != "plus" {
		t.Fatalf("%+v %v", z, err)
	}
	if anfrage["grant_type"] != "refresh_token" || anfrage["refresh_token"] != "alter-refresh" || anfrage["client_id"] != codexClientID {
		t.Errorf("Anfrage: %v", anfrage)
	}
	// Zurückgeschrieben wie von Codex selbst: neue Tokens, eigene Felder bleiben, nur für den Besitzer lesbar.
	roh, _ := os.ReadFile(pfad)
	var d map[string]any
	json.Unmarshal(roh, &d)
	tok := d["tokens"].(map[string]any)
	if tok["refresh_token"] != "neuer-refresh" || tok["account_id"] != "konto-123" || d["eigenes_feld"] != float64(42) || d["last_refresh"] == "2026-10-01T00:00:00Z" {
		t.Errorf("auth.json: %s", roh)
	}
	if info, _ := os.Stat(pfad); info.Mode().Perm() != 0o600 {
		t.Errorf("Rechte %v", info.Mode().Perm())
	}
}

func TestCodexSandboxAnmeldung(t *testing.T) {
	tok := codexSandboxToken(strings.Repeat("z", 36))
	if ablauf, ok := jwtAblauf(tok); !ok || time.Until(ablauf) < 20*time.Hour {
		t.Errorf("Sandbox-Token ohne gültigen Ablauf: %v", ablauf)
	}
	var d struct {
		Modus  string `json:"auth_mode"`
		Tokens struct {
			ID, Zugang, Refresh, Konto string
		} `json:"tokens"`
	}
	roh := codexSandboxAnmeldung(tok, "pro")
	json.Unmarshal(roh, &d)
	var t2 map[string]map[string]string
	json.Unmarshal(roh, &t2)
	if d.Modus != "chatgpt" || t2["tokens"]["access_token"] != tok || t2["tokens"]["account_id"] != codexKontoSandbox {
		t.Errorf("Attrappe: %s", roh)
	}
	if plan := jwtInhalt(t2["tokens"]["id_token"])["https://api.openai.com/auth"].(map[string]any)["chatgpt_plan_type"]; plan != "pro" {
		t.Errorf("Plan %v", plan)
	}
	// Codex-Start im Abo-Modus: Attrappe eingebunden, Anbieter mit ChatGPT-Anmeldung statt env_key.
	ordner := t.TempDir()
	args, befehl, err := agentStart(agenten["codex"], ordner, "http://p:8080", tok, "gpt-x", "A", nil, t.TempDir(), "", modellZugang{Konto: "konto-123", Plan: "pro"})
	alles, cmd := strings.Join(args, " "), strings.Join(befehl, " ")
	if err != nil || !strings.Contains(alles, ":/home/agent/.codex/auth.json:ro") || !strings.Contains(cmd, "requires_openai_auth=true") || strings.Contains(cmd, "env_key") {
		t.Errorf("Codex-Abo-Start: %v | %v | %v", alles, cmd, err)
	}
	if roh, _ := os.ReadFile(filepath.Join(ordner, "codex-auth.json")); strings.Contains(string(roh), "konto-123") {
		t.Error("echtes Konto in der Sandbox-Anmeldung")
	}
}

func TestProxyChatGPT(t *testing.T) {
	var kopf http.Header
	var pfad, modell string
	up := upstreamAttrappe(t, &kopf, &pfad, &modell)
	defer up.Close()
	tok := codexSandboxToken(strings.Repeat("z", 36))
	k := proxyKonfig{upstream: up.URL + "/backend-api/codex", upstreamKey: "echter-zugang", konto: "konto-123", api: "responses", modell: "gpt-x", aufgabenTok: tok, budget: 1000}
	var verbraucht atomic.Int64
	p := httptest.NewServer(proxyHandler(k, &verbraucht))
	defer p.Close()
	req, _ := http.NewRequest("POST", p.URL+"/v1/responses", strings.NewReader(`{"model":"gpt-x","stream":true}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("ChatGPT-Account-ID", codexKontoSandbox)
	req.Header.Set("originator", "codex_exec")
	r, err := http.DefaultClient.Do(req)
	if err != nil || r.StatusCode != 200 {
		t.Fatalf("%v %v", err, r)
	}
	io.ReadAll(r.Body)
	if pfad != "/backend-api/codex/responses" || kopf.Get("Authorization") != "Bearer echter-zugang" ||
		kopf.Get("ChatGPT-Account-ID") != "konto-123" || kopf.Get("originator") != "codex_exec" {
		t.Errorf("Pfad %q, Kopf %v", pfad, kopf)
	}
	// Modellliste kommt vom Backend, mit echtem Zugang.
	req, _ = http.NewRequest("GET", p.URL+"/v1/models?client_version=0.162.1", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	if r, err = http.DefaultClient.Do(req); err != nil || r.StatusCode != 200 {
		t.Fatalf("models: %v %v", err, r)
	}
	if pfad != "/backend-api/codex/models?client_version=0.162.1" || kopf.Get("Authorization") != "Bearer echter-zugang" {
		t.Errorf("models: Pfad %q", pfad)
	}
}
