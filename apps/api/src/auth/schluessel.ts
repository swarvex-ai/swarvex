// Signaturschlüssel (ES256) und Cookie-Schlüssel: beim ersten Start erzeugt, danach aus der DB.
import { randomBytes } from "node:crypto";
import { exportJWK, generateKeyPair } from "jose";
import type { Db } from "../db.ts";

export type Schluessel = { signatur: Record<string, unknown>; cookies: string[] };

export async function schluesselLaden(db: Db): Promise<Schluessel> {
  const vorhanden = await db.query("select name, wert from schluessel");
  const map = new Map(vorhanden.rows.map((r) => [r.name, r.wert]));
  if (!map.has("signatur")) {
    const { privateKey } = await generateKeyPair("ES256", { extractable: true });
    const jwk = { ...(await exportJWK(privateKey)), alg: "ES256", use: "sig", kid: randomBytes(8).toString("hex") };
    await db.query("insert into schluessel(name, wert) values ('signatur', $1) on conflict do nothing", [jwk]);
  }
  if (!map.has("cookies")) {
    await db.query("insert into schluessel(name, wert) values ('cookies', $1) on conflict do nothing", [JSON.stringify([randomBytes(32).toString("base64url")])]);
  }
  const neu = new Map((await db.query("select name, wert from schluessel")).rows.map((r) => [r.name, r.wert]));
  return { signatur: neu.get("signatur"), cookies: neu.get("cookies") };
}

export function oeffentlich(jwk: Record<string, unknown>) {
  const { d, ...rest } = jwk as Record<string, unknown> & { d?: string };
  return rest;
}
