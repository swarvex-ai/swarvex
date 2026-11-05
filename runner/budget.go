package main

// swarvex budget: Token-Grenzen anzeigen und ändern, ohne die ganze Einrichtung zu durchlaufen.
// Die Aufgabengrenze steht in den gespeicherten Einstellungen (work.json) und in der Dienst-Einheit,
// die Tagesgrenze nur in der Dienst-Einheit (ExecStart). Beide werden hier gemeinsam angepasst.

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// argWert liefert den Wert eines Flags (--name wert oder --name=wert); der letzte gewinnt wie bei flag.
func argWert(args []string, name string) (string, bool) {
	wert, gefunden := "", false
	for i := 0; i < len(args); i++ {
		n, w, mitGleich := strings.Cut(strings.TrimLeft(args[i], "-"), "=")
		if !strings.HasPrefix(args[i], "-") || n != name {
			continue
		}
		if mitGleich {
			wert, gefunden = w, true
		} else if i+1 < len(args) {
			wert, gefunden = args[i+1], true
			i++
		}
	}
	return wert, gefunden
}

// argSetzen ersetzt ein Flag mit Wert (alle Vorkommen) durch --name wert am Ende.
func argSetzen(args []string, name, wert string) []string {
	r := make([]string, 0, len(args)+2)
	for i := 0; i < len(args); i++ {
		n, _, mitGleich := strings.Cut(strings.TrimLeft(args[i], "-"), "=")
		if strings.HasPrefix(args[i], "-") && n == name {
			if !mitGleich {
				i++ // den Wert gleich mit überspringen
			}
			continue
		}
		r = append(r, args[i])
	}
	return append(r, "--"+name, wert)
}

// einheitArgsLesen: die Argumente nach `work --continuous` aus ExecStart der Dienst-Einheit (Gegenstück zu systemdQuote).
func einheitArgsLesen(einheit string) ([]string, bool) {
	for _, zeile := range strings.Split(einheit, "\n") {
		rest, ok := strings.CutPrefix(zeile, "ExecStart=")
		if !ok {
			continue
		}
		teile := systemdTeilen(rest)
		if len(teile) < 3 || teile[1] != "work" || teile[2] != "--continuous" {
			return nil, false
		}
		return teile[3:], true
	}
	return nil, false
}

func systemdTeilen(s string) []string {
	var teile []string
	var b strings.Builder
	inAnf, hat := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inAnf && c == '\\' && i+1 < len(s):
			i++
			b.WriteByte(s[i])
		case c == '"':
			inAnf, hat = !inAnf, true
		case !inAnf && (c == ' ' || c == '\t'):
			if hat || b.Len() > 0 {
				teile = append(teile, b.String())
				b.Reset()
				hat = false
			}
		case (c == '$' || c == '%') && i+1 < len(s) && s[i+1] == c:
			b.WriteByte(c)
			i++
		default:
			b.WriteByte(c)
		}
	}
	if hat || b.Len() > 0 {
		teile = append(teile, b.String())
	}
	return teile
}

func grenzeText(wert string, gefunden bool, standard string) string {
	if !gefunden {
		wert = standard
	}
	if wert == "0" {
		return "no limit"
	}
	if n, err := strconv.ParseInt(wert, 10, 64); err == nil {
		return tausender(n) + " tokens"
	}
	return wert
}

func budgetBefehl(args []string) {
	fs := flag.NewFlagSet("budget", flag.ExitOnError)
	aufgabe := fs.String("task", "", "tokens per task at most (0 = no limit, otherwise at least 50000)")
	tag := fs.String("daily", "", "tokens per day at most, across all tasks (0 = no limit)")
	medienTag := fs.String("media", "", fmt.Sprintf("media briefs per day at most (1–%d, 0 = no media briefs)", medienHoechstens))
	sofort := fs.Bool("now", false, "restart the service right away, even during a task (the task is handed back)")
	fs.Parse(args)

	gespeichert := gespeicherteArgs()
	einheit, _ := os.ReadFile(dienstDatei())
	dienst, mitDienst := einheitArgsLesen(string(einheit))
	quelle := gespeichert
	if mitDienst {
		quelle = dienst
	}
	if *aufgabe == "" && *tag == "" && *medienTag == "" {
		w, ok := argWert(quelle, "budget-tokens")
		fmt.Println("Per task:", grenzeText(w, ok, "0"))
		if mitDienst {
			w, ok = argWert(dienst, "daily-budget-tokens")
			fmt.Println("Per day: ", grenzeText(w, ok, "0"))
		} else {
			fmt.Println("Per day:  only applies to the service (swarvex setup → as a service)")
		}
		fmt.Printf("Used today: %s tokens (cache hits count a tenth)\n", tausender(verbrauchAmTag(verbrauchsDatei(), time.Now())))
		fmt.Println("Media:   ", medienText(quelle))
		fmt.Println("Change: swarvex budget --daily 2000000 --task 0 --media 10")
		return
	}
	if gespeichert == nil && !mitDienst {
		fehlerAus("No settings yet: run `swarvex setup` first.")
	}
	if *aufgabe != "" {
		if n, err := strconv.Atoi(*aufgabe); err != nil || (n != 0 && n < 50000) {
			fehlerAus("--task: 0 (no limit) or a number of at least 50000")
		}
		if gespeichert != nil {
			gespeichert = argSetzen(gespeichert, "budget-tokens", *aufgabe)
			pruefen(argsSpeichern(gespeichert))
		}
		if mitDienst {
			dienst = argSetzen(dienst, "budget-tokens", *aufgabe)
		}
	}
	if *tag != "" {
		if n, err := strconv.ParseInt(*tag, 10, 64); err != nil || n < 0 {
			fehlerAus("--daily: 0 (no limit) or a positive number")
		}
		if !mitDienst {
			fehlerAus("The daily limit only applies to the service; set one up with `swarvex setup`.")
		}
		dienst = argSetzen(dienst, "daily-budget-tokens", *tag)
	}
	if *medienTag != "" {
		n, err := strconv.Atoi(*medienTag)
		if err != nil || n < 0 || n > medienHoechstens {
			fehlerAus(fmt.Sprintf("--media: 0 (no media briefs) to %d briefs per day", medienHoechstens))
		}
		spec, ok := argWert(quelle, "media")
		if n > 0 && (!ok || len(medienKennungen(spec)) == 0) {
			// noch keine Werkzeuge gewählt: die gefundenen nehmen
			if spec = medienSpec(nutzbareServer(medienServerFinden())); spec == "" {
				fehlerAus("No usable media tools (Meshy, ElevenLabs …) found in your agents' MCP settings.")
			}
		}
		setzen := func(args []string) []string {
			if spec != "" {
				args = argSetzen(args, "media", spec)
			}
			return argSetzen(args, "media-daily", fmt.Sprint(n))
		}
		if gespeichert != nil {
			gespeichert = setzen(gespeichert)
			pruefen(argsSpeichern(gespeichert))
		}
		if mitDienst {
			dienst = setzen(dienst)
		}
	}
	fmt.Println("Saved.")
	if !mitDienst {
		return
	}
	einheitSchreiben(string(einheit), dienst)
	dienstNeuStarten(*sofort, "the new limits")
}

// medienText: was der Runner bei Medien-Aufträgen tut („3 a day with Meshy (Claude Code)“ oder „off“).
func medienText(args []string) string {
	spec, _ := argWert(args, "media")
	tag, ok := argWert(args, "media-daily")
	if !ok {
		tag = fmt.Sprint(medienStandard)
	}
	auswahl := medienAuswahl(spec, medienServerFinden())
	if len(medienKennungen(spec)) == 0 || tag == "0" {
		return "off"
	}
	if len(auswahl) == 0 {
		return "the chosen tools (" + spec + ") are no longer in your agents' settings"
	}
	return fmt.Sprintf("at most %s briefs a day with %s", tag, serverTitel(auswahl))
}

// einheitSchreiben: die Dienst-Einheit mit neuen Argumenten, für dasselbe Programm wie bisher.
func einheitSchreiben(einheit string, dienst []string) {
	programm, err := os.Executable()
	pruefen(err)
	if teile := systemdTeilen(strings.TrimPrefix(zeileMit(einheit, "ExecStart="), "ExecStart=")); len(teile) > 0 {
		programm = teile[0]
	}
	pruefen(os.WriteFile(dienstDatei(), []byte(dienstEinheit(programm, dienst)), 0o644))
	if aus, err := systemctl("daemon-reload"); err != nil {
		fehlerAus("systemctl --user daemon-reload: " + aus)
	}
}

func zeileMit(text, anfang string) string {
	for _, z := range strings.Split(text, "\n") {
		if strings.HasPrefix(z, anfang) {
			return z
		}
	}
	return ""
}
