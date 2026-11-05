// swarvex-pipeline: Prüfdienst und Merge-Bot beim Visionär. Die Umgebung des Projekts
// (swarvex.yaml, Dev Container oder erkannte Projektart) kommt aus dem Paket umgebung, wie beim Runner.
//
//	swarvex-pipeline check   offene Einreichungen prüfen (Patch auf Basis-Commit, Tests im Container ohne Netz)
//	swarvex-pipeline merge   angenommene Einreichungen mit Herkunfts-Trailern in den Hauptzweig übernehmen
//	swarvex-pipeline preview spielbare Web-Vorschau des Hauptzweigs bauen, wenn er sich geändert hat (vorschau.go)
//
// Getrennte Schlüssel: Der Prüfdienst liest nur (Deploy-Key read-only); allein der Merge-Bot
// darf schreiben. Alle Meldungen an die Plattform sind HMAC-signiert.
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

var (
	plattform  = strings.TrimRight(os.Getenv("SWARVEX_PLATFORM"), "/")
	geheimnis  = os.Getenv("SWARVEX_PIPELINE_SECRET")
	caPfad     = os.Getenv("SWARVEX_CA")
	arbeit     = envOder("SWARVEX_WORKDIR", "/var/lib/swarvex-pipeline")
	leseKey    = os.Getenv("SWARVEX_DEPLOY_KEY_READ")
	schreibKey = os.Getenv("SWARVEX_DEPLOY_KEY_WRITE")
)

func main() {
	if len(os.Args) < 2 || plattform == "" || geheimnis == "" {
		fmt.Fprintln(os.Stderr, "usage: swarvex-pipeline check|merge|preview  (requires SWARVEX_PLATFORM, SWARVEX_PIPELINE_SECRET)")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "check":
		pruefen()
	case "merge":
		mergen()
	case "preview":
		vorschauen()
	default:
		os.Exit(2)
	}
}

func envOder(n, s string) string {
	if v := os.Getenv(n); v != "" {
		return v
	}
	return s
}

func client() *http.Client {
	pool, _ := x509.SystemCertPool()
	if pool == nil {
		pool = x509.NewCertPool()
	}
	if pem, err := os.ReadFile(caPfad); err == nil {
		pool.AppendCertsFromPEM(pem)
	}
	return &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}
}

// signiert ruft die Prüfstraßen-API mit HMAC-Signatur auf (Zeit + Methode + Pfad + Körper).
func signiert(methode, pfad string, koerper any, aus any) error {
	var roh []byte
	if koerper != nil {
		roh, _ = json.Marshal(koerper)
	}
	zeit := strconv.FormatInt(time.Now().UnixMilli(), 10)
	zufall := make([]byte, 16)
	rand.Read(zufall)
	nonce := hex.EncodeToString(zufall) // jede Anfrage nur einmal gültig (die Plattform merkt sich die Nummer)
	mac := hmac.New(sha256.New, []byte(geheimnis))
	fmt.Fprintf(mac, "%s\n%s\n%s\n%s\n%s", zeit, nonce, methode, pfad, roh)
	req, _ := http.NewRequest(methode, plattform+pfad, bytes.NewReader(roh))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-time", zeit)
	req.Header.Set("x-nonce", nonce)
	req.Header.Set("x-signature", hex.EncodeToString(mac.Sum(nil)))
	r, err := client().Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	antwort, _ := io.ReadAll(r.Body)
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: HTTP %d %s", methode, pfad, r.StatusCode, antwort)
	}
	if aus != nil {
		return json.Unmarshal(antwort, aus)
	}
	return nil
}

type einreichung struct {
	EinreichungID int64  `json:"submission_id"`
	AufgabeID     int64  `json:"task_id"`
	Titel         string `json:"title"`
	Patch         string `json:"patch"`
	PatchSHA      string `json:"patch_sha256"`
	BasisCommit   string `json:"base_commit"`
	Slug          string `json:"slug"`
	RepoGit       string `json:"repo_git"`
	GitToken      string `json:"git_token"` // GitHub App: Token nur für dieses Repo (sonst Deploy-Key)
	Mensch        string `json:"person"`
	Agent         string `json:"agent"`
	Begruendung   string `json:"reason"`
	LeadAgent     string `json:"lead_agent"`
	Pruefung      string `json:"check"`
}

func befehl(dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	aus, err := cmd.CombinedOutput()
	return string(aus), err
}

func gitEnv(key string, e einreichung) []string {
	if e.GitToken != "" { // als HTTP-Header, nicht in der Adresse oder Konfiguration; git-lfs liest ihn mit
		kopf := "AUTHORIZATION: basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+e.GitToken))
		return []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.https://github.com/.extraheader", "GIT_CONFIG_VALUE_0=" + kopf, "GIT_CONFIG_NOSYSTEM=1"}
	}
	return []string{"GIT_SSH_COMMAND=ssh -i " + key + " -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", "GIT_CONFIG_NOSYSTEM=1"}
}

// spiegel hält einen Bare-Klon aktuell (nur lesend).
func spiegel(e einreichung) (string, error) {
	ziel := filepath.Join(arbeit, "mirror", e.Slug+".git")
	if _, err := os.Stat(ziel); err != nil {
		os.MkdirAll(filepath.Dir(ziel), 0o755)
		if aus, err := befehl("", gitEnv(leseKey, e), "git", "clone", "--quiet", "--bare", e.RepoGit, ziel); err != nil {
			return "", fmt.Errorf("clone: %s", aus)
		}
		befehl(ziel, nil, "git", "lfs", "install", "--local") // Checkout löst LFS-Zeiger auf
	} else if aus, err := befehl(ziel, nil, "git", "remote", "set-url", "origin", e.RepoGit); err != nil {
		return "", fmt.Errorf("remote: %s", aus)
	} else if aus, err := befehl(ziel, gitEnv(leseKey, e), "git", "fetch", "--quiet", "origin", "+refs/heads/*:refs/heads/*"); err != nil {
		return "", fmt.Errorf("fetch: %s", aus)
	}
	return ziel, nil
}

func pruefen() {
	var offen []einreichung
	if err := signiert("GET", "/pipeline/pending", nil, &offen); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, e := range offen {
		fmt.Printf("Checking submission %d (task %d)\n", e.EinreichungID, e.AufgabeID)
		if err := signiert("POST", fmt.Sprintf("/pipeline/%d/start", e.EinreichungID), map[string]any{}, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		ok, bericht, basisRot := einePruefung(e)
		ergebnis := "fail"
		if ok {
			ergebnis = "pass"
		}
		if len(bericht) > 20000 {
			bericht = bericht[len(bericht)-20000:]
		}
		if err := signiert("POST", fmt.Sprintf("/pipeline/%d/result", e.EinreichungID), map[string]any{"result": ergebnis, "report": bericht, "base_red": basisRot}, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Printf("  → %s\n", ergebnis)
	}
}

func einePruefung(e einreichung) (ok bool, bericht string, basisRot bool) {
	var b strings.Builder
	summe := sha256.Sum256([]byte(e.Patch))
	if hex.EncodeToString(summe[:]) != e.PatchSHA {
		return false, "patch checksum does not match", false
	}
	sp, err := spiegel(e)
	if err != nil {
		return false, err.Error(), false
	}
	dir, _ := os.MkdirTemp(filepath.Join(arbeit), "check-")
	defer os.RemoveAll(dir)
	baum := filepath.Join(dir, "tree")
	// Basis-Commit mit echten Git-LFS-Dateien auschecken (Worktree am Spiegel).
	if aus, err := befehl(sp, gitEnv(leseKey, e), "git", "lfs", "fetch", "origin", e.BasisCommit); err != nil {
		return false, "LFS: " + aus, false
	}
	if aus, err := befehl(sp, gitEnv(leseKey, e), "git", "worktree", "add", "--detach", "--force", baum, e.BasisCommit); err != nil {
		return false, "base commit: " + aus, false
	}
	defer befehl(sp, nil, "git", "worktree", "remove", "--force", baum)
	patchDatei := filepath.Join(dir, "contribution.patch")
	os.WriteFile(patchDatei, []byte(e.Patch), 0o644)
	if aus, err := befehl(baum, nil, "git", "apply", "--check", patchDatei); err != nil {
		return false, "patch does not apply to the base commit:\n" + aus, false
	}
	befehl(baum, nil, "git", "apply", patchDatei)
	// Maßgeblich ist, was sich im Baum wirklich geändert hat, nicht was im Text des Patches steht.
	if p, err := geschuetztGeaendert(baum); err != nil || p != "" {
		if err != nil {
			return false, "could not list the changed files: " + err.Error(), false
		}
		return false, "contribution changes a protected file: " + p, false
	}
	exec.Command("chmod", "-R", "a+rwX", baum).Run()

	// Prüfvertrag: Umgebung des Projekts (auf dem Stand des Basis-Commits samt Beitrag), Abhängigkeiten mit
	// Netz laden, dann Bauen und Testen ohne Netz. Geschützte Dateien darf ein Beitrag nicht ändern.
	u, err := umgebung.Ermitteln(baum)
	if err != nil {
		return false, "environment: " + err.Error(), false
	}
	fmt.Fprintf(&b, "Environment: %s (%s)\n", u.Vorlage, u.Quelle)
	if u.Engine != nil {
		// Proprietäre Engines prüfen die Runner der Mitstreiter (eigener Lauf + Gegenprüfung, E-31).
		return false, b.String() + fmt.Sprintf("this project needs %s; the platform has contributor runners check it, not the test pipeline", u.Engine), false
	}
	// Projekteigene geschützte Pfade (swarvex.yaml protect:) kennt erst die Umgebung
	if pfade, err := geaendertePfade(baum); err == nil {
		for _, p := range pfade {
			if u.Gesperrt(p) {
				return false, b.String() + "contribution changes a protected file: " + p, false
			}
		}
	}
	image, err := pruefImage(u, baum)
	if err != nil {
		return false, b.String() + err.Error(), false
	}
	cache := filepath.Join(arbeit, "cache", e.Slug)
	os.MkdirAll(cache, 0o777)
	os.Chmod(cache, 0o777)
	if strings.TrimSpace(u.Einrichten) != "" {
		aus, err := imContainer(image, baum, cache, u, u.Einrichten, true, 1200)
		if err != nil {
			return false, b.String() + "setup failed:\n" + aus, false
		}
	}
	if u.Bauen != "" {
		aus, err := imContainer(image, baum, cache, u, u.Bauen, false, 1800)
		fmt.Fprintf(&b, "=== Build\n%s\n", aus)
		if err != nil {
			return false, b.String() + "build failed", false
		}
	}
	if u.Testen == "" {
		b.WriteString("No automated tests configured (swarvex.yaml: test). The lead reviews the contribution by hand.\n")
		return true, b.String(), false
	}
	if testen(&b, func() (string, error) { return imContainer(image, baum, cache, u, u.Testen, false, 1800) }) {
		return true, b.String(), false
	}
	basisRot = gegenprobe(&b, func() (string, bool, error) {
		if aus, err := befehl(baum, nil, "git", "apply", "-R", patchDatei); err != nil {
			return aus, false, err
		}
		if u.Bauen != "" {
			if aus, err := imContainer(image, baum, cache, u, u.Bauen, false, 1800); err != nil {
				return "build failed:\n" + aus, false, nil
			}
		}
		aus, err := imContainer(image, baum, cache, u, u.Testen, false, 1800)
		return aus, err == nil, nil
	})
	return basisRot, b.String(), basisRot
}

// gegenprobe: Bleiben die Tests auch im zweiten Lauf rot, laufen sie auf dem Basis-Commit ohne den Beitrag. Sind sie
// dort ebenfalls rot, ist nicht der Beitrag schuld, sondern das Projekt: Er geht in die Bewertung, der Lead erfährt
// es. Abgelehnt wird nur, was erst durch den Beitrag rot wird. Liefert true, wenn der Stand ohne Beitrag rot ist.
func gegenprobe(b *strings.Builder, ohneBeitrag func() (aus string, bestanden bool, err error)) bool {
	aus, bestanden, err := ohneBeitrag()
	switch {
	case err != nil:
		fmt.Fprintf(b, "=== Counter-check without the contribution: not possible (%v)\n%s\n", err, aus)
		return false
	case bestanden:
		b.WriteString("=== Counter-check without the contribution: passed\nThe tests fail only with this contribution.\n")
		return false
	}
	fmt.Fprintf(b, "=== Counter-check without the contribution: failed too\nThe project's tests are already red at the base commit, "+
		"so they do not count against this contribution; it goes on to review, and the lead is told.\n%s\n", aus)
	return true
}

// testen: ein zweiter Lauf, wenn der erste scheitert. Wackelige Tests (Zeitlimits, Last auf dem Server) fallen sonst
// zufällig auf Beiträge, die sie gar nicht berühren; was ein Beitrag wirklich kaputt macht, scheitert auch beim
// zweiten Mal. Beide Läufe stehen im Bericht, damit der Lead die wackeligen Tests sieht.
func testen(b *strings.Builder, lauf func() (string, error)) bool {
	aus, err := lauf()
	fmt.Fprintf(b, "=== Test\n%s\n", aus)
	if err == nil {
		return true
	}
	aus, err = lauf()
	if err == nil {
		fmt.Fprintf(b, "=== Test, second run: passed\nThe first run failed and the second passed: tests above fail only sometimes. "+
			"The lead should fix or quarantine them.\n%s\n", aus)
		return true
	}
	fmt.Fprintf(b, "=== Test, second run: failed again\n%s\n", aus)
	return false
}

// pruefImage baut die Projekt-Umgebung ohne Agent-Schicht (einmal je Basis).
func pruefImage(u umgebung.Umgebung, baum string) (string, error) {
	name := "swarvex-check-" + u.Kennung(baum)
	if _, err := befehl("", nil, "docker", "image", "inspect", name); err == nil {
		return name, nil
	}
	basis := ""
	if u.Dockerfile != "" {
		basis = "swarvex-base-" + u.Kennung(baum)
		if aus, err := befehl("", nil, "docker", "build", "-q", "-t", basis, "-f", filepath.Join(baum, u.Dockerfile), filepath.Join(baum, u.Kontext)); err != nil {
			return "", fmt.Errorf("building the project's Dockerfile: %s", aus)
		}
	}
	leer, _ := os.MkdirTemp(arbeit, "build-")
	defer os.RemoveAll(leer)
	cmd := exec.Command("docker", "build", "-q", "-t", name, "-f", "-", leer)
	cmd.Stdin = strings.NewReader(u.Containerfile(basis, ""))
	if aus, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("building the environment: %s", aus)
	}
	return name, nil
}

// imContainer führt einen Befehl der Umgebung aus: mit Netz nur zum Einrichten, sonst ohne.
func imContainer(image, baum, cache string, u umgebung.Umgebung, skript string, mitNetz bool, sekunden int, extra ...string) (string, error) {
	netz := "none"
	if mitNetz {
		netz = "bridge"
	}
	args := []string{"run", "--rm", "--network", netz, "--read-only", "--cap-drop=ALL", "--security-opt", "no-new-privileges",
		"--user", "1000:1000", "--tmpfs", "/tmp:rw,exec,size=2g", "--tmpfs", "/home/agent:rw,exec,size=1g",
		"--memory", "4g", "--pids-limit", "1024", "-v", baum + ":/work", "-v", cache + ":/cache", "-w", filepath.Join("/work", u.Ordner)}
	for _, v := range umgebung.CacheVariablen {
		args = append(args, "-e", v)
	}
	args = append(append(args, extra...), image, "timeout", fmt.Sprint(sekunden), "sh", "-ec", skript)
	return befehl("", nil, "docker", args...)
}

func mergen() {
	var liste []einreichung
	if err := signiert("GET", "/pipeline/accepted", nil, &liste); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, e := range liste {
		if e.Pruefung != "pass" {
			fmt.Fprintf(os.Stderr, "submission %d has no passing check, skipped\n", e.EinreichungID)
			continue
		}
		commit, err := einMerge(e)
		var k *konflikt
		if errors.As(err, &k) {
			// Passt nicht mehr auf den neuen Stand: die Aufgabe geht mit diesem Stand als Basis zurück in den Pool
			if err := signiert("POST", fmt.Sprintf("/pipeline/%d/conflict", e.EinreichungID), map[string]any{"head": k.kopf, "report": k.bericht}, nil); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			fmt.Printf("submission %d conflicts with %s, back to the pool\n", e.EinreichungID, k.kopf)
			continue
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "merge %d: %v\n", e.EinreichungID, err)
			continue
		}
		if err := signiert("POST", fmt.Sprintf("/pipeline/%d/merged", e.EinreichungID), map[string]any{"commit": commit}, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Printf("submission %d merged: %s\n", e.EinreichungID, commit)
	}
}

// konflikt: der Patch lässt sich auf den aktuellen Stand (kopf) nicht mehr anwenden.
type konflikt struct{ kopf, bericht string }

func (k *konflikt) Error() string { return "conflict with " + k.kopf + ": " + k.bericht }

func einMerge(e einreichung) (string, error) {
	dir, _ := os.MkdirTemp(filepath.Join(arbeit), "merge-")
	defer os.RemoveAll(dir)
	env := gitEnv(schreibKey, e)
	if aus, err := befehl("", env, "git", "clone", "--quiet", "--depth", "50", e.RepoGit, dir); err != nil {
		return "", fmt.Errorf("clone: %s", aus)
	}
	patchDatei := filepath.Join(dir, ".git", "contribution.patch")
	os.WriteFile(patchDatei, []byte(e.Patch), 0o644)
	if aus, err := befehl(dir, env, "git", "apply", "--index", "--3way", patchDatei); err != nil {
		kopf, _ := befehl(dir, nil, "git", "rev-parse", "HEAD")
		bericht := aus
		if len(bericht) > 2000 {
			bericht = bericht[:2000]
		}
		return "", &konflikt{kopf: strings.TrimSpace(kopf), bericht: strings.TrimSpace(bericht)}
	}
	// Zweite Schranke vor dem Push: geschützte Dateien nie in den Hauptzweig, egal wie der Patch geschrieben ist
	if p, err := geschuetztGeaendert(dir); err != nil || p != "" {
		if err != nil {
			return "", fmt.Errorf("listing the changed files: %v", err)
		}
		return "", fmt.Errorf("refused: the contribution changes the protected file %s", p)
	}
	nachricht := fmt.Sprintf("%s\n\nContributed through swarvex.ai.\n\nTask: %s#%d\nContributed-by: %s (%s)\nCheck: pass (submission %d)\nReviewed-by: %s — %s\nRights-confirmed-by: %s (contribution statement with submission %d)\n",
		e.Titel, e.Slug, e.AufgabeID, e.Mensch, e.Agent, e.EinreichungID, e.LeadAgent, e.Begruendung, e.Mensch, e.EinreichungID)
	if aus, err := befehl(dir, append(env, "GIT_AUTHOR_NAME="+e.Mensch, "GIT_AUTHOR_EMAIL="+e.Mensch+"@swarvex.invalid",
		"GIT_COMMITTER_NAME=swarvex merge bot", "GIT_COMMITTER_EMAIL=merge-bot@swarvex.invalid"),
		"git", "-c", "commit.gpgsign=false", "commit", "-q", "-m", nachricht); err != nil {
		return "", fmt.Errorf("commit: %s", aus)
	}
	if aus, err := befehl(dir, env, "git", "push", "-q", "origin", "HEAD"); err != nil {
		return "", fmt.Errorf("push: %s", aus)
	}
	aus, _ := befehl(dir, nil, "git", "rev-parse", "HEAD")
	return strings.TrimSpace(aus), nil
}

// geaendertePfade: alle Dateien, die sich im Baum gegenüber HEAD geändert haben (angelegt, geändert, gelöscht,
// umbenannt als Löschen plus Anlegen), so wie Git sie sieht.
func geaendertePfade(baum string) ([]string, error) {
	if aus, err := befehl(baum, nil, "git", "add", "-A"); err != nil {
		return nil, fmt.Errorf("git add: %s", aus)
	}
	cmd := exec.Command("git", "-c", "core.quotePath=false", "diff", "--cached", "--name-only", "--no-renames", "-z", "HEAD")
	cmd.Dir = baum
	aus, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var pfade []string
	for _, p := range strings.Split(string(aus), "\x00") {
		if p != "" {
			pfade = append(pfade, p)
		}
	}
	return pfade, nil
}

// geschuetztGeaendert: der erste geänderte Pfad, den kein Beitrag ändern darf (feste Liste), sonst "".
func geschuetztGeaendert(baum string) (string, error) {
	pfade, err := geaendertePfade(baum)
	if err != nil {
		return "", err
	}
	for _, p := range pfade {
		if umgebung.IstGeschuetzt(p) {
			return p, nil
		}
	}
	return "", nil
}
