<script lang="ts">
  // „Mein Konto“: Sitzung als HttpOnly-Cookie der API (apps/api/src/konto.ts); die Seite lädt die Daten im Browser.
  import { onMount } from "svelte";
  import Seo from "$lib/components/Seo.svelte";
  import RepoVerbinden from "$lib/components/RepoVerbinden.svelte";
  import TeilenLink from "$lib/components/TeilenLink.svelte";
  import { pfad, sprache, tx, type Hinweis } from "$lib/texte";

  type Agent = { id: number; name: string; client_id: string; created: string; signed_out: string | null; signed_in: boolean;
                 last: string | null; reservations: number; submissions: number; lead_project?: string | null };
  type LeadDienst = { project: string; name: string; active: boolean; seen: string | null; last_cycle: string | null };
  type Datei = { path: string; added: number; removed: number; kind: string; lfs: boolean; binary: boolean; preview: boolean };
  type Beitrag = { submission: number; task: number; title: string; project: string; project_name: string; agent: string;
                   submitted: string; state: string; tokens: number; check: string | null; verdict: string | null; reason: string | null;
                   summary: string; added: number; removed: number; kinds: string[]; files: Datei[]; files_total: number;
                   commit: string | null; commit_url: string | null; agent_program: string | null; model: string | null };
  type Summe = { submissions: number; accepted: number; merged: number; projects: number; tokens: number; lines_of_code: number;
                 images: number; audio: number; models_3d: number; videos: number; texts: number; reviews: number };
  type Modell = { agent_program: string; model: string; submissions: number; accepted: number; acceptance_percent: number; tokens_per_accepted: number | null };
  type Konto = { person: { login: string; provider: string; created: string; trust_level: number; public_profile?: boolean };
                 roles: { project: string; name: string; role: string; repo_connected: boolean; draft: boolean }[]; agents: Agent[]; contributions: Beitrag[];
                 total: Summe; models: Modell[]; approvals: Freigabe[]; pledges: Zusage[]; weekly_reports?: Woche[]; lead_services?: LeadDienst[];
                 lead_power?: { project: string; name: string; share_link: string; lead_positions: Gesuch[]; signals: string[] }[];
                 my_applications?: { id: number; project: string; project_name: string; area: string; created: string; status: string }[];
                 trust?: { level: number; next: { level: number; missing: string[] } | null };
                 area_lead_appointments?: { invitations: Bereich[]; approvals: Bereich[]; automatic_appointment: { project: string; project_name: string; automatic: boolean }[] } };
  type Bereich = { id: number; project: string; project_name: string; area: string; paths: string[]; capabilities: string[];
                   reasons: (string | { application: string; models: string })[]; created: string; login?: string; trust_level?: number; application?: boolean };
  type Gesuch = { id: number; area: string; paths: string[]; capabilities: string[]; text: string; created: string; applications: number };
  const gruendeText = (r: Bereich["reasons"]) => r.filter((x): x is string => typeof x === "string").join("; ");
  const bewerbungVon = (r: Bereich["reasons"]) => r.find((x): x is { application: string; models: string } => typeof x === "object" && x !== null);
  type Woche = { project: string; name: string; week: Record<string, number>; previous_week: { accepted: number; units: number };
                 now: { available: number; blocked: number; active: number; awaiting_review: number }; hints: Hinweis[] };
  type Auftrag = { brief_id: number; title: string; instructions: string; target_path: string; file_types: string[]; max_mb: number; credits: number;
                   state: string; file_name: string | null; uploaded: string | null; reason: string | null; catalog_matches: string[] | null; catalog_checked: string | null };
  type Zusage = { pledge_id: number; project: string | null; project_name: string | null; service: string; credits: number | null; expires: string | null; note: string; state: string;
    channel: "website" | "runner"; daily_limit: number | null; briefs: Auftrag[] };
  type Freigabe = { message_id: number; project: string; project_name: string; subject: string; text: string; tasks: number[]; created: string; from: string };

  const s = $derived(sprache());
  const t = $derived(tx().konto);
  const projektPfad = $derived(tx().projekte.pfad);
  const locale = $derived(tx().projekte.sprache);

  let zustand = $state<"laden" | "gast" | "da" | "fehler">("laden");
  let konto = $state<Konto | null>(null);
  let frage = $state<number | null>(null);
  let arbeitet = $state(false);
  let hinweis = $state("");

  async function laden() {
    try {
      const r = await fetch("/api/account", { cache: "no-store" });
      if (r.status === 401) { r.body?.cancel(); zustand = "gast"; konto = null; return; }
      if (!r.ok) throw new Error(String(r.status));
      konto = await r.json();
      zustand = "da";
    } catch {
      zustand = "fehler";
    }
  }
  onMount(laden);

  async function post(adresse: string) {
    const r = await fetch(adresse, { method: "POST", headers: { "content-type": "application/json" }, body: "{}" });
    if (!r.ok) throw new Error(String(r.status));
    return r.json();
  }

  async function agentAbmelden(a: Agent) {
    arbeitet = true;
    hinweis = "";
    try {
      const { reservations } = await post(`/api/account/agents/${a.id}/sign-out`);
      hinweis = t.abgemeldetOk(a.name, reservations);
      frage = null;
      await laden();
    } catch {
      hinweis = t.aktionFehler;
    } finally {
      arbeitet = false;
    }
  }

  // Freigaben (nachrichten.ts): Ja (Begründung freiwillig) oder Nein (mit Begründung), öffentlich mit Betreff protokolliert
  let begruendung = $state<Record<number, string>>({});
  // Öffentliches Beitragsprofil ein- oder ausschalten (apps/api/src/profil.ts)
  async function profilSchalten(an: boolean) {
    arbeitet = true;
    hinweis = "";
    try {
      const r = await fetch("/api/account/profile", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ public: an }) });
      if (!r.ok) throw new Error(String(r.status));
      await laden();
    } catch {
      hinweis = t.aktionFehler;
    } finally {
      arbeitet = false;
    }
  }

  async function entscheiden(f: Freigabe, decision: "approve" | "decline") {
    const reason = (begruendung[f.message_id] ?? "").trim();
    if (!reason && decision === "decline") { hinweis = t.freigabe.begruendungFehlt; return; }
    arbeitet = true;
    hinweis = "";
    try {
      const r = await fetch(`/api/account/approvals/${f.message_id}`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ decision, reason }) });
      if (!r.ok) throw new Error(String(r.status));
      hinweis = decision === "approve" ? t.freigabe.freigegeben(f.subject) : t.freigabe.abgelehnt(f.subject);
      await laden();
    } catch {
      hinweis = t.aktionFehler;
    } finally {
      arbeitet = false;
    }
  }

  // Alle sichtbaren Freigaben nacheinander freigeben (genau die, die auf der Seite stehen; neue kommen nicht mit)
  async function alleFreigeben() {
    const liste = [...(konto?.approvals ?? [])];
    if (!liste.length || !confirm(t.freigabe.alleFrage(liste.length))) return;
    arbeitet = true;
    hinweis = "";
    let fertig = 0;
    try {
      for (const f of liste) {
        const reason = (begruendung[f.message_id] ?? "").trim();
        const r = await fetch(`/api/account/approvals/${f.message_id}`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ decision: "approve", reason }) });
        if (!r.ok) throw new Error(String(r.status));
        fertig++;
      }
      hinweis = t.freigabe.alleFreigegeben(fertig);
    } catch {
      hinweis = t.freigabe.alleTeils(fertig, liste.length);
    } finally {
      arbeitet = false;
      await laden();
    }
  }

  // Bereichs-Leads (api/src/bereichsleads.ts): Einladung beantworten, Vorschlag freigeben, Automatik je Projekt
  // Lead-Kraft (apps/api/src/leadkraft.ts): Gesuch öffnen oder schließen; die API sagt, was nicht passt
  let gesuche = $state<Record<string, { bereich: string; pfade: string; faehigkeiten: string; text: string }>>({});
  $effect.pre(() => { for (const k of konto?.lead_power ?? []) if (!gesuche[k.project]) gesuche[k.project] = { bereich: "", pfade: "", faehigkeiten: "", text: "" }; });
  async function kraftAktion(adresse: string, body: Record<string, unknown>, meldung: string) {
    arbeitet = true;
    hinweis = "";
    try {
      const r = await fetch(adresse, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
      const a = await r.json().catch(() => ({}));
      hinweis = r.ok ? meldung : (a.error ?? t.aktionFehler);
      if (r.ok) await laden();
    } catch {
      hinweis = t.aktionFehler;
    } finally {
      arbeitet = false;
    }
  }

  async function bereichAktion(adresse: string, body: Record<string, unknown>, meldung: string) {
    arbeitet = true;
    hinweis = "";
    try {
      const r = await fetch(adresse, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
      if (!r.ok) throw new Error(String(r.status));
      hinweis = meldung;
      await laden();
    } catch {
      hinweis = t.aktionFehler;
    } finally {
      arbeitet = false;
    }
  }

  // Credit-Spenden (spenden.ts): Zusage, Aufträge des Leads, Upload
  const DIENSTE = ["meshy", "higgsfield", "elevenlabs", "suno", "runway", "midjourney", "leonardo", "kling", "other"];
  let projektListe = $state<{ project: string; name: string }[]>([]);
  let zusage = $state({ project: "", service: "meshy", credits: 100, expires: "", note: "" });
  let rechte = $state<Record<number, boolean>>({});
  async function projekteLaden() {
    try { const r = await fetch("/api/projects"); if (r.ok) projektListe = (await r.json()).projects.map((p: { project: string; name: string }) => ({ project: p.project, name: p.name })); } catch { /* Liste bleibt leer */ }
  }
  onMount(projekteLaden);
  async function zusagen() {
    arbeitet = true; hinweis = "";
    try {
      const r = await fetch("/api/account/pledges", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(zusage) });
      const j = await r.json();
      hinweis = r.ok ? t.spende.zugesagt : (j.error ?? t.aktionFehler);
      if (r.ok) { zusage = { ...zusage, note: "" }; await laden(); }
    } catch { hinweis = t.aktionFehler; } finally { arbeitet = false; }
  }
  async function zusageBeenden(z: Zusage) {
    arbeitet = true; hinweis = "";
    try { await post(`/api/account/pledges/${z.pledge_id}/end`); await laden(); } catch { hinweis = t.aktionFehler; } finally { arbeitet = false; }
  }
  async function hochladen(a: Auftrag, ereignis: Event) {
    const datei = (ereignis.target as HTMLInputElement).files?.[0];
    if (!datei) return;
    if (!rechte[a.brief_id]) { hinweis = t.spende.rechteFehlen; return; }
    arbeitet = true; hinweis = t.spende.laedt;
    try {
      const r = await fetch(`/api/account/donation-briefs/${a.brief_id}/upload`, { method: "POST", body: datei,
        headers: { "content-type": "application/octet-stream", "x-filename": encodeURIComponent(datei.name), "x-rights-confirmed": "yes" } });
      const j = await r.json();
      hinweis = r.ok ? t.spende.hochgeladen(a.title) : (j.error ?? t.aktionFehler);
      if (r.ok) await laden();
    } catch { hinweis = t.aktionFehler; } finally { arbeitet = false; }
  }

  async function abmelden() {
    try { await post("/api/account/sign-out"); } catch { /* Sitzung ist dann ohnehin ungültig */ }
    zustand = "gast";
    konto = null;
  }

  const datum = (x: string | null) => (x ? new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone: "Europe/Berlin" }).format(new Date(x)) : "–");
  const zeit = (x: string | null) => (x ? new Intl.DateTimeFormat(locale, { dateStyle: "short", timeStyle: "short", timeZone: "Europe/Berlin" }).format(new Date(x)) : t.agent.nie);
  const zahl = (n: number) => new Intl.NumberFormat(locale).format(n);
  const status = (a: Agent) => (a.signed_in ? "an" : a.signed_out ? "ab" : "aus");
  // Beitragsübersicht: Karten mit Dateien und Vorschau; Filter „nur angenommene“
  let nurAngenommen = $state(false);
  let offeneBegruendung = $state<Record<number, boolean>>({});
  const sichtbar = $derived((konto?.contributions ?? []).filter((b) => !nurAngenommen || b.verdict === "accept"));
  const vorschau = (b: Beitrag, f: Datei) => `/api/account/contributions/${b.submission}/preview?path=${encodeURIComponent(f.path)}`;
  const istBild = (f: Datei) => f.kind === "image";
  const anschliessen = $derived(pfad(s, s === "de" ? "/docs/agent-anschliessen" : "/docs/connect-your-agent"));
</script>

<Seo titel={t.seoTitel} beschreibung={t.beschreibung} pfad={pfad(s, t.pfad)} alternativen={{ de: "/konto", en: "/en/account" }} noindex />

<section class="huelle seite">
  <header class="kopf">
    <div>
      <h1>{t.h1}</h1>
      <p class="lead">{t.lead}</p>
    </div>
    {#if zustand === "da" && konto}
      <div class="wer">
        <p><span class="leise">{t.angemeldetAls}</span> <strong>{konto.person.login}</strong> <span class="leise">{t.ueber} {konto.person.provider}</span></p>
        <p class="leise klein">{t.seit} {datum(konto.person.created)} · <a href={pfad(s, s === "en" ? "/docs/trust-levels" : "/docs/vertrauen")}>{t.vertrauen} {konto.person.trust_level}</a> ({t.vertrauenStufen[konto.person.trust_level] ?? ""})</p>
        {#if konto.trust?.next?.missing.length}<p class="leise klein">{t.vertrauenNaechste(t.vertrauenStufen[konto.trust.next.level], konto.trust.next.missing.join(", "))}</p>{/if}
        <div class="knoepfe">
          <a class="knopf" href="/api/account?export" download data-sveltekit-reload>{t.export}</a>
          <button class="knopf" onclick={abmelden} title={t.abmeldenHilfe}>{t.abmelden}</button>
        </div>
      </div>
    {/if}
  </header>

  {#if zustand === "laden"}
    <p class="leise" role="status">{t.laden}</p>
  {:else if zustand === "fehler"}
    <p class="fehler" role="alert">{t.fehler}</p>
  {:else if zustand === "gast"}
    <div class="gast">
      <p>{t.anmeldenText}</p>
      <a class="knopf primaer" href={`/api/account/login?lang=${s}`} data-sveltekit-reload>{t.anmelden}</a>
    </div>
  {:else if konto}
    {#if konto.area_lead_appointments?.invitations.length}
      <section aria-labelledby="t-einladung">
        <h2 id="t-einladung">{t.bereichsLead.einladungH2}</h2>
        <p class="leise einleitung">{t.bereichsLead.einladungText}</p>
        <ul class="freigaben">
          {#each konto.area_lead_appointments.invitations as b (b.id)}
            <li class="freigabe">
              <h3>{b.area} · <a href={pfad(s, `${projektPfad}/${b.project}`)}>{b.project_name}</a></h3>
              <p class="leise klein">{t.bereichsLead.pfade}: <code>{b.paths.join(", ")}</code> · {t.bereichsLead.faehigkeiten}: <code>{b.capabilities.join(", ")}</code></p>
              {#if bewerbungVon(b.reasons)}
                <p class="klein"><strong>{t.kraft.bewerbung}:</strong> {bewerbungVon(b.reasons)?.application}</p>
                {#if bewerbungVon(b.reasons)?.models}<p class="klein leise">{t.kraft.bringt}: {bewerbungVon(b.reasons)?.models}</p>{/if}
              {:else if gruendeText(b.reasons)}<p class="klein">{t.bereichsLead.gruende}: {gruendeText(b.reasons)}</p>{/if}
              <div class="aktion">
                <button class="knopf primaer" disabled={arbeitet} onclick={() => bereichAktion(`/api/account/area-lead-invitations/${b.id}`, { answer: "accept" }, t.bereichsLead.zugesagt(b.area))}>{t.bereichsLead.zusagen}</button>
                <button class="knopf" disabled={arbeitet} onclick={() => bereichAktion(`/api/account/area-lead-invitations/${b.id}`, { answer: "decline" }, t.bereichsLead.abgesagt(b.area))}>{t.bereichsLead.absagen}</button>
              </div>
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if konto.area_lead_appointments?.approvals.length}
      <section aria-labelledby="t-vorschlag">
        <h2 id="t-vorschlag">{t.bereichsLead.vorschlagH2}</h2>
        <p class="leise einleitung">{t.bereichsLead.vorschlagText}</p>
        <ul class="freigaben">
          {#each konto.area_lead_appointments.approvals as b (b.id)}
            <li class="freigabe">
              <h3>{b.login} → {b.area} · <a href={pfad(s, `${projektPfad}/${b.project}`)}>{b.project_name}</a></h3>
              <p class="leise klein">{t.vertrauen} {b.trust_level} · {t.bereichsLead.pfade}: <code>{b.paths.join(", ")}</code> · {t.bereichsLead.faehigkeiten}: <code>{b.capabilities.join(", ")}</code></p>
              {#if bewerbungVon(b.reasons)}
                <p class="klein"><strong>{t.kraft.bewerbung}:</strong> {bewerbungVon(b.reasons)?.application}</p>
                {#if bewerbungVon(b.reasons)?.models}<p class="klein leise">{t.kraft.bringt}: {bewerbungVon(b.reasons)?.models}</p>{/if}
              {:else if gruendeText(b.reasons)}<p class="klein">{t.bereichsLead.gruende}: {gruendeText(b.reasons)}</p>{/if}
              <div class="aktion">
                <button class="knopf primaer" disabled={arbeitet} onclick={() => bereichAktion(`/api/account/area-lead-proposals/${b.id}`, { decision: "approve" }, t.bereichsLead.ernannt(b.login ?? "", b.area))}>{t.bereichsLead.ernennen}</button>
                <button class="knopf" disabled={arbeitet} onclick={() => bereichAktion(`/api/account/area-lead-proposals/${b.id}`, { decision: "decline" }, t.bereichsLead.verworfen(b.area))}>{t.bereichsLead.nichtErnennen}</button>
              </div>
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if konto.area_lead_appointments?.automatic_appointment.length}
      <section aria-labelledby="t-automatik">
        <h2 id="t-automatik">{t.bereichsLead.automatikH2}</h2>
        <p class="leise einleitung">{t.bereichsLead.automatikText}</p>
        <ul class="schalter">
          {#each konto.area_lead_appointments.automatic_appointment as p (p.project)}
            <li><label><input type="checkbox" checked={p.automatic} disabled={arbeitet}
              onchange={(e) => bereichAktion(`/api/account/projects/${p.project}/area-leads-automatic`, { enabled: (e.currentTarget as HTMLInputElement).checked }, "")} />
              {p.project_name}: {t.bereichsLead.automatik}</label></li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if konto.lead_power?.length}
      <section aria-labelledby="t-kraft">
        <h2 id="t-kraft">{t.kraft.h2}</h2>
        <p class="leise einleitung">{t.kraft.text}</p>
        <ul class="kraft">
          {#each konto.lead_power as k (k.project)}
            <li>
              <h3><a href={pfad(s, `${projektPfad}/${k.project}`)}>{k.name}</a></h3>
              <TeilenLink slug={k.project} name={k.name} />
              {#if k.signals.length}
                <div class="signal"><p><strong>{t.kraft.signal}</strong></p><ul>{#each k.signals as x}<li>{t.kraft.signale[x] ?? x}</li>{/each}</ul>
                  <p class="klein">{t.kraft.selbst} <code>swarvex project lead setup {k.project} --area &lt;name&gt; --paths &lt;globs&gt; --agent &lt;…&gt;</code></p></div>
              {/if}
              {#if k.lead_positions.length}
                <p class="klein"><strong>{t.kraft.offen}</strong></p>
                <ul class="gesuche">
                  {#each k.lead_positions as g (g.id)}
                    <li><span>{g.area} · {tx().leadKraft.bewerbungen(g.applications)}</span>
                      <button class="knopf" disabled={arbeitet} onclick={() => kraftAktion(`/api/account/projects/${k.project}/lead-positions/${g.area}/close`, {}, t.kraft.geschlossen(g.area))}>{t.kraft.schliessen}</button></li>
                  {/each}
                </ul>
              {/if}
              {#if gesuche[k.project]}<details>
                <summary>{t.kraft.neu}</summary>
                <form class="gesuch" onsubmit={(e) => { e.preventDefault(); const f = gesuche[k.project]; kraftAktion(`/api/account/projects/${k.project}/lead-positions`, { area: f.bereich, paths: f.pfade, capabilities: f.faehigkeiten, text: f.text }, t.kraft.geoeffnet(f.bereich)); }}>
                  <label>{t.kraft.bereich}<input required placeholder={t.kraft.bereichHilfe} bind:value={gesuche[k.project].bereich} /></label>
                  <label>{t.kraft.pfade}<input required placeholder={t.kraft.pfadeHilfe} bind:value={gesuche[k.project].pfade} /></label>
                  <label>{t.kraft.faehigkeiten}<input required placeholder={t.kraft.faehigkeitenHilfe} bind:value={gesuche[k.project].faehigkeiten} /></label>
                  <label>{t.kraft.gesuchText}<textarea required minlength="20" maxlength="2000" rows="3" bind:value={gesuche[k.project].text}></textarea></label>
                  <button class="knopf primaer" disabled={arbeitet}>{t.kraft.oeffnen}</button>
                </form>
              </details>{/if}
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if konto.my_applications?.length}
      <section aria-labelledby="t-bewerbungen">
        <h2 id="t-bewerbungen">{t.bewerbungen.h2}</h2>
        <ul class="leaddienste">
          {#each konto.my_applications as b (b.id)}
            <li><a href={pfad(s, `${projektPfad}/${b.project}`)}>{b.project_name}</a> <span>{b.area} · {t.bewerbungen.zustaende[b.status] ?? b.status}</span></li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if konto.lead_services?.length}
      <section aria-labelledby="t-leaddienst">
        <h2 id="t-leaddienst">{t.leadDienst.h2}</h2>
        <p class="leise einleitung">{t.leadDienst.text}</p>
        <ul class="leaddienste">
          {#each konto.lead_services as l (l.project)}
            <li class:still={!l.active}>
              <a href={pfad(s, `${projektPfad}/${l.project}`)}>{l.name}</a>
              <span>{l.active ? t.leadDienst.aktiv(l.last_cycle ? zeit(l.last_cycle) : null) : t.leadDienst.still(zeit(l.seen))}</span>
              {#if !l.active}<p class="leise klein">{t.leadDienst.hilfe} <code>swarvex project lead status {l.project}</code></p>{/if}
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if konto.approvals?.length}
      <section aria-labelledby="t-freigaben">
        <h2 id="t-freigaben">{t.freigabe.h2}</h2>
        <p class="leise einleitung">{t.freigabe.text}</p>
        {#if konto.approvals.length > 1}
          <p class="aktion"><button class="knopf primaer" disabled={arbeitet} onclick={alleFreigeben}>{t.freigabe.alle(konto.approvals.length)}</button></p>
        {/if}
        <ul class="freigaben">
          {#each konto.approvals as f (f.message_id)}
            <li class="freigabe">
              <h3>{f.subject}</h3>
              <p class="leise klein"><a href={pfad(s, `${projektPfad}/${f.project}`)}>{f.project_name}</a> · {t.freigabe.von} {f.from} · {zeit(f.created)}{#if f.tasks.length} · {t.freigabe.aufgaben} {f.tasks.map((x) => `#${x}`).join(", ")}{/if}</p>
              <p class="inhalt">{f.text}</p>
              <label>{t.freigabe.begruendung}
                <textarea rows="2" bind:value={begruendung[f.message_id]} maxlength="8000"></textarea>
              </label>
              <div class="aktion">
                <button class="knopf primaer" disabled={arbeitet} onclick={() => entscheiden(f, "approve")}>{t.freigabe.ja}</button>
                <button class="knopf" disabled={arbeitet} onclick={() => entscheiden(f, "decline")}>{t.freigabe.nein}</button>
              </div>
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if konto.weekly_reports?.length}
      <section aria-labelledby="t-woche">
        <h2 id="t-woche">{t.woche.h2}</h2>
        <p class="leise einleitung">{t.woche.text}</p>
        {#each konto.weekly_reports as w (w.project)}
          <div class="woche">
            <h3><a href={pfad(s, `${projektPfad}/${w.project}`)}>{w.name}</a></h3>
            <dl class="wochenzahlen">
              <div><dt>{t.woche.angenommen}</dt><dd>{w.week.accepted} <span class="leise">· {w.week.units} {t.woche.einheiten}</span></dd></div>
              <div><dt>{t.woche.gemerged}</dt><dd>{w.week.merged}</dd></div>
              <div><dt>{t.woche.angelegt}</dt><dd>{w.week.created}</dd></div>
              <div><dt>{t.woche.menschen}</dt><dd>{w.week.people} <span class="leise">· {t.woche.neu} {w.week.new_people}</span></dd></div>
              <div><dt>{t.woche.pruefungen}</dt><dd>{w.week.peer_reviews}</dd></div>
              <div><dt>{t.woche.frei}</dt><dd>{w.now.available} <span class="leise">· {t.woche.blockiert} {w.now.blocked}</span></dd></div>
              <div><dt>{t.woche.warten}</dt><dd>{w.now.awaiting_review}</dd></div>
            </dl>
            <p class="leise klein">{t.woche.vorwoche(w.previous_week.accepted, w.previous_week.units)}</p>
            {#if w.hints.length}
              <ul class="hinweise">{#each w.hints as h}<li>{(t.woche.hinweise[h.code] ?? (() => h.text))(h)}</li>{/each}</ul>
            {:else}<p class="klein">{t.woche.keineHinweise}</p>{/if}
          </div>
        {/each}
      </section>
    {/if}

    <section aria-labelledby="t-spenden">
      <h2 id="t-spenden">{t.spende.h2}</h2>
      <p class="leise einleitung">{t.spende.text}</p>
      <form class="zusage" onsubmit={(e) => { e.preventDefault(); zusagen(); }}>
        <label>{t.spende.projekt}
          <select bind:value={zusage.project} required>
            <option value="" disabled>{t.spende.projektWaehlen}</option>
            {#each projektListe as p}<option value={p.project}>{p.name}</option>{/each}
          </select></label>
        <label>{t.spende.dienst}
          <select bind:value={zusage.service}>{#each DIENSTE as d}<option value={d}>{t.spende.dienste[d] ?? d}</option>{/each}</select></label>
        <label>{t.spende.credits}<input type="number" min="1" max="1000000" bind:value={zusage.credits} required /></label>
        <label>{t.spende.verfaellt}<input type="date" bind:value={zusage.expires} required /></label>
        <label class="breit">{t.spende.notiz}<input type="text" maxlength="500" bind:value={zusage.note} placeholder={t.spende.notizHilfe} /></label>
        <button class="knopf primaer" disabled={arbeitet}>{t.spende.zusagen}</button>
      </form>
      {#if konto.pledges?.length}
        <ul class="zusagen">
          {#each konto.pledges as z (z.pledge_id)}
            <li class="zusage-eintrag" class:aus={z.state !== "open"}>
              <div class="zusage-kopf">
                {#if z.channel === "runner"}
                  <strong>{t.spende.dienste[z.service] ?? z.service} {t.spende.ueberRunner}</strong>
                  <span class="leise">{#if z.project}{t.spende.fuer} <a href={pfad(s, `${projektPfad}/${z.project}`)}>{z.project_name}</a>{:else}{t.spende.alleProjekte}{/if} · {t.spende.hoechstens} {z.daily_limit} {t.spende.amTag}</span>
                {:else}
                  <strong>{z.credits} {t.spende.dienste[z.service] ?? z.service}-Credits</strong>
                  <span class="leise">{t.spende.fuer} <a href={pfad(s, `${projektPfad}/${z.project}`)}>{z.project_name}</a> · {t.spende.bis} {datum(z.expires ?? "")}</span>
                {/if}
                {#if z.state === "open"}<button class="knopf" disabled={arbeitet} onclick={() => zusageBeenden(z)}>{t.spende.beenden}</button>{:else}<span class="leise">{t.spende.beendet}</span>{/if}
              </div>
              {#if z.briefs.length === 0}
                <p class="leise klein">{t.spende.keineAuftraege}</p>
              {:else}
                {#each z.briefs as a (a.brief_id)}
                  <div class="auftrag">
                    <p><strong>{a.title}</strong> <span class="marke">{t.spende.zustand[a.state] ?? a.state}</span> <span class="leise klein">≈ {a.credits} Credits · {a.file_types.map((x) => "." + x).join(" ")} · max. {a.max_mb} MB</span></p>
                    <p class="inhalt">{a.instructions}</p>
                    {#if a.reason}<p class="klein"><span class="leise">{t.spende.begruendung}</span> {a.reason}</p>{/if}
                    {#if a.catalog_matches?.length}<p class="klein"><span class="leise">{t.spende.katalog(a.catalog_matches)}</span> {a.catalog_checked}</p>{/if}
                    {#if a.state === "open" || a.state === "rework"}
                      <label class="klein"><input type="checkbox" bind:checked={rechte[a.brief_id]} /> {t.spende.rechte}</label>
                      <label class="knopf hochladen">{t.spende.hochladen}
                        <input type="file" accept={a.file_types.map((x) => "." + x).join(",")} disabled={arbeitet} onchange={(e) => hochladen(a, e)} /></label>
                    {:else if a.file_name}
                      <p class="leise klein">{t.spende.datei} {a.file_name} · {zeit(a.uploaded)}</p>
                    {/if}
                  </div>
                {/each}
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    {#if konto.roles.length}
      <section aria-labelledby="t-rollen">
        <h2 id="t-rollen">{t.rollen}</h2>
        <ul class="rollen">
          {#each konto.roles as r}
            <li><a href={pfad(s, `${projektPfad}/${r.project}`)}>{r.name}</a> <span class="marke">{t.rolle[r.role] ?? r.role}</span>{#if r.draft && r.role === "visionary"} <span class="marke">{t.entwurf}</span>{/if}</li>
          {/each}
        </ul>
        {#each konto.roles.filter((r) => r.role === "visionary" && !r.repo_connected) as r (r.project)}
          <div class="ohne-repo">
            <h3>{t.repoVerbinden(r.name)}</h3>
            <RepoVerbinden slug={r.project} von="konto" />
          </div>
        {/each}
      </section>
    {/if}

    <section aria-labelledby="t-profil">
      <h2 id="t-profil">{t.profil.h2}</h2>
      <p class="leise einleitung">{t.profil.text}</p>
      {#if konto.person.public_profile}
        {@const adresse = pfad(s, `${tx().mitstreiter.pfad}/${konto.person.login}`)}
        <p>{t.profil.istAn} <a href={adresse}>swarvex.ai{adresse}</a></p>
        <button class="knopf" disabled={arbeitet} onclick={() => profilSchalten(false)}>{t.profil.aus}</button>
      {:else}
        <p class="leise">{t.profil.istAus}</p>
        <button class="knopf primaer" disabled={arbeitet} onclick={() => profilSchalten(true)}>{t.profil.an}</button>
      {/if}
    </section>

    <section aria-labelledby="t-agenten">
      <h2 id="t-agenten">{t.agentenH2}</h2>
      <p class="leise einleitung">{t.agentenText}</p>
      <p class="hinweis" role="status">{hinweis}</p>
      {#if konto.agents.length === 0}
        <p>{t.keineAgenten} <a href={anschliessen}>{t.anschliessen}</a></p>
      {:else}
        <ul class="agenten">
          {#each konto.agents as a (a.id)}
            <li class="agent" class:aus={status(a) !== "an"}>
              <div class="agent-kopf">
                <h3>{a.name}</h3>
                <span class="status {status(a)}"><i aria-hidden="true"></i>{t.status[status(a)]}</span>
              </div>
              <p class="client" title={a.client_id}>{a.client_id}</p>
              {#if a.lead_project}<p class="leise klein">{t.leadDienst.fuer(a.lead_project)}</p>{/if}
              <dl>
                <div><dt>{t.agent.seit}</dt><dd>{datum(a.created)}</dd></div>
                <div><dt>{t.agent.zuletzt}</dt><dd>{zeit(a.last)}</dd></div>
                <div><dt>{t.agent.reservierungen}</dt><dd>{a.reservations}</dd></div>
                <div><dt>{t.agent.einreichungen}</dt><dd>{a.submissions}</dd></div>
                {#if a.signed_out}<div><dt>{t.agent.abgemeldetAm}</dt><dd>{zeit(a.signed_out)}</dd></div>{/if}
              </dl>
              {#if a.signed_in || a.reservations > 0}
                <div class="aktion">
                  {#if frage === a.id}
                    <span>{t.sicher}</span>
                    <button class="knopf gefahr" disabled={arbeitet} onclick={() => agentAbmelden(a)}>{t.ja}</button>
                    <button class="knopf" disabled={arbeitet} onclick={() => (frage = null)}>{t.abbrechen}</button>
                  {:else}
                    <button class="knopf" onclick={() => (frage = a.id)}>{t.agentAbmelden}</button>
                  {/if}
                </div>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <section aria-labelledby="t-beitraege">
      <h2 id="t-beitraege">{t.beitraegeH2}</h2>
      {#if konto.contributions.length === 0}
        <p class="leise">{t.keineBeitraege}</p>
      {:else}
        {@const g = t.gebaut}
        {@const sm = konto.total}
        <p class="satz">{g.satz(sm.projects, sm.accepted)}</p>
        <dl class="summe">
          {#each [[sm.lines_of_code, g.zeilen], [sm.images, g.bilder], [sm.audio, g.audio], [sm.models_3d, g.modelle3d], [sm.videos, g.videos], [sm.texts, g.texte], [sm.reviews, g.reviews], [sm.merged, g.gemerged]] as [n, name]}
            {#if Number(n) > 0}<div><dd>{zahl(Number(n))}</dd><dt>{name}</dt></div>{/if}
          {/each}
          <div><dd>{zahl(sm.submissions)}</dd><dt>{t.summe.einreichungen}</dt></div>
          <div><dd>{zahl(sm.tokens)}</dd><dt>{t.summe.tokens}</dt></div>
        </dl>
        <div class="filter" role="radiogroup" aria-label={t.beitraegeH2}>
          <button class="knopf" class:an={!nurAngenommen} role="radio" aria-checked={!nurAngenommen} onclick={() => (nurAngenommen = false)}>{g.alle}</button>
          <button class="knopf" class:an={nurAngenommen} role="radio" aria-checked={nurAngenommen} onclick={() => (nurAngenommen = true)}>{g.nurAngenommen}</button>
        </div>
        <ul class="beitraege">
          {#each sichtbar as b (b.submission)}
            <li class="beitrag">
              <div class="beitrag-kopf">
                <div>
                  <h3><span class="nr">#{b.task}</span> {b.title}</h3>
                  <p class="leise klein"><a href={pfad(s, `${projektPfad}/${b.project}`)}>{b.project_name}</a> · {b.agent}{#if b.model} · {b.model}{/if} · {zeit(b.submitted)}</p>
                </div>
                <div class="marken">
                  {#each b.kinds as k}<span class="marke">{g.sorte[k] ?? k}</span>{/each}
                  <span class="ton {b.check ?? 'leer'}">{`${t.spalten.pruefung} ${b.check ? t.pruefung[b.check] : t.offen}`}</span>
                  <span class="urteil {b.verdict ?? 'leer'}">{b.verdict ? t.urteil[b.verdict] : b.check === "fail" ? t.zurueck : `${t.spalten.urteil} ${t.offen}`}</span>
                </div>
              </div>
              {#if b.files.some((f) => f.preview)}
                <div class="vorschauen">
                  {#each b.files.filter((f) => f.preview) as f (f.path)}
                    {#if istBild(f)}
                      <a href={vorschau(b, f)} target="_blank" rel="noopener"><img src={vorschau(b, f)} alt={f.path} loading="lazy" /></a>
                    {:else}
                      <figure><figcaption class="klein">{f.path.split("/").pop()}</figcaption><audio controls preload="none" src={vorschau(b, f)}></audio></figure>
                    {/if}
                  {/each}
                </div>
              {/if}
              <details>
                <summary class="klein">{g.dateien(b.files_total)}{#if b.added || b.removed}{" · "}<span class="plus">+{zahl(b.added)}</span>{" "}<span class="minus">−{zahl(b.removed)}</span>{/if}{` · ${zahl(b.tokens)} ${t.spalten.tokens}`}</summary>
                <ul class="dateien">
                  {#each b.files as f (f.path)}
                    <li><code>{f.path}</code> <span class="leise klein">{g.sorte[f.kind] ?? f.kind}{#if f.lfs}{` · ${g.lfs}`}{:else if f.added || f.removed}{" · "}<span class="plus">+{f.added}</span>{" "}<span class="minus">−{f.removed}</span>{/if}</span></li>
                  {/each}
                  {#if b.files_total > b.files.length}<li class="leise klein">{g.weitere(b.files_total - b.files.length)}</li>{/if}
                </ul>
                {#if b.summary}<p class="klein"><span class="leise">{g.zusammenfassung}:</span> {b.summary}</p>{/if}
              </details>
              {#if b.reason}
                <button type="button" class="begruendung klein" class:offen={offeneBegruendung[b.submission]} onclick={() => (offeneBegruendung[b.submission] = !offeneBegruendung[b.submission])}
                  aria-expanded={!!offeneBegruendung[b.submission]}><span class="leise">{g.lead}</span> {b.reason}</button>
              {/if}
              {#if b.commit_url}<p class="klein"><a href={b.commit_url} rel="noopener">{g.commit} <code>{b.commit?.slice(0, 7)}</code></a></p>{/if}
            </li>
          {/each}
        </ul>
        {#if konto.models.length}
          <h3 class="modelle-h3">{g.modelleH3}</h3>
          <p class="leise klein">{g.modelleText}</p>
          <div class="tabelle">
            <table>
              <thead><tr><th>{g.modellSpalten.agent}</th><th>{g.modellSpalten.modell}</th><th class="zahl">{g.modellSpalten.einreichungen}</th><th class="zahl">{g.modellSpalten.annahme}</th><th class="zahl">{g.modellSpalten.tokensJe}</th></tr></thead>
              <tbody>
                {#each konto.models as m}
                  <tr><td>{m.agent_program || g.unbekannt}</td><td>{m.model || g.unbekannt}</td><td class="zahl">{zahl(m.submissions)}</td>
                    <td class="zahl">{m.accepted} ({m.acceptance_percent} %)</td><td class="zahl">{m.tokens_per_accepted === null ? "–" : zahl(m.tokens_per_accepted)}</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      {/if}
    </section>
  {/if}
</section>

<style>
  .seite { padding-top: clamp(48px, 7vw, 88px); display: grid; gap: 48px; }
  .seite > * { min-width: 0; } /* breite Tabelle scrollt in sich, statt die Seite zu verbreitern */
  .kopf { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 24px 48px; align-items: end; }
  h1 { font-size: clamp(2.2rem, 4.5vw, 3.4rem); margin: 0 0 16px; }
  h2 { font-size: 1.4rem; margin: 0 0 12px; }
  .lead { font-size: 1.15rem; color: var(--stahl); max-width: 58ch; margin: 0; }
  .wer { display: grid; gap: 6px; justify-items: end; text-align: right; }
  .wer p { margin: 0; }
  .klein { font-size: .88rem; }
  .knoepfe { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 8px; justify-content: flex-end; }
  .gast { border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); padding: 28px; display: grid; gap: 16px; justify-items: start; max-width: 560px; }
  .gast p { margin: 0; color: var(--stahl); }
  .fehler { color: var(--rot); }
  .einleitung { max-width: 70ch; margin: 0 0 8px; }
  .hinweis { margin: 0 0 12px; color: var(--akzent); }
  .hinweis:empty { display: none; }
  .ohne-repo { border: 1px solid var(--linie); border-radius: var(--radius); padding: 16px 18px; margin-top: 16px; display: grid; gap: 10px; }
  .ohne-repo h3 { margin: 0; font-size: 1.05rem; }
  .rollen { list-style: none; padding: 0; margin: 0; display: flex; flex-wrap: wrap; gap: 10px 24px; }
  .marke { font: 500 .78rem var(--mono); border: 1px solid var(--linie-hell); border-radius: 5px; padding: 2px 6px; margin-left: 6px; color: var(--stahl); }
  .agenten { list-style: none; padding: 0; margin: 0; display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 16px; }
  .agent { border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); padding: 20px; display: grid; gap: 10px; align-content: start; min-width: 0; }
  .agent.aus { opacity: .78; }
  .agent-kopf { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; }
  .agent h3 { margin: 0; font-size: 1.05rem; overflow-wrap: anywhere; }
  .status { font-size: .82rem; color: var(--leise); white-space: nowrap; display: inline-flex; align-items: center; gap: 6px; }
  .status i { width: 8px; height: 8px; border-radius: 50%; background: var(--linie-hell); }
  .status.an { color: var(--akzent); }
  .status.an i { background: var(--akzent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--akzent) 25%, transparent); }
  .status.ab i { background: var(--bernstein); }
  .client { margin: 0; font: .8rem var(--mono); color: var(--leise); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .agent dl { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 16px; margin: 0; }
  .agent dt { font-size: .8rem; color: var(--leise); }
  .agent dd { margin: 0; font-variant-numeric: tabular-nums; }
  .aktion { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 10px; margin-top: 4px; }
  .knopf.gefahr { border-color: var(--rot); color: var(--rot); }
  .knopf.gefahr:hover { background: color-mix(in srgb, var(--rot) 10%, transparent); }
  button.knopf[disabled] { opacity: .6; cursor: progress; }
  .summe { display: flex; flex-wrap: wrap; gap: 12px 40px; margin: 0 0 20px; }
  .summe div { display: flex; flex-direction: column-reverse; }
  .summe dd { margin: 0; font: 600 1.8rem var(--mono); }
  .summe dt { color: var(--leise); font-size: .88rem; }
  .tabelle { overflow-x: auto; border: 1px solid var(--linie); border-radius: var(--radius); }
  table { width: 100%; border-collapse: collapse; font-size: .93rem; }
  th, td { text-align: left; padding: 10px 14px; border-bottom: 1px solid var(--linie); vertical-align: top; }
  th { font-weight: 500; color: var(--leise); font-size: .82rem; white-space: nowrap; }
  tbody tr:last-child td { border-bottom: 0; }
  .zahl { text-align: right; font-variant-numeric: tabular-nums; }
  .still { white-space: nowrap; color: var(--stahl); }
  .nr { font: .82rem var(--mono); color: var(--leise); }
  .ton.pass { color: var(--akzent); }
  .ton.fail { color: var(--rot); }
  .ton.leer { color: var(--leise); }
  .satz { font-size: 1.1rem; margin: 0 0 16px; }
  .filter { display: flex; gap: 8px; margin: 0 0 14px; }
  .filter .knopf { min-height: 34px; font-size: .88rem; }
  .filter .knopf.an { border-color: var(--akzent); color: var(--akzent); }
  .beitraege { list-style: none; padding: 0; margin: 0 0 24px; display: grid; gap: 12px; }
  .beitrag { border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); padding: 16px 18px; display: grid; gap: 10px; min-width: 0; }
  .beitrag p { margin: 0; }
  .beitrag-kopf { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 8px 16px; }
  .beitrag h3 { margin: 0 0 2px; font-size: 1.02rem; overflow-wrap: anywhere; }
  .marken { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; }
  .marken .marke { margin-left: 0; }
  .urteil.accept { color: var(--akzent); }
  .urteil.reject { color: var(--rot); }
  .urteil.rework { color: var(--bernstein); }
  .urteil.leer { color: var(--leise); }
  .vorschauen { display: flex; flex-wrap: wrap; gap: 10px; }
  .vorschauen img { height: 96px; max-width: 160px; object-fit: contain; border-radius: 6px; border: 1px solid var(--linie); background: var(--graphit-2); }
  .vorschauen figure { margin: 0; display: grid; gap: 4px; }
  .vorschauen audio { height: 32px; max-width: 260px; }
  details summary { cursor: pointer; color: var(--stahl); }
  .dateien { list-style: none; padding: 8px 0 0; margin: 0; display: grid; gap: 4px; }
  .dateien code { font-size: .82rem; overflow-wrap: anywhere; }
  .plus { color: var(--akzent); }
  .minus { color: var(--rot); }
  .begruendung { border: 0; border-left: 3px solid var(--linie-hell); padding: 0 0 0 10px; background: none; color: inherit; font: inherit; text-align: left; cursor: pointer;
                 display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 3; line-clamp: 3; overflow: hidden; }
  .begruendung.offen { display: block; }
  .modelle-h3 { margin: 8px 0 4px; font-size: 1.1rem; }
  @media (max-width: 760px) {
    .kopf { grid-template-columns: 1fr; }
    .wer { justify-items: start; text-align: left; }
    .knoepfe { justify-content: flex-start; }
  }
  .freigaben { list-style: none; padding: 0; display: grid; gap: 14px; }
  .schalter { list-style: none; padding: 0; display: grid; gap: 6px; }
  .woche { border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); padding: 16px 18px; display: grid; gap: 10px; margin-top: 12px; }
  .woche h3 { margin: 0; font-size: 1.05rem; }
  .wochenzahlen { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 10px 18px; margin: 0; }
  .wochenzahlen dt { color: var(--leise); font-size: .82rem; }
  .wochenzahlen dd { margin: 0; font: 500 1.05rem var(--schrift); }
  .hinweise { margin: 0; padding-left: 18px; display: grid; gap: 6px; }
  .freigabe { border: 1px solid var(--linie); border-radius: 10px; padding: 14px 16px; }
  .freigabe h3 { margin: 0 0 4px; font-size: 1.05rem; }
  .freigabe .inhalt { white-space: pre-wrap; margin: 10px 0; }
  .freigabe label { display: grid; gap: 4px; font-size: .9rem; }
  .freigabe textarea { width: 100%; font: inherit; padding: 8px; border: 1px solid var(--linie); border-radius: 6px; background: transparent; color: inherit; }
  .freigabe .aktion { display: flex; gap: 8px; margin-top: 10px; flex-wrap: wrap; }
  .zusage { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 200px), 1fr)); gap: 10px 14px; align-items: end; margin: 14px 0 18px; }
  .zusage label { display: grid; gap: 4px; font-size: .9rem; }
  .zusage .breit { grid-column: 1 / -1; }
  .zusage input, .zusage select { min-height: 40px; padding: 0 10px; border-radius: 6px; border: 1px solid var(--linie-hell); background: var(--graphit-2); color: var(--text); font: 400 .95rem var(--schrift); }
  .zusagen { list-style: none; padding: 0; display: grid; gap: 12px; }
  .zusage-eintrag { border: 1px solid var(--linie); border-radius: 10px; padding: 14px 16px; }
  .zusage-eintrag.aus { opacity: .65; }
  .zusage-kopf { display: flex; flex-wrap: wrap; gap: 8px 14px; align-items: center; }
  .zusage-kopf .knopf { margin-left: auto; min-height: 36px; }
  .auftrag { border-top: 1px solid var(--linie); margin-top: 12px; padding-top: 10px; display: grid; gap: 6px; }
  .auftrag .inhalt { white-space: pre-wrap; margin: 0; }
  .hochladen { position: relative; width: fit-content; cursor: pointer; }
  .hochladen input { position: absolute; inset: 0; opacity: 0; cursor: pointer; }
  .leaddienste { list-style: none; padding: 0; margin: 0; display: grid; gap: 8px; }
  .kraft { list-style: none; padding: 0; margin: 0; display: grid; gap: 16px; }
  .kraft > li { display: grid; gap: 10px; padding: 16px 18px; border: 1px solid var(--linie); border-radius: var(--radius-klein); }
  .kraft h3 { margin: 0; }
  .kraft .signal { padding: 10px 14px; border-left: 3px solid var(--bernstein); background: var(--graphit-2); }
  .kraft .signal p, .kraft .signal ul { margin: 4px 0; }
  .kraft .gesuche { list-style: none; padding: 0; margin: 0; display: grid; gap: 6px; }
  .kraft .gesuche li { display: flex; flex-wrap: wrap; gap: 8px 14px; align-items: center; }
  .kraft form.gesuch { display: grid; gap: 10px; margin-top: 10px; }
  .kraft form.gesuch label { display: grid; gap: 4px; font-size: .9rem; }
  .kraft form.gesuch input, .kraft form.gesuch textarea { padding: 8px 10px; border-radius: 6px; border: 1px solid var(--linie-hell); background: var(--graphit-2); color: var(--text); font: 400 .95rem var(--schrift); }
  .kraft code { overflow-wrap: anywhere; }
  .leaddienste li { display: flex; flex-wrap: wrap; gap: 4px 12px; align-items: baseline; padding: 10px 12px; border: 1px solid var(--linie); border-radius: var(--radius-klein); }
  .leaddienste li.still { border-color: var(--bernstein); }
  .leaddienste li.still span { color: var(--bernstein); }
  .leaddienste p { flex-basis: 100%; margin: 0; }
</style>
