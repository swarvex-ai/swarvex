package main

// swarvex env [ordner]: zeigt, wie swarvex ein Projekt einrichtet und testet, und probiert es
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
	fs := flag.NewFlagSet("env", flag.ExitOnError)
	nurZeigen := fs.Bool("show-only", false, "only show, do not build or run anything")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: swarvex env [--show-only] [project-folder]")
		fmt.Fprintln(os.Stderr, "Detects the environment (swarvex.yaml, dev container or project type), builds it, fetches the")
		fmt.Fprintln(os.Stderr, "dependencies and runs build and test without network, just like the runner and the test pipeline.")
	}
	fs.Parse(args)
	wurzel := "."
	if fs.NArg() > 0 {
		wurzel = fs.Arg(0)
	}
	wurzel, _ = filepath.Abs(wurzel)
	u, err := umgebung.Ermitteln(wurzel)
	pruefen(err)
	fmt.Printf("Environment: %s (source: %s)\n", u.Vorlage, u.Quelle)
	if u.Engine != nil {
		i, err := engineFuer(u, enginesFinden())
		if err != nil {
			fmt.Printf("Engine:      %s, %v\n", u.Engine, err)
			if !*nurZeigen {
				os.Exit(1)
			}
		} else {
			fmt.Printf("Engine:      %s from this machine (read-only, license stays here)\n", i)
			u.Einbinden = engineArgs(i)
		}
	}
	if u.Image != "" {
		fmt.Printf("Image:       %s\n", u.Image)
	}
	if u.Dockerfile != "" {
		fmt.Printf("Dockerfile:  %s\n", u.Dockerfile)
	}
	if u.Ordner != "" {
		fmt.Printf("Folder:      %s\n", u.Ordner)
	}
	zeigen := func(name, befehl string) {
		if befehl == "" {
			befehl = "–"
		}
		fmt.Printf("%-12s %s\n", name+":", einruecken(befehl)[4:])
	}
	zeigen("Setup", u.Einrichten)
	zeigen("Build", u.Bauen)
	zeigen("Test", u.Testen)
	if *nurZeigen {
		return
	}

	ctx := context.Background()
	fmt.Println("\n== Building the environment")
	image, err := umgebungsImage(ctx, u, wurzel, false)
	pruefen(err)
	cache := cacheOrdner("local-" + filepath.Base(wurzel))
	fmt.Println("== Setup (with network, without agent)")
	protokoll := filepath.Join(os.TempDir(), "swarvex-setup.log")
	if err := einrichten(ctx, u, image, wurzel, cache, protokoll); err != nil {
		fehlerAus(err.Error())
	}
	gruen := true
	for _, schritt := range []struct{ name, befehl string }{{"Build", u.Bauen}, {"Test", u.Testen}} {
		if schritt.befehl == "" {
			continue
		}
		fmt.Printf("== %s (without network)\n", schritt.name)
		start := time.Now()
		aus, err := ohneNetz(ctx, u, image, wurzel, cache, schritt.befehl, 1800)
		fmt.Println(letzteZeilen(aus, 40))
		if err != nil {
			fmt.Printf("✘ %s failed after %s: %v\n", schritt.name, time.Since(start).Round(time.Second), err)
			gruen = false
			break
		}
		fmt.Printf("✔ %s in %s\n", schritt.name, time.Since(start).Round(time.Second))
	}
	if u.Testen == "" {
		fmt.Println("Note: no automated tests configured. Add \"test:\" to swarvex.yaml so the test pipeline checks every contribution.")
	}
	if !gruen {
		os.Exit(1)
	}
	fmt.Println("\n✓ The project is ready for swarvex.")
}
