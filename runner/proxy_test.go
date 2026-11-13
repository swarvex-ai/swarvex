package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
	k := proxyKonfig{upstream: up.URL, api: "responses", modell: "m", aufgabenTok: tok, budget: 20}
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

// Budget 0 = keine Grenze (lokale Modelle): der Proxy lässt jede Anfrage durch.
func TestProxyOhneBudget(t *testing.T) {
	var kopf http.Header
	var pfad, modell string
	up := upstreamAttrappe(t, &kopf, &pfad, &modell)
	defer up.Close()
	tok := strings.Repeat("t", 40)
	var verbraucht atomic.Int64
	p := httptest.NewServer(proxyHandler(proxyKonfig{upstream: up.URL, api: "responses", modell: "m", aufgabenTok: tok, budget: 0}, &verbraucht))
	defer p.Close()
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("POST", p.URL+"/v1/responses", strings.NewReader(`{}`))
		req.Header.Set("authorization", "Bearer "+tok)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.ReadAll(r.Body)
		if r.StatusCode != 200 {
			t.Fatalf("Anfrage %d: %d (verbraucht %d)", i, r.StatusCode, verbraucht.Load())
		}
	}
}

// Ist der Anbieter nicht erreichbar, bekommt die Sandbox 502, und der Runner erkennt das später am Ereignis "unreachable".
func TestProxyNichtErreichbar(t *testing.T) {
	up := httptest.NewServer(http.NotFoundHandler())
	ziel := up.URL
	up.Close()
	tok := strings.Repeat("t", 40)
	var verbraucht atomic.Int64
	p := httptest.NewServer(proxyHandler(proxyKonfig{upstream: ziel, api: "responses", modell: "m", aufgabenTok: tok, budget: 0}, &verbraucht))
	defer p.Close()
	req, _ := http.NewRequest("POST", p.URL+"/v1/responses", strings.NewReader(`{}`))
	req.Header.Set("authorization", "Bearer "+tok)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.ReadAll(r.Body)
	if r.StatusCode != http.StatusBadGateway {
		t.Errorf("Anbieter nicht erreichbar: %d statt 502", r.StatusCode)
	}
}

func TestZaehlerFuersBudget(t *testing.T) {
	faelle := []struct {
		name     string
		ereignis string
		summe    int64
		budget   int64
	}{
		// OpenRouter/OpenAI: 40.000 Eingabe, davon 36.000 aus dem Cache, 500 Ausgabe
		{"chat-cache", `{"usage":{"prompt_tokens":40000,"completion_tokens":500,"total_tokens":40500,"prompt_tokens_details":{"cached_tokens":36000}}}`, 40500, 40500 - 36000 + 3600},
		// DeepSeek direkt
		{"deepseek", `{"usage":{"prompt_tokens":20000,"completion_tokens":100,"total_tokens":20100,"prompt_cache_hit_tokens":19000}}`, 20100, 20100 - 19000 + 1900},
		// Anthropic: input_tokens ohne Cache
		{"anthropic", `{"type":"message","usage":{"input_tokens":50,"cache_read_input_tokens":1000,"output_tokens":7}}`, 1057, 1057 - 1000 + 100},
		// Responses
		{"responses", `{"type":"response.completed","response":{"usage":{"input_tokens":70,"output_tokens":30,"total_tokens":100,"input_tokens_details":{"cached_tokens":50}}}}`, 100, 100 - 50 + 5},
		{"ohne-cache", `{"usage":{"total_tokens":100}}`, 100, 100},
	}
	for _, f := range faelle {
		var z zaehler
		z.lesen([]byte(f.ereignis))
		if z.summe() != f.summe || z.fuersBudget() != f.budget {
			t.Errorf("%s: summe %d (soll %d), budget %d (soll %d)", f.name, z.summe(), f.summe, z.fuersBudget(), f.budget)
		}
	}
}

// Das Budget wird vor der Anfrage reserviert (Sicherheitsaudit 12.11.2026): Parallele Anfragen überziehen es nicht
// mehr, und ein vor der Nutzungsmeldung abgebrochener Stream kostet mindestens seine Reservierung statt 0.
func TestProxyBudgetReserviert(t *testing.T) {
	tok := strings.Repeat("t", 40)
	var gestartet atomic.Int32
	alle := make(chan struct{})
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var a map[string]any
		json.NewDecoder(r.Body).Decode(&a)
		grenze, _ := a["max_tokens"].(float64)
		if a["stream"] == true {
			w.Header().Set("content-type", "text/event-stream")
			io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"x\"},\"finish_reason\":\"stop\"}]}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done() // die Nutzung käme erst danach: der Agent bricht vorher ab
			return
		}
		if gestartet.Add(1) == 5 {
			close(alle)
		}
		select {
		case <-alle:
		case <-time.After(2 * time.Second):
		}
		w.Header().Set("content-type", "application/json")
		fmt.Fprintf(w, `{"usage":{"total_tokens":%d}}`, 10+int(grenze))
	}))
	defer up.Close()
	k := proxyKonfig{upstream: up.URL, api: "chat", modell: "m", aufgabenTok: tok, budget: 1000}
	var verbraucht atomic.Int64
	p := httptest.NewServer(proxyHandler(k, &verbraucht))
	defer p.Close()
	senden := func(body string) int {
		req, _ := http.NewRequest("POST", p.URL+"/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("authorization", "Bearer "+tok)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0
		}
		io.ReadAll(r.Body)
		r.Body.Close()
		return r.StatusCode
	}
	// fünf parallele Anfragen, jede will 900 Tokens Ausgabe
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() { defer wg.Done(); senden(`{"max_tokens":900,"messages":[]}`) }()
	}
	wg.Wait()
	if v := verbraucht.Load(); v > k.budget {
		t.Fatalf("parallel requests used %d of a budget of %d", v, k.budget)
	}
	// abgebrochene Streams: kosten ihre Reservierung, bis das Budget erschöpft ist
	verbraucht.Store(0)
	abgebrochen := 0
	for range 20 {
		ctx, stopp := context.WithCancel(context.Background())
		req, _ := http.NewRequestWithContext(ctx, "POST", p.URL+"/v1/chat/completions", strings.NewReader(`{"stream":true,"max_tokens":400,"messages":[]}`))
		req.Header.Set("authorization", "Bearer "+tok)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			stopp()
			continue
		}
		if r.StatusCode == http.StatusTooManyRequests {
			r.Body.Close()
			stopp()
			break
		}
		bufio.NewReader(r.Body).ReadString('\n') // erstes Ereignis lesen, dann abbrechen
		stopp()
		r.Body.Close()
		abgebrochen++
		time.Sleep(50 * time.Millisecond) // der Proxy rechnet ab
	}
	if abgebrochen == 0 || abgebrochen >= 20 || verbraucht.Load() == 0 {
		t.Fatalf("abandoned streams: %d before the budget stopped them, %d charged", abgebrochen, verbraucht.Load())
	}
}
