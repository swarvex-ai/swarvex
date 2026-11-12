package main

// swarvex status, für Menschen gegliedert (Wunsch von neVs, 12.11.2026): oben wer und welche Version, dann was der
// Dienst gerade tut, woran und wann er arbeitet, mit welchem Modell, wie viel heute verbraucht ist, wie viel Platz die
// Arbeitsdaten brauchen und die letzten Meldungen mit Uhrzeit. Was die Zeilen sagen, stammt aus den Startargumenten
// des Dienstes (ohne Dienst aus den Einstellungen von swarvex setup); einen Schlüssel liest nichts davon.

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const statusEinzug = 16 // Breite der Spalte mit den Bezeichnungen, samt zwei Leerzeichen davor

func statusBefehl() {
	jetzt := time.Now()
	breite := terminalBreite()
	einheit, _ := os.ReadFile(dienstDatei())
	args, _ := einheitArgsLesen(string(einheit))
	if len(args) == 0 {
		args = gespeicherteArgs()
	}
	zeile := func(bezeichnung, wert string) {
		fmt.Println(umbrechen(fmt.Sprintf("  %-*s%s", statusEinzug-2, bezeichnung, wert), breite, statusEinzug))
	}
	abschnitt := func(titel string) { fmt.Printf("\n%s\n", titel) }

	// Wer und welche Version
	kopf, update := "swarvex runner "+versionText(version), ""
	if z, err := zugangLaden(); err == nil {
		kopf += " · signed in at " + z.Plattform
		if skript, err := installerHolen(z.Plattform, z.CAPfad); err == nil {
			if neu, _, _ := installerLesen(skript); istNeuer(neu, version) {
				update = fmt.Sprintf("Update available: %s → swarvex update", versionText(neu))
			}
		}
	} else {
		kopf += " · not signed in (swarvex login)"
	}
	fmt.Println(kopf)
	if update != "" {
		fmt.Println(update)
	}

	// Was der Dienst gerade tut
	if runtime.GOOS == "linux" {
		abschnitt("Service")
		if _, err := os.Stat(dienstDatei()); err != nil {
			zeile("State", "not set up (swarvex setup)")
		} else {
			zustand, _ := systemctl("is-active", dienstName)
			zeile("State", dienstZustandText(zustand))
			if zustand == "active" {
				if w, ok := wartenLesen(jetzt); ok {
					if w.Bis.IsZero() {
						zeile("Waiting", w.Grund)
					} else {
						zeile("Waiting", "until "+zeitpunkt(w.Bis)+": "+w.Grund)
					}
				}
				for _, p := range projektPausenLesen(jetzt) {
					zeile("Paused", fmt.Sprintf("%s until %s: %s (other projects continue; to try again at once: swarvex stop, then swarvex start)",
						p.Projekt, zeitpunkt(p.Bis), p.Grund))
				}
			}
		}
	}
	if len(args) > 0 {
		abschnitt("Work")
		for _, z := range arbeitZeilen(args) {
			zeile(z[0], z[1])
		}
		abschnitt("Model")
		for _, z := range modellZeilen(args) {
			zeile(z[0], z[1])
		}
	}

	// Verbrauch heute
	abschnitt("Today")
	tagesbudget, _ := strconv.ParseInt(argWertOder(args, "daily-budget-tokens", "0"), 10, 64)
	zeile("Tokens", verbrauchText(verbrauchAmTag(verbrauchsDatei(), jetzt), tagesbudget))
	if n, _ := strconv.ParseInt(argWertOder(args, "budget-tokens", "0"), 10, 64); n > 0 {
		zeile("Per task", "at most "+tausender(n)+" tokens")
	}
	// Der neueste Stand, auch aus den Lead-Diensten mit demselben Abo
	if k, ok := kontingentAktuell(aboKennung(argWertOder(args, "model-key-file", ""))); ok {
		zeile("Subscription", "as of "+uhrzeit(k.Zeit, jetzt)+": "+kontingentText(k, jetzt))
	}

	abschnitt("Disk")
	for _, z := range datenZeilen() {
		zeile(z[0], z[1])
	}

	if runtime.GOOS != "linux" {
		return
	}
	if aus, err := exec.Command("journalctl", "--user", "-u", "swarvex", "-n", "120", "--no-pager", "-o", "json").Output(); err == nil {
		if ms := journalMeldungen(string(aus), 6); len(ms) > 0 {
			abschnitt("Latest messages")
			spalte := 0
			for _, m := range ms {
				spalte = max(spalte, len(uhrzeit(m.Zeit, jetzt)))
			}
			for _, m := range ms {
				vorn := fmt.Sprintf("  %-*s  ", spalte, uhrzeit(m.Zeit, jetzt))
				fmt.Println(umbrechen(vorn+m.Text, breite, len(vorn)))
			}
		}
	}
}

func argWertOder(args []string, name, vorgabe string) string {
	if w, ok := argWert(args, name); ok && w != "" {
		return w
	}
	return vorgabe
}

// argGesetzt: ein Schalter ohne Wert (--only-when-idle), oder ausdrücklich =true.
func argGesetzt(args []string, name string) bool {
	an := false
	for _, a := range args {
		n, w, mitGleich := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if strings.HasPrefix(a, "-") && n == name {
			an = !mitGleich || w == "true"
		}
	}
	return an
}

func dienstZustandText(zustand string) string {
	if h := dienstHinweis(zustand); h != "" {
		return h
	}
	if zustand == "active" {
		return "running"
	}
	return zustand
}

// arbeitZeilen: woran und wann der Runner arbeitet.
func arbeitZeilen(args []string) [][2]string {
	var z [][2]string
	projekt := argWertOder(args, "project", "")
	switch projekt {
	case "":
		projekt = "none set (swarvex setup)"
	case "auto":
		projekt = "auto (wherever most tasks are open)"
		if arten := argWertOder(args, "kinds", ""); arten != "" && arten != "all" {
			projekt += ", kinds " + strings.ReplaceAll(arten, ",", ", ")
		}
	}
	z = append(z, [2]string{"Project", projekt})
	zeiten := argWertOder(args, "schedule", "")
	if zeiten == "" {
		zeiten = "always"
	} else {
		zeiten = strings.ReplaceAll(zeiten, ";", ",")
	}
	if argGesetzt(args, "only-when-idle") {
		zeiten += ", only while nobody uses this computer"
	}
	z = append(z, [2]string{"Hours", zeiten})
	switch {
	case argGesetzt(args, "cross-checks-only"):
		z = append(z, [2]string{"Mode", "only cross-checks of other people's submissions (no model)"})
	case argGesetzt(args, "leftover"):
		z = append(z, [2]string{"Mode", "leftover: only what the subscription would let expire before its weekly reset (beta)"})
	case argWertOder(args, "subscription-share", "0") != "0":
		z = append(z, [2]string{"Mode", "evenly over the week up to " + argWertOder(args, "subscription-share", "0") + " % of the subscription's weekly quota"})
	}
	if w, ok := argWert(args, "peer-reviews"); ok && w == "false" {
		z = append(z, [2]string{"Peer reviews", "off"})
	}
	if m := argWertOder(args, "media", ""); m != "" && m != "none" {
		z = append(z, [2]string{"Media", strings.ReplaceAll(m, ",", ", ") + ", at most " + argWertOder(args, "media-daily", strconv.Itoa(medienStandard)) + " briefs a day"})
	}
	return z
}

// modellZeilen: welches Modell, über welche Verbindung, mit welchem Agenten. Ohne --model nimmt opencode sein eigenes
// Standardmodell.
func modellZeilen(args []string) [][2]string {
	agent := argWertOder(args, "agent", "opencode")
	modell := argWertOder(args, "model", "")
	if modell == "" && agent == "opencode" {
		if m := opencodeStandardModell(); m != "" {
			modell = m + " (opencode default)"
		}
	}
	if modell == "" {
		modell = "the agent's default"
	}
	return [][2]string{{"Model", modell}, {"Connection", verbindungText(agent, argWertOder(args, "model-url", ""))}, {"Agent", agent}}
}

// verbindungText: der Anbieter hinter der Adresse, mit der Adresse selbst.
func verbindungText(agent, adresse string) string {
	if adresse == "" {
		switch agent {
		case "claude":
			return "Anthropic"
		case "codex":
			return "OpenAI"
		}
		return "the agent's default"
	}
	if strings.HasPrefix(adresse, codexAboURL) {
		return "ChatGPT subscription"
	}
	u, err := url.Parse(adresse)
	if err != nil || u.Hostname() == "" {
		return adresse
	}
	host, name := u.Hostname(), u.Hostname()
	ip := net.ParseIP(host)
	switch {
	case host == "openrouter.ai" || strings.HasSuffix(host, ".openrouter.ai"):
		name = "OpenRouter"
	case host == "api.anthropic.com":
		name = "Anthropic"
	case host == "api.openai.com":
		name = "OpenAI"
	case host == "localhost" || (ip != nil && (ip.IsLoopback() || ip.IsPrivate())):
		name = "local server"
	}
	return name + " (" + adresse + ")"
}

// verbrauchText: heute verbrauchte Token, mit dem Tagesbudget, wenn es eins gibt.
func verbrauchText(verbraucht, budget int64) string {
	if budget <= 0 {
		return tausender(verbraucht) + " (no daily budget)"
	}
	text := tausender(verbraucht) + " of " + tausender(budget) + " a day"
	if verbraucht >= budget {
		return text + ", used up"
	}
	return text + fmt.Sprintf(" (%d %%)", verbraucht*100/budget)
}

// uhrzeit: heute nur die Uhrzeit, sonst mit Tag.
func uhrzeit(t, jetzt time.Time) string {
	if t.Year() == jetzt.Year() && t.YearDay() == jetzt.YearDay() {
		return t.Format("15:04")
	}
	return zeitpunkt(t)
}

type meldung struct {
	Zeit time.Time
	Text string
}

var dienstStart = regexp.MustCompile(`^(Started|Stopped|Starting|Stopping) swarvex\.service\b`)

// journalMeldungen: die letzten n eigenen Zeilen des Runners aus `journalctl -o json`, mit Zeit. Nur was der Runner
// selbst schreibt und Start und Stopp des Dienstes (kurz); Podman, conmon und systemd-Statistik fallen weg.
func journalMeldungen(aus string, n int) []meldung {
	var ms []meldung
	for _, l := range strings.Split(aus, "\n") {
		var j struct {
			Zeit   string `json:"__REALTIME_TIMESTAMP"`
			Text   any    `json:"MESSAGE"`
			Quelle string `json:"SYSLOG_IDENTIFIER"`
		}
		if json.Unmarshal([]byte(l), &j) != nil {
			continue
		}
		if j.Quelle != "" && j.Quelle != "swarvex" && j.Quelle != "systemd" {
			continue
		}
		text, ok := j.Text.(string)
		if !ok {
			continue
		}
		us, _ := strconv.ParseInt(j.Zeit, 10, 64)
		for _, t := range letzteMeldungen(text, 1<<30) {
			if m := dienstStart.FindStringSubmatch(t); m != nil {
				if m[1] != "Started" && m[1] != "Stopped" {
					continue
				}
				t = "Service " + strings.ToLower(m[1])
			}
			ms = append(ms, meldung{time.UnixMicro(us), t})
		}
	}
	if len(ms) > n {
		ms = ms[len(ms)-n:]
	}
	return ms
}

// umbrechen: Zeilen länger als breite an Leerzeichen umbrechen, Folgezeilen um einzug eingerückt.
func umbrechen(text string, breite, einzug int) string {
	if breite <= einzug+20 || len([]rune(text)) <= breite {
		return text
	}
	var b strings.Builder
	zeile := 0
	for i, wort := range strings.Split(text, " ") {
		laenge := len([]rune(wort))
		if i > 0 && zeile+1+laenge > breite && zeile > einzug {
			b.WriteString("\n" + strings.Repeat(" ", einzug))
			zeile = einzug
		} else if i > 0 {
			b.WriteString(" ")
			zeile++
		}
		b.WriteString(wort)
		zeile += laenge
	}
	return b.String()
}

// terminalBreite: COLUMNS, sonst stty, sonst 100; nie unter 60 oder über 160.
func terminalBreite() int {
	n, _ := strconv.Atoi(os.Getenv("COLUMNS"))
	if n == 0 {
		cmd := exec.Command("stty", "size")
		cmd.Stdin = os.Stdin
		if aus, err := cmd.Output(); err == nil {
			if teile := strings.Fields(string(aus)); len(teile) == 2 {
				n, _ = strconv.Atoi(teile[1])
			}
		}
	}
	switch {
	case n == 0:
		return 100
	case n < 60:
		return 60
	case n > 160:
		return 160
	}
	return n
}
