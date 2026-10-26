// Externer Anker für das Ereignisprotokoll von swarvex (Issue #31).
// Holt den signierten Kopf-Hash ab, prüft die Signatur gegen /jwks, hängt ihn an koepfe.jsonl an
// und prüft, dass alle früher verankerten Köpfe in der Kette der Plattform noch denselben Hash haben.
// Schlägt etwas fehl, endet das Skript mit Code 1, die GitHub-Action wird rot und alle Beobachter sehen es.
// Aufruf: BASIS=https://swarvex.ai node anker.mjs   (ohne Abhängigkeiten, Node 20+)
import { createPublicKey, verify } from "node:crypto";
import { existsSync, readFileSync, appendFileSync } from "node:fs";

const BASIS = (process.env.BASIS ?? "https://swarvex.ai").replace(/\/$/, "");
const DATEI = process.env.KOEPFE ?? "koepfe.jsonl";
const holen = async (pfad) => {
  const r = await fetch(BASIS + pfad);
  if (!r.ok) throw new Error(`${pfad}: HTTP ${r.status}`);
  return r.json();
};
const b64 = (s) => Buffer.from(s, "base64url");

async function signaturPruefen(kopf) {
  const [h, p, sig] = kopf.jws.split(".");
  const kopfteil = JSON.parse(b64(h).toString());
  const inhalt = JSON.parse(b64(p).toString());
  const jwk = (await holen("/jwks")).keys.find((k) => k.kid === kopfteil.kid);
  if (!jwk || kopfteil.alg !== "ES256") throw new Error(`Schlüssel ${kopfteil.kid} nicht unter /jwks`);
  const ok = verify("sha256", Buffer.from(`${h}.${p}`), { key: createPublicKey({ key: jwk, format: "jwk" }), dsaEncoding: "ieee-p1363" }, b64(sig));
  if (!ok) throw new Error("Signatur ungültig");
  if (inhalt.iss !== BASIS || inhalt.seq !== kopf.seq || inhalt.hash !== kopf.hash) throw new Error("Signierter Inhalt passt nicht zum Kopf");
}

const verankert = existsSync(DATEI) ? readFileSync(DATEI, "utf8").split("\n").filter(Boolean).map((z) => JSON.parse(z)) : [];
const fehler = [];

// 1. Neuen Kopf holen, prüfen und anhängen
const kopf = await holen("/api/protokoll/kopf");
await signaturPruefen(kopf);
const letzter = verankert.at(-1);
if (letzter && kopf.seq < letzter.seq) fehler.push(`Kopf ging zurück: ${kopf.seq} < ${letzter.seq}`);
else if (letzter && kopf.seq === letzter.seq && kopf.hash !== letzter.hash) fehler.push(`Kopf ${kopf.seq} hat jetzt einen anderen Hash`);
else if (!letzter || kopf.seq > letzter.seq) {
  appendFileSync(DATEI, JSON.stringify({ seq: kopf.seq, hash: kopf.hash, zeit: kopf.zeit, jws: kopf.jws }) + "\n");
  console.log(`verankert: seq ${kopf.seq} ${kopf.hash}`);
} else console.log(`unverändert: seq ${kopf.seq}`);

// 2. Alle früheren Köpfe gegen die heutige Kette prüfen
for (let i = 0; i < verankert.length; i += 200) {
  const teil = verankert.slice(i, i + 200);
  const ist = await holen(`/api/protokoll/hashes?seq=${teil.map((k) => k.seq).join(",")}`);
  for (const k of teil) if (ist[k.seq] !== k.hash) fehler.push(`seq ${k.seq}: verankert ${k.hash}, Plattform ${ist[k.seq]}`);
}
console.log(`${verankert.length} frühere Köpfe geprüft`);
if (fehler.length) {
  console.error("PROTOKOLL WEICHT AB:\n" + fehler.join("\n"));
  process.exit(1);
}
