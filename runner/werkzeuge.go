package main

// Medien-Werkzeuge finden: die MCP-Server, die Mitstreiter in ihren Agenten eingetragen haben (Claude Code, Hermes,
// Codex, opencode, Gemini CLI), und davon die Medien-Dienste (Meshy, ElevenLabs …). Gespeichert wird nur, welcher
// Server genutzt wird (--media claude:meshy,hermes:elevenlabs); Befehl, Adresse und Schlüssel liest der Runner bei
// jedem Auftrag frisch aus der Konfiguration des Agenten, sie bleiben dort.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// medienServer: ein MCP-Server aus der Konfiguration eines Agenten, der zu einem Medien-Dienst gehört.
type medienServer struct {
	Quelle string            // claude, hermes, codex, opencode, gemini
	Name   string            // Name des Servers in dieser Konfiguration
	Dienst string            // Dienst der Plattform (meshy, elevenlabs, …)
	Befehl []string          // lokal (stdio): Programm und Argumente
	Env    map[string]string // lokal: Umgebung, darin meist der Schlüssel
	URL    string            // entfernt (Streamable HTTP)
	Kopf   map[string]string // entfernt: Kopfzeilen, darin meist der Token
	Fehlt  string            // warum er gerade nicht nutzbar ist; leer = nutzbar
}

func (s medienServer) Kennung() string { return s.Quelle + ":" + s.Name }

var quellenTitel = map[string]string{"claude": "Claude Code", "hermes": "Hermes", "codex": "Codex", "opencode": "opencode", "gemini": "Gemini CLI"}

// Titel für Meldungen: „Meshy (Claude Code)“.
func (s medienServer) Titel() string {
	return dienstTitel(s.Dienst) + " (" + quellenTitel[s.Quelle] + ")"
}

// medienDienste: Dienste der Plattform (apps/api/src/spenden.ts, DIENSTE) mit den Wörtern, an denen ein Server sie zeigt.
var medienDienste = []struct {
	Dienst, Titel string
	Woerter       []string
}{
	{"meshy", "Meshy", []string{"meshy"}},
	{"elevenlabs", "ElevenLabs", []string{"elevenlabs", "eleven-labs", "eleven_labs", "11labs"}},
	{"suno", "Suno", []string{"suno"}},
	{"runway", "Runway", []string{"runway"}},
	{"higgsfield", "Higgsfield", []string{"higgsfield"}},
	{"leonardo", "Leonardo", []string{"leonardo"}},
	{"kling", "Kling", []string{"kling"}},
	{"midjourney", "Midjourney", []string{"midjourney"}},
}

func dienstTitel(d string) string {
	for _, m := range medienDienste {
		if m.Dienst == d {
			return m.Titel
		}
	}
	return d
}

// dienstFuer: welcher Medien-Dienst ein Server ist (Name, Befehl, Paket, Adresse); "" = keiner.
func dienstFuer(name string, befehl []string, url string) string {
	text := strings.ToLower(name + " " + strings.Join(befehl, " ") + " " + url)
	for _, m := range medienDienste {
		for _, w := range m.Woerter {
			if strings.Contains(text, w) {
				return m.Dienst
			}
		}
	}
	return ""
}

// medienServerFinden: alle Medien-Server aus den Konfigurationen dieses Rechners, sortiert nach Kennung.
func medienServerFinden() []medienServer {
	var r []medienServer
	gesehen := map[string]bool{}
	for _, q := range []struct {
		quelle string
		lesen  func() []medienServer
	}{{"claude", claudeServer}, {"hermes", hermesServer}, {"codex", codexServer}, {"opencode", opencodeServer}, {"gemini", geminiServer}} {
		for _, s := range q.lesen() {
			s.Quelle = q.quelle
			if s.Dienst = dienstFuer(s.Name, s.Befehl, s.URL); s.Dienst == "" || gesehen[s.Kennung()] {
				continue
			}
			gesehen[s.Kennung()] = true
			if fehlt := s.aufloesen(q.quelle); s.Fehlt == "" {
				s.Fehlt = fehlt
			}
			r = append(r, s)
		}
	}
	sort.Slice(r, func(i, j int) bool { return r[i].Kennung() < r[j].Kennung() })
	return r
}

// medienAuswahl: die Server aus --media (Kennungen, durch Komma getrennt), in dieser Reihenfolge; fehlende sind nicht dabei.
func medienAuswahl(spec string, alle []medienServer) []medienServer {
	var r []medienServer
	for _, k := range medienKennungen(spec) {
		for _, s := range alle {
			if s.Kennung() == k {
				r = append(r, s)
			}
		}
	}
	return r
}

func medienKennungen(spec string) []string {
	var r []string
	for _, k := range strings.Split(spec, ",") {
		if k = strings.TrimSpace(k); k != "" && k != "none" {
			r = append(r, k)
		}
	}
	return r
}

func medienSpec(server []medienServer) string {
	k := make([]string, len(server))
	for i, s := range server {
		k[i] = s.Kennung()
	}
	return strings.Join(k, ",")
}

// ---------- Umgebungsvariablen in Werten (${VAR}, $VAR, {env:VAR}) ----------

var envVerweis = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\{env:([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

// medienEnvDatei: Schlüssel, die der Dienst (systemd) sonst nicht sähe, weil sie nur in der Shell exportiert sind.
func medienEnvDatei() string { return filepath.Join(konfigOrdner(), "media.env") }

// aufloesen ersetzt Verweise auf Umgebungsvariablen (Prozess, ~/.config/swarvex/media.env, bei Hermes ~/.hermes/.env).
// Rückgabe: was fehlt ("" = alles da).
func (s *medienServer) aufloesen(quelle string) string {
	quellen := []map[string]string{envDatei(medienEnvDatei())}
	if quelle == "hermes" {
		quellen = append(quellen, envDatei(heim(".hermes", ".env")))
	}
	wert := func(n string) (string, bool) {
		if v, ok := os.LookupEnv(n); ok && v != "" {
			return v, true
		}
		for _, q := range quellen {
			if v := q[n]; v != "" {
				return v, true
			}
		}
		return "", false
	}
	var fehlen []string
	ersetzen := func(v string) string {
		return envVerweis.ReplaceAllStringFunc(v, func(m string) string {
			t := envVerweis.FindStringSubmatch(m)
			n := t[1] + t[2] + t[3]
			if w, ok := wert(n); ok {
				return w
			}
			fehlen = append(fehlen, n)
			return ""
		})
	}
	for i, a := range s.Befehl {
		s.Befehl[i] = ersetzen(a)
	}
	for k, v := range s.Env {
		if strings.HasPrefix(v, "@env:") { // Codex env_vars / bearer_token_env_var: der Wert kommt aus der Umgebung
			n := strings.TrimPrefix(v, "@env:")
			if w, ok := wert(n); ok {
				s.Env[k] = w
			} else {
				fehlen, s.Env[k] = append(fehlen, n), ""
			}
			continue
		}
		s.Env[k] = ersetzen(v)
	}
	for k, v := range s.Kopf {
		if strings.HasPrefix(v, "@env:") {
			n := strings.TrimPrefix(v, "@env:")
			if w, ok := wert(n); ok {
				s.Kopf[k] = w
				if k == "Authorization" && !strings.Contains(w, " ") { // bearer_token_env_var: nur der Token
					s.Kopf[k] = "Bearer " + w
				}
			} else {
				fehlen, s.Kopf[k] = append(fehlen, n), ""
			}
			continue
		}
		s.Kopf[k] = ersetzen(v)
	}
	s.URL = ersetzen(s.URL)
	switch {
	case len(fehlen) > 0:
		return fmt.Sprintf("needs %s, which the runner does not see; put it into %s (NAME=value)", strings.Join(fehlen, ", "), medienEnvDatei())
	case len(s.Befehl) == 0 && s.URL == "":
		return "neither a command nor an address"
	}
	return ""
}

// ---------- Claude Code (~/.claude.json), Gemini CLI (~/.gemini/settings.json) ----------

type jsonServer struct {
	Typ     string            `json:"type"`
	Befehl  json.RawMessage   `json:"command"` // Text (Claude, Gemini) oder Liste (opencode)
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Umgeb   map[string]string `json:"environment"` // opencode
	URL     string            `json:"url"`
	HTTPURL string            `json:"httpUrl"` // Gemini: Streamable HTTP
	Kopf    map[string]string `json:"headers"`
	An      *bool             `json:"enabled"` // opencode
}

// server: urlIstSSE bei Gemini CLI (url = SSE, httpUrl = Streamable HTTP); bei Claude Code sagt es type.
func (j jsonServer) server(name string, urlIstSSE bool) (medienServer, bool) {
	s := medienServer{Name: name, Env: map[string]string{}, Kopf: map[string]string{}}
	if j.An != nil && !*j.An {
		return s, false
	}
	var text string
	var liste []string
	if json.Unmarshal(j.Befehl, &text) == nil && text != "" {
		s.Befehl = append([]string{text}, j.Args...)
	} else if json.Unmarshal(j.Befehl, &liste) == nil && len(liste) > 0 {
		s.Befehl = append(liste, j.Args...)
	}
	for k, v := range j.Env {
		s.Env[k] = v
	}
	for k, v := range j.Umgeb {
		s.Env[k] = v
	}
	for k, v := range j.Kopf {
		s.Kopf[k] = v
	}
	s.URL = j.HTTPURL
	if s.URL == "" && j.Typ != "sse" && !urlIstSSE {
		s.URL = j.URL
	}
	if len(s.Befehl) > 0 {
		s.URL = ""
	}
	if len(s.Befehl) == 0 && s.URL == "" && j.URL != "" {
		s.Fehlt = "only the older SSE transport; the runner speaks Streamable HTTP"
	}
	return s, true
}

func jsonServerAus(eintraege map[string]jsonServer, urlIstSSE bool) []medienServer {
	var r []medienServer
	for name, j := range eintraege {
		if s, ok := j.server(name, urlIstSSE); ok {
			r = append(r, s)
		}
	}
	return r
}

func claudeServer() []medienServer {
	var k struct {
		Server   map[string]jsonServer `json:"mcpServers"`
		Projekte map[string]struct {
			Server map[string]jsonServer `json:"mcpServers"`
		} `json:"projects"`
	}
	if json.Unmarshal([]byte(lesen(heim(".claude.json"))), &k) != nil {
		return nil
	}
	r := jsonServerAus(k.Server, false)
	for _, p := range k.Projekte { // Server, die nur in einem Projekt eingetragen sind (Bereich „local“)
		r = append(r, jsonServerAus(p.Server, false)...)
	}
	return r
}

func geminiServer() []medienServer {
	var k struct {
		Server map[string]jsonServer `json:"mcpServers"`
	}
	if json.Unmarshal([]byte(lesen(heim(".gemini", "settings.json"))), &k) != nil {
		return nil
	}
	return jsonServerAus(k.Server, true)
}

// ---------- opencode (~/.config/opencode/opencode.json[c]) ----------

func opencodeServer() []medienServer {
	for _, n := range []string{"opencode.json", "opencode.jsonc", "config.json"} {
		roh := lesen(heim(".config", "opencode", n))
		if roh == "" {
			continue
		}
		var k struct {
			Server map[string]jsonServer `json:"mcp"`
		}
		if json.Unmarshal([]byte(ohneKommentare(roh)), &k) == nil {
			return jsonServerAus(k.Server, false)
		}
	}
	return nil
}

// ohneKommentare entfernt // und /* */ außerhalb von Zeichenketten sowie Kommas vor } und ] (JSONC).
func ohneKommentare(s string) string {
	var b strings.Builder
	inText := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inText:
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
			} else if c == '"' {
				inText = false
			}
		case c == '"':
			inText = true
			b.WriteByte(c)
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			for i < len(s) && s[i] != '\n' {
				i++
			}
			b.WriteByte('\n')
		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			ende := strings.Index(s[i+2:], "*/")
			if ende < 0 {
				return b.String()
			}
			i += ende + 3
		default:
			b.WriteByte(c)
		}
	}
	return regexp.MustCompile(`,(\s*[}\]])`).ReplaceAllString(b.String(), "$1")
}

// ---------- Codex (~/.codex/config.toml) ----------

func codexServer() []medienServer {
	tabellen := tomlTabellen(lesen(heim(".codex", "config.toml")))
	namen := map[string]bool{}
	for t := range tabellen {
		if teile := tomlSchluessel(t); len(teile) >= 2 && teile[0] == "mcp_servers" {
			namen[teile[1]] = true
		}
	}
	var r []medienServer
	for name := range namen {
		t := tabellen["mcp_servers."+tomlKeyText(name)]
		if t["enabled"] == "false" {
			continue
		}
		s := medienServer{Name: name, Env: map[string]string{}, Kopf: map[string]string{}}
		if b := t["command"]; b != "" {
			s.Befehl = append([]string{tomlWert(b)}, tomlListe(t["args"])...)
		}
		for k, v := range tomlInline(t["env"]) {
			s.Env[k] = v
		}
		for k, v := range tabellen["mcp_servers."+tomlKeyText(name)+".env"] {
			s.Env[tomlWert(k)] = tomlWert(v)
		}
		for _, n := range tomlListe(t["env_vars"]) {
			s.Env[n] = "@env:" + n
		}
		s.URL = tomlWert(t["url"])
		for k, v := range tomlInline(t["http_headers"]) {
			s.Kopf[k] = v
		}
		for k, v := range tomlInline(t["env_http_headers"]) {
			s.Kopf[k] = "@env:" + v
		}
		if n := tomlWert(t["bearer_token_env_var"]); n != "" {
			s.Kopf["Authorization"] = "@env:" + n
		}
		r = append(r, s)
	}
	return r
}

// tomlTabellen: die Untermenge von TOML, die Codex für MCP-Server nutzt: [tabelle.name], schluessel = wert
// (Text, Liste, Inline-Tabelle, auch über mehrere Zeilen). Werte bleiben roh; tomlWert & Co. lesen sie.
func tomlTabellen(text string) map[string]map[string]string {
	r := map[string]map[string]string{}
	aktuell := ""
	r[aktuell] = map[string]string{}
	zeilen := strings.Split(text, "\n")
	for i := 0; i < len(zeilen); i++ {
		z := strings.TrimSpace(tomlOhneKommentar(zeilen[i]))
		if z == "" {
			continue
		}
		if strings.HasPrefix(z, "[") && !strings.HasPrefix(z, "[[") {
			aktuell = tomlTabellenName(z)
			if r[aktuell] == nil {
				r[aktuell] = map[string]string{}
			}
			continue
		}
		k, v, ok := strings.Cut(z, "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		for tomlOffen(v) && i+1 < len(zeilen) { // mehrzeilige Liste oder Tabelle
			i++
			v += " " + strings.TrimSpace(tomlOhneKommentar(zeilen[i]))
		}
		r[aktuell][strings.TrimSpace(k)] = v
	}
	return r
}

// tomlTabellenName: Schlüsselteile mit Punkten oder Leerzeichen bleiben in Anführungszeichen, damit sie eindeutig sind.
func tomlTabellenName(kopf string) string {
	teile := tomlSchluessel(strings.Trim(strings.TrimSpace(kopf), "[] "))
	for i, t := range teile {
		teile[i] = tomlKeyText(t)
	}
	return strings.Join(teile, ".")
}

func tomlKeyText(t string) string {
	if regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(t) {
		return t
	}
	return strconv.Quote(t)
}

// tomlSchluessel teilt a."b.c".d in [a b.c d].
func tomlSchluessel(s string) []string {
	var r []string
	var b strings.Builder
	in := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case in != 0 && c == in:
			in = 0
		case in != 0:
			b.WriteByte(c)
		case c == '"' || c == '\'':
			in = c
		case c == '.':
			r = append(r, strings.TrimSpace(b.String()))
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}
	return append(r, strings.TrimSpace(b.String()))
}

func tomlOhneKommentar(z string) string {
	in := byte(0)
	for i := 0; i < len(z); i++ {
		c := z[i]
		switch {
		case in != 0 && c == '\\' && in == '"':
			i++
		case in != 0 && c == in:
			in = 0
		case in == 0 && (c == '"' || c == '\''):
			in = c
		case in == 0 && c == '#':
			return z[:i]
		}
	}
	return z
}

func tomlOffen(v string) bool {
	tiefe, in := 0, byte(0)
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case in != 0 && c == '\\' && in == '"':
			i++
		case in != 0 && c == in:
			in = 0
		case in == 0 && (c == '"' || c == '\''):
			in = c
		case in == 0 && (c == '[' || c == '{'):
			tiefe++
		case in == 0 && (c == ']' || c == '}'):
			tiefe--
		}
	}
	return tiefe > 0
}

// tomlWert: ein Text ("…" mit Escapes, '…' wörtlich) oder der rohe Wert.
func tomlWert(v string) string {
	v = strings.TrimSpace(v)
	switch {
	case strings.HasPrefix(v, `"`):
		if s, err := strconv.Unquote(v); err == nil {
			return s
		}
		return strings.Trim(v, `"`)
	case strings.HasPrefix(v, "'"):
		return strings.Trim(v, "'")
	}
	return v
}

// tomlTeile teilt den Inhalt einer Liste oder Inline-Tabelle an Kommas außerhalb von Texten.
func tomlTeile(innen string) []string {
	var r []string
	var b strings.Builder
	in := byte(0)
	for i := 0; i < len(innen); i++ {
		c := innen[i]
		switch {
		case in != 0 && c == '\\' && in == '"' && i+1 < len(innen):
			b.WriteByte(c)
			i++
			c = innen[i]
		case in != 0 && c == in:
			in = 0
		case in == 0 && (c == '"' || c == '\''):
			in = c
		case in == 0 && c == ',':
			if t := strings.TrimSpace(b.String()); t != "" {
				r = append(r, t)
			}
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	if t := strings.TrimSpace(b.String()); t != "" {
		r = append(r, t)
	}
	return r
}

func tomlListe(v string) []string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "[") {
		return nil
	}
	var r []string
	for _, t := range tomlTeile(strings.TrimSuffix(strings.TrimPrefix(v, "["), "]")) {
		r = append(r, tomlWert(t))
	}
	return r
}

func tomlInline(v string) map[string]string {
	v = strings.TrimSpace(v)
	r := map[string]string{}
	if !strings.HasPrefix(v, "{") {
		return r
	}
	for _, t := range tomlTeile(strings.TrimSuffix(strings.TrimPrefix(v, "{"), "}")) {
		if k, w, ok := strings.Cut(t, "="); ok {
			r[tomlWert(k)] = tomlWert(w)
		}
	}
	return r
}

// ---------- Hermes (~/.hermes/config.yaml) ----------

func hermesServer() []medienServer {
	baum, ok := yamlBaum(lesen(heim(".hermes", "config.yaml")))["mcp_servers"].(map[string]any)
	if !ok {
		return nil
	}
	var r []medienServer
	for name, roh := range baum {
		e, ok := roh.(map[string]any)
		if !ok || fmt.Sprint(e["enabled"]) == "false" {
			continue
		}
		s := medienServer{Name: name, Env: map[string]string{}, Kopf: map[string]string{}}
		if b, ok := e["command"].(string); ok && b != "" {
			s.Befehl = append([]string{b}, yamlTexte(e["args"])...)
		}
		if m, ok := e["env"].(map[string]any); ok {
			for k, v := range m {
				s.Env[k] = fmt.Sprint(v)
			}
		}
		if u, ok := e["url"].(string); ok {
			s.URL = u
		}
		if m, ok := e["headers"].(map[string]any); ok {
			for k, v := range m {
				s.Kopf[k] = fmt.Sprint(v)
			}
		}
		r = append(r, s)
	}
	return r
}

func yamlTexte(v any) []string {
	l, _ := v.([]any)
	r := make([]string, 0, len(l))
	for _, x := range l {
		r = append(r, fmt.Sprint(x))
	}
	return r
}

// yamlBaum: die Untermenge von YAML in Agenten-Konfigurationen: verschachtelte Zuordnungen über Einrückung,
// Listen mit „- “, Listen in [a, b] und Werte in Anführungszeichen. Mehrzeilige Texte (|, >) werden übersprungen.
func yamlBaum(text string) map[string]any {
	type zeile struct {
		tiefe int
		text  string
	}
	var zeilen []zeile
	for _, z := range strings.Split(text, "\n") {
		inhalt := strings.TrimRight(yamlOhneKommentar(z), " \t\r")
		if strings.TrimSpace(inhalt) == "" || strings.HasPrefix(strings.TrimSpace(inhalt), "---") {
			continue
		}
		zeilen = append(zeilen, zeile{len(inhalt) - len(strings.TrimLeft(inhalt, " ")), strings.TrimSpace(inhalt)})
	}
	var block func(i, tiefe int) (any, int)
	block = func(i, tiefe int) (any, int) {
		if i < len(zeilen) && strings.HasPrefix(zeilen[i].text, "- ") || i < len(zeilen) && zeilen[i].text == "-" {
			var liste []any
			for i < len(zeilen) && zeilen[i].tiefe == tiefe && (strings.HasPrefix(zeilen[i].text, "- ") || zeilen[i].text == "-") {
				rest := strings.TrimSpace(strings.TrimPrefix(zeilen[i].text, "-"))
				i++
				if rest == "" {
					if i < len(zeilen) && zeilen[i].tiefe > tiefe {
						var v any
						v, i = block(i, zeilen[i].tiefe)
						liste = append(liste, v)
					}
					continue
				}
				if k, v, ok := yamlPaar(rest); ok { // - name: wert (Zuordnung in der Liste)
					m := map[string]any{k: yamlSkalar(v)}
					if i < len(zeilen) && zeilen[i].tiefe > tiefe {
						var mehr any
						mehr, i = block(i, zeilen[i].tiefe)
						if mm, ok := mehr.(map[string]any); ok {
							for a, b := range mm {
								m[a] = b
							}
						}
					}
					liste = append(liste, m)
					continue
				}
				liste = append(liste, yamlSkalar(rest))
			}
			return liste, i
		}
		m := map[string]any{}
		for i < len(zeilen) && zeilen[i].tiefe == tiefe {
			k, v, ok := yamlPaar(zeilen[i].text)
			i++
			if !ok {
				continue
			}
			switch {
			case v == "|" || v == ">" || strings.HasPrefix(v, "|") || strings.HasPrefix(v, ">"):
				for i < len(zeilen) && zeilen[i].tiefe > tiefe {
					i++
				}
			case v == "":
				if i < len(zeilen) && (zeilen[i].tiefe > tiefe || (zeilen[i].tiefe == tiefe && strings.HasPrefix(zeilen[i].text, "- "))) {
					m[k], i = block(i, zeilen[i].tiefe)
				} else {
					m[k] = ""
				}
			default:
				m[k] = yamlSkalar(v)
			}
		}
		return m, i
	}
	if len(zeilen) == 0 {
		return map[string]any{}
	}
	wurzel, _ := block(0, zeilen[0].tiefe)
	m, _ := wurzel.(map[string]any)
	return m
}

// yamlPaar: „schluessel: wert“ (Schlüssel auch in Anführungszeichen).
func yamlPaar(z string) (string, string, bool) {
	if strings.HasPrefix(z, `"`) || strings.HasPrefix(z, "'") {
		q := z[0]
		if ende := strings.IndexByte(z[1:], q); ende >= 0 && strings.HasPrefix(z[ende+2:], ":") {
			return z[1 : ende+1], strings.TrimSpace(z[ende+3:]), true
		}
	}
	i := strings.Index(z, ": ")
	if i < 0 {
		if strings.HasSuffix(z, ":") {
			return strings.TrimSuffix(z, ":"), "", true
		}
		return "", "", false
	}
	return z[:i], strings.TrimSpace(z[i+2:]), true
}

func yamlSkalar(v string) any {
	v = strings.TrimSpace(v)
	switch {
	case strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]"):
		var r []any
		for _, t := range tomlTeile(v[1 : len(v)-1]) {
			r = append(r, yamlWert(t))
		}
		return r
	case strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}"):
		m := map[string]any{}
		for _, t := range tomlTeile(v[1 : len(v)-1]) {
			if k, w, ok := yamlPaar(t); ok {
				m[yamlWert(k)] = yamlWert(w)
			}
		}
		return m
	}
	return yamlWert(v)
}

func yamlOhneKommentar(z string) string {
	in := byte(0)
	for i := 0; i < len(z); i++ {
		c := z[i]
		switch {
		case in != 0 && c == in:
			in = 0
		case in == 0 && (c == '"' || c == '\''):
			in = c
		case in == 0 && c == '#' && (i == 0 || z[i-1] == ' ' || z[i-1] == '\t'):
			return z[:i]
		}
	}
	return z
}
