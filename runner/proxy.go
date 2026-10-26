package main

// Schlüssel-Proxy: der einzige Weg aus der Sandbox hinaus.
// Die Sandbox kennt nur ein Wegwerf-Token pro Aufgabe; den echten Schlüssel des Spenders
// setzt erst der Proxy ein. Er lässt nur das freigegebene Modell und nur
// Chat-Completions durch und bricht hart ab, sobald das Token-Budget verbraucht ist.

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
	upstream    string // z. B. http://127.0.0.1:8080
	upstreamKey string // echter Schlüssel (darf leer sein)
	modell      string // einzig erlaubtes Modell
	aufgabenTok string // Wegwerf-Token der Sandbox
	budget      int64  // max. Tokens für diese Aufgabe
	listen      string
}

func proxyStarten() {
	k := proxyKonfig{
		upstream:    strings.TrimRight(os.Getenv("SWARVEX_UPSTREAM"), "/"),
		upstreamKey: os.Getenv("SWARVEX_UPSTREAM_KEY"),
		modell:      os.Getenv("SWARVEX_MODEL"),
		aufgabenTok: os.Getenv("SWARVEX_TASK_TOKEN"),
		listen:      envOder("SWARVEX_LISTEN", ":8080"),
	}
	k.budget, _ = strconv.ParseInt(envOder("SWARVEX_BUDGET_TOKENS", "200000"), 10, 64)
	if k.upstream == "" || k.modell == "" || len(k.aufgabenTok) < 32 {
		log.Fatal("proxy: SWARVEX_UPSTREAM, SWARVEX_MODEL and SWARVEX_TASK_TOKEN (>=32 characters) are required")
	}
	// Den echten Schlüssel nicht in der Umgebung stehen lassen (z. B. für Kindprozesse).
	os.Unsetenv("SWARVEX_UPSTREAM_KEY")
	var verbraucht atomic.Int64
	log.Printf("proxy: listening on %s, model %q, budget %d tokens", k.listen, k.modell, k.budget)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+k.aufgabenTok)) != 1 {
			ereignis("rejected", map[string]any{"reason": "token", "path": r.URL.Path})
			http.Error(w, `{"error":{"message":"invalid token"}}`, http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
			w.Header().Set("content-type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": []any{map[string]any{"id": k.modell, "object": "model"}}})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/chat/completions":
			if verbraucht.Load() >= k.budget {
				ereignis("budget_exhausted", map[string]any{"used": verbraucht.Load(), "budget": k.budget})
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				io.WriteString(w, `{"error":{"message":"the token budget for this task is used up","type":"budget_exhausted"}}`)
				return
			}
			weiterleiten(w, r, k, &verbraucht)
		default:
			ereignis("rejected", map[string]any{"reason": "path", "method": r.Method, "path": r.URL.Path})
			http.Error(w, `{"error":{"message":"not allowed"}}`, http.StatusForbidden)
		}
	})
	log.Fatal(http.ListenAndServe(k.listen, nil))
}

func weiterleiten(w http.ResponseWriter, r *http.Request, k proxyKonfig, verbraucht *atomic.Int64) {
	var anfrage map[string]any
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&anfrage); err != nil {
		http.Error(w, `{"error":{"message":"bad json"}}`, http.StatusBadRequest)
		return
	}
	anfrage["model"] = k.modell // nur das freigegebene Modell
	stream, _ := anfrage["stream"].(bool)
	if stream {
		anfrage["stream_options"] = map[string]any{"include_usage": true}
	}
	koerper, _ := json.Marshal(anfrage)
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, k.upstream+"/v1/chat/completions", bytes.NewReader(koerper))
	req.Header.Set("content-type", "application/json")
	if k.upstreamKey != "" {
		req.Header.Set("Authorization", "Bearer "+k.upstreamKey)
	}
	start := time.Now()
	antwort, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, `{"error":{"message":"upstream unreachable"}}`, http.StatusBadGateway)
		return
	}
	defer antwort.Body.Close()
	for _, kopf := range []string{"content-type", "cache-control"} {
		if v := antwort.Header.Get(kopf); v != "" {
			w.Header().Set(kopf, v)
		}
	}
	w.WriteHeader(antwort.StatusCode)
	var tokens int64
	if stream {
		// SSE durchreichen; die Nutzungszahl steht im letzten Chunk (include_usage).
		flusher, _ := w.(http.Flusher)
		leser := bufio.NewReader(antwort.Body)
		for {
			zeile, err := leser.ReadBytes('\n')
			if len(zeile) > 0 {
				w.Write(zeile)
				if flusher != nil {
					flusher.Flush()
				}
				if n := nutzungAus(bytes.TrimPrefix(bytes.TrimSpace(zeile), []byte("data:"))); n > 0 {
					tokens = n
				}
			}
			if err != nil {
				break
			}
		}
	} else {
		roh, _ := io.ReadAll(io.LimitReader(antwort.Body, 32<<20))
		w.Write(roh)
		tokens = nutzungAus(roh)
	}
	gesamt := verbraucht.Add(tokens)
	ereignis("request", map[string]any{"status": antwort.StatusCode, "tokens": tokens, "total": gesamt, "ms": time.Since(start).Milliseconds(), "stream": stream})
}

func nutzungAus(roh []byte) int64 {
	var v struct {
		Usage *struct {
			Total int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(bytes.TrimSpace(roh), &v) == nil && v.Usage != nil {
		return v.Usage.Total
	}
	return 0
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
