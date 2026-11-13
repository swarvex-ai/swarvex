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
  // Schlüssel des eigenen Prüfservers: prüfen und Vorschauen bauen, nicht mergen (pruefstrasse.ts)
  pruefServerGeheimnis: process.env.PIPELINE_SECRET_CHECK ?? "",
  /** Eigene Adresse der Projekt-Vorschauen (infra/stack/vorschau.caddy), leer = keine Vorschauen. */
  vorschau: /^https:\/\//.test(process.env.VORSCHAU_ADRESSE ?? "") ? process.env.VORSCHAU_ADRESSE!.replace(/\/$/, "") : "",
  /** Eigene Domain der Vorschauen, je Projekt eine Subdomain (infra/stack/vorschau-domain.caddy); hat Vorrang vor vorschau. */
  vorschauDomain: /^[a-z0-9-]+(\.[a-z0-9-]+)+$/.test(process.env.VORSCHAU_DOMAIN ?? "") ? process.env.VORSCHAU_DOMAIN! : "",
};

/** Vorschauen teilen keinen Cookie-Bereich mit der Plattform (Sicherheitsaudit 12.11.2026): Cookies kennen keine Ports,
 *  und eine Subdomain kann Cookies für die übergeordnete Domain setzen, die alle Geschwister bekommen. Ohne Public
 *  Suffix List gelten Namen mit denselben letzten zwei Labels als geteilt. Warnt beim Start (das Labor nutzt bewusst
 *  dieselbe Adresse mit Port 8444); doppelte Sitzungs-Cookies weist konto.ts ab. */
export function vorschauGetrennt(vorschauHost: string, basis: string) {
  const ohnePunkt = (h: string) => h.toLowerCase().replace(/\.$/, "");
  const v = ohnePunkt(vorschauHost), b = ohnePunkt(new URL(basis).hostname);
  const istIp = (h: string) => /^\d{1,3}(\.\d{1,3}){3}$/.test(h) || h.startsWith("[");
  const letzteZwei = (h: string) => h.split(".").slice(-2).join(".");
  if (v === b || v.endsWith(`.${b}`) || b.endsWith(`.${v}`) || (!istIp(v) && !istIp(b) && letzteZwei(v) === letzteZwei(b)))
    console.error(`WARNING: previews (${v}) share a host or a parent domain with BASE_URL (${b}). Cookies ignore ports and reach subdomains, so preview code can set cookies the platform reads; use a separate host or domain for previews (docs/self-hosting.md).`);
}
if (konfig.vorschau) vorschauGetrennt(new URL(konfig.vorschau).hostname, konfig.basis);
if (konfig.vorschauDomain) vorschauGetrennt(konfig.vorschauDomain, konfig.basis);

export const RESSOURCE = () => `${konfig.basis}/mcp`;

/** Adresse der Vorschau einer Einreichung vor der Bewertung (review.<Domain>/<id>/), leer ohne eingerichtete Vorschauen. */
export const einreichungsVorschauUrl = (id: number) =>
  konfig.vorschauDomain ? `https://review.${konfig.vorschauDomain}/${id}/` : konfig.vorschau ? `${konfig.vorschau}/review/${id}/` : "";

/** Adresse der Vorschau eines Projekts, leer ohne eingerichtete Vorschauen. */
export const vorschauUrl = (slug: string) =>
  konfig.vorschauDomain ? `https://${slug}.${konfig.vorschauDomain}/` : konfig.vorschau ? `${konfig.vorschau}/${slug}/` : "";
