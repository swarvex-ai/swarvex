// swarvex: verbindet die Hardware eines Mitstreiters mit der Plattform.
//
//	swarvex login   --plattform https://…        Anmeldung per Device Flow
//	swarvex arbeiten --projekt <slug> …          eine Aufgabe in der Sandbox bearbeiten
//	swarvex proxy                                 Schlüssel-Proxy (läuft im Proxy-Container)
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Befehle: login | arbeiten | proxy")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "proxy":
		proxyStarten()
	case "login":
		loginBefehl(os.Args[2:])
	case "arbeiten":
		arbeitenBefehl(os.Args[2:])
	case "angriffstest":
		angriffstestBefehl(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, "unbekannter Befehl:", os.Args[1])
		os.Exit(2)
	}
}
