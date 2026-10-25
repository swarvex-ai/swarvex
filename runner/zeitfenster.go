package main

// Zeitfenster: wann der Runner arbeiten darf. Der Beitragende legt sie frei fest, zum Beispiel
//
//	"mo-fr 18:00-23:00; sa,so 09:00-24:00"
//	"täglich 22:00-07:00"            (über Mitternacht: endet am Folgetag)
//	"mo,mi,fr 12:00-13:30; so 00:00-24:00"
//
// Tage: mo di mi do fr sa so (auch mon tue wed thu fri sat sun), Bereiche mit "-", Listen mit ",",
// "täglich"/"taeglich"/"daily"/"*" für alle Tage. Uhrzeiten HH:MM in lokaler Zeit, 24:00 ist erlaubt.
// Die Grenzen setzt der Runner selbst durch; die Plattform kann sie nicht erweitern (konzept.md §5a).

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type fenster struct {
	tage     [7]bool // Index = time.Weekday (So = 0)
	von, bis int     // Minuten seit Mitternacht; bis <= von heißt: über Mitternacht
}

type zeitfenster []fenster

var tagNamen = map[string]time.Weekday{
	"so": time.Sunday, "mo": time.Monday, "di": time.Tuesday, "mi": time.Wednesday, "do": time.Thursday, "fr": time.Friday, "sa": time.Saturday,
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// zeitfensterLesen liest eine Angabe wie "mo-fr 18:00-23:00; sa,so 09:00-24:00". Leer heißt: immer.
func zeitfensterLesen(text string) (zeitfenster, error) {
	var z zeitfenster
	for _, teil := range strings.Split(text, ";") {
		teil = strings.TrimSpace(teil)
		if teil == "" {
			continue
		}
		felder := strings.Fields(teil)
		if len(felder) != 2 {
			return nil, fmt.Errorf("Zeitfenster %q: erwartet „Tage Von-Bis“, z. B. „mo-fr 18:00-23:00“", teil)
		}
		tage, err := tageLesen(felder[0])
		if err != nil {
			return nil, err
		}
		zeiten := strings.Split(felder[1], "-")
		if len(zeiten) != 2 {
			return nil, fmt.Errorf("Zeitfenster %q: Uhrzeit als Von-Bis angeben, z. B. 18:00-23:00", teil)
		}
		von, err := uhrzeitLesen(zeiten[0])
		if err != nil {
			return nil, err
		}
		bis, err := uhrzeitLesen(zeiten[1])
		if err != nil {
			return nil, err
		}
		if von == bis && von != 0 {
			return nil, fmt.Errorf("Zeitfenster %q: Beginn und Ende sind gleich", teil)
		}
		if von == 0 && bis == 0 {
			bis = 24 * 60 // „00:00-00:00“ heißt ganzer Tag
		}
		z = append(z, fenster{tage: tage, von: von, bis: bis})
	}
	return z, nil
}

func tageLesen(text string) ([7]bool, error) {
	var tage [7]bool
	switch strings.ToLower(text) {
	case "täglich", "taeglich", "daily", "*":
		for i := range tage {
			tage[i] = true
		}
		return tage, nil
	}
	for _, stueck := range strings.Split(strings.ToLower(text), ",") {
		grenzen := strings.Split(stueck, "-")
		a, ok := tagNamen[grenzen[0]]
		if !ok {
			return tage, fmt.Errorf("unbekannter Tag %q (mo di mi do fr sa so)", grenzen[0])
		}
		b := a
		if len(grenzen) == 2 {
			if b, ok = tagNamen[grenzen[1]]; !ok {
				return tage, fmt.Errorf("unbekannter Tag %q (mo di mi do fr sa so)", grenzen[1])
			}
		} else if len(grenzen) > 2 {
			return tage, fmt.Errorf("Tagesbereich %q: höchstens ein „-“", stueck)
		}
		// Bereich in Wochenrichtung, auch über das Wochenende hinweg (z. B. fr-mo)
		for d := a; ; d = (d + 1) % 7 {
			tage[d] = true
			if d == b {
				break
			}
		}
	}
	return tage, nil
}

func uhrzeitLesen(text string) (int, error) {
	t := strings.Split(text, ":")
	if len(t) != 2 {
		return 0, fmt.Errorf("Uhrzeit %q: als HH:MM angeben", text)
	}
	h, err1 := strconv.Atoi(t[0])
	m, err2 := strconv.Atoi(t[1])
	if err1 != nil || err2 != nil || h < 0 || m < 0 || m > 59 || h > 24 || (h == 24 && m != 0) {
		return 0, fmt.Errorf("Uhrzeit %q ist ungültig", text)
	}
	return h*60 + m, nil
}

// offen sagt, ob zum Zeitpunkt t gearbeitet werden darf. Ohne Fenster ist immer offen.
func (z zeitfenster) offen(t time.Time) bool {
	if len(z) == 0 {
		return true
	}
	min := t.Hour()*60 + t.Minute()
	tag := t.Weekday()
	gestern := (tag + 6) % 7
	for _, f := range z {
		if f.bis > f.von {
			if f.tage[tag] && min >= f.von && min < f.bis {
				return true
			}
		} else { // über Mitternacht: Beginn am Starttag, Ende am Folgetag
			if (f.tage[tag] && min >= f.von) || (f.tage[gestern] && min < f.bis) {
				return true
			}
		}
	}
	return false
}

// verbleibend liefert, wie lange das gerade offene Fenster noch dauert (aneinanderstoßende Fenster
// zählen zusammen). Ist kein Fenster offen, ist das Ergebnis 0. Ohne Fenster: eine Woche.
func (z zeitfenster) verbleibend(t time.Time) time.Duration {
	if len(z) == 0 {
		return 7 * 24 * time.Hour
	}
	t = t.Truncate(time.Minute)
	for d := time.Duration(0); d <= 8*24*time.Hour; d += time.Minute {
		if !z.offen(t.Add(d)) {
			return d
		}
	}
	return 8 * 24 * time.Hour // immer offen
}

// naechsterBeginn liefert den nächsten Zeitpunkt, an dem ein Fenster öffnet (t selbst, wenn offen).
func (z zeitfenster) naechsterBeginn(t time.Time) (time.Time, bool) {
	if z.offen(t) {
		return t, true
	}
	t = t.Truncate(time.Minute)
	for d := time.Minute; d <= 8*24*time.Hour; d += time.Minute {
		if z.offen(t.Add(d)) {
			return t.Add(d), true
		}
	}
	return time.Time{}, false
}

// zeitpunkt schreibt einen Zeitpunkt kurz und deutsch, z. B. „Sa 31.10. 03:00“.
func zeitpunkt(t time.Time) string {
	return [...]string{"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"}[t.Weekday()] + " " + t.Format("02.01. 15:04")
}
