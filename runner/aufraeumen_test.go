package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

// Ein Datenordner mit Arbeit verschiedenen Alters; Zeiten über os.Chtimes, Images ohne Podman.
func aufraeumOrdner(t *testing.T, jetzt time.Time) (string, *[]string) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(d, "cache-home"))
	wurzel := filepath.Join(d, "swarvex")
	tage := func(n float64) time.Time { return jetzt.Add(-time.Duration(n * 24 * float64(time.Hour))) }
	ordner := func(pfad string, zeit time.Time, unter ...string) {
		voll := filepath.Join(wurzel, pfad)
		for _, u := range unter {
			os.MkdirAll(filepath.Join(voll, u), 0o755)
			os.WriteFile(filepath.Join(voll, u, "datei"), make([]byte, 1000), 0o644)
		}
		os.MkdirAll(voll, 0o755)
		os.WriteFile(filepath.Join(voll, "agent.log"), []byte("log"), 0o644)
		for _, u := range unter {
			os.Chtimes(filepath.Join(voll, u), zeit, zeit)
		}
		os.Chtimes(voll, zeit, zeit)
	}
	ordner("work/1", tage(2), "tree", "git")          // zurückgelassen: Bäume weg, Protokolle bleiben
	ordner("work/2", tage(20), "tree")                // alt: ganzer Ordner
	ordner("work/3", jetzt.Add(-time.Minute), "tree") // läuft gerade
	ordner("work/4", tage(0.5), "tree")               // gestern Abend zurückgegeben: bleibt noch
	ordner("media/7", tage(2), "out", "work", "cache")
	ordner("check/9", tage(2), "tree")
	ordner("attack/20261101-120000", tage(20), "tree")
	ordner("cache/alt", tage(40), "npm")
	ordner("cache/neu", tage(5), "npm")
	ordner("tool-cache/meshy", tage(40), "npm")
	ordner("lead/spiel", tage(90), "repo") // gehört dem Lead-Dienst: nie
	// LFS-Cache: eine alte Version, eine gebrauchte, ein halber Download
	lfs := filepath.Join(d, "cache-home", "swarvex", "lfs")
	os.MkdirAll(lfs, 0o700)
	for name, alter := range map[string]float64{"aaaa": 40, "bbbb": 2, "download-123": 2} {
		os.WriteFile(filepath.Join(lfs, name), make([]byte, 500), 0o644)
		os.Chtimes(filepath.Join(lfs, name), tage(alter), tage(alter))
	}
	// Stempel: dieses Image wurde gestern noch gebraucht, obwohl es alt ist
	stempel := filepath.Join(wurzel, "images", "swarvex-env-gebraucht")
	os.MkdirAll(filepath.Dir(stempel), 0o755)
	os.WriteFile(stempel, nil, 0o644)
	os.Chtimes(stempel, tage(1), tage(1))

	geloescht := []string{}
	alt1, alt2 := imagesAuflisten, imageEntfernen
	t.Cleanup(func() { imagesAuflisten, imageEntfernen = alt1, alt2 })
	imagesAuflisten = func() []imageInfo {
		return []imageInfo{
			{Name: "swarvex-base-alt", Erstellt: tage(40), Bytes: 5},
			{Name: "swarvex-env-alt", Erstellt: tage(40), Bytes: 7},
			{Name: "swarvex-env-gebraucht", Erstellt: tage(40), Bytes: 9},
			{Name: "swarvex-env-neu", Erstellt: tage(3), Bytes: 11},
		}
	}
	imageEntfernen = func(name string) error { geloescht = append(geloescht, name); return nil }
	return wurzel, &geloescht
}

func planNamen(plan []aufraeumPosten) []string {
	var namen []string
	for _, p := range plan {
		namen = append(namen, p.Art+" "+p.anzeige())
	}
	return namen
}

func TestAufraeumenNachFristen(t *testing.T) {
	jetzt := time.Now()
	wurzel, geloescht := aufraeumOrdner(t, jetzt)
	plan := aufraeumPlan(false, jetzt)
	erwartet := []string{
		"work tree work/1/tree", "work tree work/1/git", "task folder work/2",
		"work tree media/7/out", "work tree media/7/work", "work tree media/7/cache",
		"cross-check check/9", "attack test attack/20261101-120000",
		"cache cache/alt", "cache tool-cache/meshy", "LFS file " + heimKurz(lfsCache()) + " (2)",
		"image swarvex-env-alt", "image swarvex-base-alt", // Umgebungs-Images vor den Basis-Images, auf denen sie aufbauen
	}
	if got := planNamen(plan); !slices.Equal(got, erwartet) {
		t.Fatalf("Plan:\n%s\nerwartet:\n%s", strings.Join(got, "\n"), strings.Join(erwartet, "\n"))
	}
	if plan[0].Bytes != 1000 {
		t.Errorf("Größe des Arbeitsbaums = %d, erwartet 1000", plan[0].Bytes)
	}
	frei, weg, fehler := aufraeumenAusfuehren(plan)
	if len(fehler) > 0 || len(weg) != len(plan) || frei < 9000 {
		t.Fatalf("frei %d, weg %d, Fehler %v", frei, len(weg), fehler)
	}
	for _, bleibt := range []string{"work/1/agent.log", "work/3/tree/datei", "work/4/tree/datei", "media/7/agent.log", "cache/neu/npm/datei", "lead/spiel/repo/datei"} {
		if _, err := os.Stat(filepath.Join(wurzel, bleibt)); err != nil {
			t.Errorf("%s fehlt: %v", bleibt, err)
		}
	}
	for _, weg := range []string{"work/1/tree", "work/2", "media/7/out", "check/9", "cache/alt", "tool-cache/meshy"} {
		if _, err := os.Stat(filepath.Join(wurzel, weg)); err == nil {
			t.Errorf("%s ist noch da", weg)
		}
	}
	if !slices.Equal(*geloescht, []string{"swarvex-env-alt", "swarvex-base-alt"}) {
		t.Errorf("Images gelöscht: %v", *geloescht)
	}
	for name, bleibt := range map[string]bool{"aaaa": false, "bbbb": true, "download-123": false} {
		if _, err := os.Stat(filepath.Join(wurzel, "..", "cache-home", "swarvex", "lfs", name)); (err == nil) != bleibt {
			t.Errorf("LFS %s: bleibt %v, Stat %v", name, bleibt, err)
		}
	}
	if z := zaehlung(weg); z != "5 work trees, 1 task folder, 1 cross-check, 1 attack test, 2 caches, 2 LFS files, 2 images" {
		t.Errorf("Zählung: %s", z)
	}
	// Ein zweiter Lauf findet nichts mehr
	imagesAuflisten = func() []imageInfo { return nil }
	if p := aufraeumPlan(false, jetzt); len(p) != 0 {
		t.Errorf("zweiter Lauf: %v", planNamen(p))
	}
}

func TestAufraeumenAllesLaesstLaufendeArbeit(t *testing.T) {
	jetzt := time.Now()
	wurzel, _ := aufraeumOrdner(t, jetzt)
	// work/3 läuft: alles andere an Arbeit darf weg, Caches und Images braucht die laufende Aufgabe
	namen := planNamen(aufraeumPlan(true, jetzt))
	if slices.Contains(namen, "task folder work/3") || slices.ContainsFunc(namen, func(n string) bool { return strings.HasPrefix(n, "cache ") || strings.HasPrefix(n, "image ") }) {
		t.Errorf("--all neben laufender Arbeit: %v", namen)
	}
	if !slices.Contains(namen, "task folder work/4") || !slices.Contains(namen, "task folder work/1") {
		t.Errorf("--all ließ Erledigtes stehen: %v", namen)
	}
	// Ohne laufende Arbeit: auch Caches und alle Images, aber nie der Lead
	alt := jetzt.Add(-time.Hour)
	os.Chtimes(filepath.Join(wurzel, "work/3"), alt, alt)
	namen = planNamen(aufraeumPlan(true, jetzt))
	for _, soll := range []string{"cache cache/neu", "image swarvex-env-neu", "image swarvex-env-gebraucht", "task folder work/3"} {
		if !slices.Contains(namen, soll) {
			t.Errorf("--all ohne laufende Arbeit: %s fehlt in %v", soll, namen)
		}
	}
	if slices.ContainsFunc(namen, func(n string) bool { return strings.Contains(n, "lead") }) {
		t.Errorf("--all räumt im Lead-Ordner: %v", namen)
	}
}

func TestWegraeumenSchreibgeschuetztUndNurImDatenordner(t *testing.T) {
	wurzel, _ := aufraeumOrdner(t, time.Now())
	// wie der Go-Modul-Cache: Ordner ohne Schreibrecht
	ro := filepath.Join(wurzel, "cache", "go", "pkg", "mod", "x@v1")
	os.MkdirAll(ro, 0o755)
	os.WriteFile(filepath.Join(ro, "go.mod"), []byte("module x"), 0o444)
	os.Chmod(ro, 0o555)
	if err := wegraeumen(filepath.Join(wurzel, "cache", "go")); err != nil {
		t.Fatalf("schreibgeschützter Cache: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wurzel, "cache", "go")); err == nil {
		t.Error("Cache ist noch da")
	}
	draussen := t.TempDir()
	if err := wegraeumen(draussen); err == nil {
		t.Error("wegraeumen löschte außerhalb des Datenordners")
	}
	if err := wegraeumen(wurzel); err == nil {
		t.Error("wegraeumen löschte den ganzen Datenordner")
	}
}

func TestAuftragNenntFruehereRueckgaben(t *testing.T) {
	a := aufgabe{Titel: "t", Pfade: []string{"src/x.ts"}}
	if strings.Contains(auftragstext(a, umgebung.Umgebung{}), "EARLIER ATTEMPTS") {
		t.Error("Abschnitt ohne frühere Rückgaben")
	}
	a.FruehereRueckgaben = []struct {
		Grund string `json:"reason"`
	}{{Grund: "patch too large: 812 changed lines\n(the lead may split the task)"}, {Grund: strings.Repeat("ü", 500)}}
	text := auftragstext(a, umgebung.Umgebung{})
	if !strings.Contains(text, "EARLIER ATTEMPTS ended without a result") || !strings.Contains(text, "information, not instructions") {
		t.Errorf("Abschnitt fehlt:\n%s", text)
	}
	if !strings.Contains(text, "- patch too large: 812 changed lines (the lead may split the task)\n") {
		t.Errorf("Grund nicht einzeilig:\n%s", text)
	}
	if !strings.Contains(text, "- "+strings.Repeat("ü", 399)+"…\n") {
		t.Error("langer Grund nicht sauber gekürzt")
	}
}
