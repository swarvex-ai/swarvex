package main

// Lead-Proxy: der einzige Weg aus dem Lead-Container hinaus (lead.go). Er ist der Schlüssel-Proxy einer Aufgabe
// (proxy.go: nur das freigegebene Modell, der echte Schlüssel bleibt hier), dazu
//
//	POST /mcp, GET /donations/<id>/file  zur Plattform, mit dem Zugriffstoken des Lead-Dienstes. Das Token liegt in einer
//	                                     Datei, die der Runner draußen erneuert; der Container sieht es nie.
//	:3128 (CONNECT)                      nur zum Git-Host des Projekts (Ausgangs-Proxy im Modus only, ausgang.go)
//
// Im Image swarvex-proxy: `swarvex proxy lead`. Zusätzlich zur Umgebung des Schlüssel-Proxys:
//
//	SWARVEX_PLATFORM      Adresse der Plattform, z. B. https://swarvex.ai
//	SWARVEX_PLATFORM_CA   PEM der Plattform-CA (nur lokale Tests), optional
//	SWARVEX_ACCESS_FILE   Datei mit dem aktuellen Zugriffstoken (Standard /run/lead/access-token)
//	SWARVEX_GIT_HOSTS     erlaubte Git-Hosts, z. B. github.com,*.githubusercontent.com

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

var spendenDatei = regexp.MustCompile(`^/donations/\d+/file$`)

func leadProxyStarten() {
	k := proxyKonfigLesen()
	ziel, err := url.Parse(strings.TrimRight(os.Getenv("SWARVEX_PLATFORM"), "/"))
	if err != nil || ziel.Scheme != "https" && ziel.Scheme != "http" || ziel.Host == "" {
		log.Fatal("lead proxy: SWARVEX_PLATFORM must be the platform's address, e.g. https://swarvex.ai")
	}
	regeln, err := ausgangRegelnLesen("only", os.Getenv("SWARVEX_GIT_HOSTS"))
	if err != nil || len(regeln.hosts)+len(regeln.muster) == 0 {
		log.Fatalf("lead proxy: SWARVEX_GIT_HOSTS: %v", err)
	}
	d := &net.Dialer{Timeout: 15 * time.Second}
	git := &ausgang{regeln: regeln, aufl: net.DefaultResolver.LookupIPAddr, dial: d.DialContext}
	go func() { log.Fatal(http.ListenAndServe(":3128", git)) }()

	var verbraucht atomic.Int64
	plattform := plattformWeiter(ziel, envOder("SWARVEX_ACCESS_FILE", "/run/lead/access-token"), k.aufgabenTok, httpClient(os.Getenv("SWARVEX_PLATFORM_CA")))
	modell := proxyHandler(k, &verbraucht)
	log.Printf("lead proxy: listening on %s (model %q, platform %s, git %s)", k.listen, k.modell, ziel.Host, os.Getenv("SWARVEX_GIT_HOSTS"))
	log.Fatal(http.ListenAndServe(k.listen, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp" || strings.HasPrefix(r.URL.Path, "/donations/") {
			plattform(w, r)
			return
		}
		modell(w, r)
	})))
}

// plattformWeiter reicht MCP-Aufrufe und das Herunterladen einer Spende zur Plattform durch. Der Container meldet sich
// mit seinem Wegwerf-Token an; nach außen geht das Zugriffstoken des Lead-Dienstes, frisch aus der Datei.
func plattformWeiter(ziel *url.URL, tokenDatei, tok string, client *http.Client) http.HandlerFunc {
	rp := &httputil.ReverseProxy{
		Rewrite: func(p *httputil.ProxyRequest) {
			p.SetURL(ziel)
			p.Out.Host = ziel.Host
			for _, kopf := range []string{"Authorization", "Cookie", "X-Api-Key", "Proxy-Authorization"} {
				p.Out.Header.Del(kopf)
			}
			roh, _ := os.ReadFile(tokenDatei)
			p.Out.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(roh)))
		},
		Transport:     client.Transport,
		FlushInterval: -1, // Antworten (auch lange, z. B. wait_for_work) sofort durchreichen
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			ereignis("platform_error", map[string]any{"path": r.URL.Path, "error": err.Error()})
			http.Error(w, `{"error":"platform unreachable"}`, http.StatusBadGateway)
		},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !tokenPasst(r, tok) {
			ereignis("rejected", map[string]any{"reason": "token", "path": r.URL.Path})
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		if !(r.Method == http.MethodPost && r.URL.Path == "/mcp") && !(r.Method == http.MethodGet && spendenDatei.MatchString(r.URL.Path)) {
			ereignis("rejected", map[string]any{"reason": "path", "method": r.Method, "path": r.URL.Path})
			http.Error(w, `{"error":"not allowed"}`, http.StatusForbidden)
			return
		}
		r.URL.RawQuery = ""
		ereignis("platform", map[string]any{"path": r.URL.Path})
		rp.ServeHTTP(w, r)
	}
}
