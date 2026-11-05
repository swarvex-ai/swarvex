#!/usr/bin/env python3
"""Leiste oben in jedem Dashboard: alle anderen Dashboards des Bereichs, je ein Klick.

Die Leiste nimmt alle Dashboards mit dem Schlagwort „swarvex“ im selben Bereich: Admins (Bereich „Betrieb“) sehen
damit alle fünf, Team Marketing (Bereich „Marketing“) nur Marketing und Besucher. Kein Wechsel des Bereichs nötig.

Die Generatoren (analytik-dashboard.py, sicherheit-dashboard.py) rufen das am Ende auf; von Hand:
    python3 infra/monitoring/grafana/navigation.py
"""
import json
from pathlib import Path

hier = Path(__file__).parent
DATEIEN = [hier / "analytik" / "analytik.json", hier / "analytik" / "website.json", hier / "dashboards" / "server.json",
           hier / "dashboards" / "sicherheit.json", hier / "dashboards" / "swarvex-intern.json"]
LEISTE = [{"type": "dashboards", "title": "Dashboards", "tags": ["swarvex"], "asDropdown": False, "icon": "dashboard",
           "keepTime": True, "includeVars": False, "targetBlank": False, "tooltip": ""}]


def alle():
    for datei in DATEIEN:
        d = json.loads(datei.read_text())
        assert "swarvex" in d.get("tags", []), datei
        d["links"] = LEISTE
        datei.write_text(json.dumps(d, ensure_ascii=False, indent=1) + "\n")
    print(f"navigation: {len(DATEIEN)} dashboards linked")


if __name__ == "__main__":
    alle()
