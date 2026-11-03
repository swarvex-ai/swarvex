package main

// Vorschau nach jedem Merge: Für Projekte mit Web-Vorschau (Godot mit Web-Export-Preset, oder preview und preview_dir
// in swarvex.yaml) baut die Prüfstraße den aktuellen Stand des Hauptzweigs ohne Netz und legt das Ergebnis unter
// <arbeit>/vorschau/<slug>/ ab. Caddy liefert es auf einer eigenen Adresse aus (infra/stack/Caddyfile), nie unter der
// Adresse der Plattform. Unity, Unreal und native Builds gibt es hier nicht: Ihre Editoren und Lizenzen bleiben auf den
// Rechnern der Mitstreiter.

import (
	"archive/zip"
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
		status, bericht, groesse := eineVorschau(e, sp, kopf)
		if len(bericht) > 4000 {
			bericht = bericht[len(bericht)-4000:]
		}
		if err := signiert("POST", "/pipeline/previews/"+p.Slug, map[string]any{"commit": kopf, "status": status, "report": bericht, "bytes": groesse}, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Printf("preview %s at %.12s: %s\n", p.Slug, kopf, status)
	}
}

// eineVorschau liefert den Status (ok, failed, none), den Bericht und die Größe in Bytes.
func eineVorschau(e einreichung, sp, kopf string) (string, string, int64) {
	dir, _ := os.MkdirTemp(arbeit, "preview-")
	defer os.RemoveAll(dir)
	baum := filepath.Join(dir, "tree")
	if aus, err := befehl(sp, gitEnv(leseKey, e), "git", "lfs", "fetch", "origin", kopf); err != nil {
		return "failed", "LFS: " + aus, 0
	}
	if aus, err := befehl(sp, nil, "git", "worktree", "add", "--detach", "--force", baum, kopf); err != nil {
		return "failed", "checkout: " + aus, 0
	}
	defer befehl(sp, nil, "git", "worktree", "remove", "--force", baum)
	os.Chmod(baum, 0o777)
	befehl("", nil, "chmod", "-R", "a+rwX", baum)

	u, err := umgebung.Ermitteln(baum)
	switch {
	case err != nil:
		return "failed", "environment: " + err.Error(), 0
	case u.Engine != nil:
		return "none", fmt.Sprintf("%s projects get no preview on the platform: the editor and its licence stay on contributors' machines.", u.Engine), 0
	case u.Vorschau == "":
		return "none", "", 0
	}
	image, err := pruefImage(u, baum)
	if err != nil {
		return "failed", err.Error(), 0
	}
	var extra []string
	if u.Godot != "" {
		vorlagen, err := godotVorlagen(u.Godot)
		if err != nil {
			return "failed", "Godot export templates: " + err.Error(), 0
		}
		extra = []string{"-v", vorlagen + ":/home/agent/.local/share/godot/export_templates/" + u.Godot + ".stable:ro"}
	}
	cache := filepath.Join(arbeit, "cache", e.Slug)
	os.MkdirAll(cache, 0o777)
	os.Chmod(cache, 0o777)
	var b strings.Builder
	if strings.TrimSpace(u.Einrichten) != "" {
		if aus, err := imContainer(image, baum, cache, u, u.Einrichten, true, 1200); err != nil {
			return "failed", "setup failed:\n" + aus, 0
		}
	}
	aus, err := imContainer(image, baum, cache, u, u.Vorschau, false, 1800, extra...)
	fmt.Fprintf(&b, "=== Preview build\n%s\n", aus)
	if err != nil {
		return "failed", b.String() + "preview build failed", 0
	}
	quelle := filepath.Join(baum, u.Ordner, u.VorschauIn)
	if st, err := os.Stat(filepath.Join(quelle, "index.html")); err != nil || !st.Mode().IsRegular() {
		return "failed", b.String() + fmt.Sprintf("%s/index.html was not created", u.VorschauIn), 0
	}
	ziel := filepath.Join(arbeit, "vorschau", e.Slug)
	neu, alt := ziel+".neu", ziel+".alt"
	os.RemoveAll(neu)
	groesse, err := vorschauKopieren(quelle, neu)
	if err != nil {
		os.RemoveAll(neu)
		return "failed", b.String() + err.Error(), 0
	}
	// Tauschen statt überschreiben: Wer gerade spielt, bekommt nie eine halbe Vorschau.
	os.RemoveAll(alt)
	os.Rename(ziel, alt)
	if err := os.Rename(neu, ziel); err != nil {
		os.Rename(alt, ziel)
		return "failed", b.String() + err.Error(), 0
	}
	os.RemoveAll(alt)
	return "ok", b.String(), groesse
}

// vorschauKopieren übernimmt nur gewöhnliche Dateien und Ordner (keine Links, die aus der Vorschau hinauszeigen könnten)
// und bricht ab, wenn die Vorschau zu groß wird.
func vorschauKopieren(quelle, ziel string) (int64, error) {
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
		if summe += info.Size(); summe > vorschauMaxBytes {
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
