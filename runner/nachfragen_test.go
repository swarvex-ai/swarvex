package main

import (
	"bufio"
	"slices"
	"strings"
	"testing"
)

// Bestehende Einrichtungen mit Restkontingent bekommen 0.13.3 eine Frage: gleichmäßig bis 60 %? Ohne Antwort bleibt es.
func TestNachfrageGleichmaessig(t *testing.T) {
	q := frageNamens(t, "even-share")
	e := &einrichtungStand{mitDienst: true, dienst: []string{"--agent", "claude", "--leftover", "--leftover-night-share", "5"}}
	if !q.offen(e) {
		t.Fatal("leftover without a share: ask")
	}
	q.fragen(&frager{r: bufio.NewReader(strings.NewReader("\n"))}, e)
	if w, _ := argWert(e.dienst, "subscription-share"); w != "60" || !e.dienstNeu {
		t.Errorf("yes: %v", e.dienst)
	}
	if q.offen(e) || frageNamens(t, "fair-use").offen(e) {
		t.Error("asked once; the old night question is not needed any more")
	}
	if q.offen(&einrichtungStand{mitDienst: true, dienst: []string{"--agent", "claude"}}) {
		t.Error("open end or an API key: nothing to ask")
	}
}

func TestNachfrageMedien30(t *testing.T) {
	q := frageNamens(t, "media-30")
	e := &einrichtungStand{mitDienst: true, work: []string{"--media", "claude:meshy", "--media-daily", "10"}, dienst: []string{"--media", "claude:meshy", "--media-daily", "10"}}
	if !q.offen(e) {
		t.Fatal("the old default of 10: ask")
	}
	q.fragen(&frager{r: bufio.NewReader(strings.NewReader("\n"))}, e)
	if w, _ := argWert(e.dienst, "media-daily"); w != "30" {
		t.Errorf("service: %v", e.dienst)
	}
	if w, _ := argWert(e.work, "media-daily"); w != "30" || q.offen(e) {
		t.Errorf("work: %v", e.work)
	}
	if q.offen(&einrichtungStand{dienst: []string{"--media-daily", "5"}}) {
		t.Error("a value of your own stays")
	}
}

// swarvex budget --share / --open-end: die Abo-Regeln setzen oder ganz herausnehmen.
func TestNutzungUndArgEntfernen(t *testing.T) {
	d := []string{"--agent", "claude", "--leftover", "--leftover-night-share", "5", "--subscription-share=60", "--daily-budget-tokens", "100"}
	if got := nutzungText(d); !strings.Contains(got, "evenly up to 60 %") {
		t.Errorf("nutzungText = %q", got)
	}
	for name, mitWert := range aboRegeln {
		d = argEntfernen(d, name, mitWert)
	}
	if !slices.Equal(d, []string{"--agent", "claude", "--daily-budget-tokens", "100"}) {
		t.Errorf("argEntfernen: %v", d)
	}
	if got := nutzungText(argSetzen(d, "daily-budget-tokens", "0")); got != "open end (no limits)" {
		t.Errorf("open end: %q", got)
	}
	if got := nutzungText([]string{"--leftover"}); !strings.Contains(got, "only what would expire") {
		t.Errorf("leftover: %q", got)
	}
}

// API-Schlüssel: Wahl mit grober Kostenschätzung (Anthropic-Listenpreise), sonst nur die Rechenregel.
func TestKostenUndTagesbudget(t *testing.T) {
	if p, ok := preisFuer("anthropic/claude-sonnet-5.5"); !ok || p != 2 {
		t.Errorf("OpenRouter spelling: %v %v", p, ok)
	}
	if got := kostenText(2_000_000, "claude-opus-5-5"); got != "about $12 a day at most with claude-opus-5-5 at Anthropic's list prices" {
		t.Errorf("kostenText = %q", got)
	}
	if kostenText(2_000_000, "deepseek/deepseek-v4.1-flash") != "" {
		t.Error("unknown price: no number")
	}
	for eingabe, want := range map[string]string{"\n": "2000000", "2\n": "5000000", "3\n": "0", "4\n750000\n": "750000"} {
		if got := tagesbudgetFragen(&frager{r: bufio.NewReader(strings.NewReader(eingabe))}, "claude-sonnet-5-5"); got != want {
			t.Errorf("%q: %s, want %s", eingabe, got, want)
		}
	}
}

// Über SSH: eine abgebrochene Anmeldung nennt den Tunnel mit Port und Adresse statt „am Desktop“.
func TestAnmeldungNachholen(t *testing.T) {
	aus := "redirect_uri=http%3A%2F%2F127.0.0.1%3A27890%2Fcallback ... http://127.0.0.1:27890/callback"
	got := anmeldungNachholen("Hermes", "hermes mcp login swarvex", aus, "192.168.1.20 51234 192.168.10.50 22", "gi4t")
	if !strings.Contains(got, "ssh -N -L 27890:127.0.0.1:27890 gi4t@192.168.10.50") || !strings.Contains(got, "hermes mcp login swarvex") {
		t.Errorf("over SSH:\n%s", got)
	}
	if got := anmeldungNachholen("Hermes", "hermes mcp login swarvex", "", "", "gi4t"); strings.Contains(got, "ssh -N") || !strings.Contains(got, "on this machine: hermes mcp login swarvex") {
		t.Errorf("local:\n%s", got)
	}
}
