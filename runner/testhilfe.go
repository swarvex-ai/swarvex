package main

// Testhilfe für den Agenten (Hebel B aus „Schneller werden“, neVs 14.11.2026): Der Agent bekam den vollen Testbefehl
// des Projekts und ließ damit die ganze Suite laufen (Formwerk baut dann den ganzen Katalog; 26 Minuten für ein
// Sprite). /run/swarvex-test/test.sh baut und testet dagegen nur, was er in seinen Pfaden geändert hat
// (SWARVEX_CHANGED), genau wie der eigene Testlauf des Runners danach. Den vollen Lauf macht die Prüfstraße. Der
// eigene Testlauf bleibt: Er zählt bei der Plattform als Ergebnis des Runners, das der Agent nicht vortäuschen kann.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

const testHilfePfad = "/run/swarvex-test"

// shZitat: ein Wort für sh in einfachen Anführungszeichen.
func shZitat(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// testSkript: geänderte Dateien in den Pfaden der Aufgabe seit dem Start des Agenten finden, dann Bauen und Testen wie
// der eigene Testlauf (sh -ec, Arbeitsordner des Projekts).
func testSkript(u umgebung.Umgebung, pfade []string) string {
	var muster []string
	for _, p := range pfade {
		p = strings.Trim(strings.TrimSpace(p), "/")
		if p == "" {
			continue
		}
		muster = append(muster, "-path "+shZitat("./"+strings.ReplaceAll(p, "**", "*")))
		if !strings.ContainsAny(p, "*?[") { // ein Ordner ohne Glob: alles darunter
			muster = append(muster, "-path "+shZitat("./"+p+"/*"))
		}
	}
	if len(muster) == 0 {
		muster = []string{"-path ./.swarvex-keine-pfade"}
	}
	ordner := "/work"
	if u.Ordner != "" {
		ordner += "/" + u.Ordner
	}
	var b strings.Builder
	b.WriteString("#!/bin/sh\n# swarvex: build and tests for the files you changed (SWARVEX_CHANGED), as the runner checks your result\n# afterwards. The full suite runs in the test pipeline.\n")
	b.WriteString("cd /work || exit 1\n")
	fmt.Fprintf(&b, "SWARVEX_CHANGED=$(find . -type f -newer %s/start \\( %s \\) 2>/dev/null | sed 's|^\\./||' | sort)\n", testHilfePfad, strings.Join(muster, " -o "))
	b.WriteString("export SWARVEX_CHANGED\n")
	b.WriteString("if [ -z \"$SWARVEX_CHANGED\" ]; then echo \"swarvex-test: nothing changed in your paths yet, so nothing to test\"; exit 0; fi\n")
	b.WriteString("echo \"swarvex-test: $(printf '%s\\n' \"$SWARVEX_CHANGED\" | grep -c .) changed file(s):\"; printf '  %s\\n' $SWARVEX_CHANGED\n")
	fmt.Fprintf(&b, "cd %s || exit 1\n", shZitat(ordner))
	if strings.TrimSpace(u.Bauen) != "" {
		fmt.Fprintf(&b, "sh -ec %s || exit 1\n", shZitat(u.Bauen))
	}
	fmt.Fprintf(&b, "exec sh -ec %s\n", shZitat(u.Testen))
	return b.String()
}

// testHilfeAnlegen: Skript und Startmarke im Aufgabenordner, für die Sandbox nur lesbar unter /run/swarvex-test.
// Die Startmarke entsteht direkt vor dem Agenten: Was Einrichten und Snapshot geschrieben haben, ist älter. Leer, wenn
// das Projekt keinen Testbefehl hat.
func testHilfeAnlegen(ordner string, a aufgabe, u umgebung.Umgebung) (string, error) {
	if strings.TrimSpace(u.Testen) == "" {
		return "", nil
	}
	hilfe := filepath.Join(ordner, "swarvex-test")
	if err := os.MkdirAll(hilfe, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(hilfe, "test.sh"), []byte(testSkript(u, a.Pfade)), 0o755); err != nil {
		return "", err
	}
	return hilfe, os.WriteFile(filepath.Join(hilfe, "start"), nil, 0o644)
}
