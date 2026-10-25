#!/usr/bin/env python3
"""Agenten-Kompatibilitätstest: Echte Agenten-Programme melden sich per MCP-OAuth an swarvex an
und rufen danach die Werkzeuge auf. Jeder Agent bekommt eine eigene, leere Konfiguration
(eigenes HOME bzw. Konfig-Verzeichnis); die echte Einrichtung des Rechners bleibt unberührt.

Voraussetzung: Test-Stack läuft (compose.yaml) und ist per Tunnel als http://localhost:3800 erreichbar:
  ssh -f -N -L 3800:localhost:3800 -L 3999:localhost:3999 swarvex

Aufruf: kompatibilitaet/agenten.py [agent …]     (ohne Angabe: alle installierten)
Ergebnis: Tabelle auf der Konsole und kompatibilitaet/ergebnis.md
"""
import json, os, pty, re, select, shutil, subprocess, sys, tempfile, time
from pathlib import Path

HIER = Path(__file__).resolve().parent
URL = "http://localhost:3800/mcp"
AUTH = re.compile(r"http://localhost:3800/auth\?[A-Za-z0-9%._~:/?&=+-]+")


def in_pty(befehl, env, cwd, dauer=120, browser_datei=None, antworten=()):
    """Befehl in einem Pseudo-Terminal ausführen; die Anmelde-URL aus der Ausgabe (oder vom
    $BROWSER-Aufruf) mit dem Ersatz-Browser öffnen. Liefert (Exit-Code, Ausgabe)."""
    pid, fd = pty.fork()
    if pid == 0:
        os.chdir(cwd)
        os.execvpe(befehl[0], befehl, env)
    ausgabe, geoeffnet, ende, beantwortet = b"", False, time.time() + dauer, 0
    while time.time() < ende:
        r, _, _ = select.select([fd], [], [], 0.5)
        if r:
            try:
                stueck = os.read(fd, 65536)
            except OSError:  # Terminal geschlossen: Programm endet gerade, regulär abwarten
                stueck = b""
            if not stueck:
                _, status = os.waitpid(pid, 0)
                return os.waitstatus_to_exitcode(status), ausgabe.decode("utf-8", "replace")
            ausgabe += stueck
        for muster, eingabe in antworten:  # Rückfragen wie „[Y/n]“ beantworten
            treffer = len(re.findall(muster, ausgabe.decode("utf-8", "replace")))
            if treffer > beantwortet:
                beantwortet = treffer
                os.write(fd, eingabe.encode())
        if not geoeffnet:
            text = ausgabe.decode("utf-8", "replace")
            if browser_datei and browser_datei.exists():
                text += browser_datei.read_text()
            m = AUTH.search(text)
            if m:
                geoeffnet = True
                subprocess.Popen([sys.executable, str(HIER / "browser.py"), m.group(0)], env=env)
        fertig, status = os.waitpid(pid, os.WNOHANG)
        if fertig:
            return os.waitstatus_to_exitcode(status), ausgabe.decode("utf-8", "replace")
    try:
        os.kill(pid, 9)
        os.waitpid(pid, 0)
    except OSError:
        pass
    return None, ausgabe.decode("utf-8", "replace")


def lauf(befehl, env, cwd, dauer=60):
    r = subprocess.run(befehl, env=env, cwd=cwd, capture_output=True, text=True, timeout=dauer)
    return r.returncode, r.stdout + r.stderr


def umgebung(heim):
    """Leeres Zuhause pro Agent; der Ersatz-Browser schreibt die geöffnete URL in eine Datei."""
    heim.mkdir(parents=True, exist_ok=True)
    browser_datei = heim / "browser-url.txt"
    oeffnen = heim / "oeffnen.sh"
    oeffnen.write_text(f'#!/bin/sh\necho "$@" >> "{browser_datei}"\n')
    oeffnen.chmod(0o755)
    env = {k: v for k, v in os.environ.items() if not k.startswith(("CLAUDE", "CODEX", "HERMES", "OPENCODE", "OPENCLAW", "GEMINI"))}
    env.update(HOME=str(heim), XDG_CONFIG_HOME=str(heim / ".config"), XDG_DATA_HOME=str(heim / ".local/share"),
               BROWSER=str(oeffnen), KOMPAT_PROTOKOLL=str(heim / "browser.log"), TERM="xterm-256color", NO_COLOR="1")
    return env, browser_datei


# Jeder Agent: (Programm, Einrichtung, Anmeldung, Prüfung mit erwarteter Ausgabe)
def claude(heim, prog):
    env, b = umgebung(heim)
    env["CLAUDE_CONFIG_DIR"] = str(heim / ".claude")
    lauf([prog, "mcp", "add", "--transport", "http", "swarvex", URL], env, heim)
    code, aus = in_pty([prog, "mcp", "login", "swarvex"], env, heim, browser_datei=b)
    _, liste = lauf([prog, "mcp", "list"], env, heim, 90)
    zeile = next((z for z in liste.splitlines() if z.startswith("swarvex")), liste[-200:])
    return "Connected" in zeile, zeile.strip(), aus


def codex(heim, prog, registrierung="auto"):
    env, b = umgebung(heim)
    env["CODEX_HOME"] = str(heim / ".codex")
    (heim / ".codex").mkdir(exist_ok=True)
    zusatz = [] if registrierung == "auto" else ["--oauth-client-registration", registrierung]
    # Codex erkennt OAuth beim Hinzufügen und meldet sich dabei gleich an.
    code, aus = in_pty([prog, "mcp", "add", "swarvex", "--url", URL, *zusatz], env, heim, browser_datei=b)
    _, liste = lauf([prog, "mcp", "list"], env, heim, 60)
    zeile = next((z for z in liste.splitlines() if z.startswith("swarvex")), liste.strip()[-160:])
    erfolg = code == 0 and re.search(r"(?i)success|logged in|authenticat", aus) is not None
    return erfolg, f"{re.sub(chr(27) + r'\[[0-9;]*m', '', aus).strip().splitlines()[-1][:80]} · {' '.join(zeile.split())[:80]}", aus + liste


def hermes(heim, prog):
    # Eintrag wie in der Hermes-Doku (mcp_servers in config.yaml); `hermes mcp add` speichert in
    # einem frischen Hermes-Verzeichnis nicht zuverlässig.
    env, b = umgebung(heim)
    env["HERMES_HOME"] = str(heim / ".hermes")
    (heim / ".hermes").mkdir(exist_ok=True)
    (heim / ".hermes/config.yaml").write_text(f"mcp_servers:\n  swarvex:\n    url: {URL}\n    auth: oauth\n")
    code, aus = in_pty([prog, "mcp", "login", "swarvex"], env, heim, browser_datei=b, antworten=[(r"\[Y/n", "y\r")])
    _, test = lauf([prog, "mcp", "test", "swarvex"], env, heim, 90)
    sauber = re.sub(chr(27) + r"\[[0-9;?]*[a-zA-Z]", "", test)
    werkzeuge = re.search(r"Tools discovered: (\d+)", sauber)
    return bool(werkzeuge) and int(werkzeuge.group(1)) > 0, werkzeuge.group(0) if werkzeuge else " ".join(sauber.split())[-120:], aus + test


def opencode(heim, prog):
    env, b = umgebung(heim)
    ordner = heim / ".config/opencode"
    ordner.mkdir(parents=True, exist_ok=True)
    (ordner / "opencode.json").write_text(json.dumps({"$schema": "https://opencode.ai/config.json", "mcp": {"swarvex": {"type": "remote", "url": URL}}}))
    code, aus = in_pty([prog, "mcp", "auth", "swarvex"], env, heim, browser_datei=b)
    _, liste = lauf([prog, "mcp", "list"], env, heim, 90)
    sauber = " ".join(re.sub(chr(27) + r"\[[0-9;?]*[a-zA-Z]", "", liste).split())
    zeile = sauber[sauber.find("swarvex"):][:120] if "swarvex" in sauber else sauber[-120:]
    return code == 0 and re.search(r"(?i)connected", zeile) is not None and not re.search(r"(?i)needs auth|failed", zeile), zeile, aus + liste


def openclaw(heim, prog):
    env, b = umgebung(heim)
    lauf([prog, "mcp", "add", "swarvex", "--url", URL, "--transport", "streamable-http", "--auth", "oauth", "--no-probe"], env, heim, 90)
    code, aus = in_pty([prog, "mcp", "login", "swarvex"], env, heim, browser_datei=b)
    # Nach der Anmeldung prüft OpenClaw selbst die Verbindung und zählt die Werkzeuge.
    sauber = " ".join(re.sub(chr(27) + r"\[[0-9;?]*[a-zA-Z]", "", aus).split())
    werkzeuge = re.search(r"swarvex: (\d+) tools", sauber)
    if not werkzeuge:  # Prüfung lief nicht mit: selbst nachfragen (wartet ggf. auf OpenClaws Zustandssperre)
        _, probe = lauf([prog, "mcp", "probe"], env, heim, 180)
        aus += probe
        sauber = " ".join(re.sub(chr(27) + r"\[[0-9;?]*[a-zA-Z]", "", probe).split())
        werkzeuge = re.search(r"swarvex: (\d+) tools|(\d+) tools", sauber)
    return code is not None and bool(werkzeuge) and int(werkzeuge.group(1) or werkzeuge.group(2)) > 0, werkzeuge.group(0) if werkzeuge else sauber[-140:], aus


def gemini(heim, prog):
    # Gemini CLI startet die MCP-Anmeldung nur in der interaktiven Sitzung (/mcp auth); hier wird
    # geprüft, dass die Einrichtung klappt und der Server OAuth anbietet.
    env, b = umgebung(heim)
    lauf([prog, "mcp", "add", "--scope", "user", "--transport", "http", "swarvex", URL], env, heim, 90)
    _, liste = lauf([prog, "mcp", "list"], env, heim, 90)
    sauber = " ".join(re.sub(chr(27) + r"\[[0-9;?]*[a-zA-Z]", "", liste).split())
    return None, "Einrichtung: " + (sauber[sauber.find("swarvex"):][:110] if "swarvex" in sauber else sauber[-110:]) + " (Anmeldung nur interaktiv: /mcp auth swarvex)", liste


AGENTEN = {
    "claude": ("claude", claude),
    "codex": ("codex", codex),
    "codex-cimd": ("codex", lambda h, p: codex(h, p, "cimd")),
    "codex-dcr": ("codex", lambda h, p: codex(h, p, "dcr")),
    "hermes": ("hermes", hermes),
    "opencode": ("opencode", opencode),
    "openclaw": ("openclaw", openclaw),
    "gemini": ("gemini", gemini),
}


def main(namen):
    namen = namen or list(AGENTEN)
    (HIER / "ausgabe").mkdir(exist_ok=True)
    basis = Path(tempfile.mkdtemp(prefix="lauf-", dir=HIER / "ausgabe"))
    ergebnisse = []
    for name in namen:
        programm, pruefung = AGENTEN[name]
        prog = shutil.which(programm) or str(HIER / "werkzeuge/node_modules/.bin" / programm)
        if not Path(prog).exists():
            ergebnisse.append((name, None, "nicht installiert"))
            continue
        try:
            erfolg, info, roh = pruefung(basis / name, prog)
        except Exception as e:  # ein Agent darf den Lauf nicht abbrechen
            erfolg, info, roh = False, f"Fehler: {e}", ""
        (basis / name / "ausgabe.txt").write_text(roh)
        ergebnisse.append((name, erfolg, info))
        print(f"{'✔' if erfolg else ('–' if erfolg is None else '✘')} {name}: {info}", flush=True)
    tabelle = "| Agent | Ergebnis | Hinweis |\n|---|---|---|\n" + "".join(
        f"| {n} | {'✔' if e else ('–' if e is None else '✘')} | {i.replace('|', '/')[:140]} |\n" for n, e, i in ergebnisse)
    (HIER / "ergebnis.md").write_text(f"# Agenten-Kompatibilität\n\nStand: {time.strftime('%d.%m.%Y %H:%M')}\n\n{tabelle}\nRohdaten: {basis.relative_to(HIER)}\n")
    print(f"\nRohdaten: {basis}")


if __name__ == "__main__":
    main(sys.argv[1:])
