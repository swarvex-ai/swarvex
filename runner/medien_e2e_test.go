package main

// Durchlauf eines Medien-Auftrags mit echten Containern (Podman), ohne Plattform und ohne Modell: ein Ersatz-Server
// für Meshy im Werkzeug-Container, ein Ersatz-Agent in der Sandbox, der die Werkzeuge über MCP aufruft und die
// Abschottung ausprobiert. Nur mit SWARVEX_E2E=1 und SWARVEX_E2E_BINARY (das gebaute Programm für die Brücke):
//
//	go build -o .e2e/swarvex . && go test -c -o .e2e/medien.test .      (CGO_ENABLED=0)
//	SWARVEX_E2E=1 SWARVEX_E2E_BINARY=$PWD/.e2e/swarvex .e2e/medien.test -test.run TestMedienDurchlauf -test.v

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const ersatzMeshy = `
const fs = require('fs');
const rl = require('readline').createInterface({input: process.stdin});
const aus = (o) => process.stdout.write(JSON.stringify(o) + '\n');
console.log('Ersatz-Meshy startet');
rl.on('line', async (l) => {
  const m = JSON.parse(l);
  if (m.id === undefined) return;
  if (m.method === 'initialize') return aus({jsonrpc: '2.0', id: m.id, result: {protocolVersion: '2025-06-18', capabilities: {tools: {}}, serverInfo: {name: 'meshy-ersatz', version: '1'}}});
  if (m.method === 'tools/list') return aus({jsonrpc: '2.0', id: m.id, result: {tools: [
    {name: 'meshy_text_to_3d', description: 'make', inputSchema: {type: 'object', properties: {path: {type: 'string'}}}},
    {name: 'meshy_check_network', description: 'probe', inputSchema: {type: 'object', properties: {url: {type: 'string'}}}}]}});
  if (m.method === 'tools/call' && m.params.name === 'meshy_check_network') {
    let t;
    try { const r = await fetch(m.params.arguments.url, {signal: AbortSignal.timeout(15000)}); t = 'status ' + r.status; } catch (e) { t = 'error ' + (e.cause ? e.cause.code || e.cause.message : e.message); }
    return aus({jsonrpc: '2.0', id: m.id, result: {content: [{type: 'text', text: t}]}});
  }
  if (m.method === 'tools/call') {
    const p = m.params.arguments.path;
    let ok = 'saved';
    try { fs.writeFileSync(p, Buffer.concat([Buffer.from('glTF'), Buffer.alloc(200)])); } catch (e) { ok = 'refused ' + e.code; }
    return aus({jsonrpc: '2.0', id: m.id, result: {content: [{type: 'text', text: ok + ' ' + p + ' key=' + (process.env.MESHY_API_KEY ? 'yes' : 'no')}]}});
  }
  aus({jsonrpc: '2.0', id: m.id, error: {code: -32601, message: 'unknown'}});
});
`

const ersatzAgent = `
const fs = require('fs');
const k = JSON.parse(fs.readFileSync('/konfig/opencode.json'));
const [name, s] = Object.entries(k.mcp)[0];
let n = 0;
const rufen = async (method, params) => {
  const r = await fetch(s.url, {method: 'POST', headers: {...s.headers, 'content-type': 'application/json'}, body: JSON.stringify({jsonrpc: '2.0', id: ++n, method, params})});
  return (await r.json());
};
const text = (a) => a.result && a.result.content ? a.result.content[0].text : JSON.stringify(a);
(async () => {
  console.log('SERVER ' + name);
  console.log('INIT ' + JSON.stringify((await rufen('initialize', {protocolVersion: '2025-06-18', capabilities: {}, clientInfo: {name: 'ersatz', version: '1'}})).result.serverInfo));
  console.log('TOOLS ' + (await rufen('tools/list', {})).result.tools.map((t) => t.name).join(','));
  console.log('CALL1 ' + text(await rufen('tools/call', {name: 'meshy_text_to_3d', arguments: {path: '/out/drohne.glb'}})));
  console.log('CALL2 ' + text(await rufen('tools/call', {name: 'meshy_text_to_3d', arguments: {path: '/etc/boese.glb'}})));
  console.log('CALL3 ' + text(await rufen('tools/call', {name: 'meshy_text_to_3d', arguments: {path: '/out/zwei.glb'}})));
  console.log('CALL4 ' + text(await rufen('tools/call', {name: 'meshy_text_to_3d', arguments: {path: '/out/drei.glb'}})));
  console.log('LAN ' + text(await rufen('tools/call', {name: 'meshy_check_network', arguments: {url: 'http://192.168.1.1/'}})));
  console.log('WEB ' + text(await rufen('tools/call', {name: 'meshy_check_network', arguments: {url: 'https://example.com/'}})));
  try { fs.writeFileSync('/out/selbst.glb', 'glTF'); console.log('OUT_WRITABLE'); } catch (e) { console.log('OUT_READONLY'); }
  try { await fetch('https://example.com/', {signal: AbortSignal.timeout(5000)}); console.log('SANDBOX_NET_OPEN'); } catch (e) { console.log('SANDBOX_NET_BLOCKED'); }
  console.log('SANDBOX_KEY ' + (process.env.MESHY_API_KEY ? 'yes' : 'no'));
  const falsch = await fetch(s.url, {method: 'POST', headers: {'content-type': 'application/json', Authorization: 'Bearer falsch'}, body: '{}'});
  console.log('WRONG_TOKEN ' + falsch.status);
  fs.writeFileSync('/work/RESULT', 'drohne.glb\n');
})().catch((e) => { console.log('AGENT_ERROR ' + e); process.exit(1); });
`

func TestMedienDurchlauf(t *testing.T) {
	if os.Getenv("SWARVEX_E2E") != "1" || os.Getenv("SWARVEX_E2E_BINARY") == "" {
		t.Skip("only with SWARVEX_E2E=1 and SWARVEX_E2E_BINARY")
	}
	// Podman nutzt XDG_DATA_HOME selbst (seine Images); der Test bleibt deshalb im echten Datenordner und räumt auf.
	defer os.RemoveAll(filepath.Join(datenOrdner(), "media", "990001"))
	defer os.RemoveAll(filepath.Join(datenOrdner(), "tool-cache", "claude_e2e-meshy"))
	eigenesProgramm = func() (string, error) { return os.Getenv("SWARVEX_E2E_BINARY"), nil }
	medienAgentBefehl = []string{"node", "-e", ersatzAgent}
	defer func() { medienAgentBefehl = nil }()
	b := medienBrief{ID: 990001, Projekt: "test", Dienst: "meshy", Titel: "Drohne", Anleitung: "low poly", Zielpfad: "models/drohne.glb", Dateitypen: []string{"glb"}, MaxMB: 5}
	server := []medienServer{{Quelle: "claude", Name: "e2e-meshy", Dienst: "meshy", Befehl: []string{"node", "-e", ersatzMeshy}, Env: map[string]string{"MESHY_API_KEY": "msy-nur-im-werkzeug"}}}
	e := medienEinstellung{server: server, agent: agenten["opencode"], modell: "ersatz", upstream: "http://example.invalid", zeit: 5 * time.Minute, maxAufrufe: 3}
	ordner := filepath.Join(datenOrdner(), "media", "990001")
	aus, arbeit := filepath.Join(ordner, "out"), filepath.Join(ordner, "work")
	for _, d := range []string{aus, arbeit, filepath.Join(ordner, "cache")} {
		os.MkdirAll(d, 0o755)
	}
	ctx, stopp := context.WithTimeout(context.Background(), 5*time.Minute)
	defer stopp()
	_, protokoll, err := medienSandbox(ctx, b, server, e, ordner, aus, arbeit)
	t.Log(protokoll)
	if werkzeugLog, _ := os.ReadFile(filepath.Join(ordner, "swarvex-tool-990001-1.log")); len(werkzeugLog) > 0 {
		t.Log(string(werkzeugLog))
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, erwartet := range []string{
		"SERVER meshy", `INIT {"name":"meshy-ersatz"`, "TOOLS meshy_text_to_3d,meshy_check_network",
		"CALL1 saved /out/drohne.glb key=yes", // der Schlüssel ist im Werkzeug-Container
		"CALL2 refused EROFS",                 // außerhalb von /out schreibt das Werkzeug nicht
		"CALL3 saved /out/zwei.glb",
		"CALL4 swarvex: this brief may make at most 3 generating calls",
		"OUT_READONLY", "SANDBOX_NET_BLOCKED", "SANDBOX_KEY no", "WRONG_TOKEN 401",
	} {
		if !strings.Contains(protokoll, erwartet) {
			t.Errorf("missing %q", erwartet)
		}
	}
	if !strings.Contains(protokoll, "LAN status 403") && !strings.Contains(protokoll, "LAN error") {
		t.Errorf("the tool must not reach the home network")
	}
	if !strings.Contains(protokoll, "WEB status 200") {
		t.Logf("note: the tool did not reach the internet through the egress proxy (offline?)")
	}
	if d, grund := medienEinsammeln(aus, arbeit, b); filepath.Base(d) != "drohne.glb" {
		t.Errorf("collected %q (%s)", d, grund)
	}
}

// opencode in der Sandbox sieht den Werkzeug-Container als verbundenen MCP-Server (ohne Modell: nur die Liste).
func TestMedienOpencodeSiehtWerkzeug(t *testing.T) {
	if os.Getenv("SWARVEX_E2E") != "1" || os.Getenv("SWARVEX_E2E_BINARY") == "" {
		t.Skip("only with SWARVEX_E2E=1 and SWARVEX_E2E_BINARY")
	}
	defer os.RemoveAll(filepath.Join(datenOrdner(), "media", "990002"))
	defer os.RemoveAll(filepath.Join(datenOrdner(), "tool-cache", "claude_e2e-meshy"))
	eigenesProgramm = func() (string, error) { return os.Getenv("SWARVEX_E2E_BINARY"), nil }
	medienAgentBefehl = []string{"opencode", "mcp", "list"}
	defer func() { medienAgentBefehl = nil }()
	b := medienBrief{ID: 990002, Projekt: "test", Dienst: "meshy", Titel: "Drohne", Zielpfad: "models/drohne.glb", Dateitypen: []string{"glb"}, MaxMB: 5}
	server := []medienServer{{Quelle: "claude", Name: "e2e-meshy", Dienst: "meshy", Befehl: []string{"node", "-e", ersatzMeshy}, Env: map[string]string{}}}
	e := medienEinstellung{server: server, agent: agenten["opencode"], modell: "ersatz", upstream: "http://example.invalid", zeit: 3 * time.Minute, maxAufrufe: 3}
	ordner := filepath.Join(datenOrdner(), "media", "990002")
	aus, arbeit := filepath.Join(ordner, "out"), filepath.Join(ordner, "work")
	for _, d := range []string{aus, arbeit, filepath.Join(ordner, "cache")} {
		os.MkdirAll(d, 0o755)
	}
	_, protokoll, err := medienSandbox(context.Background(), b, server, e, ordner, aus, arbeit)
	t.Log(protokoll)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(protokoll, "meshy") || !strings.Contains(strings.ToLower(protokoll), "connected") {
		t.Errorf("opencode does not list meshy as connected")
	}
}
