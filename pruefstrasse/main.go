// swarvex-pruefstrasse: Prüfdienst und Merge-Bot beim Visionär (Godot-Adapter).
//
//	swarvex-pruefstrasse pruefen   offene Einreichungen prüfen (Patch auf Basis-Commit, Tests im Container ohne Netz)
//	swarvex-pruefstrasse mergen    angenommene Einreichungen mit Herkunfts-Trailern in den Hauptzweig übernehmen
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
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	plattform = strings.TrimRight(os.Getenv("SWARVEX_PLATTFORM"), "/")
	geheimnis = os.Getenv("SWARVEX_PRUEFSTRASSE_GEHEIMNIS")
	caPfad    = os.Getenv("SWARVEX_CA")
	arbeit    = envOder("SWARVEX_ARBEIT", "/var/lib/swarvex-pruefstrasse")
	leseKey   = os.Getenv("SWARVEX_DEPLOY_KEY_LESEN")
	schreibKey = os.Getenv("SWARVEX_DEPLOY_KEY_SCHREIBEN")
	image     = envOder("SWARVEX_PRUEF_IMAGE", "swarvex-pruefung")
)

func main() {
	if len(os.Args) < 2 || plattform == "" || geheimnis == "" {
		fmt.Fprintln(os.Stderr, "Aufruf: swarvex-pruefstrasse pruefen|mergen  (SWARVEX_PLATTFORM, SWARVEX_PRUEFSTRASSE_GEHEIMNIS nötig)")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "pruefen":
		pruefen()
	case "mergen":
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
	req.Header.Set("x-zeit", zeit)
	req.Header.Set("x-signatur", hex.EncodeToString(mac.Sum(nil)))
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
	EinreichungID int64  `json:"einreichung_id"`
	AufgabeID     int64  `json:"aufgabe_id"`
	Titel         string `json:"titel"`
	Patch         string `json:"patch"`
	PatchSHA      string `json:"patch_sha256"`
	BasisCommit   string `json:"basis_commit"`
	Slug          string `json:"slug"`
	RepoGit       string `json:"repo_git"`
	Mensch        string `json:"mensch"`
	Agent         string `json:"agent"`
	Begruendung   string `json:"begruendung"`
	LeadAgent     string `json:"lead_agent"`
	Pruefung      string `json:"pruefung"`
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
	ziel := filepath.Join(arbeit, "spiegel", e.Slug+".git")
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

var testDatei = regexp.MustCompile(`^diff --git a/game/(tests/[^ ]+_test\.gd) b/`)

func pruefen() {
	var offen []einreichung
	if err := signiert("GET", "/pruefstrasse/offen", nil, &offen); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, e := range offen {
		fmt.Printf("Prüfe Einreichung %d (Aufgabe %d)\n", e.EinreichungID, e.AufgabeID)
		if err := signiert("POST", fmt.Sprintf("/pruefstrasse/%d/start", e.EinreichungID), map[string]any{}, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		ok, bericht := einePruefung(e)
		ergebnis := "fehler"
		if ok {
			ergebnis = "ok"
		}
		if len(bericht) > 20000 {
			bericht = bericht[len(bericht)-20000:]
		}
		if err := signiert("POST", fmt.Sprintf("/pruefstrasse/%d/ergebnis", e.EinreichungID), map[string]any{"ergebnis": ergebnis, "bericht": bericht}, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Printf("  → %s\n", ergebnis)
	}
}

func einePruefung(e einreichung) (bool, string) {
	var b strings.Builder
	summe := sha256.Sum256([]byte(e.Patch))
	if hex.EncodeToString(summe[:]) != e.PatchSHA {
		return false, "Patch-Prüfsumme stimmt nicht"
	}
	sp, err := spiegel(e)
	if err != nil {
		return false, err.Error()
	}
	dir, _ := os.MkdirTemp(filepath.Join(arbeit), "pruefung-")
	defer os.RemoveAll(dir)
	baum := filepath.Join(dir, "baum")
	// Basis-Commit mit echten Git-LFS-Dateien auschecken (Worktree am Spiegel).
	if aus, err := befehl(sp, gitEnv(leseKey), "git", "lfs", "fetch", "origin", e.BasisCommit); err != nil {
		return false, "LFS: " + aus
	}
	if aus, err := befehl(sp, gitEnv(leseKey), "git", "worktree", "add", "--detach", "--force", baum, e.BasisCommit); err != nil {
		return false, "Basis-Commit: " + aus
	}
	defer befehl(sp, nil, "git", "worktree", "remove", "--force", baum)
	patchDatei := filepath.Join(dir, "beitrag.patch")
	os.WriteFile(patchDatei, []byte(e.Patch), 0o644)
	if aus, err := befehl(baum, nil, "git", "apply", "--check", patchDatei); err != nil {
		return false, "Patch passt nicht auf den Basis-Commit:\n" + aus
	}
	befehl(baum, nil, "git", "apply", patchDatei)
	exec.Command("chmod", "-R", "a+rwX", baum).Run()

	// Prüfvertrag (Godot-Adapter): Import, dann jeder Test, den der Beitrag anlegt oder ändert.
	var tests []string
	for _, zeile := range strings.Split(e.Patch, "\n") {
		if m := testDatei.FindStringSubmatch(zeile); m != nil {
			tests = append(tests, "res://"+m[1])
		}
	}
	skript := "cd /work/game && godot --headless --import >/tmp/import.log 2>&1; echo \"import exit $?\"\n"
	for _, t := range tests {
		skript += fmt.Sprintf("echo '=== %s'; timeout 300 godot --headless --path . -s %s; echo \"exit $?\"\n", t, t)
	}
	if len(tests) == 0 {
		skript += "echo 'Keine Tests im Beitrag – Prüfvertrag verlangt mindestens einen.'; exit 3\n"
	}
	aus, _ := befehl("", nil, "docker", "run", "--rm", "--network", "none", "--read-only", "--cap-drop=ALL",
		"--security-opt", "no-new-privileges", "--tmpfs", "/tmp:rw,exec", "--tmpfs", "/home/pruefer:rw,exec",
		"--memory", "4g", "--pids-limit", "512", "-v", baum+":/work", image, "sh", "-c", skript)
	b.WriteString(aus)
	gruen := len(tests) > 0
	for _, zeile := range strings.Split(aus, "\n") {
		z := strings.TrimSpace(zeile)
		if strings.HasPrefix(z, "exit ") && z != "exit 0" {
			gruen = false
		}
		if strings.Contains(z, " FAIL ") || strings.HasPrefix(z, "SCRIPT ERROR") || strings.Contains(z, "Parse Error") {
			gruen = false
		}
	}
	return gruen, b.String()
}

func mergen() {
	var liste []einreichung
	if err := signiert("GET", "/pruefstrasse/angenommen", nil, &liste); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, e := range liste {
		if e.Pruefung != "ok" {
			fmt.Fprintf(os.Stderr, "Einreichung %d ohne grüne Prüfung – übersprungen\n", e.EinreichungID)
			continue
		}
		commit, err := einMerge(e)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Merge %d: %v\n", e.EinreichungID, err)
			continue
		}
		if err := signiert("POST", fmt.Sprintf("/pruefstrasse/%d/gemerged", e.EinreichungID), map[string]any{"commit": commit}, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Printf("Einreichung %d gemerged: %s\n", e.EinreichungID, commit)
	}
}

func einMerge(e einreichung) (string, error) {
	dir, _ := os.MkdirTemp(filepath.Join(arbeit), "merge-")
	defer os.RemoveAll(dir)
	env := gitEnv(schreibKey)
	if aus, err := befehl("", env, "git", "clone", "--quiet", "--depth", "50", e.RepoGit, dir); err != nil {
		return "", fmt.Errorf("clone: %s", aus)
	}
	patchDatei := filepath.Join(dir, ".git", "beitrag.patch")
	os.WriteFile(patchDatei, []byte(e.Patch), 0o644)
	if aus, err := befehl(dir, env, "git", "apply", "--index", "--3way", patchDatei); err != nil {
		return "", fmt.Errorf("apply: %s", aus)
	}
	nachricht := fmt.Sprintf("%s\n\nBeitrag über swarvex.ai.\n\nAufgabe: %s#%d\nBeitrag-von: %s (%s)\nPruefung: gruen (Einreichung %d)\nBewertet-von: %s — %s\nRechte-versichert-von: %s (Beitragserklärung bei Einreichung %d)\n",
		e.Titel, e.Slug, e.AufgabeID, e.Mensch, e.Agent, e.EinreichungID, e.LeadAgent, e.Begruendung, e.Mensch, e.EinreichungID)
	if aus, err := befehl(dir, append(env, "GIT_AUTHOR_NAME="+e.Mensch, "GIT_AUTHOR_EMAIL="+e.Mensch+"@swarvex.invalid",
		"GIT_COMMITTER_NAME=swarvex Merge-Bot", "GIT_COMMITTER_EMAIL=merge-bot@swarvex.invalid"),
		"git", "-c", "commit.gpgsign=false", "commit", "-q", "-m", nachricht); err != nil {
		return "", fmt.Errorf("commit: %s", aus)
	}
	if aus, err := befehl(dir, env, "git", "push", "-q", "origin", "HEAD"); err != nil {
		return "", fmt.Errorf("push: %s", aus)
	}
	aus, _ := befehl(dir, nil, "git", "rev-parse", "HEAD")
	return strings.TrimSpace(aus), nil
}
