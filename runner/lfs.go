package main

// Git-LFS im Arbeitsbaum auflösen: Der Snapshot enthält nur Zeiger. Der Runner holt jedes
// Objekt einmal über die Plattform, prüft SHA-256 und Größe und legt es im Cache nach
// Prüfsumme ab. In den Arbeitsbaum wird KOPIERT (nicht verlinkt), damit ein Agent den
// Cache für spätere Aufgaben nicht verändern kann.

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type lfsZeiger struct {
	pfad    string // relativ zum Arbeitsbaum
	oid     string
	groesse int64
}

func lfsCache() string {
	basis := os.Getenv("XDG_CACHE_HOME")
	if basis == "" {
		h, _ := os.UserHomeDir()
		basis = filepath.Join(h, ".cache")
	}
	return filepath.Join(umziehen(filepath.Join(basis, "swarvex"), filepath.Join(basis, "schwarm-runner")), "lfs")
}

func zeigerLesen(pfad string) (lfsZeiger, bool) {
	info, err := os.Lstat(pfad)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1024 {
		return lfsZeiger{}, false
	}
	roh, err := os.ReadFile(pfad)
	if err != nil || !bytes.HasPrefix(roh, []byte("version https://git-lfs.github.com/spec/v1")) {
		return lfsZeiger{}, false
	}
	var z lfsZeiger
	sc := bufio.NewScanner(bytes.NewReader(roh))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "oid sha256:"); ok {
			z.oid = v
		}
		if v, ok := strings.CutPrefix(sc.Text(), "size "); ok {
			z.groesse, _ = strconv.ParseInt(v, 10, 64)
		}
	}
	if len(z.oid) != 64 {
		return lfsZeiger{}, false
	}
	return z, true
}

// lfsAufloesen ersetzt alle LFS-Zeiger im Baum durch die echten Dateien und gibt deren Pfade zurück.
func lfsAufloesen(z *zugangsdaten, token string, id int64, baum string) ([]string, error) {
	var zeiger []lfsZeiger
	filepath.WalkDir(baum, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if zg, ok := zeigerLesen(p); ok {
				zg.pfad, _ = filepath.Rel(baum, p)
				zeiger = append(zeiger, zg)
			}
		}
		return nil
	})
	if len(zeiger) == 0 {
		return nil, nil
	}
	cache := lfsCache()
	os.MkdirAll(cache, 0o700)
	c := httpClient(z.CAPfad)
	c.Timeout = 10 * time.Minute
	var (
		mu      sync.Mutex
		fehler  error
		geladen int64
		wg      sync.WaitGroup
	)
	auftraege := make(chan lfsZeiger)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for zg := range auftraege {
				n, err := lfsObjekt(c, z, token, id, cache, zg)
				mu.Lock()
				geladen += n
				if err != nil && fehler == nil {
					fehler = err
				}
				mu.Unlock()
			}
		}()
	}
	for _, zg := range zeiger {
		auftraege <- zg
	}
	close(auftraege)
	wg.Wait()
	if fehler != nil {
		return nil, fehler
	}
	pfade := make([]string, 0, len(zeiger))
	for _, zg := range zeiger {
		if err := kopieren(filepath.Join(cache, zg.oid), filepath.Join(baum, zg.pfad)); err != nil {
			return nil, err
		}
		pfade = append(pfade, filepath.ToSlash(zg.pfad))
	}
	fmt.Printf("LFS: %d Dateien, %.1f MB neu geladen, Rest aus dem Cache\n", len(zeiger), float64(geladen)/1e6)
	return pfade, nil
}

// lfsObjekt stellt ein Objekt im Cache bereit (Download nur, wenn es fehlt) und prüft es.
func lfsObjekt(c *http.Client, z *zugangsdaten, token string, id int64, cache string, zg lfsZeiger) (int64, error) {
	ziel := filepath.Join(cache, zg.oid)
	if info, err := os.Stat(ziel); err == nil && info.Size() == zg.groesse {
		return 0, nil
	}
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/aufgaben/%d/lfs/%s", z.Plattform, id, zg.oid), nil)
	req.Header.Set("authorization", "Bearer "+token)
	r, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		roh, _ := io.ReadAll(r.Body)
		return 0, fmt.Errorf("LFS %s: HTTP %d %s", zg.pfad, r.StatusCode, roh)
	}
	tmp, err := os.CreateTemp(cache, "laden-*")
	if err != nil {
		return 0, err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(r.Body, zg.groesse+1))
	tmp.Close()
	if err != nil {
		return 0, err
	}
	if n != zg.groesse || hex.EncodeToString(h.Sum(nil)) != zg.oid {
		return 0, fmt.Errorf("LFS %s: Prüfsumme oder Größe stimmt nicht – verworfen", zg.pfad)
	}
	return n, os.Rename(tmp.Name(), ziel)
}

func kopieren(von, nach string) error {
	q, err := os.Open(von)
	if err != nil {
		return err
	}
	defer q.Close()
	z, err := os.OpenFile(nach, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(z, q); err != nil {
		z.Close()
		return err
	}
	return z.Close()
}
