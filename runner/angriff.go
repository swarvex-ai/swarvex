package main

// Angriffstests (docs/attack-tests.md, S1–S10): startet Proxy und Sandbox mit exakt denselben
// Einstellungen wie `arbeiten`, führt aber statt des Agenten ein Angriffsskript aus. So wird
// genau die Sandbox angegriffen, die echte Aufgaben bekommen.

import (
	"context"
	"flag"
	"fmt"
	"github.com/swarvex-ai/swarvex/runner/umgebung"
	"os"
	"path/filepath"
	"time"
)

func angriffstestBefehl(args []string) {
	fs := flag.NewFlagSet("attack-test", flag.ExitOnError)
	skript := fs.String("script", "", "attack script (placed into the sandbox and run there)")
	agentName := fs.String("agent", "opencode", "agent whose sandbox setup is attacked: "+agentNamen())
	modell := fs.String("model", "Qwen3.8 Flash Next", "model name")
	upstream := fs.String("model-url", "http://127.0.0.1:8080", "model URL (OpenAI-compatible)")
	keyDatei := fs.String("model-key-file", "", "key file (for S1 a decoy key)")
	budget := fs.Int("budget-tokens", 400000, "token budget")
	zeit := fs.Duration("time-limit", 10*time.Minute, "time limit")
	fs.Parse(args)
	if *skript == "" {
		fehlerAus("--script is missing")
	}
	agent := agentPruefen(*agentName)
	ordner := filepath.Join(datenOrdner(), "attack", time.Now().Format("20060102-150405"))
	baum := filepath.Join(ordner, "tree")
	pruefen(os.MkdirAll(baum, 0o755))
	inhalt, err := os.ReadFile(*skript)
	pruefen(err)
	pruefen(os.WriteFile(filepath.Join(baum, "attack.sh"), inhalt, 0o755))
	zugang, err := zugangLesen(agent, *keyDatei, *upstream, *zeit)
	pruefen(err)
	ctx, stopp := context.WithTimeout(context.Background(), *zeit)
	defer stopp()
	a := aufgabe{ID: time.Now().Unix() % 100000, Titel: "attack test", Engine: "test"}
	// Angriffe laufen in derselben Umgebung wie echte Aufgaben (hier: allgemeine Linux-Umgebung).
	u, err := umgebung.Ermitteln(baum)
	pruefen(err)
	pruefen(agentImageBereit(ctx, agent))
	image, err := umgebungsImage(ctx, u, baum, agent.Image)
	pruefen(err)
	verbrauch, aus, err := sandboxAusfuehren(ctx, a, u, image, baum, cacheOrdner("attack-test"), ordner, agent, skillsLesen(os.Getenv("SWARVEX_ATTACK_SKILLS")), zugang, *modell, *budget, []string{"sh", "/work/attack.sh"})
	fmt.Println(aus)
	fmt.Printf("--- usage according to the proxy: %v\n", verbrauch)
	proxyLog, _ := os.ReadFile(filepath.Join(ordner, "proxy.log"))
	fmt.Printf("--- proxy log:\n%s\n", proxyLog)
	if err != nil {
		fmt.Println("--- sandbox ended:", err)
	}
}
