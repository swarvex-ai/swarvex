#!/usr/bin/env python3
""""My account" end to end against the test instance (compose.yaml, tunnel to localhost:3800):
sign in an agent (DCR + PKCE), sign in on the website, sign the agent out (access and refresh token
invalid at once), sign in again, sign out of the website. Usage: compat/account.py"""
import base64, hashlib, http.cookiejar, json, os, re, secrets, time, urllib.error, urllib.request
from urllib.parse import urlencode, urljoin, urlparse, parse_qs
B = os.environ.get("COMPAT_BASE", "http://localhost:3800")
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
    """Browser: follow redirects and meta refresh until the URL starts with `bis`."""
    for _ in range(20):
        if url.startswith(bis): return url
        s, h, t = anfrage("GET", url)
        ziel = h.get("Location") or h.get("location")
        if not ziel:
            m = re.search(r'http-equiv="refresh" content="\d+;url=([^"]+)"', t)
            if not m: raise SystemExit(f"stuck at {url}: {s} {t[:200]}")
            ziel = m.group(1).replace("&amp;", "&")
        url = urljoin(url if url.startswith("http") else B + url, ziel)
    raise SystemExit("too many steps")
def pruefe(name, ok, info=""):
    print(("✔" if ok else "✘"), name, info); assert ok

# 1. Sign in an agent via DCR + PKCE
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
tok = json.loads(t); pruefe("agent has tokens", "access_token" in tok and "refresh_token" in tok, s)
mcp = lambda: anfrage("POST", "/mcp", {"jsonrpc": "2.0", "id": 1, "method": "tools/list"}, {"authorization": "Bearer " + tok["access_token"], "accept": "application/json, text/event-stream"})[0]
pruefe("MCP with token", mcp() == 200)

# 2. Website account
pruefe("401 without session", anfrage("GET", "/api/account")[0] == 401)
ende = folgen("/api/account/login?lang=de", B + "/konto")
pruefe("sign-in ends on /konto", ende == B + "/konto", ende)
s, _, t = anfrage("GET", "/api/account"); d = json.loads(t)
agent = next(a for a in d["agents"] if a["client_id"] == client)
pruefe("account shows agent signed in", s == 200 and agent["signed_in"], d["person"]["login"])

# 3. Sign out: foreign origin rejected, own origin ok
pruefe("403 without origin", anfrage("POST", f"/api/account/agents/{agent['id']}/sign-out", {})[0] == 403)
pruefe("403 with foreign origin", anfrage("POST", f"/api/account/agents/{agent['id']}/sign-out", {}, {"origin": "https://evil.example"})[0] == 403)
s, _, t = anfrage("POST", f"/api/account/agents/{agent['id']}/sign-out", {}, {"origin": B})
pruefe("agent signed out", s == 200, t)
pruefe("old access token rejected", mcp() == 401)
s, _, t = anfrage("POST", "/token", {"grant_type": "refresh_token", "refresh_token": tok["refresh_token"], "client_id": client}, form)
pruefe("refresh token invalid", s == 400 and "invalid_grant" in t, s)
agent = next(a for a in json.loads(anfrage("GET", "/api/account")[2])["agents"] if a["client_id"] == client)
pruefe("account shows signed out", not agent["signed_in"] and agent["signed_out"])

# 4. Signing in again works
code = parse_qs(urlparse(folgen(auth, "http://127.0.0.1:9/cb")).query)["code"][0]
time.sleep(1.1)  # the new token must be issued after the sign-out (iat is in seconds)
s, _, t = anfrage("POST", "/token", {"grant_type": "authorization_code", "code": code, "redirect_uri": "http://127.0.0.1:9/cb", "client_id": client,
                                      "code_verifier": verifier, "resource": B + "/mcp"}, form)
tok = json.loads(t); pruefe("signed in again, MCP works again", mcp() == 200)

# 5. Sign out of the website
pruefe("website sign-out", anfrage("POST", "/api/account/sign-out", {}, {"origin": B})[0] == 200)
pruefe("401 afterwards", anfrage("GET", "/api/account")[0] == 401)
print("all green")
