// Schutz vor Server-Side Request Forgery: Adressen, die die Plattform im Auftrag eines Clients
// abruft (Client-ID-Metadata-Dokumente), müssen ins öffentliche Internet zeigen.
import { lookup } from "node:dns/promises";
import { isIP } from "node:net";

const PRIVAT_V4 = [
  [0x00000000, 8], [0x0a000000, 8], [0x64400000, 10], [0x7f000000, 8], [0xa9fe0000, 16], [0xac100000, 12],
  [0xc0000000, 24], [0xc0000200, 24], [0xc0a80000, 16], [0xc6120000, 15], [0xc6336400, 24], [0xcb007100, 24],
  [0xe0000000, 4], [0xf0000000, 4],
] as const;

function privatV4(ip: string) {
  const n = ip.split(".").reduce((a, b) => (a << 8) + Number(b), 0) >>> 0;
  return PRIVAT_V4.some(([netz, bits]) => (n >>> (32 - bits)) === (netz >>> (32 - bits)));
}

function privatV6(ip: string) {
  const a = ip.toLowerCase();
  if (a === "::" || a === "::1") return true;
  const v4 = /^::ffff:(\d+\.\d+\.\d+\.\d+)$/.exec(a);
  if (v4) return privatV4(v4[1]);
  return /^(fc|fd|fe8|fe9|fea|feb|ff)/.test(a) || a.startsWith("64:ff9b:") || a.startsWith("2001:db8:");
}

export function privateAdresse(ip: string) {
  return isIP(ip) === 4 ? privatV4(ip) : privatV6(ip);
}

/** true, wenn der Hostname ausschließlich auf öffentliche Adressen zeigt. */
export async function oeffentlicheAdresse(host: string): Promise<boolean> {
  const name = host.replace(/^\[|\]$/g, "");
  try {
    const adressen = isIP(name) ? [{ address: name }] : await lookup(name, { all: true, verbatim: true });
    return adressen.length > 0 && adressen.every((a) => !privateAdresse(a.address));
  } catch {
    return false;
  }
}
