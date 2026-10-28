package main

// Projekt-Umgebung als Container: Image bauen (Projekt-Basis + Agent-Schicht), Abhängigkeiten
// einrichten (mit Netz, ohne Agent und ohne Schlüssel), Befehle ohne Netz ausführen.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

const agentImage = "swarvex-agent"

func podmanAus(ctx context.Context, eingabe string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "podman", args...)
	if eingabe != "" {
		cmd.Stdin = strings.NewReader(eingabe)
	}
	aus, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(aus)), err
}

// umgebungsImage baut (einmal je Basis und Agent-Version) das Image, in dem Agent und Befehle laufen.
// mitAgent=false lässt die Agent-Schicht weg (z. B. für `swarvex env`).
func umgebungsImage(ctx context.Context, u umgebung.Umgebung, baum string, mitAgent bool) (string, error) {
	agent := ""
	agentID := "none"
	if mitAgent {
		id, err := podmanAus(ctx, "", "image", "inspect", "--format", "{{.Id}}", agentImage)
		if err != nil {
			return "", fmt.Errorf("agent image %s is missing; reinstall the runner (install.sh)", agentImage)
		}
		agent, agentID = agentImage, id
	}
	h := sha256.Sum256([]byte(u.Kennung(baum) + "\n" + agentID))
	name := "swarvex-env-" + hex.EncodeToString(h[:])[:16]
	if _, err := podmanAus(ctx, "", "image", "exists", name); err == nil {
		return name, nil
	}
	basis := ""
	if u.Dockerfile != "" {
		// Eigenes Dockerfile des Projekts (Dev Container): einmal bauen, ohne Agent und ohne Schlüssel.
		basis = "swarvex-base-" + u.Kennung(baum)
		if _, err := podmanAus(ctx, "", "image", "exists", basis); err != nil {
			if aus, err := podmanAus(ctx, "", "build", "-q", "-t", basis, "-f", filepath.Join(baum, u.Dockerfile), filepath.Join(baum, u.Kontext)); err != nil {
				return "", fmt.Errorf("building the project's Dockerfile: %s", letzteZeilen(aus, 20))
			}
		}
	}
	leer, err := os.MkdirTemp("", "swarvex-build-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(leer)
	if aus, err := podmanAus(ctx, u.Containerfile(basis, agent), "build", "-q", "-t", name, "-f", "-", leer); err != nil {
		return "", fmt.Errorf("building the environment: %s", letzteZeilen(aus, 20))
	}
	return name, nil
}

// umgebungErmitteln: Umgebung des Projekts; verlangt es eine proprietäre Engine, wird die passende
// Installation dieses Rechners eingebunden (sonst ein Fehler mit Anleitung).
func umgebungErmitteln(wurzel string) (umgebung.Umgebung, error) {
	u, err := umgebung.Ermitteln(wurzel)
	if err != nil || u.Engine == nil {
		return u, err
	}
	i, err := engineFuer(u, enginesFinden())
	if err != nil {
		return u, err
	}
	u.Einbinden = engineArgs(i)
	return u, nil
}

// cacheOrdner: Paket-Caches je Projekt; bleiben zwischen Aufgaben erhalten.
func cacheOrdner(projekt string) string {
	ordner := filepath.Join(datenOrdner(), "cache", projekt)
	os.MkdirAll(ordner, 0o777)
	return ordner
}

// containerArgs: gemeinsame, abgeschottete Einstellungen für Einrichten, Befehle und Agent.
func containerArgs(name, baum, cache string, u umgebung.Umgebung, sekunden int) []string {
	args := []string{"run", "--rm", "--name", name, "--timeout", fmt.Sprint(sekunden),
		"--cap-drop=ALL", "--security-opt", "no-new-privileges", "--read-only",
		"--tmpfs", "/tmp:rw,exec,size=2g,mode=1777", "--tmpfs", "/home/agent:rw,exec,size=1g,mode=1777",
		"--userns=keep-id:uid=1000,gid=1000", "--user", "1000:1000", "--memory", "4g", "--cpus", "3", "--pids-limit", "1024",
		"-v", baum + ":/work:Z", "-v", cache + ":/cache:Z", "-w", filepath.Join("/work", u.Ordner)}
	args = append(args, u.Einbinden...)
	for _, v := range umgebung.CacheVariablen {
		args = append(args, "-e", v)
	}
	return args
}

// einrichten lädt die Abhängigkeiten des Projekts: mit Netz, aber ohne Agent, ohne Proxy und ohne Schlüssel.
func einrichten(ctx context.Context, u umgebung.Umgebung, image, baum, cache, protokoll string) error {
	if strings.TrimSpace(u.Einrichten) == "" {
		return nil
	}
	ctx, stopp := context.WithTimeout(ctx, 20*time.Minute)
	defer stopp()
	name := "swarvex-setup-" + zufall(6)
	args := append(containerArgs(name, baum, cache, u, 1200), image, "sh", "-ec", u.Einrichten)
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Cancel = func() error { exec.Command("podman", "kill", name).Run(); return cmd.Process.Kill() }
	cmd.WaitDelay = 10 * time.Second
	aus, err := cmd.CombinedOutput()
	os.WriteFile(protokoll, aus, 0o644)
	if err != nil {
		return fmt.Errorf("setup failed (%v): %s", err, letzteZeilen(string(aus), 15))
	}
	return nil
}

// ohneNetz führt einen Befehl der Umgebung ohne jedes Netz aus (Bauen, Testen).
func ohneNetz(ctx context.Context, u umgebung.Umgebung, image, baum, cache, befehl string, sekunden int) (string, error) {
	name := "swarvex-cmd-" + zufall(6)
	args := append(containerArgs(name, baum, cache, u, sekunden), "--network", "none")
	args = append(args, image, "sh", "-ec", befehl)
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Cancel = func() error { exec.Command("podman", "kill", name).Run(); return cmd.Process.Kill() }
	cmd.WaitDelay = 10 * time.Second
	aus, err := cmd.CombinedOutput()
	return string(aus), err
}

func letzteZeilen(text string, n int) string {
	z := strings.Split(strings.TrimSpace(text), "\n")
	if len(z) > n {
		z = z[len(z)-n:]
	}
	return strings.Join(z, "\n")
}
