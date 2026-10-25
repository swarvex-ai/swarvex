package main

// Angriffstests (docs/angriffstests.md, S1–S10): startet Proxy und Sandbox mit exakt denselben
// Einstellungen wie `arbeiten`, führt aber statt des Agenten ein Angriffsskript aus. So wird
// genau die Sandbox angegriffen, die echte Aufgaben bekommen.

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func angriffstestBefehl(args []string) {
	fs := flag.NewFlagSet("angriffstest", flag.ExitOnError)
	skript := fs.String("skript", "", "Angriffsskript (wird in die Sandbox gelegt und dort ausgeführt)")
	modell := fs.String("modell", "Qwen3.8 Flash Next", "Modellname")
	upstream := fs.String("modell-url", "http://127.0.0.1:8080", "Modell-URL (OpenAI-kompatibel)")
	keyDatei := fs.String("modell-key-datei", "", "Schlüsseldatei (für S1 ein Köder-Schlüssel)")
	budget := fs.Int("budget-tokens", 400000, "Token-Budget")
	zeit := fs.Duration("zeitlimit", 10*time.Minute, "Zeitlimit")
	fs.Parse(args)
	if *skript == "" {
		fehlerAus("--skript fehlt")
	}
	ordner := filepath.Join(datenOrdner(), "angriff", time.Now().Format("20060102-150405"))
	baum := filepath.Join(ordner, "baum")
	pruefen(os.MkdirAll(baum, 0o755))
	inhalt, err := os.ReadFile(*skript)
	pruefen(err)
	pruefen(os.WriteFile(filepath.Join(baum, "angriff.sh"), inhalt, 0o755))
	key := ""
	if *keyDatei != "" {
		roh, err := os.ReadFile(*keyDatei)
		pruefen(err)
		key = strings.TrimSpace(string(roh))
	}
	ctx, stopp := context.WithTimeout(context.Background(), *zeit)
	defer stopp()
	a := aufgabe{ID: time.Now().Unix() % 100000, Titel: "Angriffstest", Engine: "test"}
	verbrauch, aus, err := sandboxAusfuehren(ctx, a, baum, ordner, *upstream, key, *modell, *budget, []string{"sh", "/work/angriff.sh"})
	fmt.Println(aus)
	fmt.Printf("--- Verbrauch laut Proxy: %v\n", verbrauch)
	proxyLog, _ := os.ReadFile(filepath.Join(ordner, "proxy.log"))
	fmt.Printf("--- Proxy-Protokoll:\n%s\n", proxyLog)
	if err != nil {
		fmt.Println("--- Sandbox-Ende:", err)
	}
}
