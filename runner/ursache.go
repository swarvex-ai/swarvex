package main

// Warum eine Runde ohne Ergebnis endete. Bis 0.13 stand in der Rückgabe nur „agent exited with an error: exit status 1“:
// Ein Runner, dessen Anbieter den Schlüssel ablehnte, gab so eine Nacht lang jede Aufgabe zurück, niemand sah warum,
// und jede Rückgabe zählte als Misserfolg seines Modells. Deshalb:
//   - Die Rückgabe nennt die Ursache (aus dem Protokoll des Schlüssel-Proxys und dem Ende der Agenten-Ausgabe).
//   - Kam das Modell gar nicht zum Arbeiten (Anbieter, Rechner, Projektumgebung, Plattform, angehalten), geht die Art
//     der Ursache als `cause` an die Plattform: Der Versuch zählt dann nicht als Misserfolg des Modells (E-45).
//   - Nach drei Runden in Folge mit derselben Ursache setzt der Dauerbetrieb aus und sagt dem Betreiber, warum.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ursache einer Rückgabe ohne Ergebnis.
type ursache struct {
	// Art: wo es scheiterte, wenn das Modell nicht zum Arbeiten kam (geht als `cause` an die Plattform). Leer: Sache der
	// Aufgabe oder nicht sicher erkennbar; der Versuch zählt dann wie bisher.
	Art string
	// Schluessel: über Aufgaben hinweg gleich, für die Serie im Verlauf (serieAus). Leer: kein Absturz, die Runde kam
	// zu einem Ergebnis in der Sache (Zeit oder Budget aufgebraucht, nichts geändert, Patch zu groß).
	Schluessel string
	// Text: ein Satz für die Rückgabe und den Betreiber.
	Text string
}

const (
	ursacheAnbieter  = "provider" // der Modell-Anbieter antwortet nicht: Schlüssel, Guthaben, Limit, Ausfall
	ursacheRunner    = "runner"   // dieser Rechner oder seine Einrichtung: Agent startet nicht, Image, Sandbox, Abo-Regeln
	ursacheProjekt   = "project"  // die Umgebung des Projekts ließ sich hier nicht herrichten
	ursachePlattform = "platform" // Aufgabe, Snapshot oder LFS-Dateien kamen nicht von der Plattform
	ursacheGestoppt  = "stopped"  // der Betreiber hat den Runner angehalten
)

// Ursachen vor dem Start des Modells, an den Stellen in arbeit.go, an denen sie auftreten.
var (
	uGestoppt     = ursache{ursacheGestoppt, "stopped", ""}
	uAufgabe      = ursache{ursachePlattform, "task", ""}
	uSnapshot     = ursache{ursachePlattform, "snapshot", ""}
	uLFS          = ursache{ursachePlattform, "lfs", ""}
	uBasis        = ursache{ursacheRunner, "base-commit", ""}
	uUmgebung     = ursache{ursacheProjekt, "environment", ""}
	uAgentImage   = ursache{ursacheRunner, "agent-image", ""}
	uProjektImage = ursache{ursacheProjekt, "project-image", ""}
	uEinrichten   = ursache{ursacheProjekt, "setup", ""}
	uZugang       = ursache{ursacheRunner, "access", ""}
	uSandbox      = ursache{ursacheRunner, "sandbox", "the sandbox could not be started"}
)

// schluesselText: der Schlüssel einer Ursache in Worten, für die Meldung an den Betreiber.
func schluesselText(schluessel string) string {
	if rest, ok := strings.CutPrefix(schluessel, "provider:"); ok {
		if rest == "5xx" {
			return "the model provider failed (HTTP 5xx)"
		}
		status := 0
		fmt.Sscan(rest, &status) // "unreachable" bleibt 0
		return anbieterText(status)
	}
	if rest, ok := strings.CutPrefix(schluessel, "agent:"); ok && rest != "exit" {
		return "the agent ended after a failed model request (HTTP " + rest + ")"
	}
	return map[string]string{
		"task": "the task could not be read from the platform", "snapshot": "the project snapshot could not be fetched",
		"lfs": "the project's LFS files could not be fetched", "base-commit": "git could not prepare the working tree",
		"environment": "the project's environment could not be determined", "agent-image": "the agent image could not be built",
		"project-image": "the image for the project's environment could not be built", "setup": "the project's dependencies could not be installed",
		"access": "the model key or sign-in could not be read", "sandbox": "the sandbox could not be started",
		"path":       "the agent called the model in a way the key proxy does not pass on",
		"no-request": "the agent ended before its first model request", "agent:exit": "the agent ended with an error although the model answered",
	}[schluessel]
}

// amProjekt: Die Ursache hängt an einem Projekt (sein Snapshot, seine Umgebung), nicht am Runner. Dann setzt der
// Dauerbetrieb nur dieses Projekt aus, wenn er mehrere bedient.
func amProjekt(schluessel string) bool {
	return enthaelt([]string{"snapshot", "lfs", "environment", "project-image", "setup"}, schluessel)
}

// gegenpruefenTrotzdem: Bei dieser Ursache laufen Gegenprüfungen (Tests, kein Modell) während der Pause weiter. Nicht,
// wenn der Rechner selbst (Sandbox, Git) oder die Verbindung zur Plattform in Frage steht.
func gegenpruefenTrotzdem(schluessel string) bool {
	return !enthaelt([]string{"sandbox", "base-commit", "task"}, schluessel)
}

// agentEnde: Der Agent lief in der Sandbox und endete mit einem Fehler oder am Zeitlimit. Jeder andere Fehler der
// Sandbox-Funktionen heißt: Sie kam gar nicht zustande.
type agentEnde struct {
	grund     string
	zeitlimit bool
}

func (e agentEnde) Error() string { return e.grund }

// proxyBild: was das Protokoll des Schlüssel-Proxys über die Anfragen ans Modell sagt.
type proxyBild struct {
	anfragen   int    // an den Anbieter gegangen
	gut        int    // davon beantwortet (Status unter 400)
	schluss    int    // so viele am Ende in Folge gescheitert
	letzter    int    // Status der letzten gescheiterten Anfrage; 0: Anbieter nicht erreichbar
	kontingent bool   // zuletzt haben die Abo-Regeln des Betreibers das Modell angehalten
	budget     bool   // zuletzt war das Token-Budget der Runde verbraucht
	abgewiesen string // ein Pfad, den der Proxy nicht weiterreicht
	ohneToken  bool   // eine Anfrage kam ohne das Wegwerf-Token der Sandbox
}

func proxyBildAus(protokoll string) proxyBild {
	var b proxyBild
	for _, zeile := range strings.Split(protokoll, "\n") {
		var e struct {
			Art    string `json:"type"`
			Status int    `json:"status"`
			Grund  string `json:"reason"`
			Pfad   string `json:"path"`
		}
		if json.Unmarshal([]byte(zeile), &e) != nil {
			continue
		}
		switch e.Art {
		case "request", "models":
			b.anfragen++
			b.kontingent, b.budget = false, false
			if e.Status < 400 {
				b.gut++
				b.schluss = 0
			} else {
				b.schluss++
				b.letzter = e.Status
			}
		case "unreachable":
			b.anfragen++
			b.kontingent, b.budget = false, false
			b.schluss++
			b.letzter = 0
		case "quota_reserve":
			b.kontingent = true
		case "budget_exhausted":
			b.budget = true
		case "rejected":
			if e.Grund == "path" {
				b.abgewiesen = e.Pfad
			} else {
				b.ohneToken = true
			}
		}
	}
	return b
}

// anbieterStatus: Mit diesem Status hat der Anbieter die Arbeit abgeschnitten (nicht die Anfrage war falsch).
func anbieterStatus(status int) bool {
	return status == 0 || status == 401 || status == 402 || status == 403 || status == 408 || status == 429 || status >= 500
}

func anbieterSchluessel(status int) string {
	switch {
	case status == 0:
		return "provider:unreachable"
	case status >= 500:
		return "provider:5xx"
	}
	return fmt.Sprintf("provider:%d", status)
}

func anbieterText(status int) string {
	switch {
	case status == 0:
		return "the model provider was unreachable"
	case status == 401:
		return "the model provider refused the key or sign-in (HTTP 401)"
	case status == 403:
		// 403 heißt auch: Ausgabelimit des Schlüssels erreicht (OpenRouter „Key limit exceeded“, 11.11.2026)
		return "the model provider refused the key or its spending limit is reached (HTTP 403)"
	case status == 402:
		return "the model provider reports no credit (HTTP 402)"
	case status == 404:
		return "the model provider does not know this model or address (HTTP 404)"
	case status == 429:
		return "the model provider's rate limit or quota is used up (HTTP 429)"
	case status >= 500:
		return fmt.Sprintf("the model provider failed (HTTP %d)", status)
	}
	return fmt.Sprintf("the model provider rejected the request (HTTP %d)", status)
}

var proxyPfadZeichen = regexp.MustCompile(`[^A-Za-z0-9/_.-]`)

// sandboxUrsache: die Ursache aus dem Protokoll des Schlüssel-Proxys, wenn der Agent mit einem Fehler endete oder am
// Zeitlimit. Vorsichtig: Eine Ursache außerhalb des Modells (Art) gibt es nur, wenn das Protokoll sie zeigt. Hat das
// Modell bis zuletzt geantwortet, bleibt die Art leer, und der Versuch zählt wie bisher.
func sandboxUrsache(protokoll string, zeitlimit bool) ursache {
	b := proxyBildAus(protokoll)
	switch {
	case b.kontingent:
		// Kein Absturz (das Modell lief, bis die Regeln des Betreibers es anhielten), aber auch kein Misserfolg des Modells.
		return ursache{ursacheRunner, "", "this runner's subscription rules stopped the model during the task"}
	case b.budget:
		return ursache{} // das Budget ist aufgebraucht: kein Absturz, und es zählt wie bisher
	case b.anfragen == 0 && b.abgewiesen != "":
		pfad := proxyPfadZeichen.ReplaceAllString(b.abgewiesen, "")
		pfad = pfad[:min(len(pfad), 60)]
		return ursache{ursacheRunner, "path", "the agent called " + pfad + ", which the key proxy does not pass on to the model"}
	case b.anfragen == 0 && b.ohneToken:
		return ursache{ursacheRunner, "no-request", "the agent's model requests did not carry the sandbox token, so none reached the model"}
	case b.anfragen == 0 && zeitlimit:
		return ursache{ursacheRunner, "no-request", "the agent made no model request"}
	case b.anfragen == 0:
		return ursache{ursacheRunner, "no-request", "the agent ended before its first model request"}
	case b.gut == 0:
		return ursache{ursacheAnbieter, anbieterSchluessel(b.letzter), "the model never answered: " + anbieterText(b.letzter)}
	// Am Zeitlimit zählt eine einzelne gescheiterte Anfrage am Ende nicht: erst, wenn der Agent mehrfach nicht mehr durchkam.
	case b.schluss > 0 && anbieterStatus(b.letzter) && (!zeitlimit || b.schluss >= 3):
		return ursache{ursacheAnbieter, anbieterSchluessel(b.letzter), "the model stopped answering: " + anbieterText(b.letzter)}
	case zeitlimit:
		return ursache{} // die Zeit ist aufgebraucht: kein Absturz
	case b.schluss > 0:
		return ursache{"", fmt.Sprintf("agent:%d", b.letzter), fmt.Sprintf("the last model request failed (HTTP %d)", b.letzter)}
	}
	return ursache{"", "agent:exit", "the model answered until the end (the cause lies in the agent or the task)"}
}

var (
	maskeMail   = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	maskeGeheim = regexp.MustCompile(`(?i)\b(?:authorization|token|secret|password|passwd|api[-_]?key)["']?\s*[:=]\s*(?:(?:bearer|basic)\s+)?\S+|bearer\s+\S+`)
	maskeKenn   = regexp.MustCompile(`(?i)\b(?:sk|pk|rk|key|org|proj|sess|user|acct)[-_][A-Za-z0-9_-]{8,}|[A-Za-z0-9_+=-]{32,}`)
	maskeHeim   = regexp.MustCompile(`(?:/var)?/(?:home|Users)/[^/\s"'` + "`" + `]+|[A-Za-z]:\\Users\\[^\\\s"'` + "`" + `]+`)
	fehlerWort  = regexp.MustCompile(`(?i)\b(error|fatal|panic|exception|failed|failure|denied|not found|invalid|unauthorized|forbidden|refused|unsupported)\b`)
)

// ausgabeEnde: die letzten n nicht leeren Zeilen der Agenten-Ausgabe, ohne Terminal-Steuerzeichen.
func ausgabeEnde(protokoll string, n int) []string {
	var z []string
	for _, l := range strings.Split(terminalSteuerung.ReplaceAllString(protokoll, ""), "\n") {
		if l = strings.TrimSpace(strings.ToValidUTF8(l, "")); l != "" {
			z = append(z, l)
		}
	}
	if len(z) > n {
		z = z[len(z)-n:]
	}
	return z
}

// letzteAusgabe: die Zeile vom Ende der Agenten-Ausgabe, die am ehesten den Fehler nennt (die letzte mit einem
// Fehlerwort unter den letzten 15, sonst die letzte überhaupt).
func letzteAusgabe(protokoll string) string {
	z := ausgabeEnde(protokoll, 15)
	for i := len(z) - 1; i >= 0; i-- {
		if fehlerWort.MatchString(z[i]) {
			return z[i]
		}
	}
	if len(z) == 0 {
		return ""
	}
	return z[len(z)-1]
}

// oeffentlicheZeile: eine Zeile der Agenten-Ausgabe für fremde Augen (der Rückgabegrund steht unveränderlich im
// öffentlichen Projektprotokoll und geht an den nächsten Agenten): ohne E-Mail-Adressen, Schlüssel, Kennungen von
// Konten und lange Zeichenfolgen mit Ziffern (Tokens, Hashes), Home-Ordner als ~, höchstens n Zeichen.
func oeffentlicheZeile(zeile string, n int) string {
	zeile = strings.Join(strings.Fields(zeile), " ")
	zeile = maskeMail.ReplaceAllString(zeile, "[address]")
	zeile = maskeGeheim.ReplaceAllString(zeile, "[hidden]")
	// Kennungen (sk-…, org-…, user-…) und lange Folgen nur mit Ziffern darin: Namen wie key_not_found bleiben lesbar.
	zeile = maskeKenn.ReplaceAllStringFunc(zeile, func(s string) string {
		if strings.ContainsAny(s, "0123456789") {
			return "[hidden]"
		}
		return s
	})
	zeile = strings.ReplaceAll(maskeHeim.ReplaceAllString(zeile, "~"), `"`, "'")
	if r := []rune(zeile); len(r) > n {
		zeile = string(r[:n-1]) + "…"
	}
	return zeile
}

// rueckgabeNachSandbox: Ursache und Rückgabegrund, wenn die Sandbox mit einem Fehler endete, aus proxy.log und
// agent.log im Ordner der Runde. Dem Betreiber schreibt sie die Ursache und das Ende der Agenten-Ausgabe ins Journal
// (unmaskiert, das bleibt auf seinem Rechner). In die Rückgabe, die unveränderlich im öffentlichen Projektprotokoll
// steht, kommt die letzte Zeile des Agenten nur, wenn das Modell lief und der Agent trotzdem abstürzte (agent:…): Dann
// kann sie dem Lead etwas über die Aufgabe sagen (etwa „prompt is too long“). Lag es am Anbieter, am Abo oder an der
// Einrichtung des Runners, sagt die Zeile nur etwas über den Rechner des Betreibers (Konto, Tarif, Adressen im eigenen
// Netz, Pfade), und die Masken sind Faustregeln: Dann steht dort nur die Ursache.
func rueckgabeNachSandbox(err error, ordner string) (ursache, string) {
	var ende agentEnde
	if !errors.As(err, &ende) {
		return uSandbox, "sandbox: " + err.Error()
	}
	proxyLog, _ := os.ReadFile(filepath.Join(ordner, "proxy.log"))
	agentLog, _ := os.ReadFile(filepath.Join(ordner, "agent.log"))
	u := sandboxUrsache(string(proxyLog), ende.zeitlimit)
	grund := "sandbox: " + ende.grund
	if u.Text == "" {
		return u, grund
	}
	grund += "; " + u.Text
	fmt.Printf("Why: %s.\n", u.Text)
	if zeilen := ausgabeEnde(string(agentLog), 8); len(zeilen) > 0 && u.Schluessel != "" {
		fmt.Printf("Last output of the agent (all of it: %s):\n", filepath.Join(ordner, "agent.log"))
		for _, z := range zeilen {
			fmt.Println("    | " + string([]rune(z)[:min(len([]rune(z)), 300)]))
		}
	}
	if strings.HasPrefix(u.Schluessel, "agent:") {
		if z := oeffentlicheZeile(letzteAusgabe(string(agentLog)), 160); z != "" {
			grund += `; last output: "` + z + `"`
		}
	}
	return u, grund
}

// serie: die neuesten Runden in Folge, die mit derselben Ursache abstürzten.
type serie struct {
	Schluessel string
	Projekt    string // nur bei einer Ursache am Projekt: Dann zählen nur dessen Runden
	Anzahl     int
	Zeit       time.Time // der neueste Absturz
	Grund      string    // sein Rückgabegrund
}

// serieAus liest die Serie aus dem Verlauf (neueste zuerst). Jede Runde mit einem Ergebnis in der Sache (eingereicht,
// bewertet, aus Gründen der Aufgabe zurückgegeben) oder mit einer anderen Ursache beendet die Serie. Nichts sagen und
// deshalb weder zählen noch unterbrechen: angehaltene Runden, und Fremdbewertungen und Medien-Aufträge, die ohne
// erkannte Ursache scheiterten (dort hängt es oft am Auftrag oder am Medienwerkzeug). Liegen zwei Abstürze mehr als
// zwei Tage auseinander, gehören sie nicht zur selben Serie.
func serieAus(eintraege []verlaufEintrag) serie {
	var s serie
	davor := time.Time{}
	for _, e := range eintraege {
		if e.Ursache == "stopped" || e.Ursache == "" && (e.Ergebnis == "review_not_sent" || e.Brief > 0 && e.Ergebnis == "returned") {
			continue
		}
		if s.Anzahl > 0 && davor.Sub(e.Zeit) > 48*time.Hour {
			break
		}
		if s.Anzahl == 0 {
			if e.Ursache == "" {
				return s
			}
			s = serie{Schluessel: e.Ursache, Anzahl: 1, Zeit: e.Zeit, Grund: e.Grund}
			if amProjekt(e.Ursache) {
				s.Projekt = e.Projekt
			}
			davor = e.Zeit
			continue
		}
		// Runden anderer Projekte sagen nichts über die Umgebung dieses Projekts, und der Absturz an der Umgebung eines
		// Projekts nichts über den Runner.
		if s.Projekt != "" && e.Projekt != s.Projekt || s.Projekt == "" && amProjekt(e.Ursache) {
			continue
		}
		if e.Ursache != s.Schluessel {
			break
		}
		s.Anzahl++
		davor = e.Zeit
	}
	return s
}

// serieGrenze: Ab so vielen gleichen Abstürzen in Folge setzt der Dauerbetrieb aus.
const serieGrenze = 3

// pause: wie lange der Dauerbetrieb nach dieser Serie aussetzt. Ab drei gleichen Abstürzen eine Stunde, mit jedem
// weiteren doppelt so lang, höchstens zwölf Stunden. Nach der Pause ist eine Runde der Versuch: Stürzt sie wieder so
// ab, wächst die Serie; kommt sie durch, ist sie beendet.
func (s serie) pause() time.Duration {
	if s.Anzahl < serieGrenze {
		return 0
	}
	return min(time.Hour<<min(s.Anzahl-serieGrenze, 4), 12*time.Hour)
}

// projektPause: ein Projekt, das der Dauerbetrieb mit --project auto ausgesetzt hat. Der Dienst arbeitet dann mit den
// anderen Projekten weiter; damit `swarvex status` die Pause trotzdem zeigt (nicht nur das Journal), schreibt er sie nach
// paused-projects.json. Die Pausen selbst stehen nur in seinem Speicher: Beim Start löscht er die Datei.
type projektPause struct {
	Projekt string    `json:"project"`
	Bis     time.Time `json:"until"`
	Grund   string    `json:"reason"`
}

func projektPausenDatei() string { return filepath.Join(datenOrdner(), "paused-projects.json") }

// projektPausenMerken schreibt die laufenden Pausen (ohne: löscht die Datei).
func projektPausenMerken(pausen map[string]time.Time, gruende map[string]string) {
	if len(pausen) == 0 {
		os.Remove(projektPausenDatei())
		return
	}
	l := []projektPause{}
	for p, bis := range pausen {
		l = append(l, projektPause{Projekt: p, Bis: bis, Grund: gruende[p]})
	}
	sort.Slice(l, func(i, j int) bool { return l[i].Projekt < l[j].Projekt })
	roh, _ := json.Marshal(l)
	os.MkdirAll(datenOrdner(), 0o755)
	os.WriteFile(projektPausenDatei(), roh, 0o644)
}

// projektPausenLesen: die Pausen, die zu diesem Zeitpunkt noch laufen.
func projektPausenLesen(jetzt time.Time) []projektPause {
	roh, err := os.ReadFile(projektPausenDatei())
	var l, r []projektPause
	if err != nil || json.Unmarshal(roh, &l) != nil {
		return nil
	}
	for _, p := range l {
		if p.Bis.After(jetzt) {
			r = append(r, p)
		}
	}
	return r
}

// text: die Serie in einem Satz für Journal und `swarvex status`.
func (s serie) text() string {
	was := schluesselText(s.Schluessel)
	if was == "" {
		was = s.Schluessel
	}
	return fmt.Sprintf("%d rounds in a row ended the same way: %s", s.Anzahl, was)
}
