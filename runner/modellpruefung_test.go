package main

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestModellVorschlaege(t *testing.T) {
	liste := []string{"google/gemini-3.8-flash", "google/gemini-3.8-flash:batch", "google/gemini-3.7-flash", "deepseek/deepseek-v4.1-flash", "qwen3:8b"}
	for _, f := range []struct {
		name string
		want []string
	}{
		{"gemini-3.8-flash", []string{"google/gemini-3.8-flash"}}, // der heutige Fall: Anbieter vergessen
		{"openrouter/google/gemini-3.8-flash", []string{"google/gemini-3.8-flash"}},
		{"Gemini 3.8 Flash", []string{"google/gemini-3.8-flash"}},
		{"gemini-3.8", []string{"google/gemini-3.8-flash", "google/gemini-3.8-flash:batch"}},
		{"QWEN3:8B", []string{"qwen3:8b"}},
		{"llama-9", nil},
	} {
		if got := modellVorschlaege(liste, f.name); !reflect.DeepEqual(got, f.want) {
			t.Errorf("%q: %v, want %v", f.name, got, f.want)
		}
	}
}

// setup prüft den Modellnamen beim Anbieter und schlägt den passenden vor (12.11.2026: HTTP 404 nach dem Wechsel auf Gemini).
func TestModellPruefen(t *testing.T) {
	anbieter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"data":[{"id":"google/gemini-3.8-flash"},{"id":"google/gemini-3.7-flash"},{"id":"deepseek/deepseek-v4.1-flash"}]}`))
	}))
	defer anbieter.Close()
	// tty gesetzt: wie am Terminal (ohne tty fragt setup nicht nach, etwa in Skripten)
	fragen := func(antworten string) *frager {
		return &frager{r: bufio.NewReader(strings.NewReader(antworten)), tty: os.Stdin}
	}
	w := modellWahl{Agent: "opencode", URL: anbieter.URL}
	for _, f := range []struct {
		modell, antworten, want string
	}{
		{"google/gemini-3.8-flash", "", "google/gemini-3.8-flash"},             // bekannt: keine Frage
		{"gemini-3.8-flash", "\n", "google/gemini-3.8-flash"},                  // ein Vorschlag, Enter nimmt ihn
		{"gemini-3.8-flash", "n\ny\n", "gemini-3.8-flash"},                     // abgelehnt, bewusst behalten
		{"gemini", "1\n", "google/gemini-3.8-flash"},                           // mehrere Vorschläge, der erste
		{"llama-9", "n\ngoogle/gemini-3.7-flash\n", "google/gemini-3.7-flash"}, // kein Vorschlag: neu eingeben
	} {
		w.Modell = f.modell
		if got := modellPruefen(fragen(f.antworten), w); got != f.want {
			t.Errorf("%q with answers %q: %q, want %q", f.modell, f.antworten, got, f.want)
		}
	}
	// Ohne Modellliste (nicht erreichbar) und bei anderen Agenten bleibt der Name, ohne Frage
	if got := modellPruefen(fragen(""), modellWahl{Agent: "opencode", URL: "http://127.0.0.1:1", Modell: "x"}); got != "x" {
		t.Errorf("unreachable: %q", got)
	}
	if got := modellPruefen(fragen(""), modellWahl{Agent: "claude", URL: anbieter.URL, Modell: "claude-sonnet-5-5"}); got != "claude-sonnet-5-5" {
		t.Errorf("claude: %q", got)
	}
	// Ohne Terminal (Skript, Test) keine Rückfrage; ist die Eingabe zu Ende, bleibt der Name (keine Endlosschleife)
	if got := modellPruefen(&frager{r: bufio.NewReader(strings.NewReader(""))}, modellWahl{Agent: "opencode", URL: anbieter.URL, Modell: "m1"}); got != "m1" {
		t.Errorf("without a terminal: %q", got)
	}
	if got := modellPruefen(fragen(""), modellWahl{Agent: "opencode", URL: anbieter.URL, Modell: "llama-9"}); got != "llama-9" {
		t.Errorf("end of input: %q", got)
	}
}
