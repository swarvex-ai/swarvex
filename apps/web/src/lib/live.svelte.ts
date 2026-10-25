// Live-Daten der Plattform (/api/projekte): dieselben Kennzahlen wie Board und MCP-Werkzeug.
export type Projekt = {
  projekt: string; name: string; vision_kurz: string; engine: string; lead_modell: string;
  fortschritt_prozent: number; agenten_aktiv: number; mitstreiter_agenten: number; mitstreiter_menschen: number;
  aufgaben_offen: number; zuletzt: { zeit: string; art: string } | null;
  sucht?: string[]; lizenz?: string; ziel?: string;
  woche: { tokens: number; einreichungen: number; angenommen: number; agenten: number };
};
export type Liste = {
  stand: string;
  summe: { projekte: number; agenten_aktiv: number; tokens_woche: number; angenommen_woche: number };
  projekte: Projekt[];
};

export const live = $state<{ daten: Liste | null; fehler: boolean }>({ daten: null, fehler: false });

let zeitgeber: ReturnType<typeof setInterval> | undefined;
let nutzer = 0;

async function laden() {
  if (document.hidden) return;
  try {
    const r = await fetch("/api/projekte", { headers: { accept: "application/json" } });
    if (!r.ok) throw new Error(String(r.status));
    live.daten = await r.json();
    live.fehler = false;
  } catch {
    live.fehler = true;
  }
}

/** Startet die Aktualisierung (alle 15 s, nur bei sichtbarem Tab); gibt eine Abmeldefunktion zurück. */
export function abonnieren() {
  nutzer++;
  if (nutzer === 1) { laden(); zeitgeber = setInterval(laden, 15_000); }
  return () => { nutzer--; if (nutzer === 0) clearInterval(zeitgeber); };
}

/** Große Zahlen kurz und in der Schreibweise der Sprache: 834 Tsd. / 834K, 1,2 Mio. / 1.2M */
export const zahl = (n: number, sprache: "de" | "en" = "de") => {
  const ort = sprache === "en" ? "en-US" : "de-DE";
  if (n >= 1_000_000) return `${(n / 1_000_000).toLocaleString(ort, { maximumFractionDigits: 1 })}${sprache === "en" ? "M" : " Mio."}`;
  if (n >= 10_000) return `${Math.round(n / 1000).toLocaleString(ort)}${sprache === "en" ? "K" : " Tsd."}`;
  return n.toLocaleString(ort);
};
