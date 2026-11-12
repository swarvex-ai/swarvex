package main

// Aufräumen (`swarvex clean`, und einmal am Tag im Dauerbetrieb zwischen zwei Aufgaben): Arbeitsbäume, Protokolle,
// Abhängigkeits-Caches und Umgebungs-Images wuchsen sonst ohne Grenze. Ein Arbeitsbaum mit LFS-Dateien hat schnell
// über 1 GB, und jedes Runner-Update baut die Umgebungs-Images der Projekte neu; die alten blieben liegen.
//
// Was in Arbeit ist, bleibt: laufende Aufgaben, Medien-Aufträge und Gegenprüfungen frischen ihren Ordner alle fünf
// Minuten auf (amLeben), Caches und Images merken sich, wann sie zuletzt gebraucht wurden (benutzt, imageBenutzt).
// Der Lead-Ordner (Klon und Notizen des Leads) gehört nicht dazu; seine Protokolle begrenzt lead.go selbst.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	baumFrist      = 24 * time.Hour      // liegengebliebene Arbeitsbäume und Gegenprüfungen (Runner abgestürzt, hart beendet)
	protokollFrist = 14 * 24 * time.Hour // Protokolle einer Aufgabe (agent.log, proxy.log, setup.log), Angriffstests
	cacheFrist     = 30 * 24 * time.Hour // Caches und Images von Projekten, an denen dieser Rechner nicht mehr arbeitet
	aktivFrist     = 15 * time.Minute    // so lange nach dem letzten Lebenszeichen gilt ein Ordner als in Arbeit
	lebenszeichen  = 5 * time.Minute
	aufraeumTakt   = 24 * time.Hour // so oft räumt der Dauerbetrieb von selbst auf
)

// aufraeumPosten: etwas, das weg kann, mit seiner Größe. Pfad liegt im Datenordner; bei Images ist es leer.
type aufraeumPosten struct {
	Art   string
	Pfad  string
	Image string
	Mehr  []string // eine Gruppe einzelner Dateien in Pfad (LFS-Cache): diese weg, der Ordner bleibt
	Bytes int64
}

type imageInfo struct {
	Name     string
	Erstellt time.Time
	Bytes    int64
}

// Austauschbar für die Tests (ohne Podman).
var (
	imagesAuflisten = podmanImages
	imageEntfernen  = func(name string) error {
		// ohne -f: ein Image, das ein laufender Container (etwa ein Lead-Zyklus) braucht, bleibt stehen
		if aus, err := exec.Command("podman", "rmi", name).CombinedOutput(); err != nil {
			return fmt.Errorf("%s", letzteZeilen(string(aus), 1))
		}
		return nil
	}
)

// amLeben frischt den Ordner einer laufenden Arbeit auf, bis ctx endet: So erkennt das Aufräumen, auch das eines
// anderen Prozesses (`swarvex clean --all` neben dem Dienst), dass er gebraucht wird.
func amLeben(ctx context.Context, ordner string) {
	benutzt(ordner)
	go func() {
		t := time.NewTicker(lebenszeichen)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				benutzt(ordner)
			}
		}
	}()
}

// benutzt setzt die Änderungszeit eines Ordners auf jetzt (letzte Benutzung).
func benutzt(ordner string) {
	jetzt := time.Now()
	os.Chtimes(ordner, jetzt, jetzt)
}

func imageStempel(name string) string {
	return filepath.Join(datenOrdner(), "images", strings.TrimPrefix(name, "localhost/"))
}

// imageBenutzt merkt sich, wann ein Umgebungs-Image zuletzt gebraucht wurde (Podman weiß das nicht).
func imageBenutzt(name string) {
	datei := imageStempel(name)
	os.MkdirAll(filepath.Dir(datei), 0o755)
	if f, err := os.OpenFile(datei, os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		f.Close()
	}
	benutzt(datei)
}

// podmanImage: die Felder aus `podman images --format json`, die das Aufräumen braucht.
type podmanImage struct {
	Names   []string
	Created int64
	Size    int64
}

func podmanImages() []imageInfo {
	var liste []podmanImage
	aus, err := exec.Command("podman", "images", "--format", "json").Output()
	if err != nil || json.Unmarshal(aus, &liste) != nil {
		return nil
	}
	return swarvexImages(liste)
}

// swarvexImages: die Umgebungs-Images, je Name einmal (Podman führt ein Image mit zwei Namen zweimal auf, jedes Mal mit beiden).
func swarvexImages(liste []podmanImage) []imageInfo {
	var images []imageInfo
	gesehen := map[string]bool{}
	for _, i := range liste {
		for _, n := range i.Names {
			kurz := strings.TrimSuffix(strings.TrimPrefix(n, "localhost/"), ":latest")
			if (strings.HasPrefix(kurz, "swarvex-env-") || strings.HasPrefix(kurz, "swarvex-base-")) && !gesehen[kurz] {
				gesehen[kurz] = true
				images = append(images, imageInfo{Name: kurz, Erstellt: time.Unix(i.Created, 0), Bytes: i.Size})
			}
		}
	}
	return images
}

// aufraeumPlan: was nach den Fristen weg kann. alles: auch, was jünger ist (nur nicht, was gerade in Arbeit ist);
// Caches und Images bleiben dann stehen, solange eine Aufgabe läuft, denn sie braucht sie.
func aufraeumPlan(alles bool, jetzt time.Time) []aufraeumPosten {
	d := datenOrdner()
	frist := func(f time.Duration) time.Duration {
		if alles {
			return aktivFrist
		}
		return f
	}
	alter := func(p string) (time.Duration, bool) {
		info, err := os.Lstat(p)
		if err != nil || !info.IsDir() {
			return 0, false
		}
		return jetzt.Sub(info.ModTime()), true
	}
	var plan []aufraeumPosten
	dazu := func(art, pfad string) {
		plan = append(plan, aufraeumPosten{Art: art, Pfad: pfad, Bytes: groesse(pfad)})
	}
	inArbeit := false
	for _, o := range []struct {
		ordner string
		baeume []string // was nach baumFrist weg kann; der ganze Ordner (mit Protokollen) nach protokollFrist
	}{{"work", []string{"tree", "git"}}, {"media", []string{"out", "work", "cache"}}} {
		for _, e := range unterordner(filepath.Join(d, o.ordner)) {
			a, ok := alter(e)
			switch {
			case !ok:
			case a < aktivFrist:
				inArbeit = true
			case a >= frist(protokollFrist):
				dazu("task folder", e)
			case a >= frist(baumFrist):
				for _, b := range o.baeume {
					if _, ok := alter(filepath.Join(e, b)); ok {
						dazu("work tree", filepath.Join(e, b))
					}
				}
			}
		}
	}
	for _, o := range []struct {
		ordner, art string
		frist       time.Duration
	}{{"check", "cross-check", baumFrist}, {"attack", "attack test", protokollFrist}} {
		for _, e := range unterordner(filepath.Join(d, o.ordner)) {
			if a, ok := alter(e); ok && a < aktivFrist {
				inArbeit = true
			} else if ok && a >= frist(o.frist) {
				dazu(o.art, e)
			}
		}
	}
	if alles && inArbeit {
		return plan
	}
	for _, o := range []string{"cache", "tool-cache"} {
		for _, e := range unterordner(filepath.Join(d, o)) {
			if a, ok := alter(e); ok && a >= frist(cacheFrist) {
				dazu("cache", e)
			}
		}
	}
	// LFS-Dateien (lfs.go) behielt der Cache sonst für immer, jede je geladene Version eines Assets; eine Aufgabe frischt
	// jede Datei auf, die sie braucht. Halbe Downloads (download-…) eines abgebrochenen Laufs gehen nach einem Tag.
	lfs := aufraeumPosten{Art: "LFS file", Pfad: lfsCache()}
	eintraege, _ := os.ReadDir(lfs.Pfad)
	for _, e := range eintraege {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		f := cacheFrist
		if strings.HasPrefix(e.Name(), "download-") {
			f = baumFrist
		}
		if jetzt.Sub(info.ModTime()) >= frist(f) {
			lfs.Mehr = append(lfs.Mehr, filepath.Join(lfs.Pfad, e.Name()))
			lfs.Bytes += info.Size()
		}
	}
	if len(lfs.Mehr) > 0 {
		plan = append(plan, lfs)
	}
	// Images der Projekt-Dockerfiles (swarvex-base-…) zuletzt: Podman löscht sie erst, wenn kein Umgebungs-Image mehr darauf aufbaut
	images := imagesAuflisten()
	sort.SliceStable(images, func(i, j int) bool {
		return strings.HasPrefix(images[i].Name, "swarvex-env-") && !strings.HasPrefix(images[j].Name, "swarvex-env-")
	})
	for _, i := range images {
		zuletzt := i.Erstellt
		if info, err := os.Stat(imageStempel(i.Name)); err == nil && info.ModTime().After(zuletzt) {
			zuletzt = info.ModTime()
		}
		if jetzt.Sub(zuletzt) >= frist(cacheFrist) {
			plan = append(plan, aufraeumPosten{Art: "image", Image: i.Name, Bytes: i.Bytes})
		}
	}
	return plan
}

// aufraeumenAusfuehren löscht den Plan; was nicht geht (etwa ein Image in Benutzung), bleibt und wird genannt.
func aufraeumenAusfuehren(plan []aufraeumPosten) (frei int64, weg []aufraeumPosten, fehler []string) {
	for _, p := range plan {
		var err error
		if p.Image != "" {
			if err = imageEntfernen(p.Image); err == nil {
				os.Remove(imageStempel(p.Image))
			}
		} else if len(p.Mehr) > 0 {
			for _, f := range p.Mehr {
				if filepath.Dir(f) != p.Pfad {
					err = fmt.Errorf("%s is outside %s", f, p.Pfad)
				} else if e := os.Remove(f); e != nil && !os.IsNotExist(e) {
					err = e
				}
			}
		} else {
			err = wegraeumen(p.Pfad)
		}
		if err != nil {
			fehler = append(fehler, fmt.Sprintf("%s %s: %v", p.Art, p.anzeige(), err))
			continue
		}
		frei += p.Bytes
		weg = append(weg, p)
	}
	return frei, weg, fehler
}

// wegraeumen löscht einen Ordner im Datenordner, auch mit schreibgeschützten Unterordnern (der Go-Modul-Cache legt
// sie so an) und mit Dateien, die ein Container unter einer anderen Nutzer-ID anlegte (die gehören Podmans
// Nutzer-Namensraum und lassen sich nur dort löschen).
func wegraeumen(pfad string) error {
	if !strings.HasPrefix(pfad, datenOrdner()+string(filepath.Separator)) {
		return fmt.Errorf("%s is outside the work data", pfad)
	}
	if os.RemoveAll(pfad) == nil {
		return nil
	}
	filepath.WalkDir(pfad, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			os.Chmod(p, 0o755)
		}
		return nil
	})
	err := os.RemoveAll(pfad)
	if err == nil {
		return nil
	}
	if _, e := exec.LookPath("podman"); e == nil {
		if exec.Command("podman", "unshare", "rm", "-rf", pfad).Run() == nil {
			return nil
		}
	}
	return err
}

func unterordner(ordner string) []string {
	eintraege, _ := os.ReadDir(ordner)
	var pfade []string
	for _, e := range eintraege {
		if e.IsDir() {
			pfade = append(pfade, filepath.Join(ordner, e.Name()))
		}
	}
	return pfade
}

// groesse: Bytes aller Dateien unter pfad (Verknüpfungen zählen nicht mit, ihnen wird nicht gefolgt).
func groesse(pfad string) int64 {
	var summe int64
	filepath.WalkDir(pfad, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				summe += info.Size()
			}
		}
		return nil
	})
	return summe
}

func (p aufraeumPosten) anzeige() string {
	if p.Image != "" {
		return p.Image
	}
	if len(p.Mehr) > 0 {
		return fmt.Sprintf("%s (%d)", heimKurz(p.Pfad), len(p.Mehr))
	}
	if rel, err := filepath.Rel(datenOrdner(), p.Pfad); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return heimKurz(p.Pfad)
}

// heimKurz: der Home-Ordner als ~
func heimKurz(pfad string) string {
	if h, err := os.UserHomeDir(); err == nil && strings.HasPrefix(pfad, h+string(filepath.Separator)) {
		return "~" + pfad[len(h):]
	}
	return pfad
}

func groesseText(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(bytes)/(1<<20))
	default:
		return fmt.Sprintf("%.0f KB", float64(bytes)/(1<<10))
	}
}

// zaehlung: "2 work trees, 1 cache, 3 images"
func zaehlung(posten []aufraeumPosten) string {
	anzahl := map[string]int{}
	var reihenfolge []string
	for _, p := range posten {
		if anzahl[p.Art] == 0 {
			reihenfolge = append(reihenfolge, p.Art)
		}
		anzahl[p.Art] += max(1, len(p.Mehr))
	}
	teile := make([]string, len(reihenfolge))
	for i, a := range reihenfolge {
		teile[i] = fmt.Sprintf("%d %s", anzahl[a], a)
		if anzahl[a] != 1 {
			teile[i] += "s"
		}
	}
	return strings.Join(teile, ", ")
}

// selbstAufraeumen: der tägliche Lauf im Dauerbetrieb, nur nach den Fristen; meldet sich nur, wenn er etwas fand.
func selbstAufraeumen() {
	plan := aufraeumPlan(false, time.Now())
	if len(plan) == 0 {
		return
	}
	frei, weg, fehler := aufraeumenAusfuehren(plan)
	if len(weg) > 0 {
		fmt.Printf("Cleaned up old work data: %s (%s freed).\n", zaehlung(weg), groesseText(frei))
	}
	for _, f := range fehler {
		fmt.Println("Could not clean up:", f)
	}
}

// swarvex clean [--all] [--dry-run]
func aufraeumenBefehl(args []string) {
	fs := flag.NewFlagSet("clean", flag.ExitOnError)
	alles := fs.Bool("all", false, "also remove logs, caches and environment images younger than the time limits (never what a running task uses)")
	probe := fs.Bool("dry-run", false, "only show what would be removed")
	fs.Parse(args)
	plan := aufraeumPlan(*alles, time.Now())
	if len(plan) == 0 {
		fmt.Println("Nothing to clean up.")
	} else {
		var summe int64
		titel := "Removing:"
		if *probe {
			titel = "Would remove:"
		}
		fmt.Println(titel)
		for _, p := range plan {
			art := p.Art
			if len(p.Mehr) > 1 {
				art += "s"
			}
			fmt.Printf("  %-12s %-44s %s\n", art, p.anzeige(), groesseText(p.Bytes))
			summe += p.Bytes
		}
		if *probe {
			fmt.Printf("That frees about %s. Run without --dry-run to remove it.\n", groesseText(summe))
			return
		}
		frei, weg, fehler := aufraeumenAusfuehren(plan)
		for _, f := range fehler {
			fmt.Println("  ✗", f)
		}
		fmt.Printf("Removed %s; about %s freed (images share layers, so podman may free less).\n", zaehlung(weg), groesseText(frei))
	}
	fmt.Printf("Work data now: %s in %s, LFS cache %s\n", groesseText(groesse(datenOrdner())), heimKurz(datenOrdner()), groesseText(groesse(lfsCache())))
	if !*alles {
		fmt.Printf("Kept: what was used recently; work trees go after %d hours, logs after %d days, caches, LFS files and images after %d days unused (--all: everything not in use right now).\n",
			int(baumFrist.Hours()), int(protokollFrist.Hours()/24), int(cacheFrist.Hours()/24))
	}
}

// datenZeilen: Arbeitsdaten und Cache für `swarvex status`, und was sich davon löschen ließe.
func datenZeilen() [][2]string {
	var frei int64
	for _, p := range aufraeumPlan(false, time.Now()) {
		if p.Image == "" {
			frei += p.Bytes
		}
	}
	z := [][2]string{{"Work data", groesseText(groesse(datenOrdner())) + " in " + heimKurz(datenOrdner())}, {"LFS cache", groesseText(groesse(lfsCache()))}}
	if frei >= 100<<20 {
		z = append(z, [2]string{"Can be freed", groesseText(frei) + " no longer needed (swarvex clean)"})
	}
	return z
}
