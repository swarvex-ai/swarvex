package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// rotierenderServer verhält sich wie die Plattform: neues Refresh-Token bei jeder Erneuerung, und ein schon
// verbrauchtes widerruft die ganze Anmeldung.
type rotierenderServer struct {
	mu         sync.Mutex
	gueltig    string
	verbraucht map[string]bool
	widerrufen bool
	n          int
}

func (s *rotierenderServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	s.mu.Lock()
	defer s.mu.Unlock()
	rt := r.Form.Get("refresh_token")
	if s.verbraucht[rt] {
		s.widerrufen = true
	}
	if s.widerrufen || rt != s.gueltig {
		w.WriteHeader(400)
		fmt.Fprint(w, `{"error":"invalid_grant","error_description":"grant request is invalid"}`)
		return
	}
	s.verbraucht[rt] = true
	s.n++
	s.gueltig = fmt.Sprintf("rt%d", s.n)
	fmt.Fprintf(w, `{"access_token":"at%d","refresh_token":%q}`, s.n, s.gueltig)
}

func zugangsTest(t *testing.T) (*rotierenderServer, *httptest.Server) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := &rotierenderServer{gueltig: "rt0", verbraucht: map[string]bool{}}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	if err := zugangSpeichern(&zugangsdaten{Plattform: srv.URL, ClientID: "runner", RefreshToken: "rt0"}); err != nil {
		t.Fatal(err)
	}
	return s, srv
}

// Eine laufende Aufgabe hält ihre Zugangsdaten im Speicher; erneuert inzwischen ein anderer Prozess
// (swarvex status, ein zweiter Runner), darf ihr nächster Herzschlag die Anmeldung nicht widerrufen.
func TestTokenErneuernNachAnderemProzess(t *testing.T) {
	s, _ := zugangsTest(t)
	aufgabe, _ := zugangLaden()
	if _, err := zugriffstoken(aufgabe); err != nil {
		t.Fatal(err)
	}
	status, _ := zugangLaden()
	if _, err := zugriffstoken(status); err != nil {
		t.Fatal(err)
	}
	if _, err := zugriffstoken(aufgabe); err != nil {
		t.Fatalf("heartbeat after another process refreshed: %v", err)
	}
	if s.widerrufen {
		t.Fatal("sign-in revoked")
	}
}

func TestTokenErneuernGleichzeitig(t *testing.T) {
	s, _ := zugangsTest(t)
	var wg sync.WaitGroup
	fehler := make(chan error, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			z, err := zugangLaden()
			if err == nil {
				_, err = zugriffstoken(z)
			}
			if err != nil {
				fehler <- err
			}
		}()
	}
	wg.Wait()
	close(fehler)
	for err := range fehler {
		t.Error(err)
	}
	if s.widerrufen || s.n != 12 {
		t.Fatalf("revoked=%v refreshes=%d", s.widerrufen, s.n)
	}
	if _, err := os.Stat(filepath.Join(konfigOrdner(), "access.json.neu")); !os.IsNotExist(err) {
		t.Fatal("temporary file left behind")
	}
}

func TestLoginWennNoetig(t *testing.T) {
	s, srv := zugangsTest(t)
	// gültig: nichts tun, Plattform aus der gespeicherten Anmeldung (ohne --platform)
	aus := ausgabeVon(t, func() { loginBefehl([]string{"--if-needed"}) })
	if !strings.Contains(aus, "already signed in") {
		t.Fatalf("output: %q", aus)
	}
	z, _ := zugangLaden()
	if z.Plattform != srv.URL || z.RefreshToken != "rt1" {
		t.Fatalf("access after check: %+v", z)
	}
	// Netzfehler: nichts überschreiben
	srv.Close()
	aus = ausgabeVon(t, func() { loginBefehl([]string{"--if-needed"}) })
	if !strings.Contains(aus, "could not check") {
		t.Fatalf("output: %q", aus)
	}
	if z2, _ := zugangLaden(); z2.RefreshToken != "rt1" {
		t.Fatal("sign-in overwritten after a network error")
	}
	_ = s
}

func TestZugriffstokenAbgemeldet(t *testing.T) {
	s, _ := zugangsTest(t)
	s.widerrufen = true
	z, _ := zugangLaden()
	_, err := zugriffstoken(z)
	if err == nil || !strings.Contains(err.Error(), "token refresh failed: grant request is invalid") {
		t.Fatalf("error: %v", err)
	}
}
