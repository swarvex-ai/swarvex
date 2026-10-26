// Kennzahlen des öffentlichen Quellcode-Repos (Sterne, Forks) für die Website.
// Abgefragt wird GitHub nur von unserem Server und höchstens einmal pro Stunde; der Browser der
// Besucher lädt nichts von GitHub (Datenschutz: keine fremden Server).
const REPO = process.env.SOURCE_REPO ?? "swarvex-ai/swarvex";
const GUELTIG_MS = 60 * 60_000;

type RepoDaten = { url: string; stars: number | null; forks: number | null };
let zwischenspeicher: { daten: RepoDaten; zeit: number } | null = null;

export async function repoDaten(): Promise<RepoDaten> {
  if (zwischenspeicher && Date.now() - zwischenspeicher.zeit < GUELTIG_MS) return zwischenspeicher.daten;
  const url = `https://github.com/${REPO}`;
  try {
    const r = await fetch(`https://api.github.com/repos/${REPO}`, {
      headers: { accept: "application/vnd.github+json", "user-agent": "swarvex.ai" }, signal: AbortSignal.timeout(5_000),
    });
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    const j = await r.json() as { stargazers_count: number; forks_count: number; html_url: string };
    zwischenspeicher = { daten: { url: j.html_url, stars: j.stargazers_count, forks: j.forks_count }, zeit: Date.now() };
  } catch (e) {
    console.error("repo:", (e as Error).message);
    // Bei Fehlern die letzten bekannten Zahlen behalten und erst in 10 Minuten erneut fragen
    zwischenspeicher = { daten: zwischenspeicher?.daten ?? { url, stars: null, forks: null }, zeit: Date.now() - GUELTIG_MS + 10 * 60_000 };
  }
  return zwischenspeicher.daten;
}
