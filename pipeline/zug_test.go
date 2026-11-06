package main

import (
	"fmt"
	"testing"
)

// Ersatzbahn: ein Beitrag ist „kaputt“, wenn er in kaputt steht; der Zug ist rot, sobald einer dabei ist.
// main wandert mit jedem Push weiter, Konflikte gibt es für die Beiträge in konflikt.
type ersatzBahn struct {
	kaputt, konflikt map[int64]bool
	kopf             int
	gemergt, roete   []int64
	tests            int
	nichtTestbar     bool
}

func (b *ersatzBahn) bauen(zug []einreichung) (*zugStand, error) {
	s := &zugStand{kopf: fmt.Sprint(b.kopf)}
	for _, e := range zug {
		if b.konflikt[e.EinreichungID] {
			continue
		}
		s.dabei = append(s.dabei, e)
		s.commits = append(s.commits, fmt.Sprintf("c%d", e.EinreichungID))
	}
	return s, nil
}

func (b *ersatzBahn) testen(s *zugStand) (zugErgebnis, string) {
	b.tests++
	if b.nichtTestbar {
		return zugFehler, "docker down"
	}
	for _, e := range s.dabei {
		if b.kaputt[e.EinreichungID] {
			return zugRot, "FAIL test"
		}
	}
	return zugGruen, ""
}

func (b *ersatzBahn) schieben(s *zugStand) error { b.kopf++; return nil }
func (b *ersatzBahn) gemerged(e einreichung, commit string) {
	b.gemergt = append(b.gemergt, e.EinreichungID)
}
func (b *ersatzBahn) rot(e einreichung, kopf, bericht string) {
	b.roete = append(b.roete, e.EinreichungID)
}
func (b *ersatzBahn) aufraeumen(s *zugStand) {}

func zugAus(ids ...int64) []einreichung {
	var z []einreichung
	for _, id := range ids {
		z = append(z, einreichung{EinreichungID: id})
	}
	return z
}

func TestZugFahren(t *testing.T) {
	for _, f := range []struct {
		name           string
		zug            []int64
		kaputt, konfl  []int64
		gemergt, roete string
		tests          int
		nichtTestbar   bool
	}{
		{name: "alles grün: ein Testlauf", zug: []int64{1, 2, 3, 4}, gemergt: "[1 2 3 4]", roete: "[]", tests: 1},
		{name: "einer kaputt: halbieren bis zu ihm", zug: []int64{1, 2, 3, 4}, kaputt: []int64{3}, gemergt: "[1 2 4]", roete: "[3]", tests: 5},
		{name: "allein kaputt", zug: []int64{7}, kaputt: []int64{7}, gemergt: "[]", roete: "[7]", tests: 1},
		{name: "Konflikt fährt nicht mit", zug: []int64{1, 2, 3}, konfl: []int64{2}, gemergt: "[1 3]", roete: "[]", tests: 1},
		{name: "nicht testbar: nichts passiert", zug: []int64{1, 2}, gemergt: "[]", roete: "[]", tests: 1, nichtTestbar: true},
	} {
		b := &ersatzBahn{kaputt: map[int64]bool{}, konflikt: map[int64]bool{}, nichtTestbar: f.nichtTestbar}
		for _, id := range f.kaputt {
			b.kaputt[id] = true
		}
		for _, id := range f.konfl {
			b.konflikt[id] = true
		}
		zugFahren(b, zugAus(f.zug...))
		if g, r := fmt.Sprint(b.gemergt), fmt.Sprint(b.roete); g != f.gemergt || r != f.roete || b.tests != f.tests {
			t.Errorf("%s: merged %s, red %s, %d test runs", f.name, g, r, b.tests)
		}
	}
}

func TestOhne(t *testing.T) {
	if got := fmt.Sprint(ohne([]string{"FAIL a", "FAIL b"}, []string{"FAIL b", "FAIL c"})); got != "[FAIL a]" {
		t.Fatal(got)
	}
	if !teilmenge([]string{"FAIL b"}, []string{"FAIL b", "FAIL c"}) || teilmenge([]string{"FAIL a"}, []string{"FAIL b"}) {
		t.Fatal("teilmenge")
	}
}
