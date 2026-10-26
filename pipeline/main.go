// swarvex-pipeline: Prüfdienst und Merge-Bot beim Visionär. Die Umgebung des Projekts
// (swarvex.yaml, Dev Container oder erkannte Projektart) kommt aus dem Paket umgebung, wie beim Runner.
//
//	swarvex-pipeline check   offene Einreichungen prüfen (Patch auf Basis-Commit, Tests im Container ohne Netz)
//	swarvex-pipeline merge   angenommene Einreichungen mit Herkunfts-Trailern in den Hauptzweig übernehmen
//
// Getrennte Schlüssel: Der Prüfdienst liest nur (Deploy-Key read-only); allein der Merge-Bot
// darf schreiben. Alle Meldungen an die Plattform sind HMAC-signiert.
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
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
		fmt.Fprintln(os.Stderr, "usage: swarvex-pipeline check|merge  (requires SWARVEX_PLATFORM, SWARVEX_PIPELINE_SECRET)")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "check":
		pruefen()
	case "merge":
		mergen()
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
	mac := hmac.New(sha256.New, []byte(geheimnis))
	fmt.Fprintf(mac, "%s\n%s\n%s\n%s", zeit, methode, pfad, roh)
	req, _ := http.NewRequest(methode, plattform+pfad, bytes.NewReader(roh))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-time", zeit)
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

func gitEnv(key string) []string {
	return []string{"GIT_SSH_COMMAND=ssh -i " + key + " -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", "GIT_CONFIG_NOSYSTEM=1"}
}

// spiegel hält einen Bare-Klon aktuell (nur lesend).
func spiegel(e einreichung) (string, error) {
	ziel := filepath.Join(arbeit, "mirror", e.Slug+".git")
	if _, err := os.Stat(ziel); err != nil {
		os.MkdirAll(filepath.Dir(ziel), 0o755)
		if aus, err := befehl("", gitEnv(leseKey), "git", "clone", "--quiet", "--bare", e.RepoGit, ziel); err != nil {
			return "", fmt.Errorf("clone: %s", aus)
		}
		befehl(ziel, nil, "git", "lfs", "install", "--local") // Checkout löst LFS-Zeiger auf
	} else if aus, err := befehl(ziel, gitEnv(leseKey), "git", "fetch", "--quiet", "origin", "+refs/heads/*:refs/heads/*"); err != nil {
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
		ok, bericht := einePruefung(e)
		ergebnis := "fail"
		if ok {
			ergebnis = "pass"
		}
		if len(bericht) > 20000 {
			bericht = bericht[len(bericht)-20000:]
		}
		if err := signiert("POST", fmt.Sprintf("/pipeline/%d/result", e.EinreichungID), map[string]any{"result": ergebnis, "report": bericht}, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Printf("  → %s\n", ergebnis)
	}
}

func einePruefung(e einreichung) (bool, string) {
	var b strings.Builder
	summe := sha256.Sum256([]byte(e.Patch))
	if hex.EncodeToString(summe[:]) != e.PatchSHA {
		return false, "patch checksum does not match"
	}
	sp, err := spiegel(e)
	if err != nil {
		return false, err.Error()
	}
	dir, _ := os.MkdirTemp(filepath.Join(arbeit), "check-")
	defer os.RemoveAll(dir)
	baum := filepath.Join(dir, "tree")
	// Basis-Commit mit echten Git-LFS-Dateien auschecken (Worktree am Spiegel).
	if aus, err := befehl(sp, gitEnv(leseKey), "git", "lfs", "fetch", "origin", e.BasisCommit); err != nil {
		return false, "LFS: " + aus
	}
	if aus, err := befehl(sp, gitEnv(leseKey), "git", "worktree", "add", "--detach", "--force", baum, e.BasisCommit); err != nil {
		return false, "base commit: " + aus
	}
	defer befehl(sp, nil, "git", "worktree", "remove", "--force", baum)
	patchDatei := filepath.Join(dir, "contribution.patch")
	os.WriteFile(patchDatei, []byte(e.Patch), 0o644)
	if aus, err := befehl(baum, nil, "git", "apply", "--check", patchDatei); err != nil {
		return false, "patch does not apply to the base commit:\n" + aus
	}
	befehl(baum, nil, "git", "apply", patchDatei)
	exec.Command("chmod", "-R", "a+rwX", baum).Run()

	// Prüfvertrag: Umgebung des Projekts (auf dem Stand des Basis-Commits samt Beitrag), Abhängigkeiten mit
	// Netz laden, dann Bauen und Testen ohne Netz. Geschützte Dateien darf ein Beitrag nicht ändern.
	u, err := umgebung.Ermitteln(baum)
	if err != nil {
		return false, "environment: " + err.Error()
	}
	fmt.Fprintf(&b, "Environment: %s (%s)\n", u.Vorlage, u.Quelle)
	for _, zeile := range strings.Split(e.Patch, "\n") {
		if teile := strings.Fields(strings.TrimPrefix(zeile, "diff --git ")); strings.HasPrefix(zeile, "diff --git ") && len(teile) == 2 {
			for _, p := range teile {
				if pfad := strings.TrimPrefix(strings.TrimPrefix(p, "a/"), "b/"); u.Gesperrt(pfad) {
					return false, b.String() + "contribution changes a protected file: " + pfad
				}
			}
		}
	}
	image, err := pruefImage(u, baum)
	if err != nil {
		return false, b.String() + err.Error()
	}
	cache := filepath.Join(arbeit, "cache", e.Slug)
	os.MkdirAll(cache, 0o777)
	os.Chmod(cache, 0o777)
	if strings.TrimSpace(u.Einrichten) != "" {
		aus, err := imContainer(image, baum, cache, u, u.Einrichten, true, 1200)
		if err != nil {
			return false, b.String() + "setup failed:\n" + aus
		}
	}
	if u.Bauen != "" {
		aus, err := imContainer(image, baum, cache, u, u.Bauen, false, 1800)
		fmt.Fprintf(&b, "=== Build\n%s\n", aus)
		if err != nil {
			return false, b.String() + "build failed"
		}
	}
	if u.Testen == "" {
		b.WriteString("No automated tests configured (swarvex.yaml: test). The lead reviews the contribution by hand.\n")
		return true, b.String()
	}
	aus, err := imContainer(image, baum, cache, u, u.Testen, false, 1800)
	fmt.Fprintf(&b, "=== Test\n%s\n", aus)
	return err == nil, b.String()
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
func imContainer(image, baum, cache string, u umgebung.Umgebung, skript string, mitNetz bool, sekunden int) (string, error) {
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
	args = append(args, image, "timeout", fmt.Sprint(sekunden), "sh", "-ec", skript)
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

func einMerge(e einreichung) (string, error) {
	dir, _ := os.MkdirTemp(filepath.Join(arbeit), "merge-")
	defer os.RemoveAll(dir)
	env := gitEnv(schreibKey)
	if aus, err := befehl("", env, "git", "clone", "--quiet", "--depth", "50", e.RepoGit, dir); err != nil {
		return "", fmt.Errorf("clone: %s", aus)
	}
	patchDatei := filepath.Join(dir, ".git", "contribution.patch")
	os.WriteFile(patchDatei, []byte(e.Patch), 0o644)
	if aus, err := befehl(dir, env, "git", "apply", "--index", "--3way", patchDatei); err != nil {
		return "", fmt.Errorf("apply: %s", aus)
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
