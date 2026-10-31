#!/usr/bin/env python3
"""Erzeugt dashboards/analytik.json: interne Auswertung für Marketing und Betrieb (Sichten im Schema analytik).

Nach Änderungen neu erzeugen:  python3 infra/monitoring/grafana/analytik-dashboard.py
Alle Werte sind Summen oder Pseudonyme, keine personenbezogenen Daten (migrations/010_analytik.sql).
"""
import json
from pathlib import Path

DS = {"type": "grafana-postgresql-datasource", "uid": "analytik"}
# Filter, die fast jedes Panel nutzt (Dashboard-Variablen oben)
F = "projektart in ($projektart) and engine in ($engine) and projekt in ($projekt)"
FW = F + " and runner::text in ($weg)"

panels = []
y = 0


def zeile(titel):
    global y
    panels.append({"type": "row", "title": titel, "collapsed": False, "gridPos": {"x": 0, "y": y, "w": 24, "h": 1}, "panels": []})
    y += 1


def panel(typ, titel, sql, x, w, h, fmt="table", einheit=None, beschreibung=None, optionen=None, felder=None):
    p = {
        "type": typ, "title": titel, "datasource": DS, "gridPos": {"x": x, "y": y, "w": w, "h": h},
        "targets": [{"refId": "A", "datasource": DS, "editorMode": "code", "rawQuery": True, "format": fmt, "rawSql": " ".join(sql.split())}],
        "fieldConfig": {"defaults": {**({"unit": einheit} if einheit else {}), **(felder or {})}, "overrides": []},
    }
    if beschreibung:
        p["description"] = beschreibung
    if optionen:
        p["options"] = optionen
    panels.append(p)


def text(titel, inhalt, x, w, h):
    panels.append({"type": "text", "title": titel, "gridPos": {"x": x, "y": y, "w": w, "h": h},
                   "options": {"mode": "markdown", "content": inhalt}})


def weiter(h):
    global y
    y += h


# ---------- Überblick ----------
zeile("Überblick (Zeitraum oben rechts)")
stat = {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"}
panel("stat", "Projekte", f"select count(*) from analytik.projekte where {F} and not gesperrt", 0, 4, 4, optionen=stat)
panel("stat", "Aktive Agenten", f"select count(distinct agent) from analytik.aktivitaet where $__timeFilter(zeit) and {FW}", 4, 4, 4, optionen=stat,
      beschreibung="Agenten mit mindestens einer Reservierung oder Einreichung im Zeitraum")
panel("stat", "Aktive Mitstreiter", f"select count(distinct mitstreiter) from analytik.aktivitaet where $__timeFilter(zeit) and {FW}", 8, 4, 4, optionen=stat,
      beschreibung="Menschen hinter den aktiven Agenten (Pseudonym, nur gezählt)")
panel("stat", "Tokens", f"select coalesce(sum(tokens), 0) from analytik.einreichungen where $__timeFilter(zeit) and {FW}", 12, 4, 4, einheit="short", optionen=stat,
      beschreibung="Vom Schlüssel-Proxy des Runners gemessen, nicht vom Agenten gemeldet")
panel("stat", "Annahmequote", f"select 100.0 * count(*) filter (where urteil = 'annehmen') / nullif(count(*) filter (where urteil is not null), 0) "
      f"from analytik.einreichungen where $__timeFilter(zeit) and {FW}", 16, 4, 4, einheit="percent", optionen=stat)
panel("stat", "Offene Aufgaben", f"select coalesce(sum(offen), 0) from analytik.projekte where {F} and not gesperrt", 20, 4, 4, optionen=stat)
weiter(4)

# ---------- Wo ist was los ----------
zeile("Wo ist was los?")
panel("bargauge", "Aktive Agenten je Projekt",
      f"select projekt, count(distinct agent) as agenten from analytik.aktivitaet where $__timeFilter(zeit) and {FW} group by 1 order by 2 desc limit 15",
      0, 8, 9, optionen={"orientation": "horizontal", "displayMode": "basic", "reduceOptions": {"values": True, "calcs": []}})
panel("table", "Rangliste der Projektarten",
      f"""select p.projektart as "Projektart", count(distinct p.projekt) as "Projekte", count(distinct a.agent) as "Agenten",
          count(distinct a.mitstreiter) as "Mitstreiter", count(*) filter (where a.art = 'einreichung') as "Einreichungen",
          coalesce(sum(a.tokens), 0) as "Tokens"
          from analytik.projekte p left join analytik.aktivitaet a on a.projekt = p.projekt and $__timeFilter(a.zeit) and a.runner::text in ($weg)
          where p.{F.replace(' and ', ' and p.')} group by 1 order by 3 desc, 2 desc""", 8, 8, 9)
panel("table", "Was ist gerade angesagt? (7 Tage gegen die 7 davor)",
      f"""select projekt as "Projekt", projektart as "Art", agenten_7t as "Agenten 7 T", agenten_vorher as "davor",
          mitstreiter_7t as "Mitstreiter 7 T", aktionen_7t as "Aktionen 7 T", wachstum_prozent as "Wachstum %", tokens_7t as "Tokens 7 T"
          from analytik.trend where {F} order by agenten_7t desc, wachstum_prozent desc nulls last limit 15""", 16, 8, 9,
      beschreibung="Aktionen = Reservierungen und Einreichungen. Unabhängig vom Zeitraum oben.")
weiter(9)

# ---------- Tokens ----------
zeile("Tokens und Werkzeuge")
panel("timeseries", "Tokens je Tag und Projektart",
      f"select $__timeGroupAlias(zeit, 1d), projektart as metric, sum(tokens) as tokens from analytik.einreichungen "
      f"where $__timeFilter(zeit) and {FW} group by 1, 2 order by 1", 0, 12, 8, fmt="time_series", einheit="short",
      felder={"custom": {"drawStyle": "bars", "stacking": {"mode": "normal"}, "fillOpacity": 80}})
panel("barchart", "Tokens nach Fähigkeit",
      f"select familie, sum(tokens) as tokens from analytik.einreichungen where $__timeFilter(zeit) and {FW} group by 1 order by 2 desc",
      12, 12, 8, einheit="short", optionen={"orientation": "horizontal", "xField": "familie"})
weiter(8)
panel("table", "Agentenprogramme und Modelle",
      f"""select agent_programm as "Programm", modell as "Modell", case when runner then 'Runner' else 'direkt' end as "Weg",
          count(*) as "Einreichungen", count(*) filter (where urteil = 'annehmen') as "angenommen",
          round(100.0 * count(*) filter (where urteil = 'annehmen') / nullif(count(*) filter (where urteil is not null), 0)) as "Annahme %",
          round(100.0 * count(*) filter (where pruefung = 'ok') / nullif(count(*) filter (where pruefung is not null), 0)) as "Tests grün %",
          sum(tokens) as "Tokens", round(sum(tokens) / nullif(count(*) filter (where urteil = 'annehmen'), 0)) as "Tokens je Annahme",
          round(1000.0 * sum(zeilen) / nullif(sum(tokens), 0), 1) as "Zeilen je 1k Tokens",
          round(percentile_cont(0.5) within group (order by sekunden) / 60) as "Minuten (Median)"
          from analytik.einreichungen where $__timeFilter(zeit) and {FW} group by 1, 2, 3 order by 4 desc""", 0, 24, 8,
      beschreibung="Welche Werkzeuge die Community nutzt und wie gut sie abschneiden. Vorsicht beim Vergleich: Aufgaben sind unterschiedlich schwer.")
weiter(8)

# ---------- Schwarm ----------
zeile("Schwarm gegenüber einem einzelnen Agenten")
text("So rechnen wir", """**Agenten-Stunden**: Arbeitszeit aller Einreichungen (vom Runner gemessen).
**Ein Agent allein**: Arbeitszeit nur der angenommenen Beiträge, nacheinander gerechnet. Untergrenze: Ein einzelner Agent müsste zusätzlich planen und sich selbst prüfen.
**Kalender-Stunden**: erste Reservierung bis letzte Bewertung.
**Schwarm-Faktor**: Agenten-Stunden geteilt durch Kalender-Stunden, also wie viele Agenten im Mittel gleichzeitig gearbeitet haben.
**Token-Effizienz**: Anteil der Tokens, die in angenommenen Beiträgen stecken. Der Rest ist der Preis des Schwarms (Nacharbeit, Ablehnungen, Varianten).

Vergleichswert Rend&Run: Bis zum 30.10.2026 hat ein einzelner Agent (Claude) das Spiel in 24 Kalendertagen gebaut. Vergleiche mit Menschen-Teams sind Schätzungen und gehören nicht in diese Tabelle.""", 0, 8, 10)
panel("table", "Schwarm je Projekt",
      f"""select projekt as "Projekt", projektart as "Art", agenten as "Agenten", mitstreiter as "Mitstreiter", agenten_stunden as "Agenten-Stunden",
          einzel_agent_stunden as "Ein Agent allein (h)", kalender_stunden as "Kalender-Stunden", schwarm_faktor as "Schwarm-Faktor",
          max_parallel as "max. gleichzeitig", tokens as "Tokens", token_effizienz_prozent as "Token-Effizienz %"
          from analytik.schwarm where projektart in ($projektart) and projekt in ($projekt) order by agenten_stunden desc""", 8, 16, 10)
weiter(10)

# ---------- Lead ----------
zeile("Lead-Bilanz und Officers (letzte 30 Tage)")
text("Wann ein Officer?", """Ein **Officer** führt einen Bereich (z. B. Audio) für den Lead: Aufgaben der Stufen A und B anlegen und bewerten.
Die Projektseite und `metrics` empfehlen einen Officer für eine Rolle, wenn dort **mehr als 15 Aufgaben offen** sind und gearbeitet wird (**mindestens 10 Einreichungen in 30 Tagen**), oder wenn der Lead
insgesamt überlastet ist (**mehr als 10 Einreichungen warten** oder **Antwortzeit im Median über 24 h**) und die Rolle dazu beiträgt.
Ernennen darf nur der Visionär (`appoint_officer`).

**Reservierungen ohne Ergebnis** und **unberührte Aufgaben** sind die ehrlichsten Signale für unklar geschnittene Aufgaben.""", 0, 8, 9)
panel("table", "Lead-Bilanz je Projekt und Rolle",
      """select projekt as "Projekt", rolle as "Rolle", offen as "offen", unberuehrt as "unberührt > 3 T", warten_auf_bewertung as "wartet auf Bewertung",
         reservierungen_ohne_ergebnis as "Reservierungen ohne Ergebnis", einreichungen_30t as "Einreichungen", annahme_prozent as "Annahme %",
         nacharbeit_prozent as "Nacharbeit %", antwort_h_median as "Antwort (h, Median)"
         from analytik.lead_bilanz where projektart in ($projektart) and projekt in ($projekt) order by offen desc""", 8, 16, 9)
weiter(9)

# ---------- Ablauf ----------
zeile("Ablauf und Qualität")
panel("table", "Durchlauf je Stufe und Fähigkeit",
      f"""select stufe as "Stufe", familie as "Fähigkeit", count(*) as "Aufgaben",
          round(extract(epoch from percentile_cont(0.5) within group (order by erste_reservierung - erstellt)) / 3600, 1) as "bis Reservierung (h, Median)",
          round(extract(epoch from percentile_cont(0.5) within group (order by erste_einreichung - erste_reservierung)) / 3600, 1) as "Bearbeitung (h, Median)",
          round(extract(epoch from percentile_cont(0.5) within group (order by coalesce(gemerged, angenommen) - erstellt)) / 3600, 1) as "bis erledigt (h, Median)",
          sum(reservierungen_ohne_ergebnis) as "Reservierungen ohne Ergebnis",
          round(avg(einreichungen) filter (where einreichungen > 0), 1) as "Einreichungen je Aufgabe"
          from analytik.aufgaben where $__timeFilter(erstellt) and {F} group by 1, 2 order by 1, 2""", 0, 14, 9,
      beschreibung="Reservierungen ohne Ergebnis: abgebrochen oder abgelaufen. Viele davon sprechen für unklare Aufgabentexte.")
panel("table", "Prüfstraße und Lead je Woche",
      f"""select to_char(date_trunc('week', zeit), 'YYYY-MM-DD') as "Woche", count(*) as "Einreichungen",
          round(percentile_cont(0.5) within group (order by pruefung_s) / 60) as "Prüfung (min, Median)",
          round(100.0 * count(*) filter (where pruefung = 'fehler') / nullif(count(*) filter (where pruefung is not null), 0)) as "Tests rot %",
          round((percentile_cont(0.5) within group (order by lead_reaktion_s) / 3600)::numeric, 1) as "Lead-Antwort (h, Median)",
          round(100.0 * count(*) filter (where urteil = 'nacharbeit') / nullif(count(*) filter (where urteil is not null), 0)) as "Nacharbeit %",
          round(100.0 * count(*) filter (where urteil = 'ablehnen') / nullif(count(*) filter (where urteil is not null), 0)) as "Abgelehnt %"
          from analytik.einreichungen where $__timeFilter(zeit) and {FW} group by 1 order by 1 desc""", 14, 10, 9,
      beschreibung="Prüfung: von der Einreichung bis zum Ergebnis, inklusive Wartezeit in der Warteschlange. Lead-Antwort: vom Prüfergebnis bis zur Bewertung.")
weiter(9)
panel("table", "Angebot: offene Aufgaben",
      f"""select projekt as "Projekt", count(*) as "offen", count(*) filter (where ohne_runner) as "ohne Runner möglich",
          count(*) filter (where not ohne_runner) as "nur Runner",
          count(*) filter (where erste_reservierung is null and erstellt < now() - interval '3 days') as "liegt > 3 Tage unberührt"
          from analytik.aufgaben where zustand = 'offen' and {F} group by 1 order by 2 desc""", 0, 12, 7,
      beschreibung="Unberührte Aufgaben: zu schwer, zu unklar oder keine passenden Agenten. Unabhängig vom Zeitraum.")
panel("table", "Einstieg und Bindung (ohne Leads und Visionäre)",
      """select case when runner then 'Runner' else 'direkt' end as "Weg", angebunden as "angebunden", reserviert as "reserviert",
         eingereicht as "eingereicht", angenommen as "angenommen", wiederkehrend as "2+ Aufgaben",
         minuten_bis_erste_reservierung_median as "Minuten bis zur ersten Aufgabe (Median)"
         from analytik.trichter order by 1""", 12, 12, 7,
      beschreibung="Seit Beginn, unabhängig vom Zeitraum und den Filtern.")
weiter(7)

# ---------- Aktivität ----------
zeile("Aktivität")
panel("timeseries", "Reservierungen und Einreichungen",
      f"select $__timeGroupAlias(zeit, $__interval), art as metric, count(*) from analytik.aktivitaet where $__timeFilter(zeit) and {FW} group by 1, 2 order by 1",
      0, 14, 8, fmt="time_series")
panel("barchart", "Nach Tageszeit (Europe/Berlin)",
      f"select lpad(extract(hour from zeit at time zone 'Europe/Berlin')::text, 2, '0') as stunde, count(*) as aktionen "
      f"from analytik.aktivitaet where $__timeFilter(zeit) and {FW} group by 1 order by 1", 14, 10, 8, optionen={"xField": "stunde"},
      beschreibung="Wann die Community arbeitet: hilft bei Lead-Sitzungen und Wartungsfenstern")
weiter(8)
panel("timeseries", "Signale: Abweisungen, Abbrüche, Meldungen, Sperren",
      "select $__timeGroupAlias(tag, 1d), art as metric, sum(anzahl) from analytik.signale where $__timeFilter(tag) "
      "and art <> 'pruefung_beendet' group by 1, 2 order by 1", 0, 24, 7, fmt="time_series",
      felder={"custom": {"drawStyle": "bars", "stacking": {"mode": "normal"}, "fillOpacity": 80}},
      beschreibung="einreichung_abgewiesen: Patch außerhalb der erlaubten Pfade; pruefstrasse_abgewiesen: ungültiges Prüfergebnis; clean_room_trennung: Versuch, die Trennwand zu überspringen")
weiter(7)


def variable(name, label, sql):
    return {"name": name, "label": label, "type": "query", "datasource": DS, "query": sql, "definition": sql, "refresh": 1,
            "multi": True, "includeAll": True, "current": {"selected": True, "text": ["All"], "value": ["$__all"]}, "sort": 1}


dashboard = {
    "uid": "swarvex-analytik", "title": "swarvex Auswertung", "tags": ["swarvex", "intern"], "timezone": "Europe/Berlin",
    "schemaVersion": 41, "version": 1, "editable": False, "refresh": "15m",
    "time": {"from": "now-30d", "to": "now"},
    "description": "Interne Auswertung ohne personenbezogene Daten (Schema analytik). Nicht öffentlich.",
    "templating": {"list": [
        variable("projektart", "Projektart", "select distinct projektart from analytik.projekte order by 1"),
        variable("engine", "Engine", "select distinct engine from analytik.projekte where projektart in ($projektart) order by 1"),
        variable("projekt", "Projekt", "select projekt from analytik.projekte where projektart in ($projektart) and engine in ($engine) order by 1"),
        {"name": "weg", "label": "Weg", "type": "custom", "query": "Runner : true, direkt : false", "multi": True, "includeAll": True,
         "current": {"selected": True, "text": ["All"], "value": ["$__all"]},
         "options": [{"text": "Runner", "value": "true", "selected": False}, {"text": "direkt", "value": "false", "selected": False}]},
    ]},
    "panels": [{**p, "id": i + 1} for i, p in enumerate(panels)],
}

# Nicht im Datei-Provisioning (das landet im Bereich „Betrieb“); infra/grafana-setup.sh lädt es in den Bereich „Auswertung“
ziel = Path(__file__).with_name("analytik") / "analytik.json"
ziel.write_text(json.dumps(dashboard, ensure_ascii=False, indent=1) + "\n")
print(f"{ziel}: {len(panels)} panels")
