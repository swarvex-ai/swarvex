package main

// Projekt-Umgebung als Container: Image bauen (Projekt-Basis + Agent-Schicht), Abhängigkeiten
// einrichten (mit Netz, ohne Agent und ohne Schlüssel), Befehle ohne Netz ausführen.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

func podmanAus(ctx context.Context, eingabe string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "podman", args...)
	if eingabe != "" {
		cmd.Stdin = strings.NewReader(eingabe)
	}
	aus, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(aus)), err
}

// umgebungsImage baut (einmal je Basis und Agent-Version) das Image, in dem Agent und Befehle laufen.
// agentImage "" lässt die Agent-Schicht weg (z. B. für `swarvex env` und Gegenprüfungen).
func umgebungsImage(ctx context.Context, u umgebung.Umgebung, baum string, agentImage string) (string, error) {
	agent := ""
	agentID := "none"
	if agentImage != "" {
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

// Netz beim Einrichten (--setup-network oder SWARVEX_SETUP_NETWORK): public (Standard) über den Ausgangs-Proxy, nur
// öffentliche Adressen; lan über den Proxy, auch private Adressen; open wie früher, das volle Netz des Rechners.
// --setup-allow (SWARVEX_SETUP_ALLOW): Ausnahmen trotz public, etwa ein Paketspiegel im Heimnetz (IP, CIDR, Hostname).
var setupNetz, setupErlaubt string

func einrichtNetz() (modus, erlaubt string) {
	modus, erlaubt = setupNetz, setupErlaubt
	if modus == "" {
		modus = envOder("SWARVEX_SETUP_NETWORK", "public")
	}
	if erlaubt == "" {
		erlaubt = os.Getenv("SWARVEX_SETUP_ALLOW")
	}
	return
}

// sandboxNetz: das Podman-Netz ohne Ausgang, in dem Sandbox und Einrichten hängen. Ohne eigenen DNS: Hängt ein
// Container an einem internen Netz mit DNS und zugleich am Standardnetz, trägt Podman (ab 5) nur den DNS des internen
// Netzes ein, und der löst keine externen Namen auf; die Proxys erreichten dann keinen Anbieter. Sandbox und
// Einrichten finden ihren Proxy deshalb über einen /etc/hosts-Eintrag (proxyHost). Bis 0.9.2 hieß es swarvex-intern.
const sandboxNetz = "swarvex-sandbox"

func internesNetz(ctx context.Context) (string, error) {
	if exec.CommandContext(ctx, "podman", "network", "exists", sandboxNetz).Run() != nil {
		if aus, err := exec.CommandContext(ctx, "podman", "network", "create", "--internal", "--disable-dns", sandboxNetz).CombinedOutput(); err != nil {
			return "", fmt.Errorf("creating the network: %s", aus)
		}
	}
	return sandboxNetz, nil
}

// proxyHost: "--add-host name:ip" für einen Proxy-Container im Sandbox-Netz, damit sein Name ohne DNS auflöst.
func proxyHost(ctx context.Context, name string) ([]string, error) {
	aus, err := exec.CommandContext(ctx, "podman", "inspect", "-f", `{{(index .NetworkSettings.Networks "`+sandboxNetz+`").IPAddress}}`, name).Output()
	ip := strings.TrimSpace(string(aus))
	if err != nil || net.ParseIP(ip) == nil {
		return nil, fmt.Errorf("address of %s in %s not found (%v)", name, sandboxNetz, err)
	}
	return []string{"--add-host", name + ":" + ip}, nil
}

// einrichten lädt die Abhängigkeiten des Projekts: ohne Agent und ohne Schlüssel; ins Internet nur über den
// Ausgangs-Proxy (ausgang.go), der das Heimnetz und den eigenen Rechner sperrt.
func einrichten(ctx context.Context, u umgebung.Umgebung, image, baum, cache, protokoll string) error {
	if strings.TrimSpace(u.Einrichten) == "" {
		return nil
	}
	ctx, stopp := context.WithTimeout(ctx, 20*time.Minute)
	defer stopp()
	name := "swarvex-setup-" + zufall(6)
	args := containerArgs(name, baum, cache, u, 1200)
	modus, erlaubt := einrichtNetz()
	var ausgangName string
	switch modus {
	case "open":
	case "public", "lan":
		netz, err := internesNetz(ctx)
		if err != nil {
			return err
		}
		ausgangName = "swarvex-egress-" + zufall(6)
		start := []string{"run", "-d", "--rm", "--name", ausgangName, "--network", netz, "--network", "podman", "--timeout", "1500",
			"--read-only", "--cap-drop=ALL", "--security-opt", "no-new-privileges", "--memory", "256m", "--pids-limit", "128",
			"-e", "SWARVEX_EGRESS_MODE=" + modus, "-e", "SWARVEX_EGRESS_ALLOW=" + erlaubt, "swarvex-proxy", "egress"}
		if aus, err := exec.CommandContext(ctx, "podman", start...).CombinedOutput(); err != nil {
			return fmt.Errorf("starting the egress proxy for the setup: %s", strings.TrimSpace(string(aus)))
		}
		defer exec.Command("podman", "rm", "-f", ausgangName).Run()
		proxy := "http://" + ausgangName + ":3128"
		hosts, err := proxyHost(ctx, ausgangName)
		if err != nil {
			return err
		}
		args = append(append(args, "--network", netz), hosts...)
		for _, v := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"} {
			args = append(args, "-e", v+"="+proxy)
		}
		args = append(args, "-e", "NO_PROXY=localhost,127.0.0.1", "-e", "no_proxy=localhost,127.0.0.1",
			// Java-Werkzeuge (Gradle, Maven) lesen keine Proxy-Variablen, nur Systemeigenschaften
			"-e", fmt.Sprintf("JAVA_TOOL_OPTIONS=-Dhttp.proxyHost=%s -Dhttp.proxyPort=3128 -Dhttps.proxyHost=%s -Dhttps.proxyPort=3128", ausgangName, ausgangName))
	default:
		return fmt.Errorf("--setup-network: public, lan or open, not %q", modus)
	}
	args = append(args, image, "sh", "-ec", u.Einrichten)
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Cancel = func() error { exec.Command("podman", "kill", name).Run(); return cmd.Process.Kill() }
	cmd.WaitDelay = 10 * time.Second
	aus, err := cmd.CombinedOutput()
	if ausgangName != "" {
		// Gesperrte Ziele ins Protokoll, damit sichtbar ist, warum ein Download scheiterte
		if log, _ := exec.Command("podman", "logs", ausgangName).CombinedOutput(); strings.Contains(string(log), "egress_blocked") {
			for _, z := range strings.Split(string(log), "\n") {
				if strings.Contains(z, "egress_blocked") {
					aus = append(aus, []byte("\n[swarvex] "+z)...)
				}
			}
			aus = append(aus, []byte("\n[swarvex] The setup may only reach public addresses. For a mirror in your network: swarvex work --setup-allow <ip|host>, or --setup-network lan\n")...)
		}
	}
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
