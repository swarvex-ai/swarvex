package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func proxyZeilen(ereignisse ...string) string { return strings.Join(ereignisse, "\n") + "\n" }

func anfrage(status int) string {
	return fmt.Sprintf(`{"type":"request","status":%d,"tokens":10,"path":"/v1/messages"}`, status)
}

func TestSandboxUrsache(t *testing.T) {
	const unerreichbar = `{"type":"unreachable","path":"/v1/messages"}`
	faelle := []struct {
		name       string
		protokoll  string
		zeitlimit  bool
		art        string
		schluessel string
	}{
		// Das Modell kam nie zum Arbeiten: nicht sein Misserfolg.
		{"Schlüssel abgelehnt", proxyZeilen(anfrage(401), anfrage(401)), false, ursacheAnbieter, "provider:401"},
		{"kein Guthaben", proxyZeilen(anfrage(402)), false, ursacheAnbieter, "provider:402"},
		{"Modell unbekannt", proxyZeilen(anfrage(404)), false, ursacheAnbieter, "provider:404"},
		{"Anfrage nie angenommen", proxyZeilen(anfrage(400), anfrage(400)), false, ursacheAnbieter, "provider:400"},
		{"Anbieter nicht erreichbar", proxyZeilen(unerreichbar, unerreichbar), false, ursacheAnbieter, "provider:unreachable"},
		{"nie beantwortet, am Zeitlimit", proxyZeilen(anfrage(503), anfrage(502)), true, ursacheAnbieter, "provider:5xx"},
		{"keine Anfrage", "", false, ursacheRunner, "no-request"},
		{"keine Anfrage, am Zeitlimit", "", true, ursacheRunner, "no-request"},
		{"nur ein fremder Pfad", proxyZeilen(`{"type":"rejected","reason":"path","path":"/v1/embeddings"}`), false, ursacheRunner, "path"},
		{"ohne das Token der Sandbox", proxyZeilen(`{"type":"rejected","reason":"token","path":"/v1/messages"}`), false, ursacheRunner, "no-request"},
		// Der Anbieter schneidet mitten in der Arbeit ab.
		{"Limit mitten in der Arbeit", proxyZeilen(anfrage(200), anfrage(200), anfrage(429)), false, ursacheAnbieter, "provider:429"},
		{"Ausfall mitten in der Arbeit", proxyZeilen(anfrage(200), anfrage(529), anfrage(503)), false, ursacheAnbieter, "provider:5xx"},
		{"am Zeitlimit kam der Agent nicht mehr durch", proxyZeilen(anfrage(200), anfrage(429), anfrage(429), anfrage(429)), true, ursacheAnbieter, "provider:429"},
		{"Abo-Regeln des Betreibers", proxyZeilen(anfrage(200), `{"type":"quota_reserve","reason":"reserve"}`), false, ursacheRunner, ""},
		// Das Modell hat gearbeitet: zählt wie bisher.
		{"Absturz, obwohl das Modell antwortete", proxyZeilen(anfrage(200), anfrage(200)), false, "", "agent:exit"},
		{"letzte Anfrage falsch gestellt", proxyZeilen(anfrage(200), anfrage(400)), false, "", "agent:400"},
		{"Modell nach dem Start unbekannt", proxyZeilen(anfrage(200), anfrage(404)), false, "", "agent:404"},
		{"Zeit aufgebraucht", proxyZeilen(anfrage(200), anfrage(200)), true, "", ""},
		{"Zeit aufgebraucht, eine Anfrage scheiterte zuletzt", proxyZeilen(anfrage(200), anfrage(429)), true, "", ""},
		{"Token-Budget aufgebraucht", proxyZeilen(anfrage(200), `{"type":"budget_exhausted","used":10,"budget":10}`), false, "", ""},
		{"Abo-Regeln, danach ging es weiter", proxyZeilen(`{"type":"quota_reserve","reason":"reserve"}`, anfrage(200)), false, "", "agent:exit"},
		{"Modellliste zählt mit", proxyZeilen(`{"type":"models","status":401}`), false, ursacheAnbieter, "provider:401"},
	}
	for _, f := range faelle {
		u := sandboxUrsache(f.protokoll, f.zeitlimit)
		if u.Art != f.art || u.Schluessel != f.schluessel {
			t.Errorf("%s: Art %q, Schlüssel %q; erwartet %q, %q", f.name, u.Art, u.Schluessel, f.art, f.schluessel)
		}
		if (u.Schluessel != "" || u.Art != "") && u.Text == "" {
			t.Errorf("%s: ohne Satz für die Rückgabe", f.name)
		}
		if u.Schluessel != "" && schluesselText(u.Schluessel) == "" {
			t.Errorf("%s: Schlüssel %q ohne Worte für den Betreiber", f.name, u.Schluessel)
		}
	}
	for _, u := range []ursache{uAufgabe, uSnapshot, uLFS, uBasis, uUmgebung, uAgentImage, uProjektImage, uEinrichten, uZugang, uSandbox} {
		if schluesselText(u.Schluessel) == "" {
			t.Errorf("Schlüssel %q ohne Worte für den Betreiber", u.Schluessel)
		}
	}
}

func TestOeffentlicheZeile(t *testing.T) {
	faelle := []struct{ ein, soll string }{
		{"\x1b[31mError:\x1b[0m   model   not found", "Error: model not found"},
		{"Error: mail anna.beispiel@example.org rejected", "Error: mail [address] rejected"},
		{"401 Incorrect API key provided: sk-proj-Ab12Cd34Ef56Gh78", "401 Incorrect API key provided: [hidden]"},
		{"Authorization: Bearer abc.def.ghi failed", "[hidden] failed"},
		{`request failed api_key="geheim123" for organization org-Ab12Cd34Ef56`, "request failed [hidden] for organization [hidden]"},
		{"session 0123456789abcdef0123456789abcdef0123 expired", "session [hidden] expired"},
		{"error: key_not_found_in_configuration", "error: key_not_found_in_configuration"},
		{"panic: open /home/anna/.config/tool/settings.json: permission denied", "panic: open ~/.config/tool/settings.json: permission denied"},
		{`cannot read "C:\Users\anna\file"`, `cannot read '~\file'`},
		{"TypeError: undefined is not a function at /work/src/index.ts:12", "TypeError: undefined is not a function at /work/src/index.ts:12"},
	}
	for _, f := range faelle {
		ein := strings.Join(ausgabeEnde(f.ein, 1), "")
		if ist := oeffentlicheZeile(ein, 160); ist != f.soll {
			t.Errorf("%q: %q statt %q", f.ein, ist, f.soll)
		}
	}
	if lang := oeffentlicheZeile(strings.Repeat("wort ", 100), 160); len([]rune(lang)) != 160 || !strings.HasSuffix(lang, "…") {
		t.Errorf("Kürzen: %d Zeichen, %q", len([]rune(lang)), lang)
	}
}

func TestLetzteAusgabe(t *testing.T) {
	ausgabe := "reading files\nError: provider returned no choices\n\x1b[2mcleanup\x1b[0m\n\nbye\n"
	if z := letzteAusgabe(ausgabe); z != "Error: provider returned no choices" {
		t.Errorf("Fehlerzeile: %q", z)
	}
	if z := letzteAusgabe("step one\nstep two\n"); z != "step two" {
		t.Errorf("ohne Fehlerwort: %q", z)
	}
	if z := letzteAusgabe(" \n"); z != "" {
		t.Errorf("leere Ausgabe: %q", z)
	}
}

func TestRueckgabeNachSandbox(t *testing.T) {
	ordner := t.TempDir()
	schreiben := func(proxy, agent string) {
		os.WriteFile(filepath.Join(ordner, "proxy.log"), []byte(proxy), 0o644)
		os.WriteFile(filepath.Join(ordner, "agent.log"), []byte(agent), 0o644)
	}
	ende := agentEnde{grund: "agent exited with an error: exit status 1"}

	// Der Anbieter lehnt ab: die Ursache steht in der Rückgabe, die Fehlermeldung des Anbieters (Konto, Organisation) nicht.
	schreiben(proxyZeilen(anfrage(401)), "Error: 401 organization org-Ab12Cd34Ef56 has no access\n")
	u, grund := rueckgabeNachSandbox(ende, ordner)
	if u.Art != ursacheAnbieter || grund != "sandbox: agent exited with an error: exit status 1; the model never answered: the model provider refused the key or sign-in (HTTP 401)" {
		t.Errorf("Anbieter: %q %q", u.Art, grund)
	}
	// Absturz des Agenten: mit der letzten Zeile, maskiert.
	schreiben(proxyZeilen(anfrage(200)), "working\nfatal: cannot write /home/anna/x for anna@example.org\n")
	u, grund = rueckgabeNachSandbox(ende, ordner)
	if u.Art != "" || u.Schluessel != "agent:exit" || !strings.HasSuffix(grund, `; the model answered until the end (the cause lies in the agent or the task); last output: "fatal: cannot write ~/x for [address]"`) {
		t.Errorf("Absturz: %q %q %q", u.Art, u.Schluessel, grund)
	}
	// Zeit aufgebraucht, das Modell lief: wie bisher.
	u, grund = rueckgabeNachSandbox(agentEnde{grund: "time limit reached", zeitlimit: true}, ordner)
	if u != (ursache{}) || grund != "sandbox: time limit reached" {
		t.Errorf("Zeitlimit: %+v %q", u, grund)
	}
	// Die Sandbox kam nicht zustande.
	u, grund = rueckgabeNachSandbox(errors.New("starting the proxy: no such image"), ordner)
	if u.Art != ursacheRunner || u.Schluessel != "sandbox" || grund != "sandbox: starting the proxy: no such image" {
		t.Errorf("Sandbox: %+v %q", u, grund)
	}
	// Auch mit langer Ausgabe bleibt der Grund unter der Grenze der Plattform (400 Zeichen).
	schreiben("", strings.Repeat("x", 3000)+" error\n")
	if _, grund = rueckgabeNachSandbox(ende, ordner); len([]rune(grund)) > 400 {
		t.Errorf("Grund zu lang: %d Zeichen", len([]rune(grund)))
	}
}

func TestSerieAus(t *testing.T) {
	jetzt := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	// neueste zuerst; jeder Eintrag zehn Minuten älter als der davor
	verlauf := func(e ...verlaufEintrag) []verlaufEintrag {
		for i := range e {
			if e[i].Zeit.IsZero() {
				e[i].Zeit = jetzt.Add(-time.Duration(i) * 10 * time.Minute)
			}
			if e[i].Aufgabe == 0 && e[i].Brief == 0 {
				e[i].Aufgabe = int64(100 - i)
			}
			if e[i].Ergebnis == "" {
				e[i].Ergebnis = "returned"
			}
		}
		return e
	}
	ab := func(schluessel, projekt string) verlaufEintrag {
		return verlaufEintrag{Ursache: schluessel, Projekt: projekt}
	}
	faelle := []struct {
		name    string
		verlauf []verlaufEintrag
		anzahl  int
		projekt string
	}{
		{"leer", nil, 0, ""},
		{"drei gleiche", verlauf(ab("provider:401", "a"), ab("provider:401", "b"), ab("provider:401", "a")), 3, ""},
		{"ein Ergebnis beendet die Serie", verlauf(ab("provider:401", "a"), ab("provider:401", "a"), verlaufEintrag{Ergebnis: "submitted"}, ab("provider:401", "a")), 2, ""},
		{"Rückgabe in der Sache zuletzt", verlauf(verlaufEintrag{Grund: "sandbox: time limit reached"}, ab("provider:401", "a"), ab("provider:401", "a"), ab("provider:401", "a")), 0, ""},
		{"andere Ursache beendet die Serie", verlauf(ab("agent:exit", "a"), ab("agent:exit", "a"), ab("provider:429", "a"), ab("agent:exit", "a")), 2, ""},
		{"angehalten zählt nicht und unterbricht nicht", verlauf(ab("stopped", "a"), ab("no-request", "a"), ab("stopped", "a"), ab("no-request", "a"), ab("no-request", "a")), 3, ""},
		{"Fremdbewertung ohne Ursache sagt nichts", verlauf(ab("provider:402", "a"), verlaufEintrag{Einreichung: 7, Ergebnis: "review_not_sent"}, ab("provider:402", "a"), ab("provider:402", "a")), 3, ""},
		{"Medien-Auftrag ohne Ursache sagt nichts", verlauf(ab("provider:402", "a"), verlaufEintrag{Brief: 4, Ergebnis: "returned"}, ab("provider:402", "a"), ab("provider:402", "a")), 3, ""},
		{"gelungene Fremdbewertung beendet die Serie", verlauf(ab("provider:402", "a"), verlaufEintrag{Einreichung: 7, Ergebnis: "approved"}, ab("provider:402", "a"), ab("provider:402", "a")), 1, ""},
		{"Umgebung eines Projekts: nur dessen Runden", verlauf(ab("setup", "a"), verlaufEintrag{Ergebnis: "submitted", Projekt: "b"}, ab("setup", "a"), ab("provider:401", "b"), ab("setup", "a")), 3, "a"},
		{"Umgebung eines Projekts: sein Ergebnis beendet", verlauf(ab("setup", "a"), verlaufEintrag{Ergebnis: "submitted", Projekt: "a"}, ab("setup", "a")), 1, "a"},
		{"Projekt-Absturz unterbricht die Serie des Runners nicht", verlauf(ab("provider:401", "a"), ab("setup", "b"), ab("provider:401", "a"), ab("provider:401", "b")), 3, ""},
		{"zwei Tage Abstand trennen", verlauf(ab("provider:401", "a"), ab("provider:401", "a"),
			verlaufEintrag{Ursache: "provider:401", Zeit: jetzt.Add(-60 * time.Hour)}, verlaufEintrag{Ursache: "provider:401", Zeit: jetzt.Add(-61 * time.Hour)}), 2, ""},
	}
	for _, f := range faelle {
		s := serieAus(f.verlauf)
		if s.Anzahl != f.anzahl || s.Projekt != f.projekt {
			t.Errorf("%s: %d (Projekt %q) statt %d (%q)", f.name, s.Anzahl, s.Projekt, f.anzahl, f.projekt)
		}
	}
	s := serieAus(verlauf(ab("provider:401", "a"), ab("provider:401", "a"), ab("provider:401", "a")))
	if !s.Zeit.Equal(jetzt) || !strings.Contains(s.text(), "3 rounds in a row") || !strings.Contains(s.text(), "HTTP 401") {
		t.Errorf("Serie: %+v, %q", s, s.text())
	}
}

func TestSeriePause(t *testing.T) {
	for anzahl, soll := range map[int]time.Duration{0: 0, 2: 0, 3: time.Hour, 4: 2 * time.Hour, 5: 4 * time.Hour, 6: 8 * time.Hour, 7: 12 * time.Hour, 40: 12 * time.Hour} {
		if ist := (serie{Anzahl: anzahl}).pause(); ist != soll {
			t.Errorf("%d in Folge: %s statt %s", anzahl, ist, soll)
		}
	}
}

func TestProjektPausen(t *testing.T) {
	jetzt := time.Now()
	pausen := map[string]time.Time{"b": jetzt.Add(time.Hour), "a": jetzt.Add(time.Minute), "vorbei": jetzt.Add(-time.Second)}
	if got := nochAusgesetzt(pausen, jetzt); strings.Join(got, ",") != "a,b" {
		t.Errorf("nochAusgesetzt = %q", got)
	}
	if _, da := pausen["vorbei"]; da {
		t.Error("eine abgelaufene Pause bleibt nicht stehen")
	}
	bleiben, aus := ohneAusgesetzte([]string{"a", "b", "c"}, "c,a")
	if strings.Join(bleiben, ",") != "b" || strings.Join(aus, ",") != "a,c" {
		t.Errorf("ohneAusgesetzte = %q, %q", bleiben, aus)
	}
	if bleiben, aus := ohneAusgesetzte([]string{"a"}, ""); len(bleiben) != 1 || aus != nil {
		t.Errorf("ohne Liste: %q, %q", bleiben, aus)
	}
}
