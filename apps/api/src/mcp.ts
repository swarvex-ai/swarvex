// MCP-Gateway (Streamable HTTP, zustandslos). Werkzeuge, Parameter und Ergebnisse sind englisch (aussen.ts). Welche Werkzeuge ein Agent sieht, hängt von
// den Rollen seines Menschen ab (docs/architecture.md, „MCP-Gateway: Werkzeuge pro Rolle“).
import { REGELN_TEXT, type Raum } from "./cleanroom.ts";
import { nachAussen, umkehren, verbrauchRein, WERTE } from "./aussen.ts";
import type { IncomingMessage, ServerResponse } from "node:http";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { z } from "zod";
import type { Db } from "./db.ts";
import * as k from "./aufgaben.ts";
import { kennzahlen } from "./kennzahlen.ts";
import { gegenpruefungMelden, naechsteGegenpruefung } from "./gegenpruefung.ts";
import type { Zugang } from "./auth/zugang.ts";

const text = (wert: unknown) => ({ content: [{ type: "text" as const, text: typeof wert === "string" ? wert : JSON.stringify(nachAussen(wert), null, 2) }] });

/** Werkzeug-Aufruf mit einheitlicher Fehlerbehandlung: fachliche Ablehnungen sind Tool-Fehler, keine Abstürze. */
function sicher<A>(f: (a: A) => Promise<unknown>) {
  return async (a: A) => {
    try { return text(await f(a)); }
    catch (e) {
      if (e instanceof k.Abgelehnt) return { ...text(`Rejected: ${e.message}`), isError: true };
      throw e;
    }
  };
}

async function istLeadIrgendwo(db: Db, menschId: number) {
  const r = await db.query("select 1 from projekt_rollen where mensch_id = $1 and rolle in ('lead', 'visionaer') limit 1", [menschId]);
  return r.rowCount! > 0;
}

async function istVisionaerIrgendwo(db: Db, menschId: number) {
  const r = await db.query("select 1 from projekt_rollen where mensch_id = $1 and rolle = 'visionaer' limit 1", [menschId]);
  return r.rowCount! > 0;
}

const RAUM_REIN = umkehren(WERTE.raum) as Record<string, Raum>;
const URTEIL_REIN = umkehren(WERTE.urteil) as Record<string, "annehmen" | "nacharbeit" | "ablehnen">;
const usage = z.record(z.string(), z.number()).default({}).describe("Usage so far, e.g. {\"tokens\": 12000, \"requests\": 8, \"seconds\": 340}");

export async function mcpServerFuer(db: Db, z0: Zugang): Promise<McpServer> {
  const s = new McpServer({ name: "swarvex", version: "0.1.0" });
  const wer = { menschId: z0.menschId, agentId: z0.agentId, runner: z0.runner };

  s.registerTool("project_context", {
    description: "Vision, goal, engine, licenses, lead model and your roles in a project.",
    inputSchema: { project: z.string().describe("Project slug, e.g. rend-and-run-test") },
  }, sicher(async ({ project }) => {
    const p = await k.projektPerSlug(db, project);
    return { projekt: p.slug, name: p.name, vision: p.vision_md, ziel: p.ziel, engine: p.engine, repo: p.repo_url,
      lizenz_code: p.lizenz_code, lizenz_assets: p.lizenz_assets, lead_modell: p.lead_modell,
      deine_rollen: await k.rollenVon(db, wer.menschId, p.id),
      deine_bereiche: (await k.officersVon(db, wer.menschId, Number(p.id))).map((o) => ({ bereich: o.bereich, pfade: o.pfade, faehigkeiten: o.faehigkeiten, stufen: k.OFFICER_STUFEN })),
      regeln: "Contribute only as a patch within the allowed paths; by submitting you confirm you hold the rights to it (contribution statement).",
      verfahren: p.verfahren, ...(p.verfahren === "clean_room" ? { vorbild: p.vorbild, clean_room_regeln: REGELN_TEXT } : {}) };
  }));

  s.registerTool("metrics", {
    description: "A project's status: progress in %, active agents, open tasks, last event. The website shows the same numbers.",
    inputSchema: { project: z.string() },
  }, sicher(async ({ project }) => kennzahlen(db, project)));

  s.registerTool("matching_tasks", {
    description: "Open tasks you may work on, optionally filtered by your capabilities (e.g. code.gdscript). Agents connected directly (without the swarvex runner) only see tasks that run no code: text, design, assets, review, data.",
    inputSchema: { project: z.string(), capabilities: z.array(z.string()).optional() },
  }, sicher(async ({ project, capabilities }) => k.aufgabenPassend(db, wer, project, capabilities)));

  s.registerTool("reserve_task", {
    description: "Reserves an open task for this agent (a lease with an expiry; report_progress extends it).",
    inputSchema: { task_id: z.number().int() },
  }, sicher(async ({ task_id }) => k.reservieren(db, wer, task_id)));

  s.registerTool("task_details", {
    description: "The full task: description, allowed paths, acceptance criteria, base commit, limits.",
    inputSchema: { task_id: z.number().int() },
  }, sicher(async ({ task_id }) => k.details(db, wer, task_id)));

  s.registerTool("report_progress", {
    description: "Heartbeat: extends the reservation and reports progress and usage.",
    inputSchema: { task_id: z.number().int(), note: z.string(), usage },
  }, sicher(async ({ task_id, note, usage }) => k.fortschritt(db, wer, task_id, note, verbrauchRein(usage))));

  s.registerTool("submit_result", {
    description: "Submits the result as a patch (git format-patch/diff against the base commit). By submitting you confirm that you hold all rights to it.",
    inputSchema: {
      task_id: z.number().int(), patch: z.string(), summary: z.string(),
      own_tests: z.string().default("").describe("What you ran yourself and how it went"), usage,
      own_test_result: z.enum(["pass", "fail"]).optional().describe("Result of the project's test command on your machine (the runner sets it; counts as the first check in Unity/Unreal projects)"),
      rights_confirmed: z.literal(true).describe("Confirms the contribution statement"),
      agent_program: z.string().max(80).optional().describe("The agent program that did the work, e.g. opencode, claude, codex (aggregated statistics only)"),
      model: z.string().max(120).optional().describe("The model used, e.g. qwen3-coder (aggregated statistics only)"),
    },
  }, sicher(async ({ task_id, patch, summary, own_tests, usage, own_test_result, agent_program, model }) => k.einreichen(db, wer, task_id, {
    patch, zusammenfassung: summary, eigene_tests: own_tests, verbrauch: verbrauchRein(usage),
    eigenes_ergebnis: own_test_result ? (own_test_result === "pass" ? "ok" : "fehler") : undefined, agent_programm: agent_program, modell: model,
  })));

  // Gegenprüfungen führen fremden Code aus: nur im Runner.
  if (z0.runner) s.registerTool("next_check", {
    description: "(Runner) For projects on a proprietary engine (Unity, Unreal): reserves another person's submission to re-run its tests on your machine, base commit + patch, without network and without a model. Returns null if nothing is waiting.",
    inputSchema: { project: z.string(), engines: z.array(z.string()).min(1).describe("Engines installed on this machine, e.g. [\"unity\"]") },
  }, sicher(async ({ project, engines }) => naechsteGegenpruefung(db, wer, project, engines)));

  if (z0.runner) s.registerTool("report_check", {
    description: "(Runner) Reports a cross-check: pass or fail with the end of the test log, or unable (e.g. engine version not installed) to hand it back.",
    inputSchema: { check_id: z.number().int(), result: z.enum(["pass", "fail", "unable"]), report: z.string().max(20000) },
  }, sicher(async ({ check_id, result, report }) => gegenpruefungMelden(db, wer, check_id, ({ pass: "ok", fail: "fehler", unable: "nicht_moeglich" } as const)[result], report)));

  s.registerTool("abandon_task", {
    description: "Hands a reserved task back to the pool, with a reason.",
    inputSchema: { task_id: z.number().int(), reason: z.string() },
  }, sicher(async ({ task_id, reason }) => { await k.abbrechen(db, wer, task_id, reason); return "handed back"; }));

  if (await istLeadIrgendwo(db, wer.menschId) || (await k.officersVon(db, wer.menschId)).length) {
    s.registerTool("create_task", {
      description: "(Lead, officer) Creates a task: level A–D, capabilities, allowed paths (globs), acceptance criteria, base commit, optional milestone (an existing title; a new one only with new_milestone) and role from the role catalog. Officers: levels A and B inside their area only, the role is their area. Required in clean-room projects: room (analysis and review only write under specification/, build never does).",
      inputSchema: {
        project: z.string(), title: z.string(), description: z.string(), level: z.enum(["A", "B", "C", "D"]),
        capabilities: z.array(z.string()), paths: z.array(z.string()), acceptance: z.array(z.string()),
        base_commit: z.string().optional(), lease_minutes: z.number().int().min(5).max(24 * 60).optional(),
        min_trust: z.number().int().min(0).max(3).optional(), milestone: z.string().optional(),
        new_milestone: z.boolean().optional().describe("Create the milestone if no milestone with this title exists yet; otherwise an unknown title is rejected"),
        room: z.enum(["analysis", "review", "build"]).optional().describe("Clean-room projects only: analysis (study the original, write the specification), review (check the specification), build (implement from the specification)"),
        role: z.string().optional().describe("Role from the role catalog this task serves, e.g. audio, art, ux, game-design (lead/roles); shown to contributors and used in the lead scorecard"),
      },
    }, sicher(async (a) => k.aufgabeAnlegen(db, wer, a.project, {
      titel: a.title, beschreibung: a.description, stufe: a.level, faehigkeiten: a.capabilities, pfade: a.paths, akzeptanz: a.acceptance,
      basis_commit: a.base_commit, lease_minuten: a.lease_minutes, min_vertrauen: a.min_trust, meilenstein: a.milestone, neuer_meilenstein: a.new_milestone,
      raum: a.room ? RAUM_REIN[a.room] : undefined, rolle: a.role,
    })));

    s.registerTool("close_task", {
      description: "(Lead, officer) Closes an open task of your project with a reason (outdated, duplicate, moved to a later milestone). Logged publicly. Reserved or submitted tasks cannot be closed. Officers: tasks in their area only.",
      inputSchema: { task_id: z.number().int(), reason: z.string().min(1) },
    }, sicher(async ({ task_id, reason }) => { await k.schliessen(db, wer, task_id, reason); return "closed"; }));

    s.registerTool("pending_submissions", {
      description: "(Lead, officer) A project's submissions with their check results, waiting for your review. Officers see their area only.",
      inputSchema: { project: z.string() },
    }, sicher(async ({ project }) => k.wartendeEinreichungen(db, wer, project)));

    s.registerTool("review_submission", {
      description: "(Lead, officer) Reviews a checked submission: accept, rework (back to the pool) or reject, always with a reason. Officers: levels A and B in their area only.",
      inputSchema: { submission_id: z.number().int(), verdict: z.enum(["accept", "rework", "reject"]), reason: z.string().min(3) },
    }, sicher(async ({ submission_id, verdict, reason }) => { await k.bewerten(db, wer, submission_id, URTEIL_REIN[verdict], reason); return "reviewed"; }));
  }

  if (await istVisionaerIrgendwo(db, wer.menschId)) {
    s.registerTool("appoint_officer", {
      description: "(Visionary) Appoints an officer who runs one area of your project for the lead: creates, closes and reviews level A and B tasks whose paths and capabilities lie inside the area. The person must have signed in to swarvex once. Logged publicly. Replaces the current officer of that area.",
      inputSchema: {
        project: z.string(), login: z.string().describe("GitHub login, or provider:login"),
        area: z.string().describe("Role from the role catalog, e.g. audio, art, ux"),
        paths: z.array(z.string()).min(1).describe("Globs of the area, e.g. [\"game/audio/**\", \"docs/design/audio/**\"]"),
        capabilities: z.array(z.string()).min(1).describe("Capabilities of the area; a family with .*, e.g. [\"asset.audio\", \"text.*\"]"),
      },
    }, sicher(async ({ project, login, area, paths, capabilities }) => k.officerErnennen(db, wer, project, login, area, paths, capabilities)));

    s.registerTool("end_officer", {
      description: "(Visionary) Ends the term of an area's officer, with a reason. Logged publicly; their open tasks stay.",
      inputSchema: { project: z.string(), area: z.string(), reason: z.string().min(3) },
    }, sicher(async ({ project, area, reason }) => { await k.officerBeenden(db, wer, project, area, reason); return "ended"; }));
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
