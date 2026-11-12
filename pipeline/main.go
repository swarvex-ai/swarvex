// swarvex-pipeline: Prüfdienst und Merge-Bot beim Visionär. Die Umgebung des Projekts
// (swarvex.yaml, Dev Container oder erkannte Projektart) kommt aus dem Paket umgebung, wie beim Runner.
//
//	swarvex-pipeline check   offene Einreichungen prüfen (Patch auf Basis-Commit, Tests im Container ohne Netz)
//	swarvex-pipeline merge   angenommene Einreichungen als Merge-Zug zusammen testen und mit Herkunfts-Trailern in den Hauptzweig übernehmen (zug.go)
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
	"regexp"
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
	// Eigener Prüfserver (infra/setup-pipeline-host.sh): Dort laufen check und preview (SWARVEX_JOBS=check,preview) mit
	// gVisor um jeden Container (SWARVEX_RUNTIME=runsc), auf der Plattform nur merge (SWARVEX_JOBS=merge); deren preview
	// holt dann die fertigen Vorschauen nur lesend vom Prüfserver (SWARVEX_PREVIEW_FROM, SWARVEX_PREVIEW_KEY).
	// Leer: alles auf diesem Rechner, ohne zweite Laufzeit (Labor, Selbsthoster mit einem Server).
	jobs        = os.Getenv("SWARVEX_JOBS")
	laufzeit    = os.Getenv("SWARVEX_RUNTIME")
	vorschauVon = os.Getenv("SWARVEX_PREVIEW_FROM")
	vorschauKey = os.Getenv("SWARVEX_PREVIEW_KEY")
)

func main() {
	if len(os.Args) < 2 || plattform == "" || geheimnis == "" {
		fmt.Fprintln(os.Stderr, "usage: swarvex-pipeline check|merge|preview  (requires SWARVEX_PLATFORM, SWARVEX_PIPELINE_SECRET)")
		os.Exit(2)
	}
	if !jobHier(jobs, os.Args[1]) {
		if os.Args[1] == "preview" && vorschauVon != "" {
			vorschauenHolen()
			return
		}
		fmt.Printf("%s runs on the pipeline server, not here (SWARVEX_JOBS=%s)\n", os.Args[1], jobs)
		return
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

// jobHier: Läuft dieser Teil der Prüfstraße auf diesem Rechner? Ohne SWARVEX_JOBS alle.
func jobHier(liste, job string) bool {
	if strings.TrimSpace(liste) == "" {
		return true
	}
	for _, j := range strings.Split(liste, ",") {
		if strings.TrimSpace(j) == job {
			return true
		}
	}
	return false
}

// vorschauenHolen spiegelt die auf dem Prüfserver gebauten Vorschauen in den Ordner, den Caddy hier ausliefert. Nur
// lesend (dort erzwingt rrsync -ro den Ordner) und nur gewöhnliche Dateien: Ein übernommener Prüfserver soll über einen
// Link keine Datei dieses Rechners ausliefern lassen können.
func vorschauenHolen() {
	ziel := filepath.Join(arbeit, "vorschau")
	os.MkdirAll(ziel, 0o755)
	ssh := fmt.Sprintf("ssh -i %s -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=%s.known_hosts", vorschauKey, vorschauKey)
	if aus, err := befehl("", nil, "rsync", "-rt", "--delete", "--no-links", "--no-devices", "--no-specials", "--chmod=D755,F644",
		"--max-size=200m", "-e", ssh, vorschauQuelle(vorschauVon), ziel+"/"); err != nil {
		fmt.Fprintf(os.Stderr, "fetching previews from the pipeline server: %v\n%s\n", err, aus)
		os.Exit(1)
	}
	fmt.Println("previews fetched from the pipeline server")
}

// vorschauQuelle: „vorschau@server:“ wird zu „vorschau@server:./“, relativ zu dem Ordner, auf den rrsync den Zugang
// beschränkt; immer mit Schrägstrich am Ende, damit rsync den Inhalt spiegelt und nicht den Ordner selbst.
func vorschauQuelle(v string) string {
	if strings.HasSuffix(v, ":") {
		v += "./"
	}
	return strings.TrimRight(v, "/") + "/"
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
func spiegel(e einreichung) (string, error) { return spiegelIn(e, "mirror") }

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
	// Was der Beitrag ändert, an den Test (SWARVEX_CHANGED): Ein Projekt kann damit nur das Betroffene prüfen
	geaendert, _ := geaendertePfade(baum)
	gruen := testen(&b, func() (string, error) {
		return imContainer(image, baum, cache, u, u.Testen, false, testSekunden, geaendertEnv(geaendert)...)
	})
	// Bildvergleich im Review: geänderte Szenenbilder dieses Laufs an die Plattform (vor einer Gegenprobe ohne Beitrag)
	if bilder, err := szenenBilder(baum, u.Ordner, u.Bilder, u.Referenzen, e.BasisCommit); err != nil {
		fmt.Fprintf(&b, "scene images: %v\n", err)
	} else {
		for _, bild := range bilder {
			if err := signiert("POST", fmt.Sprintf("/pipeline/%d/images", e.EinreichungID), bild.daten(), nil); err != nil {
				fmt.Fprintf(&b, "scene image %s: %v\n", bild.Name, err)
			}
		}
		if len(bilder) > 0 {
			fmt.Fprintf(&b, "Scene images that change: %d (submission_images shows them)\n", len(bilder))
		}
	}
	if gruen {
		return true, b.String(), false
	}
	if strings.Contains(b.String(), Zeitlimit) {
		// Am Zeitlimit liegt es am Projekt, nicht am Beitrag: weiter in die Bewertung, der Lead erfährt es
		return true, b.String(), true
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
		aus, err := imContainer(image, baum, cache, u, u.Testen, false, testSekunden, geaendertEnv(geaendert)...)
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
// zweiten Mal, und zwar an denselben Tests. Scheitern beide Läufe an ganz verschiedenen Tests, wackeln die Tests.
// Beide Läufe stehen im Bericht, damit der Lead die wackeligen Tests sieht.
func testen(b *strings.Builder, lauf func() (string, error)) bool {
	erster, err := lauf()
	fmt.Fprintf(b, "=== Test\n%s\n", erster)
	if err == nil {
		return true
	}
	if zeitUm(err) {
		// Ein zweiter Lauf (und eine Gegenprobe) kosteten nur noch einmal so lange und hielten alle anderen auf
		fmt.Fprintf(b, "=== Test %s (%d minutes); not run a second time.\nThe project's tests take too long for the test pipeline: "+
			"the lead should make them faster, for example by testing only what a contribution changes (SWARVEX_CHANGED lists its files).\n", Zeitlimit, testSekunden/60)
		return false
	}
	zweiter, err := lauf()
	if err == nil {
		fmt.Fprintf(b, "=== Test, second run: passed\nThe first run failed and the second passed: tests above fail only sometimes. "+
			"The lead should fix or quarantine them.\n%s\n", zweiter)
		return true
	}
	if a, z := fehlschlaege(erster), fehlschlaege(zweiter); len(a) > 0 && len(z) > 0 && !gemeinsam(a, z) {
		fmt.Fprintf(b, "=== Test, second run: failed on other tests\nThe two runs failed on different tests (first: %s; second: %s): "+
			"they fail only sometimes, not because of this contribution. It goes on to review; the lead should fix or quarantine them.\n%s\n",
			strings.Join(a, ", "), strings.Join(z, ", "), zweiter)
		return true
	}
	fmt.Fprintf(b, "=== Test, second run: failed again\n%s\n", zweiter)
	return false
}

var (
	fehlZeile   = regexp.MustCompile(`\bFAIL(ED)?\b`)
	zaehlZeile  = regexp.MustCompile(`^\d+ FAIL`)                      // Zusammenfassung wie „3 FAIL headless“
	dauerAmEnde = regexp.MustCompile(` ?\(?\d+(\.\d+)? ?(ms|s|m)\)?$`) // „(0.01s)“, „0.012s“
)

// fehlschlaege: die Zeilen, die einen gescheiterten Test nennen (FAIL oder FAILED mit Namen, wie bei go test, pytest,
// jest, cargo und den Testskripten der Projekte), ohne Zeitangaben und Zusammenfassungen. Erkennt das nichts, bleibt
// es beim strengen Weg: zweimal rot heißt rot.
func fehlschlaege(aus string) []string {
	gesehen := map[string]bool{}
	var liste []string
	for _, z := range strings.Split(aus, "\n") {
		z = strings.Join(strings.Fields(z), " ")
		if !fehlZeile.MatchString(z) || zaehlZeile.MatchString(z) {
			continue
		}
		z = dauerAmEnde.ReplaceAllString(z, "")
		if z == "FAIL" || z == "FAILED" || gesehen[z] {
			continue
		}
		gesehen[z] = true
		liste = append(liste, z)
	}
	return liste
}

func gemeinsam(a, z []string) bool {
	for _, x := range a {
		for _, y := range z {
			if x == y {
				return true
			}
		}
	}
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

// testSekunden: höchstens so lange je Testlauf; Zeitlimit steht dann im Bericht (`timeout` endet mit 124).
const (
	testSekunden = 1800
	Zeitlimit    = "stopped at the time limit"
)

func zeitUm(err error) bool {
	var ee *exec.ExitError
	return errors.As(err, &ee) && ee.ExitCode() == 124
}

// geaendertEnv: die geänderten Pfade für den Test, eine Zeile je Pfad (SWARVEX_CHANGED). Ohne die Variable (eigener
// Lauf, CI) prüft ein Projekt alles; mit ihr kann es nur prüfen, was der Beitrag berührt.
func geaendertEnv(pfade []string) []string {
	if len(pfade) == 0 {
		return nil
	}
	return []string{"-e", "SWARVEX_CHANGED=" + strings.Join(pfade, "\n")}
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
	if laufzeit != "" {
		args = append(args, "--runtime", laufzeit) // gVisor auf dem Prüfserver: eigener Kernel im Nutzerraum je Container
	}
	for _, v := range umgebung.CacheVariablen {
		args = append(args, "-e", v)
	}
	args = append(append(args, extra...), image, "timeout", fmt.Sprint(sekunden), "sh", "-ec", skript)
	return befehl("", nil, "docker", args...)
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
