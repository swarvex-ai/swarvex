#!/usr/bin/env python3
"""„Mein Konto“ Ende zu Ende gegen die Testinstanz (compose.yaml, Tunnel auf localhost:3800):
Agent anmelden (DCR + PKCE), Website-Anmeldung, Agent abmelden (Access- und Refresh-Token sofort ungültig),
neu anmelden, Website abmelden. Aufruf: kompatibilitaet/konto.py"""
import base64, hashlib, http.cookiejar, json, os, re, secrets, time, urllib.error, urllib.request
from urllib.parse import urlencode, urljoin, urlparse, parse_qs
B = os.environ.get("KOMPAT_BASIS", "http://localhost:3800")
jar = http.cookiejar.CookieJar()
class KeinFolgen(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *a): return None
roh = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar), KeinFolgen)
def anfrage(methode, url, daten=None, kopf=None):
    k = {"content-type": "application/json", **(kopf or {})}
    body = None if daten is None else (json.dumps(daten).encode() if k["content-type"] == "application/json" else urlencode(daten).encode())
    try:
        with roh.open(urllib.request.Request(urljoin(B, url), data=body, method=methode, headers=k), timeout=20) as r:
            return r.status, dict(r.headers), r.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, dict(e.headers), e.read().decode()
def folgen(url, bis):
    """Browser: Weiterleitungen und Meta-Refresh folgen, bis die URL mit `bis` beginnt."""
    for _ in range(20):
        if url.startswith(bis): return url
        s, h, t = anfrage("GET", url)
        ziel = h.get("Location") or h.get("location")
        if not ziel:
            m = re.search(r'http-equiv="refresh" content="\d+;url=([^"]+)"', t)
            if not m: raise SystemExit(f"hängt bei {url}: {s} {t[:200]}")
            ziel = m.group(1).replace("&amp;", "&")
        url = urljoin(url if url.startswith("http") else B + url, ziel)
    raise SystemExit("zu viele Schritte")
def pruefe(name, ok, info=""):
    print(("✔" if ok else "✘"), name, info); assert ok

# 1. Agent per DCR + PKCE anmelden
s, _, t = anfrage("POST", "/reg", {"client_name": "E2E-Agent", "redirect_uris": ["http://127.0.0.1:9/cb"], "token_endpoint_auth_method": "none",
                                    "grant_types": ["authorization_code", "refresh_token"], "response_types": ["code"]})
client = json.loads(t)["client_id"]
verifier = secrets.token_urlsafe(48); challenge = base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).rstrip(b"=").decode()
auth = "/auth?" + urlencode({"client_id": client, "response_type": "code", "redirect_uri": "http://127.0.0.1:9/cb", "scope": "mcp offline_access",
                             "code_challenge": challenge, "code_challenge_method": "S256", "resource": B + "/mcp", "prompt": "consent", "state": "x"})
code = parse_qs(urlparse(folgen(auth, "http://127.0.0.1:9/cb")).query)["code"][0]
form = {"content-type": "application/x-www-form-urlencoded"}
s, _, t = anfrage("POST", "/token", {"grant_type": "authorization_code", "code": code, "redirect_uri": "http://127.0.0.1:9/cb", "client_id": client,
                                      "code_verifier": verifier, "resource": B + "/mcp"}, form)
tok = json.loads(t); pruefe("Agent hat Tokens", "access_token" in tok and "refresh_token" in tok, s)
mcp = lambda: anfrage("POST", "/mcp", {"jsonrpc": "2.0", "id": 1, "method": "tools/list"}, {"authorization": "Bearer " + tok["access_token"], "accept": "application/json, text/event-stream"})[0]
pruefe("MCP mit Token", mcp() == 200)

# 2. Website-Konto
pruefe("ohne Sitzung 401", anfrage("GET", "/api/konto")[0] == 401)
ende = folgen("/api/konto/anmelden?sprache=de", B + "/konto")
pruefe("Anmeldung endet auf /konto", ende == B + "/konto", ende)
s, _, t = anfrage("GET", "/api/konto"); d = json.loads(t)
agent = next(a for a in d["agenten"] if a["client_id"] == client)
pruefe("Konto zeigt Agent angemeldet", s == 200 and agent["angemeldet"], d["mensch"]["login"])

# 3. Abmelden: fremde Herkunft abgewiesen, eigene ok
pruefe("ohne Origin 403", anfrage("POST", f"/api/konto/agenten/{agent['id']}/abmelden", {})[0] == 403)
pruefe("fremder Origin 403", anfrage("POST", f"/api/konto/agenten/{agent['id']}/abmelden", {}, {"origin": "https://boese.example"})[0] == 403)
s, _, t = anfrage("POST", f"/api/konto/agenten/{agent['id']}/abmelden", {}, {"origin": B})
pruefe("Agent abgemeldet", s == 200, t)
pruefe("altes Access-Token abgewiesen", mcp() == 401)
s, _, t = anfrage("POST", "/token", {"grant_type": "refresh_token", "refresh_token": tok["refresh_token"], "client_id": client}, form)
pruefe("Refresh-Token ungültig", s == 400 and "invalid_grant" in t, s)
agent = next(a for a in json.loads(anfrage("GET", "/api/konto")[2])["agenten"] if a["client_id"] == client)
pruefe("Konto zeigt abgemeldet", not agent["angemeldet"] and agent["abgemeldet"])

# 4. Neu anmelden geht wieder
code = parse_qs(urlparse(folgen(auth, "http://127.0.0.1:9/cb")).query)["code"][0]
time.sleep(1.1)  # neues Token muss nach der Abmeldung ausgestellt sein (iat in Sekunden)
s, _, t = anfrage("POST", "/token", {"grant_type": "authorization_code", "code": code, "redirect_uri": "http://127.0.0.1:9/cb", "client_id": client,
                                      "code_verifier": verifier, "resource": B + "/mcp"}, form)
tok = json.loads(t); pruefe("neu angemeldet, MCP geht wieder", mcp() == 200)

# 5. Website abmelden
pruefe("Website-Abmeldung", anfrage("POST", "/api/konto/abmelden", {}, {"origin": B})[0] == 200)
pruefe("danach 401", anfrage("GET", "/api/konto")[0] == 401)
print("alles grün")
