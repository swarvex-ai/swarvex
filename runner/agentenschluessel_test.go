package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSchluesselIn(t *testing.T) {
	d := t.TempDir()
	schreib := func(name, inhalt string) string {
		p := filepath.Join(d, name)
		if err := os.WriteFile(p, []byte(inhalt), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	env := schluesselIn(schreib("a.env", "# x\nDEEPSEEK_API_KEY=sk-ds\nexport OPENROUTER_API_KEY='sk-or'\nKIMI_API_KEY=sk-kimi\nOTHER=1\n"))
	if env["deepseek"] != "sk-ds" || env["openrouter"] != "sk-or" || env["moonshot"] != "sk-kimi" || len(env) != 3 {
		t.Fatalf(".env: %v", env)
	}
	toml := schluesselIn(schreib("config.toml", "provider = \"deepseek\"\napi_key = \"sk-top\"\n\n[providers.openrouter]\napi_key = \"sk-or2\" # kommentar\n"))
	if toml["deepseek"] != "sk-top" || toml["openrouter"] != "sk-or2" {
		t.Fatalf("toml: %v", toml)
	}
	auth := schluesselIn(schreib("auth.json", `{"openrouter":{"type":"api","key":"sk-or3"},"moonshotai":{"type":"api","key":"sk-m"},"github-copilot":{"type":"oauth","refresh":"x"}}`))
	if auth["openrouter"] != "sk-or3" || auth["moonshot"] != "sk-m" || len(auth) != 2 {
		t.Fatalf("auth.json: %v", auth)
	}
}

func TestAgentenSchluesselOhneDoppelte(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("WSL_DISTRO_NAME", "")
	for _, k := range []string{"OPENROUTER_API_KEY", "DEEPSEEK_API_KEY"} {
		t.Setenv(k, "")
	}
	os.MkdirAll(filepath.Join(h, ".codewhale"), 0o755)
	os.MkdirAll(filepath.Join(h, ".openclaw"), 0o755)
	os.WriteFile(filepath.Join(h, ".codewhale", ".env"), []byte("DEEPSEEK_API_KEY=sk-ds\n"), 0o600)
	os.WriteFile(filepath.Join(h, ".openclaw", ".env"), []byte("DEEPSEEK_API_KEY=sk-ds\nOPENROUTER_API_KEY=sk-or\n"), 0o600)
	w := agentenSchluessel([]modellWahl{{Key: "wert:sk-or"}}) // derselbe OpenRouter-Schlüssel kam schon über Hermes
	if len(w) != 1 || w[0].URL != "https://api.deepseek.com" || w[0].Agent != "opencode" {
		t.Fatalf("erwartet nur DeepSeek einmal: %+v", w)
	}
}
