// OAuth-2.1-/OIDC-Server der Plattform (oidc-provider, panva).
// - Agenten melden sich selbst an (MCP-Authorization), auf beiden Wegen der Spezifikation:
//   Client-ID-Metadata-Dokument (Standard seit 2025-11-25: Client-ID ist eine HTTPS-URL) und
//   dynamische Client-Registrierung (RFC 7591, für ältere Clients).
// - PKCE Pflicht, Resource Indicators: Access-Tokens sind JWTs nur für die MCP-Ressource.
// - Device Flow: der Runner meldet sich mit einem Befehl + Bestätigung im Browser an.
// - Menschen melden sich über GitHub, Codeberg oder Gitea an (interaktion.ts); Konto = Mensch, Client = Agent.
import Provider, { errors } from "oidc-provider";
import type { Db } from "../db.ts";
import { konfig, RESSOURCE } from "../konfig.ts";
import { pgAdapter } from "./adapter.ts";
import type { Schluessel } from "./schluessel.ts";
import { oeffentlicheAdresse } from "./netz.ts";

const englisch = (ctx: { get(name: string): string }) => !/^de\b/i.test(ctx.get("accept-language") ?? "");
const seite = (titel: string, inhalt: string, en = true) => `<!doctype html><html lang="${en ? "en" : "de"}"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>${titel}</title>
<style>body{font:16px/1.5 system-ui,sans-serif;max-width:32rem;margin:3rem auto;padding:0 1rem;color:#1d2328}
input,button{font:inherit;padding:.5rem .75rem}code{background:#eef1f3;padding:.1rem .3rem}</style></head>
<body><h1>${titel}</h1>${inhalt}</body></html>`;

export function anbieterErstellen(db: Db, schluessel: Schluessel) {
  const anbieter = new Provider(konfig.basis, {
    adapter: pgAdapter(db),
    jwks: { keys: [schluessel.signatur as any] },
    cookies: { keys: schluessel.cookies },
    pkce: { required: () => true },
    // OAuth 2.1: nur Code-Flow (kein Implicit); ID-Tokens mit unserem ES256-Schlüssel.
    responseTypes: ["code"],
    clientDefaults: { id_token_signed_response_alg: "ES256", grant_types: ["authorization_code", "refresh_token"], response_types: ["code"] },
    scopes: ["openid", "offline_access", "mcp"],
    claims: { openid: ["sub"], profile: ["preferred_username"] },
    clientBasedCORS: () => true,
    // Lokale Agenten (nur Loopback-Rücksprung) sind „native“ Apps nach RFC 8252: Der Port darf dann
    // wechseln. Metadata-Dokumente wie das von Claude Code nennen den Typ nicht, deshalb hier gesetzt.
    extraClientMetadata: {
      properties: ["x_lokal"],
      validator: (_ctx, eigenschaft, _wert, metadaten: any) => {
        if (eigenschaft !== "x_lokal") return;
        const uris: string[] = metadaten.redirect_uris ?? [];
        const lokal = uris.length > 0 && uris.every((u) => {
          const r = URL.parse(u);
          return r?.protocol === "http:" && ["127.0.0.1", "localhost", "[::1]"].includes(r.hostname);
        });
        if (lokal) metadaten.application_type = "native";
      },
    },
    features: {
      devInteractions: { enabled: false },
      registration: { enabled: true },
      clientIdMetadataDocument: {
        enabled: true,
        ack: "draft-02",
        // Nur öffentliche Ziele abrufen: Eine Client-ID darf den Server nicht ins interne Netz schicken.
        allowFetch: async (_ctx, clientId) => oeffentlicheAdresse(new URL(clientId).hostname),
        // Rücksprung nur auf Loopback (lokale Agenten) oder HTTPS.
        allowClient: async (_ctx, client) => (client.redirectUris ?? []).every((u: string) => {
          const r = URL.parse(u);
          return !!r && (r.protocol === "https:" || (r.protocol === "http:" && ["127.0.0.1", "localhost", "[::1]"].includes(r.hostname)));
        }),
        cacheDuration: { min: 300, max: 86400 },
      },
      revocation: { enabled: true },
      resourceIndicators: {
        enabled: true,
        defaultResource: () => RESSOURCE(),
        useGrantedResource: () => true,
        getResourceServerInfo: (_ctx, ressource) => {
          if (ressource !== RESSOURCE()) throw new errors.InvalidTarget();
          return { scope: "mcp", audience: RESSOURCE(), accessTokenTTL: 3600, accessTokenFormat: "jwt", jwt: { sign: { alg: "ES256" } } };
        },
      },
      deviceFlow: {
        enabled: true,
        userCodeInputSource: async (ctx, form, _out, err) => {
          const en = englisch(ctx);
          ctx.body = en
            ? seite("Sign in a device", `${err ? "<p><strong>The code is invalid or has expired.</strong></p>" : ""}
              <p>Enter the code your runner shows.</p>${form}<button type="submit" form="op.deviceInputForm">Continue</button>`)
            : seite("Gerät anmelden", `${err ? "<p><strong>Der Code ist ungültig oder abgelaufen.</strong></p>" : ""}
              <p>Gib den Code ein, den dein Runner anzeigt.</p>${form}<button type="submit" form="op.deviceInputForm">Weiter</button>`, false);
        },
        userCodeConfirmSource: async (ctx, form, client, _info, userCode) => {
          const name = client.clientName ?? client.clientId;
          ctx.body = englisch(ctx)
            ? seite("Confirm the device", `<p>The agent <strong>${name}</strong> wants to work on tasks on your behalf.</p>
              <p>Code: <code>${userCode}</code>. Does it match what the runner shows?</p>${form}
              <button type="submit" form="op.deviceConfirmForm">Yes, sign in</button>
              <button type="submit" form="op.deviceConfirmForm" value="yes" name="abort">Cancel</button>`)
            : seite("Gerät bestätigen", `<p>Der Agent <strong>${name}</strong> möchte in deinem Namen Aufgaben bearbeiten.</p>
              <p>Code: <code>${userCode}</code>. Stimmt er mit der Anzeige des Runners überein?</p>${form}
              <button type="submit" form="op.deviceConfirmForm">Ja, anmelden</button>
              <button type="submit" form="op.deviceConfirmForm" value="yes" name="abort">Abbrechen</button>`, false);
        },
        successSource: async (ctx) => {
          ctx.body = englisch(ctx)
            ? seite("Signed in", "<p>The runner is signed in. You can close this window.</p>")
            : seite("Angemeldet", "<p>Der Runner ist angemeldet. Du kannst dieses Fenster schließen.</p>", false);
        },
      },
    },
    findAccount: async (_ctx, sub) => {
      const m = (await db.query("select id, login from menschen where id = $1", [sub])).rows[0];
      if (!m) return undefined;
      return { accountId: String(m.id), claims: async () => ({ sub: String(m.id), preferred_username: m.login }) };
    },
    interactions: { url: (_ctx, interaktion) => `/interaction/${interaktion.uid}` },
    issueRefreshToken: async (_ctx, client) => client.grantTypeAllowed("refresh_token"),
    ttl: {
      AccessToken: 3600, AuthorizationCode: 600, DeviceCode: 600, Grant: 14 * 86400, IdToken: 3600,
      Interaction: 3600, RefreshToken: 14 * 86400, Session: 14 * 86400,
    },
    renderError: async (ctx, out) => {
      ctx.type = "html";
      const en = englisch(ctx);
      ctx.body = seite(en ? "Sign-in failed" : "Anmeldung fehlgeschlagen", `<p>${Object.values(out).map((v) => String(v).replace(/[<>&]/g, "")).join("<br>")}</p>`, en);
    },
  });
  anbieter.proxy = true; // hinter Caddy: X-Forwarded-Proto/Host vertrauen
  return anbieter;
}
