#!/usr/bin/env python3
"""Ersatz-Browser für den Kompatibilitätstest: folgt einer Anmelde-URL wie ein Mensch, der überall
„weiter“ klickt (Weiterleitungen, Meta-Refresh, Cookies), bis der Agent seinen Code auf Loopback erhält.
Aufruf: browser.py <url>   (als $BROWSER gesetzt; protokolliert nach $KOMPAT_PROTOKOLL)"""
import http.cookiejar, os, re, sys, time, urllib.request
from urllib.parse import urljoin

protokoll = open(os.environ.get("KOMPAT_PROTOKOLL", os.devnull), "a")
keks = http.cookiejar.CookieJar()
oeffner = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(keks))
url = sys.argv[-1]
for schritt in range(20):
    print(f"[browser] {url[:160]}", file=protokoll, flush=True)
    try:
        with oeffner.open(url, timeout=20) as r:
            text = r.read().decode("utf-8", "replace")
            ende = r.geturl()
    except Exception as e:  # Loopback-Server des Agenten schließt teils sofort
        print(f"[browser] Ende: {e}", file=protokoll, flush=True)
        break
    m = re.search(r'http-equiv="refresh" content="\d+;url=([^"]+)"', text)
    if not m:
        print(f"[browser] fertig bei {ende[:120]}", file=protokoll, flush=True)
        break
    url = urljoin(ende, m.group(1).replace("&amp;", "&"))
    time.sleep(0.1)
