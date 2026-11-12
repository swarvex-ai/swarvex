package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestZugAntwortLesen(t *testing.T) {
	for _, f := range []struct {
		roh      string
		zuGross  bool
		ergebnis zugErgebnis
		fehler   bool
	}{
		{`{"result":0,"report":"ok"}`, false, zugGruen, false},
		{`{"result":1,"report":"FAIL"}`, false, zugRot, false},
		{`{"result":2,"report":"docker"}`, false, zugFehler, false},
		// Ohne Ergebnis wäre es 0, also grün: muss ein Fehler sein
		{`{}`, false, 0, true},
		{`{"report":"all fine"}`, false, 0, true},
		{`{"result":7}`, false, 0, true},
		{`{"result":-1}`, false, 0, true},
		{`kein json`, false, 0, true},
		{``, false, 0, true},
		{`{"result":0}`, true, 0, true},
	} {
		a, err := zugAntwortLesen([]byte(f.roh), f.zuGross)
		if (err != nil) != f.fehler || (!f.fehler && a.Ergebnis != f.ergebnis) {
			t.Errorf("%q (too large %v): result %d, err %v", f.roh, f.zuGross, a.Ergebnis, err)
		}
	}
	lang := strings.Repeat("x", zugBerichtMax+10) + "ENDE"
	a, err := zugAntwortLesen([]byte(`{"result":1,"report":"`+lang+`"}`), false)
	if err != nil || len(a.Bericht) != zugBerichtMax || !strings.HasSuffix(a.Bericht, "ENDE") {
		t.Errorf("report not cut from the front: %d %v", len(a.Bericht), err)
	}
}

func TestBegrenzt(t *testing.T) {
	w := &begrenzt{max: 5}
	w.Write([]byte("abc"))
	w.Write([]byte("defg"))
	if w.b.String() != "abcde" || !w.voll {
		t.Errorf("%q voll=%v", w.b.String(), w.voll)
	}
}

// Der Prüfserver antwortet immer mit einer lesbaren Zeile; Unsinn heißt „nicht testbar“, nie grün.
func TestZugTestAnnehmen(t *testing.T) {
	arbeit = t.TempDir()
	unbekannt := func(string) (einreichung, error) { return einreichung{}, errors.New("project x unknown") }
	gefragt := false
	merken := func(s string) (einreichung, error) { gefragt = true; return unbekannt(s) }
	c := strings.Repeat("a", 40)
	fall := func(name, eingabe string) zugAntwort {
		t.Helper()
		var aus bytes.Buffer
		zugTestAnnehmen(strings.NewReader(eingabe), &aus, merken)
		a, err := zugAntwortLesen(aus.Bytes(), false)
		if err != nil {
			t.Fatalf("%s: unreadable answer %q: %v", name, aus.String(), err)
		}
		return a
	}
	if a := fall("probe", `{"probe":true}`+"\n"); a.Ergebnis != zugGruen || !strings.Contains(a.Bericht, "ready") {
		t.Errorf("probe: %+v", a)
	}
	for name, eingabe := range map[string]string{
		"empty":           "",
		"no json":         "hallo\n",
		"bad slug":        `{"slug":"../etc","head":"` + c + `","tip":"` + c + `","contributions":1}` + "\nBUNDLE",
		"bad commit":      `{"slug":"formwerk","head":"main","tip":"` + c + `","contributions":1}` + "\nBUNDLE",
		"no contribution": `{"slug":"formwerk","head":"` + c + `","tip":"` + c + `","contributions":0}` + "\nBUNDLE",
		"too many":        `{"slug":"formwerk","head":"` + c + `","tip":"` + c + `","contributions":99}` + "\nBUNDLE",
		"no bundle":       `{"slug":"formwerk","head":"` + c + `","tip":"` + c + `","contributions":1}` + "\n",
	} {
		if a := fall(name, eingabe); a.Ergebnis != zugFehler {
			t.Errorf("%s: %+v", name, a)
		}
	}
	if gefragt {
		t.Error("an invalid request reached the project lookup")
	}
	// Gültiger Auftrag, unbekanntes Projekt: nicht testbar
	if a := fall("unknown", `{"slug":"formwerk","head":"`+c+`","tip":"`+c+`","contributions":1}`+"\nBUNDLE"); a.Ergebnis != zugFehler || !strings.Contains(a.Bericht, "unknown") {
		t.Errorf("unknown project: %+v", a)
	}
}

// zugAnfrage und zugTestAnnehmen sprechen dasselbe Format.
func TestZugAnfrageRundweg(t *testing.T) {
	arbeit = t.TempDir()
	ein, err := zugAnfrage(zugAuftrag{Probe: true}, "")
	if err != nil {
		t.Fatal(err)
	}
	var aus bytes.Buffer
	zugTestAnnehmen(ein, &aus, nil)
	a, err := zugAntwortLesen(aus.Bytes(), false)
	if err != nil || a.Ergebnis != zugGruen {
		t.Errorf("%+v %v", a, err)
	}
}
