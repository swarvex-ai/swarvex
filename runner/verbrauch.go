package main

// Tagesbudget: Der Runner schreibt nach jeder Aufgabe den Token-Verbrauch in ein lokales Journal
// (verbrauch.jsonl im Datenordner) und beginnt keine Aufgabe mehr, wenn das Budget des Tages verbraucht ist.
// Das Budget jeder Aufgabe wird auf den Rest des Tages gekürzt; der Schlüssel-Proxy bricht dann hart ab.
// Die Grenze setzt allein der Runner durch, die Plattform kann sie nicht erhöhen.

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type verbrauchsEintrag struct {
	Zeit    time.Time `json:"time"`
	Aufgabe int64     `json:"task"`
	Tokens  int64     `json:"tokens"`
}

func verbrauchsDatei() string { return filepath.Join(datenOrdner(), "usage.jsonl") }

func verbrauchMerken(datei string, aufgabe int64, tokens int64, t time.Time) error {
	os.MkdirAll(filepath.Dir(datei), 0o755)
	f, err := os.OpenFile(datei, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(verbrauchsEintrag{Zeit: t, Aufgabe: aufgabe, Tokens: tokens})
}

// verbrauchAmTag summiert die Tokens des Kalendertags von t (lokale Zeit).
func verbrauchAmTag(datei string, t time.Time) int64 {
	f, err := os.Open(datei)
	if err != nil {
		return 0
	}
	defer f.Close()
	j, m, d := t.Date()
	var summe int64
	s := bufio.NewScanner(f)
	for s.Scan() {
		var e verbrauchsEintrag
		if json.Unmarshal(s.Bytes(), &e) != nil {
			continue
		}
		if ej, em, ed := e.Zeit.In(t.Location()).Date(); ej == j && em == m && ed == d {
			summe += e.Tokens
		}
	}
	return summe
}

// mitternacht liefert den Beginn des nächsten Kalendertags (lokale Zeit).
func mitternacht(t time.Time) time.Time {
	j, m, d := t.Date()
	return time.Date(j, m, d+1, 0, 0, 0, 0, t.Location())
}

// tokensAus liest den Token-Verbrauch aus dem Bericht des Schlüssel-Proxys.
func tokensAus(verbrauch map[string]float64) int64 {
	return int64(verbrauch["tokens"])
}
