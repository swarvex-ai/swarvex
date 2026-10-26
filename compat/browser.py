#!/usr/bin/env python3
"""Stand-in browser for the compatibility test: follows a sign-in URL like a person clicking "continue"
everywhere (redirects, meta refresh, cookies) until the agent receives its code on loopback.
Usage: browser.py <url>   (set as $BROWSER; logs to $COMPAT_LOG)"""
import http.cookiejar, os, re, sys, time, urllib.request
from urllib.parse import urljoin

protokoll = open(os.environ.get("COMPAT_LOG", os.devnull), "a")
keks = http.cookiejar.CookieJar()
oeffner = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(keks))
url = sys.argv[-1]
for schritt in range(20):
    print(f"[browser] {url[:160]}", file=protokoll, flush=True)
    try:
        with oeffner.open(url, timeout=20) as r:
            text = r.read().decode("utf-8", "replace")
            ende = r.geturl()
    except Exception as e:  # the agent's loopback server sometimes closes right away
        print(f"[browser] end: {e}", file=protokoll, flush=True)
        break
    m = re.search(r'http-equiv="refresh" content="\d+;url=([^"]+)"', text)
    if not m:
        print(f"[browser] done at {ende[:120]}", file=protokoll, flush=True)
        break
    url = urljoin(ende, m.group(1).replace("&amp;", "&"))
    time.sleep(0.1)
