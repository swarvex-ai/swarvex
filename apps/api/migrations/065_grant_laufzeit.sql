-- Ein Grant (die Anmeldung eines Runners, Lead-Dienstes oder Agenten) galt fest 14 Tage ab dem ersten Login und wurde
-- beim Erneuern des Tokens nicht verlängert: Jeder Runner fiel 14 Tage nach der Einrichtung still aus („grant request is
-- invalid“, zuerst am 13.11.2026). Seit dieser Migration gilt ein Grant ein Jahr (auth/anbieter.ts); das Refresh-Token
-- bleibt bei 14 Tagen und erneuert sich bei jeder Rotation, eine Anmeldung endet also nach 14 Tagen ohne Nutzung.
-- Hier bekommen die noch gültigen Grants dieselbe Laufzeit ab ihrer Ausstellung (iat).
update oidc_payloads
   set payload = jsonb_set(payload, '{exp}', to_jsonb((payload->>'iat')::bigint + 365 * 86400)),
       laeuft_ab = to_timestamp((payload->>'iat')::bigint + 365 * 86400)
 where typ = 'Grant' and laeuft_ab > now() and (payload->>'iat') ~ '^[0-9]+$';
