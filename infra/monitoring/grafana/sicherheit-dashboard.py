#!/usr/bin/env python3
"""Erzeugt dashboards/sicherheit.json: Bereich „Betrieb“ (nur admins), Dashboard „swarvex Sicherheit“.

Quellen: Postgres-Sichten analytik.sicherheit_* (migrations/037_sicherheit.sql, nur Zahlen), Prometheus (Zertifikate,
Erreichbarkeit, Backup; infra/monitoring/config.alloy) und Loki (SSH, Firewall, Ratenlimits, Zustimmungen, Fehler;
IP- und MAC-Adressen sind vor dem Speichern entfernt).

Nach Änderungen neu erzeugen:  python3 infra/monitoring/grafana/sicherheit-dashboard.py
"""
import json
from pathlib import Path

SQL = {"type": "grafana-postgresql-datasource", "uid": "analytik"}
PROM = {"type": "prometheus", "uid": "prometheus"}
LOKI = {"type": "loki", "uid": "loki"}
API = '{container=~"swarvex-api.*"}'

panels, y = [], 0


def zeile(titel):
    global y
    panels.append({"type": "row", "title": titel, "collapsed": False, "gridPos": {"x": 0, "y": y, "w": 24, "h": 1}, "panels": []})
    y += 1


def weiter(h):
    global y
    y += h


def schwellen(*stufen):
    """stufen: (wert, farbe) aufsteigend; der erste Wert ist die Grundfarbe."""
    return {"mode": "absolute", "steps": [{"value": None if i == 0 else w, "color": f} for i, (w, f) in enumerate(stufen)]}


def panel(typ, titel, ziel, x, w, h, quelle=SQL, einheit=None, beschreibung=None, optionen=None, felder=None, legende=None, fmt="time_series"):
    if quelle is SQL:
        targets = [{"refId": "A", "datasource": SQL, "editorMode": "code", "rawQuery": True, "format": fmt, "rawSql": " ".join(ziel.split())}]
    else:
        ziele = ziel if isinstance(ziel, list) else [(ziel, legende)]
        targets = [{"refId": chr(65 + i), "datasource": quelle, "expr": e, **({"legendFormat": l} if l else {}),
                    **({"queryType": "range"} if quelle is LOKI else {})} for i, (e, l) in enumerate(ziele)]
    p = {"type": typ, "title": titel, "datasource": quelle, "gridPos": {"x": x, "y": y, "w": w, "h": h}, "targets": targets,
         "fieldConfig": {"defaults": {**({"unit": einheit} if einheit else {}), **(felder or {})}, "overrides": []}}
    if beschreibung:
        p["description"] = beschreibung
    if optionen:
        p["options"] = optionen
    panels.append(p)


def text(titel, inhalt, x, w, h):
    panels.append({"type": "text", "title": titel, "gridPos": {"x": x, "y": y, "w": w, "h": h}, "options": {"mode": "markdown", "content": inhalt}})


STAT = {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "background", "graphMode": "none", "textMode": "value"}
BALKEN = {"legend": {"displayMode": "list", "placement": "bottom"}, "tooltip": {"mode": "multi"}}
ZEIT = {"custom": {"drawStyle": "bars", "fillOpacity": 70, "stacking": {"mode": "normal"}}}

# ---------- Auf einen Blick ----------
zeile("Auf einen Blick")
panel("stat", "Zertifikat Plattform", "min((probe_ssl_earliest_cert_expiry{job=~\".*platform\"} - time()) / 86400)", 0, 4, 4, quelle=PROM,
      einheit="d", optionen=STAT, felder={"thresholds": schwellen((0, "red"), (14, "orange"), (30, "green")), "decimals": 0},
      beschreibung="Tage bis zum Ablauf des TLS-Zertifikats von swarvex.ai. Caddy erneuert nach 60 Tagen; unter 30 stimmt etwas nicht.")
panel("stat", "Zertifikat Vorschauen", "min((probe_ssl_earliest_cert_expiry{job=~\".*previews\"} - time()) / 86400)", 4, 4, 4, quelle=PROM,
      einheit="d", optionen=STAT, felder={"thresholds": schwellen((0, "red"), (14, "orange"), (30, "green")), "decimals": 0},
      beschreibung="Tage bis zum Ablauf des Zertifikats der Vorschau-Domain (swarvex.run)")
panel("stat", "Plattform erreichbar", "min(probe_success{job=~\".*platform\"})", 8, 4, 4, quelle=PROM, optionen=STAT,
      felder={"thresholds": schwellen((0, "red"), (1, "green")), "mappings": [{"type": "value", "options": {"0": {"text": "nein"}, "1": {"text": "ja"}}}]},
      beschreibung="/healthz über HTTPS, alle 30 Sekunden von diesem Server aus")
panel("stat", "Letztes Backup", "(time() - max(swarvex_backup_last_success_timestamp_seconds)) / 3600", 12, 4, 4, quelle=PROM, einheit="h", optionen=STAT,
      felder={"thresholds": schwellen((0, "green"), (26, "red")), "decimals": 1, "noValue": "nie"},
      beschreibung="Stunden seit dem letzten gelungenen Backup (infra/backup.sh, nachts). Über 26 Stunden: das Backup ist ausgefallen.")
panel("stat", "Letzter signierter Kopf-Hash", "select alter_h as \"Stunden\" from analytik.sicherheit_kopf", 16, 4, 4, einheit="h", optionen=STAT, fmt="table",
      felder={"thresholds": schwellen((0, "green"), (26, "red")), "decimals": 1},
      beschreibung="Das Protokoll wird täglich um 03:17 UTC signiert und extern verankert. Über 26 Stunden: der Manipulationsschutz hängt.")
panel("stat", "Prüfschlange", "select wartend as \"wartend\", aelteste_h as \"älteste (h)\" from analytik.pruefschlange", 20, 4, 4, optionen={**STAT, "textMode": "value_and_name"},
      fmt="table", felder={"thresholds": schwellen((0, "green"), (10, "orange"), (30, "red"))},
      beschreibung="Einreichungen, die auf die Prüfstraße warten, und wie lange die älteste schon wartet. Wächst das, braucht die Werkstatt mehr Rechenkraft.")
weiter(4)
panel("stat", "Offene Meldungen", "select meldungen_offen as \"alle\", sicherheitsmeldungen_offen as \"Sicherheit\" from analytik.sicherheit_stand", 0, 6, 4,
      optionen={**STAT, "textMode": "value_and_name"}, fmt="table", felder={"thresholds": schwellen((0, "green"), (1, "orange"))},
      beschreibung="Meldungen über das Formular der Website (cli reports); Sicherheitsmeldungen zuerst bearbeiten")
panel("stat", "Gesperrt", "select menschen_gesperrt as \"Menschen\", projekte_gesperrt as \"Projekte\" from analytik.sicherheit_stand", 6, 6, 4,
      optionen={**STAT, "colorMode": "none", "textMode": "value_and_name"}, fmt="table")
panel("stat", "Anmeldungen", "select sitzungen as \"Sitzungen\", grants as \"Zugriffe (Grants)\", bereichs_leads as \"Bereichs-Leads\" from analytik.sicherheit_stand", 12, 6, 4,
      optionen={**STAT, "colorMode": "none", "textMode": "value_and_name"}, fmt="table",
      beschreibung="Gültige Browser-Sitzungen beim OAuth-Server, erteilte Zugriffe an Agenten und amtierende Bereichs-Leads")
panel("stat", "Zweite Schranke", "select coalesce(sum(zweite_schranke), 0) as \"Treffer\" from analytik.sicherheit_pruefungen where $__timeFilter(tag)", 18, 6, 4,
      optionen=STAT, fmt="table", felder={"thresholds": schwellen((0, "green"), (1, "red"))},
      beschreibung="Prüfstraße oder Merge-Bot fanden nach dem Anwenden eine geschützte Datei, die die Pfadprüfung der Plattform hätte abweisen müssen. "
                   "Muss 0 sein; jeder Treffer ist ein Umgehungsversuch, der an der ersten Schranke vorbeikam: Einreichung ansehen.")
weiter(4)
text("Was ist normal, was ist ein Alarm?", """\
**Normal:** Hunderte SSH-Fehlversuche und Tausende Firewall-Blocks am Tag (Bots im Internet; SSH nimmt nur Schlüssel an).
Abgewiesene Einreichungen außerhalb der erlaubten Pfade (Agenten irren sich). Ratenlimits einzelner Regeln in Spitzen.

**Alarm, sofort ansehen:** eine **erfolgreiche SSH-Anmeldung**, die nicht von einem Deploy kommt · **Zweite Schranke > 0** ·
Einreichungen mit **geschützter Datei** oder **ungültigem Pfad** (Umgehungsversuch) · **Prüfstraße: falsche Signatur** ·
ein **Sprung bei neuen OAuth-Clients** (Web-Clients) · Zertifikat unter 14 Tagen · Kopf-Hash oder Backup älter als 26 Stunden ·
viele **abgelehnte Zustimmungen** (jemand verteilt Anmelde-Links).""", 0, 24, 5)
weiter(5)

# ---------- Anmeldung und Agenten ----------
zeile("Anmeldung und Agenten")
panel("timeseries", "Menschen und Agenten je Tag",
      "select $__timeGroupAlias(tag, 1d), art as metric, sum(anzahl) from analytik.sicherheit_ereignisse where $__timeFilter(tag) "
      "and art in ('neue Menschen', 'Anmeldung abgewiesen (Konto zu neu)', 'Anmeldung mit Ausnahme', 'Agent angebunden', 'Agent abgemeldet') group by 1, 2 order by 1",
      0, 12, 8, optionen=BALKEN, felder=ZEIT,
      beschreibung="Abgewiesen: Konto beim Anbieter jünger als das Mindestalter (Schutz vor Wegwerf-Konten). Ausnahmen sind Testkonten (ACCOUNT_AGE_EXEMPT).")
panel("timeseries", "Neue OAuth-Clients je Tag",
      "select $__timeGroupAlias(tag, 1d), art as metric, sum(anzahl) from analytik.oauth_clients where $__timeFilter(tag) group by 1, 2 order by 1",
      12, 12, 8, optionen=BALKEN, felder=ZEIT,
      beschreibung="Jeder Agent registriert sich selbst (dynamische Registrierung). Web-Clients brauchen eine Zustimmung; ein Sprung ohne neue Menschen deutet auf Missbrauch.")
weiter(8)
panel("timeseries", "Zustimmungen für Web-Clients", [
      (f'sum by (type) (count_over_time({API} |= "consent_" | json | type=~"consent_(allowed|declined)" [1h]))', "{{type}}")],
      0, 12, 7, quelle=LOKI, optionen=BALKEN, felder=ZEIT,
      beschreibung="Zugelassen oder abgebrochen auf der Zustimmungsseite. Viele Abbrüche: jemand verschickt Anmelde-Links für einen fremden Client.")
panel("timeseries", "Ratenlimit-Treffer je Regel", [
      (f'sum by (rule) (sum_over_time({API} |= "rate_limited" | json | unwrap count [5m]))', "{{rule}}")],
      12, 12, 7, quelle=LOKI, optionen=BALKEN, felder=ZEIT,
      beschreibung="Abgewiesene Anfragen (HTTP 429) je Regel aus apps/api/src/grenzen.ts. anmeldung/registrierung in Spitzen: Durchprobieren.")
weiter(7)

# ---------- Beiträge und Prüfstraße ----------
zeile("Beiträge und Prüfstraße (fremder Code)")
panel("timeseries", "Abgewiesene Einreichungen nach Grund",
      "select $__timeGroupAlias(tag, 1d), grund as metric, sum(anzahl) from analytik.sicherheit_abweisungen where $__timeFilter(tag) group by 1, 2 order by 1",
      0, 12, 8, optionen=BALKEN, felder=ZEIT,
      beschreibung="Erste Schranke: die Plattform liest die Pfade mit git selbst. Geschützte Datei und ungültiger Pfad sind Umgehungsversuche, außerhalb der Pfade meist Irrtümer.")
panel("timeseries", "Prüfungen je Tag",
      "select $__timeGroupAlias(tag, 1d), sum(pruefungen) - sum(fehlgeschlagen) as \"bestanden\", sum(fehlgeschlagen) as \"fehlgeschlagen\", sum(zweite_schranke) as \"zweite Schranke\" "
      "from analytik.sicherheit_pruefungen where $__timeFilter(tag) group by 1 order by 1",
      12, 12, 8, optionen=BALKEN, felder=ZEIT)
weiter(8)
panel("timeseries", "Prüfstraße und Merge-Bot: Abweisungen", [
      ('sum(count_over_time({container=~"swarvex-api.*"} |= "pipeline: rejected" [1h]))', "falsche Signatur an der Schnittstelle"),
      ('sum(count_over_time({job="journal", unit="swarvex-merge.service"} |= "refused" [1h]))', "Merge-Bot verweigert"),
      ('sum(count_over_time({job="journal", unit=~"swarvex-(check|merge|preview).service"} |~ "(?i)panic|error:" [1h]))', "Fehler in den Diensten")],
      0, 12, 7, quelle=LOKI, optionen=BALKEN, felder=ZEIT,
      beschreibung="Die Schnittstelle /pipeline/ verlangt HMAC-Signaturen; Fehlversuche kommen von außen. Der Merge-Bot verweigert Pushes mit geschützten Dateien.")
panel("logs", "Letzte Meldungen der Prüfstraße", [('{job="journal", unit=~"swarvex-(check|merge|preview).service"} |~ "(?i)refused|rejected|protected|error"', None)],
      12, 12, 7, quelle=LOKI, optionen={"showTime": True, "wrapLogMessage": True, "sortOrder": "Descending"})
weiter(7)

# ---------- Vertrauen und Moderation ----------
zeile("Vertrauen und Moderation")
panel("timeseries", "Vertrauen, Regressionen, Meldungen, Sperren",
      "select $__timeGroupAlias(tag, 1d), art as metric, sum(anzahl) from analytik.sicherheit_ereignisse where $__timeFilter(tag) "
      "and art in ('Vertrauen gestiegen', 'Vertrauen gesunken', 'Regression gemeldet', 'Meldung eingegangen', 'Mensch gesperrt', 'Projekt gesperrt', "
      "'Prüfstraße: falsche Signatur', 'Clean Room: Trennwand gegriffen', 'Bereichs-Lead ernannt', 'Bereichs-Lead beendet') group by 1, 2 order by 1",
      0, 16, 8, optionen=BALKEN, felder=ZEIT)
panel("bargauge", "Menschen je Vertrauensstufe",
      "select case stufe when 0 then '0 neu' when 1 then '1 Prüfer' when 2 then '2 erfahren' else '3 Bereichs-Lead' end as \"Stufe\", menschen "
      "from analytik.vertrauen_verteilung order by stufe", 16, 8, 8, fmt="table",
      optionen={"orientation": "horizontal", "displayMode": "basic", "reduceOptions": {"values": True, "calcs": []}},
      beschreibung="Peer-Reviews ab Stufe 1, Bereichs-Leads ab Stufe 2 (docs: Vertrauensstufen)")
weiter(8)

# ---------- Server ----------
zeile("Server")
panel("timeseries", "SSH", [
      ('sum(count_over_time({job="journal", unit="ssh.service"} |~ "Invalid user|Connection closed by authenticating user|Failed (password|publickey)" [1h]))', "abgewiesen"),
      ('sum(count_over_time({job="journal", unit="ssh.service"} |= "Accepted " [1h]))', "angemeldet")],
      0, 12, 7, quelle=LOKI, optionen=BALKEN, felder=ZEIT,
      beschreibung="Passwort-Anmeldung ist aus; abgewiesen sind Bots. Jede Anmeldung, die nicht zu einem Deploy oder Zugriff von euch passt, ist ein Alarm.")
panel("timeseries", "Firewall-Blocks und Serverfehler", [
      ('sum(count_over_time({job="journal", transport="kernel"} |= "UFW BLOCK" [1h]))', "Firewall-Blocks"),
      (f'sum(count_over_time({API} |= "server_error" [1h]))', "API-Serverfehler (500)")],
      12, 12, 7, quelle=LOKI, optionen=BALKEN, felder=ZEIT)
weiter(7)
panel("logs", "SSH-Anmeldungen", [('{job="journal", unit="ssh.service"} |= "Accepted "', None)], 0, 12, 7, quelle=LOKI,
      optionen={"showTime": True, "wrapLogMessage": True, "sortOrder": "Descending"},
      beschreibung="Erfolgreiche Anmeldungen mit Schlüssel-Fingerabdruck (Adressen entfernt). Passt jede zu euch?")
panel("table", "Agent-Programme und Versionen (letzte 30 Tage)",
      "select programm as \"Programm\", version as \"Version\", agenten as \"Agenten\", case when runner then 'Runner' else 'direkt' end as \"Weg\" "
      "from analytik.agent_versionen order by agenten desc", 12, 12, 7, fmt="table",
      beschreibung="Aus dem MCP-Handshake. Zeigt, ob Sicherheitsupdates ankommen (Runner ab v0.9.3 melden ihre echte Version).")
weiter(7)

dashboard = {
    "uid": "swarvex-sicherheit", "title": "Sicherheit", "tags": ["swarvex", "sicherheit"], "timezone": "browser",
    "schemaVersion": 41, "refresh": "5m", "time": {"from": "now-14d", "to": "now"}, "editable": True, "panels": panels,
    "description": "Anmeldung, fremder Code, Vertrauen, Server: nur Zahlen, keine Namen, keine Adressen (infra/monitoring/grafana/sicherheit-dashboard.py)",
}
ziel = Path(__file__).parent / "dashboards" / "sicherheit.json"
ziel.write_text(json.dumps(dashboard, ensure_ascii=False, indent=1) + "\n")
print(f"{ziel}: {sum(1 for p in panels if p['type'] != 'row')} panels")

import navigation  # noqa: E402  (Leiste zu den anderen Dashboards)
navigation.alle()
