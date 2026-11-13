package main

// Tests des Merge-Zugs auf dem Prüfserver. Die Plattform baut den Zug (nur sie hat Schlüssel, die in ein Repo
// schreiben) und schickt per SSH eine Zeile JSON (zugAuftrag) und danach ein Git-Bundle mit den Commits des Zugs. Ihr
// Schlüssel darf dort nur „swarvex-pipeline train-test“ ausführen (infra/setup-pipeline-host.sh). Der Prüfserver holt
// main mit seinem eigenen Lesezugriff (/pipeline/heads), testet unter gVisor und antwortet mit einer Zeile JSON
// (zugAntwort). Beide Seiten trauen der anderen nur so weit: Größen sind begrenzt, Ergebnis und Commits werden geprüft.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const (
	zugKopfMax    = 4 << 20   // Auftragszeile (mit der Ausgabe der Gegenprobe)
	zugBundleMax  = 200 << 20 // Commits von höchstens zugGroesse Beiträgen
	zugAntwortMax = 4 << 20
	zugBerichtMax = 100 << 10
	zugAusgabeMax = 2 << 20
)

type zugAuftrag struct {
	Probe    bool      `json:"probe,omitempty"` // nur „bist du bereit?“ (infra/connect-pipeline-host.sh)
	Slug     string    `json:"slug"`
	Kopf     string    `json:"head"` // Stand von main, auf dem der Zug steht
	Spitze   string    `json:"tip"`  // letzter Commit des Zugs (im Bundle)
	Anzahl   int       `json:"contributions"`
	KopfLauf *kopfLauf `json:"head_run,omitempty"` // Gegenprobe auf main, falls schon gelaufen
}

type zugAntwort struct {
	Ergebnis zugErgebnis `json:"result"` // 0 grün, 1 rot, 2 nicht testbar
	Bericht  string      `json:"report"`
	KopfLauf *kopfLauf   `json:"head_run,omitempty"`
}

var (
	slugMuster   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,99}$`)
	commitMuster = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
)

func (a zugAuftrag) pruefen() error {
	switch {
	case !slugMuster.MatchString(a.Slug):
		return errors.New("invalid project")
	case !commitMuster.MatchString(a.Kopf) || !commitMuster.MatchString(a.Spitze):
		return errors.New("invalid commit")
	case a.Anzahl < 1 || a.Anzahl > zugGroesse:
		return errors.New("invalid number of contributions")
	}
	return nil
}

// --- Plattform ---

// sshZugTest: ein Testlauf auf dem Prüfserver. Ein Fehler hier (Server weg, Verbindung abgerissen) heißt „nicht
// testbar“: Der Zug bleibt angenommen, und der nächste Lauf versucht es wieder.
func sshZugTest(a zugAuftrag, bundle string) (zugAntwort, error) {
	ein, err := zugAnfrage(a, bundle)
	if err != nil {
		return zugAntwort{}, err
	}
	defer ein.Close()
	cmd := exec.Command("ssh", "-T", "-i", zugTestsKey, "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=yes",
		"-o", "UserKnownHostsFile="+zugTestsKey+".known_hosts", "-o", "ServerAliveInterval=30", "-o", "ServerAliveCountMax=10", zugTestsAuf)
	cmd.Stdin = ein
	aus := &begrenzt{max: zugAntwortMax}
	cmd.Stdout = aus
	cmd.Stderr = os.Stderr // Fortschritt dort ins Journal hier
	if err := cmd.Run(); err != nil {
		return zugAntwort{}, fmt.Errorf("ssh %s: %v", zugTestsAuf, err)
	}
	return zugAntwortLesen(aus.b.Bytes(), aus.voll)
}

// zugAnfrage: Auftragszeile und Bundle als ein Strom.
func zugAnfrage(a zugAuftrag, bundle string) (io.ReadCloser, error) {
	kopf, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	if bundle == "" {
		return io.NopCloser(bytes.NewReader(append(kopf, '\n'))), nil
	}
	f, err := os.Open(bundle)
	if err != nil {
		return nil, err
	}
	return struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(append(kopf, '\n')), f), f}, nil
}

func zugAntwortLesen(b []byte, zuGross bool) (zugAntwort, error) {
	if zuGross {
		return zugAntwort{}, errors.New("answer too large")
	}
	// Das Ergebnis muss dastehen: Fehlte es, wäre es 0, und 0 heißt grün
	var roh struct {
		Ergebnis *zugErgebnis `json:"result"`
		Bericht  string       `json:"report"`
		KopfLauf *kopfLauf    `json:"head_run"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(b), &roh); err != nil {
		return zugAntwort{}, fmt.Errorf("unreadable answer: %v", err)
	}
	if roh.Ergebnis == nil {
		return zugAntwort{}, errors.New("answer without result")
	}
	switch *roh.Ergebnis {
	case zugGruen, zugRot, zugFehler:
	default:
		return zugAntwort{}, fmt.Errorf("unknown result %d", *roh.Ergebnis)
	}
	a := zugAntwort{Ergebnis: *roh.Ergebnis, Bericht: kuerzenVorn(roh.Bericht, zugBerichtMax), KopfLauf: roh.KopfLauf}
	if a.KopfLauf != nil {
		a.KopfLauf.Ausgabe = kuerzenVorn(a.KopfLauf.Ausgabe, zugAusgabeMax)
	}
	return a, nil
}

// begrenzt: nimmt höchstens max Bytes an und merkt sich, ob mehr kam.
type begrenzt struct {
	b    bytes.Buffer
	max  int
	voll bool
}

func (w *begrenzt) Write(p []byte) (int, error) {
	if rest := w.max - w.b.Len(); len(p) > rest {
		w.voll = true
		if rest > 0 {
			w.b.Write(p[:rest])
		}
		return len(p), nil
	}
	return w.b.Write(p)
}

// --- Prüfserver ---

// zugTestAnnehmen: erzwungener Befehl des Zug-Schlüssels der Plattform. Antwortet immer mit einer Zeile JSON; was
// nicht stimmt, heißt „nicht testbar“.
func zugTestAnnehmen(r io.Reader, w io.Writer, suchen func(slug string) (einreichung, error)) {
	antwort := func() zugAntwort {
		br := bufio.NewReaderSize(r, 64<<10)
		zeile, err := zeileLesen(br, zugKopfMax)
		if err != nil {
			return nichtTestbar("reading the request: " + err.Error())
		}
		var a zugAuftrag
		if err := json.Unmarshal(zeile, &a); err != nil {
			return nichtTestbar("unreadable request")
		}
		if a.Probe {
			return zugAntwort{Ergebnis: zugGruen, Bericht: "pipeline server ready, container runtime " + envOder("SWARVEX_RUNTIME", "runc")}
		}
		if err := a.pruefen(); err != nil {
			return nichtTestbar(err.Error())
		}
		if a.KopfLauf != nil {
			a.KopfLauf.Ausgabe = kuerzenVorn(a.KopfLauf.Ausgabe, zugAusgabeMax)
		}
		f, err := os.CreateTemp(arbeit, "train-*.bundle")
		if err != nil {
			return nichtTestbar(err.Error())
		}
		defer os.Remove(f.Name())
		n, err := io.Copy(f, io.LimitReader(br, zugBundleMax+1))
		f.Close()
		if err != nil || n == 0 || n > zugBundleMax {
			return nichtTestbar("train bundle missing or too large")
		}
		return zugTestAusfuehren(a, f.Name(), suchen)
	}()
	// Ohne HTML-Escaping und auf die Größe nach der Kodierung begrenzt: Escapes machten aus 2 MiB Ausgabe mehr als die
	// 4 MiB, die die Plattform liest, und der Zug galt dann jedes Mal als nicht testbar (Sicherheitsaudit 12.11.2026)
	antwortSchreiben(w, antwort)
}

func antwortSchreiben(w io.Writer, a zugAntwort) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(antwortBegrenzt(a))
}

// antwortBegrenzt kürzt Kopflauf-Ausgabe und Bericht, bis die kodierte Antwort sicher unter zugAntwortMax liegt.
func antwortBegrenzt(a zugAntwort) zugAntwort {
	for {
		roh, _ := json.Marshal(a) // obere Schranke: Marshal escapt HTML, der Encoder oben nicht
		if len(roh) < zugAntwortMax-4096 {
			return a
		}
		if a.KopfLauf != nil && a.KopfLauf.Ausgabe != "" {
			k := *a.KopfLauf
			k.Ausgabe = kuerzenVorn(k.Ausgabe, len(k.Ausgabe)/2)
			a.KopfLauf = &k
			continue
		}
		if a.Bericht == "" {
			return a
		}
		a.Bericht = kuerzenVorn(a.Bericht, len(a.Bericht)/2)
	}
}

// zugTestAusfuehren: Bundle in den eigenen Spiegel, prüfen, dass es genau der angekündigte Zug auf main ist, testen.
func zugTestAusfuehren(a zugAuftrag, bundle string, suchen func(slug string) (einreichung, error)) zugAntwort {
	e, err := suchen(a.Slug)
	if err != nil {
		return nichtTestbar(err.Error())
	}
	fmt.Fprintf(os.Stderr, "train test %s: %d contributions on %s (runtime %s)\n", a.Slug, a.Anzahl, kurz(a.Kopf), envOder("SWARVEX_RUNTIME", "runc"))
	sp, err := spiegelIn(e, "merge-mirror")
	if err != nil {
		return nichtTestbar(err.Error())
	}
	if aus, err := befehl(sp, nil, "git", "fetch", "--quiet", bundle, "+HEAD:refs/swarvex/train"); err != nil {
		return nichtTestbar("fetching the train: " + letzteZeilen(aus, 5))
	}
	if spitze, _ := befehl(sp, nil, "git", "rev-parse", "refs/swarvex/train"); strings.TrimSpace(spitze) != a.Spitze {
		return nichtTestbar("the bundle does not end at the announced commit")
	}
	if _, err := befehl(sp, nil, "git", "merge-base", "--is-ancestor", a.Kopf, a.Spitze); err != nil {
		return nichtTestbar("the train does not sit on main at " + kurz(a.Kopf))
	}
	ergebnis, bericht, k := zugTesten(sp, e, a.Kopf, a.Spitze, a.Anzahl, a.KopfLauf)
	// Gekürzt, damit die Antwort sicher unter zugAntwortMax bleibt (sonst gälte jeder Zug als nicht testbar)
	if k != nil {
		k.Ausgabe = kuerzenVorn(k.Ausgabe, zugAusgabeMax)
	}
	return zugAntwort{Ergebnis: ergebnis, Bericht: kuerzenVorn(bericht, zugBerichtMax), KopfLauf: k}
}

// projektAusKoepfen: Repo und Lesezugriff des Projekts, wie für Vorschauen (der Prüfserver bekommt nie einen
// schreibenden Token).
func projektAusKoepfen(slug string) (einreichung, error) {
	var liste []vorschauProjekt
	if err := signiert("GET", "/pipeline/heads", nil, &liste); err != nil {
		return einreichung{}, err
	}
	for _, p := range liste {
		if p.Slug == slug {
			return einreichung{Slug: p.Slug, RepoGit: p.RepoGit, GitToken: p.GitToken}, nil
		}
	}
	return einreichung{}, fmt.Errorf("project %s unknown", slug)
}

func nichtTestbar(grund string) zugAntwort {
	return zugAntwort{Ergebnis: zugFehler, Bericht: "pipeline server: " + grund}
}

// zeileLesen: eine Zeile bis höchstens max Bytes.
func zeileLesen(br *bufio.Reader, max int) ([]byte, error) {
	var zeile []byte
	for {
		teil, err := br.ReadSlice('\n')
		zeile = append(zeile, teil...)
		if len(zeile) > max {
			return nil, errors.New("request line too long")
		}
		if err == nil {
			return bytes.TrimSpace(zeile), nil
		}
		if err != bufio.ErrBufferFull {
			return nil, err
		}
	}
}
