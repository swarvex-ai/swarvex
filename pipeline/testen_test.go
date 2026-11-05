package main

import (
	"errors"
	"strings"
	"testing"
)

func TestTestenZweiterLauf(t *testing.T) {
	for _, f := range []struct {
		name     string
		laeufe   []error
		ok       bool
		anzahl   int
		enthaelt string
	}{
		{"gleich gruen", []error{nil}, true, 1, "=== Test\n"},
		{"wackelig", []error{errors.New("1"), nil}, true, 2, "second run: passed"},
		{"wirklich kaputt", []error{errors.New("1"), errors.New("1")}, false, 2, "failed again"},
	} {
		n := 0
		var b strings.Builder
		ok := testen(&b, func() (string, error) { n++; return "lauf", f.laeufe[n-1] })
		if ok != f.ok || n != f.anzahl || !strings.Contains(b.String(), f.enthaelt) {
			t.Errorf("%s: ok=%v runs=%d report=%q", f.name, ok, n, b.String())
		}
	}
}
