package main

// swarvex update: holt die aktuelle Version von der Plattform, ohne den Einzeiler erneut einzufügen.
// Es läuft dasselbe install.sh wie beim Einzeiler (Signatur- und Prüfsummenprüfung inklusive), nur ohne
// die Einrichtungsfragen; die Einstellungen bleiben. Zusätzlich muss der Signaturschlüssel der Plattform
// derselbe sein wie bei der Installation (release.pub), sonst wird nichts installiert.
//
// Sicherheitsnetz: Vor dem Installieren wird die bisherige Version (release/) als release.prev aufgehoben.
// Startet die neue nicht, kommt die alte von selbst zurück; `swarvex update --rollback` geht von Hand zurück.
// Neue Einrichtungsfragen einer Version stellt danach das neue Programm (setup --new), nur diese.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
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

// version setzt release.sh beim Bauen aus runner/VERSION (-ldflags -X main.version=…), z. B. 0.9.0.
var version = "dev"

// versionText: so steht die Version vor Menschen, z. B. „v0.9.0“.
func versionText(v string) string {
	if v != "" && v[0] >= '0' && v[0] <= '9' {
		return "v" + v
	}
	return v
}

func versionBefehl() { fmt.Println("swarvex runner", versionText(version)) }

// istNeuer vergleicht Versionen Teil für Teil als Zahlen (0.10.0 > 0.9.3 > 0.1.202611041358);
// eine eigene Entwicklerversion ("dev") gilt als älter als jede veröffentlichte.
func istNeuer(neu, alt string) bool {
	zahlen := func(v string) ([]int, bool) {
		teile := strings.Split(strings.TrimPrefix(v, "v"), ".")
		r := make([]int, len(teile))
		for i, t := range teile {
			n, err := strconv.Atoi(t)
			if err != nil {
				return nil, false
			}
			r[i] = n
		}
		return r, true
	}
	n, okN := zahlen(neu)
	a, okA := zahlen(alt)
	switch {
	case !okN:
		return false
	case !okA:
		return true
	}
	for i := 0; i < max(len(n), len(a)); i++ {
		x, y := 0, 0
		if i < len(n) {
			x = n[i]
		}
		if i < len(a) {
			y = a[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

var installWert = regexp.MustCompile(`(?m)^(VERSION|SIGNING_KEY|KEY_ROTATION_SIGNATURE)="([^"\n]*)"`)

// installerLesen liefert Version, Signaturschlüssel und (nach einem Schlüsselwechsel) dessen Signatur mit dem alten.
func installerLesen(skript string) (ver, schluessel, wechsel string) {
	for _, m := range installWert.FindAllStringSubmatch(skript, -1) {
		switch m[1] {
		case "VERSION":
			ver = m[2]
		case "SIGNING_KEY":
			schluessel = m[2]
		default:
			wechsel = m[2]
		}
	}
	return
}

// schluesselwechselGeprueft: Der neue Schlüssel ist mit dem gemerkten signiert (ssh-keygen -Y, Namensraum
// swarvex-key-rotation). Signiert ist genau der Schlüsseltext ohne Zeilenumbruch.
func schluesselwechselGeprueft(gemerkt, neu, signaturB64 string) bool {
	gemerkt, neu = strings.TrimSpace(gemerkt), strings.TrimSpace(neu)
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signaturB64))
	if gemerkt == "" || neu == "" || err != nil || len(sig) == 0 {
		return false
	}
	dir, err := os.MkdirTemp("", "swarvex-key-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	erlaubt, sigDatei := filepath.Join(dir, "allowed"), filepath.Join(dir, "rotation.sig")
	if os.WriteFile(erlaubt, []byte("swarvex-release "+gemerkt+"\n"), 0o600) != nil || os.WriteFile(sigDatei, sig, 0o600) != nil {
		return false
	}
	cmd := exec.Command("ssh-keygen", "-Y", "verify", "-f", erlaubt, "-I", "swarvex-release", "-n", "swarvex-key-rotation", "-s", sigDatei)
	cmd.Stdin = strings.NewReader(neu)
	return cmd.Run() == nil
}

func pinDatei() string { return filepath.Join(konfigOrdner(), "release.pub") }

// schluesselPasst: ohne gemerkten Schlüssel (Installation vor dieser Version) gilt der aktuelle.
func schluesselPasst(gemerkt, neu string) bool {
	gemerkt = strings.TrimSpace(gemerkt)
	return gemerkt == "" || gemerkt == strings.TrimSpace(neu)
}

func installerHolen(plattform, ca string) (string, error) {
	c := httpClient(ca)
	c.Timeout = 15 * time.Second
	r, err := c.Get(strings.TrimRight(plattform, "/") + "/install.sh")
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", r.StatusCode)
	}
	roh, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	return string(roh), err
}

// plattformUndCA: wo der Runner angemeldet ist (oder --platform).
func plattformUndCA(vorgabe string) (string, string) {
	plattform, ca := vorgabe, ""
	if z, err := zugangLaden(); err == nil {
		if plattform == "" {
			plattform = z.Plattform
		}
		ca = z.CAPfad
	}
	return plattform, ca
}

// neuesteVersion liest install.sh der Plattform: Skript, Version und ob der Signaturschlüssel der gemerkte ist.
func neuesteVersion(plattform, ca string) (skript, neu string, err error) {
	skript, err = installerHolen(plattform, ca)
	if err != nil {
		return "", "", fmt.Errorf("could not fetch %s/install.sh: %w", plattform, err)
	}
	neu, schluessel, wechsel := installerLesen(skript)
	if neu == "" || schluessel == "" {
		return "", "", fmt.Errorf("the platform's install.sh has no version or signing key")
	}
	gemerkt, _ := os.ReadFile(pinDatei())
	if !schluesselPasst(string(gemerkt), schluessel) && schluesselwechselGeprueft(string(gemerkt), schluessel, wechsel) {
		fmt.Println("The platform signs releases with a new key; the key you installed with confirmed it (signed key change).")
	} else if !schluesselPasst(string(gemerkt), schluessel) {
		return "", neu, fmt.Errorf("the platform's release signing key is not the one you installed with; " +
			"if the project announced a new key, install again with the one-liner from the website")
	}
	return skript, neu, nil
}

var versionForm = regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,3}$`)

// releaseGeprueft: install.sh läuft nur, wenn die SHA256SUMS seiner Version mit dem Schlüssel signiert ist, dem dieser
// Rechner vertraut (release.pub oder ein neuer Schlüssel, den der gemerkte signiert hat), und genau dieses install.sh
// aufführt. Vorher prüfte nur das Skript selbst die Signatur, und ein Skript von einer übernommenen Plattform hätte
// es einfach weglassen können: der Hardware-Schlüssel schützte Updates nicht (Sicherheitsaudit 12.11.2026).
// Ohne gemerkten Schlüssel gibt es kein Update mehr, sondern den Hinweis auf den Einzeiler.
func releaseGeprueft(plattform, ca, skript, neu string) error {
	gemerkt, _ := os.ReadFile(pinDatei())
	pin := strings.TrimSpace(string(gemerkt))
	if pin == "" {
		return errors.New("no release key is pinned on this machine (release.pub); install again with the one-liner from the website")
	}
	_, schluessel, wechsel := installerLesen(skript)
	vertraut := pin
	if strings.TrimSpace(schluessel) != pin {
		if !schluesselwechselGeprueft(pin, schluessel, wechsel) {
			return errors.New("the platform's release signing key is not the one you installed with")
		}
		vertraut = strings.TrimSpace(schluessel)
	}
	if !versionForm.MatchString(neu) {
		return fmt.Errorf("unexpected version %q", neu)
	}
	c := httpClient(ca)
	c.Timeout = 30 * time.Second
	holen := func(name string) ([]byte, error) {
		r, err := c.Get(strings.TrimRight(plattform, "/") + "/downloads/" + neu + "/" + name)
		if err != nil {
			return nil, err
		}
		defer r.Body.Close()
		if r.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: HTTP %d", name, r.StatusCode)
		}
		return io.ReadAll(io.LimitReader(r.Body, 1<<20))
	}
	summen, err := holen("SHA256SUMS")
	if err != nil {
		return err
	}
	sig, err := holen("SHA256SUMS.sig")
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "swarvex-release-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	erlaubt, sigDatei := filepath.Join(dir, "allowed"), filepath.Join(dir, "SHA256SUMS.sig")
	if os.WriteFile(erlaubt, []byte("swarvex-release "+vertraut+"\n"), 0o600) != nil || os.WriteFile(sigDatei, sig, 0o600) != nil {
		return errors.New("could not prepare the signature check")
	}
	pruefung := exec.Command("ssh-keygen", "-Y", "verify", "-f", erlaubt, "-I", "swarvex-release", "-n", "swarvex-release", "-s", sigDatei)
	pruefung.Stdin = bytes.NewReader(summen)
	if pruefung.Run() != nil {
		return errors.New("the release signature does not verify with your release key")
	}
	h := sha256.Sum256([]byte(skript))
	if !bytes.Contains(append([]byte("\n"), summen...), []byte("\n"+hex.EncodeToString(h[:])+"  install.sh\n")) {
		return errors.New("install.sh is not part of the signed release")
	}
	return nil
}

// installieren führt install.sh ohne Einrichtungsfragen aus (dieselbe Signatur- und Prüfsummenprüfung wie der Einzeiler);
// vorher hat releaseGeprueft das Skript selbst gegen die signierte SHA256SUMS geprüft.
func installieren(skript, ca string, ausgabe io.Writer) error {
	datei, err := os.CreateTemp("", "swarvex-install-*.sh")
	if err != nil {
		return err
	}
	defer os.Remove(datei.Name())
	_, err = datei.WriteString(skript)
	datei.Close()
	if err != nil {
		return err
	}
	install := exec.Command("sh", datei.Name())
	install.Stdout, install.Stderr = ausgabe, ausgabe
	install.Env = append(os.Environ(), "SWARVEX_NO_SETUP=1")
	if ca != "" {
		install.Env = append(install.Env, "SWARVEX_CA="+ca)
	}
	return install.Run()
}

func updateBefehl(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	nurSehen := fs.Bool("check", false, "only show whether a newer version exists and what is new")
	erzwingen := fs.Bool("force", false, "install even if this version is already installed")
	zurueck := fs.Bool("rollback", false, "go back to the version installed before the last update")
	sofort := fs.Bool("now", false, "restart the service right away, even during a task (the task is handed back)")
	plattformWert := fs.String("platform", "", "platform address (default: where you are signed in)")
	fs.Parse(args)
	if *zurueck {
		zurueckBefehl(*sofort)
		return
	}

	plattform, ca := plattformUndCA(*plattformWert)
	if plattform == "" {
		fehlerAus("Not signed in: pass --platform https://swarvex.ai or run the one-liner from the website.")
	}
	skript, neu, err := neuesteVersion(plattform, ca)
	if neu != "" {
		fmt.Printf("Installed: %s · on %s: %s\n", versionText(version), plattform, versionText(neu))
	}
	if err != nil {
		fehlerAus(err.Error() + "; nothing was installed.")
	}
	if !istNeuer(neu, version) && !*erzwingen {
		fmt.Println("✓ Up to date.")
		return
	}
	neuigkeitenZeigen(plattform, ca, version, neu)
	if *nurSehen {
		fmt.Println("Update with: swarvex update")
		return
	}
	os.Remove(ueberspringenDatei()) // von Hand gewollt: auch eine zurückgerollte Version
	if err := releaseGeprueft(plattform, ca, skript, neu); err != nil {
		fehlerAus(err.Error() + "; nothing was installed.")
	}
	if err := sicherInstallieren(skript, ca, neu, os.Stdout); err != nil {
		fehlerAus("The update did not finish (" + err.Error() + "); " + versionText(version) + " stays installed.")
	}
	fmt.Println("✓ " + versionText(neu) + " installed (back to the previous version, if needed: swarvex update --rollback)")
	// Neue Einrichtungsfragen kennt nur das neue Programm; es fragt nur, was noch offen ist (sonst still).
	nachfragen := exec.Command(installiertesProgramm(), "setup", "--new", "--quiet")
	nachfragen.Stdin, nachfragen.Stdout, nachfragen.Stderr = os.Stdin, os.Stdout, os.Stderr
	nachfragen.Run()
	dienstNeuStarten(*sofort, "the new version")
}

// autoUpdate: im Dauerbetrieb höchstens einmal am Tag, zwischen zwei Aufgaben. Danach lädt sich die Schleife neu.
func autoUpdate() {
	plattform, ca := plattformUndCA("")
	if plattform == "" {
		return
	}
	skript, neu, err := neuesteVersion(plattform, ca)
	switch {
	case err != nil:
		fmt.Println("Automatic update skipped:", err)
	case istNeuer(neu, version) && strings.TrimSpace(lesen(ueberspringenDatei())) == neu:
		fmt.Printf("Automatic update: %s skipped (rolled back on this machine; `swarvex update` installs it anyway)\n", versionText(neu))
	case istNeuer(neu, version):
		fmt.Printf("Automatic update: %s → %s\n", versionText(version), versionText(neu))
		if text, err := changelogHolen(plattform, ca); err == nil {
			for _, z := range neuigkeiten(text, version, neu) {
				fmt.Println(strings.TrimRight("  "+z, " "))
			}
		}
		if err := releaseGeprueft(plattform, ca, skript, neu); err != nil {
			fmt.Printf("Automatic update skipped: %v; continuing with %s\n", err, versionText(version))
			return
		}
		if err := sicherInstallieren(skript, ca, neu, os.Stdout); err != nil {
			fmt.Printf("Automatic update did not finish (%v); continuing with %s\n", err, versionText(version))
			if errors.Is(err, errStartetNicht) {
				// diese Version nicht jeden Tag wieder versuchen; die nächste kommt wie gewohnt
				os.WriteFile(ueberspringenDatei(), []byte(neu+"\n"), 0o644)
			}
		}
	}
}

// dienstNeuStarten: Ohne laufende Aufgabe startet der Dienst gleich neu. Mit Aufgabe läuft sie zu Ende; danach
// lädt sich der Dauerbetrieb selbst neu (neues Programm oder geänderte Dienst-Einheit, siehe dauerbetrieb).
// Einen gestoppten Dienst startet ein Update nicht, es sagt nur, wie er weiterläuft.
func dienstNeuStarten(sofort bool, was string) {
	if _, err := os.Stat(dienstDatei()); err != nil {
		return
	}
	if zustand, _ := systemctl("is-active", dienstName); zustand != "active" {
		if h := dienstHinweis(zustand); h != "" {
			fmt.Printf("The service is not running (%s); it runs %s from its next start.\n", h, was)
		}
		return
	}
	if aufgabeLaeuft() && !sofort {
		fmt.Printf("A task is running: it finishes as it is, the service then continues with %s.\n", was)
		fmt.Println("  To switch right away (the task is handed back), add --now")
		return
	}
	if aus, err := systemctl("restart", dienstName); err != nil {
		fehlerAus("restarting the service: " + aus)
	}
	fmt.Printf("Service restarted with %s.\n", was)
}

// aufgabeLaeuft: Jede Aufgabe hat ihren eigenen Schlüssel-Proxy-Container.
func aufgabeLaeuft() bool {
	aus, err := exec.Command("podman", "ps", "-q", "--filter", "name=^swarvex-proxy-").Output()
	return err == nil && strings.TrimSpace(string(aus)) != ""
}

// neueVersionInstalliert: Hat `swarvex update` das Programm seit dem Start ersetzt?
func neueVersionInstalliert(programm string, start os.FileInfo) bool {
	jetzt, err := os.Stat(programm)
	return err == nil && start != nil && !os.SameFile(start, jetzt)
}

// ---- Sicherheitsnetz: vorherige Version aufheben, neue prüfen, zurückrollen ----

// installiertesProgramm: dorthin legt install.sh das Programm ($BIN/swarvex).
func installiertesProgramm() string { return heim(".local", "bin", "swarvex") }

// releaseOrdner: das entpackte Paket der installierten Version (Programm und Bauanleitungen der Images).
func releaseOrdner() string { return filepath.Join(datenOrdner(), "release") }

// ueberspringenDatei: eine zurückgerollte Version, die das automatische Update auslässt.
func ueberspringenDatei() string { return filepath.Join(konfigOrdner(), "update-skip") }

func lesen(datei string) string {
	roh, _ := os.ReadFile(datei)
	return string(roh)
}

var (
	errStartetNicht = errors.New("the new version does not start")
	// errImages: das Programm ist zurück, nur die Images ließen sich nicht neu bauen
	errImages = errors.New("the images could not be rebuilt")
)

// versionAus liest die Version aus der Ausgabe von `swarvex version` („swarvex runner v0.9.1“).
func versionAus(ausgabe string) string {
	felder := strings.Fields(ausgabe)
	if len(felder) < 3 || felder[0] != "swarvex" || felder[1] != "runner" {
		return ""
	}
	return strings.TrimPrefix(felder[2], "v")
}

// programmVersion startet ein Programm einmal mit `version`: startet es überhaupt, und welche Version ist es?
func programmVersion(programm string) (string, error) {
	ctx, ende := context.WithTimeout(context.Background(), 20*time.Second)
	defer ende()
	aus, err := exec.CommandContext(ctx, programm, "version").Output()
	if err != nil {
		return "", err
	}
	if v := versionAus(string(aus)); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("unexpected output %q", kuerzen(strings.TrimSpace(string(aus)), 80))
}

// imagesBauen: wie install.sh, aus dem Paket im Ordner (Bauschritte sind im Cache, wenn sich nichts geändert hat).
var imagesBauen = func(ordner string, ausgabe io.Writer) error {
	fmt.Fprintln(ausgabe, "   rebuilding the agent and key proxy images of that version …")
	schritte := []struct {
		ordner, was string
		args        []string
	}{
		{ordner, "proxy", []string{"build", "-q", "-f", "Containerfile.proxy", "-t", "swarvex-proxy", "."}},
		{filepath.Join(ordner, "sandbox"), "agent", []string{"build", "-q", "--target", "opencode", "-t", "swarvex-agent", "."}},
	}
	for _, s := range schritte {
		c := exec.Command("podman", s.args...)
		c.Dir = s.ordner
		if aus, err := c.CombinedOutput(); err != nil {
			grund := strings.TrimSpace(string(aus))
			if grund == "" {
				grund = err.Error()
			}
			return fmt.Errorf("%w (%s image: %s)", errImages, s.was, kuerzen(grund, 200))
		}
	}
	exec.Command("podman", "rmi", "-f", "swarvex-agent-claude", "swarvex-agent-codex").Run() // beim ersten Gebrauch neu gebaut
	return nil
}

// sicherInstallieren: bisherige Version aufheben, installieren, die neue einmal starten. Geht etwas schief,
// kommt die bisherige zurück (Programm, Paket und, falls schon ersetzt, die Images).
func sicherInstallieren(skript, ca, neu string, ausgabe io.Writer) error {
	rel, vorher := releaseOrdner(), releaseOrdner()+".prev"
	gesichert := false
	if _, err := os.Stat(filepath.Join(rel, "swarvex")); err == nil {
		os.RemoveAll(vorher)
		gesichert = os.Rename(rel, vorher) == nil
	}
	err := installieren(skript, ca, ausgabe)
	if err == nil {
		if v, e := programmVersion(installiertesProgramm()); e != nil {
			err = fmt.Errorf("%w: %v", errStartetNicht, e)
		} else if v != neu {
			err = fmt.Errorf("%w: it reports version %s instead of %s", errStartetNicht, v, neu)
		}
	}
	if err == nil || !gesichert {
		return err
	}
	fmt.Fprintf(ausgabe, "\n%v; going back to the previous version …\n", err)
	// Hat install.sh das Programm schon ersetzt, sind womöglich auch die Images neu: dann neu bauen.
	ersetzt := true
	if v, e := programmVersion(installiertesProgramm()); e == nil && v == version {
		ersetzt = false
	}
	switch e := zurueckholen(ausgabe, ersetzt); {
	case errors.Is(e, errImages):
		fmt.Fprintf(ausgabe, "   previous version restored, but %v: swarvex doctor shows what is missing\n", e)
	case e != nil:
		return fmt.Errorf("%v; going back failed too (%v): run the one-liner from the website again", err, e)
	default:
		fmt.Fprintln(ausgabe, "   previous version restored")
	}
	return err
}

// zurueckholen setzt release.prev wieder als installierte Version ein; mitImages baut deren Images neu.
func zurueckholen(ausgabe io.Writer, mitImages bool) error {
	rel, vorher := releaseOrdner(), releaseOrdner()+".prev"
	if _, err := os.Stat(filepath.Join(vorher, "swarvex")); err != nil {
		return errors.New("no previous version kept")
	}
	weg := rel + ".failed"
	os.RemoveAll(weg)
	if _, err := os.Stat(rel); err == nil {
		if err := os.Rename(rel, weg); err != nil {
			return err
		}
	}
	if err := os.Rename(vorher, rel); err != nil {
		return err
	}
	os.RemoveAll(weg)
	if err := programmEinsetzen(filepath.Join(rel, "swarvex"), installiertesProgramm()); err != nil {
		return err
	}
	if mitImages {
		return imagesBauen(rel, ausgabe)
	}
	return nil
}

// programmEinsetzen kopiert ein Programm an sein Ziel und tauscht es in einem Schritt aus (neue Datei, wie install -m 755;
// ein laufender Dauerbetrieb merkt das und lädt sich zwischen zwei Aufgaben neu).
func programmEinsetzen(quelle, ziel string) error {
	roh, err := os.ReadFile(quelle)
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(ziel), 0o755)
	tmp := ziel + ".new"
	if err := os.WriteFile(tmp, roh, 0o755); err != nil {
		return err
	}
	os.Chmod(tmp, 0o755)
	return os.Rename(tmp, ziel)
}

// zurueckBefehl: swarvex update --rollback
func zurueckBefehl(sofort bool) {
	alt, err := programmVersion(filepath.Join(releaseOrdner()+".prev", "swarvex"))
	if err != nil {
		fehlerAus("No previous version is kept on this machine (it is kept from v0.9.1 on, at the next update); nothing changed.")
	}
	fmt.Printf("Going back from %s to %s …\n", versionText(version), versionText(alt))
	switch err := zurueckholen(os.Stdout, true); {
	case errors.Is(err, errImages):
		fmt.Printf("! %s is back, but %v.\n  Check with: swarvex doctor\n", versionText(alt), err)
	case err != nil:
		fehlerAus("Going back failed (" + err.Error() + "). If swarvex no longer works, run the one-liner from the website again.")
	}
	if version != "dev" {
		os.WriteFile(ueberspringenDatei(), []byte(version+"\n"), 0o644)
	}
	fmt.Printf("✓ %s installed. Automatic updates skip %s; newer versions come as usual (or now: swarvex update).\n", versionText(alt), versionText(version))
	dienstNeuStarten(sofort, versionText(alt))
}

// ---- Was ist neu: die Einträge aus CHANGELOG.md zwischen installierter und neuer Version ----

func changelogHolen(plattform, ca string) (string, error) {
	c := httpClient(ca)
	c.Timeout = 10 * time.Second
	r, err := c.Get(strings.TrimRight(plattform, "/") + "/downloads/CHANGELOG.md")
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", r.StatusCode)
	}
	roh, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	return string(roh), err
}

var changelogKopf = regexp.MustCompile(`^## v?(\d+(?:\.\d+)*)\b`)

// neuigkeiten: die Abschnitte „## X.Y.Z …“ mit alt < X.Y.Z ≤ neu, neueste zuerst wie in der Datei.
func neuigkeiten(text, alt, neu string) []string {
	var r []string
	drin := false
	for _, z := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		switch m := changelogKopf.FindStringSubmatch(z); {
		case m != nil:
			drin = istNeuer(m[1], alt) && !istNeuer(m[1], neu)
			if drin {
				r = append(r, "", strings.TrimPrefix(z, "## "))
			}
		case strings.HasPrefix(z, "#"):
			drin = false
		case drin:
			r = append(r, strings.TrimRight(z, " "))
		}
	}
	// Leerzeilen am Rand weg, mehrere hintereinander zu einer
	var sauber []string
	for _, z := range r {
		if z == "" && (len(sauber) == 0 || sauber[len(sauber)-1] == "") {
			continue
		}
		sauber = append(sauber, z)
	}
	for len(sauber) > 0 && sauber[len(sauber)-1] == "" {
		sauber = sauber[:len(sauber)-1]
	}
	return sauber
}

func neuigkeitenZeigen(plattform, ca, alt, neu string) {
	text, err := changelogHolen(plattform, ca)
	if err != nil {
		return
	}
	zeilen := neuigkeiten(text, alt, neu)
	if len(zeilen) == 0 {
		return
	}
	fmt.Println("\nWhat's new:")
	const hoechstens = 40
	for i, z := range zeilen {
		if i == hoechstens {
			fmt.Printf("  … the full list: %s/downloads/CHANGELOG.md\n", strings.TrimRight(plattform, "/"))
			break
		}
		fmt.Println(strings.TrimRight("  "+z, " "))
	}
	fmt.Println()
}
