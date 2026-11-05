"""Tageswerte im gewählten Zeitraum, für die Generatoren (analytik-dashboard.py, sicherheit-dashboard.py).

Sichten mit einer Spalte „tag“ zählen je Tag; ein Tag steht dort als Mitternacht. $__timeFilter(tag) nähme einen Tag
nur, wenn seine Mitternacht im Zeitraum liegt: Bei „Last 6 hours“ wäre fast jedes Panel leer („No data“). Stattdessen
zählt jeder Tag, den der Zeitraum berührt.
  tag als date (Besuche): Kalendertag in Europe/Berlin (so schreibt ihn apps/api/src/besuche.ts)
  tag als timestamptz (date_trunc('day', …) in der Datenbank, also UTC)
"""

TAG_DATUM = ("tag between ($__timeFrom()::timestamptz at time zone 'Europe/Berlin')::date "
             "and ($__timeTo()::timestamptz at time zone 'Europe/Berlin')::date")
TAG_ZEIT = "tag >= date_trunc('day', $__timeFrom()::timestamptz) and tag <= $__timeTo()::timestamptz"


def tagesfilter(sql):
    return sql.replace("$__timeFilter(tag::timestamptz)", TAG_DATUM).replace("$__timeFilter(tag)", TAG_ZEIT)
