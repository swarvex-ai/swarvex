package main

// Neue Einstellungen: Bringt eine Version eine neue Frage in die Einrichtung, stellt `swarvex update` danach nur
// diese (über `swarvex setup --new`), nie die ganze Einrichtung noch einmal. Was schon gefragt wurde, steht in
// asked.json; was ohne Terminal (automatisches Update) offen bleibt, gilt mit dem Standard und erscheint als
// Hinweis im Log und in `swarvex doctor`.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// einrichtungStand: gespeicherte Angaben (work.json) und die der Dienst-Einheit, so wie die Fragen sie brauchen.
type einrichtungStand struct {
	work, dienst       []string
	mitDienst          bool
	workNeu, dienstNeu bool
	einheit            string
}

type neueFrage struct {
	Name, Seit, Titel string
	offen             func(e *einrichtungStand) bool // betrifft diese Einrichtung und ist noch nicht gesetzt
	fragen            func(f *frager, e *einrichtungStand)
}

const autoUpdateFrage = "Install runner updates automatically (at most once a day, between tasks, signature checked)?"

// neueFragen: jede Frage, die nach v0.9.0 in die Einrichtung kommt, gehört auch hierher (mit der Version).
var neueFragen = []neueFrage{
	{Name: "auto-update", Seit: "0.9.0", Titel: "automatic updates",
		offen: func(e *einrichtungStand) bool { return e.mitDienst && !hatFlag(e.dienst, "auto-update") },
		fragen: func(f *frager, e *einrichtungStand) {
			if f.jaNein(autoUpdateFrage, true) {
				e.dienst, e.dienstNeu = append(e.dienst, "--auto-update"), true
			}
		}},
	{Name: "fair-use", Seit: "0.9.4", Titel: "fair night use of your subscription",
		offen: func(e *einrichtungStand) bool {
			return e.mitDienst && hatFlag(e.dienst, "leftover") && !hatFlag(e.dienst, "leftover-night-share")
		},
		fragen: func(f *frager, e *einrichtungStand) {
			agent, _ := argWert(e.dienst, "agent")
			key, _ := argWert(e.dienst, "model-key-file")
			p := aboErmitteln(f, modellWahl{Agent: agent, Key: key})
			if p.NachtAnteil <= 0 {
				fmt.Println("  " + p.Name + ": your agent keeps using only what would expire before the weekly reset.")
				return
			}
			if f.jaNein(fmt.Sprintf("%s allows more: nights 01:00–06:00 up to %g %% of the week (not above %g %% used). Use it?", p.Name, p.NachtAnteil, fairDecke), true) {
				e.dienst, e.dienstNeu = append(e.dienst, "--leftover-night", fairNacht, "--leftover-night-share", fmt.Sprint(p.NachtAnteil), "--leftover-night-max", fmt.Sprint(fairDecke)), true
			}
		}},
	{Name: "task-budget", Seit: "0.9.8", Titel: "no token limit per task",
		offen: func(e *einrichtungStand) bool { return aufgabenGrenze(e.work) > 0 || aufgabenGrenze(e.dienst) > 0 },
		fragen: func(f *frager, e *einrichtungStand) {
			tag, _ := argWert(e.dienst, "daily-budget-tokens")
			deckel := "your daily budget (" + grenzeText(tag, tag != "", "0") + ")"
			if hatFlag(e.dienst, "leftover") {
				deckel = "leftover mode (only what your subscription allows)"
			}
			n := max(aufgabenGrenze(e.work), aufgabenGrenze(e.dienst))
			fmt.Printf("  Your runner stops a task at %s tokens. Big tasks then end without a result, although %s already caps every task.\n", tausender(int64(n)), deckel)
			if !f.jaNein("Drop the limit per task?", true) {
				return
			}
			if aufgabenGrenze(e.work) > 0 {
				e.work, e.workNeu = argSetzen(e.work, "budget-tokens", "0"), true
			}
			if aufgabenGrenze(e.dienst) > 0 {
				e.dienst, e.dienstNeu = argSetzen(e.dienst, "budget-tokens", "0"), true
			}
			if e.mitDienst && !hatFlag(e.dienst, "leftover") && (tag == "" || tag == "0") {
				fmt.Println("  Without a daily budget nothing caps a task any more; with a paid API key set one: swarvex budget --daily 2000000")
			}
		}},
	{Name: "media", Seit: "0.10.0", Titel: "media tools for projects' briefs",
		offen: func(e *einrichtungStand) bool {
			return !hatFlag(e.work, "media") && !hatFlag(e.dienst, "media") && len(nutzbareServer(medienServerFinden())) > 0
		},
		fragen: func(f *frager, e *einrichtungStand) {
			mehr := medienFragen(f, medienServerFinden())
			if len(mehr) == 0 {
				return
			}
			if e.work != nil {
				e.work, e.workNeu = append(e.work, mehr...), true
			}
			if e.mitDienst {
				e.dienst, e.dienstNeu = append(e.dienst, mehr...), true
			}
		}},
}

// aufgabenGrenze: die gespeicherte Grenze je Aufgabe (0, wenn keine).
func aufgabenGrenze(args []string) int {
	w, _ := argWert(args, "budget-tokens")
	n, _ := strconv.Atoi(w)
	return n
}

func gefragtDatei() string { return filepath.Join(konfigOrdner(), "asked.json") }

func gefragtLesen(datei string) map[string]bool {
	var g struct {
		Gefragt []string `json:"asked"`
	}
	json.Unmarshal([]byte(lesen(datei)), &g)
	r := map[string]bool{}
	for _, n := range g.Gefragt {
		r[n] = true
	}
	return r
}

func gefragtMerken(datei string, namen ...string) {
	g := gefragtLesen(datei)
	for _, n := range namen {
		g[n] = true
	}
	liste := make([]string, 0, len(g))
	for _, f := range neueFragen { // in fester Reihenfolge, Unbekanntes hinten
		if g[f.Name] {
			liste = append(liste, f.Name)
			delete(g, f.Name)
		}
	}
	for n := range g {
		liste = append(liste, n)
	}
	roh, _ := json.MarshalIndent(map[string]any{"asked": liste}, "", "  ")
	os.MkdirAll(filepath.Dir(datei), 0o700)
	os.WriteFile(datei, roh, 0o600)
}

func einrichtungLaden() *einrichtungStand {
	e := &einrichtungStand{work: gespeicherteArgs()}
	einheit, err := os.ReadFile(dienstDatei())
	if err == nil {
		e.einheit = string(einheit)
		e.dienst, e.mitDienst = einheitArgsLesen(e.einheit)
	}
	return e
}

// offeneFragen: neue Fragen, die diese Einrichtung betreffen und noch nie gestellt wurden.
func offeneFragen(e *einrichtungStand, gefragt map[string]bool) []neueFrage {
	var r []neueFrage
	for _, f := range neueFragen {
		if !gefragt[f.Name] && f.offen(e) {
			r = append(r, f)
		}
	}
	return r
}

// offeneTitel: für Hinweise („automatic updates, …“); leer, wenn nichts offen ist.
func offeneTitel() string {
	offen := offeneFragen(einrichtungLaden(), gefragtLesen(gefragtDatei()))
	titel := make([]string, len(offen))
	for i, f := range offen {
		titel[i] = f.Titel
	}
	return strings.Join(titel, ", ")
}

// neueEinstellungen: swarvex setup --new. still: von `swarvex update` aus, ohne Ausgabe, wenn nichts offen ist.
func neueEinstellungen(still bool) {
	e := einrichtungLaden()
	offen := offeneFragen(e, gefragtLesen(gefragtDatei()))
	if len(offen) == 0 {
		if !still {
			fmt.Println("No new settings: everything is set up (all questions again: swarvex setup).")
		}
		return
	}
	f, ok := neuerFrager()
	if !ok {
		fmt.Println("New settings to choose: " + offeneTitel() + ". Run in a terminal: swarvex setup --new")
		return
	}
	fmt.Println("\nNew in this version, one question each (everything else stays as it is):")
	namen := make([]string, 0, len(offen))
	for _, q := range offen {
		q.fragen(f, e)
		namen = append(namen, q.Name)
	}
	if e.workNeu {
		pruefen(argsSpeichern(e.work))
	}
	if e.dienstNeu {
		einheitSchreiben(e.einheit, e.dienst)
	}
	gefragtMerken(gefragtDatei(), namen...)
	fmt.Println("Saved.")
	if e.dienstNeu && !still { // von update aus startet update den Dienst selbst neu
		dienstNeuStarten(false, "the new settings")
	}
}

func setupBefehl(args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	nurNeue := fs.Bool("new", false, "only ask what is new since your last setup (swarvex update does this by itself)")
	still := fs.Bool("quiet", false, "with --new: say nothing if there is nothing new")
	fs.Parse(args)
	if *nurNeue {
		neueEinstellungen(*still)
		return
	}
	setupDialog(true)
}
