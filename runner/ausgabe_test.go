package main

import (
	"bytes"
	"strings"
	"testing"
)

// Die Ausgabe eines Containers bleibt begrenzt: Anfang und Ende bleiben, die Mitte fällt weg (Sicherheitsaudit 12.11.2026).
func TestBegrenzteAusgabe(t *testing.T) {
	b := &begrenzteAusgabe{grenze: 1000}
	b.Write([]byte("ANFANG\n"))
	for range 10000 {
		b.Write([]byte(strings.Repeat("x", 99) + "\n"))
	}
	b.Write([]byte("ENDE\n"))
	aus := b.Bytes()
	if len(aus) > 1100 {
		t.Fatalf("%d bytes kept, limit 1000", len(aus))
	}
	if !bytes.HasPrefix(aus, []byte("ANFANG\n")) || !bytes.HasSuffix(aus, []byte("ENDE\n")) || !bytes.Contains(aus, []byte("bytes of output left out")) {
		t.Fatalf("head, tail or note missing: %q … %q", aus[:40], aus[len(aus)-40:])
	}
	if len(b.schwanz) > 2*750 {
		t.Fatalf("tail buffer grew to %d", len(b.schwanz))
	}
	klein := &begrenzteAusgabe{grenze: 1000}
	klein.Write([]byte("kurz"))
	if string(klein.Bytes()) != "kurz" {
		t.Fatalf("short output changed: %q", klein.Bytes())
	}
}
