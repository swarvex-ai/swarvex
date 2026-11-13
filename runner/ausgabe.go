package main

import (
	"fmt"
	"os/exec"
	"sync"
)

// Ausgabe von Containern höchstens bis zu einer Grenze im Speicher halten (Sicherheitsaudit 12.11.2026): Code im
// Container (auch der Patch eines anderen Menschen bei einer Gegenprüfung) kann beliebig viel ausgeben; vorher landete
// alles im Speicher des Runners und in agent.log, außerhalb der Speichergrenze des Containers. Behalten werden Anfang
// und Ende, dazwischen ein Hinweis; alle Auswertungen lesen ohnehin nur das Ende.
const ausgabeGrenze = 8 << 20

type begrenzteAusgabe struct {
	mu        sync.Mutex
	grenze    int
	kopf      []byte
	schwanz   []byte
	verworfen int64
}

func (b *begrenzteAusgabe) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	kopfGrenze := b.grenze / 4
	if rest := kopfGrenze - len(b.kopf); rest > 0 {
		teil := min(rest, len(p))
		b.kopf = append(b.kopf, p[:teil]...)
		p = p[teil:]
	}
	b.schwanz = append(b.schwanz, p...)
	if schwanzGrenze := b.grenze - kopfGrenze; len(b.schwanz) > 2*schwanzGrenze {
		weg := len(b.schwanz) - schwanzGrenze
		b.verworfen += int64(weg)
		b.schwanz = append(b.schwanz[:0:0], b.schwanz[weg:]...)
	}
	return n, nil
}

// Bytes: Anfang, Hinweis auf Ausgelassenes, Ende.
func (b *begrenzteAusgabe) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	schwanz, verworfen := b.schwanz, b.verworfen
	if schwanzGrenze := b.grenze - b.grenze/4; len(schwanz) > schwanzGrenze {
		verworfen += int64(len(schwanz) - schwanzGrenze)
		schwanz = schwanz[len(schwanz)-schwanzGrenze:]
	}
	aus := append([]byte{}, b.kopf...)
	if verworfen > 0 {
		aus = append(aus, fmt.Sprintf("\n[… %d bytes of output left out …]\n", verworfen)...)
	}
	return append(aus, schwanz...)
}

// begrenztAusfuehren: wie cmd.CombinedOutput, aber mit begrenztem Puffer.
func begrenztAusfuehren(cmd *exec.Cmd) ([]byte, error) {
	b := &begrenzteAusgabe{grenze: ausgabeGrenze}
	cmd.Stdout, cmd.Stderr = b, b
	err := cmd.Run()
	return b.Bytes(), err
}
