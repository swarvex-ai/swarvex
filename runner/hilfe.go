package main

// swarvex help (auch -h, --help, -?, ?): alle Befehle mit Kurzbeschreibung; `swarvex help <befehl>` zeigt dessen Optionen.

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type befehlInfo struct{ name, aufruf, text string }

var befehlsGruppen = []struct {
	titel  string
	befehl []befehlInfo
}{
	{"Get started", []befehlInfo{
		{"setup", "setup", "set up with a few questions: model, project, when it works, limits"},
		{"doctor", "doctor", "check everything the runner needs and say how to fix problems"},
		{"connect", "connect", "add swarvex as an MCP server to agents found here (tasks without code)"},
		{"login", "login --platform <url>", "sign in again (opens the browser)"},
	}},
	{"Run", []befehlInfo{
		{"work", "work", "work on one task now (with the saved settings)"},
		{"service", "service", "run as a background service within your time windows"},
		{"start", "start / stop", "start or stop the service (stop hands a running task back)"},
		{"status", "status", "state, version, today's usage, latest messages"},
		{"logs", "logs", "follow the service's output"},
		{"history", "history", "last tasks: submitted or handed back, reason, tokens"},
		{"budget", "budget [--task N] [--daily N]", "show or change the token limits (0 = no limit)"},
	}},
	{"Maintain", []befehlInfo{
		{"update", "update", "install the newest version; settings stay (--check only looks)"},
		{"version", "version", "show the installed version"},
		{"env", "env", "visionary: check the project environment in its repository"},
		{"uninstall", "uninstall", "remove everything and sign this device out"},
	}},
}

var mitOptionen = map[string]bool{"setup": true, "connect": true, "login": true, "work": true, "service": true, "update": true,
	"history": true, "budget": true, "env": true, "uninstall": true}

func istHilfe(a string) bool {
	switch a {
	case "help", "-h", "--help", "-help", "-?", "?", "/?":
		return true
	}
	return false
}

func hilfeBefehl(args []string) {
	if len(args) > 0 && !istHilfe(args[0]) {
		name := args[0]
		for _, g := range befehlsGruppen {
			for _, b := range g.befehl {
				if b.name == name || (name == "stop" && b.name == "start") {
					fmt.Printf("swarvex %s\n  %s\n", b.aufruf, b.text)
					if mitOptionen[name] {
						fmt.Println("\nOptions:")
						// Die Optionen kennt der Befehl selbst (flag): mit -h aufrufen; "service" hat die von "work".
						ziel := name
						if name == "service" {
							ziel = "work"
						}
						c := exec.Command(os.Args[0], ziel, "-h")
						c.Stdout, c.Stderr = os.Stdout, os.Stdout
						c.Run()
					}
					return
				}
			}
		}
		fmt.Printf("Unknown command %q.\n\n", name)
	}
	fmt.Printf("swarvex runner %s: lets your agent work on open-source tasks in a sandbox.\n\nUsage: swarvex <command> [options]\n", version)
	for _, g := range befehlsGruppen {
		fmt.Printf("\n%s:\n", g.titel)
		for _, b := range g.befehl {
			fmt.Printf("  %-32s %s\n", b.aufruf, b.text)
		}
	}
	fmt.Println("\nOptions of a command: swarvex help <command>   ·   Docs: " + strings.TrimRight(standardPlattform, "/") + "/en/docs/runner")
}
