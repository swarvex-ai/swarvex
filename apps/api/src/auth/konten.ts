// Konto-Anbieter für den Login der Menschen (E-06, Issue #32): GitHub, Codeberg und beliebige
// Gitea-/Forgejo-Instanzen. Codeberg läuft auf Forgejo und spricht dieselbe Schnittstelle wie Gitea.
// Welche Anbieter aktiv sind, steht nur in der Konfiguration (Labor = Live):
//   GITHUB_CLIENT_ID/GITHUB_CLIENT_SECRET
//   CODEBERG_CLIENT_ID/CODEBERG_CLIENT_SECRET   (CODEBERG_URL, Standard https://codeberg.org)
//   GITEA_URL/GITEA_CLIENT_ID/GITEA_CLIENT_SECRET
// Rücksprung-Adresse bei jedem Anbieter: <BASIS_URL>/interaction/callback

export type KontoNutzer = { id: string; login: string; erstellt: Date | null };

export type KontoAnbieter = {
  name: string;
  titel: string;
  /** Adresse, an die der Browser zur Anmeldung geschickt wird. */
  anmelden(rueckkehr: string, state: string): string;
  /** Code gegen Token tauschen und das Konto lesen. */
  nutzer(code: string, rueckkehr: string): Promise<KontoNutzer>;
};

async function jsonHolen<T>(url: string, init: RequestInit): Promise<T> {
  const r = await fetch(url, { ...init, signal: AbortSignal.timeout(10_000) });
  if (!r.ok) throw new Error(`${new URL(url).host}: HTTP ${r.status}`);
  return r.json() as Promise<T>;
}

function giteaArtig(name: string, titel: string, basis: string, clientId: string, geheimnis: string): KontoAnbieter {
  const url = basis.replace(/\/$/, "");
  return {
    name, titel,
    anmelden(rueckkehr, state) {
      const z = new URL(`${url}/login/oauth/authorize`);
      z.search = new URLSearchParams({ client_id: clientId, redirect_uri: rueckkehr, response_type: "code", scope: "read:user", state }).toString();
      return z.toString();
    },
    async nutzer(code, rueckkehr) {
      const t = await jsonHolen<{ access_token?: string }>(`${url}/login/oauth/access_token`, {
        method: "POST", headers: { "content-type": "application/json", accept: "application/json" },
        body: JSON.stringify({ client_id: clientId, client_secret: geheimnis, code, grant_type: "authorization_code", redirect_uri: rueckkehr }),
      });
      if (!t.access_token) throw new Error(`${titel}: kein Token`);
      const n = await jsonHolen<{ id: number; login: string; created?: string }>(`${url}/api/v1/user`, { headers: { authorization: `token ${t.access_token}` } });
      return { id: String(n.id), login: n.login, erstellt: n.created ? new Date(n.created) : null };
    },
  };
}

function github(clientId: string, geheimnis: string): KontoAnbieter {
  return {
    name: "github", titel: "GitHub",
    anmelden(rueckkehr, state) {
      const z = new URL("https://github.com/login/oauth/authorize");
      // Ohne Scope: nur öffentliche Profildaten, keine Rechte an Repos.
      z.search = new URLSearchParams({ client_id: clientId, redirect_uri: rueckkehr, state, allow_signup: "true" }).toString();
      return z.toString();
    },
    async nutzer(code, rueckkehr) {
      const t = await jsonHolen<{ access_token?: string }>("https://github.com/login/oauth/access_token", {
        method: "POST", headers: { "content-type": "application/json", accept: "application/json" },
        body: JSON.stringify({ client_id: clientId, client_secret: geheimnis, code, redirect_uri: rueckkehr }),
      });
      if (!t.access_token) throw new Error("GitHub: kein Token");
      const n = await jsonHolen<{ id: number; login: string; created_at?: string }>("https://api.github.com/user", {
        headers: { authorization: `Bearer ${t.access_token}`, accept: "application/vnd.github+json", "user-agent": "swarvex" },
      });
      return { id: String(n.id), login: n.login, erstellt: n.created_at ? new Date(n.created_at) : null };
    },
  };
}

/** Aktive Anbieter in fester Reihenfolge (GitHub, Codeberg, Gitea). */
export function kontoAnbieterLaden(env: NodeJS.ProcessEnv = process.env): KontoAnbieter[] {
  const liste: KontoAnbieter[] = [];
  if (env.GITHUB_CLIENT_ID && env.GITHUB_CLIENT_SECRET) liste.push(github(env.GITHUB_CLIENT_ID, env.GITHUB_CLIENT_SECRET));
  if (env.CODEBERG_CLIENT_ID && env.CODEBERG_CLIENT_SECRET) {
    liste.push(giteaArtig("codeberg", "Codeberg", env.CODEBERG_URL ?? "https://codeberg.org", env.CODEBERG_CLIENT_ID, env.CODEBERG_CLIENT_SECRET));
  }
  if (env.GITEA_URL && env.GITEA_CLIENT_ID && env.GITEA_CLIENT_SECRET) {
    liste.push(giteaArtig("gitea", env.GITEA_TITEL ?? "Gitea", env.GITEA_URL, env.GITEA_CLIENT_ID, env.GITEA_CLIENT_SECRET));
  }
  if (!liste.length) throw new Error("Kein Konto-Anbieter konfiguriert (GITHUB_*, CODEBERG_* oder GITEA_*)");
  return liste;
}
