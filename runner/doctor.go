package main

// swarvex doctor: prüft alles, was ein Runner braucht, und sagt bei jedem Problem, was zu tun ist.
// Liest nur; Schlüssel werden nie ausgegeben (nur, ob die Datei da und lesbar ist).

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

type befund struct {
	ok, warnung bool
	text, rat   string
}

func doctorBefehl() {
	var befunde []befund
	gut := func(text string) { befunde = append(befunde, befund{ok: true, text: text}) }
	schlecht := func(text, rat string) { befunde = append(befunde, befund{text: text, rat: rat}) }
	hinweis := func(text, rat string) { befunde = append(befunde, befund{warnung: true, text: text, rat: rat}) }
	ausgeben := func(b befund) {
		zeichen := "✗"
		if b.ok {
			zeichen = "✓"
		} else if b.warnung {
			zeichen = "!"
		}
		fmt.Printf("%s %s\n", zeichen, b.text)
		if b.rat != "" {
			fmt.Printf("    → %s\n", b.rat)
		}
	}
	pruefe := func(f func()) {
		n := len(befunde)
		f()
		for _, b := range befunde[n:] {
			ausgeben(b)
		}
	}

	fmt.Println("swarvex doctor · runner", versionText(version))
	// 1. Anmeldung und Version
	pruefe(func() {
		z, err := zugangLaden()
		if err != nil {
			schlecht("not signed in", "swarvex login --platform "+standardPlattform)
			return
		}
		if _, err := zugriffstoken(z); err != nil {
			schlecht("sign-in at "+z.Plattform+" no longer valid ("+kuerzen(err.Error(), 120)+")", "swarvex login --platform "+z.Plattform)
		} else {
			gut("signed in at " + z.Plattform)
		}
		switch _, neu, err := neuesteVersion(z.Plattform, z.CAPfad); {
		case err != nil:
			hinweis("could not check for updates: "+kuerzen(err.Error(), 160), "")
		case istNeuer(neu, version):
			hinweis("update available: "+versionText(version)+" → "+versionText(neu), "swarvex update")
		default:
			gut("runner is up to date")
		}
	})
	// 2. Podman, Images, Netz
	pruefe(func() {
		if _, err := exec.LookPath("podman"); err != nil {
			schlecht("podman is not installed", "install Podman 4 or newer (e.g. sudo apt install podman), then run the one-liner again")
			return
		}
		ver, _ := exec.Command("podman", "version", "--format", "{{.Client.Version}}").Output()
		haupt, _ := strconv.Atoi(strings.SplitN(strings.TrimSpace(string(ver)), ".", 2)[0])
		if haupt < 4 {
			schlecht("podman "+strings.TrimSpace(string(ver))+" is too old", "install Podman 4 or newer")
		} else {
			gut("podman " + strings.TrimSpace(string(ver)))
		}
		if roh, _ := exec.Command("podman", "info", "--format", "{{.Host.Security.Rootless}}").Output(); strings.TrimSpace(string(roh)) != "true" {
			schlecht("podman does not run rootless for this user", "run the runner as a normal user, not as root")
		}
		bereit := true
		for _, image := range []string{"swarvex-proxy", "swarvex-agent"} {
			if exec.Command("podman", "image", "exists", image).Run() != nil {
				schlecht("image "+image+" is missing", "swarvex update --force (rebuilds the images)")
				bereit = false
			}
		}
		if exec.Command("podman", "network", "exists", sandboxNetz).Run() != nil {
			hinweis("isolated network "+sandboxNetz+" is not there yet", "the runner creates it before the first task (podman network create --internal --disable-dns "+sandboxNetz+")")
		}
		if bereit {
			gut("images and isolated network ready")
		}
	})
	// 3. Einstellungen und Modell
	pruefe(func() {
		args := gespeicherteArgs()
		if args == nil {
			schlecht("no settings yet", "swarvex setup")
			return
		}
		modell, _ := argWert(args, "model")
		agent, _ := argWert(args, "agent")
		if agent == "" {
			agent = "opencode"
		}
		gut("settings: agent " + agent + ", model " + modell)
		if datei, ok := argWert(args, "model-key-file"); ok && datei != "" {
			if f, err := os.Open(datei); err != nil {
				schlecht("key file is not readable: "+datei, "swarvex setup (enter the key again)")
			} else {
				f.Close()
			}
		}
		if adresse, ok := argWert(args, "model-url"); ok && adresse != "" {
			if ok, aus := erreichbar(adresse); ok {
				gut("the sandbox reaches the model at " + adresse)
			} else {
				schlecht("the sandbox does not reach the model at "+adresse+" ("+kuerzen(aus, 160)+")",
					"a local model server must listen on the container network too (Ollama: OLLAMA_HOST=0.0.0.0), or check the address with swarvex setup")
			}
		}
	})
	// 4. Dienst
	pruefe(func() {
		if runtime.GOOS != "linux" {
			return
		}
		if _, err := os.Stat(dienstDatei()); err != nil {
			hinweis("no background service set up", "swarvex setup → as a service (or run tasks yourself with swarvex work)")
			return
		}
		zustand, _ := systemctl("is-active", dienstName)
		switch zustand {
		case "active":
			gut("service is running")
		case "failed":
			schlecht("service has failed", "swarvex logs (shows why), then swarvex start")
		default:
			hinweis("service is "+zustand, "swarvex start")
		}
		if aus, _ := exec.Command("loginctl", "show-user", os.Getenv("USER"), "-p", "Linger", "--value").Output(); strings.TrimSpace(string(aus)) != "yes" && !istWSL() {
			hinweis("the service only runs while you are logged in", "loginctl enable-linger")
		}
	})
	// 5. Neue Einstellungen, die noch niemand beantwortet hat (z. B. nach einem automatischen Update)
	pruefe(func() {
		if t := offeneTitel(); t != "" {
			hinweis("new settings to choose: "+t, "swarvex setup --new (asks only these)")
		}
	})
	// 6. Speicherplatz
	pruefe(func() {
		var st syscall.Statfs_t
		ordner := datenOrdner()
		os.MkdirAll(ordner, 0o755)
		if syscall.Statfs(ordner, &st) != nil {
			return
		}
		frei := int64(st.Bavail) * int64(st.Bsize) / (1 << 30)
		if frei < 5 {
			schlecht(fmt.Sprintf("only %d GB free for work data (%s)", frei, ordner), "free some space; tasks with large assets need several GB")
		} else {
			gut(fmt.Sprintf("%d GB free for work data", frei))
		}
	})

	fehler := 0
	for _, b := range befunde {
		if !b.ok && !b.warnung {
			fehler++
		}
	}
	if fehler > 0 {
		fmt.Printf("\n%d problem(s) found.\n", fehler)
		os.Exit(1)
	}
	fmt.Println("\nAll good.")
}

// hatFlag: ein boolsches Flag (--name, --name=true) ist gesetzt.
func hatFlag(args []string, name string) bool {
	an := false
	for _, a := range args {
		n, w, mitGleich := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if strings.HasPrefix(a, "-") && n == name {
			an = !mitGleich || w == "true" || w == "1"
		}
	}
	return an
}
