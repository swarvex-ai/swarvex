#!/usr/bin/env python3
"""Angriffe von außen (S5, S7, S8): ein bösartiger Mitstreiter umgeht seinen Runner und spricht
die Plattform direkt an. Läuft als Runner-Nutzer auf der Mitstreiter-VM und nutzt dessen
echte Anmeldung (Refresh-Token wird rotiert und zurückgeschrieben).
Jede Zeile: "Sx BESTANDEN|GESCHEITERT …" – BESTANDEN heißt: der Angriff ist misslungen."""
import base64, json, os, ssl, time, urllib.error, urllib.parse, urllib.request, hmac, hashlib

ZUGANG = os.path.expanduser("~/.config/swarvex/zugang.json")
z = json.load(open(ZUGANG))
BASIS = z["plattform"]
ctx = ssl.create_default_context(cafile=z["ca_pfad"])

def anfrage(methode, pfad, daten=None, kopf=None, form=False):
    kopf = dict(kopf or {})
    roh = None
    if daten is not None:
        if form:
            roh = urllib.parse.urlencode(daten).encode(); kopf["content-type"] = "application/x-www-form-urlencoded"
        else:
            roh = json.dumps(daten).encode() if not isinstance(daten, bytes) else daten; kopf.setdefault("content-type", "application/json")
    req = urllib.request.Request(BASIS + pfad, data=roh, method=methode, headers=kopf)
    try:
        with urllib.request.urlopen(req, context=ctx, timeout=60) as r:
            return r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()

def tokens():
    s, roh = anfrage("POST", "/token", {"grant_type": "refresh_token", "refresh_token": z["refresh_token"], "client_id": z["client_id"], "resource": BASIS + "/mcp", "scope": "openid offline_access mcp"}, form=True)
    t = json.loads(roh)
    if t.get("refresh_token"):
        z["refresh_token"] = t["refresh_token"]
        json.dump(z, open(ZUGANG, "w"), indent=2)
    return t

def mcp(token, name, argumente):
    s, roh = anfrage("POST", "/mcp", {"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": name, "arguments": argumente}},
                     {"authorization": f"Bearer {token}", "accept": "application/json, text/event-stream"})
    if s != 200:
        return s, roh
    erg = json.loads(roh).get("result", {})
    return ("fehler" if erg.get("isError") else "ok"), erg.get("content", [{}])[0].get("text", "")

def ok(nr, text): print(f"{nr} BESTANDEN   {text}")
def nein(nr, text): print(f"{nr} GESCHEITERT {text}  <<<<<<")

t = tokens()
tok = t["access_token"]
s, _ = mcp(tok, "kennzahlen", {"projekt": "angriffstest"})
print(f"Kontrolle: echtes Token funktioniert → {s}")

# ---------- S7: Tokens ----------
s, _ = mcp("", "kennzahlen", {"projekt": "angriffstest"})
ok("S7", "ohne Token: 401") if s == 401 else nein("S7", f"ohne Token: {s}")
s, _ = mcp("Unsinn.Unsinn.Unsinn", "kennzahlen", {"projekt": "angriffstest"})
ok("S7", "Fantasie-Token: 401") if s == 401 else nein("S7", f"Fantasie-Token: {s}")
kopf, nutz, sig = tok.split(".")
claims = json.loads(base64.urlsafe_b64decode(nutz + "=="))
claims["sub"] = "1"  # sich als molty (Lead von rend-and-run-test) ausgeben
gefaelscht = kopf + "." + base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip("=") + "." + sig
s, _ = mcp(gefaelscht, "kennzahlen", {"projekt": "rend-and-run-test"})
ok("S7", "Token mit gefälschtem Nutzer (sub=molty): 401") if s == 401 else nein("S7", f"gefälschter Nutzer akzeptiert: {s}")
kopf_none = base64.urlsafe_b64encode(json.dumps({"alg": "none", "typ": "JWT"}).encode()).decode().rstrip("=")
s, _ = mcp(kopf_none + "." + nutz + ".", "kennzahlen", {"projekt": "angriffstest"})
ok("S7", "Token mit alg=none: 401") if s == 401 else nein("S7", f"alg=none akzeptiert: {s}")
if t.get("id_token"):
    s, _ = mcp(t["id_token"], "kennzahlen", {"projekt": "angriffstest"})
    ok("S7", "ID-Token statt Access-Token (falsche Zielgruppe): 401") if s == 401 else nein("S7", f"ID-Token akzeptiert: {s}")
s, roh = anfrage("GET", "/aufgaben/1/snapshot", kopf={"authorization": f"Bearer {tok}"})
ok("S7", f"Snapshot einer fremden/abgeschlossenen Aufgabe verweigert ({s})") if s in (401, 403) else nein("S7", f"Snapshot ohne Reservierung geliefert: {s}")
s, roh = mcp(tok, "einreichung_bewerten", {"einreichung_id": 1, "urteil": "ablehnen", "begruendung": "Sabotage"})
ok("S7", "Lead-Werkzeug in fremdem Projekt abgewiesen") if s == "fehler" or (isinstance(s, int) and s >= 400) else nein("S7", f"fremde Einreichung bewertet: {roh[:120]}")

# ---------- S5: verbotene Pfade direkt an der Plattform (Runner umgangen) ----------
s, roh = mcp(tok, "aufgabe_anlegen", {"projekt": "angriffstest", "titel": "Pfadtest", "beschreibung": "Nur game/erlaubt/** darf geändert werden",
                                       "stufe": "B", "faehigkeiten": ["test"], "pfade": ["game/erlaubt/**"], "akzeptanz": ["-"], "lease_minuten": 10})
aid = json.loads(roh)["id"]
mcp(tok, "aufgabe_reservieren", {"aufgabe_id": aid})
def patch(pfad):
    return f"diff --git a/{pfad} b/{pfad}\nnew file mode 100644\n--- /dev/null\n+++ b/{pfad}\n@@ -0,0 +1 @@\n+boese\n"
faelle = {
    "fremde Datei (Testsuite überschreiben)": patch("tools/run_tests.sh"),
    "Pfad mit ..": patch("game/erlaubt/../../tools/run_tests.sh"),
    "absoluter Pfad": "diff --git a//etc/passwd b//etc/passwd\n--- a//etc/passwd\n+++ b//etc/passwd\n@@ -1 +1 @@\n-x\n+y\n",
    "erlaubt + versteckte fremde Datei": patch("game/erlaubt/ok.gd") + patch(".github/workflows/boese.yml"),
    "zu groß (> 500 Zeilen)": f"diff --git a/game/erlaubt/gross.gd b/game/erlaubt/gross.gd\nnew file mode 100644\n--- /dev/null\n+++ b/game/erlaubt/gross.gd\n@@ -0,0 +1,600 @@\n" + "+x\n" * 600,
}
for name, p in faelle.items():
    s, roh = mcp(tok, "ergebnis_einreichen", {"aufgabe_id": aid, "patch": p, "zusammenfassung": "Angriff", "rechte_versichert": True})
    abgewiesen = s == "ok" and "abgewiesen" in roh
    ok("S5", f"abgewiesen: {name}") if abgewiesen else nein("S5", f"angenommen: {name} → {roh[:120]}")
s, roh = mcp(tok, "ergebnis_einreichen", {"aufgabe_id": aid, "patch": patch("game/erlaubt/ok.gd"), "zusammenfassung": "Kontrolle", "rechte_versichert": True})
print(f"Kontrolle: erlaubter Patch → {'angenommen' if 'einreichung_id' in roh else roh[:120]}")

# ---------- S8: Prüfergebnis fälschen ----------
def pruef(pfad, sig, zeit):
    return anfrage("POST", pfad, b'{"ergebnis":"ok","bericht":"alles gruen (gefaelscht)"}', {"x-zeit": zeit, "x-signatur": sig})
eid = json.loads(roh).get("einreichung_id", 1)
pfad = f"/pruefstrasse/{eid}/ergebnis"
s, _ = pruef(pfad, "", str(int(time.time() * 1000)))
ok("S8", "ohne Signatur: 401") if s == 401 else nein("S8", f"ohne Signatur: {s}")
s, _ = pruef(pfad, "0" * 64, str(int(time.time() * 1000)))
ok("S8", "falsche Signatur: 401") if s == 401 else nein("S8", f"falsche Signatur: {s}")
falsch = hmac.new(b"geraten", f"{int(time.time()*1000)}\nPOST\n{pfad}\n".encode(), hashlib.sha256).hexdigest()
s, _ = pruef(pfad, falsch, str(int(time.time() * 1000)))
ok("S8", "mit geratenem Geheimnis signiert: 401") if s == 401 else nein("S8", f"geratenes Geheimnis: {s}")
s, _ = pruef(pfad, "0" * 64, str(int(time.time() * 1000) - 3600_000))
ok("S8", "veralteter Zeitstempel (Wiederholung): 401") if s == 401 else nein("S8", f"alter Zeitstempel: {s}")
s, _ = anfrage("GET", "/pruefstrasse/offen", kopf={"authorization": f"Bearer {tok}"})
ok("S8", "Prüfstraßen-Liste mit Mitstreiter-Token verweigert") if s == 401 else nein("S8", f"Prüfstraßen-Liste: {s}")
