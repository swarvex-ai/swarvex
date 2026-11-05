package main

// swarvex history: die letzten Aufgaben dieses Runners mit Ergebnis, Grund und Verbrauch (lokales Journal
// history.jsonl im Datenordner), dazu der heutige Stand der Aufgabe auf der Plattform.

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type verlaufEintrag struct {
	Zeit     time.Time `json:"time"`
	Aufgabe  int64     `json:"task"`
	Brief    int64     `json:"brief,omitempty"` // Medien-Auftrag (dann ohne Aufgabe)
	Projekt  string    `json:"project,omitempty"`
	Titel    string    `json:"title,omitempty"`
	Ergebnis string    `json:"outcome"` // submitted | returned | uploaded (Medien-Auftrag)
	Grund    string    `json:"reason,omitempty"`
	Tokens   int64     `json:"tokens,omitempty"`
	Budget   int64     `json:"budget_tokens,omitempty"`
	Sekunden int64     `json:"seconds,omitempty"`
}

func verlaufsDatei() string { return filepath.Join(datenOrdner(), "history.jsonl") }

func verlaufMerken(datei string, e verlaufEintrag) {
	os.MkdirAll(filepath.Dir(datei), 0o755)
	f, err := os.OpenFile(datei, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(e)
}

// verlaufLesen liefert die letzten n Einträge, neueste zuerst.
func verlaufLesen(datei string, n int) []verlaufEintrag {
	f, err := os.Open(datei)
	if err != nil {
		return nil
	}
	defer f.Close()
	var alle []verlaufEintrag
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), 1<<20)
	for s.Scan() {
		var e verlaufEintrag
		if json.Unmarshal(s.Bytes(), &e) == nil && (e.Aufgabe > 0 || e.Brief > 0) {
			alle = append(alle, e)
		}
	}
	if len(alle) > n {
		alle = alle[len(alle)-n:]
	}
	for i, j := 0, len(alle)-1; i < j; i, j = i+1, j-1 {
		alle[i], alle[j] = alle[j], alle[i]
	}
	return alle
}

func verlaufEintragAus(id int64, a aufgabe, ergebnis, grund string, verbrauch map[string]float64) verlaufEintrag {
	return verlaufEintrag{Zeit: time.Now(), Aufgabe: id, Projekt: a.Projekt, Titel: a.Titel, Ergebnis: ergebnis, Grund: kuerzen(grund, 400),
		Tokens: tokensAus(verbrauch), Budget: budgetTokensAus(verbrauch), Sekunden: int64(verbrauch["seconds"])}
}

func historyBefehl(args []string) {
	fs := flag.NewFlagSet("history", flag.ExitOnError)
	anzahl := fs.Int("n", 15, "how many tasks")
	ohneStand := fs.Bool("offline", false, "do not ask the platform for the current state")
	fs.Parse(args)
	eintraege := verlaufLesen(verlaufsDatei(), *anzahl)
	if len(eintraege) == 0 {
		fmt.Println("No tasks yet (the history starts with this runner version).")
		return
	}
	stand := map[int64]string{}
	if !*ohneStand {
		stand = aufgabenStand(eintraege)
	}
	for _, e := range eintraege {
		nummer := fmt.Sprintf("#%-4d", e.Aufgabe)
		if e.Brief > 0 {
			nummer = fmt.Sprintf("media brief %d:", e.Brief)
		}
		fmt.Printf("%s  %s %s\n", e.Zeit.Local().Format("2006-01-02 15:04"), nummer, kuerzen(e.Titel, 60))
		was := map[string]string{"submitted": "✓ submitted", "uploaded": "✓ uploaded", "returned": "✗ handed back"}[e.Ergebnis]
		if was == "" {
			was = e.Ergebnis
		}
		details := []string{was}
		if e.Budget > 0 || e.Tokens > 0 {
			details = append(details, fmt.Sprintf("%s tokens (%s counted)", tausender(e.Tokens), tausender(e.Budget)))
		}
		if e.Sekunden > 0 {
			details = append(details, (time.Duration(e.Sekunden)*time.Second).String()+" model time")
		}
		if s, ok := stand[e.Aufgabe]; ok {
			details = append(details, "now: "+s)
		}
		fmt.Println("        " + strings.Join(details, " · "))
		if e.Grund != "" {
			fmt.Println("        reason: " + e.Grund)
		}
	}
}

// aufgabenStand fragt den heutigen Zustand der Aufgaben bei der Plattform ab (ohne Anmeldung: leer).
func aufgabenStand(eintraege []verlaufEintrag) map[int64]string {
	stand := map[int64]string{}
	z, err := zugangLaden()
	if err != nil {
		return stand
	}
	token, err := zugriffstoken(z)
	if err != nil {
		return stand
	}
	m := &mcpClient{basis: z.Plattform, token: token, http: httpClient(z.CAPfad)}
	if m.initialisieren() != nil {
		return stand
	}
	for _, e := range eintraege {
		if _, schon := stand[e.Aufgabe]; schon || e.Aufgabe == 0 {
			continue
		}
		var a struct {
			Zustand string `json:"state"`
		}
		if m.werkzeug("task_details", map[string]any{"task_id": e.Aufgabe}, &a) == nil && a.Zustand != "" {
			stand[e.Aufgabe] = strings.ReplaceAll(a.Zustand, "_", " ")
		}
	}
	return stand
}

func tausender(n int64) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
