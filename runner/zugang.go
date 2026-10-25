package main

// Anmeldung per OAuth Device Flow (RFC 8628) und Token-Erneuerung.
// Der Runner registriert sich beim ersten Login selbst als nativer Client (RFC 7591).
// Gespeichert werden nur Client-ID und Refresh-Token (Datei 0600) – nie ein Passwort.

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type zugangsdaten struct {
	Plattform    string `json:"plattform"`
	CAPfad       string `json:"ca_pfad,omitempty"`
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
	roh, err := os.ReadFile(filepath.Join(konfigOrdner(), "zugang.json"))
	if err != nil {
		return nil, fmt.Errorf("nicht angemeldet – zuerst `swarvex login`")
	}
	var z zugangsdaten
	return &z, json.Unmarshal(roh, &z)
}

func zugangSpeichern(z *zugangsdaten) error {
	if err := os.MkdirAll(konfigOrdner(), 0o700); err != nil {
		return err
	}
	roh, _ := json.MarshalIndent(z, "", "  ")
	return os.WriteFile(filepath.Join(konfigOrdner(), "zugang.json"), roh, 0o600)
}

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
	plattform := fs.String("plattform", "", "Adresse der Plattform, z. B. https://swarvex.ai")
	ca := fs.String("ca", "", "PEM-Datei der Plattform-CA (nur für lokale Tests)")
	fs.Parse(args)
	if *plattform == "" {
		fehlerAus("--plattform fehlt")
	}
	basis := strings.TrimRight(*plattform, "/")
	c := httpClient(*ca)

	// 1. Als nativer Client registrieren (einmal pro Gerät).
	host, _ := os.Hostname()
	reg := map[string]any{
		"client_name":                fmt.Sprintf("swarvex-Runner (%s)", host),
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
		fehlerAus("Registrierung fehlgeschlagen: " + client.Fehler)
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
	fmt.Printf("\nÖffne im Browser:\n\n    %s\n\nund bestätige den Code  %s\n\n", geraet.VerificationURI, geraet.UserCode)

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
				fehlerAus("kein Refresh-Token erhalten")
			}
			pruefen(zugangSpeichern(&zugangsdaten{Plattform: basis, CAPfad: *ca, ClientID: client.ClientID, RefreshToken: tok.RefreshToken}))
			fmt.Println("Angemeldet. Zugang gespeichert in", filepath.Join(konfigOrdner(), "zugang.json"))
			return
		default:
			fehlerAus("Anmeldung abgebrochen: " + tok.Error)
		}
	}
	fehlerAus("Gerätecode abgelaufen")
}

// zugriffstoken holt mit dem Refresh-Token ein frisches Access-Token (und speichert das rotierte Refresh-Token).
func zugriffstoken(z *zugangsdaten) (string, error) {
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
		return "", fmt.Errorf("Token-Erneuerung fehlgeschlagen: %s – neu anmelden mit `swarvex login`", tok.Error)
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
	fmt.Fprintln(os.Stderr, "Fehler:", text)
	os.Exit(1)
}
