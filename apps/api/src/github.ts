// Repo-Zugriff über die swarvex GitHub App statt fester Deploy-Keys (die GitHub nur einmal je Repo zulässt).
// Der Visionär installiert die App auf seinem Repo; die Plattform holt sich je Zugriff einen Token, der
// eine Stunde gilt und nur dieses eine Repo betrifft: lesend für Spiegel und Prüfstraße, schreibend nur für den Merge-Bot.
// Konfiguration: GITHUB_APP_ID, GITHUB_APP_NAME (aus github.com/apps/<name>), Schlüssel unter GITHUB_APP_KEY_FILE.
import { createSign } from "node:crypto";
import { readFileSync } from "node:fs";

const API = "https://api.github.com";
const ID = process.env.GITHUB_APP_ID ?? "";
const NAME = process.env.GITHUB_APP_NAME ?? "";
const SCHLUESSEL = process.env.GITHUB_APP_KEY_FILE ?? "/data/github-app/key.pem";

let schluessel: string | null | undefined;
function privat(): string | null {
  if (schluessel === undefined) { try { schluessel = readFileSync(SCHLUESSEL, "utf8"); } catch { schluessel = null; } }
  return schluessel;
}

/** Ist die App eingerichtet? Ohne sie bleibt nur der Weg über das swarvex-Team. */
export const appBereit = () => !!(ID && NAME && privat());
export const installierenUrl = (zustand: string) => `https://github.com/apps/${NAME}/installations/new?state=${encodeURIComponent(zustand)}`;

const b64url = (x: string | Buffer) => Buffer.from(x).toString("base64url");
function appJwt(): string {
  const jetzt = Math.floor(Date.now() / 1000);
  const kopf = b64url(JSON.stringify({ alg: "RS256", typ: "JWT" }));
  const inhalt = b64url(JSON.stringify({ iat: jetzt - 60, exp: jetzt + 540, iss: ID }));
  const sig = createSign("RSA-SHA256").update(`${kopf}.${inhalt}`).sign(privat()!, "base64url");
  return `${kopf}.${inhalt}.${sig}`;
}

async function gh(pfad: string, auth: string, init: RequestInit = {}) {
  const r = await fetch(API + pfad, { ...init, headers: { accept: "application/vnd.github+json", "x-github-api-version": "2022-11-28",
    "user-agent": "swarvex.ai", authorization: auth, ...(init.headers ?? {}) }, signal: AbortSignal.timeout(15_000) });
  return { status: r.status, daten: r.status === 204 ? null : await r.json().catch(() => null) };
}

/** owner/name aus einer GitHub-Adresse (https://github.com/owner/name[.git]). */
export function repoAus(adresse: string): { owner: string; repo: string } | null {
  const m = /^https:\/\/github\.com\/([\w.-]+)\/([\w.-]+?)(?:\.git)?\/?$/.exec(adresse.trim());
  return m ? { owner: m[1], repo: m[2] } : null;
}

const tokens = new Map<string, { token: string; bis: number }>();
/** Installations-Token für genau ein Repo, lesend oder schreibend (zwischengespeichert, bis 10 Minuten vor Ablauf). */
export async function repoToken(installation: number, adresse: string, schreiben = false): Promise<string> {
  const r = repoAus(adresse);
  if (!r || !appBereit()) throw new Error("GitHub App not configured or not a GitHub repository");
  const schl = `${installation}:${r.owner}/${r.repo}:${schreiben}`;
  const alt = tokens.get(schl);
  if (alt && alt.bis > Date.now() + 600_000) return alt.token;
  const t = await gh(`/app/installations/${installation}/access_tokens`, `Bearer ${appJwt()}`, { method: "POST",
    body: JSON.stringify({ repositories: [r.repo], permissions: { contents: schreiben ? "write" : "read", metadata: "read" } }) });
  if (t.status !== 201) throw new Error(`GitHub token for ${r.owner}/${r.repo}: HTTP ${t.status}`);
  tokens.set(schl, { token: t.daten.token, bis: Date.parse(t.daten.expires_at) });
  return t.daten.token;
}

/** Git-Umgebung, die einen Token als HTTP-Header mitgibt (nicht in der Adresse, nicht in der Konfiguration; auch für git-lfs). */
export const gitTokenUmgebung = (token: string) => ({ GIT_CONFIG_COUNT: "1", GIT_CONFIG_KEY_0: "http.https://github.com/.extraheader",
  GIT_CONFIG_VALUE_0: `AUTHORIZATION: basic ${Buffer.from(`x-access-token:${token}`).toString("base64")}` });

export type RepoPruefung = { installation: number; repo_url: string; repo_git: string } | { fehler: "not_github" | "app_not_installed" | "private" | "no_write_access" };

/** Ist die App auf dem Repo installiert, ist es öffentlich, und darf login dort schreiben? Sonst wird nichts verbunden. */
export async function repoPruefen(adresse: string, login: string): Promise<RepoPruefung> {
  const r = repoAus(adresse);
  if (!r) return { fehler: "not_github" };
  const inst = await gh(`/repos/${r.owner}/${r.repo}/installation`, `Bearer ${appJwt()}`);
  if (inst.status !== 200) return { fehler: "app_not_installed" };
  const installation = Number(inst.daten.id);
  const token = await repoToken(installation, adresse);
  const repo = await gh(`/repos/${r.owner}/${r.repo}`, `token ${token}`);
  if (repo.status !== 200) return { fehler: "app_not_installed" };
  if (repo.daten.private) return { fehler: "private" };
  const recht = await gh(`/repos/${r.owner}/${r.repo}/collaborators/${encodeURIComponent(login)}/permission`, `token ${token}`);
  if (recht.status !== 200 || !["admin", "maintain", "write"].includes(recht.daten.role_name ?? recht.daten.permission)) return { fehler: "no_write_access" };
  const name = repo.daten.full_name as string;
  return { installation, repo_url: `https://github.com/${name}`, repo_git: `https://github.com/${name}.git` };
}
