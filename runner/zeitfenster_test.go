package main

import (
	"testing"
	"time"
)

// 2026-10-26 ist ein Montag.
func zp(tag, h, m int) time.Time { return time.Date(2026, 10, 26+tag, h, m, 0, 0, time.Local) }

func TestZeitfensterWerktagsAbends(t *testing.T) {
	z, err := zeitfensterLesen("mo-fr 18:00-23:00; sa,so 09:00-24:00")
	if err != nil {
		t.Fatal(err)
	}
	faelle := []struct {
		t    time.Time
		offen bool
	}{
		{zp(0, 17, 59), false}, {zp(0, 18, 0), true}, {zp(0, 22, 59), true}, {zp(0, 23, 0), false},
		{zp(4, 20, 0), true},                         // Freitag
		{zp(5, 8, 59), false}, {zp(5, 9, 0), true}, {zp(5, 23, 59), true}, // Samstag
		{zp(6, 12, 0), true}, // Sonntag
	}
	for _, f := range faelle {
		if got := z.offen(f.t); got != f.offen {
			t.Errorf("%s: offen=%v, erwartet %v", f.t.Format("Mon 15:04"), got, f.offen)
		}
	}
}

func TestZeitfensterUeberMitternacht(t *testing.T) {
	z, err := zeitfensterLesen("täglich 22:00-07:00")
	if err != nil {
		t.Fatal(err)
	}
	if !z.offen(zp(0, 23, 30)) || !z.offen(zp(1, 6, 59)) || z.offen(zp(1, 7, 0)) || z.offen(zp(1, 12, 0)) {
		t.Fatal("Fenster über Mitternacht falsch ausgewertet")
	}
	// nur freitags gestartet: Samstag früh noch offen, Samstag abends nicht
	z, _ = zeitfensterLesen("fr 22:00-06:00")
	if !z.offen(zp(5, 5, 0)) || z.offen(zp(5, 23, 0)) || z.offen(zp(0, 1, 0)) {
		t.Fatal("Fenster über Mitternacht am Wochenende falsch")
	}
}

func TestZeitfensterVerbleibendUndNaechster(t *testing.T) {
	z, _ := zeitfensterLesen("mo-fr 18:00-23:00")
	if got := z.verbleibend(zp(0, 22, 30)); got != 30*time.Minute {
		t.Errorf("verbleibend = %v, erwartet 30m", got)
	}
	if got := z.verbleibend(zp(0, 12, 0)); got != 0 {
		t.Errorf("verbleibend außerhalb = %v, erwartet 0", got)
	}
	n, ok := z.naechsterBeginn(zp(4, 23, 30)) // Freitag spät → Montag 18:00
	if !ok || !n.Equal(zp(7, 18, 0)) {
		t.Errorf("nächster Beginn = %v", n)
	}
	// aneinanderstoßende Fenster zählen zusammen
	z, _ = zeitfensterLesen("mo 20:00-24:00; di 00:00-02:00")
	if got := z.verbleibend(zp(0, 23, 0)); got != 3*time.Hour {
		t.Errorf("verbleibend über Fenstergrenze = %v, erwartet 3h", got)
	}
}

func TestZeitfensterLeerUndFehler(t *testing.T) {
	z, err := zeitfensterLesen("")
	if err != nil || !z.offen(zp(2, 3, 0)) {
		t.Fatal("leere Angabe muss immer offen sein")
	}
	for _, kaputt := range []string{"mo 18-23", "xx 10:00-11:00", "mo 25:00-26:00", "mo 10:00-10:00", "mo-fr-sa 10:00-11:00", "mo"} {
		if _, err := zeitfensterLesen(kaputt); err == nil {
			t.Errorf("%q hätte abgelehnt werden müssen", kaputt)
		}
	}
	z, _ = zeitfensterLesen("fr-mo 10:00-12:00") // Bereich über das Wochenende
	if !z.offen(zp(6, 11, 0)) || !z.offen(zp(0, 11, 0)) || z.offen(zp(2, 11, 0)) {
		t.Fatal("Tagesbereich über das Wochenende falsch")
	}
}
