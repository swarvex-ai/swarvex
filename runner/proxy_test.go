package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestZaehler(t *testing.T) {
	faelle := []struct {
		name     string
		ereignis []string
		soll     int64
	}{
		{"chat", []string{`{"choices":[],"usage":{"prompt_tokens":90,"completion_tokens":10,"total_tokens":100}}`}, 100},
		{"chat-stream", []string{`{"choices":[{"delta":{}}]}`, `[DONE]`, `{"choices":[],"usage":{"total_tokens":42}}`}, 42},
		{"anthropic", []string{`{"type":"message","usage":{"input_tokens":50,"cache_read_input_tokens":1000,"output_tokens":7}}`}, 1057},
		{"anthropic-stream", []string{
			`{"type":"message_start","message":{"usage":{"input_tokens":20,"cache_creation_input_tokens":300,"output_tokens":1}}}`,
			`{"type":"content_block_delta","delta":{"text":"hi"}}`,
			`{"type":"message_delta","usage":{"output_tokens":15}}`}, 335},
		{"responses-stream", []string{`{"type":"response.created","response":{"usage":null}}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":70,"output_tokens":30,"total_tokens":100}}}`}, 100},
	}
	for _, f := range faelle {
		var z zaehler
		for _, e := range f.ereignis {
			z.lesen([]byte(e))
		}
		if z.summe() != f.soll {
			t.Errorf("%s: %d statt %d", f.name, z.summe(), f.soll)
		}
	}
}

// Ein Upstream, der Anfrage und Kopfzeilen zurückmeldet.
func upstreamAttrappe(t *testing.T, gesehen *http.Header, pfad *string, modell *string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gesehen = r.Header.Clone()
		*pfad = r.URL.RequestURI()
		var a map[string]any
		json.NewDecoder(r.Body).Decode(&a)
		*modell, _ = a["model"].(string)
		w.Header().Set("content-type", "text/event-stream")
		io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10,\"output_tokens\":1}}}\n\n")
		io.WriteString(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":5}}\n\n")
	}))
}

func TestProxyAnthropic(t *testing.T) {
	var kopf http.Header
	var pfad, modell string
	up := upstreamAttrappe(t, &kopf, &pfad, &modell)
	defer up.Close()
	tok := strings.Repeat("t", 40)
	for _, fall := range []struct{ key, kopfName, kopfWert string }{
		{"sk-ant-api03-echt", "x-api-key", "sk-ant-api03-echt"},
		{"sk-ant-oat01-abo", "Authorization", "Bearer sk-ant-oat01-abo"},
	} {
		k := proxyKonfig{upstream: up.URL, upstreamKey: fall.key, api: "anthropic", modell: "claude-x", aufgabenTok: tok, budget: 1000}
		var verbraucht atomic.Int64
		p := httptest.NewServer(proxyHandler(k, &verbraucht))
		req, _ := http.NewRequest("POST", p.URL+"/v1/messages?beta=true", strings.NewReader(`{"model":"claude-haiku","stream":true,"messages":[]}`))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("anthropic-beta", "claude-code-20250219")
		r, err := http.DefaultClient.Do(req)
		if err != nil || r.StatusCode != 200 {
			t.Fatalf("%s: %v %v", fall.key, err, r)
		}
		io.ReadAll(r.Body)
		p.Close()
		if kopf.Get(fall.kopfName) != fall.kopfWert || pfad != "/v1/messages?beta=true" || modell != "claude-x" {
			t.Errorf("%s: Kopf %q, Pfad %q, Modell %q", fall.key, kopf.Get(fall.kopfName), pfad, modell)
		}
		if kopf.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("%s: anthropic-version nicht weitergereicht", fall.key)
		}
		if strings.HasPrefix(fall.key, "sk-ant-oat") != strings.Contains(kopf.Get("anthropic-beta"), "oauth-2025-04-20") {
			t.Errorf("%s: anthropic-beta %q", fall.key, kopf.Get("anthropic-beta"))
		}
		if strings.Contains(kopf.Get("Authorization"), tok) || kopf.Get("x-api-key") == tok {
			t.Errorf("%s: Aufgaben-Token wurde weitergereicht", fall.key)
		}
		if verbraucht.Load() != 15 {
			t.Errorf("%s: Verbrauch %d statt 15", fall.key, verbraucht.Load())
		}
	}
}

func TestProxyPfadeUndBudget(t *testing.T) {
	var kopf http.Header
	var pfad, modell string
	up := upstreamAttrappe(t, &kopf, &pfad, &modell)
	defer up.Close()
	tok := strings.Repeat("t", 40)
	k := proxyKonfig{upstream: up.URL, api: "responses", modell: "m", aufgabenTok: tok, budget: 10}
	var verbraucht atomic.Int64
	p := httptest.NewServer(proxyHandler(k, &verbraucht))
	defer p.Close()
	senden := func(pfad, token string) int {
		req, _ := http.NewRequest("POST", p.URL+pfad, strings.NewReader(`{}`))
		req.Header.Set("x-api-key", token) // Claude Code kann das Token auch so schicken
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.ReadAll(r.Body)
		return r.StatusCode
	}
	if c := senden("/v1/messages", tok); c != http.StatusForbidden {
		t.Errorf("fremde Schnittstelle: %d", c)
	}
	if c := senden("/v1/responses", "falsch"); c != http.StatusUnauthorized {
		t.Errorf("falsches Token: %d", c)
	}
	if c := senden("/v1/responses", tok); c != 200 {
		t.Errorf("erlaubt: %d", c)
	}
	if c := senden("/v1/responses", tok); c != http.StatusTooManyRequests {
		t.Errorf("Budget (verbraucht %d): %d", verbraucht.Load(), c)
	}
}
