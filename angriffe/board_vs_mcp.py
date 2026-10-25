#!/usr/bin/env python3
"""P9: Statusboard und MCP-Werkzeug `kennzahlen` müssen dieselben Werte liefern."""
import json, os, ssl, urllib.parse, urllib.request
z = json.load(open(os.path.expanduser("~/.config/swarvex/zugang.json")))
ctx = ssl.create_default_context(cafile=z["ca_pfad"])
def post(pfad, daten, kopf, form=False):
    roh = urllib.parse.urlencode(daten).encode() if form else json.dumps(daten).encode()
    kopf = {**kopf, "content-type": "application/x-www-form-urlencoded" if form else "application/json"}
    with urllib.request.urlopen(urllib.request.Request(z["plattform"] + pfad, roh, kopf, method="POST"), context=ctx) as r:
        return json.loads(r.read())
t = post("/token", {"grant_type": "refresh_token", "refresh_token": z["refresh_token"], "client_id": z["client_id"], "resource": z["plattform"] + "/mcp"}, {}, form=True)
z["refresh_token"] = t.get("refresh_token", z["refresh_token"]); json.dump(z, open(os.path.expanduser("~/.config/swarvex/zugang.json"), "w"), indent=2)
for slug in ["rend-and-run-test", "angriffstest"]:
    mcp = post("/mcp", {"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "kennzahlen", "arguments": {"projekt": slug}}},
               {"authorization": "Bearer " + t["access_token"], "accept": "application/json, text/event-stream"})
    werte_mcp = json.loads(mcp["result"]["content"][0]["text"])
    with urllib.request.urlopen(z["plattform"] + f"/projekte/{slug}/kennzahlen.json", context=ctx) as r:
        werte_board = json.loads(r.read())
    unterschiede = {k for k in werte_mcp if werte_mcp[k] != werte_board.get(k)} | (set(werte_board) - set(werte_mcp))
    print(f"{slug}: {'IDENTISCH' if not unterschiede else 'ABWEICHUNG ' + str(unterschiede)} ({len(werte_mcp)} Felder) – Fortschritt {werte_board['fortschritt_prozent']} %, Spenden {werte_board['spenden_monat']}")
