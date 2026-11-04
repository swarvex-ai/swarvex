package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ausführbare Dateien im Snapshot (z. B. tools/run_tests.sh) müssen in der Sandbox ausführbar bleiben.
func TestSnapshotBehaeltAusfuehrbarkeit(t *testing.T) {
	var puffer bytes.Buffer
	gz := gzip.NewWriter(&puffer)
	tw := tar.NewWriter(gz)
	for _, d := range []struct {
		name string
		mode int64
	}{{"tools/run_tests.sh", 0o775}, {"README.md", 0o664}} {
		inhalt := []byte("x\n")
		tw.WriteHeader(&tar.Header{Name: d.name, Mode: d.mode, Size: int64(len(inhalt)), Typeflag: tar.TypeReg})
		tw.Write(inhalt)
	}
	tw.Close()
	gz.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(puffer.Bytes()) }))
	defer srv.Close()
	ziel := t.TempDir()
	if err := snapshotHolen(&zugangsdaten{Plattform: srv.URL}, "t", 1, ziel); err != nil {
		t.Fatal(err)
	}
	for name, soll := range map[string]os.FileMode{"tools/run_tests.sh": 0o755, "README.md": 0o644} {
		fi, err := os.Stat(filepath.Join(ziel, name))
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm()&^0o022 != soll&^0o022 {
			t.Errorf("%s: mode %v, want %v", name, fi.Mode().Perm(), soll)
		}
	}
}

func TestZusammenfassungOhneTerminalSteuerung(t *testing.T) {
	z := zusammenfassungAus("\x1b[0mCreated \x1b[1;32mfile\x1b[0m\x1b]0;title\x07 done")
	if strings.Contains(z, "\x1b") || !strings.HasSuffix(z, "Created file done") {
		t.Fatalf("%q", z)
	}
	lang := strings.Repeat("ä", 3000) // 6000 Bytes: der Schnitt darf kein halbes Zeichen hinterlassen
	if z := zusammenfassungAus(lang); !strings.HasSuffix(z, "ä") || strings.ContainsRune(z, '�') || !utf8Gueltig(z) {
		t.Fatalf("invalid cut")
	}
}

func utf8Gueltig(s string) bool { return strings.ToValidUTF8(s, "") == s }
