package main

// Restkontingent (--leftover, Beta): Abos wie Claude Pro/Max oder ChatGPT Plus/Pro haben ein kurzes Fenster (5 Stunden)
// und ein langes (eine Woche). Was am Ende der Woche ungenutzt ist, verfällt. Im Restkontingent-Modus arbeitet der Runner
// nur mit diesem Rest: erst kurz vor dem Reset des langen Fensters (--leftover-before-reset), nie über die Reserve des
// Besitzers hinaus (--leftover-reserve) und nie so, dass die laufende Sitzung voll wird (--leftover-session-max).
//
// Den Stand melden die Anbieter in Kopfzeilen jeder Antwort; der Schlüssel-Proxy liest sie mit, stoppt hart, sobald eine
// Grenze erreicht ist, und meldet den Stand als Ereignis "quota". Der Runner merkt ihn sich in quota.json und entscheidet
// damit vor der nächsten Reservierung. Mit einem API-Schlüssel verfällt nichts, dort gibt es den Modus nicht.
//
//	Anthropic (Abo-Token): anthropic-ratelimit-unified-5h-utilization, -5h-reset, -7d-utilization, -7d-reset
//	Codex (ChatGPT-Abo):   x-codex-primary-used-percent, -primary-window-minutes, -primary-reset-at, dasselbe für secondary,
//	                       im Datenstrom auch als Ereignis codex.rate_limits

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type kontingentFenster struct {
	Name    string    `json:"window"`         // "5h", "7d" oder "<Minuten>m"
	Minuten int       `json:"window_minutes"` // Länge des Fensters
	Prozent float64   `json:"used_percent"`
	Reset   time.Time `json:"resets_at"`
}

type kontingentStand struct {
	Zeit    time.Time           `json:"time"`
	Fenster []kontingentFenster `json:"windows"`
}

func fensterName(minuten int) string {
	switch minuten {
	case 300:
		return "5h"
	case 10080:
		return "7d"
	}
	return fmt.Sprintf("%dm", minuten)
}

func zahlKopf(h http.Header, name string) (float64, bool) {
	v := strings.TrimSpace(h.Get(name))
	if v == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
}

func unixZeit(s float64) time.Time { return time.Unix(int64(s), 0) }

// kontingentAusKoepfen liest den Stand aus den Kopfzeilen einer Antwort; leer, wenn der Anbieter keinen meldet.
func kontingentAusKoepfen(h http.Header, jetzt time.Time) []kontingentFenster {
	var f []kontingentFenster
	for _, w := range []struct {
		name    string
		minuten int
	}{{"5h", 300}, {"7d", 10080}} {
		anteil, ok := zahlKopf(h, "anthropic-ratelimit-unified-"+w.name+"-utilization")
		if !ok {
			continue
		}
		// Anthropic meldet einen Anteil (0,73); ein Wert über 1 wäre schon ein Prozentwert. Im Zweifel zählt der höhere Verbrauch.
		if anteil <= 1 {
			anteil *= 100
		}
		reset := jetzt.Add(time.Duration(w.minuten) * time.Minute)
		if r, ok := zahlKopf(h, "anthropic-ratelimit-unified-"+w.name+"-reset"); ok {
			reset = unixZeit(r)
		}
		f = append(f, kontingentFenster{Name: w.name, Minuten: w.minuten, Prozent: anteil, Reset: reset})
	}
	for _, teil := range []string{"primary", "secondary"} {
		p, ok := zahlKopf(h, "x-codex-"+teil+"-used-percent")
		if !ok {
			continue
		}
		minuten := 300
		if m, ok := zahlKopf(h, "x-codex-"+teil+"-window-minutes"); ok && m > 0 {
			minuten = int(m)
		}
		reset := jetzt.Add(time.Duration(minuten) * time.Minute)
		if r, ok := zahlKopf(h, "x-codex-"+teil+"-reset-at"); ok {
			reset = unixZeit(r)
		} else if r, ok := zahlKopf(h, "x-codex-"+teil+"-reset-after-seconds"); ok {
			reset = jetzt.Add(time.Duration(r) * time.Second)
		}
		f = append(f, kontingentFenster{Name: fensterName(minuten), Minuten: minuten, Prozent: p, Reset: reset})
	}
	return f
}

// kontingentAusEreignis liest das Codex-Ereignis codex.rate_limits aus dem Datenstrom.
func kontingentAusEreignis(roh []byte, jetzt time.Time) []kontingentFenster {
	type fenster struct {
		Prozent float64 `json:"used_percent"`
		Minuten int     `json:"window_minutes"`
		Reset   int64   `json:"reset_at"`
	}
	var e struct {
		Art    string `json:"type"`
		Limits *struct {
			Primary   *fenster `json:"primary"`
			Secondary *fenster `json:"secondary"`
		} `json:"rate_limits"`
	}
	if json.Unmarshal(roh, &e) != nil || e.Art != "codex.rate_limits" || e.Limits == nil {
		return nil
	}
	var f []kontingentFenster
	for _, w := range []*fenster{e.Limits.Primary, e.Limits.Secondary} {
		if w == nil {
			continue
		}
		minuten := w.Minuten
		if minuten <= 0 {
			minuten = 300
		}
		reset := jetzt.Add(time.Duration(minuten) * time.Minute)
		if w.Reset > 0 {
			reset = time.Unix(w.Reset, 0)
		}
		f = append(f, kontingentFenster{Name: fensterName(minuten), Minuten: minuten, Prozent: w.Prozent, Reset: reset})
	}
	return f
}

// fortgeschrieben: Fenster, deren Reset vorbei ist, beginnen wieder bei 0 und enden eine Fensterlänge später.
func (k kontingentStand) fortgeschrieben(jetzt time.Time) []kontingentFenster {
	r := make([]kontingentFenster, 0, len(k.Fenster))
	for _, f := range k.Fenster {
		dauer := time.Duration(max(f.Minuten, 1)) * time.Minute
		for !f.Reset.After(jetzt) {
			f.Reset = f.Reset.Add(dauer)
			f.Prozent = 0
		}
		r = append(r, f)
	}
	sort.Slice(r, func(i, j int) bool { return r[i].Minuten < r[j].Minuten })
	return r
}

// zusammenfuehren: neue Werte ersetzen die alten desselben Fensters.
func (k *kontingentStand) zusammenfuehren(neu []kontingentFenster, jetzt time.Time) bool {
	geaendert := false
	for _, n := range neu {
		gefunden := false
		for i, f := range k.Fenster {
			if f.Name == n.Name {
				gefunden = true
				if math.Abs(f.Prozent-n.Prozent) >= 0.5 || f.Reset.Unix() != n.Reset.Unix() {
					k.Fenster[i], geaendert = n, true
				}
			}
		}
		if !gefunden {
			k.Fenster, geaendert = append(k.Fenster, n), true
		}
	}
	if len(neu) > 0 {
		k.Zeit = jetzt
	}
	return geaendert
}

// ---- Regeln ----

type restRegeln struct {
	Reserve  float64       // Prozent des langen Fensters, die dem Besitzer bleiben
	VorReset time.Duration // so lange vor dem Reset des langen Fensters beginnt die Restarbeit
	Sitzung  float64       // höchstens so viele Prozent des kurzen Fensters
}

func (r restRegeln) String() string {
	return fmt.Sprintf("reserve=%g;before=%s;session=%g", r.Reserve, r.VorReset, r.Sitzung)
}

func restRegelnLesen(s string) (restRegeln, error) {
	r := restRegeln{Reserve: 10, VorReset: 24 * time.Hour, Sitzung: 80}
	for _, teil := range strings.Split(s, ";") {
		name, wert, ok := strings.Cut(strings.TrimSpace(teil), "=")
		if !ok {
			continue
		}
		var err error
		switch name {
		case "reserve":
			r.Reserve, err = strconv.ParseFloat(wert, 64)
		case "before":
			r.VorReset, err = time.ParseDuration(wert)
		case "session":
			r.Sitzung, err = strconv.ParseFloat(wert, 64)
		}
		if err != nil {
			return r, fmt.Errorf("leftover %s: %v", name, err)
		}
	}
	if r.Reserve < 0 || r.Reserve >= 100 || r.Sitzung <= 0 || r.Sitzung > 100 || r.VorReset <= 0 {
		return r, fmt.Errorf("leftover: reserve 0–99 %%, session 1–100 %%, before-reset above 0")
	}
	return r, nil
}

// pruefen: darf jetzt Arbeit beginnen bzw. weiterlaufen? Sonst der Grund und ab wann es wieder gehen kann.
// Ohne bekannten Stand ist die Antwort ja: Die erste Anfrage zeigt ihn, und der Proxy stoppt danach, falls nötig.
func (r restRegeln) pruefen(f []kontingentFenster, jetzt time.Time) (bool, string, time.Time) {
	if len(f) == 0 {
		return true, "", time.Time{}
	}
	kurz, lang := f[0], f[len(f)-1]
	start := lang.Reset.Add(-r.VorReset)
	if jetzt.Before(start) {
		return false, fmt.Sprintf("the %s window resets at %s; leftover work starts %s before", lang.Name, zeitpunkt(lang.Reset), r.VorReset), start
	}
	grenze := 100 - r.Reserve
	if lang.Name == kurz.Name {
		grenze = min(grenze, r.Sitzung)
	}
	if lang.Prozent >= grenze {
		naechster := lang.Reset.Add(time.Duration(lang.Minuten)*time.Minute - r.VorReset)
		return false, fmt.Sprintf("%.0f %% of the %s window used, your reserve starts at %.0f %%", lang.Prozent, lang.Name, grenze), naechster
	}
	if kurz.Name != lang.Name && kurz.Prozent >= r.Sitzung {
		return false, fmt.Sprintf("%.0f %% of the %s window used (at most %.0f %% in leftover mode)", kurz.Prozent, kurz.Name, r.Sitzung), kurz.Reset
	}
	return true, "", time.Time{}
}

// ---- gemerkter Stand (quota.json im Datenordner) ----

func kontingentDatei() string { return filepath.Join(datenOrdner(), "quota.json") }

func kontingentMerken(datei string, k kontingentStand) error {
	os.MkdirAll(filepath.Dir(datei), 0o755)
	roh, _ := json.MarshalIndent(k, "", "  ")
	return os.WriteFile(datei, roh, 0o644)
}

func kontingentLesen(datei string) (kontingentStand, bool) {
	var k kontingentStand
	roh, err := os.ReadFile(datei)
	if err != nil || json.Unmarshal(roh, &k) != nil || len(k.Fenster) == 0 {
		return kontingentStand{}, false
	}
	return k, true
}

// kontingentAusProtokoll: das letzte Ereignis "quota" im Protokoll des Proxys.
func kontingentAusProtokoll(protokoll string) (kontingentStand, bool) {
	var letzter kontingentStand
	gefunden := false
	for _, zeile := range strings.Split(protokoll, "\n") {
		var e struct {
			Art     string              `json:"type"`
			Zeit    time.Time           `json:"time"`
			Fenster []kontingentFenster `json:"windows"`
		}
		if json.Unmarshal([]byte(zeile), &e) == nil && e.Art == "quota" && len(e.Fenster) > 0 {
			letzter, gefunden = kontingentStand{Zeit: e.Zeit, Fenster: e.Fenster}, true
		}
	}
	return letzter, gefunden
}

// kontingentText für `swarvex status`: „5h 34 % (neu ab 18:20), 7d 71 % (neu ab So 09:00)“.
func kontingentText(k kontingentStand, jetzt time.Time) string {
	var teile []string
	for _, f := range k.fortgeschrieben(jetzt) {
		teile = append(teile, fmt.Sprintf("%s window %.0f %% (resets %s)", f.Name, f.Prozent, zeitpunkt(f.Reset)))
	}
	return strings.Join(teile, ", ")
}

// ---- im Proxy ----

// kontingentWaechter hält den Stand während einer Aufgabe; mehrere Anfragen laufen gleichzeitig.
type kontingentWaechter struct {
	mu     sync.Mutex
	stand  kontingentStand
	regeln *restRegeln // nil: nur mitlesen, nie stoppen
}

// sehen übernimmt neue Werte und meldet geänderte als Ereignis "quota".
func (w *kontingentWaechter) sehen(neu []kontingentFenster) {
	if w == nil || len(neu) == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stand.zusammenfuehren(neu, time.Now()) {
		ereignis("quota", map[string]any{"windows": w.stand.Fenster})
	}
}

// erlaubt: nil, wenn die Anfrage durch darf, sonst der Grund.
func (w *kontingentWaechter) erlaubt() error {
	if w == nil || w.regeln == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if ok, grund, _ := w.regeln.pruefen(w.stand.fortgeschrieben(time.Now()), time.Now()); !ok {
		return fmt.Errorf("%s", grund)
	}
	return nil
}
