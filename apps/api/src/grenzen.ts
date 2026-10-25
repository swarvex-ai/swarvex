// Ratenlimits (Issue #46): Token-Eimer im Speicher, je Regel und Schlüssel (IP oder Agent).
// Die Plattform läuft als ein Prozess; bei mehreren Instanzen gehören die Eimer nach PostgreSQL.
import type { IncomingMessage } from "node:http";

type Regel = { anzahl: number; proSekunden: number };

/** Grenzen je Bereich. Großzügig für echte Agenten, eng für Registrierung und Anmeldung. */
export const REGELN = {
  registrierung: { anzahl: 20, proSekunden: 3600 },   // neue OAuth-Clients je IP
  anmeldung: { anzahl: 60, proSekunden: 60 },        // /interaction, /auth, /token, /device je IP
  mcp: { anzahl: 600, proSekunden: 60 },             // MCP-Aufrufe je Agent
  mcpIp: { anzahl: 1200, proSekunden: 60 },          // MCP-Aufrufe je IP (vor der Token-Prüfung)
  oeffentlich: { anzahl: 300, proSekunden: 60 },     // öffentliche Lese-API je IP
} satisfies Record<string, Regel>;

type Eimer = { inhalt: number; zuletzt: number };

export function grenzenErstellen(regeln: Record<string, Regel> = REGELN, jetzt: () => number = Date.now) {
  const eimer = new Map<string, Eimer>();
  let aufgeraeumt = jetzt();

  /** Verbraucht einen Platz; liefert 0 oder die Sekunden bis zum nächsten freien Platz. */
  return function pruefen(regel: keyof typeof regeln & string, schluessel: string): number {
    const r = regeln[regel];
    const t = jetzt();
    if (t - aufgeraeumt > 60_000) {  // volle Eimer vergessen, damit die Tabelle klein bleibt
      for (const [k, e] of eimer) if (t - e.zuletzt > 3_600_000) eimer.delete(k);
      aufgeraeumt = t;
    }
    const k = `${regel}:${schluessel}`;
    const e = eimer.get(k) ?? { inhalt: r.anzahl, zuletzt: t };
    e.inhalt = Math.min(r.anzahl, e.inhalt + ((t - e.zuletzt) / 1000) * (r.anzahl / r.proSekunden));
    e.zuletzt = t;
    eimer.set(k, e);
    if (e.inhalt >= 1) {
      e.inhalt -= 1;
      return 0;
    }
    return Math.ceil((1 - e.inhalt) / (r.anzahl / r.proSekunden));
  };
}

/** Adresse des Aufrufers: Caddy setzt X-Forwarded-For selbst (fremde Werte werden überschrieben). */
export function aufruferIp(req: IncomingMessage): string {
  const weitergeleitet = String(req.headers["x-forwarded-for"] ?? "").split(",")[0].trim();
  return weitergeleitet || req.socket.remoteAddress || "unbekannt";
}
