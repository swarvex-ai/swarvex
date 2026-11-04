// Anmelde-Interaktion: Der Mensch meldet sich über einen Konto-Anbieter an (GitHub, Codeberg,
// Gitea/Forgejo; konten.ts). Danach erteilt die Plattform als First-Party-Dienst die Zustimmung automatisch.
// Ablauf: /interaction/<uid> → (Auswahl, falls mehrere Anbieter) → Anbieter → /interaction/callback
//         → /interaction/<uid>/login.
import { createHmac, timingSafeEqual } from "node:crypto";
import type { IncomingMessage, ServerResponse } from "node:http";
import type Provider from "oidc-provider";
import type { Db } from "../db.ts";
import { transaktion } from "../db.ts";
import { protokollieren } from "../ereignisse.ts";
import { konfig } from "../konfig.ts";
import type { KontoAnbieter, KontoNutzer } from "./konten.ts";

export const CALLBACK = () => `${konfig.basis}/interaction/callback`;
const LOOPBACK = ["127.0.0.1", "localhost", "[::1]"];
export const KONTO_PRAEFIX = "konto-";

export function signieren(geheimnis: string, wert: string) {
  return createHmac("sha256", geheimnis).update(wert).digest("base64url");
}
export function pruefen(geheimnis: string, wert: string, sig: string) {
  const a = Buffer.from(signieren(geheimnis, wert));
  const b = Buffer.from(sig);
  return a.length === b.length && timingSafeEqual(a, b);
}

function weiter(res: ServerResponse, ziel: string) {
  res.writeHead(303, { location: ziel });
  res.end();
}

export const esc = (s: string) => s.replace(/[<>&"]/g, (z) => `&#${z.charCodeAt(0)};`);
export const englisch = (req: IncomingMessage) => !/^de\b/i.test(req.headers["accept-language"] ?? "");

export function seite(res: ServerResponse, status: number, titel: string, inhalt: string, en: boolean) {
  res.writeHead(status, { "content-type": "text/html; charset=utf-8", "cache-control": "no-store", "referrer-policy": "no-referrer" });
  res.end(`<!doctype html><html lang="${en ? "en" : "de"}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>${esc(titel)} · swarvex.ai</title><style>
body{font:16px/1.5 system-ui,sans-serif;max-width:26rem;margin:4rem auto;padding:0 1rem;color:#1d2328;background:#f6f8fa}
h1{font-size:1.4rem}a.k{display:block;margin:.6rem 0;padding:.75rem 1rem;border:1px solid #c9d1d9;border-radius:8px;background:#fff;color:inherit;text-decoration:none;font-weight:600}
a.k:hover,a.k:focus-visible{border-color:#0f9f8f;outline:none}p.l{color:#57606a;font-size:.9rem}
button{font:inherit;padding:.55rem 1.1rem;border-radius:8px;border:1px solid #c9d1d9;background:#fff;color:inherit;cursor:pointer;margin:.3rem .3rem 0 0}
button.haupt{background:#0f9f8f;border-color:#0f9f8f;color:#fff;font-weight:600}
@media (prefers-color-scheme:dark){body{background:#0d1117;color:#e6edf3}a.k,button{background:#161b22;border-color:#30363d}button.haupt{background:#0f9f8f}p.l{color:#8b949e}}
</style></head><body><h1>${esc(titel)}</h1>${inhalt}</body></html>`);
}

/** Mensch zum Konto anlegen bzw. finden. */
async function menschAusKonto(db: Db, anbieter: string, nutzer: KontoNutzer): Promise<number> {
  return transaktion(db, async (tx) => {
    const r = await tx.query(
      `insert into menschen(konto_provider, konto_id, login) values ($1, $2, $3)
       on conflict (konto_provider, konto_id) do update set login = excluded.login returning id, (xmax = 0) as neu`,
      [anbieter, nutzer.id, nutzer.login],
    );
    const { id, neu } = r.rows[0];
    if (neu) await protokollieren(tx, { projekt_id: null, akteur: `mensch:${id}`, art: "mensch_registriert", bezug: `mensch:${id}`, daten: { provider: anbieter, login: nutzer.login } });
    return Number(id);
  });
}

export function interaktionBehandeln(anbieter: Provider, db: Db, geheimnis: string, konten: KontoAnbieter[]) {
  return async (req: IncomingMessage, res: ServerResponse, url: URL): Promise<void> => {
    // Rückkehr vom Konto-Anbieter: Code tauschen, Konto lesen, dann zurück auf den Interaktionspfad
    // (dort liegt das Interaktions-Cookie des Anbieters).
    if (url.pathname === "/interaction/callback") {
      const [uid, name, sig] = (url.searchParams.get("state") ?? "").split(".");
      const code = url.searchParams.get("code");
      const konto = konten.find((k) => k.name === name);
      if (!uid || !konto || !sig || !code || !pruefen(geheimnis, `state:${uid}:${name}`, sig)) { res.writeHead(400).end("invalid sign-in"); return; }
      let nutzer: KontoNutzer;
      try { nutzer = await konto.nutzer(code, CALLBACK()); }
      catch (e) { console.error(e); res.writeHead(502).end(`${konto.titel} sign-in failed`); return; }
      const en = englisch(req);
      const alterTage = nutzer.erstellt ? (Date.now() - nutzer.erstellt.getTime()) / 86_400_000 : Infinity;
      const ausnahme = konfig.kontoAlterAusnahmen.has(`${name}:${nutzer.id}`);
      if (alterTage < konfig.kontoMindestalterTage && ausnahme) {
        // sichtbar im Protokoll, damit Ausnahmen nicht still bleiben
        await transaktion(db, (tx) => protokollieren(tx, { projekt_id: null, akteur: "unbekannt", art: "anmeldung_ausnahme", bezug: `konto:${name}:${nutzer.id}`, daten: { grund: "kontoalter", tage: Math.floor(alterTage) } }));
      } else if (alterTage < konfig.kontoMindestalterTage) {
        await transaktion(db, (tx) => protokollieren(tx, { projekt_id: null, akteur: "unbekannt", art: "anmeldung_abgewiesen", bezug: `konto:${name}:${nutzer.id}`, daten: { grund: "kontoalter", tage: Math.floor(alterTage) } }));
        return seite(res, 403, en ? "Account too new" : "Konto zu neu", en
          ? `<p>To protect projects from throwaway accounts, your ${esc(konto.titel)} account must be at least ${konfig.kontoMindestalterTage} days old.</p>`
          : `<p>Zum Schutz der Projekte vor Wegwerf-Konten muss dein ${esc(konto.titel)}-Konto mindestens ${konfig.kontoMindestalterTage} Tage alt sein.</p>`, en);
      }
      const menschId = await menschAusKonto(db, name, nutzer);
      const ablauf = Date.now() + 120_000;
      // Website-Anmeldung („Mein Konto“, konto.ts) statt Agenten-Anmeldung: state beginnt mit "konto-<nonce>"
      const ziel = uid.startsWith(KONTO_PRAEFIX)
        ? `/api/account/done?m=${menschId}&t=${ablauf}&s=${signieren(geheimnis, `konto:${uid}:${menschId}:${ablauf}`)}`
        : `/interaction/${uid}/login?m=${menschId}&t=${ablauf}&s=${signieren(geheimnis, `login:${uid}:${menschId}:${ablauf}`)}`;
      // Keine Server-Weiterleitung: Die Kette begann mit einem Formular auf Gitea (fremde Site),
      // und manche Browser senden dann das SameSite=Lax-Interaktions-Cookie nicht mit.
      // Eine eigene Seite, die selbst weiterleitet, macht den nächsten Schritt same-site.
      res.writeHead(200, { "content-type": "text/html; charset=utf-8", "cache-control": "no-store", "referrer-policy": "no-referrer" });
      res.end(`<!doctype html><meta charset="utf-8"><meta http-equiv="refresh" content="0;url=${ziel}"><title>Signing in …</title><p>Finishing sign-in … <a href="${ziel}">continue</a></p>`);
      return;
    }

    const teile = url.pathname.split("/").filter(Boolean); // ["interaction", uid, ("login")]
    const uid = teile[1];
    if (teile[2] === "login") {
      const cookies = (req.headers.cookie ?? "").split(";").map((c) => c.trim().split("=")[0]).filter(Boolean);
      console.log(`interaction ${uid}: login, cookies: ${cookies.join(", ") || "none"}`);
      const m = url.searchParams.get("m") ?? "", t = url.searchParams.get("t") ?? "", s = url.searchParams.get("s") ?? "";
      if (Number(t) < Date.now() || !pruefen(geheimnis, `login:${uid}:${m}:${t}`, s)) { res.writeHead(400).end("sign-in expired"); return; }
      await anbieter.interactionFinished(req, res, { login: { accountId: m } }, { mergeWithLastSubmission: false });
      return;
    }

    let details;
    try { details = await anbieter.interactionDetails(req, res); }
    catch (e) {
      // Abgelaufen, schon benutzt oder in einem anderen Browser begonnen: verständlich sagen statt Serverfehler
      if ((e as { statusCode?: number }).statusCode !== 400) throw e;
      const en = englisch(req);
      return seite(res, 400, en ? "Sign-in expired" : "Anmeldung abgelaufen", en
        ? "<p>This sign-in has expired or was started in another browser. Start it again from your agent (for the runner: <code>swarvex login</code>).</p>"
        : "<p>Diese Anmeldung ist abgelaufen oder wurde in einem anderen Browser begonnen. Starte sie in deinem Agenten neu (beim Runner: <code>swarvex login</code>).</p>", en);
    }
    if (details.prompt.name === "login") {
      const zuAnbieter = (k: KontoAnbieter) => k.anmelden(CALLBACK(), `${details.uid}.${k.name}.${signieren(geheimnis, `state:${details.uid}:${k.name}`)}`);
      const gewaehlt = konten.length === 1 ? konten[0] : konten.find((k) => k.name === url.searchParams.get("anbieter"));
      if (gewaehlt) return weiter(res, zuAnbieter(gewaehlt));
      const en = englisch(req);
      const client = await anbieter.Client.find(String((details.params as any).client_id));
      const wer = client?.clientName ? `<p class="l">${en ? "Agent" : "Agent"}: <strong>${esc(String(client.clientName))}</strong></p>` : "";
      return seite(res, 200, en ? "Sign in to swarvex.ai" : "Bei swarvex.ai anmelden",
        `${wer}${konten.map((k) => `<a class="k" href="/interaction/${details.uid}?anbieter=${k.name}">${en ? "Continue with" : "Weiter mit"} ${esc(k.titel)}</a>`).join("")}
         <p class="l">${en ? "We only read your public profile (name and account ID)." : "Wir lesen nur dein öffentliches Profil (Name und Konto-ID)."}</p>`, en);
    }
    if (details.prompt.name === "consent") {
      const { prompt, params, session } = details as any;
      // Zustimmung: Agenten auf dem eigenen Rechner (Rücksprung auf Loopback) und die Geräte-Anmeldung (eigene
      // Bestätigungsseite, ohne Rücksprung) brauchen keine weitere Frage: Der Code landet nur bei der Person selbst.
      // Jeder andere Client (Rücksprung auf eine Website) muss einmal ausdrücklich bestätigt werden, sonst könnte
      // jede Website mit einem selbst registrierten Client und einem Link ein Token im Namen der Person holen.
      const en = englisch(req);
      const ziel = params.redirect_uri ? URL.parse(String(params.redirect_uri)) : null;
      const lokal = !params.redirect_uri || (ziel?.protocol === "http:" && LOOPBACK.includes(ziel.hostname));
      if (!lokal) {
        const vonHier = req.method === "POST" && req.headers.origin === new URL(konfig.basis).origin;
        if (teile[2] === "abort" && vonHier) {
          console.log(JSON.stringify({ type: "consent_declined", client_host: ziel?.host ?? "" }));
          await anbieter.interactionFinished(req, res, { error: "access_denied", error_description: "The person declined" }, { mergeWithLastSubmission: false });
          return;
        }
        if (!(teile[2] === "confirm" && vonHier)) {
          const client = await anbieter.Client.find(String(params.client_id));
          const login = (await db.query("select login from menschen where id = $1", [session.accountId])).rows[0]?.login ?? "?";
          const name = esc(String(client?.clientName ?? params.client_id));
          const host = esc(ziel?.host ?? String(params.redirect_uri));
          const knopf = (pfad: string, text: string, haupt: boolean) =>
            `<form method="post" action="/interaction/${esc(details.uid)}/${pfad}" style="display:inline"><button type="submit"${haupt ? ' class="haupt"' : ""}>${text}</button></form>`;
          return seite(res, 200, en ? "Allow this agent?" : "Diesen Agenten zulassen?", en
            ? `<p>The agent <strong>${name}</strong> wants to work on swarvex.ai on your behalf: pick up and submit tasks, and where you are lead or area lead, review and accept contributions.</p>
               <p>After you allow it, you are sent back to <strong>${host}</strong>.</p>
               <p class="l">Signed in as <strong>${esc(login)}</strong>. Only allow agents you are setting up yourself right now. You can sign agents out any time under My account.</p>
               ${knopf("confirm", "Allow", true)} ${knopf("abort", "Cancel", false)}`
            : `<p>Der Agent <strong>${name}</strong> möchte in deinem Namen auf swarvex.ai arbeiten: Aufgaben holen und einreichen und, wo du Lead oder Area lead bist, Beiträge bewerten und annehmen.</p>
               <p>Nach der Zustimmung geht es zurück zu <strong>${host}</strong>.</p>
               <p class="l">Angemeldet als <strong>${esc(login)}</strong>. Lass nur Agenten zu, die du gerade selbst einrichtest. Abmelden kannst du sie jederzeit unter Mein Konto.</p>
               ${knopf("confirm", "Zulassen", true)} ${knopf("abort", "Abbrechen", false)}`, en);
        }
      }
      if (!lokal) console.log(JSON.stringify({ type: "consent_allowed", client_host: ziel?.host ?? "" }));
      const grant = prompt.details.grantId
        ? await anbieter.Grant.find(prompt.details.grantId)
        : new anbieter.Grant({ accountId: session.accountId, clientId: String(params.client_id) });
      if (!grant) { res.writeHead(400).end("unknown grant"); return; }
      if (prompt.details.missingOIDCScope) grant.addOIDCScope(prompt.details.missingOIDCScope.join(" "));
      if (prompt.details.missingOIDCClaims) grant.addOIDCClaims(prompt.details.missingOIDCClaims);
      for (const [ressource, scopes] of Object.entries(prompt.details.missingResourceScopes ?? {})) grant.addResourceScope(ressource, (scopes as string[]).join(" "));
      const grantId = await grant.save();
      await anbieter.interactionFinished(req, res, { consent: { grantId } }, { mergeWithLastSubmission: true });
      return;
    }
    res.writeHead(400).end(`unknown step: ${details.prompt.name}`);
  };
}
