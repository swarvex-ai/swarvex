#!/usr/bin/env python3
"""P10: Lead (hier mitstreiter-test im Projekt lfs-test) legt eine Aufgabe an, deren Prüfung echte LFS-Dateien braucht."""
import json, os, ssl, urllib.parse, urllib.request
P = os.path.expanduser("~/.config/swarvex/access.json"); z = json.load(open(P))
ctx = ssl.create_default_context(cafile=z["ca_path"])
def post(pfad, daten, kopf, form=False):
    roh = urllib.parse.urlencode(daten).encode() if form else json.dumps(daten).encode()
    kopf = {**kopf, "content-type": "application/x-www-form-urlencoded" if form else "application/json"}
    with urllib.request.urlopen(urllib.request.Request(z["platform"] + pfad, roh, kopf, method="POST"), context=ctx) as r: return json.loads(r.read())
t = post("/token", {"grant_type": "refresh_token", "refresh_token": z["refresh_token"], "client_id": z["client_id"], "resource": z["platform"] + "/mcp"}, {}, form=True)
z["refresh_token"] = t.get("refresh_token", z["refresh_token"]); json.dump(z, open(P, "w"), indent=2)
argumente = {"project": "lfs-test", "title": "LFS-Probe: echte Sprachdatei laden", "level": "A", "capabilities": ["gdscript"],
  "paths": ["game/tests/lfs_probe_test.gd", "game/tests/lfs_probe_test.gd.uid"], "base_commit": "d52c6cf8781caeecd933668ff00e3dd857bcfb69",
  "lease_minutes": 30, "milestone": "P10",
  "description": "Lege genau eine neue Datei an: game/tests/lfs_probe_test.gd (extends SceneTree). In _initialize(): lade per load(\"res://audio/voice/pa_airlock.wav\") die Datei als AudioStream. Gib `LFSPROBE PASS laenge=<sekunden>` aus und beende mit quit(0), wenn das Laden gelingt und get_length() > 0.5 ist; sonst gib `LFSPROBE FAIL` aus und beende mit quit(1). Nichts sonst ändern. Ausführen: cd game && godot --headless --import && godot --headless --path . -s res://tests/lfs_probe_test.gd",
  "acceptance": ["Nur die neue Datei game/tests/lfs_probe_test.gd (plus .uid)", "Test lädt res://audio/voice/pa_airlock.wav und endet mit Exit 0 und LFSPROBE PASS, wenn die Datei echt ist"]}
r = post("/mcp", {"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "create_task", "arguments": argumente}},
         {"authorization": "Bearer " + t["access_token"], "accept": "application/json, text/event-stream"})
print("Aufgabe:", json.loads(r["result"]["content"][0]["text"])["id"])
