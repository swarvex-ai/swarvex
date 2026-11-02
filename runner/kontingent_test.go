package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestKontingentAusKoepfen(t *testing.T) {
	jetzt := time.Unix(1_800_000_000, 0)
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-5h-utilization", "0.34")
	h.Set("anthropic-ratelimit-unified-5h-reset", "1800003600")
	h.Set("anthropic-ratelimit-unified-7d-utilization", "0.71")
	h.Set("anthropic-ratelimit-unified-7d-reset", "1800086400")
	f := kontingentAusKoepfen(h, jetzt)
	if len(f) != 2 || f[0].Name != "5h" || f[0].Prozent < 33.9 || f[0].Prozent > 34.1 || f[1].Name != "7d" || f[1].Reset.Unix() != 1800086400 {
		t.Fatalf("Anthropic: %+v", f)
	}
	h = http.Header{}
	h.Set("x-codex-primary-used-percent", "12.5")
	h.Set("x-codex-primary-window-minutes", "300")
	h.Set("x-codex-primary-reset-at", "1800003600")
	h.Set("x-codex-secondary-used-percent", "88")
	h.Set("x-codex-secondary-window-minutes", "10080")
	h.Set("x-codex-secondary-reset-after-seconds", "7200")
	f = kontingentAusKoepfen(h, jetzt)
	if len(f) != 2 || f[0].Name != "5h" || f[0].Prozent != 12.5 || f[1].Name != "7d" || !f[1].Reset.Equal(jetzt.Add(2*time.Hour)) {
		t.Fatalf("Codex: %+v", f)
	}
	if len(kontingentAusKoepfen(http.Header{"Content-Type": {"application/json"}}, jetzt)) != 0 {
		t.Error("ohne Kopfzeilen kein Stand")
	}
	e := kontingentAusEreignis([]byte(`{"type":"codex.rate_limits","rate_limits":{"primary":{"used_percent":40,"window_minutes":300,"reset_at":1800003600},"secondary":{"used_percent":95,"window_minutes":10080}}}`), jetzt)
	if len(e) != 2 || e[1].Name != "7d" || e[1].Prozent != 95 {
		t.Fatalf("Codex-Ereignis: %+v", e)
	}
	if kontingentAusEreignis([]byte(`{"type":"response.completed"}`), jetzt) != nil {
		t.Error("anderes Ereignis")
	}
}

func TestRestRegeln(t *testing.T) {
	r, err := restRegelnLesen("reserve=10;before=24h;session=80")
	if err != nil || r.Reserve != 10 || r.VorReset != 24*time.Hour || r.Sitzung != 80 {
		t.Fatalf("lesen: %+v %v", r, err)
	}
	if _, err := restRegelnLesen("reserve=100"); err == nil {
		t.Error("Reserve 100 % lässt nichts übrig")
	}
	if s, _ := restRegelnLesen(r.String()); s != r {
		t.Errorf("hin und zurück: %s", r)
	}
	jetzt := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	woche := func(prozent float64, inStunden int) kontingentFenster {
		return kontingentFenster{Name: "7d", Minuten: 10080, Prozent: prozent, Reset: jetzt.Add(time.Duration(inStunden) * time.Hour)}
	}
	sitzung := func(prozent float64) kontingentFenster {
		return kontingentFenster{Name: "5h", Minuten: 300, Prozent: prozent, Reset: jetzt.Add(2 * time.Hour)}
	}
	for _, fall := range []struct {
		name    string
		fenster []kontingentFenster
		ok      bool
		ab      time.Time
	}{
		{"unbekannt: die erste Anfrage zeigt den Stand", nil, true, time.Time{}},
		{"Reset in 3 Tagen: noch kein Rest", []kontingentFenster{sitzung(10), woche(40, 72)}, false, jetzt.Add(48 * time.Hour)},
		{"Reset in 10 Stunden, 40 % genutzt: Rest", []kontingentFenster{sitzung(10), woche(40, 10)}, true, time.Time{}},
		{"Reserve erreicht: erst in der nächsten Woche", []kontingentFenster{sitzung(10), woche(91, 10)}, false, jetzt.Add(10*time.Hour + 168*time.Hour - 24*time.Hour)},
		{"Sitzung voll: bis zu ihrem Reset", []kontingentFenster{sitzung(85), woche(40, 10)}, false, jetzt.Add(2 * time.Hour)},
		{"nur ein Fenster: Sitzungsgrenze gilt auch", []kontingentFenster{sitzung(85)}, false, jetzt.Add(2*time.Hour + 5*time.Hour - 24*time.Hour)},
	} {
		ok, grund, ab := r.pruefen(fall.fenster, jetzt)
		if ok != fall.ok || !ab.Equal(fall.ab) {
			t.Errorf("%s: ok=%v ab=%s (%s)", fall.name, ok, ab, grund)
		}
	}
	// Nach dem Reset beginnt ein Fenster wieder bei 0
	k := kontingentStand{Fenster: []kontingentFenster{woche(95, -1)}}
	if f := k.fortgeschrieben(jetzt); f[0].Prozent != 0 || !f[0].Reset.Equal(jetzt.Add(167*time.Hour)) {
		t.Errorf("fortgeschrieben: %+v", f)
	}
}

func TestKontingentProtokoll(t *testing.T) {
	protokoll := `{"type":"request","tokens":5}
{"type":"quota","time":"2026-11-03T12:00:00Z","windows":[{"window":"5h","window_minutes":300,"used_percent":20,"resets_at":"2026-11-03T14:00:00Z"}]}
{"type":"quota","time":"2026-11-03T12:05:00Z","windows":[{"window":"5h","window_minutes":300,"used_percent":25,"resets_at":"2026-11-03T14:00:00Z"}]}`
	k, ok := kontingentAusProtokoll(protokoll)
	if !ok || k.Fenster[0].Prozent != 25 {
		t.Fatalf("letzter Stand: %+v", k)
	}
	datei := t.TempDir() + "/quota.json"
	if err := kontingentMerken(datei, k); err != nil {
		t.Fatal(err)
	}
	if g, ok := kontingentLesen(datei); !ok || g.Fenster[0].Prozent != 25 {
		t.Errorf("gemerkt: %+v", g)
	}
}

// Der Proxy liest den Stand mit und stoppt im Restkontingent-Modus, sobald die Reserve erreicht ist.
func TestProxyStopptAnDerReserve(t *testing.T) {
	var prozent atomic.Int64
	prozent.Store(50)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.Header().Set("anthropic-ratelimit-unified-7d-utilization", fmt.Sprintf("%.2f", float64(prozent.Load())/100))
		w.Header().Set("anthropic-ratelimit-unified-7d-reset", fmt.Sprint(time.Now().Add(3*time.Hour).Unix()))
		io.WriteString(w, `{"usage":{"input_tokens":1,"output_tokens":1}}`)
	}))
	defer up.Close()
	tok := strings.Repeat("t", 40)
	r, _ := restRegelnLesen("reserve=10;before=24h;session=80")
	k := proxyKonfig{upstream: up.URL, api: "anthropic", modell: "m", aufgabenTok: tok, budget: 1000, kontingent: &kontingentWaechter{regeln: &r}}
	var verbraucht atomic.Int64
	p := httptest.NewServer(proxyHandler(k, &verbraucht))
	defer p.Close()
	senden := func() int {
		req, _ := http.NewRequest("POST", p.URL+"/v1/messages", strings.NewReader(`{}`))
		req.Header.Set("x-api-key", tok)
		a, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.ReadAll(a.Body)
		return a.StatusCode
	}
	if c := senden(); c != 200 {
		t.Fatalf("erste Anfrage: %d", c)
	}
	if c := senden(); c != 200 {
		t.Fatalf("50 %% der Woche, Reset in 3 h: %d", c)
	}
	prozent.Store(92)
	senden() // diese Antwort meldet 92 %
	if c := senden(); c != http.StatusTooManyRequests {
		t.Errorf("Reserve erreicht: %d", c)
	}
}
