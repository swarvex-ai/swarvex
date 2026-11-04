// swarvex: verbindet die Hardware eines Mitstreiters mit der Plattform.
//
//	swarvex setup                                 Einrichtung mit Fragen (Modell, Projekt, Betrieb); danach reicht `swarvex work`
//	swarvex connect                               swarvex bei allen gefundenen Agenten als MCP-Server eintragen
//	swarvex login --platform https://…           Anmeldung per Device Flow
//	swarvex work --project <slug> …              eine Aufgabe in der Sandbox bearbeiten
//	swarvex service <wie work>                    als Dienst im Dauerbetrieb einrichten (start, stop, status)
//	swarvex update                                neueste Version von der Plattform holen (Einstellungen bleiben)
//	swarvex version                               installierte Version
//	swarvex logs                                  Ausgabe des Dienstes laufend anzeigen
//	swarvex history                               letzte Aufgaben mit Ergebnis, Grund und Verbrauch
//	swarvex doctor                                alles prüfen, was der Runner braucht, mit Rat bei Problemen
//	swarvex budget [--task N] [--daily N]         Token-Grenzen anzeigen oder ändern
//	swarvex help [befehl]                         alle Befehle bzw. die Optionen eines Befehls (auch -h, --help, -?, ?)
//	swarvex uninstall                             alles entfernen, Gerät abmelden
//	swarvex proxy                                 Schlüssel-Proxy (läuft im Proxy-Container)
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 || istHilfe(os.Args[1]) {
		hilfeBefehl(os.Args[min(2, len(os.Args)):])
		if len(os.Args) < 2 {
			os.Exit(2)
		}
		return
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
	case "update":
		updateBefehl(os.Args[2:])
	case "version", "--version", "-v":
		versionBefehl()
	case "logs", "log":
		logsBefehl()
	case "history":
		historyBefehl(os.Args[2:])
	case "doctor":
		doctorBefehl()
	case "budget":
		budgetBefehl(os.Args[2:])
	case "uninstall":
		uninstallBefehl(os.Args[2:])
	case "env":
		umgebungBefehl(os.Args[2:])
	case "attack-test":
		angriffstestBefehl(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s (all commands: swarvex help)\n", os.Args[1])
		os.Exit(2)
	}
}
