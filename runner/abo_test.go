package main

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Nachtanteil: nachts bis zum Anteil der Woche je Nacht, nie ab der Grenze; vor dem Reset zählt er nicht.
func TestNachtanteil(t *testing.T) {
	basis, err := restRegelnLesen("reserve=10;before=24h;session=80")
	if err != nil {
		t.Fatal(err)
	}
	basis.NachtText, basis.NachtAnteil, basis.NachtDecke = fairNacht, 10, 80
	nacht := time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC) // Dienstag 02:00
	naechsteNacht := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	woche := func(jetzt time.Time, prozent float64, resetInStunden int) []kontingentFenster {
		return []kontingentFenster{{Name: "5h", Minuten: 300, Prozent: 5, Reset: jetzt.Add(time.Hour)},
			{Name: "7d", Minuten: 10080, Prozent: prozent, Reset: jetzt.Add(time.Duration(resetInStunden) * time.Hour)}}
	}
	mitBasis := func(jetzt time.Time, b float64) restRegeln {
		return basis.fuer(kontingentStand{Nacht: &nachtStand{Beginn: time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC), Prozent: b}}, jetzt)
	}
	for _, fall := range []struct {
		name  string
		r     restRegeln
		jetzt time.Time
		f     []kontingentFenster
		ok    bool
		ab    time.Time
	}{
		{"nachts, Beginn noch unbekannt", basis.fuer(kontingentStand{}, nacht), nacht, woche(nacht, 40, 72), true, time.Time{}},
		{"nachts, 9 von 10 % genutzt", mitBasis(nacht, 40), nacht, woche(nacht, 49, 72), true, time.Time{}},
		{"nachts, Anteil verbraucht: nächste Nacht", mitBasis(nacht, 40), nacht, woche(nacht, 50, 72), false, naechsteNacht},
		{"nachts, Woche zu 80 % verbraucht", mitBasis(nacht, 75), nacht, woche(nacht, 80, 72), false, naechsteNacht},
		{"tagsüber: bis zur nächsten Nacht", basis.fuer(kontingentStand{}, nacht.Add(10*time.Hour)), nacht.Add(10 * time.Hour), woche(nacht.Add(10*time.Hour), 40, 72), false, naechsteNacht},
		{"vor dem Reset gilt der Nachtanteil nicht", mitBasis(nacht, 40), nacht, woche(nacht, 70, 10), true, time.Time{}},
		{"die Reserve gilt immer", mitBasis(nacht, 40), nacht, woche(nacht, 91, 10), false, nacht.Add(10*time.Hour + 144*time.Hour)},
		{"Restzeit beginnt vor der nächsten Nacht", mitBasis(nacht, 40), nacht, woche(nacht, 50, 30), false, nacht.Add(6 * time.Hour)},
	} {
		ok, grund, ab := fall.r.pruefen(fall.f, fall.jetzt)
		if ok != fall.ok || !ab.Equal(fall.ab) {
			t.Errorf("%s: ok=%v ab=%s (%s)", fall.name, ok, ab, grund)
		}
	}
	// Der gemerkte Beginn gilt nur für dieselbe Nacht
	if r := mitBasis(naechsteNacht.Add(time.Hour), 40); r.NachtBasis != -1 || !r.NachtVon.Equal(naechsteNacht) {
		t.Errorf("andere Nacht: %+v", r)
	}
	// Für den Proxy: Zeitpunkte statt Text, hin und zurück
	r := mitBasis(nacht, 40)
	g, err := restRegelnLesen(r.String())
	if err != nil || !g.NachtVon.Equal(r.NachtVon) || !g.NachtBis.Equal(r.NachtBis) || g.NachtBasis != 40 || g.NachtAnteil != 10 || g.NachtDecke != 80 {
		t.Fatalf("hin und zurück: %s → %+v (%v)", r, g, err)
	}
	if !g.inNacht(nacht) || g.inNacht(nacht.Add(4*time.Hour)) {
		t.Error("die Nacht endet um 06:00")
	}
	// Alles bis 100 % ist erlaubt, darüber nicht
	voll := restRegeln{Reserve: 0, VorReset: time.Hour, Sitzung: 100, NachtText: fairNacht, NachtAnteil: 100, NachtDecke: 100}
	if err := voll.gueltig(); err != nil {
		t.Errorf("100 %%: %v", err)
	}
	voll.NachtAnteil = 101
	if voll.gueltig() == nil {
		t.Error("101 % ist kein Anteil")
	}
}

func TestZeitfensterAbschnitt(t *testing.T) {
	z, _ := zeitfensterLesen(fairNacht)
	von, bis := z.abschnitt(time.Date(2026, 11, 3, 3, 17, 42, 0, time.UTC))
	if !von.Equal(time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC)) || !bis.Equal(time.Date(2026, 11, 3, 6, 0, 0, 0, time.UTC)) {
		t.Errorf("offen: %s–%s", von, bis)
	}
	von, bis = z.abschnitt(time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC))
	if !von.Equal(time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)) || !bis.Equal(time.Date(2026, 11, 4, 6, 0, 0, 0, time.UTC)) {
		t.Errorf("nächstes: %s–%s", von, bis)
	}
}

// Der Proxy setzt den Beginn der Nacht mit der ersten Antwort und stoppt am Anteil.
func TestProxyNachtanteil(t *testing.T) {
	r := restRegeln{Reserve: 10, VorReset: 24 * time.Hour, Sitzung: 80, NachtAnteil: 10, NachtDecke: 80, NachtBasis: -1,
		NachtVon: time.Now().Add(-time.Hour), NachtBis: time.Now().Add(time.Hour)}
	w := &kontingentWaechter{regeln: &r}
	reset := time.Now().Add(72 * time.Hour)
	w.sehen([]kontingentFenster{{Name: "7d", Minuten: 10080, Prozent: 40, Reset: reset}})
	if r.NachtBasis != 40 || w.erlaubt() != nil {
		t.Fatalf("Beginn: %v, erlaubt: %v", r.NachtBasis, w.erlaubt())
	}
	w.sehen([]kontingentFenster{{Name: "7d", Minuten: 10080, Prozent: 51, Reset: reset}})
	if r.NachtBasis != 40 || w.erlaubt() == nil {
		t.Fatal("nach 11 % in der Nacht muss der Proxy stoppen")
	}
	k, ok := kontingentAusProtokoll(`{"type":"quota","time":"2026-11-03T02:00:00Z","windows":[{"window":"7d","window_minutes":10080,"used_percent":40,"resets_at":"2026-11-06T00:00:00Z"}],"night":{"start":"2026-11-03T01:00:00Z","base":40}}`)
	if !ok || k.Nacht == nil || k.Nacht.Prozent != 40 || !k.Nacht.Beginn.Equal(time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC)) {
		t.Errorf("Protokoll: %+v", k.Nacht)
	}
}

func TestAboStufen(t *testing.T) {
	for typ, erwartet := range map[string]aboPlan{
		"plus":       {Name: "ChatGPT Plus"},
		"pro":        {Name: "ChatGPT Pro", NachtAnteil: 10},
		"business":   {Name: "ChatGPT Business (work account)", Firma: true},
		"free":       {Name: "ChatGPT Free", ZuKlein: true},
		"enterprise": {Name: "ChatGPT Enterprise (work account)", Firma: true},
	} {
		if p, ok := chatgptPlan(typ); !ok || p != erwartet {
			t.Errorf("%s: %+v", typ, p)
		}
	}
	if _, ok := chatgptPlan("neu"); ok {
		t.Error("unbekannte Stufe: das Setup fragt")
	}
	datei := filepath.Join(t.TempDir(), "auth.json")
	os.WriteFile(datei, []byte(`{"auth_mode":"chatgpt","tokens":{"id_token":"`+jwtAttrappe(map[string]any{"https://api.openai.com/auth": map[string]any{"chatgpt_plan_type": "pro"}})+`"}}`), 0o600)
	if p := codexPlanLesen(datei); p != "pro" {
		t.Errorf("aus der Anmeldung: %q", p)
	}
	if a := claudePlaene[2].fairArgs(); !slices.Equal(a, []string{"--leftover", "--leftover-night", fairNacht, "--leftover-night-share", "10", "--leftover-night-max", "80"}) {
		t.Errorf("Max 20x: %v", a)
	}
	if a := claudePlaene[1].fairArgs(); a[len(a)-3] != "5" {
		t.Errorf("Max 5x: %v", a)
	}
	if a := claudePlaene[0].fairArgs(); !slices.Equal(a, []string{"--leftover"}) {
		t.Errorf("Pro: nur der Rest, %v", a)
	}
}

func TestEmpfehlung(t *testing.T) {
	abo := modellWahl{Agent: "claude", Modell: "claude-sonnet-5-5", URL: "https://api.anthropic.com", Key: "/k"}
	e := empfehlung(abo, true, claudePlaene[2], nil)
	if !e.moeglich || !slices.Contains(e.dienstArgs, "--leftover-night-share") || !slices.Contains(e.dienstArgs, "--auto-update") ||
		slices.Contains(e.dienstArgs, "--daily-budget-tokens") || !slices.Contains(e.args, "auto") {
		t.Errorf("Max 20x: %+v", e)
	}
	if e := empfehlung(abo, true, claudePlaene[3], nil); e.moeglich {
		t.Error("Firmenkonto: nie von selbst")
	}
	lokal := modellWahl{Agent: "opencode", Modell: "qwen3", URL: "http://host.containers.internal:11434", Lokal: true}
	if e := empfehlung(lokal, false, aboPlan{}, nil); slices.Contains(e.args, "--budget-tokens") || slices.Contains(e.dienstArgs, "--daily-budget-tokens") {
		t.Errorf("lokal: %+v", e)
	}
	// keine Grenze je Aufgabe: das Tagesbudget deckelt jede Aufgabe
	if e := empfehlung(modellWahl{Agent: "opencode", Modell: "m", URL: "https://openrouter.ai/api", Key: "/k"}, false, aboPlan{}, nil); !slices.Contains(e.dienstArgs, "2000000") || slices.Contains(e.args, "--budget-tokens") {
		t.Errorf("API-Schlüssel: %+v", e)
	}
}

// Unter „Anpassen“ lässt sich alles frei einstellen, auch 100 %.
func TestEigeneWerte(t *testing.T) {
	f := &frager{r: bufio.NewReader(strings.NewReader("4\n0\n12h\n100\ndaily 22:00-07:00\n100\n100\n"))}
	a := aboNutzungFragen(f, claudePlaene[2])
	text := strings.Join(a, " ")
	for _, teil := range []string{"--leftover-reserve 0", "--leftover-before-reset 12h0m0s", "--leftover-session-max 100", "--leftover-night daily 22:00-07:00", "--leftover-night-share 100", "--leftover-night-max 100"} {
		if !strings.Contains(text, teil) {
			t.Errorf("fehlt %q in %s", teil, text)
		}
	}
	f = &frager{r: bufio.NewReader(strings.NewReader("3\n"))}
	if a := aboNutzungFragen(f, claudePlaene[2]); a != nil {
		t.Errorf("ganz nutzen: ohne Restkontingent-Modus, %v", a)
	}
}

func TestNachfrageFaireNutzung(t *testing.T) {
	q := neueFragen[len(neueFragen)-2]
	if q.Name != "fair-use" {
		t.Fatal(q.Name)
	}
	if !q.offen(&einrichtungStand{mitDienst: true, dienst: []string{"--agent", "claude", "--leftover"}}) {
		t.Error("Restkontingent ohne Nachtanteil: fragen")
	}
	if q.offen(&einrichtungStand{mitDienst: true, dienst: []string{"--leftover", "--leftover-night-share", "5"}}) || q.offen(&einrichtungStand{mitDienst: true, dienst: []string{"--agent", "claude"}}) {
		t.Error("schon gesetzt oder ganz genutzt: nicht fragen")
	}
	e := &einrichtungStand{mitDienst: true, dienst: []string{"--agent", "claude", "--leftover"}}
	q.fragen(&frager{r: bufio.NewReader(strings.NewReader("2\n\n"))}, e) // Max 5x, ja
	if !e.dienstNeu || !strings.Contains(strings.Join(e.dienst, " "), "--leftover-night-share 5") {
		t.Errorf("Max 5x: %v", e.dienst)
	}
}

// Der ganze Dialog mit vorgegebenen Antworten: Schnellstart mit einer Frage, „Anpassen“ wie bisher, Codex-Abo ohne
// Frage nach der Stufe.
func TestSetupDialog(t *testing.T) {
	plattform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"projects":[]}`) }))
	defer plattform.Close()
	vorbereiten := func(t *testing.T, antworten string) string {
		heimat := t.TempDir()
		t.Setenv("HOME", heimat)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(heimat, ".config"))
		t.Setenv("XDG_DATA_HOME", filepath.Join(heimat, ".local", "share"))
		t.Setenv("PATH", t.TempDir()) // kein podman, claude, codex …
		for _, n := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OPENROUTER_API_KEY"} {
			t.Setenv(n, "")
		}
		os.MkdirAll(konfigOrdner(), 0o700)
		os.WriteFile(filepath.Join(konfigOrdner(), "access.json"), []byte(`{"platform":"`+plattform.URL+`","client_id":"c","refresh_token":"r"}`), 0o600)
		alt := neuerFrager
		t.Cleanup(func() { neuerFrager = alt })
		neuerFrager = func() (*frager, bool) { return &frager{r: bufio.NewReader(strings.NewReader(antworten))}, true }
		return heimat
	}
	t.Run("Schnellstart", func(t *testing.T) {
		vorbereiten(t, "1\ndeepseek/deepseek-v4\n\n")
		t.Setenv("OPENROUTER_API_KEY", "sk-or-test")
		args := setupDialog(false)
		text := strings.Join(args, " ")
		if !strings.Contains(text, "--project auto") || !strings.Contains(text, "--model deepseek/deepseek-v4") || strings.Contains(text, "--budget-tokens") {
			t.Fatalf("args: %v", args)
		}
		if g := gespeicherteArgs(); !slices.Equal(g, args) {
			t.Errorf("gespeichert: %v", g)
		}
		if g := gefragtLesen(gefragtDatei()); !g["fair-use"] || !g["task-budget"] {
			t.Error("die Nachfrage zur fairen Nutzung gilt als gestellt")
		}
	})
	t.Run("Anpassen", func(t *testing.T) {
		vorbereiten(t, "1\nm1\nn\nm2\n3\n200000\n")
		t.Setenv("OPENROUTER_API_KEY", "sk-or-test")
		if text := strings.Join(setupDialog(false), " "); !strings.Contains(text, "--model m2") || !strings.Contains(text, "--budget-tokens 200000") {
			t.Fatalf("args: %s", text)
		}
	})
	t.Run("Codex-Abo", func(t *testing.T) {
		heimat := vorbereiten(t, "1\ngpt-5.5\n\n")
		os.MkdirAll(filepath.Join(heimat, ".codex"), 0o700)
		os.WriteFile(filepath.Join(heimat, ".codex", "auth.json"), []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"a","account_id":"k","id_token":"`+
			jwtAttrappe(map[string]any{"https://api.openai.com/auth": map[string]any{"chatgpt_plan_type": "pro"}})+`"}}`), 0o600)
		var args []string
		aus := ausgabeVon(t, func() { args = setupDialog(false) })
		if text := strings.Join(args, " "); !strings.Contains(text, "--agent codex") || !strings.Contains(text, "auth.json") {
			t.Fatalf("args: %s", text)
		}
		if !strings.Contains(aus, "Subscription: ChatGPT Pro (from your Codex sign-in)") || strings.Contains(aus, "Which ChatGPT plan") ||
			!strings.Contains(aus, "ChatGPT Pro, fair use: nights 01:00–06:00 up to 10 % of the week (not above 80 % used)") {
			t.Errorf("Ausgabe:\n%s", aus)
		}
	})
}

// ausgabeVon: was f auf stdout schreibt.
func ausgabeVon(t *testing.T, f func()) string {
	t.Helper()
	r, w, _ := os.Pipe()
	alt := os.Stdout
	os.Stdout = w
	fertig := make(chan string)
	go func() { roh, _ := io.ReadAll(r); fertig <- string(roh) }()
	f()
	w.Close()
	os.Stdout = alt
	return <-fertig
}

// Bestehende Einrichtungen: einmal fragen, ob die Grenze je Aufgabe wegfällt (das Tagesbudget bleibt).
func TestNachfrageAufgabenBudget(t *testing.T) {
	q := neueFragen[len(neueFragen)-1]
	if q.Name != "task-budget" {
		t.Fatal(q.Name)
	}
	if q.offen(&einrichtungStand{work: []string{"--budget-tokens", "0"}}) || q.offen(&einrichtungStand{work: []string{"--project", "auto"}}) {
		t.Error("ohne Grenze: nicht fragen")
	}
	e := &einrichtungStand{mitDienst: true, work: []string{"--project", "auto", "--budget-tokens", "1000000"},
		dienst: []string{"--project", "auto", "--budget-tokens", "1000000", "--daily-budget-tokens", "2000000"}}
	if !q.offen(e) {
		t.Fatal("Grenze 1.000.000: fragen")
	}
	aus := ausgabeVon(t, func() { q.fragen(&frager{r: bufio.NewReader(strings.NewReader("\n"))}, e) })
	if w, _ := argWert(e.work, "budget-tokens"); w != "0" || !e.workNeu {
		t.Errorf("work: %v", e.work)
	}
	if w, _ := argWert(e.dienst, "budget-tokens"); w != "0" || !e.dienstNeu {
		t.Errorf("dienst: %v", e.dienst)
	}
	if !strings.Contains(aus, "2,000,000 tokens") || strings.Contains(aus, "Without a daily budget") {
		t.Errorf("Ausgabe: %q", aus)
	}
	// nein: nichts ändern
	e2 := &einrichtungStand{work: []string{"--budget-tokens", "500000"}}
	ausgabeVon(t, func() { q.fragen(&frager{r: bufio.NewReader(strings.NewReader("n\n"))}, e2) })
	if w, _ := argWert(e2.work, "budget-tokens"); w != "500000" || e2.workNeu {
		t.Errorf("nein: %v", e2.work)
	}
}
