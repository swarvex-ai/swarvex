#!/bin/sh
# Schlüsselpaar für API-Schlüssel von Spendern (Zusage mit Schlüssel, apps/api/src/spendenschluessel.ts), einmal je Server.
# public.pem bekommt die API (verschlüsselt), private.pem nur der Container spendenwerk (entschlüsselt, Benutzer 1000).
# private.pem kommt nicht in die Sicherung (infra/backup.sh). Geht es verloren, enden die Zusagen mit Schlüssel und die
# Spender sagen neu zu; ein neues Paar entsteht beim nächsten Deploy von selbst.
set -eu
D=/opt/swarvex-data/spenden-schluessel
[ -s "$D/private.pem" ] && [ -s "$D/public.pem" ] && exit 0
mkdir -p "$D"
chmod 755 "$D"
umask 077
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 -out "$D/private.pem.tmp" 2>/dev/null
openssl pkey -in "$D/private.pem.tmp" -pubout -out "$D/public.pem.tmp"
chown 1000:1000 "$D/private.pem.tmp"
chmod 600 "$D/private.pem.tmp"
chmod 644 "$D/public.pem.tmp"
mv "$D/public.pem.tmp" "$D/public.pem"
mv "$D/private.pem.tmp" "$D/private.pem"
echo "donation key pair created in $D"
