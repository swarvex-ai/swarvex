#!/usr/bin/env python3
"""Leiste oben in jedem Dashboard: ein Klick zu den anderen, auch in den anderen Bereich.

Die Links nennen den Bereich (orgId); Grafana wechselt ihn beim Öffnen selbst, wenn man Mitglied ist. Die Ziele im
Bereich „Betrieb“ öffnen nur Admins; Marketing sieht dort „kein Zugriff“, das Dashboard selbst bleibt geschützt.

Die Generatoren (analytik-dashboard.py, sicherheit-dashboard.py) rufen das am Ende auf; von Hand:
    python3 infra/monitoring/grafana/navigation.py
"""
import json
from pathlib import Path

hier = Path(__file__).parent
# uid, Titel der Leiste, Bereich (orgId), Datei
DASHBOARDS = [
    ("swarvex-analytik", "Analytics", 2, hier / "analytik" / "analytik.json"),
    ("swarvex-website", "Website (Marketing)", 2, hier / "analytik" / "website.json"),
    ("swarvex-server", "Betrieb", 1, hier / "dashboards" / "server.json"),
    ("swarvex-sicherheit", "Sicherheit", 1, hier / "dashboards" / "sicherheit.json"),
    ("swarvex-intern", "Leads und Moderation", 1, hier / "dashboards" / "swarvex-intern.json"),
]
BEREICH = {1: "Bereich Betrieb, nur Admins", 2: "Bereich Auswertung"}


def leiste(eigene_uid):
    return [{
        "type": "link", "title": titel, "url": f"/d/{uid}?orgId={org}", "tooltip": BEREICH[org],
        "icon": "dashboard", "keepTime": False, "includeVars": False, "targetBlank": False, "asDropdown": False, "tags": [],
    } for uid, titel, org, _ in DASHBOARDS if uid != eigene_uid]


def alle():
    for uid, _, _, datei in DASHBOARDS:
        d = json.loads(datei.read_text())
        assert d["uid"] == uid, datei
        d["links"] = leiste(uid)
        datei.write_text(json.dumps(d, ensure_ascii=False, indent=1) + "\n")
    print(f"navigation: {len(DASHBOARDS)} dashboards linked")


if __name__ == "__main__":
    alle()
