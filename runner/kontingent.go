package main

// Restkontingent (--leftover, Beta): Abos wie Claude Pro/Max oder ChatGPT Plus/Pro haben ein kurzes Fenster (5 Stunden)
// und ein langes (eine Woche). Was am Ende der Woche ungenutzt ist, verfällt. Im Restkontingent-Modus arbeitet der Runner
// nur mit diesem Rest: erst kurz vor dem Reset des langen Fensters (--leftover-before-reset), nie über die Reserve des
// Besitzers hinaus (--leftover-reserve) und nie so, dass die laufende Sitzung voll wird (--leftover-session-max).
// Dazu kann ein Nachtanteil kommen (--leftover-night, -night-share, -night-max): in den Nachtfenstern je Nacht bis zu
// einem Anteil der Woche, nur solange die Woche unter einer Grenze verbraucht ist. Das Setup wählt ihn nach dem Abo.
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
	Nacht   *nachtStand         `json:"night,omitempty"` // Stand des langen Fensters zu Beginn der letzten Nacht
}

type nachtStand struct {
	Beginn  time.Time `json:"start"`
	Prozent float64   `json:"base"`
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
	// Nachtanteil: in den Nachtfenstern (NachtText, Ortszeit dieses Rechners) auch vor dem Reset, je Nacht bis
	// NachtAnteil Prozent des langen Fensters und nur, solange weniger als NachtDecke Prozent verbraucht sind.
	NachtText   string
	NachtAnteil float64
	NachtDecke  float64
	// Die laufende oder nächste Nacht als Zeitpunkte (der Runner rechnet sie aus, der Proxy vergleicht nur) und der
	// Stand des langen Fensters zu ihrem Beginn (< 0: unbekannt; die erste Antwort in der Nacht setzt ihn).
	NachtVon, NachtBis time.Time
	NachtBasis         float64
	// Gleichmäßig: über die ganze Woche, solange der Verbrauch des langen Fensters (eigener und des Agenten) unter einer
	// Linie bleibt, die von 0 zu Beginn bis Gleichmaessig Prozent am Reset steigt (0 = aus). Nutzt der Besitzer selbst
	// viel, liegt er über der Linie, und der Agent pausiert. Vorlauf: so viele Prozentpunkte darf eine laufende Aufgabe
	// über die Linie gehen (der Proxy setzt ihn), damit sie nicht mittendrin endet; begonnen wird nur darunter.
	Gleichmaessig float64
	Vorlauf       float64
}

// laufendVorlauf: so weit darf eine laufende Aufgabe über die gleichmäßige Linie gehen (Prozentpunkte der Woche).
const laufendVorlauf = 5.0

// String: die Regeln für den Proxy (ohne NachtText, die Nacht steht als Zeitpunkte darin).
func (r restRegeln) String() string {
	s := fmt.Sprintf("reserve=%g;before=%s;session=%g", r.Reserve, r.VorReset, r.Sitzung)
	if r.NachtAnteil > 0 && !r.NachtVon.IsZero() {
		s += fmt.Sprintf(";nightshare=%g;nightmax=%g;night=%d-%d;nightbase=%g", r.NachtAnteil, r.NachtDecke, r.NachtVon.Unix(), r.NachtBis.Unix(), r.NachtBasis)
	}
	if r.Gleichmaessig > 0 {
		s += fmt.Sprintf(";pace=%g", r.Gleichmaessig)
	}
	return s
}

// gueltig prüft die Werte; 100 % sind erlaubt, wo sie Sinn ergeben (alles nutzen: reserve=0, Nachtanteil 100).
func (r restRegeln) gueltig() error {
	if r.Reserve < 0 || r.Reserve >= 100 || r.Sitzung <= 0 || r.Sitzung > 100 || r.VorReset <= 0 {
		return fmt.Errorf("leftover: reserve 0–99 %%, session 1–100 %%, before-reset above 0")
	}
	if r.NachtAnteil < 0 || r.NachtAnteil > 100 || (r.NachtAnteil > 0 && (r.NachtDecke <= 0 || r.NachtDecke > 100)) {
		return fmt.Errorf("leftover: night share 0–100 %%, night max 1–100 %%")
	}
	if r.Gleichmaessig < 0 || r.Gleichmaessig > 100 {
		return fmt.Errorf("subscription share: 0–100 %% of the week")
	}
	if r.NachtText != "" {
		if _, err := zeitfensterLesen(r.NachtText); err != nil {
			return fmt.Errorf("leftover night: %v", err)
		}
	}
	return nil
}

func restRegelnLesen(s string) (restRegeln, error) {
	r := restRegeln{Reserve: 10, VorReset: 24 * time.Hour, Sitzung: 80, NachtDecke: 80, NachtBasis: -1}
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
		case "nightshare":
			r.NachtAnteil, err = strconv.ParseFloat(wert, 64)
		case "nightmax":
			r.NachtDecke, err = strconv.ParseFloat(wert, 64)
		case "nightbase":
			r.NachtBasis, err = strconv.ParseFloat(wert, 64)
		case "pace":
			r.Gleichmaessig, err = strconv.ParseFloat(wert, 64)
		case "night":
			von, bis, _ := strings.Cut(wert, "-")
			var a, b int64
			if a, err = strconv.ParseInt(von, 10, 64); err == nil {
				if b, err = strconv.ParseInt(bis, 10, 64); err == nil {
					r.NachtVon, r.NachtBis = time.Unix(a, 0), time.Unix(b, 0)
				}
			}
		}
		if err != nil {
			return r, fmt.Errorf("leftover %s: %v", name, err)
		}
	}
	return r, r.gueltig()
}

// fuer: die Regeln für diesen Moment: welche Nacht gilt (die laufende oder die nächste) und, wenn gemerkt, der Stand
// zu ihrem Beginn. Ohne Nachtanteil bleiben sie, wie sie sind.
func (r restRegeln) fuer(k kontingentStand, jetzt time.Time) restRegeln {
	z, err := zeitfensterLesen(r.NachtText)
	if r.NachtAnteil <= 0 || err != nil || len(z) == 0 {
		return r
	}
	r.NachtVon, r.NachtBis = z.abschnitt(jetzt)
	r.NachtBasis = -1
	if k.Nacht != nil && k.Nacht.Beginn.Equal(r.NachtVon) {
		r.NachtBasis = k.Nacht.Prozent
	}
	return r
}

// inNacht: liegt jetzt in der Nacht, für die diese Regeln gelten?
func (r restRegeln) inNacht(jetzt time.Time) bool {
	return r.NachtAnteil > 0 && !r.NachtVon.IsZero() && !jetzt.Before(r.NachtVon) && jetzt.Before(r.NachtBis)
}

// naechsteChance nach einer gesperrten Nacht: die nächste Nacht oder, wenn früher, der Beginn der Restzeit vor dem Reset.
func (r restRegeln) naechsteChance(restStart, jetzt time.Time) time.Time {
	n := r.NachtBis
	if z, err := zeitfensterLesen(r.NachtText); err == nil && len(z) > 0 {
		if b, ok := z.naechsterBeginn(r.NachtBis); ok {
			n = b
		}
	}
	if restStart.After(jetzt) && restStart.Before(n) {
		n = restStart
	}
	return n
}

// pruefen: darf jetzt Arbeit beginnen bzw. weiterlaufen? Sonst der Grund und ab wann es wieder gehen kann.
// Ohne bekannten Stand ist die Antwort ja: Die erste Anfrage zeigt ihn, und der Proxy stoppt danach, falls nötig.
func (r restRegeln) pruefen(f []kontingentFenster, jetzt time.Time) (bool, string, time.Time) {
	if len(f) == 0 {
		return true, "", time.Time{}
	}
	kurz, lang := f[0], f[len(f)-1]
	start := lang.Reset.Add(-r.VorReset)
	vorReset, nacht := !jetzt.Before(start), r.inNacht(jetzt)
	if r.Gleichmaessig > 0 && !vorReset {
		ok, grund, ab := r.gleichmaessig(kurz, lang, jetzt)
		if ok {
			return true, "", time.Time{}
		}
		if !nacht {
			// Wieder möglich, sobald die Linie den Verbrauch einholt, eine Nacht beginnt oder die Restzeit vor dem Reset
			if r.NachtAnteil > 0 && r.NachtVon.After(jetzt) && r.NachtVon.Before(ab) {
				ab = r.NachtVon
			}
			if start.Before(ab) {
				ab = start
			}
			return false, grund, ab
		}
	}
	if !vorReset && !nacht {
		grund := fmt.Sprintf("the %s window resets at %s; leftover work starts %s before", lang.Name, zeitpunkt(lang.Reset), r.VorReset)
		if r.NachtAnteil > 0 && r.NachtVon.After(jetzt) && r.NachtVon.Before(start) {
			return false, grund + ", night work from " + zeitpunkt(r.NachtVon), r.NachtVon
		}
		return false, grund, start
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
	if nacht && !vorReset { // vor dem Reset gilt der Nachtanteil nicht: dann verfiele der Rest ohnehin
		if lang.Prozent >= r.NachtDecke {
			return false, fmt.Sprintf("%.0f %% of the %s window used; night work only below %.0f %%", lang.Prozent, lang.Name, r.NachtDecke), r.naechsteChance(start, jetzt)
		}
		if r.NachtBasis >= 0 && lang.Prozent-r.NachtBasis >= r.NachtAnteil {
			return false, fmt.Sprintf("tonight's share is used (%.0f of %.0f %% of the %s window)", lang.Prozent-r.NachtBasis, r.NachtAnteil, lang.Name), r.naechsteChance(start, jetzt)
		}
	}
	return true, "", time.Time{}
}

// gleichmaessig: liegt der Verbrauch der Woche unter der Linie bis Gleichmaessig Prozent am Reset (plus Vorlauf für eine
// laufende Aufgabe), und ist im kurzen Fenster noch Platz für den Besitzer? Sonst der Grund und ab wann wieder.
func (r restRegeln) gleichmaessig(kurz, lang kontingentFenster, jetzt time.Time) (bool, string, time.Time) {
	if kurz.Name != lang.Name && kurz.Prozent >= r.Sitzung {
		return false, fmt.Sprintf("%.0f %% of the %s window used (%.0f %% of it stays yours)", kurz.Prozent, kurz.Name, 100-r.Sitzung), kurz.Reset
	}
	dauer := time.Duration(lang.Minuten) * time.Minute
	if dauer <= 0 {
		return false, "the length of the " + lang.Name + " window is unknown", lang.Reset
	}
	beginn := lang.Reset.Add(-dauer)
	anteil := min(max(float64(jetzt.Sub(beginn))/float64(dauer), 0), 1)
	linie := r.Gleichmaessig * anteil
	if lang.Prozent < min(linie+r.Vorlauf, r.Gleichmaessig+r.Vorlauf) {
		return true, "", time.Time{}
	}
	ab := lang.Reset
	if lang.Prozent < r.Gleichmaessig {
		ab = beginn.Add(time.Duration(lang.Prozent / r.Gleichmaessig * float64(dauer)))
	}
	return false, fmt.Sprintf("%.0f %% of the %s window used; evenly up to %.0f %% allows %.0f %% by now", lang.Prozent, lang.Name, r.Gleichmaessig, linie), ab
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
			Nacht   *nachtStand         `json:"night"`
		}
		if json.Unmarshal([]byte(zeile), &e) == nil && e.Art == "quota" && len(e.Fenster) > 0 {
			letzter, gefunden = kontingentStand{Zeit: e.Zeit, Fenster: e.Fenster, Nacht: e.Nacht}, true
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
	jetzt := time.Now()
	geaendert := w.stand.zusammenfuehren(neu, jetzt)
	// Erste Antwort in der Nacht: ihr Stand ist der Beginn, an dem sich der Nachtanteil misst
	if r := w.regeln; r != nil && r.NachtBasis < 0 && r.inNacht(jetzt) {
		if f := w.stand.fortgeschrieben(jetzt); len(f) > 0 {
			r.NachtBasis, geaendert = f[len(f)-1].Prozent, true
		}
	}
	if geaendert {
		e := map[string]any{"windows": w.stand.Fenster}
		if r := w.regeln; r != nil && r.NachtBasis >= 0 && !r.NachtVon.IsZero() {
			e["night"] = nachtStand{Beginn: r.NachtVon, Prozent: r.NachtBasis}
		}
		ereignis("quota", e)
	}
}

// erlaubt: nil, wenn die Anfrage durch darf, sonst der Grund.
func (w *kontingentWaechter) erlaubt() error {
	if w == nil || w.regeln == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	r := *w.regeln
	r.Vorlauf = laufendVorlauf // eine laufende Aufgabe endet nicht, weil sie die gleichmäßige Linie knapp überschreitet
	if ok, grund, _ := r.pruefen(w.stand.fortgeschrieben(time.Now()), time.Now()); !ok {
		return fmt.Errorf("%s", grund)
	}
	return nil
}
