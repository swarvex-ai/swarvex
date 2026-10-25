// MCP-Gateway (Streamable HTTP, zustandslos). Welche Werkzeuge ein Agent sieht, hängt von
// den Rollen seines Menschen ab (docs/architektur.md, „MCP-Gateway: Werkzeuge pro Rolle“).
import { REGELN_TEXT } from "./cleanroom.ts";
import type { IncomingMessage, ServerResponse } from "node:http";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { z } from "zod";
import type { Db } from "./db.ts";
import * as k from "./aufgaben.ts";
import { kennzahlen } from "./kennzahlen.ts";
import type { Zugang } from "./auth/zugang.ts";

const text = (wert: unknown) => ({ content: [{ type: "text" as const, text: typeof wert === "string" ? wert : JSON.stringify(wert, null, 2) }] });

/** Werkzeug-Aufruf mit einheitlicher Fehlerbehandlung: fachliche Ablehnungen sind Tool-Fehler, keine Abstürze. */
function sicher<A>(f: (a: A) => Promise<unknown>) {
  return async (a: A) => {
    try { return text(await f(a)); }
    catch (e) {
      if (e instanceof k.Abgelehnt) return { ...text(`Abgelehnt: ${e.message}`), isError: true };
      throw e;
    }
  };
}

async function istLeadIrgendwo(db: Db, menschId: number) {
  const r = await db.query("select 1 from projekt_rollen where mensch_id = $1 and rolle in ('lead', 'visionaer') limit 1", [menschId]);
  return r.rowCount! > 0;
}

export async function mcpServerFuer(db: Db, z0: Zugang): Promise<McpServer> {
  const s = new McpServer({ name: "swarvex", version: "0.1.0" });
  const wer = { menschId: z0.menschId, agentId: z0.agentId };

  s.registerTool("projekt_kontext", {
    description: "Vision, Ziel, Engine, Lizenz, Lead-Modell und deine Rolle in einem Projekt.",
    inputSchema: { projekt: z.string().describe("Projekt-Slug, z. B. rend-and-run-test") },
  }, sicher(async ({ projekt }) => {
    const p = await k.projektPerSlug(db, projekt);
    return { projekt: p.slug, name: p.name, vision: p.vision_md, ziel: p.ziel, engine: p.engine, repo: p.repo_url,
      lizenz_code: p.lizenz_code, lizenz_assets: p.lizenz_assets, lead_modell: p.lead_modell,
      deine_rollen: await k.rollenVon(db, wer.menschId, p.id), regeln: "Beiträge nur als Patch in den erlaubten Pfaden; mit der Einreichung versicherst du die Rechte daran (docs/beitragserklaerung.md).",
      verfahren: p.verfahren, ...(p.verfahren === "clean_room" ? { vorbild: p.vorbild, clean_room_regeln: REGELN_TEXT } : {}) };
  }));

  s.registerTool("kennzahlen", {
    description: "Statusboard eines Projekts: Fortschritt in %, aktive Agenten, offene Aufgaben, letzter Vorgang.",
    inputSchema: { projekt: z.string() },
  }, sicher(async ({ projekt }) => kennzahlen(db, projekt)));

  s.registerTool("aufgaben_passend", {
    description: "Offene Aufgaben, die du bearbeiten darfst; optional gefiltert nach deinen Fähigkeiten (z. B. code.gdscript).",
    inputSchema: { projekt: z.string(), faehigkeiten: z.array(z.string()).optional() },
  }, sicher(async ({ projekt, faehigkeiten }) => k.aufgabenPassend(db, wer, projekt, faehigkeiten)));

  s.registerTool("aufgabe_reservieren", {
    description: "Reserviert eine offene Aufgabe für diesen Agenten (Lease mit Ablaufzeit).",
    inputSchema: { aufgabe_id: z.number().int() },
  }, sicher(async ({ aufgabe_id }) => k.reservieren(db, wer, aufgabe_id)));

  s.registerTool("aufgabe_details", {
    description: "Vollständige Aufgabe: Beschreibung, erlaubte Pfade, Akzeptanzkriterien, Basis-Commit, Grenzen.",
    inputSchema: { aufgabe_id: z.number().int() },
  }, sicher(async ({ aufgabe_id }) => k.details(db, wer, aufgabe_id)));

  s.registerTool("fortschritt_melden", {
    description: "Lebenszeichen: verlängert die Reservierung und meldet Fortschritt und Verbrauch.",
    inputSchema: { aufgabe_id: z.number().int(), notiz: z.string(), verbrauch: z.record(z.string(), z.number()).default({}) },
  }, sicher(async ({ aufgabe_id, notiz, verbrauch }) => k.fortschritt(db, wer, aufgabe_id, notiz, verbrauch)));

  s.registerTool("ergebnis_einreichen", {
    description: "Reicht das Ergebnis als Patch (git format-patch/diff gegen den Basis-Commit) ein. Mit der Einreichung versicherst du, alle Rechte daran zu halten.",
    inputSchema: {
      aufgabe_id: z.number().int(), patch: z.string(), zusammenfassung: z.string(),
      eigene_tests: z.string().default(""), verbrauch: z.record(z.string(), z.number()).default({}),
      rechte_versichert: z.literal(true).describe("Bestätigung der Beitragserklärung"),
    },
  }, sicher(async ({ aufgabe_id, patch, zusammenfassung, eigene_tests, verbrauch }) => k.einreichen(db, wer, aufgabe_id, { patch, zusammenfassung, eigene_tests, verbrauch })));

  s.registerTool("abbruch_melden", {
    description: "Gibt eine reservierte Aufgabe mit Begründung zurück in den Pool.",
    inputSchema: { aufgabe_id: z.number().int(), grund: z.string() },
  }, sicher(async ({ aufgabe_id, grund }) => { await k.abbrechen(db, wer, aufgabe_id, grund); return "zurückgegeben"; }));

  if (await istLeadIrgendwo(db, wer.menschId)) {
    s.registerTool("aufgabe_anlegen", {
      description: "(Lead) Legt eine Aufgabe an: Stufe A–D, Fähigkeiten, erlaubte Pfade (Globs), Akzeptanzkriterien, Basis-Commit, optional Meilenstein. In Clean-Room-Projekten Pflicht: raum (analyse/pruefung schreiben nur unter spezifikation/, bau nie dort).",
      inputSchema: {
        projekt: z.string(), titel: z.string(), beschreibung: z.string(), stufe: z.enum(["A", "B", "C", "D"]),
        faehigkeiten: z.array(z.string()), pfade: z.array(z.string()), akzeptanz: z.array(z.string()),
        basis_commit: z.string().optional(), lease_minuten: z.number().int().min(5).max(24 * 60).optional(),
        min_vertrauen: z.number().int().min(0).max(3).optional(), meilenstein: z.string().optional(),
        raum: z.enum(["analyse", "pruefung", "bau"]).optional().describe("Nur Clean-Room-Projekte: analyse (Vorbild untersuchen, Spezifikation schreiben), pruefung (Spezifikation prüfen), bau (aus der Spezifikation implementieren)"),
      },
    }, sicher(async ({ projekt, ...a }) => k.aufgabeAnlegen(db, wer, projekt, a)));

    s.registerTool("einreichungen_offen", {
      description: "(Lead) Einreichungen eines Projekts mit Prüfergebnis, die auf deine Bewertung warten.",
      inputSchema: { projekt: z.string() },
    }, sicher(async ({ projekt }) => {
      const p = await k.projektPerSlug(db, projekt);
      const r = await db.query(
        `select e.id as einreichung_id, a.id as aufgabe_id, a.titel, a.zustand, e.zusammenfassung, e.patch, p.ergebnis as pruefung, p.bericht
         from einreichungen e join aufgaben a on a.id = e.aufgabe_id
         left join lateral (select ergebnis, bericht from pruefungen where einreichung_id = e.id order by id desc limit 1) p on true
         where a.projekt_id = $1 and a.zustand in ('eingereicht', 'pruefung', 'geprueft_ok') order by e.id`, [p.id]);
      return r.rows;
    }));

    s.registerTool("einreichung_bewerten", {
      description: "(Lead) Bewertet eine geprüfte Einreichung: annehmen, nacharbeit (zurück in den Pool) oder ablehnen – immer mit Begründung.",
      inputSchema: { einreichung_id: z.number().int(), urteil: z.enum(["annehmen", "nacharbeit", "ablehnen"]), begruendung: z.string().min(3) },
    }, sicher(async ({ einreichung_id, urteil, begruendung }) => { await k.bewerten(db, wer, einreichung_id, urteil, begruendung); return "bewertet"; }));
  }
  return s;
}

/** Ein HTTP-Aufruf an /mcp: zustandsloser Transport, neuer Server pro Anfrage. */
export async function mcpBehandeln(db: Db, zugang: Zugang, req: IncomingMessage, res: ServerResponse, body: unknown) {
  const server = await mcpServerFuer(db, zugang);
  const transport = new StreamableHTTPServerTransport({ sessionIdGenerator: undefined, enableJsonResponse: true });
  res.on("close", () => { transport.close(); server.close(); });
  await server.connect(transport);
  (req as any).auth = { token: zugang.token, clientId: zugang.clientId, scopes: zugang.scopes, expiresAt: zugang.ablauf };
  await transport.handleRequest(req as any, res, body);
}
