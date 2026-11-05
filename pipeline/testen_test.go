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

func TestGegenprobe(t *testing.T) {
	for _, f := range []struct {
		name      string
		bestanden bool
		err       error
		rot       bool
		enthaelt  string
	}{
		{"Projekt schon rot", false, nil, true, "failed too"},
		{"nur mit Beitrag rot", true, nil, false, "fail only with this contribution"},
		{"Gegenprobe unmöglich", false, errors.New("patch -R"), false, "not possible"},
	} {
		var b strings.Builder
		if rot := gegenprobe(&b, func() (string, bool, error) { return "lauf", f.bestanden, f.err }); rot != f.rot || !strings.Contains(b.String(), f.enthaelt) {
			t.Errorf("%s: rot=%v report=%q", f.name, rot, b.String())
		}
	}
}
