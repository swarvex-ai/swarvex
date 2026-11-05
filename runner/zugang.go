package main

// Anmeldung per OAuth Device Flow (RFC 8628) und Token-Erneuerung.
// Der Runner registriert sich beim ersten Login selbst als nativer Client (RFC 7591).
// Gespeichert werden nur Client-ID und Refresh-Token (Datei 0600) – nie ein Passwort.

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type zugangsdaten struct {
	Plattform    string `json:"platform"`
	CAPfad       string `json:"ca_path,omitempty"`
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
}

// zugangAlt: Format vor der Umstellung auf Englisch (zugang.json); wird beim ersten Laden übernommen.
type zugangAlt struct {
	Plattform    string `json:"plattform"`
	CAPfad       string `json:"ca_pfad"`
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
}

func konfigOrdner() string {
	basis := os.Getenv("XDG_CONFIG_HOME")
	if basis == "" {
		h, _ := os.UserHomeDir()
		basis = filepath.Join(h, ".config")
	}
	return umziehen(filepath.Join(basis, "swarvex"), filepath.Join(basis, "schwarm-runner"))
}

// umziehen übernimmt einen Ordner unter dem alten Namen (vor der Umbenennung in swarvex),
// damit niemand sich neu anmelden muss. Liefert immer den neuen Pfad.
func umziehen(neu, alt string) string {
	if _, err := os.Stat(neu); os.IsNotExist(err) {
		if _, err := os.Stat(alt); err == nil {
			_ = os.Rename(alt, neu)
		}
	}
	return neu
}

func zugangLaden() (*zugangsdaten, error) {
	roh, err := os.ReadFile(filepath.Join(konfigOrdner(), "access.json"))
	if err != nil {
		alt, err2 := os.ReadFile(filepath.Join(konfigOrdner(), "zugang.json"))
		if err2 != nil {
			return nil, fmt.Errorf("not signed in: run `swarvex login` first")
		}
		var a zugangAlt
		if err := json.Unmarshal(alt, &a); err != nil {
			return nil, err
		}
		z := &zugangsdaten{Plattform: a.Plattform, CAPfad: a.CAPfad, ClientID: a.ClientID, RefreshToken: a.RefreshToken}
		if err := zugangSpeichern(z); err == nil {
			os.Remove(filepath.Join(konfigOrdner(), "zugang.json"))
		}
		return z, nil
	}
	var z zugangsdaten
	return &z, json.Unmarshal(roh, &z)
}

func zugangSpeichern(z *zugangsdaten) error {
	if err := os.MkdirAll(konfigOrdner(), 0o700); err != nil {
		return err
	}
	roh, _ := json.MarshalIndent(z, "", "  ")
	// Erst ganz schreiben, dann umbenennen: Wer gleichzeitig liest, sieht nie eine halbe Datei.
	ziel := filepath.Join(konfigOrdner(), "access.json")
	if err := os.WriteFile(ziel+".neu", roh, 0o600); err != nil {
		return err
	}
	return os.Rename(ziel+".neu", ziel)
}

// zugangSperren: Es erneuert immer nur ein Prozess zugleich (Dienst, laufende Aufgabe, swarvex status, Gegenprüfung).
// Die Plattform tauscht das Refresh-Token bei jeder Erneuerung aus und widerruft die ganze Anmeldung, sobald ein
// schon verbrauchtes noch einmal kommt (Schutz gegen gestohlene Tokens).
func zugangSperren() func() {
	if os.MkdirAll(konfigOrdner(), 0o700) != nil {
		return func() {}
	}
	f, err := os.OpenFile(filepath.Join(konfigOrdner(), "access.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return func() {}
	}
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX) != nil {
		f.Close()
		return func() {}
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }
}

// errAbgemeldet: Die Plattform hat die Anmeldung abgelehnt (abgelaufen oder widerrufen), kein Netzfehler.
var errAbgemeldet = errors.New("token refresh failed")

// httpClient vertraut zusätzlich der lokalen CA der Plattform (Machbarkeitsnachweis).
func httpClient(caPfad string) *http.Client {
	pool, _ := x509.SystemCertPool()
	if pool == nil {
		pool = x509.NewCertPool()
	}
	if caPfad != "" {
		if pem, err := os.ReadFile(caPfad); err == nil {
			pool.AppendCertsFromPEM(pem)
		}
	}
	return &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}
}

func formPost(c *http.Client, ziel string, werte url.Values, aus any) (int, error) {
	r, err := c.PostForm(ziel, werte)
	if err != nil {
		return 0, err
	}
	defer r.Body.Close()
	return r.StatusCode, json.NewDecoder(r.Body).Decode(aus)
}

func loginBefehl(args []string) {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	plattform := fs.String("platform", "", "address of the platform (default: the one you are signed in to, else "+standardPlattform+")")
	ca := fs.String("ca", "", "PEM file of the platform CA (local tests only)")
	wennNoetig := fs.Bool("if-needed", false, "keep a saved sign-in that still works; sign in only otherwise")
	fs.Parse(args)
	alt, _ := zugangLaden()
	if *plattform == "" {
		*plattform = standardPlattform
		if alt != nil && alt.Plattform != "" {
			*plattform = alt.Plattform
		}
	}
	basis := strings.TrimRight(*plattform, "/")
	if alt != nil && alt.Plattform == basis && *ca == "" {
		*ca = alt.CAPfad
	}
	if *wennNoetig && alt != nil && alt.Plattform == basis && alt.RefreshToken != "" {
		_, err := zugriffstoken(alt)
		switch {
		case err == nil:
			fmt.Println("   already signed in (to sign in again: swarvex login)")
			return
		case !errors.Is(err, errAbgemeldet):
			// Netz weg: Die Anmeldung kann trotzdem gültig sein, also nichts überschreiben
			fmt.Println("   could not check the saved sign-in (" + err.Error() + "); if tasks fail, run: swarvex login")
			return
		}
		fmt.Println("   the saved sign-in no longer works (expired or revoked); signing in again")
	}
	c := httpClient(*ca)

	// 1. Als nativer Client registrieren (einmal pro Gerät).
	host, _ := os.Hostname()
	reg := map[string]any{
		"client_name":                fmt.Sprintf("swarvex runner (%s)", host),
		"application_type":           "native",
		"redirect_uris":              []string{},
		"response_types":             []string{},
		"grant_types":                []string{"urn:ietf:params:oauth:grant-type:device_code", "refresh_token"},
		"token_endpoint_auth_method": "none",
	}
	roh, _ := json.Marshal(reg)
	r, err := c.Post(basis+"/reg", "application/json", strings.NewReader(string(roh)))
	pruefen(err)
	var client struct {
		ClientID string `json:"client_id"`
		Fehler   string `json:"error_description"`
	}
	json.NewDecoder(r.Body).Decode(&client)
	r.Body.Close()
	if client.ClientID == "" {
		fehlerAus("registration failed: " + client.Fehler)
	}

	// 2. Gerätecode anfordern und dem Menschen zeigen.
	var geraet struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri_complete"`
		Interval        int    `json:"interval"`
		ExpiresIn       int    `json:"expires_in"`
	}
	_, err = formPost(c, basis+"/device/auth", url.Values{
		"client_id": {client.ClientID}, "scope": {"openid offline_access mcp"}, "resource": {basis + "/mcp"},
	}, &geraet)
	pruefen(err)
	fmt.Printf("\nOpen in your browser:\n\n    %s\n\nand confirm the code  %s\n\n", geraet.VerificationURI, geraet.UserCode)
	browserOeffnen(geraet.VerificationURI)

	// 3. Warten, bis der Mensch bestätigt hat.
	intervall := time.Duration(max(geraet.Interval, 5)) * time.Second
	ende := time.Now().Add(time.Duration(geraet.ExpiresIn) * time.Second)
	for time.Now().Before(ende) {
		time.Sleep(intervall)
		var tok struct {
			RefreshToken string `json:"refresh_token"`
			Error        string `json:"error"`
		}
		_, err := formPost(c, basis+"/token", url.Values{
			"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "device_code": {geraet.DeviceCode},
			"client_id": {client.ClientID}, "resource": {basis + "/mcp"},
		}, &tok)
		pruefen(err)
		switch tok.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			intervall += 5 * time.Second
			continue
		case "":
			if tok.RefreshToken == "" {
				fehlerAus("no refresh token received")
			}
			pruefen(zugangSpeichern(&zugangsdaten{Plattform: basis, CAPfad: *ca, ClientID: client.ClientID, RefreshToken: tok.RefreshToken}))
			fmt.Println("Signed in. Access saved in", filepath.Join(konfigOrdner(), "access.json"))
			return
		default:
			fehlerAus("sign-in cancelled: " + tok.Error)
		}
	}
	fehlerAus("device code expired")
}

// zugriffstoken holt mit dem Refresh-Token ein frisches Access-Token (und speichert das rotierte Refresh-Token).
func zugriffstoken(z *zugangsdaten) (string, error) {
	frei := zugangSperren()
	defer frei()
	// Hat ein anderer Prozess inzwischen erneuert, gilt dessen Token: das eigene im Speicher ist dann schon verbraucht.
	if aktuell, err := zugangLaden(); err == nil && aktuell.Plattform == z.Plattform && aktuell.ClientID == z.ClientID && aktuell.RefreshToken != "" {
		z.RefreshToken = aktuell.RefreshToken
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Error        string `json:"error_description"`
	}
	_, err := formPost(httpClient(z.CAPfad), z.Plattform+"/token", url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {z.RefreshToken}, "client_id": {z.ClientID}, "resource": {z.Plattform + "/mcp"},
	}, &tok)
	if err != nil {
		return "", err
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("%w: %s; sign in again with `swarvex login`", errAbgemeldet, tok.Error)
	}
	if tok.RefreshToken != "" && tok.RefreshToken != z.RefreshToken {
		z.RefreshToken = tok.RefreshToken
		if err := zugangSpeichern(z); err != nil {
			return "", err
		}
	}
	return tok.AccessToken, nil
}

func pruefen(err error) {
	if err != nil {
		fehlerAus(err.Error())
	}
}

func fehlerAus(text string) {
	fmt.Fprintln(os.Stderr, "error:", text)
	os.Exit(1)
}
