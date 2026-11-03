package main

// Vorschau nach jedem Merge: Für Projekte mit Web-Vorschau (Godot mit Web-Export-Preset, oder preview und preview_dir
// in swarvex.yaml) und mit Doku oder Bericht (docs und docs_dir: API-Doku, Blueprint, Ergebnisbericht, Storybook) baut die
// Prüfstraße den aktuellen Stand des Hauptzweigs ohne Netz und legt das Ergebnis unter <arbeit>/vorschau/<slug>/ ab,
// die Doku unter docs/. Caddy liefert es auf einer eigenen Adresse aus (infra/stack/Caddyfile), nie unter der
// Adresse der Plattform. Unity, Unreal und native Builds gibt es hier nicht: Ihre Editoren und Lizenzen bleiben auf den
// Rechnern der Mitstreiter.

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

const vorschauMaxBytes = 300 << 20 // so groß darf eine Vorschau höchstens sein

type vorschauProjekt struct {
	Slug     string `json:"slug"`
	RepoGit  string `json:"repo_git"`
	GitToken string `json:"git_token"`
	Commit   string `json:"last_commit"` // Stand der letzten Vorschau
}

func vorschauen() {
	var liste []vorschauProjekt
	if err := signiert("GET", "/pipeline/previews", nil, &liste); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, p := range liste {
		e := einreichung{Slug: p.Slug, RepoGit: p.RepoGit, GitToken: p.GitToken}
		sp, err := spiegel(e)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p.Slug, err)
			continue
		}
		kopf, err := befehl(sp, nil, "git", "rev-parse", "HEAD")
		if kopf = strings.TrimSpace(kopf); err != nil || kopf == p.Commit {
			continue
		}
		st := eineVorschau(e, sp, kopf)
		if len(st.bericht) > 4000 {
			st.bericht = st.bericht[len(st.bericht)-4000:]
		}
		if err := signiert("POST", "/pipeline/previews/"+p.Slug, map[string]any{"commit": kopf, "status": st.status, "report": st.bericht, "bytes": st.groesse,
			"preview": st.spiel, "docs": st.doku}, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Printf("preview %s at %.12s: %s (preview %v, docs %v)\n", p.Slug, kopf, st.status, st.spiel, st.doku)
	}
	einreichungenVorschauen()
}

// vorschauStand: Ergebnis eines Baus; Spiel und Doku sagen, was die Projektseite verlinkt.
type vorschauStand struct {
	status, bericht string
	groesse         int64
	spiel, doku     bool
}

// auschecken legt einen Arbeitsbaum des Stands an (mit Git-LFS); aufraeumen entfernt ihn wieder.
func auschecken(e einreichung, sp, stand string) (baum string, aufraeumen func(), err error) {
	dir, _ := os.MkdirTemp(arbeit, "preview-")
	baum = filepath.Join(dir, "tree")
	aufraeumen = func() { befehl(sp, nil, "git", "worktree", "remove", "--force", baum); os.RemoveAll(dir) }
	if aus, err := befehl(sp, gitEnv(leseKey, e), "git", "lfs", "fetch", "origin", stand); err != nil {
		return baum, aufraeumen, fmt.Errorf("LFS: %s", aus)
	}
	if aus, err := befehl(sp, nil, "git", "worktree", "add", "--detach", "--force", baum, stand); err != nil {
		return baum, aufraeumen, fmt.Errorf("checkout: %s", aus)
	}
	return baum, aufraeumen, nil
}

// eineVorschau baut Vorschau (Wurzel) und Doku (/docs/) des Stands kopf und tauscht sie zusammen ein.
func eineVorschau(e einreichung, sp, kopf string) vorschauStand {
	baum, aufraeumen, err := auschecken(e, sp, kopf)
	defer aufraeumen()
	if err != nil {
		return vorschauStand{status: "failed", bericht: err.Error()}
	}
	return vorschauBauen(e, baum, filepath.Join(arbeit, "vorschau", e.Slug), true)
}

// vorschauBauen baut im ausgecheckten Baum die Vorschau und, wenn gewünscht, die Doku und tauscht sie unter ziel ein.
func vorschauBauen(e einreichung, baum, ziel string, mitDoku bool) vorschauStand {
	os.Chmod(baum, 0o777)
	befehl("", nil, "chmod", "-R", "a+rwX", baum)
	u, err := umgebung.Ermitteln(baum)
	if !mitDoku {
		u.Doku = ""
	}
	switch {
	case err != nil:
		return vorschauStand{status: "failed", bericht: "environment: " + err.Error()}
	case u.Engine != nil:
		return vorschauStand{status: "none", bericht: fmt.Sprintf("%s projects get no preview on the platform: the editor and its licence stay on contributors' machines.", u.Engine)}
	case u.Vorschau == "" && u.Doku == "":
		return vorschauStand{status: "none"}
	}
	image, err := pruefImage(u, baum)
	if err != nil {
		return vorschauStand{status: "failed", bericht: err.Error()}
	}
	var extra []string
	if u.Godot != "" && u.Vorschau != "" {
		vorlagen, err := godotVorlagen(u.Godot)
		if err != nil {
			return vorschauStand{status: "failed", bericht: "Godot export templates: " + err.Error()}
		}
		extra = []string{"-v", vorlagen + ":/home/agent/.local/share/godot/export_templates/" + u.Godot + ".stable:ro"}
	}
	cache := filepath.Join(arbeit, "cache", e.Slug)
	os.MkdirAll(cache, 0o777)
	os.Chmod(cache, 0o777)
	var b strings.Builder
	if strings.TrimSpace(u.Einrichten) != "" {
		if aus, err := imContainer(image, baum, cache, u, u.Einrichten, true, 1200); err != nil {
			return vorschauStand{status: "failed", bericht: "setup failed:\n" + aus}
		}
	}
	os.MkdirAll(filepath.Dir(ziel), 0o755)
	neu, alt := ziel+".neu", ziel+".alt"
	os.RemoveAll(neu)
	defer os.RemoveAll(neu)
	st := vorschauStand{}
	// Teil bauen und an seinen Platz im neuen Stand kopieren
	bauen := func(titel, skript, ordner, nach string, args []string) error {
		aus, err := imContainer(image, baum, cache, u, skript, false, 1800, args...)
		fmt.Fprintf(&b, "=== %s\n%s\n", titel, aus)
		if err != nil {
			return fmt.Errorf("%s failed", strings.ToLower(titel))
		}
		quelle := filepath.Join(baum, u.Ordner, ordner)
		if s, err := os.Stat(filepath.Join(quelle, "index.html")); err != nil || !s.Mode().IsRegular() {
			return fmt.Errorf("%s/index.html was not created", ordner)
		}
		n, err := vorschauKopieren(quelle, nach, vorschauMaxBytes-st.groesse)
		st.groesse += n
		return err
	}
	if u.Vorschau != "" {
		if err := bauen("Preview build", u.Vorschau, u.VorschauIn, neu, extra); err != nil {
			return vorschauStand{status: "failed", bericht: b.String() + err.Error()}
		}
		st.spiel = true
	}
	if u.Doku != "" {
		os.RemoveAll(filepath.Join(neu, "docs")) // /docs/ gehört der Doku
		if err := bauen("Docs build", u.Doku, u.DokuIn, filepath.Join(neu, "docs"), nil); err != nil {
			return vorschauStand{status: "failed", bericht: b.String() + err.Error()}
		}
		st.doku = true
		if !st.spiel { // nur Doku: die Wurzel führt dorthin
			os.WriteFile(filepath.Join(neu, "index.html"), []byte(`<!doctype html><meta charset="utf-8"><meta http-equiv="refresh" content="0; url=docs/"><a href="docs/">docs</a>`+"\n"), 0o644)
		}
	}
	// Tauschen statt überschreiben: Wer gerade spielt oder liest, bekommt nie einen halben Stand.
	os.RemoveAll(alt)
	os.Rename(ziel, alt)
	if err := os.Rename(neu, ziel); err != nil {
		os.Rename(alt, ziel)
		return vorschauStand{status: "failed", bericht: b.String() + err.Error()}
	}
	os.RemoveAll(alt)
	st.status, st.bericht = "ok", b.String()
	return st
}

// Vorschau je Einreichung: geprüfte Einreichungen, die auf Bewertung warten, werden auf ihrem Basis-Commit samt Patch
// gebaut und unter vorschau/review/<id>/ abgelegt (review.<Domain>/<id>/). Der Code ist noch nicht bewertet, deshalb nur
// auf der Vorschau-Domain und nie unter der Adresse eines Projekts. Entschiedene Einreichungen verschwinden wieder.
func einreichungenVorschauen() {
	var r struct {
		Offen    []einreichung `json:"open"`
		Behalten []int64       `json:"keep"`
	}
	if err := signiert("GET", "/pipeline/submission-previews", nil, &r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	for _, e := range r.Offen {
		st := eineEinreichungsVorschau(e)
		if len(st.bericht) > 4000 {
			st.bericht = st.bericht[len(st.bericht)-4000:]
		}
		if err := signiert("POST", fmt.Sprintf("/pipeline/submission-previews/%d", e.EinreichungID), map[string]any{"status": st.status, "report": st.bericht, "bytes": st.groesse}, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Printf("submission preview %d (%s): %s\n", e.EinreichungID, e.Slug, st.status)
		if st.status == "ok" {
			r.Behalten = append(r.Behalten, e.EinreichungID)
		}
	}
	// aufräumen: nur Vorschauen von Einreichungen, die noch auf Bewertung warten
	behalten := map[string]bool{}
	for _, id := range r.Behalten {
		behalten[fmt.Sprint(id)] = true
	}
	eintraege, _ := os.ReadDir(filepath.Join(arbeit, "vorschau", "review"))
	for _, d := range eintraege {
		if !behalten[d.Name()] {
			os.RemoveAll(filepath.Join(arbeit, "vorschau", "review", d.Name()))
		}
	}
}

func eineEinreichungsVorschau(e einreichung) vorschauStand {
	summe := sha256.Sum256([]byte(e.Patch))
	if hex.EncodeToString(summe[:]) != e.PatchSHA {
		return vorschauStand{status: "failed", bericht: "patch checksum does not match"}
	}
	sp, err := spiegel(e)
	if err != nil {
		return vorschauStand{status: "failed", bericht: err.Error()}
	}
	baum, aufraeumen, err := auschecken(e, sp, e.BasisCommit)
	defer aufraeumen()
	if err != nil {
		return vorschauStand{status: "failed", bericht: err.Error()}
	}
	patchDatei := filepath.Join(filepath.Dir(baum), "contribution.patch")
	os.WriteFile(patchDatei, []byte(e.Patch), 0o644)
	if aus, err := befehl(baum, nil, "git", "apply", patchDatei); err != nil {
		return vorschauStand{status: "failed", bericht: "patch does not apply:\n" + aus}
	}
	return vorschauBauen(e, baum, filepath.Join(arbeit, "vorschau", "review", fmt.Sprint(e.EinreichungID)), false)
}

// vorschauKopieren übernimmt nur gewöhnliche Dateien und Ordner (keine Links, die aus der Vorschau hinauszeigen könnten)
// und bricht ab, wenn die Vorschau zu groß wird.
func vorschauKopieren(quelle, ziel string, rest int64) (int64, error) {
	var summe int64
	err := filepath.WalkDir(quelle, func(pfad string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(quelle, pfad)
		nach := filepath.Join(ziel, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(nach, 0o755)
		case !d.Type().IsRegular():
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if summe += info.Size(); summe > rest {
			return fmt.Errorf("the preview is larger than %d MB", vorschauMaxBytes>>20)
		}
		von, err := os.Open(pfad)
		if err != nil {
			return err
		}
		defer von.Close()
		f, err := os.OpenFile(nach, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(f, von)
		return err
	})
	return summe, err
}

// godotVorlagen lädt die Web-Export-Vorlagen einer Godot-Version einmal auf den Host (aus dem offiziellen Paket mit allen
// Vorlagen nur die für Web) und liefert den Ordner, der nur lesend in den Container kommt.
func godotVorlagen(voll string) (string, error) {
	ordner := filepath.Join(arbeit, "godot-templates", voll+".stable")
	if treffer, _ := filepath.Glob(filepath.Join(ordner, "web*.zip")); len(treffer) > 0 {
		return ordner, nil
	}
	os.MkdirAll(filepath.Dir(ordner), 0o755)
	paket, err := os.CreateTemp(arbeit, "godot-templates-*.tpz")
	if err != nil {
		return "", err
	}
	defer os.Remove(paket.Name())
	adresse := fmt.Sprintf("https://github.com/godotengine/godot/releases/download/%s-stable/Godot_v%s-stable_export_templates.tpz", voll, voll)
	antwort, err := http.Get(adresse)
	if err != nil {
		return "", err
	}
	defer antwort.Body.Close()
	if antwort.StatusCode != 200 {
		return "", fmt.Errorf("%s: %s", adresse, antwort.Status)
	}
	if _, err := io.Copy(paket, io.LimitReader(antwort.Body, 4<<30)); err != nil {
		return "", err
	}
	paket.Close()
	z, err := zip.OpenReader(paket.Name())
	if err != nil {
		return "", err
	}
	defer z.Close()
	neu := ordner + ".neu"
	os.RemoveAll(neu)
	os.MkdirAll(neu, 0o755)
	for _, f := range z.File {
		name := strings.TrimPrefix(f.Name, "templates/")
		if strings.Contains(name, "/") || !(strings.HasPrefix(name, "web") || name == "version.txt") {
			continue
		}
		von, err := f.Open()
		if err != nil {
			return "", err
		}
		nach, err := os.Create(filepath.Join(neu, name))
		if err == nil {
			_, err = io.Copy(nach, von)
			nach.Close()
		}
		von.Close()
		if err != nil {
			return "", err
		}
	}
	if treffer, _ := filepath.Glob(filepath.Join(neu, "web*.zip")); len(treffer) == 0 {
		return "", fmt.Errorf("no web templates in %s", adresse)
	}
	befehl("", nil, "chmod", "-R", "a+rX", neu)
	return ordner, os.Rename(neu, ordner)
}
