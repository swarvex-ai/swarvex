// Laufzeit-Konfiguration aus der Umgebung (infra/stack/.env).
function pflicht(name: string): string {
  const wert = process.env[name];
  if (!wert) throw new Error(`environment variable ${name} is missing`);
  return wert;
}

export const konfig = {
  datenbank: pflicht("DATABASE_URL"),
  /** Öffentliche Basis-URL, z. B. https://swarvex.ai – zugleich OAuth-Issuer. */
  basis: pflicht("BASE_URL").replace(/\/$/, ""),
  port: Number(process.env.PORT ?? 3000),
  /** Mindestalter des Kontos beim Konto-Anbieter in Tagen (Schutz vor Wegwerf-Konten; Labor 0, live z. B. 30). */
  kontoMindestalterTage: Number(process.env.ACCOUNT_MIN_AGE_DAYS ?? 0),
  /** Gemeinsames Geheimnis der Prüfstraße (HMAC über Prüfergebnisse, P5). */
  pruefstrasseGeheimnis: process.env.PIPELINE_SECRET ?? "",
  /** Eigene Adresse der Projekt-Vorschauen (infra/stack/Caddyfile), leer = keine Vorschauen. */
  vorschau: /^https:\/\//.test(process.env.VORSCHAU_ADRESSE ?? "") ? process.env.VORSCHAU_ADRESSE!.replace(/\/$/, "") : "",
};

export const RESSOURCE = () => `${konfig.basis}/mcp`;
