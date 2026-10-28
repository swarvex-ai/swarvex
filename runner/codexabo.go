package main

// Codex mit ChatGPT-Abo (E-32). Die echte Anmeldung (~/.codex/auth.json) bleibt auf dem Rechner:
// Der Runner liest daraus Zugangstoken und Konto und gibt beides nur dem Schlüssel-Proxy. Die
// Sandbox bekommt eine Attrappe der Anmeldung, deren Zugangstoken das Wegwerf-Token der Aufgabe ist.
// Läuft das echte Token vor dem Ende der Aufgabe ab, erneuert der Runner es so wie Codex selbst und
// schreibt es zurück, damit die eigene Codex-Anmeldung des Mitstreiters gültig bleibt.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	codexAboURL       = "https://chatgpt.com/backend-api/codex"
	codexClientID     = "app_EMoamEEZ73f0CkXaXp7hrann" // öffentliche Client-ID von Codex (codex-rs/login)
	codexKontoSandbox = "swarvex-sandbox"
)

// codexTokenURL: Erneuerung der ChatGPT-Anmeldung (variabel nur für Tests).
var codexTokenURL = "https://auth.openai.com/oauth/token"

// modellZugang: wohin der Proxy das Modell anfragt und womit er sich anmeldet.
type modellZugang struct {
	URL   string // Basis-URL ohne /v1 (bei ChatGPT: codexAboURL)
	Key   string // API-Schlüssel, Abo-Token oder ChatGPT-Zugangstoken
	Konto string // nur ChatGPT-Abo: Konto-ID (Kopfzeile ChatGPT-Account-ID)
	Plan  string // nur ChatGPT-Abo: Abo-Art, für die Attrappe in der Sandbox
}

func (z modellZugang) chatgpt() bool { return z.Konto != "" }

// istCodexAnmeldung: Ist die Schlüsseldatei eine Codex-Anmeldung (auth.json) statt eines Schlüssels?
func istCodexAnmeldung(roh []byte) bool { return bytes.HasPrefix(bytes.TrimSpace(roh), []byte("{")) }

// codexAboLaden liest die Codex-Anmeldung und erneuert das Zugangstoken, wenn es vor ablaufNach abläuft.
func codexAboLaden(pfad string, ablaufNach time.Duration) (modellZugang, error) {
	roh, err := os.ReadFile(pfad)
	if err != nil {
		return modellZugang{}, err
	}
	var datei map[string]any
	if err := json.Unmarshal(roh, &datei); err != nil {
		return modellZugang{}, fmt.Errorf("%s is not a Codex sign-in (auth.json): %v", pfad, err)
	}
	tokens, _ := datei["tokens"].(map[string]any)
	zugang, _ := tokens["access_token"].(string)
	konto, _ := tokens["account_id"].(string)
	if datei["auth_mode"] != "chatgpt" || zugang == "" || konto == "" {
		return modellZugang{}, fmt.Errorf("%s holds no ChatGPT sign-in; sign in with `codex login` first", pfad)
	}
	if ablauf, ok := jwtAblauf(zugang); !ok || time.Until(ablauf) < ablaufNach {
		if zugang, err = codexErneuern(pfad, datei, tokens); err != nil {
			return modellZugang{}, err
		}
	}
	plan := ""
	if id, ok := tokens["id_token"].(string); ok {
		if c, ok := jwtInhalt(id)["https://api.openai.com/auth"].(map[string]any); ok {
			plan, _ = c["chatgpt_plan_type"].(string)
		}
	}
	return modellZugang{URL: codexAboURL, Key: zugang, Konto: konto, Plan: plan}, nil
}

// codexErneuern: wie Codex (codex-rs/login, request_chatgpt_token_refresh); Antwort zurück in auth.json.
func codexErneuern(pfad string, datei, tokens map[string]any) (string, error) {
	refresh, _ := tokens["refresh_token"].(string)
	if refresh == "" {
		return "", fmt.Errorf("the Codex sign-in has expired; run `codex login` again")
	}
	koerper, _ := json.Marshal(map[string]string{"grant_type": "refresh_token", "client_id": codexClientID, "refresh_token": refresh})
	c := &http.Client{Timeout: 30 * time.Second}
	r, err := c.Post(codexTokenURL, "application/json", bytes.NewReader(koerper))
	if err != nil {
		return "", fmt.Errorf("renewing the Codex sign-in: %v", err)
	}
	defer r.Body.Close()
	antwort, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if r.StatusCode != http.StatusOK {
		return "", fmt.Errorf("renewing the Codex sign-in failed (HTTP %d); run `codex login` again", r.StatusCode)
	}
	var neu struct {
		ID      string `json:"id_token"`
		Zugang  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
	}
	if err := json.Unmarshal(antwort, &neu); err != nil || neu.Zugang == "" {
		return "", fmt.Errorf("renewing the Codex sign-in: unexpected answer")
	}
	for k, v := range map[string]string{"id_token": neu.ID, "access_token": neu.Zugang, "refresh_token": neu.Refresh} {
		if v != "" {
			tokens[k] = v
		}
	}
	datei["last_refresh"] = time.Now().UTC().Format(time.RFC3339Nano)
	roh, _ := json.MarshalIndent(datei, "", "  ")
	tmp := filepath.Join(filepath.Dir(pfad), ".auth.json.swarvex")
	if err := os.WriteFile(tmp, roh, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, pfad); err != nil {
		return "", err
	}
	fmt.Println("Codex sign-in renewed (written back to", pfad+")")
	return neu.Zugang, nil
}

// jwtInhalt: Nutzdaten eines JWT, ohne Signaturprüfung (nur für Ablauf und Abo-Art).
func jwtInhalt(jwt string) map[string]any {
	teile := strings.Split(jwt, ".")
	if len(teile) != 3 {
		return nil
	}
	roh, err := base64.RawURLEncoding.DecodeString(teile[1])
	if err != nil {
		return nil
	}
	var m map[string]any
	json.Unmarshal(roh, &m)
	return m
}

func jwtAblauf(jwt string) (time.Time, bool) {
	exp, ok := jwtInhalt(jwt)["exp"].(float64)
	return time.Unix(int64(exp), 0), ok
}

// jwtAttrappe: unsigniertes JWT mit den Feldern, die Codex liest. Die Sandbox hält es für echt;
// außerhalb ist es wertlos.
func jwtAttrappe(inhalt map[string]any) string {
	kopf := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	roh, _ := json.Marshal(inhalt)
	return kopf + "." + base64.RawURLEncoding.EncodeToString(roh) + ".c3dhcnZleA"
}

// codexSandboxToken: Wegwerf-Token der Aufgabe in JWT-Form (Codex prüft Format und Ablauf).
func codexSandboxToken(zufallsTeil string) string {
	return jwtAttrappe(map[string]any{"exp": time.Now().Add(24 * time.Hour).Unix(), "swarvex": zufallsTeil,
		"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": codexKontoSandbox}})
}

// codexSandboxAnmeldung: auth.json für die Sandbox. Zugangstoken = Wegwerf-Token, kein echter Refresh-Token.
func codexSandboxAnmeldung(token, plan string) []byte {
	if plan == "" {
		plan = "plus"
	}
	id := jwtAttrappe(map[string]any{"exp": time.Now().Add(24 * time.Hour).Unix(), "email": "agent@swarvex.invalid",
		"https://api.openai.com/auth": map[string]any{"chatgpt_plan_type": plan, "chatgpt_account_id": codexKontoSandbox, "chatgpt_user_id": "swarvex-agent"}})
	roh, _ := json.MarshalIndent(map[string]any{
		"auth_mode": "chatgpt", "OPENAI_API_KEY": nil, "last_refresh": time.Now().UTC().Format(time.RFC3339Nano),
		"tokens": map[string]any{"id_token": id, "access_token": token, "refresh_token": "swarvex-sandbox-no-refresh", "account_id": codexKontoSandbox},
	}, "", "  ")
	return roh
}
