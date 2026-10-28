package main

// Schlüssel-Proxy: der einzige Weg aus der Sandbox hinaus.
// Die Sandbox kennt nur ein Wegwerf-Token pro Aufgabe; den echten Schlüssel des Mitstreiters
// setzt erst der Proxy ein. Er lässt nur das freigegebene Modell und nur die Modell-Schnittstelle
// des gewählten Agenten durch und bricht hart ab, sobald das Token-Budget verbraucht ist.
//
// Schnittstellen (SWARVEX_API, je Agent, E-32):
//   chat       OpenAI Chat Completions (opencode)      POST /v1/chat/completions
//   anthropic  Anthropic Messages (Claude Code)         POST /v1/messages, /v1/messages/count_tokens
//   responses  OpenAI Responses (Codex)                 POST /v1/responses
//
// Codex mit ChatGPT-Abo (SWARVEX_ACCOUNT_ID gesetzt): Ziel ist das ChatGPT-Backend von Codex
// (…/backend-api/codex/responses, ohne /v1), mit Zugangstoken und Kopfzeile ChatGPT-Account-ID.

import (
	"bufio"
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type proxyKonfig struct {
	upstream    string // z. B. http://127.0.0.1:8080 (ohne /v1)
	upstreamKey string // echter Schlüssel (darf leer sein)
	konto       string // nur ChatGPT-Abo: echte Konto-ID
	api         string // chat, anthropic oder responses
	modell      string // einzig erlaubtes Modell
	aufgabenTok string // Wegwerf-Token der Sandbox
	budget      int64  // max. Tokens für diese Aufgabe
	listen      string
}

// Pfade je Schnittstelle: true = kostet Tokens (Budget prüfen, Verbrauch zählen).
var proxyPfade = map[string]map[string]bool{
	"chat":      {"/v1/chat/completions": true},
	"anthropic": {"/v1/messages": true, "/v1/messages/count_tokens": false},
	"responses": {"/v1/responses": true},
}

// Kopfzeilen, die nie weitergereicht werden: Anmeldung setzt der Proxy, Kodierung regelt Go selbst.
var proxyOhneKopf = map[string]bool{"authorization": true, "x-api-key": true, "host": true, "content-length": true,
	"accept-encoding": true, "connection": true, "keep-alive": true, "transfer-encoding": true, "te": true, "upgrade": true,
	"proxy-authorization": true, "cookie": true}

func proxyStarten() {
	k := proxyKonfig{
		upstream:    strings.TrimRight(os.Getenv("SWARVEX_UPSTREAM"), "/"),
		upstreamKey: os.Getenv("SWARVEX_UPSTREAM_KEY"),
		konto:       os.Getenv("SWARVEX_ACCOUNT_ID"),
		api:         envOder("SWARVEX_API", "chat"),
		modell:      os.Getenv("SWARVEX_MODEL"),
		aufgabenTok: os.Getenv("SWARVEX_TASK_TOKEN"),
		listen:      envOder("SWARVEX_LISTEN", ":8080"),
	}
	k.budget, _ = strconv.ParseInt(envOder("SWARVEX_BUDGET_TOKENS", "200000"), 10, 64)
	if k.upstream == "" || k.modell == "" || len(k.aufgabenTok) < 32 || proxyPfade[k.api] == nil {
		log.Fatal("proxy: SWARVEX_UPSTREAM, SWARVEX_MODEL, SWARVEX_TASK_TOKEN (>=32 characters) and SWARVEX_API (chat, anthropic, responses) are required")
	}
	// Den echten Schlüssel nicht in der Umgebung stehen lassen (z. B. für Kindprozesse).
	os.Unsetenv("SWARVEX_UPSTREAM_KEY")
	var verbraucht atomic.Int64
	log.Printf("proxy: listening on %s, %s API, model %q, budget %d tokens", k.listen, k.api, k.modell, k.budget)
	http.HandleFunc("/", proxyHandler(k, &verbraucht))
	log.Fatal(http.ListenAndServe(k.listen, nil))
}

func proxyHandler(k proxyKonfig, verbraucht *atomic.Int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !tokenPasst(r, k.aufgabenTok) {
			ereignis("rejected", map[string]any{"reason": "token", "path": r.URL.Path})
			http.Error(w, `{"error":{"message":"invalid token"}}`, http.StatusUnauthorized)
			return
		}
		kostet, erlaubt := proxyPfade[k.api][r.URL.Path]
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/models" && k.konto != "":
			// ChatGPT-Abo: Codex erwartet die Modellliste des Backends (eigenes Format).
			modelleHolen(w, r, k)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
			w.Header().Set("content-type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": []any{map[string]any{"id": k.modell, "object": "model", "type": "model", "display_name": k.modell}}})
		case r.Method == http.MethodPost && erlaubt:
			if kostet && verbraucht.Load() >= k.budget {
				ereignis("budget_exhausted", map[string]any{"used": verbraucht.Load(), "budget": k.budget})
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				io.WriteString(w, `{"error":{"message":"the token budget for this task is used up","type":"budget_exhausted"}}`)
				return
			}
			weiterleiten(w, r, k, verbraucht, kostet)
		default:
			ereignis("rejected", map[string]any{"reason": "path", "method": r.Method, "path": r.URL.Path})
			http.Error(w, `{"error":{"message":"not allowed"}}`, http.StatusForbidden)
		}
	}
}

// modelleHolen: Modellliste des ChatGPT-Backends, ohne Verbrauch (nur lesend, nur dieser Pfad).
func modelleHolen(w http.ResponseWriter, r *http.Request, k proxyKonfig) {
	ziel := k.upstream + "/models"
	if r.URL.RawQuery != "" {
		ziel += "?" + r.URL.RawQuery
	}
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, ziel, nil)
	for name, werte := range r.Header {
		if !proxyOhneKopf[strings.ToLower(name)] {
			req.Header[name] = werte
		}
	}
	anmelden(req, k.api, k.upstreamKey)
	req.Header.Set("ChatGPT-Account-ID", k.konto)
	antwort, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, `{"error":{"message":"upstream unreachable"}}`, http.StatusBadGateway)
		return
	}
	defer antwort.Body.Close()
	if v := antwort.Header.Get("content-type"); v != "" {
		w.Header().Set("content-type", v)
	}
	w.WriteHeader(antwort.StatusCode)
	io.Copy(w, io.LimitReader(antwort.Body, 8<<20))
	ereignis("models", map[string]any{"status": antwort.StatusCode})
}

// tokenPasst: Claude Code schickt das Token je nach Einstellung als Bearer oder als x-api-key.
func tokenPasst(r *http.Request, tok string) bool {
	gleich := func(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }
	return gleich(r.Header.Get("Authorization"), "Bearer "+tok) || gleich(r.Header.Get("x-api-key"), tok)
}

// anmelden setzt den echten Schlüssel. Anthropic: API-Schlüssel als x-api-key, ein Abo-Token aus
// `claude setup-token` (sk-ant-oat…) als Bearer mit der OAuth-Beta-Kennung.
func anmelden(req *http.Request, api, key string) {
	if key == "" {
		return
	}
	if api != "anthropic" {
		req.Header.Set("Authorization", "Bearer "+key)
		return
	}
	if !strings.HasPrefix(key, "sk-ant-oat") {
		req.Header.Set("x-api-key", key)
		return
	}
	req.Header.Set("Authorization", "Bearer "+key)
	if beta := req.Header.Get("anthropic-beta"); !strings.Contains(beta, "oauth-2025-04-20") {
		req.Header.Set("anthropic-beta", strings.TrimLeft(beta+",oauth-2025-04-20", ","))
	}
}

func weiterleiten(w http.ResponseWriter, r *http.Request, k proxyKonfig, verbraucht *atomic.Int64, kostet bool) {
	var anfrage map[string]any
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<20)).Decode(&anfrage); err != nil {
		http.Error(w, `{"error":{"message":"bad json"}}`, http.StatusBadRequest)
		return
	}
	anfrage["model"] = k.modell // nur das freigegebene Modell
	stream, _ := anfrage["stream"].(bool)
	if stream && k.api == "chat" {
		anfrage["stream_options"] = map[string]any{"include_usage": true}
	}
	koerper, _ := json.Marshal(anfrage)
	ziel := k.upstream + r.URL.Path
	if k.konto != "" {
		ziel = k.upstream + strings.TrimPrefix(r.URL.Path, "/v1")
	}
	if r.URL.RawQuery != "" {
		ziel += "?" + r.URL.RawQuery
	}
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, ziel, bytes.NewReader(koerper))
	for name, werte := range r.Header {
		if !proxyOhneKopf[strings.ToLower(name)] {
			req.Header[name] = werte
		}
	}
	req.Header.Set("content-type", "application/json")
	anmelden(req, k.api, k.upstreamKey)
	if k.konto != "" {
		req.Header.Set("ChatGPT-Account-ID", k.konto) // statt der Attrappe aus der Sandbox
	}
	start := time.Now()
	antwort, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, `{"error":{"message":"upstream unreachable"}}`, http.StatusBadGateway)
		return
	}
	defer antwort.Body.Close()
	for _, kopf := range []string{"content-type", "cache-control", "request-id", "x-request-id", "retry-after"} {
		if v := antwort.Header.Get(kopf); v != "" {
			w.Header().Set(kopf, v)
		}
	}
	w.WriteHeader(antwort.StatusCode)
	var z zaehler
	if strings.HasPrefix(antwort.Header.Get("content-type"), "text/event-stream") {
		// SSE durchreichen; die Nutzung steht je nach Schnittstelle in einzelnen Ereignissen.
		flusher, _ := w.(http.Flusher)
		leser := bufio.NewReader(antwort.Body)
		for {
			zeile, err := leser.ReadBytes('\n')
			if len(zeile) > 0 {
				w.Write(zeile)
				if flusher != nil {
					flusher.Flush()
				}
				if daten, ok := bytes.CutPrefix(bytes.TrimSpace(zeile), []byte("data:")); ok {
					z.lesen(daten)
				}
			}
			if err != nil {
				break
			}
		}
	} else {
		roh, _ := io.ReadAll(io.LimitReader(antwort.Body, 32<<20))
		w.Write(roh)
		z.lesen(roh)
	}
	tokens := z.summe()
	if !kostet {
		tokens = 0
	}
	gesamt := verbraucht.Add(tokens)
	ereignis("request", map[string]any{"status": antwort.StatusCode, "tokens": tokens, "total": gesamt, "ms": time.Since(start).Milliseconds(), "stream": stream, "path": r.URL.Path})
}

// zaehler sammelt die Nutzung einer Antwort (ganz oder als Folge von SSE-Ereignissen).
// Gezählt werden alle Tokens, auch aus dem Cache gelesene: So bleibt das Budget für alle
// Schnittstellen gleich bemessen.
type zaehler struct {
	gesamt   int64 // Chat Completions, Responses: total_tokens
	ein, aus int64 // Anthropic: Eingabe (inkl. Cache) und Ausgabe
}

type nutzung struct {
	Total         int64 `json:"total_tokens"`
	Input         int64 `json:"input_tokens"`
	Output        int64 `json:"output_tokens"`
	CacheCreation int64 `json:"cache_creation_input_tokens"`
	CacheRead     int64 `json:"cache_read_input_tokens"`
}

func (z *zaehler) lesen(roh []byte) {
	var v struct {
		Art     string   `json:"type"`
		Usage   *nutzung `json:"usage"`
		Message *struct {
			Usage *nutzung `json:"usage"`
		} `json:"message"`
		Response *struct {
			Usage *nutzung `json:"usage"`
		} `json:"response"`
	}
	if json.Unmarshal(bytes.TrimSpace(roh), &v) != nil {
		return
	}
	u := v.Usage
	if u == nil && v.Message != nil {
		u = v.Message.Usage // Anthropic message_start
	}
	if u == nil && v.Response != nil {
		u = v.Response.Usage // Responses: response.completed
	}
	if u == nil {
		return
	}
	if u.Total > 0 {
		z.gesamt = u.Total
		return
	}
	// Anthropic: Zahlen sind kumulativ (message_start, dann message_delta) → jeweils das Maximum.
	z.ein = max(z.ein, u.Input+u.CacheCreation+u.CacheRead)
	z.aus = max(z.aus, u.Output)
}

func (z zaehler) summe() int64 {
	if z.gesamt > 0 {
		return z.gesamt
	}
	return z.ein + z.aus
}

// ereignis schreibt eine JSON-Zeile auf stdout; der Runner sammelt sie als Verbrauchsprotokoll.
func ereignis(art string, daten map[string]any) {
	daten["type"] = art
	daten["time"] = time.Now().UTC().Format(time.RFC3339)
	zeile, _ := json.Marshal(daten)
	fmt.Println(string(zeile))
}

func envOder(name, standard string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return standard
}
