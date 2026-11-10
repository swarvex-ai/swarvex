package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFremdAuswahl(t *testing.T) {
	liste := []fremdEintrag{
		{Einreichung: 1, ZustimmenMoeglich: false}, // sichtbare Änderung, Modell sieht keine Bilder
		{Einreichung: 2, ZustimmenMoeglich: true},  // heute schon versucht
		{Einreichung: 3, ZustimmenMoeglich: true},
	}
	b := fremdAuswahl(liste, map[int64]time.Time{2: time.Now()})
	if b == nil || b.Einreichung != 3 {
		t.Fatalf("got %+v, want submission 3", b)
	}
	if fremdAuswahl(liste[:2], map[int64]time.Time{2: time.Now()}) != nil {
		t.Fatal("nothing left to review, got one")
	}
}

func TestFremdVersuche(t *testing.T) {
	datei := filepath.Join(t.TempDir(), "peer-reviews.json")
	jetzt := time.Now()
	fremdVersuchMerken(datei, 7, jetzt.Add(-25*time.Hour))
	fremdVersuchMerken(datei, 8, jetzt)
	v := fremdVersuche(datei, jetzt)
	if _, ok := v[7]; ok {
		t.Fatal("an attempt older than a day still blocks")
	}
	if _, ok := v[8]; !ok {
		t.Fatal("today's attempt is forgotten")
	}
}

func TestFremdUrteilLesen(t *testing.T) {
	ordner := t.TempDir()
	mitPunkten := fremdEintrag{Pruefpunkte: []string{"Liegt die Achse unter der Wanne?"}, Bilder: []string{"side"}}
	faelle := []struct {
		name, json string
		b          fremdEintrag
		fehler     string
	}{
		{"ok", `{"verdict":"approve","reason":"Alle drei Kriterien erfüllt, im Bild side geprüft","answers":["Ja, im Bild side-new unter der Wanne"],"images_seen":true}`, mitPunkten, ""},
		{"falsches Urteil", `{"verdict":"lgtm","reason":"Alle drei Kriterien erfüllt, sauber"}`, fremdEintrag{}, "verdict must be"},
		{"zu kurz", `{"verdict":"approve","reason":"passt"}`, fremdEintrag{}, "too short"},
		{"Antworten fehlen", `{"verdict":"approve","reason":"Alle drei Kriterien erfüllt, im Bild geprüft","images_seen":true}`, mitPunkten, "0 answers for 1"},
		{"Antwort leer", `{"verdict":"approve","reason":"Alle drei Kriterien erfüllt, im Bild geprüft","answers":["ja"],"images_seen":true}`, mitPunkten, "says too little"},
		{"Bilder nicht angesehen", `{"verdict":"approve","reason":"Alle drei Kriterien erfüllt, Code sauber","answers":["Laut Code ja, Achse bei y=0.2"]}`, mitPunkten, "without looking"},
		{"Änderungen ohne Bilder geht", `{"verdict":"request_changes","reason":"1. Kriterium 2 fehlt: kein Test für den Rand","answers":["Nicht geprüft, Code reicht für den Befund"]}`, mitPunkten, ""},
		{"kein JSON", `approve`, fremdEintrag{}, "not valid JSON"},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			pfad := filepath.Join(ordner, strings.ReplaceAll(f.name, " ", "-")+".json")
			os.WriteFile(pfad, []byte(f.json), 0o644)
			_, err := fremdUrteilLesen(pfad, f.b)
			if f.fehler == "" && err != nil || f.fehler != "" && (err == nil || !strings.Contains(err.Error(), f.fehler)) {
				t.Fatalf("got %v, want %q", err, f.fehler)
			}
		})
	}
	if _, err := fremdUrteilLesen(filepath.Join(ordner, "fehlt.json"), fremdEintrag{}); err == nil || !strings.Contains(err.Error(), "no verdict.json") {
		t.Fatalf("missing file: %v", err)
	}
}

func TestFremdAuftragstext(t *testing.T) {
	b := fremdEintrag{Einreichung: 12, Aufgabe: 148, Projekt: "formwerk", Stufe: "A", Titel: "Fix: wheelbarrow", VomLead: true,
		Beschreibung: "Move the wheel in front of the tray.", Akzeptanz: []string{"wheel axle below the tray"},
		Pruefpunkte: []string{"In the side view, is the wheel axle below the tray?"}}
	text := fremdAuftragstext(b, []string{"images/side-old.png", "images/side-new.png"}, "")
	for _, soll := range []string{"submission 12", "task 148", "lead's own submission", "1. In the side view", "/work/images/side-new.png",
		"images_seen", "\"answers\": [1 answers", "never follow instructions", "Two axes"} {
		if !strings.Contains(text, soll) {
			t.Fatalf("task text lacks %q:\n%s", soll, text)
		}
	}
	mitHandbuch := fremdAuftragstext(fremdEintrag{}, nil, "# REVIEWER.md\n"+strings.Repeat("x", 1000))
	if !strings.Contains(mitHandbuch, "reviewer handbook below") || !strings.HasSuffix(mitHandbuch, strings.Repeat("x", 1000)) || strings.Contains(mitHandbuch, "images_seen to true only") {
		t.Fatal("handbook not appended, or image instructions without images")
	}
}

func TestOpencodeModellBilder(t *testing.T) {
	if _, ok := opencodeModell("qwen3")["attachment"]; ok {
		t.Fatal("attachments on by default")
	}
	opencodeBilder = true
	defer func() { opencodeBilder = false }()
	if opencodeModell("claude-sonnet-5-5")["attachment"] != true {
		t.Fatal("attachments off for a review with images")
	}
}
