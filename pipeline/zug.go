package main

// Merge-Zug: Angenommene Einreichungen eines Projekts kommen gemeinsam auf den aktuellen Stand von main und werden
// zusammen getestet, bevor sie gepusht werden. Jede Einreichung ist auf ihrem eigenen Basis-Commit geprüft; zwei
// Beiträge, die einzeln grün sind, können zusammen rot sein (oder einer auf dem neuen Stand). Grün: alle rein, mit
// einem Testlauf für bis zu zugGroesse Beiträge. Rot: halbieren, bis der Beitrag gefunden ist, der allein auf dem
// aktuellen Stand rot ist; er geht mit dem Testbericht und dem neuen Stand als Basis zurück in den Pool, der Rest
// wird gemerged. Lässt sich gar nicht testen (Umgebung, Docker), bleibt alles angenommen und der nächste Lauf
// versucht es wieder: das liegt nicht an den Beiträgen.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

const zugGroesse = 16

type zugErgebnis int

const (
	zugGruen  zugErgebnis = iota
	zugRot                // die Tests scheitern erst mit diesen Beiträgen
	zugFehler             // ließ sich nicht testen: später noch einmal
)

// zugStand: ein gebauter Zug (Klon mit einem Commit je Beitrag auf kopf).
type zugStand struct {
	dir     string
	kopf    string // Stand von main, auf dem der Zug steht
	spitze  string // letzter Commit des Zugs
	dabei   []einreichung
	commits []string
	umg     umgebung.Umgebung
}

// zugBahn: was der Zug von außen braucht (Git, Tests, Meldungen an die Plattform); im Test ersetzt.
type zugBahn interface {
	bauen(zug []einreichung) (*zugStand, error) // Konflikte und geschützte Dateien meldet bauen selbst und lässt sie weg
	testen(s *zugStand) (zugErgebnis, string)
	schieben(s *zugStand) error
	gemerged(e einreichung, commit string)
	rot(e einreichung, kopf, bericht string)
	aufraeumen(s *zugStand)
}

// zugFahren bringt einen Zug nach main. Rot wird halbiert, bis der Beitrag gefunden ist, der allein auf dem aktuellen
// Stand rot ist; die erste Hälfte fährt zuerst, die zweite danach auf dem dann neuen Stand.
func zugFahren(b zugBahn, zug []einreichung) {
	if len(zug) == 0 {
		return
	}
	s, err := b.bauen(zug)
	if err != nil {
		fmt.Fprintln(os.Stderr, "merge train:", err)
		return
	}
	defer b.aufraeumen(s)
	if len(s.dabei) == 0 {
		return
	}
	ergebnis, bericht := b.testen(s)
	switch {
	case ergebnis == zugFehler:
		fmt.Fprintf(os.Stderr, "merge train of %d could not be tested, trying again later:\n%s\n", len(s.dabei), letzteZeilen(bericht, 40))
	case ergebnis == zugGruen:
		if err := b.schieben(s); err != nil {
			fmt.Fprintln(os.Stderr, "merge train: push:", err)
			return
		}
		for i, e := range s.dabei {
			b.gemerged(e, s.commits[i])
		}
	case len(s.dabei) == 1:
		b.rot(s.dabei[0], s.kopf, bericht)
	default:
		dabei := append([]einreichung(nil), s.dabei...)
		h := len(dabei) / 2
		fmt.Printf("merge train of %d is red, splitting into %d and %d\n", len(dabei), h, len(dabei)-h)
		b.aufraeumen(s)
		zugFahren(b, dabei[:h])
		zugFahren(b, dabei[h:])
	}
}

func mergen() {
	var liste []einreichung
	if err := signiert("GET", "/pipeline/accepted", nil, &liste); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Je Projekt ein Zug, in der Reihenfolge der Einreichungen
	var reihenfolge []string
	zuege := map[string][]einreichung{}
	for _, e := range liste {
		if e.Pruefung != "pass" {
			fmt.Fprintf(os.Stderr, "submission %d has no passing check, skipped\n", e.EinreichungID)
			continue
		}
		if _, ok := zuege[e.Slug]; !ok {
			reihenfolge = append(reihenfolge, e.Slug)
		}
		if len(zuege[e.Slug]) < zugGroesse {
			zuege[e.Slug] = append(zuege[e.Slug], e)
		}
	}
	for _, slug := range reihenfolge {
		fmt.Printf("merge train %s: %d contributions\n", slug, len(zuege[slug]))
		zugFahren(neueBahn(), zuege[slug])
	}
}

type kopfLauf struct {
	gruen   bool
	ausgabe string
}

// echteBahn: Klon mit Schreibschlüssel für die Commits, eigener Spiegel (getrennt vom Prüfdienst) mit LFS für den Testbaum.
type echteBahn struct {
	kopfErgebnis map[string]kopfLauf // Tests auf main ohne Zug, je Stand nur einmal
	melden       func(pfad string, daten map[string]any) error
}

func neueBahn() *echteBahn {
	return &echteBahn{kopfErgebnis: map[string]kopfLauf{}, melden: func(pfad string, daten map[string]any) error {
		return signiert("POST", pfad, daten, nil)
	}}
}

func (b *echteBahn) bauen(zug []einreichung) (*zugStand, error) {
	e0 := zug[0]
	dir, _ := os.MkdirTemp(arbeit, "merge-")
	env := gitEnv(schreibKey, e0)
	if aus, err := befehl("", env, "git", "clone", "--quiet", "--depth", "50", e0.RepoGit, dir); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("clone: %s", aus)
	}
	kopf, _ := befehl(dir, nil, "git", "rev-parse", "HEAD")
	s := &zugStand{dir: dir, kopf: strings.TrimSpace(kopf)}
	// Projekteigene geschützte Pfade (swarvex.yaml protect:) aus dem Stand von main, nicht aus dem Zug
	u, err := umgebung.Ermitteln(dir)
	if err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("environment: %v", err)
	}
	s.umg = u
	for _, e := range zug {
		patchDatei := filepath.Join(dir, ".git", fmt.Sprintf("contribution-%d.patch", e.EinreichungID))
		os.WriteFile(patchDatei, []byte(e.Patch), 0o644)
		if aus, err := befehl(dir, env, "git", "apply", "--index", "--3way", patchDatei); err != nil {
			befehl(dir, nil, "git", "reset", "-q", "--hard", "HEAD")
			// Passt nicht mehr auf den neuen Stand: die Aufgabe geht mit diesem Stand als Basis zurück in den Pool
			if err := b.melden(fmt.Sprintf("/pipeline/%d/conflict", e.EinreichungID), map[string]any{"head": s.kopf, "report": kuerzenVorn(strings.TrimSpace(aus), 2000)}); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			fmt.Printf("submission %d conflicts with %s, back to the pool\n", e.EinreichungID, s.kopf)
			continue
		}
		// Zweite Schranke vor dem Push: geschützte Dateien nie in den Hauptzweig, egal wie der Patch geschrieben ist
		if p := geschuetztImIndex(dir, u); p != "" {
			befehl(dir, nil, "git", "reset", "-q", "--hard", "HEAD")
			b.rot(e, s.kopf, "refused before merging: the contribution changes the protected file "+p)
			continue
		}
		pruefung := fmt.Sprintf("pass (submission %d)", e.EinreichungID)
		if len(zug) > 1 {
			andere := "contributions"
			if len(zug) == 2 {
				andere = "contribution"
			}
			pruefung += fmt.Sprintf("; tested together with %d other accepted %s on %s", len(zug)-1, andere, kurz(s.kopf))
		}
		nachricht := fmt.Sprintf("%s\n\nContributed through swarvex.ai.\n\nTask: %s#%d\nContributed-by: %s (%s)\nCheck: %s\nReviewed-by: %s — %s\nRights-confirmed-by: %s (contribution statement with submission %d)\n",
			e.Titel, e.Slug, e.AufgabeID, e.Mensch, e.Agent, pruefung, e.LeadAgent, e.Begruendung, e.Mensch, e.EinreichungID)
		if aus, err := befehl(dir, append(env, "GIT_AUTHOR_NAME="+e.Mensch, "GIT_AUTHOR_EMAIL="+e.Mensch+"@swarvex.invalid",
			"GIT_COMMITTER_NAME=swarvex merge bot", "GIT_COMMITTER_EMAIL=merge-bot@swarvex.invalid"),
			"git", "-c", "commit.gpgsign=false", "commit", "-q", "-m", nachricht); err != nil {
			befehl(dir, nil, "git", "reset", "-q", "--hard", "HEAD")
			fmt.Fprintf(os.Stderr, "merge %d: commit: %s\n", e.EinreichungID, aus)
			continue
		}
		c, _ := befehl(dir, nil, "git", "rev-parse", "HEAD")
		s.dabei = append(s.dabei, e)
		s.commits = append(s.commits, strings.TrimSpace(c))
	}
	if len(s.commits) > 0 {
		s.spitze = s.commits[len(s.commits)-1]
	}
	return s, nil
}

// geschuetztImIndex: der erste vorgemerkte Pfad, den kein Beitrag ändern darf (feste Liste und swarvex.yaml protect:).
func geschuetztImIndex(dir string, u umgebung.Umgebung) string {
	cmd := exec.Command("git", "-c", "core.quotePath=false", "diff", "--cached", "--name-only", "--no-renames", "-z", "HEAD")
	cmd.Dir = dir
	aus, err := cmd.Output()
	if err != nil {
		return "(could not list the changed files)"
	}
	for _, p := range strings.Split(string(aus), "\x00") {
		if p != "" && (umgebung.IstGeschuetzt(p) || u.Gesperrt(p)) {
			return p
		}
	}
	return ""
}

func (b *echteBahn) testen(s *zugStand) (zugErgebnis, string) {
	if s.umg.Engine != nil {
		// Unity/Unreal: die Prüfstraße kann nicht testen; geprüft haben die Runner (eigener Lauf + Gegenprüfung)
		return zugGruen, ""
	}
	if strings.TrimSpace(s.umg.Testen) == "" {
		return zugGruen, ""
	}
	e := s.dabei[0]
	sp, err := spiegelIn(e, "merge-mirror")
	if err != nil {
		return zugFehler, err.Error()
	}
	if aus, err := befehl(sp, nil, "git", "fetch", "--quiet", s.dir, "+HEAD:refs/swarvex/train"); err != nil {
		return zugFehler, "fetching the train: " + aus
	}
	if aus, err := befehl(sp, gitEnv(leseKey, e), "git", "lfs", "fetch", "origin", s.kopf); err != nil {
		return zugFehler, "LFS: " + aus
	}
	ordner, _ := os.MkdirTemp(arbeit, "train-")
	defer os.RemoveAll(ordner)
	lauf := func(commit string) (zugErgebnis, string, string) {
		baum := filepath.Join(ordner, commit[:12])
		if aus, err := befehl(sp, gitEnv(leseKey, e), "git", "worktree", "add", "--detach", "--force", baum, commit); err != nil {
			return zugFehler, "checkout: " + aus, ""
		}
		defer befehl(sp, nil, "git", "worktree", "remove", "--force", baum)
		return projektTesten(baum, e.Slug)
	}
	var bericht strings.Builder
	fmt.Fprintf(&bericht, "Merge train: %d accepted contributions on main at %s\n", len(s.dabei), kurz(s.kopf))
	ergebnis, text, ausgabe := lauf(s.spitze)
	bericht.WriteString(text)
	if ergebnis != zugRot {
		return ergebnis, bericht.String()
	}
	// Gegenprobe: Ist main schon ohne den Zug rot, zählt nur, was der Zug neu kaputt macht
	k, ok := b.kopfErgebnis[s.kopf]
	if !ok {
		erg, _, aus := lauf(s.kopf)
		if erg == zugFehler {
			return zugFehler, bericht.String() + "\n=== Counter-check on main without the train: not possible\n"
		}
		k = kopfLauf{gruen: erg == zugGruen, ausgabe: aus}
		b.kopfErgebnis[s.kopf] = k
	}
	if k.gruen {
		bericht.WriteString("\n=== Counter-check on main without the train: passed\nThe tests fail only with these contributions on the current main.\n")
		return zugRot, bericht.String()
	}
	neu, schon := fehlschlaege(ausgabe), fehlschlaege(k.ausgabe)
	if len(neu) > 0 && len(schon) > 0 && !teilmenge(neu, schon) {
		fmt.Fprintf(&bericht, "\n=== Counter-check on main without the train: failed too, but on fewer tests\nNew failures with the train: %s\n", strings.Join(ohne(neu, schon), ", "))
		return zugRot, bericht.String()
	}
	bericht.WriteString("\n=== Counter-check on main without the train: failed too\nThe project's tests are already red on main; the train adds no new failures and is merged.\n")
	return zugGruen, bericht.String()
}

// projektTesten: Umgebung, Einrichten (mit Netz), Bauen und Testen (ohne Netz, zweiter Lauf bei Rot) in baum.
// Liefert das Ergebnis, den Bericht und die Ausgabe des letzten Testlaufs.
func projektTesten(baum, slug string) (zugErgebnis, string, string) {
	var b strings.Builder
	exec.Command("chmod", "-R", "a+rwX", baum).Run()
	u, err := umgebung.Ermitteln(baum)
	if err != nil {
		return zugFehler, "environment: " + err.Error(), ""
	}
	fmt.Fprintf(&b, "Environment: %s (%s)\n", u.Vorlage, u.Quelle)
	image, err := pruefImage(u, baum)
	if err != nil {
		return zugFehler, b.String() + err.Error(), ""
	}
	cache := filepath.Join(arbeit, "cache", slug)
	os.MkdirAll(cache, 0o777)
	os.Chmod(cache, 0o777)
	if strings.TrimSpace(u.Einrichten) != "" {
		if aus, err := imContainer(image, baum, cache, u, u.Einrichten, true, 1200); err != nil {
			return zugFehler, b.String() + "setup failed:\n" + letzteZeilen(aus, 60), ""
		}
	}
	if u.Bauen != "" {
		aus, err := imContainer(image, baum, cache, u, u.Bauen, false, 1800)
		fmt.Fprintf(&b, "=== Build\n%s\n", letzteZeilen(aus, 80))
		if err != nil {
			return zugRot, b.String() + "build failed", aus
		}
	}
	var letzte string
	ok := testen(&b, func() (string, error) {
		aus, err := imContainer(image, baum, cache, u, u.Testen, false, 1800)
		letzte = aus
		return aus, err
	})
	if ok {
		return zugGruen, b.String(), letzte
	}
	return zugRot, b.String(), letzte
}

func (b *echteBahn) schieben(s *zugStand) error {
	// Ohne --force: Hat sich main inzwischen bewegt, scheitert der Push, und der nächste Lauf baut den Zug neu.
	if aus, err := befehl(s.dir, gitEnv(schreibKey, s.dabei[0]), "git", "push", "-q", "origin", "HEAD"); err != nil {
		return errors.New(strings.TrimSpace(aus))
	}
	return nil
}

func (b *echteBahn) gemerged(e einreichung, commit string) {
	if err := b.melden(fmt.Sprintf("/pipeline/%d/merged", e.EinreichungID), map[string]any{"commit": commit}); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	fmt.Printf("submission %d merged: %s\n", e.EinreichungID, commit)
}

func (b *echteBahn) rot(e einreichung, kopf, bericht string) {
	if err := b.melden(fmt.Sprintf("/pipeline/%d/train-red", e.EinreichungID), map[string]any{"head": kopf, "report": kuerzenVorn(bericht, 20000)}); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	fmt.Printf("submission %d fails on %s, back to the pool with the report\n", e.EinreichungID, kurz(kopf))
}

func (b *echteBahn) aufraeumen(s *zugStand) {
	if s != nil && s.dir != "" {
		os.RemoveAll(s.dir)
		s.dir = ""
	}
}

// spiegelIn: Bare-Klon in einem eigenen Ordner je Dienst (Prüfdienst und Merge-Bot laufen gleichzeitig).
func spiegelIn(e einreichung, ordner string) (string, error) {
	ziel := filepath.Join(arbeit, ordner, e.Slug+".git")
	if _, err := os.Stat(ziel); err != nil {
		os.MkdirAll(filepath.Dir(ziel), 0o755)
		if aus, err := befehl("", gitEnv(leseKey, e), "git", "clone", "--quiet", "--bare", e.RepoGit, ziel); err != nil {
			return "", fmt.Errorf("clone: %s", aus)
		}
		befehl(ziel, nil, "git", "lfs", "install", "--local")
	} else if aus, err := befehl(ziel, nil, "git", "remote", "set-url", "origin", e.RepoGit); err != nil {
		return "", fmt.Errorf("remote: %s", aus)
	} else if aus, err := befehl(ziel, gitEnv(leseKey, e), "git", "fetch", "--quiet", "origin", "+refs/heads/*:refs/heads/*"); err != nil {
		return "", fmt.Errorf("fetch: %s", aus)
	}
	return ziel, nil
}

func teilmenge(a, b []string) bool {
	return len(ohne(a, b)) == 0
}

// ohne: die Einträge von a, die nicht in b stehen.
func ohne(a, b []string) []string {
	var rest []string
	for _, x := range a {
		if !gemeinsam([]string{x}, b) {
			rest = append(rest, x)
		}
	}
	return rest
}

func kurz(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func kuerzenVorn(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return text[len(text)-n:]
}

func letzteZeilen(text string, n int) string {
	zeilen := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(zeilen) > n {
		zeilen = zeilen[len(zeilen)-n:]
	}
	return strings.Join(zeilen, "\n")
}
