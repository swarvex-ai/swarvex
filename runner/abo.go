package main

// Faire Nutzung eines Abos (Claude Pro/Max, ChatGPT Plus/Pro): Das Setup erkennt die Stufe (Codex: aus der Anmeldung;
// Claude: eine Frage) und schlägt vor, was der Agent davon nutzen darf. Grundlage ist der Restkontingent-Modus
// (kontingent.go): nur, was vor dem Wochen-Reset verfallen würde, dazu bei großen Abos ein Nachtanteil. Alles in
// Prozent der echten Fenster, die der Anbieter mit jeder Antwort meldet; jeder Wert lässt sich frei einstellen.

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Die Nacht: genau ein 5-Stunden-Fenster, das bis zum Morgen abgelaufen ist.
const (
	fairNacht   = "daily 01:00-06:00"
	fairDecke   = 80.0 // keine Nachtarbeit mehr, sobald so viel der Woche verbraucht ist
	fairReserve = 10.0 // bleibt immer dem Besitzer
)

type aboPlan struct {
	Name        string
	NachtAnteil float64 // Prozent der Woche je Nacht; 0: nur der Rest vor dem Reset
	Firma       bool    // Team, Business, Enterprise: nie von selbst
	ZuKlein     bool    // ChatGPT Free: zu wenig für Aufgaben
}

var claudePlaene = []aboPlan{
	{Name: "Claude Pro"},
	{Name: "Claude Max 5x", NachtAnteil: 5},
	{Name: "Claude Max 20x", NachtAnteil: 10},
	{Name: "Claude Team or Enterprise (work account)", Firma: true},
}

var chatgptPlaene = []aboPlan{
	{Name: "ChatGPT Plus"},
	{Name: "ChatGPT Pro", NachtAnteil: 10},
	{Name: "ChatGPT Team, Business or Enterprise (work account)", Firma: true},
}

// chatgptPlan: die Stufe aus der Codex-Anmeldung (chatgpt_plan_type); ok=false, wenn sie unbekannt ist.
func chatgptPlan(typ string) (aboPlan, bool) {
	switch strings.ToLower(typ) {
	case "plus":
		return chatgptPlaene[0], true
	case "go":
		return aboPlan{Name: "ChatGPT Go"}, true
	case "pro":
		return chatgptPlaene[1], true
	case "team", "business", "enterprise", "edu":
		return aboPlan{Name: "ChatGPT " + strings.ToUpper(typ[:1]) + typ[1:] + " (work account)", Firma: true}, true
	case "free":
		return aboPlan{Name: "ChatGPT Free", ZuKlein: true}, true
	}
	return aboPlan{}, false
}

// codexPlanLesen: chatgpt_plan_type aus dem id_token einer Codex-Anmeldung (nur lokal gelesen, nichts wird gesendet).
func codexPlanLesen(pfad string) string {
	roh, err := os.ReadFile(pfad)
	if err != nil {
		return ""
	}
	var datei struct {
		Tokens struct {
			ID string `json:"id_token"`
		} `json:"tokens"`
	}
	if json.Unmarshal(roh, &datei) != nil {
		return ""
	}
	if c, ok := jwtInhalt(datei.Tokens.ID)["https://api.openai.com/auth"].(map[string]any); ok {
		plan, _ := c["chatgpt_plan_type"].(string)
		return plan
	}
	return ""
}

// aboErmitteln: Codex liest die Stufe aus der Anmeldung, sonst fragt das Setup einmal.
func aboErmitteln(f *frager, w modellWahl) aboPlan {
	if w.Agent == "codex" {
		if p, ok := chatgptPlan(codexPlanLesen(w.Key)); ok {
			fmt.Println("Subscription: " + p.Name + " (from your Codex sign-in)")
			return p
		}
		return chatgptPlaene[f.wahl("Which ChatGPT plan do you have?", planTitel(chatgptPlaene))]
	}
	return claudePlaene[f.wahl("Which Claude plan do you have? (it decides what your agent may use)", planTitel(claudePlaene))]
}

func planTitel(p []aboPlan) []string {
	t := make([]string, len(p))
	for i, x := range p {
		t[i] = x.Name
	}
	return t
}

// fairMoeglich: gibt es für diese Stufe einen Vorschlag, der ohne Rückfrage gilt?
func (p aboPlan) fairMoeglich() bool { return !p.Firma && !p.ZuKlein }

func (p aboPlan) fairText() string {
	if p.NachtAnteil > 0 {
		return fmt.Sprintf("nights 01:00–06:00 up to %g %% of the week (not above %g %% used), plus what would expire before the weekly reset; %g %% of the week always stays yours",
			p.NachtAnteil, fairDecke, fairReserve)
	}
	return fmt.Sprintf("only what would expire: the last 24 hours before the weekly reset; %g %% of the week always stays yours", fairReserve)
}

func (p aboPlan) fairArgs() []string {
	a := []string{"--leftover"}
	if p.NachtAnteil > 0 {
		a = append(a, "--leftover-night", fairNacht, "--leftover-night-share", fmt.Sprint(p.NachtAnteil), "--leftover-night-max", fmt.Sprint(fairDecke))
	}
	return a
}

// aboNutzungFragen: unter „Anpassen“: fair, nur der Rest, ganz oder eigene Werte (alles bis 100 % möglich).
func aboNutzungFragen(f *frager, p aboPlan) []string {
	var optionen []string
	var ergebnis []func() []string
	if p.fairMoeglich() {
		optionen = append(optionen, "Fair for "+p.Name+" (recommended): "+p.fairText())
		ergebnis = append(ergebnis, p.fairArgs)
	}
	optionen = append(optionen, "Only what would expire before the weekly reset ("+fmt.Sprint(fairReserve)+" % stays yours)",
		"Fully, without limits (your own work can run short)", "My own values")
	ergebnis = append(ergebnis, func() []string { return []string{"--leftover"} }, func() []string { return nil }, func() []string { return eigeneWerteFragen(f) })
	if p.Firma {
		fmt.Println("  " + p.Name + ": check that your organisation allows this; swarvex never uses work accounts on its own.")
	}
	if p.ZuKlein {
		fmt.Println("  " + p.Name + " has too little quota for tasks; a local model or an API key works better.")
	}
	return ergebnis[f.wahl("How may your agent use your subscription?", optionen)]()
}

func eigeneWerteFragen(f *frager) []string {
	for {
		zahl := func(text, vorschlag string) float64 {
			for {
				if v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(f.frage(text, vorschlag)), "%"), 64); err == nil {
					return v
				}
				fmt.Println("  Please enter a number.")
			}
		}
		r := restRegeln{NachtDecke: fairDecke}
		r.Reserve = zahl("Percent of the week that always stays yours (0 = none)", fmt.Sprint(fairReserve))
		vor, err := time.ParseDuration(f.frage("Use what would expire from this long before the weekly reset", "24h"))
		if err != nil {
			fmt.Println("  " + err.Error())
			continue
		}
		r.VorReset = vor
		r.Sitzung = zahl("Never fill the 5-hour window beyond this percent", "80")
		r.NachtText = f.frage("Also at night, in these time windows (empty = no)", fairNacht)
		if r.NachtText != "" {
			r.NachtAnteil = zahl("Percent of the week per night (100 = no limit)", "10")
			r.NachtDecke = zahl("No night work once this percent of the week is used", fmt.Sprint(fairDecke))
		}
		if err := r.gueltig(); err != nil {
			fmt.Println("  " + err.Error())
			continue
		}
		a := []string{"--leftover", "--leftover-reserve", fmt.Sprint(r.Reserve), "--leftover-before-reset", r.VorReset.String(), "--leftover-session-max", fmt.Sprint(r.Sitzung)}
		if r.NachtText != "" && r.NachtAnteil > 0 {
			a = append(a, "--leftover-night", r.NachtText, "--leftover-night-share", fmt.Sprint(r.NachtAnteil), "--leftover-night-max", fmt.Sprint(r.NachtDecke))
		}
		return a
	}
}
