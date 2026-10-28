// Public interfaces speak English (MCP tools, HTTP API, event types). Internally names are still German
// (database, code); this module translates at the boundary, in one place. A test makes sure no German field
// name leaks out (test/aussen.test.ts). Unknown keys pass through unchanged, so dynamic map keys keep working.

/** German field name → English field name. */
export const FELDER: Record<string, string> = {
  // projects and metrics
  projekt: "project", projekte: "projects", vision_kurz: "vision_short", vision_veroeffentlicht: "vision_published",
  vision: "vision", vision_md: "vision_md", lead_modell: "lead_model", lizenz: "license", lizenz_code: "license_code",
  lizenz_assets: "license_assets", ziel: "goal", fortschritt_prozent: "progress_percent", fortschritt_berechnung: "progress_calculation",
  meilensteine: "milestones", meilenstein: "milestone", agenten_aktiv: "agents_active", agenten_aktiv_nach_art: "agents_active_by_kind",
  mitstreiter_menschen: "contributor_people", mitstreiter_agenten: "contributor_agents", aufgaben_offen: "tasks_open",
  spenden_monat: "donated_this_month", zuletzt: "last", sucht: "wants", woche: "week", stand: "as_of", summe: "total",
  tokens_woche: "tokens_week", angenommen_woche: "accepted_week", repo: "repo", repo_url: "repo_url", repo_git: "repo_git",
  verfahren: "procedure", vorbild: "original", deine_rollen: "your_roles", regeln: "rules", clean_room_regeln: "clean_room_rules",
  menschen_je_raum: "people_per_room", trennung_eingehalten: "separation_kept", verletzungen: "violations",
  aufgaben: "tasks", beitragende: "contributors", ereignisse: "events", protokoll: "log", eintraege: "entries", kopf: "head",
  veroeffentlicht: "published", ok: "ok",
  // tasks, leases, submissions
  aufgabe_id: "task_id", titel: "title", beschreibung: "description", stufe: "level", faehigkeiten: "capabilities",
  pfade: "paths", akzeptanz: "acceptance", basis_commit: "base_commit", lease_minuten: "lease_minutes", min_vertrauen: "min_trust",
  zustand: "state", erstellt: "created", raum: "room", projekt_id: "project_id", meilenstein_id: "milestone_id",
  grenzen: "limits", zeilen: "lines", dateien: "files", arbeitet: "working", beitrag_von: "contribution_by",
  lease_id: "lease_id", laeuft_ab: "expires", einreichung_id: "submission_id", patch_sha256: "patch_sha256",
  abgewiesen: "rejected", zusammenfassung: "summary", eigene_tests: "own_tests", verbrauch: "usage", eingereicht: "submitted",
  pruefung: "check", bericht: "report", urteil: "verdict", begruendung: "reason", ergebnis: "result",
  anfragen: "requests", sekunden: "seconds", tokens: "tokens", agenten: "agents", menschen: "people", angenommen: "accepted",
  einreichungen: "submissions", mensch: "person", agent: "agent", lead_agent: "lead_agent", einreichung: "submission",
  // events
  seq: "seq", zeit: "time", wer: "who", art: "type", bezug: "subject", akteur: "actor", daten: "data",
  // account
  login: "login", anbieter: "provider", vertrauensstufe: "trust_level", rollen: "roles", rolle: "role",
  client_id: "client_id", abgemeldet: "signed_out", angemeldet: "signed_in", reservierungen: "reservations",
  projekt_name: "project_name", beitraege: "contributions", aufgabe: "task", clean_room: "clean_room",
  // log heads
  hash: "hash", jws: "jws",
  // cross-checks (proprietary engines)
  gegenpruefung: "cross_check", gegenpruefung_id: "check_id", quelle: "source", engine: "engine",
  widerspricht_selbst: "contradicts_own_run", ohne_gegenpruefung: "without_cross_check", grund: "reason",
};

/** Values that are German words, translated per (German) field name. */
export const WERTE: Record<string, Record<string, string>> = {
  zustand: {
    offen: "open", reserviert: "reserved", in_arbeit: "in_progress", eingereicht: "submitted", pruefung: "checking",
    geprueft_ok: "checked_ok", geprueft_fehler: "check_failed", angenommen: "accepted", gemerged: "merged", geschlossen: "closed",
  },
  rolle: { visionaer: "visionary", lead: "lead", mitstreiter: "contributor" },
  urteil: { annehmen: "accept", nacharbeit: "rework", ablehnen: "reject" },
  ergebnis: { ok: "pass", fehler: "fail", nicht_moeglich: "unable" },
  quelle: { projekt: "pipeline", stichprobe: "sample", selbst: "own_run", gegenpruefung: "cross_check" },
  pruefung: { ok: "pass", fehler: "fail" },
  raum: { analyse: "analysis", pruefung: "review", bau: "build" },
  verfahren: { standard: "standard", clean_room: "clean_room" },
  art: {
    // event types
    vision_veroeffentlicht: "vision_published", aufgabe_angelegt: "task_created", aufgabe_reserviert: "task_reserved",
    arbeit_begonnen: "work_started", fortschritt: "progress", ergebnis_eingereicht: "result_submitted", einreichung_abgewiesen: "submission_rejected",
    abgebrochen: "abandoned", reservierung_abgelaufen: "reservation_expired", reservierung_beendet_sperre: "reservation_ended_block",
    reservierung_beendet_abmeldung: "reservation_ended_sign_out", pruefung_gestartet: "check_started", pruefung_beendet: "check_finished",
    zurueck_in_pool: "back_to_pool", bewertet: "reviewed", gemerged: "merged", mitstreiter_beigetreten: "contributor_joined",
    mensch_registriert: "person_registered", agent_angebunden: "agent_connected", agent_abgemeldet: "agent_signed_out",
    anmeldung_abgewiesen: "sign_in_rejected", clean_room_trennung: "clean_room_separation", pruefstrasse_abgewiesen: "pipeline_rejected",
    gegenpruefung_reserviert: "cross_check_reserved", gegenpruefung_zurueckgegeben: "cross_check_returned", rolle_gesetzt: "role_set", repo_gesetzt: "repo_set", lizenz_gesetzt: "license_set", meldung_eingegangen: "report_received", meldung_bearbeitet: "report_handled",
    mensch_gesperrt: "person_blocked", mensch_entsperrt: "person_unblocked", projekt_gesperrt: "project_blocked", projekt_entsperrt: "project_unblocked",
  },
};
WERTE.rollen = WERTE.rolle;
WERTE.deine_rollen = WERTE.rolle;

/** English → German, for inputs (e.g. MCP tool arguments). */
export const umkehren = (karte: Record<string, string>) => Object.fromEntries(Object.entries(karte).map(([de, en]) => [en, de]));

/** References like "aufgabe:12" or "mensch:3" (event subject and actor). */
const BEZUG: Record<string, string> = { aufgabe: "task", mensch: "person", projekt: "project", meldung: "report", einreichung: "submission",
  agent: "agent", konto: "account", system: "system", pruefstrasse: "pipeline", unbekannt: "unknown" };
const bezugUebersetzen = (v: string) => v.replace(/^([a-z]+)(?=:|$)/, (p) => BEZUG[p] ?? p);

/** Usage reported by runners: English keys outside, German keys in the database. */
const VERBRAUCH_REIN: Record<string, string> = { requests: "anfragen", seconds: "sekunden" };
export const verbrauchRein = (v: Record<string, number>) => Object.fromEntries(Object.entries(v).map(([k, n]) => [VERBRAUCH_REIN[k] ?? k, n]));

function wertUebersetzen(feld: string, wert: unknown): unknown {
  if ((feld === "bezug" || feld === "akteur" || feld === "wer") && typeof wert === "string") return bezugUebersetzen(wert);
  const karte = WERTE[feld];
  if (!karte) return nachAussen(wert);
  if (typeof wert === "string") return karte[wert] ?? wert;
  if (Array.isArray(wert)) return wert.map((w) => (typeof w === "string" ? (karte[w] ?? w) : nachAussen(w)));
  return nachAussen(wert);
}

/** Translates a result for the outside: field names and known German values, recursively. */
export function nachAussen(wert: unknown): unknown {
  if (Array.isArray(wert)) return wert.map(nachAussen);
  if (wert === null || typeof wert !== "object" || wert instanceof Date) return wert;
  return Object.fromEntries(Object.entries(wert as Record<string, unknown>).map(([k, v]) => [FELDER[k] ?? k, wertUebersetzen(k, v)]));
}
