package main

// Agenten in der Sandbox (E-32): Mitstreiter wählen ihren Agenten (opencode, Claude Code, Codex)
// und ihr Modell. Die Abschottung bleibt für alle gleich: kein Netz außer dem Schlüssel-Proxy,
// der echte Schlüssel bzw. das Abo-Token bleibt im Proxy, Updates und Telemetrie sind aus.
//
// Skills: Die Skills des Projekts (.agents/skills, .claude/skills im Repo) sehen alle Agenten.
// Eigene Skills des Mitstreiters kommen nur mit --skills dazu, als Kopie und nur lesend.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type agentArt struct {
	Name   string
	API    string // Schnittstelle zum Modell, die der Proxy durchlässt (siehe proxy.go)
	Image  string // Agent-Schicht (sandbox/Containerfile, Stufe = Name)
	Skills string // Ordner der eigenen Skills unter /home/agent
}

var agenten = map[string]agentArt{
	"opencode": {Name: "opencode", API: "chat", Image: "swarvex-agent", Skills: ".agents/skills"},
	"claude":   {Name: "claude", API: "anthropic", Image: "swarvex-agent-claude", Skills: ".claude/skills"},
	"codex":    {Name: "codex", API: "responses", Image: "swarvex-agent-codex", Skills: ".agents/skills"},
}

func agentNamen() string {
	var n []string
	for name := range agenten {
		n = append(n, name)
	}
	sort.Strings(n)
	return strings.Join(n, ", ")
}

// agentImageBereit baut die Agent-Schicht beim ersten Gebrauch aus dem Containerfile der installierten Version.
func agentImageBereit(ctx context.Context, a agentArt) error {
	if _, err := podmanAus(ctx, "", "image", "exists", a.Image); err == nil {
		return nil
	}
	kontext := filepath.Join(datenOrdner(), "release", "sandbox")
	if _, err := os.Stat(filepath.Join(kontext, "Containerfile")); err != nil {
		return fmt.Errorf("agent image %s is missing and %s/Containerfile was not found; reinstall the runner (install.sh)", a.Image, kontext)
	}
	fmt.Printf("Preparing the %s agent (only the first time, takes a few minutes)\n", a.Name)
	if aus, err := podmanAus(ctx, "", "build", "-q", "--target", a.Name, "-t", a.Image, kontext); err != nil {
		return fmt.Errorf("building the %s agent: %s", a.Name, letzteZeilen(aus, 20))
	}
	return nil
}

// agentStart: zusätzliche Podman-Argumente (Umgebung, Mounts) und der Befehl des Agenten.
// ordner ist der Aufgabenordner außerhalb der Sandbox; dort entstehen Konfiguration und Skill-Kopie.
// projektOrdner ist der Unterordner, in dem der Agent startet ("" = Wurzel).
// zugang zeigt nur an, ob Codex mit ChatGPT-Abo läuft (Schlüssel bleiben im Proxy).
func agentStart(a agentArt, ordner, proxyURL, token, modell, auftrag string, skills []string, baum, projektOrdner string, zugang modellZugang) ([]string, []string, error) {
	args := []string{"-e", "HOME=/home/agent", "-e", "SWARVEX_TOKEN=" + token, "-e", "SWARVEX_PROXY=" + proxyURL}
	// Eigene Skills, dazu Projekt-Skills, die der Agent nicht selbst findet: Claude Code liest
	// .agents/skills nicht, und ohne .git (die Sandbox hat keins) suchen opencode und Codex nur ab dem
	// Startordner. Projekt-Skills kommen zuletzt und gewinnen bei gleichem Namen.
	quellen := append([]string{}, skills...)
	if a.Name == "claude" || projektOrdner != "" {
		quellen = append(quellen, filepath.Join(baum, ".agents", "skills"))
	}
	if a.Name == "claude" && projektOrdner != "" {
		quellen = append(quellen, filepath.Join(baum, ".claude", "skills"), filepath.Join(baum, projektOrdner, ".agents", "skills"))
	}
	kopie := filepath.Join(ordner, "skills")
	n, err := skillsKopieren(kopie, quellen)
	if err != nil {
		return nil, nil, err
	}
	if n > 0 {
		// Der Elternordner (.claude, .agents) bleibt beschreibbar: eigenes tmpfs, sonst legt Podman ihn root-eigen an.
		eltern := strings.SplitN(a.Skills, "/", 2)[0]
		args = append(args, "--tmpfs", "/home/agent/"+eltern+":rw,exec,size=256m,mode=1777", "-v", kopie+":/home/agent/"+a.Skills+":ro,Z")
		fmt.Printf("Skills for the agent: %d\n", n)
	}

	switch a.Name {
	case "opencode":
		konfig := map[string]any{
			"$schema": "https://opencode.ai/config.json", "autoupdate": false, "share": "disabled",
			"model": "swarvex/" + modell,
			"provider": map[string]any{"swarvex": map[string]any{
				"npm": "@ai-sdk/openai-compatible", "name": "swarvex proxy",
				"options": map[string]any{"baseURL": proxyURL + "/v1", "apiKey": "{env:SWARVEX_TOKEN}"},
				"models":  map[string]any{modell: map[string]any{"name": modell, "tool_call": true}},
			}},
		}
		pfad, err := konfigSchreiben(ordner, "opencode.json", konfig)
		if err != nil {
			return nil, nil, err
		}
		args = append(args, "-v", pfad+":/konfig/opencode.json:ro,Z", "-e", "OPENCODE_CONFIG=/konfig/opencode.json",
			"-e", "OPENCODE_DISABLE_AUTOUPDATE=1", "-e", "OPENCODE_DISABLE_MODELS_FETCH=1")
		return args, []string{"opencode", "run", "--pure", "--auto", "-m", "swarvex/" + modell, auftrag}, nil

	case "claude":
		// Alle Modellrollen (auch Hilfsaufgaben und Subagenten) auf das eine freigegebene Modell.
		for _, v := range []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL", "ANTHROPIC_SMALL_FAST_MODEL", "CLAUDE_CODE_SUBAGENT_MODEL"} {
			args = append(args, "-e", v+"="+modell)
		}
		args = append(args, "-e", "ANTHROPIC_BASE_URL="+proxyURL, "-e", "ANTHROPIC_AUTH_TOKEN="+token,
			"-e", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "-e", "DISABLE_AUTOUPDATER=1", "-e", "DISABLE_TELEMETRY=1",
			"-e", "DISABLE_ERROR_REPORTING=1", "-e", "IS_SANDBOX=1")
		return args, []string{"claude", "-p", "--dangerously-skip-permissions", "--output-format", "text", "--model", modell, auftrag}, nil

	case "codex":
		// Einstellungen per -c statt config.toml: ~/.codex bleibt Codex' eigener, beschreibbarer Ordner.
		// Codex' eigene Sandbox ist überflüssig (und im Container nicht verfügbar): Wir sind die Sandbox.
		anbieter := fmt.Sprintf("{name=%s, base_url=%s, env_key=\"SWARVEX_TOKEN\", wire_api=\"responses\"}", tomlText("swarvex proxy"), tomlText(proxyURL+"/v1"))
		if zugang.chatgpt() {
			// ChatGPT-Abo: Codex meldet sich wie gewohnt mit auth.json an, die hier eine Attrappe ist
			// (Zugangstoken = Wegwerf-Token). Den echten Zugang setzt der Proxy ein.
			pfad := filepath.Join(ordner, "codex-auth.json")
			if err := os.WriteFile(pfad, codexSandboxAnmeldung(token, zugang.Plan), 0o644); err != nil {
				return nil, nil, err
			}
			args = append(args, "--tmpfs", "/home/agent/.codex:rw,exec,size=256m,mode=1777", "-v", pfad+":/home/agent/.codex/auth.json:ro,Z")
			anbieter = fmt.Sprintf("{name=%s, base_url=%s, requires_openai_auth=true, wire_api=\"responses\"}", tomlText("swarvex proxy"), tomlText(proxyURL+"/v1"))
		}
		return args, []string{"codex", "exec", "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", "--color", "never",
			"-c", "model_providers.swarvex=" + anbieter, "-c", `model_provider="swarvex"`, "-c", "check_for_update_on_startup=false",
			"-c", "features.apps=false", // ChatGPT-Apps (…/backend-api/ps/mcp) sind in der Sandbox ohnehin nicht erreichbar
			"--model", modell, auftrag}, nil
	}
	return nil, nil, fmt.Errorf("unknown agent %q (available: %s)", a.Name, agentNamen())
}

func konfigSchreiben(ordner, name string, inhalt any) (string, error) {
	pfad := filepath.Join(ordner, name)
	roh, _ := json.MarshalIndent(inhalt, "", "  ")
	return pfad, os.WriteFile(pfad, roh, 0o644)
}

// tomlText: JSON-Zeichenketten sind gültige TOML-Basic-Strings.
func tomlText(s string) string {
	roh, _ := json.Marshal(s)
	return string(roh)
}

// skillOrdnerStandard: wo Agenten eigene Skills ablegen (für --skills auto).
func skillOrdnerStandard() []string {
	home, _ := os.UserHomeDir()
	return []string{filepath.Join(home, ".agents", "skills"), filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".codex", "skills"), filepath.Join(home, ".config", "opencode", "skills")}
}

// skillsLesen: der Wert von --skills (leer, "auto" oder Ordner mit ":" getrennt).
func skillsLesen(wert string) []string {
	if strings.TrimSpace(wert) == "" {
		return nil
	}
	if wert == "auto" {
		return skillOrdnerStandard()
	}
	return strings.Split(wert, ":")
}

const skillGrenze = 50 << 20 // 50 MB für alle Skills zusammen

// skillsKopieren legt alle Skills (Ordner mit SKILL.md) aus den Quellen als Kopie in ziel ab.
// Spätere Quellen gewinnen bei gleichem Namen. Nur normale Dateien, keine Links nach außen.
func skillsKopieren(ziel string, quellen []string) (int, error) {
	os.RemoveAll(ziel)
	namen := map[string]bool{}
	var groesse int64
	for _, q := range quellen {
		eintraege, err := os.ReadDir(q)
		if err != nil {
			continue // Ordner gibt es nicht: kein Fehler (z. B. bei auto)
		}
		for _, e := range eintraege {
			quelle, err := filepath.EvalSymlinks(filepath.Join(q, e.Name()))
			if err != nil {
				continue
			}
			if info, err := os.Stat(filepath.Join(quelle, "SKILL.md")); err != nil || !info.Mode().IsRegular() {
				continue
			}
			os.RemoveAll(filepath.Join(ziel, e.Name()))
			err = filepath.WalkDir(quelle, func(pfad string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(quelle, pfad)
				if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
					return filepath.SkipDir
				}
				nach := filepath.Join(ziel, e.Name(), rel)
				if d.IsDir() {
					return os.MkdirAll(nach, 0o755)
				}
				if !d.Type().IsRegular() {
					return nil
				}
				info, err := d.Info()
				if err != nil {
					return err
				}
				if groesse += info.Size(); groesse > skillGrenze {
					return fmt.Errorf("skills are larger than %d MB together", skillGrenze>>20)
				}
				return dateiKopieren(pfad, nach, info.Mode().Perm())
			})
			if err != nil {
				return 0, fmt.Errorf("copying skill %s: %w", e.Name(), err)
			}
			namen[e.Name()] = true
		}
	}
	return len(namen), nil
}

func dateiKopieren(von, nach string, modus os.FileMode) error {
	q, err := os.Open(von)
	if err != nil {
		return err
	}
	defer q.Close()
	z, err := os.OpenFile(nach, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, modus|0o444)
	if err != nil {
		return err
	}
	if _, err := io.Copy(z, q); err != nil {
		z.Close()
		return err
	}
	return z.Close()
}

// agentPruefen: frühe, klare Fehlermeldung für --agent.
func agentPruefen(name string) agentArt {
	a, ok := agenten[name]
	if !ok {
		fehlerAus(fmt.Sprintf("--agent %q is unknown (available: %s)", name, agentNamen()))
	}
	return a
}
