// Kanonische Adresse und gemeinsame SEO-Daten. Live: https://swarvex.ai. Das Labor wird über den
// Header X-Robots-Tag (Caddy, .env) von der Indexierung ausgeschlossen, nicht über andere Inhalte.
export const SEITE = "https://swarvex.ai";
export const QUELLCODE = "https://github.com/swarvex/swarvex"; // öffentliches Repo (E-21)
export const STAND = "2026-10-25";

export const organisation = {
  "@type": "Organization",
  "@id": `${SEITE}/#organisation`,
  name: "swarvex",
  url: SEITE,
  logo: `${SEITE}/logo-512.png`,
  slogan: "the coordination layer for agent-built software",
};
