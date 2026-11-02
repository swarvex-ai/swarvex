// swarvex: verbindet die Hardware eines Mitstreiters mit der Plattform.
//
//	swarvex setup                                 Einrichtung mit Fragen (Modell, Projekt, Betrieb); danach reicht `swarvex work`
//	swarvex connect                               swarvex bei allen gefundenen Agenten als MCP-Server eintragen
//	swarvex login --platform https://…           Anmeldung per Device Flow
//	swarvex work --project <slug> …              eine Aufgabe in der Sandbox bearbeiten
//	swarvex service <wie work>                    als Dienst im Dauerbetrieb einrichten (start, stop, status)
//	swarvex uninstall                             alles entfernen, Gerät abmelden
//	swarvex proxy                                 Schlüssel-Proxy (läuft im Proxy-Container)
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "commands: setup | connect | login | work | service | start | stop | status | uninstall | env | attack-test")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "proxy":
		proxyStarten()
	case "setup":
		setupBefehl(os.Args[2:])
	case "connect":
		connectBefehl(os.Args[2:])
	case "probe":
		probeBefehl(os.Args[2:])
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
