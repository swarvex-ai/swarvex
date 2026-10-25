package main

// swarvex umgebung [ordner]: zeigt, wie swarvex ein Projekt einrichtet und testet, und probiert es
// aus (wie die Prüfstraße, ohne Agent). Für Visionäre, bevor sie ein Projekt einstellen.

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

func umgebungBefehl(args []string) {
	fs := flag.NewFlagSet("umgebung", flag.ExitOnError)
	nurZeigen := fs.Bool("nur-zeigen", false, "nur anzeigen, nichts bauen oder ausführen")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Aufruf: swarvex umgebung [--nur-zeigen] [projektordner]")
		fmt.Fprintln(os.Stderr, "Erkennt die Umgebung (swarvex.yaml, Dev Container oder Projektart), baut sie, lädt die")
		fmt.Fprintln(os.Stderr, "Abhängigkeiten und führt Bauen und Testen ohne Netz aus – so wie Runner und Prüfstraße.")
	}
	fs.Parse(args)
	wurzel := "."
	if fs.NArg() > 0 {
		wurzel = fs.Arg(0)
	}
	wurzel, _ = filepath.Abs(wurzel)
	u, err := umgebung.Ermitteln(wurzel)
	pruefen(err)
	fmt.Printf("Umgebung:   %s (Quelle: %s)\n", u.Vorlage, u.Quelle)
	if u.Image != "" {
		fmt.Printf("Image:      %s\n", u.Image)
	}
	if u.Dockerfile != "" {
		fmt.Printf("Dockerfile: %s\n", u.Dockerfile)
	}
	if u.Ordner != "" {
		fmt.Printf("Ordner:     %s\n", u.Ordner)
	}
	zeigen := func(name, befehl string) {
		if befehl == "" {
			befehl = "–"
		}
		fmt.Printf("%-11s %s\n", name+":", einruecken(befehl)[4:])
	}
	zeigen("Einrichten", u.Einrichten)
	zeigen("Bauen", u.Bauen)
	zeigen("Testen", u.Testen)
	if *nurZeigen {
		return
	}

	ctx := context.Background()
	fmt.Println("\n== Umgebung bauen")
	image, err := umgebungsImage(ctx, u, wurzel, false)
	pruefen(err)
	cache := cacheOrdner("lokal-" + filepath.Base(wurzel))
	fmt.Println("== Einrichten (mit Netz, ohne Agent)")
	protokoll := filepath.Join(os.TempDir(), "swarvex-einrichten.log")
	if err := einrichten(ctx, u, image, wurzel, cache, protokoll); err != nil {
		fehlerAus(err.Error())
	}
	gruen := true
	for _, schritt := range []struct{ name, befehl string }{{"Bauen", u.Bauen}, {"Testen", u.Testen}} {
		if schritt.befehl == "" {
			continue
		}
		fmt.Printf("== %s (ohne Netz)\n", schritt.name)
		start := time.Now()
		aus, err := ohneNetz(ctx, u, image, wurzel, cache, schritt.befehl, 1800)
		fmt.Println(letzteZeilen(aus, 40))
		if err != nil {
			fmt.Printf("✘ %s fehlgeschlagen nach %s: %v\n", schritt.name, time.Since(start).Round(time.Second), err)
			gruen = false
			break
		}
		fmt.Printf("✔ %s in %s\n", schritt.name, time.Since(start).Round(time.Second))
	}
	if u.Testen == "" {
		fmt.Println("Hinweis: keine automatischen Tests hinterlegt – in swarvex.yaml mit „testen:“ ergänzen, dann prüft die Prüfstraße jeden Beitrag.")
	}
	if !gruen {
		os.Exit(1)
	}
	fmt.Println("\n✓ Das Projekt ist bereit für swarvex.")
}
