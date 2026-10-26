// swarvex: verbindet die Hardware eines Mitstreiters mit der Plattform.
//
//	swarvex login   --plattform https://…        Anmeldung per Device Flow
//	swarvex arbeiten --projekt <slug> …          eine Aufgabe in der Sandbox bearbeiten
//	swarvex dienst <wie arbeiten>                 als Dienst im Dauerbetrieb einrichten (start, stop, status)
//	swarvex uninstall                             alles entfernen, Gerät abmelden
//	swarvex proxy                                 Schlüssel-Proxy (läuft im Proxy-Container)
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "commands: login | work | service | start | stop | status | uninstall | env | attack-test")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "proxy":
		proxyStarten()
	case "login":
		loginBefehl(os.Args[2:])
	case "work":
		arbeitenBefehl(os.Args[2:])
	case "service":
		dienstBefehl(os.Args[2:])
	case "start":
		startBefehl()
	case "stop":
		stopBefehl()
	case "status":
		statusBefehl()
	case "uninstall":
		uninstallBefehl(os.Args[2:])
	case "env":
		umgebungBefehl(os.Args[2:])
	case "attack-test":
		angriffstestBefehl(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", os.Args[1])
		os.Exit(2)
	}
}
