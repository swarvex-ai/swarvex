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
@media (prefers-color-scheme:dark){body{background:#0d1117;color:#e6edf3}a.k{background:#161b22;border-color:#30363d}p.l{color:#8b949e}}
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
      if (!uid || !konto || !sig || !code || !pruefen(geheimnis, `state:${uid}:${name}`, sig)) { res.writeHead(400).end("Ungültige Anmeldung"); return; }
      let nutzer: KontoNutzer;
      try { nutzer = await konto.nutzer(code, CALLBACK()); }
      catch (e) { console.error(e); res.writeHead(502).end(`${konto.titel}-Anmeldung fehlgeschlagen`); return; }
      const en = englisch(req);
      const alterTage = nutzer.erstellt ? (Date.now() - nutzer.erstellt.getTime()) / 86_400_000 : Infinity;
      if (alterTage < konfig.kontoMindestalterTage) {
        await transaktion(db, (tx) => protokollieren(tx, { projekt_id: null, akteur: "unbekannt", art: "anmeldung_abgewiesen", bezug: `konto:${name}:${nutzer.id}`, daten: { grund: "kontoalter", tage: Math.floor(alterTage) } }));
        return seite(res, 403, en ? "Account too new" : "Konto zu neu", en
          ? `<p>To protect projects from throwaway accounts, your ${esc(konto.titel)} account must be at least ${konfig.kontoMindestalterTage} days old.</p>`
          : `<p>Zum Schutz der Projekte vor Wegwerf-Konten muss dein ${esc(konto.titel)}-Konto mindestens ${konfig.kontoMindestalterTage} Tage alt sein.</p>`, en);
      }
      const menschId = await menschAusKonto(db, name, nutzer);
      const ablauf = Date.now() + 120_000;
      // Website-Anmeldung („Mein Konto“, konto.ts) statt Agenten-Anmeldung: state beginnt mit "konto-<nonce>"
      const ziel = uid.startsWith(KONTO_PRAEFIX)
        ? `/api/konto/fertig?m=${menschId}&t=${ablauf}&s=${signieren(geheimnis, `konto:${uid}:${menschId}:${ablauf}`)}`
        : `/interaction/${uid}/login?m=${menschId}&t=${ablauf}&s=${signieren(geheimnis, `login:${uid}:${menschId}:${ablauf}`)}`;
      // Keine Server-Weiterleitung: Die Kette begann mit einem Formular auf Gitea (fremde Site),
      // und manche Browser senden dann das SameSite=Lax-Interaktions-Cookie nicht mit.
      // Eine eigene Seite, die selbst weiterleitet, macht den nächsten Schritt same-site.
      res.writeHead(200, { "content-type": "text/html; charset=utf-8", "cache-control": "no-store", "referrer-policy": "no-referrer" });
      res.end(`<!doctype html><meta charset="utf-8"><meta http-equiv="refresh" content="0;url=${ziel}"><title>Anmeldung …</title><p>Anmeldung wird abgeschlossen … <a href="${ziel}">weiter</a></p>`);
      return;
    }

    const teile = url.pathname.split("/").filter(Boolean); // ["interaction", uid, ("login")]
    const uid = teile[1];
    if (teile[2] === "login") {
      const cookies = (req.headers.cookie ?? "").split(";").map((c) => c.trim().split("=")[0]).filter(Boolean);
      console.log(`interaktion ${uid}: login, Cookies: ${cookies.join(", ") || "keine"}`);
      const m = url.searchParams.get("m") ?? "", t = url.searchParams.get("t") ?? "", s = url.searchParams.get("s") ?? "";
      if (Number(t) < Date.now() || !pruefen(geheimnis, `login:${uid}:${m}:${t}`, s)) { res.writeHead(400).end("Anmeldung abgelaufen"); return; }
      await anbieter.interactionFinished(req, res, { login: { accountId: m } }, { mergeWithLastSubmission: false });
      return;
    }

    const details = await anbieter.interactionDetails(req, res);
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
      const grant = prompt.details.grantId
        ? await anbieter.Grant.find(prompt.details.grantId)
        : new anbieter.Grant({ accountId: session.accountId, clientId: String(params.client_id) });
      if (!grant) { res.writeHead(400).end("Grant unbekannt"); return; }
      if (prompt.details.missingOIDCScope) grant.addOIDCScope(prompt.details.missingOIDCScope.join(" "));
      if (prompt.details.missingOIDCClaims) grant.addOIDCClaims(prompt.details.missingOIDCClaims);
      for (const [ressource, scopes] of Object.entries(prompt.details.missingResourceScopes ?? {})) grant.addResourceScope(ressource, (scopes as string[]).join(" "));
      const grantId = await grant.save();
      await anbieter.interactionFinished(req, res, { consent: { grantId } }, { mergeWithLastSubmission: true });
      return;
    }
    res.writeHead(400).end(`Unbekannter Schritt: ${details.prompt.name}`);
  };
}
