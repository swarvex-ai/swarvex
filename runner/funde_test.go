package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

func TestFundeLesen(t *testing.T) {
	baum := t.TempDir()
	keinGit := filepath.Join(t.TempDir(), "fehlt") // ohne Git-Verzeichnis: die Datei gilt als nicht im Projekt
	datei := filepath.Join(baum, fundDatei)
	schreiben := func(s string) { os.WriteFile(datei, []byte(s), 0o644) }

	schreiben(`[
	  {"title": "Pause menu   leaks a tween", "text": "Seen in /home/anna/game, ask anna@example.org", "paths": ["/work/game/ui/pause.gd", "./game/x.gd", "../geheim", "/etc/passwd"], "level": "b"},
	  {"title": "ok"},
	  {"title": "Missing test for save slots", "level": "Z"},
	  {"title": "Third one"},
	  {"title": "Fourth one over the limit"}]`)
	f := fundeLesen(keinGit, baum)
	if len(f) != 3 || f[0].Titel != "Pause menu leaks a tween" || f[0].Stufe != "B" || f[1].Stufe != "" || f[2].Titel != "Third one" {
		t.Fatalf("gelesen: %+v", f)
	}
	if f[0].Text != "Seen in ~/game, ask [address]" || !reflect.DeepEqual(f[0].Pfade, []string{"game/ui/pause.gd", "game/x.gd"}) {
		t.Errorf("maskiert/Pfade: %q %q", f[0].Text, f[0].Pfade)
	}
	if _, err := os.Stat(datei); !os.IsNotExist(err) {
		t.Error("die Datei bleibt nicht im Arbeitsbaum (sonst landete sie im Patch)")
	}
	// ein einzelnes Objekt statt einer Liste; kaputtes JSON fällt weg, die Datei auch
	schreiben(`{"title": "Only one finding here"}`)
	if f := fundeLesen(keinGit, baum); len(f) != 1 || f[0].Titel != "Only one finding here" {
		t.Errorf("einzeln: %+v", f)
	}
	schreiben(`[{"title": `)
	if f := fundeLesen(keinGit, baum); f != nil {
		t.Errorf("kaputt: %+v", f)
	}
	// Ein Link auf eine Datei des Betreibers wird nicht gelesen (er würde außerhalb der Sandbox aufgelöst), nur entfernt
	geheim := filepath.Join(t.TempDir(), "id_ed25519")
	os.WriteFile(geheim, []byte(`[{"title": "PRIVATE KEY material"}]`), 0o600)
	os.Symlink(geheim, datei)
	if f := fundeLesen(keinGit, baum); f != nil {
		t.Errorf("Link gelesen: %+v", f)
	}
	if _, err := os.Lstat(datei); !os.IsNotExist(err) {
		t.Error("der Link bleibt nicht liegen")
	}
	if roh, _ := os.ReadFile(geheim); !strings.Contains(string(roh), "PRIVATE") {
		t.Error("das Ziel des Links bleibt unberührt")
	}
	// zu groß
	schreiben(`[{"title": "` + strings.Repeat("x", sandboxMaxB) + `"}]`)
	if f := fundeLesen(keinGit, baum); f != nil {
		t.Errorf("zu groß gelesen: %d", len(f))
	}
}

func TestVerworfenFund(t *testing.T) {
	if f := verworfenFund(7, "Effekt", []string{"game/a.png.import", "game/a.gd.uid", ".godot/editor/x.cfg", "package-lock.json"}); f != nil {
		t.Errorf("nur Erzeugtes: %+v", f)
	}
	f := verworfenFund(7, "Effekt", []string{"game/a.png.import", "game/ui/pause.gd", "tests/pause_test.gd"})
	if f == nil || f.Titel != "Changes outside the allowed paths of task #7" || !reflect.DeepEqual(f.Pfade, []string{"game/ui/pause.gd", "tests/pause_test.gd"}) ||
		!strings.Contains(f.Text, "task #7 (Effekt)") || !strings.Contains(f.Text, "content stayed on the contributor's machine") {
		t.Errorf("gemischt: %+v", f)
	}
	viele := make([]string, 14)
	for i := range viele {
		viele[i] = "src/datei" + string(rune('a'+i)) + ".go"
	}
	if f := verworfenFund(8, "x", viele); len(f.Pfade) != fundePfade || !strings.Contains(f.Text, "and 4 more") {
		t.Errorf("viele: %+v", f)
	}
	// Mit drei eigenen Meldungen des Agenten bleibt Platz für die des Runners
	agent := []fund{{Titel: "eins eins"}, {Titel: "zwei zwei"}, {Titel: "drei drei"}}
	z := fundeZusammen(agent, f)
	if len(z) != fundeMax || z[2].Titel != f.Titel || len(agent) != 3 || agent[2].Titel != "drei drei" {
		t.Errorf("zusammen: %+v (agent danach: %+v)", z, agent)
	}
	if z := fundeZusammen(agent, nil); len(z) != 3 {
		t.Errorf("ohne Verworfenes: %+v", z)
	}
}

func TestAuftragNenntFundmeldungen(t *testing.T) {
	text := auftragstext(aufgabe{Titel: "T", Pfade: []string{"game/**"}}, umgebung.Umgebung{})
	if !strings.Contains(text, "/work/"+fundDatei) || !strings.Contains(text, "FINDINGS:") || !strings.Contains(text, "The one exception is /work/"+fundDatei) {
		t.Errorf("Auftrag ohne Fundmeldungen:\n%s", text)
	}
}
