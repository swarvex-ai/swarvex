// Dienste für Zusagen mit API-Schlüssel (migrations/067): Schlüssel prüfen, Preis eines Auftrags vorab berechnen,
// Auftrag erzeugen. Nur Dienste, deren API dieselben Credits verbraucht wie das Abo (die verfallen sonst):
//   Meshy       Text/Bild → 3D, Text → Bild. API-Schlüssel gibt es nur in bezahlten Tarifen (die erlauben kommerzielle
//               Nutzung); jede Aufgabe meldet consumed_credits, fehlgeschlagene werden erstattet.
//   ElevenLabs  Geräusche, Sprache. Der Spender kann den Schlüssel selbst begrenzen (Rechte, Credit-Limit, Ablauf);
//               der Verbrauch ergibt sich aus character_count vorher/nachher. Kostenloser Tarif: nicht kommerziell, abgelehnt.
// Nicht dabei (Stand 17.11.2026): Runway, Leonardo, Kling, Higgsfield (API-Guthaben getrennt vom Abo), Suno (keine
// öffentliche API), Midjourney (keine API, Automatisierung verboten).
// Der Schlüssel geht nur an die API des Dienstes, nie an Download-Adressen, und steht in keiner Fehlermeldung.
import { z } from "zod";

export const SCHLUESSEL_DIENSTE = ["meshy", "elevenlabs"] as const;
export type SchluesselDienst = (typeof SCHLUESSEL_DIENSTE)[number];

const MESHY = () => process.env.MESHY_API_BASIS ?? "https://api.meshy.ai";
const ELEVEN = () => process.env.ELEVENLABS_API_BASIS ?? "https://api.elevenlabs.io";
const POLL_MS = () => Number(process.env.SPENDEN_POLL_MS ?? 5000);
const AUFGABE_MAX_MS = 25 * 60_000;

// ── Was der Lead bestellen kann ──────────────────────────────────────────────────────────────────────────────────────
const MODELL_3D = z.enum(["latest", "meshy-7.1", "meshy-6", "meshy-6-lite"]);
const BILD_MODELLE = { "nano-banana": 3, "nano-banana-2": 6, "nano-banana-2-lite": 3, "nano-banana-2-1": 4, "nano-banana-pro": 9, "gpt-image-2": 9 } as const;
const geometrie = {
  ai_model: MODELL_3D.default("latest"),
  texture: z.boolean().default(true).describe("Textured model (default); false: mesh only, cheaper"),
  texture_prompt: z.string().max(800).optional(),
  enable_pbr: z.boolean().default(true).describe("Metallic, roughness and normal maps"),
  texture_resolution: z.enum(["2k", "4k"]).default("2k"),
  target_polycount: z.number().int().min(100).max(300000).optional().describe("Remeshes to about this many faces"),
  topology: z.enum(["triangle", "quad"]).optional(),
  pose_mode: z.enum(["a-pose", "t-pose"]).optional().describe("For characters that will be rigged"),
};
export const ERZEUGUNG = z.discriminatedUnion("kind", [
  z.object({ kind: z.literal("text_to_3d"), prompt: z.string().min(3).max(800), ...geometrie }),
  z.object({ kind: z.literal("image_to_3d"), image_from_brief: z.number().int().describe("Brief whose image result (png, jpg, webp) is the input"), ...geometrie }),
  z.object({ kind: z.literal("text_to_image"), prompt: z.string().min(3).max(2000), ai_model: z.enum(Object.keys(BILD_MODELLE) as [keyof typeof BILD_MODELLE]).default("nano-banana-2"),
    aspect_ratio: z.enum(["1:1", "16:9", "9:16", "4:3", "3:4"]).default("1:1"), remove_background: z.boolean().default(false) }),
  z.object({ kind: z.literal("sound_effect"), text: z.string().min(3).max(450), duration_seconds: z.number().min(0.5).max(30),
    prompt_influence: z.number().min(0).max(1).optional() }),
  z.object({ kind: z.literal("text_to_speech"), text: z.string().min(1).max(3000), voice_id: z.string().regex(/^[A-Za-z0-9]{10,40}$/),
    model_id: z.enum(["eleven_multilingual_v2", "eleven_v3", "eleven_flash_v2_5", "eleven_turbo_v2_5"]).default("eleven_multilingual_v2") }),
]);
export type Erzeugung = z.infer<typeof ERZEUGUNG>;

const ARTEN: Record<SchluesselDienst, Erzeugung["kind"][]> = {
  meshy: ["text_to_3d", "image_to_3d", "text_to_image"],
  elevenlabs: ["sound_effect", "text_to_speech"],
};
export const artenVon = (d: string) => ARTEN[d as SchluesselDienst] ?? [];

/** Dateitypen des Ergebnisses: der Auftrag nennt sie, damit Lead und Prüfung wissen, was kommt. */
export function dateitypenVon(e: Erzeugung): string[] {
  if (e.kind === "text_to_3d" || e.kind === "image_to_3d") return ["glb"];
  if (e.kind === "text_to_image") return ["png", "jpg", "webp"];
  return ["mp3"];
}

/** Preis vorab, in Credits des Dienstes (Meshy-Preisliste 17.11.2026; ElevenLabs mit Reserve nach oben). */
export function preis(e: Erzeugung): number {
  switch (e.kind) {
    case "text_to_3d": return (e.ai_model === "meshy-6-lite" ? 5 : 20) + (e.texture ? 10 : 0);
    case "image_to_3d": return e.ai_model === "meshy-6-lite" ? (e.texture ? 15 : 5) : (e.texture ? 30 : 20);
    case "text_to_image": return BILD_MODELLE[e.ai_model];
    // Doku nennt 40 Credits je Sekunde (Hilfe-Center 11 für die API): reserviert wird der höhere Wert
    case "sound_effect": return Math.ceil(e.duration_seconds * 40);
    // höchstens 1 Credit je Zeichen (Flash/Turbo kosten die Hälfte)
    case "text_to_speech": return e.text.length;
  }
}

/** Passt die Erzeugung zum Dienst der Zusage? Prüft auch Kombinationen, die der Dienst ablehnen würde. */
export function erzeugungPruefen(dienst: string, roh: unknown): Erzeugung {
  const r = ERZEUGUNG.safeParse(roh);
  if (!r.success) throw new Error(`generation: ${r.error.issues.map((i) => `${i.path.join(".") || "kind"}: ${i.message}`).join("; ")}`);
  const e = r.data;
  if (!artenVon(dienst).includes(e.kind)) throw new Error(`generation: ${dienst} pledges take ${artenVon(dienst).join(", ")}`);
  if ((e.kind === "text_to_3d" || e.kind === "image_to_3d") && e.ai_model === "meshy-6-lite" && e.texture_resolution !== "2k") throw new Error("generation: meshy-6-lite textures only in 2k");
  return e;
}

// ── Fehler ────────────────────────────────────────────────────────────────────────────────────────────────────────
export class DienstFehler extends Error {
  /** schluessel: der Schlüssel gilt nicht mehr (Zusage endet); spaeter: vorübergehend, ohne Pause erneut versuchen */
  readonly art: "schluessel" | "guthaben" | "auftrag" | "spaeter";
  readonly verbraucht: number;
  constructor(message: string, art: DienstFehler["art"], verbraucht = 0) { super(message); this.art = art; this.verbraucht = verbraucht; }
}

async function antwortFehler(dienst: string, r: Response): Promise<DienstFehler> {
  let text = "";
  try { const j = await r.json() as Record<string, unknown>; text = String(j.message ?? (j.detail as Record<string, unknown>)?.message ?? j.detail ?? j.error ?? ""); } catch { /* kein JSON */ }
  text = text.replace(/\b(msy|sk)[-_][A-Za-z0-9_-]{6,}/g, "[key]").slice(0, 300);
  if (r.status === 401) return new DienstFehler(`${dienst} does not accept the key (${r.status})`, "schluessel");
  if (r.status === 403) return new DienstFehler(`${dienst} refused: the key lacks a permission${text ? ` (${text})` : ""}`, "schluessel");
  if (r.status === 402) return new DienstFehler(`${dienst}: not enough credits${text ? ` (${text})` : ""}`, "guthaben");
  if (r.status === 429 || r.status >= 500) return new DienstFehler(`${dienst} is busy or unavailable (${r.status})`, "spaeter");
  return new DienstFehler(`${dienst} refused the brief (${r.status})${text ? `: ${text}` : ""}`, "auftrag");
}

async function rufen(dienst: SchluesselDienst, schluessel: string, pfad: string, init: { method?: string; body?: unknown; roh?: boolean } = {}): Promise<Response> {
  const basis = dienst === "meshy" ? MESHY() : ELEVEN();
  const kopf: Record<string, string> = dienst === "meshy" ? { authorization: `Bearer ${schluessel}` } : { "xi-api-key": schluessel };
  if (init.body !== undefined) kopf["content-type"] = "application/json";
  let r: Response;
  try {
    r = await fetch(basis + pfad, { method: init.method ?? "GET", headers: kopf, body: init.body === undefined ? undefined : JSON.stringify(init.body), signal: AbortSignal.timeout(120_000) });
  } catch {
    throw new DienstFehler(`${dienst} could not be reached`, "spaeter");
  }
  if (!r.ok) throw await antwortFehler(dienst, r);
  return r;
}
const json = async (r: Response) => await r.json() as Record<string, any>;

// ── Schlüssel prüfen ──────────────────────────────────────────────────────────────────────────────────────────────
export type Guthaben = { guthaben: number; verfallend: number | null; erneuert: string | null; tarif: string | null };

/** Prüft den Schlüssel beim Dienst und liest das Guthaben. Wirft DienstFehler mit einer Meldung für den Spender. */
export async function guthabenLesen(dienst: SchluesselDienst, schluessel: string): Promise<Guthaben> {
  if (dienst === "meshy") {
    const b = await json(await rufen("meshy", schluessel, "/openapi/v1/balance"));
    const erneuert = typeof b.monthly_resets_at === "number" ? new Date(b.monthly_resets_at).toISOString().slice(0, 10) : null;
    return { guthaben: Number(b.balance ?? 0), verfallend: typeof b.monthly_credits === "number" ? b.monthly_credits : null, erneuert, tarif: null };
  }
  let s: Record<string, any>;
  try { s = await json(await rufen("elevenlabs", schluessel, "/v1/user/subscription")); } catch (e) {
    if (e instanceof DienstFehler && e.art === "schluessel" && /permission/.test(e.message)) throw new DienstFehler("elevenlabs: the key needs the permission user_read (to read your credits), plus sound_generation and/or text_to_speech", "schluessel");
    throw e;
  }
  const frei = Math.max(0, Number(s.character_limit ?? 0) - Number(s.character_count ?? 0));
  const erneuert = typeof s.next_character_count_reset_unix === "number" ? new Date(s.next_character_count_reset_unix * 1000).toISOString().slice(0, 10) : null;
  return { guthaben: frei, verfallend: frei, erneuert, tarif: s.tier ? String(s.tier) : null };
}

/** Kostenlose Tarife erlauben keine kommerzielle Nutzung: die Ergebnisse dürften nicht unter die Asset-Lizenz. */
export function tarifErlaubt(dienst: SchluesselDienst, g: Guthaben): string | null {
  if (dienst === "elevenlabs" && (!g.tarif || g.tarif === "free")) return "elevenlabs: the free plan allows no commercial use, so its results cannot go under the project's asset licence";
  return null;
}

// ── Erzeugen ──────────────────────────────────────────────────────────────────────────────────────────────────────
export type Ergebnis = { daten: Buffer; endung: string; verbraucht: number };
export type Laden = (url: string, maxBytes: number) => Promise<Buffer>;

/** Lädt ein Ergebnis von der Adresse, die der Dienst nennt: ohne Schlüssel, nur https bei ihm (oder seiner Test-Basis). */
export async function herunterladen(dienst: SchluesselDienst, url: string, maxBytes: number): Promise<Buffer> {
  let u: URL;
  try { u = new URL(url); } catch { throw new DienstFehler(`${dienst} returned no valid download address`, "auftrag"); }
  const basis = new URL(dienst === "meshy" ? MESHY() : ELEVEN());
  const erlaubt = (u.protocol === "https:" && (u.hostname === "meshy.ai" || u.hostname.endsWith(".meshy.ai"))) || u.origin === basis.origin;
  if (!erlaubt) throw new DienstFehler(`${dienst} returned a download address outside its own domain`, "auftrag");
  const r = await fetch(u, { signal: AbortSignal.timeout(300_000) }).catch(() => { throw new DienstFehler("the result could not be downloaded", "spaeter"); });
  if (!r.ok || !r.body) throw new DienstFehler(`the result could not be downloaded (${r.status})`, "spaeter");
  const teile: Buffer[] = [];
  let n = 0;
  for await (const teil of r.body as unknown as AsyncIterable<Uint8Array>) {
    n += teil.length;
    if (n > maxBytes) throw new DienstFehler(`the result is larger than ${Math.round(maxBytes / 1048576)} MB`, "auftrag");
    teile.push(Buffer.from(teil));
  }
  return Buffer.concat(teile);
}

async function meshyWarten(schluessel: string, pfad: string, id: string): Promise<Record<string, any>> {
  const ende = Date.now() + AUFGABE_MAX_MS;
  while (Date.now() < ende) {
    const t = await json(await rufen("meshy", schluessel, `${pfad}/${encodeURIComponent(id)}`));
    if (t.status === "SUCCEEDED") return t;
    if (t.status === "FAILED" || t.status === "CANCELED") {
      const grund = String(t.task_error?.message ?? t.status).replace(/\b(msy|sk)[-_][A-Za-z0-9_-]{6,}/g, "[key]").slice(0, 300);
      throw new DienstFehler(`meshy: the task ${t.status === "FAILED" ? "failed" : "was cancelled"}: ${grund}`, "auftrag", Number(t.consumed_credits ?? 0));
    }
    await new Promise((ok) => setTimeout(ok, POLL_MS()));
  }
  throw new DienstFehler("meshy: the task took longer than 25 minutes", "spaeter");
}

const BILDTYP = (b: Buffer) => b[0] === 0x89 && b[1] === 0x50 ? "png" : b[0] === 0xff && b[1] === 0xd8 ? "jpg" : b.subarray(8, 12).toString("latin1") === "WEBP" ? "webp" : null;

/**
 * Führt einen Auftrag beim Dienst aus. bildLaden liefert für image_to_3d das Bild eines anderen Auftrags.
 * vorPreis: Guthaben-Prüfung vor jedem kostenpflichtigen Schritt (Meshy-Vorschau und -Textur einzeln).
 */
export async function erzeugen(dienst: SchluesselDienst, schluessel: string, e: Erzeugung, maxBytes: number,
  bildLaden: (auftragId: number) => Promise<Buffer>): Promise<Ergebnis> {
  if (dienst === "meshy") {
    let verbraucht = 0;
    const formate = { target_formats: ["glb"] };
    const netz = (x: typeof e & { kind: "text_to_3d" | "image_to_3d" }) => ({
      ...(x.target_polycount ? { should_remesh: true, target_polycount: x.target_polycount } : {}),
      ...(x.topology ? { should_remesh: true, topology: x.topology } : {}), ...(x.pose_mode ? { pose_mode: x.pose_mode } : {}) });
    try {
      if (e.kind === "text_to_3d") {
        const v = await json(await rufen("meshy", schluessel, "/openapi/v2/text-to-3d", { method: "POST", body: { mode: "preview", prompt: e.prompt, ai_model: e.ai_model, ...netz(e), ...formate } }));
        let t = await meshyWarten(schluessel, "/openapi/v2/text-to-3d", String(v.result));
        verbraucht += Number(t.consumed_credits ?? 0);
        if (e.texture) {
          const f = await json(await rufen("meshy", schluessel, "/openapi/v2/text-to-3d", { method: "POST", body: { mode: "refine", preview_task_id: String(v.result), enable_pbr: e.enable_pbr,
            texture_resolution: e.texture_resolution, ...(e.texture_prompt ? { texture_prompt: e.texture_prompt } : {}), ...formate } }));
          t = await meshyWarten(schluessel, "/openapi/v2/text-to-3d", String(f.result));
          verbraucht += Number(t.consumed_credits ?? 0);
        }
        return { daten: await herunterladen("meshy", String(t.model_urls?.glb ?? ""), maxBytes), endung: "glb", verbraucht };
      }
      if (e.kind === "image_to_3d") {
        const bild = await bildLaden(e.image_from_brief);
        const typ = BILDTYP(bild);
        if (!typ) throw new DienstFehler("image_from_brief: that brief's result is not a png, jpg or webp image", "auftrag");
        if (bild.length > 20 * 1048576) throw new DienstFehler("image_from_brief: the image is larger than 20 MB", "auftrag");
        const v = await json(await rufen("meshy", schluessel, "/openapi/v1/image-to-3d", { method: "POST", body: {
          image_url: `data:image/${typ === "jpg" ? "jpeg" : typ};base64,${bild.toString("base64")}`, ai_model: e.ai_model, should_texture: e.texture,
          ...(e.texture ? { enable_pbr: e.enable_pbr, texture_resolution: e.texture_resolution, ...(e.texture_prompt ? { texture_prompt: e.texture_prompt } : {}) } : {}), ...netz(e), ...formate } }));
        const t = await meshyWarten(schluessel, "/openapi/v1/image-to-3d", String(v.result));
        verbraucht += Number(t.consumed_credits ?? 0);
        return { daten: await herunterladen("meshy", String(t.model_urls?.glb ?? ""), maxBytes), endung: "glb", verbraucht };
      }
      if (e.kind === "text_to_image") {
        const v = await json(await rufen("meshy", schluessel, "/openapi/v1/text-to-image", { method: "POST", body: { ai_model: e.ai_model, prompt: e.prompt, aspect_ratio: e.aspect_ratio, remove_background: e.remove_background } }));
        const t = await meshyWarten(schluessel, "/openapi/v1/text-to-image", String(v.result));
        verbraucht += Number(t.consumed_credits ?? 0);
        const daten = await herunterladen("meshy", String(t.image_urls?.[0] ?? ""), maxBytes);
        const typ = BILDTYP(daten);
        if (!typ) throw new DienstFehler("meshy returned no png, jpg or webp image", "auftrag");
        return { daten, endung: typ, verbraucht };
      }
    } catch (f) {
      // Bereits abgebuchte Schritte (z. B. Vorschau fertig, Textur fehlgeschlagen) zählen gegen die Zusage
      if (f instanceof DienstFehler) throw new DienstFehler(f.message, f.art, verbraucht + f.verbraucht);
      throw f;
    }
    throw new DienstFehler(`meshy pledges take ${ARTEN.meshy.join(", ")}`, "auftrag");
  }

  // ElevenLabs: Verbrauch = character_count nachher − vorher (ein Auftrag je Schlüssel gleichzeitig, src/spendenwerk.ts)
  if (e.kind !== "sound_effect" && e.kind !== "text_to_speech") throw new DienstFehler(`elevenlabs pledges take ${ARTEN.elevenlabs.join(", ")}`, "auftrag");
  const vorher = await json(await rufen("elevenlabs", schluessel, "/v1/user/subscription"));
  const frei = Number(vorher.character_limit ?? 0) - Number(vorher.character_count ?? 0);
  // Nie in den kostenpflichtigen Überhang: nur, was im Abrechnungszeitraum noch frei ist
  if (frei < preis(e)) throw new DienstFehler(`elevenlabs: only ${Math.max(0, frei)} credits left in this billing period`, "guthaben");
  const r = e.kind === "sound_effect"
    ? await rufen("elevenlabs", schluessel, "/v1/sound-generation?output_format=mp3_44100_128", { method: "POST", body: { text: e.text, duration_seconds: e.duration_seconds, ...(e.prompt_influence !== undefined ? { prompt_influence: e.prompt_influence } : {}) } })
    : await rufen("elevenlabs", schluessel, `/v1/text-to-speech/${encodeURIComponent(e.voice_id)}?output_format=mp3_44100_128`, { method: "POST", body: { text: e.text, model_id: e.model_id } });
  const daten = Buffer.from(await r.arrayBuffer());
  if (daten.length > maxBytes) throw new DienstFehler(`the result is larger than ${Math.round(maxBytes / 1048576)} MB`, "auftrag");
  const nachher = await json(await rufen("elevenlabs", schluessel, "/v1/user/subscription")).catch(() => null);
  const gemessen = nachher ? Number(nachher.character_count ?? 0) - Number(vorher.character_count ?? 0) : 0;
  // Zählt der Dienst verzögert, zählt die Vorab-Schätzung: lieber zu viel als zu wenig gegen die Zusage
  return { daten, endung: "mp3", verbraucht: gemessen > 0 ? gemessen : preis(e) };
}
