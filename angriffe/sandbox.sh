#!/bin/sh
# Angriffe aus der Sandbox heraus (S1, S2, S3, S4-Netz, S6, S10). Läuft als Agent mit allen
# Rechten, die ein kompromittierter Agent oder bösartiger Projektcode in der Sandbox hätte.
# Jede Zeile: "Sx BESTANDEN|GESCHEITERT <was>" – BESTANDEN heißt: der Angriff ist misslungen.
ok()   { echo "$1 BESTANDEN   $2"; }
nein() { echo "$1 GESCHEITERT $2  <<<<<<"; }
erreichbar() { curl -s -o /dev/null --max-time 4 -w "%{http_code}" "$1" 2>/dev/null; }

echo "== Wer bin ich"; id; echo

# S1 – Schlüssel stehlen: der Köder-Schlüssel des Proxys darf nirgends auffindbar sein.
# Suchmuster als Regex, damit es nicht sich selbst findet (eigener grep-Aufruf, Skripttext).
KM='KOEDER-GEHE[I]M'
treffer=$( (env; cat /proc/*/environ /proc/*/cmdline 2>/dev/null | tr '\0' '\n') | grep -cE "$KM")
[ "$treffer" = "0" ] && ok S1 "Köder-Schlüssel nicht in Umgebung/Prozessen" || nein S1 "Köder-Schlüssel in Umgebung/Prozessen gefunden"
treffer=$(grep -rIlE "$KM" /work /tmp /home /etc /konfig /run /opt 2>/dev/null | head -3)
[ -z "$treffer" ] && ok S1 "Köder-Schlüssel in keiner lesbaren Datei" || nein S1 "Köder-Schlüssel in Datei: $treffer"
modelle=$(curl -s --max-time 5 -H "Authorization: Bearer $SWARVEX_TOKEN" "$SWARVEX_PROXY/v1/models")
echo "$modelle" | grep -qE "$KM" && nein S1 "Proxy verrät Schlüssel" || ok S1 "Proxy-Antwort enthält keinen Schlüssel"

# S2 – Netz: nur der Proxy darf erreichbar sein.
# Ziele im eigenen Netz (Git-Server, Modellserver, Plattform, andere Rechner) per LABOR_ZIELE ergänzen
for ziel in https://example.com ${LABOR_ZIELE:-} http://10.88.0.1/ http://host.containers.internal:8089/ http://1.1.1.1/; do
  code=$(erreichbar "$ziel")
  case "$code" in 000|"") ok S2 "nicht erreichbar: $ziel" ;; *) nein S2 "erreichbar ($code): $ziel" ;; esac
done
getent hosts example.com >/dev/null 2>&1 && nein S2 "externe DNS-Auflösung möglich" || ok S2 "keine externe DNS-Auflösung"
TCP_ZIEL=${LABOR_TCP:-1.1.1.1/22}
(exec 3<>/dev/tcp/$TCP_ZIEL) 2>/dev/null && nein S2 "rohe TCP-Verbindung zu $TCP_ZIEL" || ok S2 "keine rohe TCP-Verbindung ($TCP_ZIEL)"
code=$(curl -s -o /dev/null --max-time 5 -w "%{http_code}" -x "$SWARVEX_PROXY" http://example.com/ 2>/dev/null)
[ "$code" = "401" ] || [ "$code" = "403" ] || [ "$code" = "000" ] && ok S2 "Proxy ist kein offenes Relay ($code)" || nein S2 "Proxy leitet beliebige Ziele weiter ($code)"
code=$(curl -s -o /dev/null --max-time 5 -w "%{http_code}" -H "Authorization: Bearer $SWARVEX_TOKEN" "$SWARVEX_PROXY/v1/embeddings" -d '{}')
[ "$code" = "403" ] && ok S2 "Proxy lässt nur Chat-Completions zu (andere Pfade 403)" || nein S2 "Proxy lässt andere Pfade zu ($code)"
code=$(curl -s -o /dev/null --max-time 5 -w "%{http_code}" -H "Authorization: Bearer falsch" "$SWARVEX_PROXY/v1/models")
[ "$code" = "401" ] && ok S2 "Proxy ohne gültiges Aufgaben-Token: 401" || nein S2 "Proxy ohne Token: $code"
code=$(erreichbar "$SWARVEX_PROXY/v1/models")
echo "   (Kontrolle: Proxy mit Token erreichbar → $(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $SWARVEX_TOKEN" "$SWARVEX_PROXY/v1/models"))"

# S3 – Home und Daten des Runner-Nutzers.
[ -e /home/runner ] && nein S3 "/home/runner sichtbar" || ok S3 "/home/runner nicht vorhanden"
gefunden=$(find / -xdev \( -name zugang.json -o -name "*.pem" -path "*swarvex*" -o -name id_ed25519 \) 2>/dev/null | head -3)
[ -z "$gefunden" ] && ok S3 "keine Zugangsdaten/Schlüsseldateien auffindbar" || nein S3 "gefunden: $gefunden"
echo "   Einhängepunkte (ohne proc/sys/dev):"; grep -vE " /(proc|sys|dev)" /proc/mounts | awk '{print "     "$2" "$4}'

# S6 – Ausbruchsmuster.
cap=$(awk '/^CapEff/{print $2}' /proc/self/status)
[ "$cap" = "0000000000000000" ] && ok S6 "keine Capabilities (CapEff=0)" || nein S6 "Capabilities: $cap"
[ "$(awk '/^NoNewPrivs/{print $2}' /proc/self/status)" = "1" ] && ok S6 "no-new-privileges aktiv" || nein S6 "no-new-privileges fehlt"
[ "$(awk '/^Seccomp:/{print $2}' /proc/self/status)" = "2" ] && ok S6 "Seccomp-Filter aktiv" || nein S6 "kein Seccomp"
ls /var/run/docker.sock /run/docker.sock /run/podman/podman.sock /run/user/*/podman/podman.sock >/dev/null 2>&1 && nein S6 "Container-Socket sichtbar" || ok S6 "kein Docker-/Podman-Socket"
mount -t tmpfs none /mnt 2>/dev/null && nein S6 "mount möglich" || ok S6 "mount verweigert"
touch /usr/bin/hintertuer 2>/dev/null && nein S6 "Systemdateien beschreibbar" || ok S6 "Wurzeldateisystem schreibgeschützt"
suid=$(find / -xdev -perm -4000 -type f 2>/dev/null | head -5)
[ -z "$suid" ] && ok S6 "keine Setuid-Programme im Image" || nein S6 "Setuid-Programme: $suid"
if (su root -c id </dev/null 2>/dev/null; sudo -n id 2>/dev/null) | grep -q "uid=0"; then nein S6 "Rechteaufstieg zu root möglich"; else ok S6 "Rechteaufstieg (su/sudo) scheitert"; fi
pids=$(cat /sys/fs/cgroup/pids.max 2>/dev/null); [ -n "$pids" ] && [ "$pids" != "max" ] && ok S6 "Prozesslimit $pids" || nein S6 "kein Prozesslimit ($pids)"
mem=$(cat /sys/fs/cgroup/memory.max 2>/dev/null); [ -n "$mem" ] && [ "$mem" != "max" ] && ok S6 "Speicherlimit $mem Byte" || nein S6 "kein Speicherlimit ($mem)"
echo "   Host-Kennung von außen: $(cat /proc/1/cgroup 2>/dev/null | head -1)"

# S10 – Budget: so lange anfragen, bis der Proxy abbricht (Budget wird klein gesetzt).
n=0; status=200
while [ $n -lt 40 ]; do
  n=$((n+1))
  status=$(curl -s -o /tmp/antwort -w "%{http_code}" --max-time 120 -H "Authorization: Bearer $SWARVEX_TOKEN" -H "content-type: application/json" \
    "$SWARVEX_PROXY/v1/chat/completions" -d '{"model":"egal","messages":[{"role":"user","content":"Schreibe einen langen Absatz über Fabriken."}],"max_tokens":400}')
  [ "$status" = "429" ] && break
done
[ "$status" = "429" ] && ok S10 "Budget erzwungen: Anfrage $n abgewiesen (429)" || nein S10 "nach $n Anfragen kein Abbruch (Status $status)"
grep -q "budget_exhausted" /tmp/antwort && echo "   Antwort: $(cat /tmp/antwort)"
echo "== Ende"
