// External anchor for the swarvex event log (issue #31).
// Fetches the signed head hash, verifies the signature against /jwks, appends it to heads.jsonl
// and checks that all earlier anchored heads still have the same hash in the platform's chain.
// If anything fails, the script exits with code 1, the GitHub action turns red and every watcher sees it.
// Usage: BASE=https://swarvex.ai node anchor.mjs   (no dependencies, Node 20+)
import { createPublicKey, verify } from "node:crypto";
import { existsSync, readFileSync, appendFileSync } from "node:fs";

const BASIS = (process.env.BASE ?? "https://swarvex.ai").replace(/\/$/, "");
const DATEI = process.env.HEADS ?? "heads.jsonl";
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
  if (!jwk || kopfteil.alg !== "ES256") throw new Error(`key ${kopfteil.kid} not under /jwks`);
  const ok = verify("sha256", Buffer.from(`${h}.${p}`), { key: createPublicKey({ key: jwk, format: "jwk" }), dsaEncoding: "ieee-p1363" }, b64(sig));
  if (!ok) throw new Error("invalid signature");
  if (inhalt.iss !== BASIS || inhalt.seq !== kopf.seq || inhalt.hash !== kopf.hash) throw new Error("signed content does not match the head");
}

const verankert = existsSync(DATEI) ? readFileSync(DATEI, "utf8").split("\n").filter(Boolean).map((z) => JSON.parse(z)) : [];
const fehler = [];

// 1. Fetch, verify and append the new head
const kopf = await holen("/api/log/head");
await signaturPruefen(kopf);
const letzter = verankert.at(-1);
if (letzter && kopf.seq < letzter.seq) fehler.push(`head went backwards: ${kopf.seq} < ${letzter.seq}`);
else if (letzter && kopf.seq === letzter.seq && kopf.hash !== letzter.hash) fehler.push(`head ${kopf.seq} now has a different hash`);
else if (!letzter || kopf.seq > letzter.seq) {
  appendFileSync(DATEI, JSON.stringify({ seq: kopf.seq, hash: kopf.hash, time: kopf.time, jws: kopf.jws }) + "\n");
  console.log(`anchored: seq ${kopf.seq} ${kopf.hash}`);
} else console.log(`unchanged: seq ${kopf.seq}`);

// 2. Check all earlier heads against today's chain
for (let i = 0; i < verankert.length; i += 200) {
  const teil = verankert.slice(i, i + 200);
  const ist = await holen(`/api/log/hashes?seq=${teil.map((k) => k.seq).join(",")}`);
  for (const k of teil) if (ist[k.seq] !== k.hash) fehler.push(`seq ${k.seq}: anchored ${k.hash}, platform ${ist[k.seq]}`);
}
console.log(`${verankert.length} earlier heads checked`);
if (fehler.length) {
  console.error("LOG DEVIATES:\n" + fehler.join("\n"));
  process.exit(1);
}
