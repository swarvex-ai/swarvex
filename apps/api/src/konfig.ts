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
  /** Ausnahmen vom Mindestalter, je Eintrag „anbieter:konto-id“ (unveränderliche ID, nicht der Name), z. B. Testkonten. */
  kontoAlterAusnahmen: new Set((process.env.ACCOUNT_AGE_EXEMPT ?? "").split(",").map((s) => s.trim()).filter(Boolean)),
  /** Gemeinsames Geheimnis der Prüfstraße (HMAC über Prüfergebnisse, P5). */
  pruefstrasseGeheimnis: process.env.PIPELINE_SECRET ?? "",
  /** Eigene Adresse der Projekt-Vorschauen (infra/stack/vorschau.caddy), leer = keine Vorschauen. */
  vorschau: /^https:\/\//.test(process.env.VORSCHAU_ADRESSE ?? "") ? process.env.VORSCHAU_ADRESSE!.replace(/\/$/, "") : "",
  /** Eigene Domain der Vorschauen, je Projekt eine Subdomain (infra/stack/vorschau-domain.caddy); hat Vorrang vor vorschau. */
  vorschauDomain: /^[a-z0-9-]+(\.[a-z0-9-]+)+$/.test(process.env.VORSCHAU_DOMAIN ?? "") ? process.env.VORSCHAU_DOMAIN! : "",
};

export const RESSOURCE = () => `${konfig.basis}/mcp`;

/** Adresse der Vorschau einer Einreichung vor der Bewertung (review.<Domain>/<id>/), leer ohne eingerichtete Vorschauen. */
export const einreichungsVorschauUrl = (id: number) =>
  konfig.vorschauDomain ? `https://review.${konfig.vorschauDomain}/${id}/` : konfig.vorschau ? `${konfig.vorschau}/review/${id}/` : "";

/** Adresse der Vorschau eines Projekts, leer ohne eingerichtete Vorschauen. */
export const vorschauUrl = (slug: string) =>
  konfig.vorschauDomain ? `https://${slug}.${konfig.vorschauDomain}/` : konfig.vorschau ? `${konfig.vorschau}/${slug}/` : "";
