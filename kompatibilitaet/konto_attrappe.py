"""Attrappe eines Gitea-artigen Konto-Anbieters für den Kompatibilitätstest: genehmigt jede Anmeldung
sofort für das Testkonto „kompat“. Nur im Test-Stack (compose.yaml), nie im Labor oder live."""
import json, sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlencode, urlparse


class Attrappe(BaseHTTPRequestHandler):
    def antwort(self, status, daten=None, kopf=None):
        self.send_response(status)
        for k, v in (kopf or {}).items():
            self.send_header(k, v)
        roh = json.dumps(daten).encode() if daten is not None else b""
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(roh)))
        self.end_headers()
        self.wfile.write(roh)

    def do_GET(self):
        url = urlparse(self.path)
        q = {k: v[0] for k, v in parse_qs(url.query).items()}
        if url.path == "/login/oauth/authorize":
            return self.antwort(302, kopf={"location": q["redirect_uri"] + "?" + urlencode({"code": "testcode", "state": q["state"]})})
        if url.path == "/api/v1/user":
            return self.antwort(200, {"id": 4242, "login": "kompat", "created": "2020-01-01T00:00:00Z"})
        self.antwort(404, {"error": "unbekannt"})

    def do_POST(self):
        if urlparse(self.path).path == "/login/oauth/access_token":
            self.rfile.read(int(self.headers.get("content-length", 0)))
            return self.antwort(200, {"access_token": "testtoken", "token_type": "bearer"})
        self.antwort(404, {"error": "unbekannt"})

    def log_message(self, *a):
        pass


ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), Attrappe).serve_forever()
