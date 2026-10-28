package main

// Eigener Testlauf und Gegenprüfung (E-31). Für Projekte mit proprietärer Engine (Unity, Unreal)
// kann die Prüfstraße nicht testen. Deshalb:
//   - Nach der Arbeit des Agenten führt der Runner den Testbefehl des Projekts selbst aus und
//     reicht das Ergebnis mit ein (own_test_result). Das gilt für alle Projekte; zählen tut es
//     nur bei Engine-Projekten.
//   - Hat der Runner gerade nichts zu tun und ist eine passende Engine installiert, prüft er die
//     Einreichung eines anderen Menschen gegen: Basis-Commit + Patch, Einrichten, Bauen, Testen
//     ohne Netz. Kein Modell, keine Tokens.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

// eigenerTestlauf: Bauen und Testen im Arbeitsbaum, ohne Netz. Liefert "pass"/"fail" (leer ohne
// Testbefehl) und das Ende der Ausgabe für die Zusammenfassung.
func eigenerTestlauf(ctx context.Context, u umgebung.Umgebung, image, baum, cache string) (string, string) {
	if strings.TrimSpace(u.Testen) == "" {
		return "", "no test command configured"
	}
	var bericht strings.Builder
	for _, schritt := range []struct{ name, befehl string }{{"build", u.Bauen}, {"test", u.Testen}} {
		if strings.TrimSpace(schritt.befehl) == "" {
			continue
		}
		start := time.Now()
		aus, err := ohneNetz(ctx, u, image, baum, cache, schritt.befehl, 3600)
		fmt.Fprintf(&bericht, "=== %s (%s)\n%s\n", schritt.name, time.Since(start).Round(time.Second), letzteZeilen(aus, 60))
		if err != nil {
			fmt.Fprintf(&bericht, "%s failed: %v\n", schritt.name, err)
			return "fail", bericht.String()
		}
	}
	return "pass", bericht.String()
}

type gegenpruefungsAuftrag struct {
	ID          int64  `json:"check_id"`
	Aufgabe     int64  `json:"task_id"`
	Einreichung int64  `json:"submission_id"`
	Titel       string `json:"title"`
	BasisCommit string `json:"base_commit"`
	Patch       string `json:"patch"`
	Engine      string `json:"engine"`
}

// gegenpruefenWennMoeglich fragt nach einer offenen Gegenprüfung für dieses Projekt und führt sie aus.
// true, wenn eine Gegenprüfung lief (dann nimmt dieser Aufruf keine Aufgabe mehr an).
func gegenpruefenWennMoeglich(ctx context.Context, m *mcpClient, z *zugangsdaten, projekt string) bool {
	var namen []string
	for _, i := range enginesFinden() {
		if !enthaelt(namen, i.Name) {
			namen = append(namen, i.Name)
		}
	}
	if len(namen) == 0 {
		return false
	}
	var job *gegenpruefungsAuftrag
	if err := m.werkzeug("next_check", map[string]any{"project": projekt, "engines": namen}, &job); err != nil || job == nil || job.ID == 0 {
		return false
	}
	fmt.Printf("Cross-check %d: submission %d of task %d (%s), no model involved\n", job.ID, job.Einreichung, job.Aufgabe, job.Titel)
	ergebnis, bericht := gegenpruefen(ctx, m, z, projekt, *job)
	if neu, err := zugriffstoken(z); err == nil {
		m.token = neu
	}
	if err := m.werkzeug("report_check", map[string]any{"check_id": job.ID, "result": ergebnis, "report": kuerzen(bericht, 19000)}, nil); err != nil {
		fmt.Fprintln(os.Stderr, "reporting the cross-check:", err)
	}
	fmt.Printf("Cross-check %d: %s\n", job.ID, ergebnis)
	return true
}

// gegenpruefen: Snapshot des Basis-Commits, Patch anwenden, Umgebung mit eigener Engine, Tests ohne Netz.
// Ergebnis "pass", "fail" oder "unable" (z. B. Engine-Version fehlt, Snapshot nicht ladbar).
func gegenpruefen(ctx context.Context, m *mcpClient, z *zugangsdaten, projekt string, job gegenpruefungsAuftrag) (string, string) {
	ordner := filepath.Join(datenOrdner(), "check", fmt.Sprint(job.ID))
	os.RemoveAll(ordner)
	defer os.RemoveAll(ordner)
	baum, gitDir := filepath.Join(ordner, "tree"), filepath.Join(ordner, "git")
	os.MkdirAll(baum, 0o755)
	if err := snapshotHolen(z, m.token, job.Aufgabe, baum); err != nil {
		return "unable", "snapshot: " + err.Error()
	}
	lfsPfade, err := lfsAufloesen(z, m.token, job.Aufgabe, baum)
	if err != nil {
		return "unable", "LFS: " + err.Error()
	}
	if err := gitBasis(gitDir, baum, lfsPfade); err != nil {
		return "unable", "base commit: " + err.Error()
	}
	patchDatei := filepath.Join(ordner, "submission.patch")
	os.WriteFile(patchDatei, []byte(job.Patch), 0o644)
	if aus, err := git(gitDir, baum, "apply", "--check", patchDatei); err != nil {
		return "fail", "patch does not apply to the base commit:\n" + aus + err.Error()
	}
	if _, err := git(gitDir, baum, "apply", patchDatei); err != nil {
		return "fail", err.Error()
	}
	// Die Umgebung kommt aus dem Basis-Stand samt Beitrag; geschützte Dateien kann ein Beitrag nicht ändern.
	u, err := umgebungErmitteln(baum)
	if err != nil {
		return "unable", "environment: " + err.Error()
	}
	if u.Engine == nil {
		return "unable", "the project does not use a proprietary engine; the test pipeline checks it"
	}
	image, err := umgebungsImage(ctx, u, baum, "")
	if err != nil {
		return "unable", err.Error()
	}
	cache := cacheOrdner(projekt)
	if err := einrichten(ctx, u, image, baum, cache, filepath.Join(ordner, "setup.log")); err != nil {
		return "unable", err.Error()
	}
	ergebnis, bericht := eigenerTestlauf(ctx, u, image, baum, cache)
	if ergebnis == "" {
		return "unable", bericht
	}
	return ergebnis, fmt.Sprintf("Environment: %s, %s\n%s", u.Vorlage, u.Engine, bericht)
}

func enthaelt(liste []string, s string) bool {
	for _, x := range liste {
		if x == s {
			return true
		}
	}
	return false
}

func kuerzen(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return "…\n" + text[len(text)-n:]
}
