package main

// Werkzeug-Brücke: läuft im Werkzeug-Container (`swarvex proxy tool`) und bietet der Sandbox einen MCP-Server des
// Mitstreiters (Meshy, ElevenLabs …) als Streamable HTTP an, nur mit dem Wegwerf-Token des Auftrags. Der Server läuft
// lokal als Kindprozess (stdio) oder liegt beim Anbieter (Streamable HTTP); den Schlüssel kennt nur dieser Container.
// Die Brücke zählt Werkzeug-Aufrufe: Ein Auftrag darf nur wenige erzeugende Aufrufe machen, damit ein verwirrter
// oder manipulierter Agent keine Credits verbrennt. Abfragen (Status, Liste, Download) zählen nicht dagegen.
//
//	SWARVEX_TOOL_TOKEN      Wegwerf-Token der Sandbox (mindestens 32 Zeichen)
//	SWARVEX_TOOL_COMMAND    lokaler Server: Programm und Argumente als JSON-Liste
//	SWARVEX_TOOL_URL        entfernter Server: Adresse; SWARVEX_TOOL_HEADERS seine Kopfzeilen (JSON)
//	SWARVEX_TOOL_MAX_CALLS  erzeugende Aufrufe je Auftrag (Standard 8), SWARVEX_TOOL_MAX_ALL alle Aufrufe (Standard 300)

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Werkzeuge, die nur abfragen oder ablegen, was schon erzeugt ist: zählen nicht als erzeugend. Entschieden wird an
// ganzen Wortteilen des Namens (meshy_get_task_status, getVoices), damit etwa „retarget“ nicht als „get“ durchgeht.
var abfrageWoerter = map[string]bool{"status": true, "get": true, "list": true, "check": true, "balance": true, "usage": true,
	"credit": true, "credits": true, "download": true, "save": true, "fetch": true, "info": true, "describe": true, "search": true,
	"poll": true, "wait": true, "result": true, "results": true, "history": true, "voices": true, "models": true, "quota": true,
	"account": true, "subscription": true, "help": true, "analyze": true}

var wortGrenze = regexp.MustCompile(`[A-Z]?[a-z0-9]+|[A-Z]+`)

func istAbfrage(name string) bool {
	for _, w := range wortGrenze.FindAllString(name, -1) {
		if abfrageWoerter[strings.ToLower(w)] {
			return true
		}
	}
	return false
}

type bruecke struct {
	token     string
	maxErzeug int
	maxAlle   int
	mu        sync.Mutex
	erzeugend int
	alle      int
	aufrufe   func(ctx context.Context, nachricht []byte, antwortErwartet bool) ([]byte, error)
	wartezeit time.Duration
}

func brueckeStarten() {
	b := &bruecke{token: os.Getenv("SWARVEX_TOOL_TOKEN"), wartezeit: 45 * time.Minute}
	b.maxErzeug, _ = strconv.Atoi(envOder("SWARVEX_TOOL_MAX_CALLS", "8"))
	b.maxAlle, _ = strconv.Atoi(envOder("SWARVEX_TOOL_MAX_ALL", "300"))
	if len(b.token) < 32 {
		log.Fatal("tool: SWARVEX_TOOL_TOKEN (>=32 characters) is required")
	}
	os.Unsetenv("SWARVEX_TOOL_TOKEN") // nicht an den Kindprozess
	switch {
	case os.Getenv("SWARVEX_TOOL_COMMAND") != "":
		var befehl []string
		if err := json.Unmarshal([]byte(os.Getenv("SWARVEX_TOOL_COMMAND")), &befehl); err != nil || len(befehl) == 0 {
			log.Fatal("tool: SWARVEX_TOOL_COMMAND must be a JSON list")
		}
		os.Unsetenv("SWARVEX_TOOL_COMMAND")
		k, err := kindStarten(befehl)
		if err != nil {
			log.Fatal("tool: " + err.Error())
		}
		b.aufrufe = k.senden
	case os.Getenv("SWARVEX_TOOL_URL") != "":
		kopf := map[string]string{}
		json.Unmarshal([]byte(os.Getenv("SWARVEX_TOOL_HEADERS")), &kopf)
		os.Unsetenv("SWARVEX_TOOL_HEADERS")
		f := &fernServer{url: os.Getenv("SWARVEX_TOOL_URL"), kopf: kopf, http: &http.Client{Timeout: b.wartezeit}}
		b.aufrufe = f.senden
	default:
		log.Fatal("tool: SWARVEX_TOOL_COMMAND or SWARVEX_TOOL_URL is required")
	}
	listen := envOder("SWARVEX_LISTEN", ":8090")
	log.Printf("tool: listening on %s, at most %d generating calls", listen, b.maxErzeug)
	http.HandleFunc("/", b.behandeln)
	log.Fatal(http.ListenAndServe(listen, nil))
}

type rpc struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

func (b *bruecke) behandeln(w http.ResponseWriter, r *http.Request) {
	if !tokenPasst(r, b.token) {
		ereignis("rejected", map[string]any{"reason": "token", "path": r.URL.Path})
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodPost:
	case http.MethodDelete:
		w.WriteHeader(http.StatusOK) // Sitzung beenden: nichts zu tun
		return
	default:
		// Kein Ereignis-Strom vom Server (GET): Clients fragen dann nur per POST, wie die Spezifikation es vorsieht.
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	roh, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var n rpc
	if json.Unmarshal(roh, &n) != nil || n.Method == "" && len(n.Result) == 0 && len(n.Error) == 0 {
		http.Error(w, "one JSON-RPC message per request", http.StatusBadRequest)
		return
	}
	erwartet := n.Method != "" && len(n.ID) > 0
	if n.Method == "tools/call" {
		if abgelehnt := b.zaehlen(n); abgelehnt != nil {
			w.Header().Set("content-type", "application/json")
			w.Write(abgelehnt)
			return
		}
	}
	antwort, err := b.aufrufe(r.Context(), roh, erwartet)
	if err != nil {
		ereignis("tool_error", map[string]any{"method": n.Method, "error": err.Error()})
		if !erwartet {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("content-type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": n.ID, "error": map[string]any{"code": -32603, "message": "tool server: " + err.Error()}})
		return
	}
	if !erwartet {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.Header().Set("content-type", "application/json")
	w.Write(antwort)
}

// zaehlen: Werkzeug-Aufruf zählen; über der Grenze eine Fehlerantwort (als Ergebnis, damit der Agent sie liest).
func (b *bruecke) zaehlen(n rpc) []byte {
	var p struct {
		Name string `json:"name"`
	}
	json.Unmarshal(n.Params, &p)
	erzeugt := !istAbfrage(p.Name)
	b.mu.Lock()
	defer b.mu.Unlock()
	grund := ""
	switch {
	case b.alle >= b.maxAlle:
		grund = fmt.Sprintf("this brief may make at most %d tool calls", b.maxAlle)
	case erzeugt && b.erzeugend >= b.maxErzeug:
		grund = fmt.Sprintf("this brief may make at most %d generating calls (%s would be one more); finish with what you have", b.maxErzeug, p.Name)
	}
	if grund != "" {
		ereignis("tool_limit", map[string]any{"tool": p.Name, "generating": erzeugt})
		roh, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": n.ID, "result": map[string]any{
			"isError": true, "content": []any{map[string]any{"type": "text", "text": "swarvex: " + grund}}}})
		return roh
	}
	b.alle++
	if erzeugt {
		b.erzeugend++
	}
	ereignis("tool_call", map[string]any{"tool": p.Name, "generating": erzeugt})
	return nil
}

// ---------- lokaler Server (stdio, eine JSON-Nachricht je Zeile) ----------

type kind struct {
	mu      sync.Mutex
	ein     io.Writer
	offen   map[string]chan []byte
	beendet chan struct{}
	fehler  error
}

func kindStarten(befehl []string) (*kind, error) {
	cmd := exec.Command(befehl[0], befehl[1:]...)
	ein, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	aus, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr // Meldungen des Servers ins Protokoll des Containers
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %v", befehl[0], err)
	}
	k := &kind{ein: ein, offen: map[string]chan []byte{}, beendet: make(chan struct{})}
	go func() {
		leser := bufio.NewReaderSize(aus, 1<<20)
		for {
			zeile, err := leser.ReadBytes('\n')
			if len(bytes.TrimSpace(zeile)) > 0 {
				k.angekommen(bytes.TrimSpace(zeile))
			}
			if err != nil {
				break
			}
		}
		k.mu.Lock()
		k.fehler = fmt.Errorf("the tool server ended (%v)", cmd.Wait())
		k.mu.Unlock()
		close(k.beendet)
	}()
	return k, nil
}

// angekommen: Antworten an die wartende Anfrage; Anfragen des Servers an den Client beantwortet die Brücke selbst.
func (k *kind) angekommen(zeile []byte) {
	var n rpc
	if json.Unmarshal(zeile, &n) != nil {
		return // keine JSON-Zeile (manche Server schreiben Meldungen auf stdout)
	}
	if n.Method == "" && len(n.ID) > 0 {
		k.mu.Lock()
		c := k.offen[string(n.ID)]
		delete(k.offen, string(n.ID))
		k.mu.Unlock()
		if c != nil {
			c <- zeile
		}
		return
	}
	if n.Method != "" && len(n.ID) > 0 {
		var antwort map[string]any
		switch n.Method {
		case "ping":
			antwort = map[string]any{"jsonrpc": "2.0", "id": n.ID, "result": map[string]any{}}
		case "roots/list": // der einzige Ordner, in den das Werkzeug schreiben darf
			antwort = map[string]any{"jsonrpc": "2.0", "id": n.ID, "result": map[string]any{"roots": []any{map[string]any{"uri": "file:///out", "name": "out"}}}}
		default: // Sampling, Rückfragen an den Nutzer: gibt es hier nicht
			antwort = map[string]any{"jsonrpc": "2.0", "id": n.ID, "error": map[string]any{"code": -32601, "message": "not supported by the swarvex runner"}}
		}
		roh, _ := json.Marshal(antwort)
		k.schreiben(roh)
	}
	// Benachrichtigungen des Servers (Fortschritt, Protokoll) werden nicht weitergereicht.
}

func (k *kind) schreiben(roh []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.fehler != nil {
		return k.fehler
	}
	_, err := k.ein.Write(append(bytes.TrimSpace(roh), '\n'))
	return err
}

func (k *kind) senden(ctx context.Context, nachricht []byte, antwortErwartet bool) ([]byte, error) {
	var n rpc
	json.Unmarshal(nachricht, &n)
	var c chan []byte
	if antwortErwartet {
		c = make(chan []byte, 1)
		k.mu.Lock()
		k.offen[string(n.ID)] = c
		k.mu.Unlock()
		defer func() { k.mu.Lock(); delete(k.offen, string(n.ID)); k.mu.Unlock() }()
	}
	if err := k.schreiben(nachricht); err != nil {
		return nil, err
	}
	if !antwortErwartet {
		return nil, nil
	}
	ctx, stopp := context.WithTimeout(ctx, 45*time.Minute)
	defer stopp()
	select {
	case a := <-c:
		return a, nil
	case <-k.beendet:
		return nil, k.fehler
	case <-ctx.Done():
		return nil, fmt.Errorf("no answer from the tool server")
	}
}

// ---------- entfernter Server (Streamable HTTP beim Anbieter) ----------

type fernServer struct {
	url     string
	kopf    map[string]string
	http    *http.Client
	mu      sync.Mutex
	sitzung string
}

func (f *fernServer) senden(ctx context.Context, nachricht []byte, antwortErwartet bool) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, f.url, bytes.NewReader(nachricht))
	for k, v := range f.kopf {
		req.Header.Set(k, v)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json, text/event-stream")
	f.mu.Lock()
	if f.sitzung != "" {
		req.Header.Set("Mcp-Session-Id", f.sitzung)
	}
	f.mu.Unlock()
	r, err := f.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if s := r.Header.Get("Mcp-Session-Id"); s != "" {
		f.mu.Lock()
		f.sitzung = s
		f.mu.Unlock()
	}
	if r.StatusCode >= 300 {
		roh, _ := io.ReadAll(io.LimitReader(r.Body, 2000))
		return nil, fmt.Errorf("HTTP %d: %s", r.StatusCode, strings.TrimSpace(string(roh)))
	}
	if !antwortErwartet {
		return nil, nil
	}
	var n rpc
	json.Unmarshal(nachricht, &n)
	if !strings.HasPrefix(r.Header.Get("content-type"), "text/event-stream") {
		return io.ReadAll(io.LimitReader(r.Body, 64<<20))
	}
	// Ereignis-Strom: die Antwort mit derselben ID suchen; Fortschritt davor überspringen
	leser := bufio.NewReaderSize(r.Body, 1<<20)
	var daten bytes.Buffer
	for {
		zeile, err := leser.ReadBytes('\n')
		z := bytes.TrimRight(zeile, "\r\n")
		switch {
		case bytes.HasPrefix(z, []byte("data:")):
			daten.Write(bytes.TrimSpace(z[5:]))
		case len(z) == 0 && daten.Len() > 0:
			var a rpc
			if json.Unmarshal(daten.Bytes(), &a) == nil && a.Method == "" && string(a.ID) == string(n.ID) {
				return append([]byte(nil), daten.Bytes()...), nil
			}
			daten.Reset()
		}
		if err != nil {
			return nil, fmt.Errorf("the tool server's stream ended without an answer")
		}
	}
}
