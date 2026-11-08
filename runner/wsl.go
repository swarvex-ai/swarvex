package main

// Windows über WSL2 (install.ps1): Der Runner läuft in der WSL-Distribution „swarvex“, die Sandbox darin mit Podman.
// Von hier aus öffnet er den Windows-Browser, findet Modell-Server, die unter Windows laufen, und die Codex-Anmeldung
// im Windows-Benutzerordner. Auf normalem Linux tut nichts davon etwas.

import (
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"strings"
)

func istWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	_, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop")
	return err == nil
}

// browserOeffnen öffnet eine Adresse, wenn das ohne Rückfrage geht (WSL: Windows-Standardbrowser; Linux mit Desktop: xdg-open).
// (im Test austauschbar)
var browserOeffnen = func(adresse string) {
	switch {
	case istWSL():
		exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", adresse).Start()
	case os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "":
		if _, err := exec.LookPath("xdg-open"); err == nil {
			exec.Command("xdg-open", adresse).Start()
		}
	}
}

// standardGateway liest die Adresse des Standard-Gateways aus einer Routing-Tabelle im Format von /proc/net/route.
func standardGateway(tabelle string) string {
	for _, zeile := range strings.Split(tabelle, "\n")[1:] {
		f := strings.Fields(zeile)
		if len(f) < 3 || f[1] != "00000000" {
			continue
		}
		b, err := hex.DecodeString(f[2])
		if err != nil || len(b) != 4 {
			continue
		}
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, binary.LittleEndian.Uint32(b))
		return ip.String()
	}
	return ""
}

// windowsAdressen: wo Windows aus der WSL-VM erreichbar ist (NAT: das Gateway; gespiegeltes Netz: die eigene Adresse).
func windowsAdressen() []string {
	if !istWSL() {
		return nil
	}
	var r []string
	if roh, err := os.ReadFile("/proc/net/route"); err == nil {
		if g := standardGateway(string(roh)); g != "" {
			r = append(r, g)
		}
	}
	if adressen, err := net.InterfaceAddrs(); err == nil {
		for _, a := range adressen {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && !n.IP.IsLoopback() && !enthaelt(r, n.IP.String()) {
				r = append(r, n.IP.String())
			}
		}
	}
	return r
}

// windowsHeim: der Windows-Benutzerordner als WSL-Pfad (z. B. /mnt/c/Users/anna), sonst "".
func windowsHeim() string {
	if !istWSL() {
		return ""
	}
	aus, err := exec.Command("cmd.exe", "/c", "echo %USERPROFILE%").Output()
	if err != nil {
		return ""
	}
	pfad, err := exec.Command("wslpath", "-u", strings.TrimSpace(string(aus))).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(pfad))
}
