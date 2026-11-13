#!/usr/bin/env python3
"""Softwarestand eines Servers für Prometheus (Textfile des Node-Exporters, wie infra/backup.sh).

Läuft täglich und nach dem Start (swarvex-software.timer), auf der Plattform und auf dem Prüfserver, als root:
  python3 -I /opt/swarvex/infra/software-stand.py            (--stdout: nur ausgeben, nichts schreiben)

Schreibt /var/lib/swarvex-metrics/software.prom: Betriebssystem und Kernel (laufend und installiert, Neustart nötig?),
ausstehende Paketupdates (davon Sicherheit), wann die automatischen Updates zuletzt liefen und zuletzt etwas
änderten, Docker und gVisor, und jedes Image mit Version, Alter, den Containern, die es nutzen, und ob es eine neuere
Version gibt. Neuer heißt: bei festen Versionen (grafana:13.3.1) die neueste veröffentlichte, bei mitwandernden Tags
(postgres:18) ein anderer Digest unter demselben Tag. Gefragt wird Docker Hub, ohne Anmeldung; geht das nicht, steht
dort „unknown“. Das Grafana-Dashboard Sicherheit zeigt es (infra/monitoring/grafana/sicherheit-dashboard.py).
Nichts davon ist geheim, aber auch nicht öffentlich: Die Kennzahlen bleiben im internen Monitoring.
"""
import calendar
import gzip
import json
import os
import re
import subprocess
import sys
import time
import urllib.request
from pathlib import Path

ZIEL = Path("/var/lib/swarvex-metrics/software.prom")
# Images, die aus anderen entstehen (Testumgebungen der Prüfstraße): zählen nicht einzeln
ABGELEITET = re.compile(r"^(swarvex-check-|swarvex-base-)")
SEMVER = re.compile(r"^v?(\d+)\.(\d+)\.(\d+)$")
VERSION_ENV = ("PG_VERSION", "CADDY_VERSION", "GOLANG_VERSION", "NODE_VERSION", "PYTHON_VERSION")


def befehl(*args, timeout=60):
    try:
        return subprocess.run(args, capture_output=True, text=True, timeout=timeout).stdout
    except (OSError, subprocess.TimeoutExpired):
        return ""


def wert(s):
    return str(s).replace("\\", "\\\\").replace('"', '\\"').replace("\n", " ")


def zeile(name, labels, v):
    innen = ",".join(f'{k}="{wert(x)}"' for k, x in labels.items())
    return f"{name}{{{innen}}} {v}" if labels else f"{name} {v}"


# ---------- Betriebssystem ----------

def os_stand():
    info = {}
    for z in Path("/etc/os-release").read_text().splitlines():
        if "=" in z:
            k, v = z.split("=", 1)
            info[k] = v.strip('"')
    laufend = os.uname().release
    kernel = sorted((p.name.removeprefix("vmlinuz-") for p in Path("/boot").glob("vmlinuz-*")), key=versionsschluessel)
    installiert = kernel[-1] if kernel else laufend
    offen = [z for z in befehl("apt", "list", "--upgradable").splitlines() if "upgradable" in z]
    sicherheit = [z for z in offen if "-security" in z]
    neustart = laufend != installiert or Path("/var/run/reboot-required").exists()
    return {"name": info.get("PRETTY_NAME", "?"), "laufend": laufend, "installiert": installiert, "neustart": neustart,
            "offen": len(offen), "sicherheit": len(sicherheit), "letzte_aenderung": letzte_paketaenderung(),
            "letzter_lauf": letzter_unattended_lauf()}


def versionsschluessel(s):
    return [int(t) if t.isdigit() else t for t in re.split(r"(\d+)", s)]


def letzte_paketaenderung():
    """Start der letzten Paketänderung (apt-Protokoll, auch rotierte Dateien)."""
    zeiten = []
    for datei in sorted(Path("/var/log/apt").glob("history.log*")):
        try:
            text = gzip.open(datei, "rt").read() if datei.suffix == ".gz" else datei.read_text()
        except OSError:
            continue
        for m in re.finditer(r"^Start-Date: (\d{4}-\d\d-\d\d)\s+(\d\d:\d\d:\d\d)", text, re.M):
            zeiten.append(time.mktime(time.strptime(f"{m[1]} {m[2]}", "%Y-%m-%d %H:%M:%S")))
    return max(zeiten) if zeiten else 0


def letzter_unattended_lauf():
    """Letzter Lauf der automatischen Sicherheitsupdates, auch wenn es nichts zu tun gab."""
    datei = Path("/var/log/unattended-upgrades/unattended-upgrades.log")
    try:
        zeilen = datei.read_text().splitlines()
    except OSError:
        return 0
    for z in reversed(zeilen):
        m = re.match(r"(\d{4}-\d\d-\d\d \d\d:\d\d:\d\d)", z)
        if m:
            return time.mktime(time.strptime(m[1], "%Y-%m-%d %H:%M:%S"))
    return 0


# ---------- Docker und Images ----------

def docker_hub(repo):
    """library/postgres für postgres, grafana/grafana bleibt; None für Images, die nicht von Docker Hub kommen."""
    teile = repo.split("/")
    if len(teile) > 2 or (len(teile) == 2 and ("." in teile[0] or ":" in teile[0]) and teile[0] != "docker.io"):
        return None
    if teile[0] == "docker.io":
        teile = teile[1:]
    return "/".join(teile) if len(teile) == 2 else "library/" + teile[0]


def holen(url, kopf=None, methode="GET"):
    req = urllib.request.Request(url, headers={"User-Agent": "swarvex-software-stand", **(kopf or {})}, method=methode)
    with urllib.request.urlopen(req, timeout=15) as r:
        return r.headers, r.read()


def neueste_version(hub, aktuell, hauptversion=None):
    """Höchste veröffentlichte x.y.z (mit oder ohne v, wie das aktuelle Tag), auf Wunsch nur einer Hauptversion."""
    _, roh = holen(f"https://hub.docker.com/v2/repositories/{hub}/tags?page_size=100&ordering=last_updated")
    tags = [t["name"] for t in json.loads(roh)["results"] if SEMVER.match(t["name"])]
    if hauptversion:
        tags = [t for t in tags if SEMVER.match(t)[1] == hauptversion]
    mit_v = aktuell.startswith("v")
    tags = [t for t in tags if t.startswith("v") == mit_v] or tags
    return max(tags, key=lambda t: tuple(int(x) for x in SEMVER.match(t).groups())) if tags else ""


def entfernter_digest(hub, tag):
    _, roh = holen(f"https://auth.docker.io/token?service=registry.docker.io&scope=repository:{hub}:pull")
    token = json.loads(roh)["token"]
    akzeptiert = ", ".join(["application/vnd.oci.image.index.v1+json", "application/vnd.docker.distribution.manifest.list.v2+json",
                            "application/vnd.docker.distribution.manifest.v2+json", "application/vnd.oci.image.manifest.v1+json"])
    kopf, _ = holen(f"https://registry-1.docker.io/v2/{hub}/manifests/{tag}", {"Authorization": f"Bearer {token}", "Accept": akzeptiert}, "HEAD")
    return kopf.get("Docker-Content-Digest", "")


def image_stand():
    container = {}
    if ids := befehl("docker", "ps", "-q").split():
        for z in befehl("docker", "inspect", "--format", "{{.Image}}|{{.Name}}", *ids).splitlines():
            iid, name = z.split("|", 1)
            container.setdefault(iid.removeprefix("sha256:")[:12], []).append(name.lstrip("/"))
    bilder = []
    for z in befehl("docker", "images", "--format", "{{.Repository}}|{{.Tag}}|{{.ID}}").splitlines():
        repo, tag, iid = z.split("|")
        if repo == "<none>" or tag == "<none>" or ABGELEITET.match(repo):
            continue
        roh = befehl("docker", "image", "inspect", iid)
        try:
            info = json.loads(roh)[0]
        except (ValueError, IndexError):
            continue
        env = dict(e.split("=", 1) for e in (info.get("Config") or {}).get("Env") or [] if "=" in e)
        version = tag if SEMVER.match(tag) else ""
        for k in VERSION_ENV:
            if not version and env.get(k):
                version = re.sub(r"-\d+\.pgdg.*$", "", env[k])
                if k in ("NODE_VERSION", "PYTHON_VERSION") and not repo.startswith(("node", "python")):
                    version = k.split("_")[0].lower() + " " + version
        version = version or ((info.get("Config") or {}).get("Labels") or {}).get("org.opencontainers.image.version", "") or tag
        erstellt = info.get("Created", "")[:19]
        try:
            erstellt_ts = calendar.timegm(time.strptime(erstellt, "%Y-%m-%dT%H:%M:%S"))  # Docker: UTC
        except ValueError:
            erstellt_ts = 0
        hub, neueste, update = docker_hub(repo), "", "built here"
        lokal = [d.split("@", 1)[1] for d in info.get("RepoDigests") or [] if "@" in d]
        if m := re.fullmatch(r"node (\d+)\.(\d+\.\d+)", version):
            # Hier gebaut auf node:<Hauptversion>: Gibt es eine neuere Node-Version derselben Hauptversion, ist ein Neubau
            # mit frischem Basis-Image fällig (deploy.sh zieht es mit --pull)
            try:
                neueste = "node " + neueste_version("library/node", f"{m[1]}.{m[2]}", hauptversion=m[1])
                update = "rebuild" if neueste != version else "no"
            except Exception:
                update = "unknown"
        elif hub and lokal:
            try:
                if SEMVER.match(tag):
                    neueste = neueste_version(hub, tag)
                    update = "yes" if neueste and neueste.lstrip("v") != tag.lstrip("v") else "no"
                else:
                    update = "no" if entfernter_digest(hub, tag) in lokal else "yes"
            except Exception:  # Netz, Ratenlimit, unbekanntes Repo: lieber „unbekannt“ als ein Ausfall des ganzen Laufs
                update = "unknown"
        bilder.append({"image": f"{repo}:{tag}", "version": version, "containers": ", ".join(sorted(container.get(iid[:12], []))) or "-",
                       "newest": neueste, "update": update, "erstellt": erstellt_ts})
    return bilder


def software_stand():
    s = {}
    if v := befehl("docker", "version", "--format", "{{.Server.Version}}").strip():
        s["Docker"] = v
    if m := re.search(r"release-(\S+)", befehl("runsc", "--version")):
        s["gVisor"] = m[1]
    return s


def main():
    o = os_stand()
    z = [
        "# HELP swarvex_os_info Operating system and kernel (running and newest installed)",
        "# TYPE swarvex_os_info gauge",
        zeile("swarvex_os_info", {"os": o["name"], "kernel_running": o["laufend"], "kernel_installed": o["installiert"]}, 1),
        "# HELP swarvex_os_reboot_required 1 when a newer kernel or library waits for a reboot",
        "# TYPE swarvex_os_reboot_required gauge",
        zeile("swarvex_os_reboot_required", {}, int(o["neustart"])),
        "# HELP swarvex_os_updates_pending Package updates not yet installed",
        "# TYPE swarvex_os_updates_pending gauge",
        zeile("swarvex_os_updates_pending", {"kind": "all"}, o["offen"]),
        zeile("swarvex_os_updates_pending", {"kind": "security"}, o["sicherheit"]),
        "# HELP swarvex_os_last_package_change_timestamp_seconds Start of the last package installation or upgrade",
        "# TYPE swarvex_os_last_package_change_timestamp_seconds gauge",
        zeile("swarvex_os_last_package_change_timestamp_seconds", {}, int(o["letzte_aenderung"])),
        "# HELP swarvex_os_unattended_upgrades_last_run_timestamp_seconds Last run of the automatic security updates",
        "# TYPE swarvex_os_unattended_upgrades_last_run_timestamp_seconds gauge",
        zeile("swarvex_os_unattended_upgrades_last_run_timestamp_seconds", {}, int(o["letzter_lauf"])),
        "# HELP swarvex_software_info Version of server software outside the package list view",
        "# TYPE swarvex_software_info gauge",
    ]
    z += [zeile("swarvex_software_info", {"software": k, "version": v}, 1) for k, v in software_stand().items()]
    z += ["# HELP swarvex_image_created_timestamp_seconds Container image with version, users and whether a newer one exists; value: when it was built",
          "# TYPE swarvex_image_created_timestamp_seconds gauge"]
    z += [zeile("swarvex_image_created_timestamp_seconds", {k: b[k] for k in ("image", "version", "containers", "newest", "update")}, int(b["erstellt"]))
          for b in image_stand()]
    z += ["# HELP swarvex_software_check_timestamp_seconds When this was collected", "# TYPE swarvex_software_check_timestamp_seconds gauge",
          zeile("swarvex_software_check_timestamp_seconds", {}, int(time.time()))]
    if "--stdout" in sys.argv:  # nur ansehen, nichts schreiben
        print("\n".join(z))
        return
    ZIEL.parent.mkdir(parents=True, exist_ok=True)
    tmp = ZIEL.with_suffix(".prom.tmp")
    tmp.write_text("\n".join(z) + "\n")
    tmp.chmod(0o644)
    tmp.replace(ZIEL)


if __name__ == "__main__":
    main()
