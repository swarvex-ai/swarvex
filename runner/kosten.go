package main

// Grobe Kostenschätzung für das Tagesbudget mit API-Schlüssel (neVs 09.11.2026: eine Wahl mit Schätzung statt einer
// Zahl). Listenpreise von Anthropic je Million Eingabe-Tokens (Stand 06.10.2026, auch über OpenRouter gleich); für
// andere Anbieter keine Zahl, nur die Rechenregel. Das Budget zählt Cache-Treffer als ein Zehntel; die Schätzung
// rechnet je Budget-Token mit dem 1,5-fachen Eingabepreis (Ausgabe kostet mehr, Cache weniger), also eher nach oben.

import (
	"fmt"
	"strconv"
	"strings"
)

var eingabePreis = map[string]float64{
	"claude-fable-5-1": 10, "claude-fable-5": 10,
	"claude-opus-5-5": 4, "claude-opus-5": 5, "claude-opus-4-8": 5, "claude-opus-4-7": 5, "claude-opus-4-6": 5,
	"claude-sonnet-5-5": 2, "claude-sonnet-5": 2, "claude-sonnet-4-6": 3,
	"claude-haiku-5-5": 0.10, "claude-haiku-4-5": 1,
}

const kostenFaktor = 1.5

// preisFuer: Eingabepreis in Dollar je Million Tokens, auch für „anthropic/claude-sonnet-5.5“ (OpenRouter).
func preisFuer(modell string) (float64, bool) {
	m := strings.ReplaceAll(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(modell)), "anthropic/"), ".", "-")
	p, ok := eingabePreis[m]
	return p, ok
}

func dollar(x float64) string {
	if x < 10 {
		return "$" + strconv.FormatFloat(x, 'f', 2, 64)
	}
	return fmt.Sprintf("$%.0f", x)
}

// kostenText: „about $6.00 a day at most with claude-sonnet-5-5 at Anthropic's list prices“ oder leer (Preis unbekannt).
func kostenText(tokens int64, modell string) string {
	p, ok := preisFuer(modell)
	if !ok || tokens <= 0 {
		return ""
	}
	return fmt.Sprintf("about %s a day at most with %s at Anthropic's list prices", dollar(float64(tokens)/1e6*p*kostenFaktor), modell)
}

// tagesbudgetFragen: Wie viel darf der Agent am Tag nutzen? Mit Schätzung, wo der Preis bekannt ist.
func tagesbudgetFragen(f *frager, modell string) string {
	mit := func(n int64, text string) string {
		if k := kostenText(n, modell); k != "" {
			return text + " (" + k + ")"
		}
		return text
	}
	if _, ok := preisFuer(modell); !ok {
		fmt.Println("  Cost, roughly: tokens ÷ 1,000,000 × your provider's price per million input tokens (cache hits count a tenth).")
	}
	switch f.wahl("How much may your agent use per day? You pay your provider for every token.", []string{
		mit(2_000_000, "2,000,000 tokens a day (recommended)"),
		mit(5_000_000, "5,000,000 tokens a day"),
		"Open end: no daily limit (you pay what your agent uses)",
		"My own number",
	}) {
	case 0:
		return "2000000"
	case 1:
		return "5000000"
	case 2:
		return "0"
	}
	for {
		v := strings.TrimSpace(f.frage("Tokens per day at most (0 = no limit)", "2000000"))
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			return v
		}
		fmt.Println("  Please enter a number.")
	}
}
