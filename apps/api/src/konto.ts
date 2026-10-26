// „Mein Konto“ auf der Website (Issue #25, Teil): eigene Agenten sehen und abmelden, eigene Beiträge sehen.
// Anmeldung über denselben Konto-Anbieter wie bei Agenten und mit derselben Rücksprung-Adresse:
//   /api/account/login → Anbieter → /interaction/callback (state "konto-<nonce>…") → /api/account/done → /konto
// Die Sitzung ist ein signiertes Cookie ohne Serverzustand; „Abmelden“ beendet alle Sitzungen des Menschen.
import { randomBytes } from "node:crypto";
import type { IncomingMessage, ServerResponse } from "node:http";
import { Abgelehnt, leasesBeenden } from "./aufgaben.ts";
import { transaktion, type Db } from "./db.ts";
import { protokollieren } from "./ereignisse.ts";
import { konfig } from "./konfig.ts";
import { nachAussen } from "./aussen.ts";
import { CALLBACK, englisch, esc, KONTO_PRAEFIX, pruefen, seite, signieren } from "./auth/interaktion.ts";
import type { KontoAnbieter } from "./auth/konten.ts";

const SITZUNG = "swarvex_account";
const ANMELDUNG = "swarvex_login";
const SITZUNG_TAGE = 14;

function cookie(req: IncomingMessage, name: string) {
  for (const teil of (req.headers.cookie ?? "").split(";")) {
    const [k, ...v] = teil.trim().split("=");
    if (k === name) return v.join("=");
  }
  return undefined;
}

function cookieSetzen(name: string, wert: string, pfad: string, sekunden: number) {
  const sicher = konfig.basis.startsWith("https:") ? "; Secure" : "";
  return `${name}=${wert}; Path=${pfad}; Max-Age=${sekunden}; HttpOnly; SameSite=Lax${sicher}`;
}

/** Mensch der Website-Sitzung oder undefined (abgelaufen, gesperrt, abgemeldet, gefälscht). */
export async function sitzungLesen(db: Db, geheimnis: string, wert: string | undefined): Promise<number | undefined> {
  const [m, ausgestellt, sig] = (wert ?? "").split(".");
  if (!m || !ausgestellt || !sig || !pruefen(geheimnis, `sitzung:${m}:${ausgestellt}`, sig)) return undefined;
  const zeit = Number(ausgestellt);
  if (!(zeit > Date.now() - SITZUNG_TAGE * 86_400_000)) return undefined;
  const r = (await db.query("select gesperrt, sitzungen_ab from menschen where id = $1", [m])).rows[0];
  if (!r || r.gesperrt || (r.sitzungen_ab && zeit <= r.sitzungen_ab.getTime())) return undefined;
  return Number(m);
}

export function sitzungAusstellen(geheimnis: string, menschId: number, jetzt = Date.now()) {
  return `${menschId}.${jetzt}.${signieren(geheimnis, `sitzung:${menschId}:${jetzt}`)}`;
}

/** Alles, was die Konto-Seite zeigt; zugleich der Export „Meine Daten“. */
export async function kontoDaten(db: Db, menschId: number) {
  const mensch = (await db.query("select login, konto_provider as anbieter, erstellt, vertrauensstufe from menschen where id = $1", [menschId])).rows[0];
  const rollen = (await db.query(
    `select p.slug as projekt, p.name, r.rolle from projekt_rollen r join projekte p on p.id = r.projekt_id
     where r.mensch_id = $1 order by p.name, r.rolle`, [menschId])).rows;
  const agenten = (await db.query(
    `select a.id::int, a.name, a.client_id, a.erstellt, a.abgemeldet,
       exists(select 1 from oidc_payloads o where o.typ = 'Grant' and o.payload->>'accountId' = a.mensch_id::text
              and o.payload->>'clientId' = a.client_id and (o.laeuft_ab is null or o.laeuft_ab > now())) as angemeldet,
       greatest((select max(l.erstellt) from leases l where l.agent_id = a.id),
                (select max(e.eingereicht) from einreichungen e where e.agent_id = a.id)) as zuletzt,
       (select count(*)::int from leases l where l.agent_id = a.id and l.beendet is null) as reservierungen,
       (select count(*)::int from einreichungen e where e.agent_id = a.id) as einreichungen
     from agenten a where a.mensch_id = $1 order by a.erstellt desc`, [menschId])).rows;
  const beitraege = (await db.query(
    `select e.id::int as einreichung, a.id::int as aufgabe, a.titel, p.slug as projekt, p.name as projekt_name, ag.name as agent,
       e.eingereicht, a.zustand, coalesce((e.verbrauch->>'tokens')::bigint, 0)::bigint as tokens,
       (select pr.ergebnis from pruefungen pr where pr.einreichung_id = e.id and pr.quelle = 'projekt' order by pr.id desc limit 1) as pruefung,
       (select b.urteil from bewertungen b where b.einreichung_id = e.id order by b.id desc limit 1) as urteil
     from einreichungen e join agenten ag on ag.id = e.agent_id join aufgaben a on a.id = e.aufgabe_id join projekte p on p.id = a.projekt_id
     where ag.mensch_id = $1 order by e.eingereicht desc limit 500`, [menschId])).rows.map((b) => ({ ...b, tokens: Number(b.tokens) }));
  const summe = {
    einreichungen: beitraege.length,
    angenommen: beitraege.filter((b) => b.urteil === "annehmen").length,
    tokens: beitraege.reduce((n, b) => n + b.tokens, 0),
  };
  return { mensch, rollen, agenten, beitraege, summe };
}

/**
 * Meldet einen Agenten ab: Grants und alle daraus ausgestellten Tokens (Refresh, Device, Codes) werden gelöscht,
 * schon ausgestellte Access-Tokens verlieren über agenten.abgemeldet ihre Gültigkeit (auth/zugang.ts),
 * laufende Reservierungen enden und die Aufgaben gehen zurück in den Pool.
 */
export async function agentAbmelden(db: Db, menschId: number, agentId: number) {
  return transaktion(db, async (tx) => {
    const a = (await tx.query("select id, client_id, name from agenten where id = $1 and mensch_id = $2 for update", [agentId, menschId])).rows[0];
    if (!a) throw new Abgelehnt("unknown agent");
    const grants = (await tx.query(
      "select id from oidc_payloads where typ = 'Grant' and payload->>'accountId' = $1 and payload->>'clientId' = $2",
      [String(menschId), a.client_id])).rows.map((r) => r.id);
    if (grants.length) await tx.query("delete from oidc_payloads where grant_id = any($1) or (typ = 'Grant' and id = any($1))", [grants]);
    await tx.query("update agenten set abgemeldet = now() where id = $1", [a.id]);
    const reservierungen = await leasesBeenden(tx, { agent: Number(a.id) }, "reservierung_beendet_abmeldung");
    await protokollieren(tx, { projekt_id: null, akteur: `mensch:${menschId}`, art: "agent_abgemeldet", bezug: `agent:${a.id}`, daten: { name: a.name, reservierungen } });
    return { reservierungen };
  });
}

export function kontoBehandeln(db: Db, geheimnis: string, konten: KontoAnbieter[]) {
  const json = (res: ServerResponse, status: number, wert: unknown, kopf: Record<string, string> = {}) => {
    res.writeHead(status, { "content-type": "application/json", "cache-control": "no-store", ...kopf });
    res.end(JSON.stringify(wert));
  };
  const zurSeite = (en: boolean) => (en ? "/en/account" : "/konto");
  // Zustandsändernde Aufrufe nur von der eigenen Website (zusätzlich zu SameSite=Lax)
  const eigeneHerkunft = (req: IncomingMessage) => req.headers.origin === new URL(konfig.basis).origin;

  return async (req: IncomingMessage, res: ServerResponse, url: URL): Promise<void> => {
    const pfad = url.pathname;

    if (pfad === "/api/account/login" && req.method === "GET") {
      const en = url.searchParams.get("lang") === "en" || (!url.searchParams.has("lang") && englisch(req));
      const sprache = en ? "en" : "de";
      const gewaehlt = konten.length === 1 ? konten[0] : konten.find((k) => k.name === url.searchParams.get("provider"));
      if (!gewaehlt) {
        return seite(res, 200, en ? "Sign in to swarvex.ai" : "Bei swarvex.ai anmelden",
          `${konten.map((k) => `<a class="k" href="/api/account/login?lang=${sprache}&amp;provider=${k.name}">${en ? "Continue with" : "Weiter mit"} ${esc(k.titel)}</a>`).join("")}
           <p class="l">${en ? "We only read your public profile (name and account ID)." : "Wir lesen nur dein öffentliches Profil (Name und Konto-ID)."}</p>`, en);
      }
      const nonce = randomBytes(16).toString("base64url");
      const uid = KONTO_PRAEFIX + nonce;
      res.writeHead(303, {
        location: gewaehlt.anmelden(CALLBACK(), `${uid}.${gewaehlt.name}.${signieren(geheimnis, `state:${uid}:${gewaehlt.name}`)}`),
        "set-cookie": cookieSetzen(ANMELDUNG, `${nonce}.${sprache}`, "/api/account", 900), "cache-control": "no-store",
      });
      res.end();
      return;
    }

    if (pfad === "/api/account/done" && req.method === "GET") {
      // Hierher leitet /interaction/callback mit einer eigenen Seite weiter (same-site), deshalb ist das Anmelde-Cookie dabei.
      const [nonce, sprache] = (cookie(req, ANMELDUNG) ?? "").split(".");
      const en = sprache === "en";
      const m = url.searchParams.get("m") ?? "", t = url.searchParams.get("t") ?? "", s = url.searchParams.get("s") ?? "";
      if (!nonce || Number(t) < Date.now() || !pruefen(geheimnis, `konto:${KONTO_PRAEFIX}${nonce}:${m}:${t}`, s)) {
        return seite(res, 400, en ? "Sign-in expired" : "Anmeldung abgelaufen",
          `<p>${en ? "Please try again." : "Bitte versuch es noch einmal."}</p><a class="k" href="/api/account/login?lang=${en ? "en" : "de"}">${en ? "Sign in" : "Anmelden"}</a>`, en);
      }
      const gesperrt = (await db.query("select gesperrt from menschen where id = $1", [m])).rows[0]?.gesperrt;
      if (gesperrt) return seite(res, 403, en ? "Account blocked" : "Konto gesperrt", `<p>${en ? "This account is blocked." : "Dieses Konto ist gesperrt."}</p>`, en);
      res.writeHead(303, {
        location: zurSeite(en), "cache-control": "no-store",
        "set-cookie": [cookieSetzen(SITZUNG, sitzungAusstellen(geheimnis, Number(m)), "/", SITZUNG_TAGE * 86400), cookieSetzen(ANMELDUNG, "", "/api/account", 0)],
      });
      res.end();
      return;
    }

    const menschId = await sitzungLesen(db, geheimnis, cookie(req, SITZUNG));

    if (pfad === "/api/account" && req.method === "GET") {
      if (!menschId) return json(res, 401, { error: "not_signed_in" });
      const kopf: Record<string, string> = url.searchParams.has("export")
        ? { "content-disposition": `attachment; filename="swarvex-account-${new Date().toISOString().slice(0, 10)}.json"` } : {};
      return json(res, 200, nachAussen(await kontoDaten(db, menschId)), kopf);
    }

    if (req.method === "POST" && (pfad === "/api/account/sign-out" || /^\/api\/account\/agents\/\d+\/sign-out$/.test(pfad))) {
      if (!eigeneHerkunft(req)) return json(res, 403, { error: "foreign_origin" });
      if (!menschId) return json(res, 401, { error: "not_signed_in" });
      if (pfad === "/api/account/sign-out") {
        await db.query("update menschen set sitzungen_ab = now() where id = $1", [menschId]);
        return json(res, 200, { ok: true }, { "set-cookie": cookieSetzen(SITZUNG, "", "/", 0) });
      }
      try {
        return json(res, 200, nachAussen(await agentAbmelden(db, menschId, Number(pfad.split("/")[4]))));
      } catch (e) {
        if (e instanceof Abgelehnt) return json(res, 404, { error: e.message });
        throw e;
      }
    }

    return json(res, 404, { error: "not_found" });
  };
}
