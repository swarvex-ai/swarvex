// Public interfaces speak English (MCP tools, HTTP API, event types). Internally names are still German
// (database, code); this module translates at the boundary, in one place. A test makes sure no German field
// name leaks out (test/aussen.test.ts). Unknown keys pass through unchanged, so dynamic map keys keep working.

/** German field name → English field name. */
export const FELDER: Record<string, string> = {
  // projects and metrics
  projekt: "project", projekte: "projects", sprache: "language", uebersetzungen: "translations", kritisch_vertrauen: "critical_path_trust", vision_kurz: "vision_short", vision_veroeffentlicht: "vision_published",
  vision: "vision", vision_md: "vision_md", lead_modell: "lead_model", lizenz: "license", lizenz_code: "license_code",
  lizenz_assets: "license_assets", ziel: "goal", fortschritt_prozent: "progress_percent", aufgaben_geplant: "tasks_planned", geplant: "planned", fertig: "done", schaetzung: "estimate", geschaetzt_am: "estimated_at", reihenfolge: "order", vorher: "before", fortschritt_berechnung: "progress_calculation",
  meilensteine: "milestones", meilenstein: "milestone", agenten_aktiv: "agents_active", agenten_aktiv_nach_art: "agents_active_by_kind",
  mitstreiter_menschen: "contributor_people", mitstreiter_agenten: "contributor_agents", aufgaben_offen: "tasks_open",
  spenden_monat: "donated_this_month", zuletzt: "last", sucht: "wants", woche: "week", stand: "as_of", summe: "total",
  tokens_woche: "tokens_week", angenommen_woche: "accepted_week", repo: "repo", repo_url: "repo_url", repo_git: "repo_git", repo_verbunden: "repo_connected", entwurf: "draft", ueber: "via",
  // contributions (beitraege.ts)
  // swarm balance (bilanz.ts)
  bilanz: "swarm_balance", gesamt: "total", aufgaben_fertig: "tasks_done",
  ein_agent: "single_agent", stunden: "hours", beschleunigung: "speedup", 
  vier_augen_prozent: "four_eyes_percent", pruefstrasse_prozent: "pipeline_passed_percent", abgeschlossen: "completed", vergleich: "comparison",
  kalender_tage: "calendar_days", beschreibung_en: "description_en", einheiten: "units", einheiten_je_tag: "units_per_day", gleichzeitig: "concurrent", agentenstunden_je_tag: "agent_hours_per_day", durchsatz_faktor: "throughput_factor", beleg: "evidence", abhaengig_von: "depends_on", abhaengig_eins_von: "depends_on_any", folgeaufgaben: "unlocks", variante_von: "variant_of", varianten: "variants", folgeaufgaben_mit: "follow_ups_with", hinweis: "hint", tage: "days", anzahl: "count", fremdpruefungen: "peer_reviews", zustimmungen: "approvals", automatisch_ab: "automatic_from", automatisch_angenommen: "accepted_automatically", automatisch: "automatic", fremdpruefer: "peer_reviewers", vom_lead: "by_lead", eigener_beitrag: "own_submission", zurueckgegeben: "sent_back", ohne_fremdbewertung: "without_peer_review", neue_basis: "new_base", wartet_auf: "waiting_for", zum_bewerten: "to_review", angebot: "supply", blockiert: "blocked", aktiv: "active", nachschub_noetig: "backlog_low", zeitfluss: "time_flow", jetzt: "now", warten_auf_agent: "waiting_for_agent", bearbeitung: "working", bewertung: "review", merge: "merge", warten_auf_pruefung: "waiting_for_check", aelteste_pruefung_h: "oldest_check_hours", aelteste_bewertung_h: "oldest_review_hours", warten_auf_fremdbewertung: "awaiting_peer_review", aelteste_fremdbewertung_h: "oldest_peer_review_hours", warten_auf_merge: "waiting_for_merge", aelteste_merge_h: "oldest_merge_hours",
  qualitaet: "quality", regressionen: "regressions", zug_rot: "merge_train_failed", kosten: "cost", tokens_je_einheit: "tokens_per_unit",
  // lead power (leadkraft.ts, E-39)
  gesuch_id: "position_id", bewerbung: "application", bewerbungen: "applications", bewerbung_id: "application_id", lead_gesuche: "lead_positions",
  schreiben: "write", halter: "holder", bereitschaft: "standby", lead_kraft: "lead_power", signale: "signals", backlog_leer: "backlog_empty",
  idee: "idea", ideen: "ideas", kurz: "short", warum: "why_now", schwarm_vorteil: "why_a_swarm", formwerk: "ready_assets", katalog_treffer: "catalog_matches", katalog_begruendung: "catalog_checked", quellen: "sources", bereiche: "areas", uebernommen: "taken_by", lead_gesucht: "lead_wanted",
  bedarf_id: "request_id", bedarf: "requests", an_uns: "incoming", von_uns: "outgoing", von_aufgabe: "waiting_task_id",
  wartet_auf_lead: "waiting_for_lead", d_stau: "level_d_backlog", meine_bewerbungen: "my_applications", teilen_link: "share_link",
  lead_dienst: "lead_service", lead_dienste: "lead_services", lead_projekt: "lead_project", gebunden: "bound", letzte_runde: "last_cycle", gesehen: "seen", autor: "author", gueltig_bis: "valid_until",
  lead_last: "lead_load", kontext: "context", index: "index", probe: "trial", stufe_c: "level_c", erlaubt: "allowed", engstellen: "bottleneck_files", konflikte: "conflicts", eigene_einheiten: "own_units", aktionen_je_einheit: "actions_per_unit", leistung: "performance", versuche: "attempts", zurueck: "sent_back",
  pruefung_rot: "check_failed", erfolg_prozent: "success_percent", weiter: "continue",
  gemerged: "merged", plus: "added", minus: "removed", sorte: "kind", sorten: "kinds", dateien_gesamt: "files_total", commit_url: "commit_url", zeilen_code: "lines_of_code",
  bilder: "images", audio: "audio", modelle_3d: "models_3d", videos: "videos", texte: "texts", reviews: "reviews", modelle: "models", agent_programm: "agent_program",
  modell: "model", tokens_je_annahme: "tokens_per_accepted", pfad: "path", lfs: "lfs", binaer: "binary", vorschau: "preview",
  verfahren: "procedure", vorbild: "original", forschung: "research", forschung_regeln: "research_rules",
  // model ranking (modelle.ts)
  ok_commit: "playable_commit", ok_zeit: "playable_since", letzter_commit: "last_commit", hat_vorschau: "has_preview", hat_doku: "has_docs", vorschau_url: "preview_url", autor_modell: "author_model", anderes_modell: "different_model",
  // peer reviews by runners (schwarmpruefung.ts, 10 Nov 2026)
  pruefpunkte: "review_points", antworten: "answers", bilder_gesehen: "images_seen", zustimmen_moeglich: "approve_allowed", bild_adresse: "image_url",
  modellfamilie: "model_family", aenderungen: "change_requests", widerlegt: "refuted", widerlegt_prozent: "refuted_percent", pruefer: "reviewers", behebt: "fixes", punkte: "points",
  // weekly report (wochenbericht.ts)
  bis: "until", vorwoche: "previous_week", angelegt: "created", neue_menschen: "new_people", offene_freigaben: "open_approvals", hinweise: "hints", wochenberichte: "weekly_reports",
  // public profile (profil.ts)
  abzeichen: "badges", profil_oeffentlich: "public_profile", oeffentlich: "public",
  // trust levels (vertrauen.ts)
  bereichs_vorschlaege: "area_lead_proposals", bereichs_leads: "area_lead_appointments", einladungen: "invitations", automatik: "automatic_appointment", deine_einladungen: "your_invitations",
  gemerged_ab_b: "merged_level_b_plus", naechste: "next", fehlt: "missing",
  dein_vertrauen: "your_trust", vertrauen: "trust", nach: "to", anlass: "occasion",
  // preview feedback (feedback.ts)
  gut: "good", hakt: "problems", geraet: "device", neu: "new", rueckmeldungen: "feedback",
  nacharbeit: "rework", nacharbeit_bis: "rework_until", abgelehnt: "rejected", untere_grenze_prozent: "lower_bound_percent", pruefung_ok_prozent: "check_passed_percent", tokens_median_je_annahme: "median_tokens_per_accepted",
  familien: "kinds", familie: "kind", programme: "programs", programm: "program", anteil_prozent: "share_percent", ausgeblendet: "hidden", methode: "method",
  zeitraum_tage: "window_days", mindestens_bewertet: "min_reviewed", mindestens_mitstreiter: "min_contributors", mindestens_je_zeile: "min_per_row", schluessel: "key", weiterfuehren_regeln: "continuation_rules", deine_rollen: "your_roles", regeln: "rules", clean_room_regeln: "clean_room_rules",
  menschen_je_raum: "people_per_room", trennung_eingehalten: "separation_kept", verletzungen: "violations",
  aufgaben: "tasks", beitragende: "contributors", ereignisse: "events", protokoll: "log", eintraege: "entries", kopf: "head",
  veroeffentlicht: "published", ok: "ok",
  // tasks, leases, submissions
  aufgabe_id: "task_id", titel: "title", beschreibung: "description", stufe: "level", faehigkeiten: "capabilities",
  pfade: "paths", akzeptanz: "acceptance", basis_commit: "base_commit", lease_minuten: "lease_minutes", min_vertrauen: "min_trust",
  zustand: "state", erstellt: "created", raum: "room", projekt_id: "project_id", meilenstein_id: "milestone_id",
  grenzen: "limits", letzte_pruefung: "last_check", letzte_bewertung: "last_review", fruehere_rueckgaben: "earlier_hand_backs", zeilen: "lines", dateien: "files", arbeitet: "working", arbeitet_login: "working_login", arbeitet_profil: "working_profile", beitrag_von: "contribution_by", beitrag_login: "contribution_login", beitrag_profil: "contribution_profile",
  lease_id: "lease_id", laeuft_ab: "expires", einreichung_id: "submission_id", patch_sha256: "patch_sha256",
  abgewiesen: "rejected", entscheidet: "decided_by", zusammenfassung: "summary", eigene_tests: "own_tests", verbrauch: "usage", eingereicht: "submitted",
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
  widerspricht_selbst: "contradicts_own_run", ohne_gegenpruefung: "without_cross_check", grund: "reason", zurueckgehalten: "held_back", installieren_url: "install_url",
  // officers and lead scorecard
  officer_id: "area_lead_id", officers: "area_leads", officer: "area_lead", bereich: "area", deine_bereiche: "your_areas", stufen: "levels", seit: "since",
  fortlaufend: "ongoing", laufend: "ongoing_figures", katalog: "catalogue", arten: "kinds", diese_woche: "this_week", beantwortet: "answered",
  lead_bilanz: "lead_scorecard", bewertet: "reviewed", je_rolle: "by_role", offen: "open", unberuehrt: "untouched_3_days", warten_auf_bewertung: "awaiting_review",
  reservierungen_ohne_ergebnis: "reservations_without_result", einreichungen_30t: "submissions_30d", annahme_prozent: "acceptance_percent",
  nacharbeit_prozent: "rework_percent",
  nachricht_id: "message_id", nachricht_art: "kind", an: "to", von: "from", betreff: "subject", inhalt: "text", antwort: "answer",
  entscheidung: "decision", beantwortet_von: "answered_by", beantwortet_am: "answered_at", an_mich: "to_me", von_mir: "from_me",
  projektart: "project_type", deine_nachrichten: "your_messages",
  zusage_id: "pledge_id", zusagen: "pledges", dienst: "service", verfaellt: "expires", notiz: "note", spender: "donor", verplant: "planned", frei: "available",
  auftrag_id: "brief_id", auftraege: "briefs", weg: "channel", tageslimit: "daily_limit", heute_frei: "left_today", alle_projekte: "all_projects", heute: "today", dienste: "services", anleitung: "instructions", zielpfad: "target_path", dateitypen: "file_types", max_mb: "max_mb",
  dateiname: "file_name", sha256: "sha256", bytes: "bytes", hochgeladen: "uploaded", datei_url: "file_url", abgebrochen: "handed_back", abbruchgrund: "handed_back_reason", gestoppt: "stopped",
  credit_spenden: "credit_donations", zugesagt: "pledged", credits_angenommen: "credits_accepted", credits: "credits",
  schwarm: "swarm", agenten_stunden: "agent_hours", einzel_agent_stunden: "single_agent_hours", kalender_stunden: "calendar_hours",
  schwarm_faktor: "swarm_factor", max_parallel: "max_parallel", token_effizienz_prozent: "token_efficiency_percent", aelteste_h: "oldest_hours", freigaben: "approvals",
  nachrichten_offen: "messages_open", nachrichten_antwort_h_median: "messages_response_hours_median", antwort_h_median: "response_hours_median", officer_empfehlung: "area_lead_recommendation", gruende: "reasons", engpass: "bottleneck",
};

/** Values that are German words, translated per (German) field name. */
export const WERTE: Record<string, Record<string, string>> = {
  zustand: {
    offen: "open", reserviert: "reserved", in_arbeit: "in_progress", eingereicht: "submitted", pruefung: "checking",
    geprueft_ok: "checked_ok", geprueft_fehler: "check_failed", angenommen: "accepted", gemerged: "merged", geschlossen: "closed",
    neu: "new", gesehen: "seen", aufgabe: "task",
    beantwortet: "answered", erledigt: "done", hochgeladen: "uploaded", nacharbeit: "rework", nacharbeit_bis: "rework_until", abgelehnt: "rejected", zurueckgezogen: "withdrawn", beendet: "ended",
  },
  rolle: { visionaer: "visionary", lead: "lead", mitstreiter: "contributor", bereichs_lead: "area_lead" },
  urteil: { annehmen: "accept", nacharbeit: "rework", ablehnen: "reject", zustimmen: "approve", aenderungen: "request_changes" },
  ergebnis: { ok: "pass", fehler: "fail", nicht_moeglich: "unable" },
  quelle: { projekt: "pipeline", stichprobe: "sample", selbst: "own_run", gegenpruefung: "cross_check" },
  pruefung: { ok: "pass", fehler: "fail" },
  raum: { analyse: "analysis", pruefung: "review", bau: "build" },
  verfahren: { standard: "standard", clean_room: "clean_room" },
  engpass: { lead: "lead", mitstreiter: "contributors", bewertung: "review", fremdbewertung: "peer_review", pruefung: "check", merge: "merge", nachschub: "backlog", abhaengigkeiten: "dependencies" },
  dienst: { andere: "other" },
  projektart: { spiel: "game", app: "app", website: "website", dienst: "service", tool: "tool", bibliothek: "library", plugin: "plugin", forschung: "research", sonstiges: "other" },
  nachricht_art: { frage: "question", freigabe: "approval", info: "info" },
  entscheidung: { freigegeben: "approved", abgelehnt: "declined" },
  art: {
    // event types
    vision_veroeffentlicht: "vision_published", vision_aktualisiert: "vision_updated", projekt_fortlaufend: "project_ongoing_set", uebersetzung_gesetzt: "translation_set", aufgabe_angelegt: "task_created", aufgabe_reserviert: "task_reserved",
    arbeit_begonnen: "work_started", fortschritt: "progress", ergebnis_eingereicht: "result_submitted", einreichung_abgewiesen: "submission_rejected",
    abgebrochen: "abandoned", reservierung_abgelaufen: "reservation_expired", aufgabe_geschlossen: "task_closed", meilenstein_geschaetzt: "milestone_estimated", reservierung_beendet_sperre: "reservation_ended_block",
    reservierung_beendet_abmeldung: "reservation_ended_sign_out", pruefung_gestartet: "check_started", pruefung_beendet: "check_finished",
    zurueck_in_pool: "back_to_pool", projekt_tests_rot: "project_tests_red", zur_bewertung_freigegeben: "released_for_review", vorschau_gebaut: "preview_built", vorschau_feedback: "preview_feedback", bewertet: "reviewed", gemerged: "merged", mitstreiter_beigetreten: "contributor_joined",
    mensch_registriert: "person_registered", agent_angebunden: "agent_connected", agent_abgemeldet: "agent_signed_out",
    anmeldung_abgewiesen: "sign_in_rejected", anmeldung_ausnahme: "sign_in_exemption", clean_room_trennung: "clean_room_separation", pruefstrasse_abgewiesen: "pipeline_rejected",
    gegenpruefung_reserviert: "cross_check_reserved", gegenpruefung_zurueckgegeben: "cross_check_returned", gegenpruefung_rot: "cross_check_failed", rolle_gesetzt: "role_set", repo_gesetzt: "repo_set", lizenz_gesetzt: "license_set", meldung_eingegangen: "report_received", meldung_bearbeitet: "report_handled",
    lead_dienst_verbunden: "lead_service_bound", lead_dienst_uebernommen: "lead_service_took_over", lead_gesuch_geoeffnet: "lead_position_opened",
    lead_gesuch_geschlossen: "lead_position_closed", bereichs_lead_beworben: "area_lead_applied",
    bedarf_gemeldet: "request_sent", bedarf_angenommen: "request_accepted", bedarf_abgelehnt: "request_declined",
    officer_ernannt: "area_lead_appointed", officer_beendet: "area_lead_ended", bereich_uebernommen: "area_taken", bereich_abgegeben: "area_left", stufe_c_freigabe: "level_c_released", vertrauen_geaendert: "trust_changed", regression_gemeldet: "regression_reported", freigabe_angefragt: "approval_requested", freigabe_entschieden: "approval_decided",
    zusage_eingegangen: "pledge_received", zusage_beendet: "pledge_ended", zusage_geaendert: "pledge_changed", spendenauftrag_begonnen: "donation_brief_started", spendenauftrag_abgebrochen: "donation_brief_abandoned", spendenauftrag_angelegt: "donation_brief_created", spende_hochgeladen: "donation_uploaded", spende_bewertet: "donation_reviewed",
    fremdpruefung: "peer_review", pruefpunkte_gesetzt: "review_points_set", vom_lead_erledigt: "done_by_lead", merge_konflikt: "merge_conflict", merge_zug_rot: "merge_train_failed", mensch_gesperrt: "person_blocked", mensch_entsperrt: "person_unblocked", projekt_gesperrt: "project_blocked", projekt_entsperrt: "project_unblocked",
  },
};
WERTE.sorte = { code: "code", test: "test", bild: "image", audio: "audio", video: "video", "3d": "3d", text: "text", daten: "data", sonstiges: "other" };
WERTE.sorten = WERTE.sorte;
// Fähigkeitsfamilie (analytik.familie): erster Teil der Fähigkeit, mehrere Familien = gemischt
WERTE.familie = { daten: "data", gemischt: "mixed", keine: "none" };
WERTE.programm = { unbekannt: "unknown" };
WERTE.rollen = WERTE.rolle;
WERTE.deine_rollen = WERTE.rolle;

/** English → German, for inputs (e.g. MCP tool arguments). */
export const umkehren = (karte: Record<string, string>) => Object.fromEntries(Object.entries(karte).map(([de, en]) => [en, de]));

/** References like "aufgabe:12" or "mensch:3" (event subject and actor). */
const BEZUG: Record<string, string> = { aufgabe: "task", meilenstein: "milestone", mensch: "person", projekt: "project", meldung: "report", einreichung: "submission", nachricht: "message", zusage: "pledge", spendenauftrag: "donation_brief",
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
