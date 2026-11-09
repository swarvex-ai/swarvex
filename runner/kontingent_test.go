package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

// Gleichmäßig (0.13.3): Der Agent arbeitet über die ganze Woche, solange der Verbrauch unter der Linie bis 60 % am
// Reset liegt; nutzt der Besitzer mehr, pausiert er bis die Linie ihn einholt. Laufende Aufgaben dürfen 5 Punkte drüber.
func TestGleichmaessig(t *testing.T) {
	reset := time.Date(2026, 11, 14, 4, 0, 0, 0, time.UTC)
	woche := 7 * 24 * time.Hour
	mitte := reset.Add(-woche / 2) // Hälfte der Woche: Linie bei 30 %
	f := func(kurz, lang float64) []kontingentFenster {
		return []kontingentFenster{{Name: "5h", Minuten: 300, Prozent: kurz, Reset: mitte.Add(2 * time.Hour)}, {Name: "7d", Minuten: 10080, Prozent: lang, Reset: reset}}
	}
	r, err := restRegelnLesen("reserve=10;before=24h;session=80;pace=60")
	if err != nil || r.Gleichmaessig != 60 {
		t.Fatalf("lesen: %+v %v", r, err)
	}
	if ok, grund, _ := r.pruefen(f(10, 25), mitte); !ok {
		t.Errorf("unter der Linie (25 < 30): %s", grund)
	}
	ok, grund, ab := r.pruefen(f(10, 36), mitte)
	if ok || !strings.Contains(grund, "evenly up to 60 % allows 30 % by now") {
		t.Errorf("über der Linie: %v %q", ok, grund)
	}
	// wieder, wenn die Linie 36 % erreicht: 36/60 der Woche nach ihrem Beginn
	if want := reset.Add(-woche).Add(time.Duration(36.0 / 60 * float64(woche))); !ab.Equal(want) {
		t.Errorf("ab %v, want %v", ab, want)
	}
	if ok, grund, ab := r.pruefen(f(85, 20), mitte); ok || !ab.Equal(mitte.Add(2*time.Hour)) || !strings.Contains(grund, "stays yours") {
		t.Errorf("5-Stunden-Fenster voll: %v %q %v", ok, grund, ab)
	}
	// Eine laufende Aufgabe (Proxy) darf bis 5 Punkte über die Linie
	r.Vorlauf = laufendVorlauf
	if ok, _, _ := r.pruefen(f(10, 33), mitte); !ok {
		t.Error("laufende Aufgabe knapp über der Linie: weiter")
	}
	if ok, _, _ := r.pruefen(f(10, 36), mitte); ok {
		t.Error("laufende Aufgabe deutlich über der Linie: stoppen")
	}
	// Die letzten 24 Stunden: wie bisher der Rest bis zur Reserve
	r.Vorlauf = 0
	if ok, grund, _ := r.pruefen(f(10, 70), reset.Add(-2*time.Hour)); !ok {
		t.Errorf("vor dem Reset gilt der Rest bis 90 %%: %s", grund)
	}
	if !strings.Contains(r.String(), ";pace=60") {
		t.Errorf("String: %s", r.String())
	}
}

// Runner und Lead-Dienste mit demselben Abo teilen den neuesten Stand (0.13.4); verschiedene Abos nie.
func TestGeteilterKontingentStand(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	dir := t.TempDir()
	claude := dir + "/claude-token"
	os.WriteFile(claude, []byte("sk-ant-oat-test\n"), 0o600)
	anders := dir + "/anderes"
	os.WriteFile(anders, []byte("sk-ant-oat-anders"), 0o600)
	codex1, codex2 := dir+"/auth1.json", dir+"/auth2.json"
	os.WriteFile(codex1, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"a","account_id":"konto-1","id_token":"x"}}`), 0o600)
	os.WriteFile(codex2, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"b-erneuert","account_id":"konto-1","id_token":"y"}}`), 0o600)
	if aboKennung(claude) == "" || aboKennung(claude) == aboKennung(anders) || aboKennung("") != "" || aboKennung(dir+"/fehlt") != "" {
		t.Fatal("aboKennung: each subscription its own, empty without a file")
	}
	if aboKennung(codex1) != aboKennung(codex2) {
		t.Error("Codex: the same ChatGPT account after a token refresh")
	}
	kennung := aboKennung(claude)
	alt := time.Date(2026, 11, 9, 8, 32, 0, 0, time.UTC)
	reset := time.Date(2026, 11, 14, 4, 0, 0, 0, time.UTC)
	stand := func(z time.Time, p float64) kontingentStand {
		return kontingentStand{Zeit: z, Fenster: []kontingentFenster{{Name: "7d", Minuten: 10080, Prozent: p, Reset: reset}}}
	}
	eigen := stand(alt, 27)
	eigen.Nacht = &nachtStand{Beginn: alt, Prozent: 20}
	kontingentMerken(kontingentDatei(), eigen)
	// ein Lead-Dienst mit demselben Abo meldet später mehr Verbrauch
	kontingentTeilen(kennung, stand(alt.Add(5*time.Hour), 41))
	k, ok := kontingentAktuell(kennung)
	if !ok || k.Fenster[0].Prozent != 41 || k.Nacht == nil || k.Nacht.Prozent != 20 {
		t.Errorf("newest state with the runner's night: %+v", k)
	}
	// ein älterer Stand überschreibt den neueren nicht
	kontingentTeilen(kennung, stand(alt.Add(time.Hour), 30))
	if k, _ := kontingentAktuell(kennung); k.Fenster[0].Prozent != 41 {
		t.Errorf("an older state must not win: %+v", k)
	}
	// ein anderes Abo sieht nur den eigenen Stand
	if k, _ := kontingentAktuell(aboKennung(anders)); k.Fenster[0].Prozent != 27 {
		t.Errorf("another subscription: %+v", k)
	}
}
