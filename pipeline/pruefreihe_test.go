package main

import "testing"

// Die Prüfstraße nimmt nach jeder Prüfung die erste noch nicht versuchte Einreichung der neu abgeholten Liste
// (reihum nach Projekt) und endet, wenn alle versucht sind; eine, deren Start scheitert, kommt nicht erneut dran.
func TestNaechsteOffene(t *testing.T) {
	versucht := map[int64]bool{}
	liste := []einreichung{{EinreichungID: 7}, {EinreichungID: 3}, {EinreichungID: 9}}
	var reihe []int64
	for {
		e, ok := naechsteOffene(liste, versucht)
		if !ok {
			break
		}
		versucht[e.EinreichungID] = true
		reihe = append(reihe, e.EinreichungID)
		if e.EinreichungID == 7 {
			// eine neue Einreichung eines anderen Projekts kommt in der nächsten Abholung nach vorn
			liste = []einreichung{{EinreichungID: 12}, {EinreichungID: 7}, {EinreichungID: 3}, {EinreichungID: 9}}
		}
	}
	if len(reihe) != 4 || reihe[0] != 7 || reihe[1] != 12 || reihe[2] != 3 || reihe[3] != 9 {
		t.Fatalf("order %v, want [7 12 3 9]", reihe)
	}
}
