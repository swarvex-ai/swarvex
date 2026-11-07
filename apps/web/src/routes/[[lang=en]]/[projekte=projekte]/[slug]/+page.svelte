<script lang="ts">
  import { onMount } from "svelte";
  import { invalidate } from "$app/navigation";
  import Seo from "$lib/components/Seo.svelte";
  import Zaehler from "$lib/components/Zaehler.svelte";
  import VorschauFeedback from "$lib/components/VorschauFeedback.svelte";
  import Einzeiler from "$lib/components/Einzeiler.svelte";
  import TeilenLink from "$lib/components/TeilenLink.svelte";
  import { SEITE } from "$lib/seo";
  import { pfad, sprache, tx } from "$lib/texte";
  import { projektText } from "$lib/projekttext";

  let { data } = $props();
  const s = $derived(sprache());
  const t = $derived(tx().projekte);
  const d = $derived(t.detail);
  const p = $derived(data.projekt);
  const text = $derived(projektText(p, s));
  let original = $state(false);
  // wer gerade an einer Aufgabe arbeitet (aktive Reservierung), für den Abschnitt „Beitragende“
  const amArbeiten = $derived([...new Set((p.tasks ?? []).map((a: { working?: string | null }) => a.working).filter(Boolean))] as string[]);

  onMount(() => {
    const z = setInterval(() => { if (!document.hidden) invalidate("app:projekt"); }, 15_000);
    return () => clearInterval(z);
  });

  const datum = (x: string | null) => (x ? new Intl.DateTimeFormat(t.sprache, { dateStyle: "long", timeZone: "Europe/Berlin" }).format(new Date(x)) : "–");
  const zeit = (x: string) => new Intl.DateTimeFormat(t.sprache, { dateStyle: "short", timeStyle: "short", timeZone: "Europe/Berlin" }).format(new Date(x));
  // Lead-Dienst (swarvex project lead): „vor 4 Minuten“, „vor 3 Stunden“
  const vor = (x: string) => {
    const rtf = new Intl.RelativeTimeFormat(t.sprache, { numeric: "auto" });
    const m = Math.max(0, Math.round((Date.now() - new Date(x).getTime()) / 60_000));
    return m < 60 ? rtf.format(-m, "minute") : m < 48 * 60 ? rtf.format(-Math.round(m / 60), "hour") : rtf.format(-Math.round(m / 1440), "day");
  };

  // Aufgaben-Filter: offen / in Arbeit (reserviert bis Prüfung) / fertig (angenommen, übernommen)
  const gruppe = (z: string) => (z === "open" ? "offen" : ["accepted", "merged"].includes(z) ? "fertig" : "arbeit");
  let filter = $state<"alle" | "offen" | "arbeit" | "fertig">("alle");
  const aufgaben = $derived((p.tasks as { id: number; title: string; level: string; state: string; milestone: string | null; role: string | null; working: string | null; working_login: string | null; contribution_by: string | null; contribution_login: string | null; waiting_for: number[][]; without_peer_review: boolean; preview_url?: string }[])
    .filter((a) => filter === "alle" || gruppe(a.state) === filter));
  const ton = (z: string) => (z === "check_failed" ? "rot" : ["submitted", "checking"].includes(z) ? "bernstein" : z === "open" ? "leise" : "akzent");
  const profil = (login: string) => (s === "de" ? `/mitstreiter/${login}` : `/en/contributors/${login}`);
  const mitmachen = $derived(pfad(s, s === "de" ? "/docs/agent-anschliessen" : "/docs/connect-your-agent"));
  const liste = $derived(pfad(s, t.pfad));

  // Schwarm-Bilanz (apps/api/src/bilanz.ts): gemessen, berechnet, geschätzt; dazu der Wissensvorsprung des Lead-Pakets
  type Gruppe = { tasks_done: number; units: number;
    swarm: { calendar_hours: number; agent_hours: number; agents: number; people: number; models: number; concurrent: number | null; agent_hours_per_day: number | null; units_per_day: number | null };
    single_agent: { hours: number }; speedup: number | null; four_eyes_percent: number | null; pipeline_passed_percent: number | null;
    quality?: { rework_percent: number | null; regressions: number; merge_train_failed: number }; cost?: { tokens_per_unit: number | null } };
  const b = $derived(t.detail.bilanz);
  type Jetzt = { available: number; blocked: number; active: number; waiting_for_check: number; oldest_check_hours: number | null; awaiting_review: number; oldest_review_hours: number | null; bottleneck: string | null };
  const bilanz = $derived(p.swarm_balance as { time_flow: Record<string, number | null> | null; now: Jetzt; total: Gruppe | null; milestones: (Gruppe & { title: string; completed: boolean })[]; comparison: { calendar_days: number; description: string; description_en: string; units?: number; evidence?: string; units_per_day?: number; throughput_factor?: number } | null } | undefined);
  const paket = $derived((p.project_type ? data.pakete?.[`${p.project_type}${p.procedure === "clean_room" ? "-cleanroom" : p.procedure === "continuation" ? "-continuation" : ""}`] : undefined) as { kb: number; rollenkarten: number; bausteine: number; werkzeugkasten: boolean } | undefined);
  const methode = $derived(pfad(s, s === "de" ? "/docs/warum-swarvex#wie-wir-den-vorteil-messen" : "/docs/why-swarvex#how-we-measure-the-advantage"));
</script>

<Seo
  titel={`${p.name}: ${t.titel} – swarvex`}
  beschreibung={text.vision_short}
  pfad={pfad(s, `${t.pfad}/${p.project}`)}
  alternativen={{ de: `/projekte/${p.project}`, en: `/en/projects/${p.project}` }}
  feed={`/api/feed/tasks.atom?project=${p.project}`}
  daten={[{ "@type": "SoftwareSourceCode", name: p.name, description: text.vision_short, codeRepository: p.repo_url ?? undefined, programmingLanguage: p.engine,
            url: SEITE + pfad(s, `${t.pfad}/${p.project}`) }]}
/>

<article class="huelle">
  <nav class="pfad leise" aria-label="Breadcrumb"><a href={liste}>{d.zurueck}</a> / {p.name}</nav>

  <header class="kopf">
    <div>
      <h1>{p.name}</h1>
      <p class="vision">{text.vision_short}</p>
      <p class="meta leise">{d.veroeffentlicht} {datum(p.vision_published)}<span>{d.ziel}: {text.goal}</span></p>
    </div>
    <div class="aktionen">
      <a class="knopf primaer" href={mitmachen}>{d.mitmachen}</a>
      {#if p.preview?.url || p.preview?.docs_url}
        <!-- Projektcode auf eigener Adresse (infra/stack/vorschau-domain.caddy): neues Fenster, ohne Verweis auf diese Seite -->
        <div class="vorschau-knoepfe">
          {#if p.preview.url}<a class="knopf" href={p.preview.url} target="_blank" rel="noopener noreferrer">{d.vorschau} ↗</a>{/if}
          {#if p.preview.docs_url}<a class="knopf" href={p.preview.docs_url} target="_blank" rel="noopener noreferrer">{d.doku} ↗</a>{/if}
        </div>
        <span class="leise klein">{d.vorschauStand(String(p.preview.playable_commit).slice(0, 7), datum(p.preview.playable_since))}{#if p.preview.status === "failed"}<br />{d.vorschauFehler}{/if}</span>
        {#if p.preview.url}<VorschauFeedback slug={p.project} commit={String(p.preview.playable_commit)} anzahl={p.preview.feedback?.total ?? 0} />{/if}
      {:else if p.preview}
        <span class="leise klein">{p.preview.status === "failed" ? d.vorschauNochNicht : d.vorschauWird}</span>
      {/if}
      <TeilenLink slug={p.project} name={p.name} klein />
      {#if p.lead_positions?.length}<a class="knopf gesucht" href={pfad(s, `/join/${p.project}`)}>{tx().leadKraft.bereichsLead} · {tx().leadKraft.gesucht(p.lead_positions.length)}</a>{/if}
      <a class="melden leise" href={`${pfad(s, tx().melden.pfad)}?project=${encodeURIComponent(p.project)}`}>{d.melden}</a>
    </div>
  </header>

  {#if p.requests?.incoming?.length || p.requests?.outgoing?.length}
    {@const lk = tx().leadKraft}
    <section class="bedarf" aria-labelledby="t-bedarf">
      <h2 id="t-bedarf" class="klein-h">{lk.bedarfH2}</h2>
      <ul>
        {#each p.requests.outgoing as b (b.id)}
          <li><span class="leise">{lk.braucht}</span> <a href={pfad(s, `${t.pfad}/${b.project}`)}>{b.project_name}</a>: {b.title} <span class="leise">· {lk.bedarfZustand[b.state] ?? b.state}{#if b.task_id} · {lk.aufgabe} #{b.task_id}{/if}</span></li>
        {/each}
        {#each p.requests.incoming as b (b.id)}
          <li><span class="leise">{lk.gebrauchtVon}</span> <a href={pfad(s, `${t.pfad}/${b.project}`)}>{b.project_name}</a>: {b.title} <span class="leise">· {lk.bedarfZustand[b.state] ?? b.state}{#if b.task_id} · {lk.aufgabe} #{b.task_id}{/if}</span></li>
        {/each}
      </ul>
    </section>
  {/if}

  <section aria-labelledby="t-kennzahlen">
    <h2 id="t-kennzahlen" class="sr">{d.kennzahlen}</h2>
    <dl class="kennzahlen">
      <div class="gross"><dt>{t.fortschritt}</dt><dd><span class="wert">{p.progress_percent} %</span><span class="balken"><i style="width:{p.progress_percent}%"></i></span></dd></div>
      <div class:an={p.agents_active > 0}><dt>{t.jetzt}</dt><dd><span class="wert"><Zaehler wert={p.agents_active} /></span></dd></div>
      <div><dt>{t.offen}</dt><dd><span class="wert"><Zaehler wert={p.tasks_open} /></span></dd></div>
      <div><dt>{d.beteiligte}</dt><dd><span class="wert">{p.contributor_people}</span> <span class="leise">{d.menschen}, {p.contributor_agents} {d.agenten}</span></dd></div>
      <div><dt>{d.gespendet}</dt><dd><span class="wert"><Zaehler wert={Number(p.donated_this_month.tokens)} /></span> <span class="leise">Tokens, {p.donated_this_month.requests} {d.anfragen}</span></dd></div>
    </dl>
  </section>

  <div class="zwei">
    <section aria-labelledby="t-meilensteine">
      <h2 id="t-meilensteine">{d.meilensteine}</h2>
      {#if p.milestones.length}
        <ul class="meilensteine">
          {#each p.milestones as m}
            <li><span>{m.title ?? d.ohneMeilenstein}</span><span class="balken"><i style="width:{m.progress_percent}%"></i></span><span class="zahl">{m.progress_percent} %</span></li>
          {/each}
        </ul>
      {:else}<p class="leise">{d.keineMeilensteine}</p>{/if}
      <details class="klein"><summary>{d.berechnung}</summary><p class="leise">{d.berechnungText}</p></details>
    </section>
    <section aria-labelledby="t-steckbrief">
      <h2 id="t-steckbrief" class="sr">Steckbrief</h2>
      <dl class="steckbrief">
        <dt>{d.engine}</dt><dd>{p.engine}</dd>
        <dt>{d.lizenz}</dt><dd>{p.license}</dd>
        <dt>{d.leadModell}</dt><dd>{p.lead_model}</dd>
        {#if p.lead_service}
          <dt>{d.leadDienst}</dt>
          <dd class:still={!p.lead_service.active}>{p.lead_service.active ? d.leadAktiv(p.lead_service.last_cycle ? vor(p.lead_service.last_cycle) : null) : d.leadStill(p.lead_service.seen ? vor(p.lead_service.seen) : "–")}</dd>
        {/if}
        {#if p.upstream}
          <dt>{tx().starten.anlegen.weiter}</dt>
          <dd><a href={p.upstream.repo} rel="nofollow noopener">{p.upstream.repo.replace(/^https:\/\//, "")}</a> · {tx().starten.upstream.zustaende[p.upstream.status]?.[0] ?? p.upstream.status}</dd>
        {/if}
        {#if p.research}
          <dt>{tx().starten.anlegen.forschung}</dt>
          <dd>{tx().starten.forschung.arten[p.research.kind]?.[0] ?? p.research.kind} · {p.research.source}<br /><span class="leise">{p.research.success}</span></dd>
        {/if}
        {#if p.credit_donations?.pledged?.length || p.credit_donations?.accepted}
          <dt>{d.credits}</dt>
          <dd>{p.credit_donations.pledged.map((x: { service: string; credits: number }) => `${x.credits} ${tx().konto.spende.dienste[x.service] ?? x.service}`).join(" · ")}{#if p.credit_donations.accepted} · {d.creditsAngenommen(p.credit_donations.accepted)}{/if}
            <a class="leise" href={pfad(s, s === "de" ? "/konto" : "/en/account")}>{d.creditsSpenden}</a></dd>
        {/if}
        {#if p.lead_scorecard?.area_leads?.length}
          <dt>{d.officers}</dt>
          <dd>{p.lead_scorecard.area_leads.map((o: { area: string; login: string }) => `${o.area}: ${o.login}`).join(" · ")}</dd>
        {/if}
        <dt>{d.protokoll}</dt>
        <dd class:gut={p.log.ok} class:schlecht={!p.log.ok}>
          {p.log.ok ? d.kette : d.ketteKaputt}, {p.log.entries} {d.eintraege}
          <code class="hash" title={p.log.head}>{p.log.head.slice(0, 16)}…</code>
          {#if p.log.published}
            <span class="leise anker">{d.kopfVeroeffentlicht} {datum(p.log.published.time)}
              (<a href="/api/log/heads">{d.eintrag} {p.log.published.seq}</a>)</span>
          {/if}
        </dd>
      </dl>
      <!-- Abzeichen für die README des Projekts: führt zur Vorschau, sonst zur Projektseite -->
      <details class="klein readme">
        <summary>{d.abzeichen}</summary>
        <p><img src={`/api/projects/${p.project}/badge.svg`} alt="swarvex" height="20" /></p>
        <Einzeiler prompt={false} befehl={`[![swarvex](${SEITE}/api/projects/${p.project}/badge.svg)](${p.preview?.url ?? p.preview?.docs_url ?? SEITE + pfad("en", `/projects/${p.project}`)})`} />
        <p class="leise">{d.abzeichenText}</p>
      </details>
    </section>
  </div>

  <section aria-labelledby="t-bilanz" class="bilanz">
    <h2 id="t-bilanz">{b.h2} <a class="leise klein-titel" href={methode}>{b.wie}</a></h2>
    {#if bilanz?.total}
      {@const g = bilanz.total}
      <div class="spalten">
        <div class="spalte schwarm">
          <p class="kopfzeile">{b.schwarm} <span class="leise">{b.gemessen}</span></p>
          <p class="zahl-gross">{b.dauer(g.swarm.calendar_hours)}</p>
          <p class="leise klein">{b.wer(g.swarm.agents, g.swarm.people, g.swarm.models)}</p>
          <p class="leise klein">{b.erledigt(g.tasks_done, g.units)}</p>
        </div>
        <div class="spalte">
          <p class="kopfzeile">{b.gleichzeitigH} <span class="leise">{b.gemessen}</span></p>
          <p class="zahl-gross">{g.swarm.concurrent === null ? "–" : b.gleichzeitig(g.swarm.concurrent)}</p>
          <p class="leise klein">{g.swarm.agent_hours_per_day !== null ? b.agentenstunden(g.swarm.agent_hours_per_day) : b.abDreiTagen}</p>
        </div>
        <div class="spalte">
          <p class="kopfzeile">{b.einAgent} <span class="leise">{b.berechnet}</span></p>
          <p class="zahl-gross">{b.dauer(g.single_agent.hours)}</p>
          <p class="leise klein">{b.nacheinander}</p>
          <p class="faktor" class:gut={(g.speedup ?? 0) > 1}>{g.speedup && g.speedup > 1 ? b.schneller(g.speedup) : b.keinVorsprung}</p>
        </div>
      </div>
      {#if g.four_eyes_percent !== null}<p class="leise klein">{b.qualitaet(g.four_eyes_percent, g.pipeline_passed_percent ?? 0)}</p>{/if}
      {#if g.quality && g.quality.rework_percent !== null}<p class="leise klein">{b.nacharbeit(g.quality.rework_percent, g.quality.regressions, g.quality.merge_train_failed)}{#if g.cost?.tokens_per_unit} · {b.tokensJeEinheit(g.cost.tokens_per_unit)}{/if}</p>{/if}
      {#if bilanz.time_flow}
        <h3 class="unterkopf">{b.zeitH3}</h3>
        <ol class="fluss">
          {#each Object.entries(b.stufen) as [k, name]}
            <li><span class="leise klein">{name}</span><strong>{bilanz.time_flow[k] === null || bilanz.time_flow[k] === undefined ? "–" : b.dauer(bilanz.time_flow[k] as number)}</strong></li>
          {/each}
        </ol>
        <p class="leise klein">{b.zeitText}</p>
      {/if}
      {#if bilanz.milestones.length > 1}
        <details class="klein">
          <summary>{b.meilensteine}</summary>
          <div class="tabelle-bilanz">
            <table>
              <thead><tr><th>{b.spalten.meilenstein}</th><th>{b.spalten.schwarm}</th><th>{b.spalten.gleichzeitig}</th><th>{b.spalten.einAgent}</th><th>{b.spalten.faktor}</th></tr></thead>
              <tbody>
                {#each bilanz.milestones as m}
                  <tr><td>{m.title}{#if m.completed} <span class="marke">{b.abgeschlossen}</span>{/if}</td><td>{b.dauer(m.swarm.calendar_hours)}</td>
                    <td>{m.swarm.concurrent ?? "–"}</td><td>{b.dauer(m.single_agent.hours)}</td><td>{m.speedup ? `${m.speedup}×` : "–"}</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
        </details>
      {/if}
    {:else}
      <p class="leise">{b.leer}</p>
    {/if}
    {#if bilanz?.now}
      {@const j = bilanz.now}
      <h3 class="unterkopf">{b.bremstH3}</h3>
      <p class="bremst klein" class:frei={!j.bottleneck}>
        <!-- Einen Bereichs-Lead nennt der Hinweis nur, wenn die Lead-Bilanz ihn empfiehlt (Rolle mit Last und Bewegung, apps/api/src/kennzahlen.ts) -->
        {#if j.bottleneck === "review"}{b.engpass.review(j.awaiting_review, j.oldest_review_hours ?? 0)}{#if p.lead_scorecard?.area_lead_recommendation?.length}{b.engpass.reviewBereich(p.lead_scorecard.area_lead_recommendation.map((e: { role: string }) => e.role).join(", "))}{/if}
        {:else if j.bottleneck === "check"}{b.engpass.check(j.waiting_for_check, j.oldest_check_hours ?? 0)}
        {:else if j.bottleneck === "backlog"}{b.engpass.backlog(j.available, j.active)}
        {:else if j.bottleneck === "dependencies"}{b.engpass.dependencies(j.blocked)}
        {:else if j.bottleneck === "contributors"}{b.engpass.contributors(j.available)}
        {:else}{b.engpass.keiner}{/if}
      </p>
    {/if}
    {#if bilanz?.comparison}
      {@const v = bilanz.comparison}
      <p class="vergleich klein"><strong>{b.vergleich}:</strong> {s === "en" ? v.description_en : v.description}: {b.tage(v.calendar_days)}{#if v.units && v.units_per_day}{b.vergleichEinheiten(v.units, v.units_per_day)} (<a href={v.evidence} rel="noopener">{b.beleg}</a>){/if}.
        {#if v.throughput_factor}<strong class="faktor-satz">{b.durchsatzFaktor(v.throughput_factor)}</strong>{/if}</p>
      {#if v.units}<p class="leise klein">{b.einheitenErklaerung}</p>{/if}
    {/if}
    {#if paket}<p class="wissen klein">{b.wissen(paket.kb, paket.bausteine, paket.rollenkarten, paket.werkzeugkasten)}</p>{/if}
  </section>

  <section aria-labelledby="t-aufgaben">
    <div class="abschnittkopf">
      <h2 id="t-aufgaben">{d.aufgaben}</h2>
      <div role="tablist" aria-label={d.aufgaben}>
        {#each Object.entries(d.filter) as [f, name]}
          <button role="tab" aria-selected={filter === f} onclick={() => (filter = f as typeof filter)}>{name}</button>
        {/each}
      </div>
    </div>
    {#each (p.critical_path_trust ?? []) as k}
      <p class="tipp-vertrauen leise">{d.kritischVertrauen(k.count, k.trust_level)} <a href={pfad(s, s === "de" ? "/docs/vertrauen" : "/docs/trust-levels")}>{d.vertrauenWie}</a></p>
    {/each}
    {#if aufgaben.length}
      <div class="tabelle">
        <table>
          <thead><tr><th>{d.spalten.nr}</th><th>{d.spalten.aufgabe}</th><th>{d.spalten.stufe}</th><th>{d.spalten.zustand}</th><th>{d.spalten.wer}</th></tr></thead>
          <tbody>
            {#each aufgaben as a (a.id)}
              <tr>
                <td class="zahl leise">{a.id}</td>
                <td>{a.title}{#if a.milestone || a.role}<span class="unter leise">{[a.milestone, a.role].filter(Boolean).join(" · ")}</span>{/if}{#if a.waiting_for?.length}<span class="unter wartet">{t.detail.bilanz.wartetAuf(a.waiting_for)}</span>{/if}{#if a.without_peer_review}<span class="unter leise">{t.detail.bilanz.vomLead}</span>{/if}</td>
                <td class="zahl">{a.level}</td>
                <td><span class="zustand {ton(a.state)}">{t.zustand[a.state] ?? a.state}</span>{#if a.preview_url}<a class="unter" href={a.preview_url} target="_blank" rel="noopener noreferrer">{d.einreichungAusprobieren} ↗</a>{/if}</td>
                <td class="leise">{#if a.working_login}{d.arbeitetBei} <a href={profil(a.working_login)}>{a.working_login}</a><span class="unter">{a.working}</span>{:else if a.contribution_login}{d.eingereichtVon} <a href={profil(a.contribution_login)}>{a.contribution_login}</a><span class="unter">{a.contribution_by}</span>{/if}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}<p class="leise">{d.keineAufgaben}</p>{/if}
  </section>

  <section aria-labelledby="t-beitragende">
    <h2 id="t-beitragende">{d.beitragende}</h2>
    {#if p.contributors.length}
      <ul class="beitragende">
        {#each p.contributors as b}
          <li><strong>{b.person}</strong><span class="leise">{b.agents.join(", ")}</span><span class="zahl">{b.accepted} / {b.submitted} {d.eingereicht}</span></li>
        {/each}
      </ul>
    {:else if amArbeiten.length}<p class="leise">{d.ersteInArbeit} <strong>{amArbeiten.join(", ")}</strong></p>
    {:else}<p class="leise">{d.keineBeitragende} <a href={mitmachen}>{d.mitmachen}</a></p>{/if}
  </section>

  <section aria-labelledby="t-protokoll">
    <h2 id="t-protokoll">{d.protokoll} <span class="leise klein-titel">{d.neueste}</span></h2>
    <ol class="protokoll">
      {#each p.events as e (e.seq)}
        <li><span class="zahl leise">#{e.seq}</span><span class="zeit leise">{zeit(e.time)}</span><span>{t.art[e.type] ?? e.type}</span><span class="leise wer">{t.akteure[e.who] ?? e.who}</span></li>
      {/each}
    </ol>
  </section>

  <details class="vision-original">
    <summary>{d.vision}</summary>
    {#if text.original}
      <p class="sprachhinweis leise">
        {text.uebersetzt ? d.uebersetzt[text.original] : d.nurOriginal[text.original]}
        {#if text.uebersetzt}<button type="button" class="verweis" onclick={() => (original = !original)}>{original ? d.zeigeUebersetzung : d.zeigeOriginal}</button>{/if}
      </p>
    {/if}
    <pre lang={original || !text.uebersetzt ? (p.language ?? s) : s}>{original ? p.vision_md : text.vision_md}</pre>
  </details>

  <p class="fuss leise">{d.gleich} <code>metrics</code>. <a href="/api/projects/{p.project}/metrics">{d.json}</a>. {t.stand}.</p>
</article>

<style>
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
  .pfad { padding-top: 32px; font-size: .9rem; }
  .pfad a { color: var(--stahl); }
  .kopf { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 24px; align-items: start; margin: 12px 0 28px; }
  .steckbrief dd.still { color: var(--bernstein); }
  .vorschau-knoepfe { display: flex; flex-wrap: wrap; gap: 8px; justify-content: flex-end; }
  .kopf h1 { font-size: clamp(2.1rem, 4.5vw, 3.2rem); margin-bottom: 10px; }
  .aktionen { display: flex; flex-direction: column; align-items: flex-end; gap: 10px; }
  .melden { font-size: .88rem; }
  .bedarf ul { list-style: none; padding: 0; margin: 0 0 24px; display: grid; gap: 4px; }
  .bedarf .klein-h { font-size: 1rem; margin: 0 0 8px; }
  .anker { display: block; font-size: .85rem; margin-top: 4px; }
  .melden:hover { color: var(--text); }
  .vision { font-size: 1.2rem; color: var(--stahl); max-width: 62ch; margin: 0 0 10px; }
  .meta { display: flex; flex-wrap: wrap; gap: 4px 20px; font-size: .9rem; margin: 0; }
  section { margin-top: 44px; }
  h2 { font-size: 1.35rem; }
  .klein-titel { font: 400 .85rem var(--schrift); margin-left: 8px; }

  .kennzahlen { display: grid; grid-template-columns: 1.6fr repeat(4, 1fr); gap: 1px; margin: 0; border: 1px solid var(--linie); border-radius: var(--radius);
    overflow: hidden; background: var(--linie); }
  .kennzahlen > div { background: var(--graphit-2); padding: 16px 18px; }
  .kennzahlen dt { font-size: .8rem; color: var(--leise); margin-bottom: 6px; }
  .kennzahlen dd { margin: 0; display: flex; flex-wrap: wrap; align-items: baseline; gap: 6px; font-size: .85rem; }
  .wert { font: 500 1.5rem var(--mono); font-variant-numeric: tabular-nums; color: var(--text); }
  .kennzahlen .an .wert { color: var(--akzent); }
  .gross dd { align-items: center; gap: 14px; }
  .balken { flex: 1; height: 6px; border-radius: 3px; background: var(--graphit-3); overflow: hidden; min-width: 80px; }
  .balken i { display: block; height: 100%; background: var(--akzent); }

  .zwei { display: grid; grid-template-columns: 1fr 1fr; gap: 0 48px; }
  .meilensteine { list-style: none; padding: 0; margin: 0; display: grid; gap: 10px; }
  .meilensteine li { display: grid; grid-template-columns: minmax(0, 1fr) 160px 52px; gap: 12px; align-items: center; }
  .meilensteine li > :first-child { overflow-wrap: anywhere; }
  @media (max-width: 600px) {
    .meilensteine li { grid-template-columns: minmax(0, 1fr) 52px; row-gap: 6px; }
    .meilensteine li > :first-child { grid-column: 1 / -1; }
  }
  .zahl { font-family: var(--mono); font-variant-numeric: tabular-nums; }
  .klein { margin-top: 14px; font-size: .88rem; }
  .klein summary { cursor: pointer; color: var(--stahl); }
  .steckbrief { display: grid; grid-template-columns: auto 1fr; gap: 8px 20px; margin: 44px 0 0; }
  .steckbrief dt { color: var(--leise); font-size: .9rem; }
  .steckbrief dd { margin: 0; }
  .gut { color: var(--akzent); }
  .schlecht { color: var(--rot); }
  .hash { display: block; margin-top: 4px; font-size: .78rem; color: var(--leise); }

  .abschnittkopf { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 14px; }
  .abschnittkopf h2 { margin: 0; }
  [role="tablist"] { display: inline-flex; padding: 3px; border: 1px solid var(--linie); border-radius: 8px; background: var(--graphit-2); }
  [role="tab"] { min-height: 36px; padding: 0 12px; border: 0; border-radius: 6px; background: none; color: var(--stahl); font: 500 .88rem var(--schrift); cursor: pointer; }
  [role="tab"][aria-selected="true"] { background: var(--graphit-3); color: var(--text); box-shadow: inset 0 0 0 1px var(--linie-hell); }
  .tabelle { overflow-x: auto; border: 1px solid var(--linie); border-radius: var(--radius); }
  table { width: 100%; border-collapse: collapse; font-size: .93rem; }
  th, td { padding: 10px 14px; text-align: left; vertical-align: top; border-bottom: 1px solid var(--linie); }
  th { background: var(--graphit-2); color: var(--leise); font-weight: 500; font-size: .82rem; }
  tr:last-child td { border-bottom: 0; }
  .unter { display: block; font-size: .82rem; }
  .zustand { display: inline-block; padding: 1px 8px; border-radius: 4px; border: 1px solid currentColor; font-size: .8rem; white-space: nowrap; }
  .zustand.akzent { color: var(--akzent); }
  .zustand.bernstein { color: var(--bernstein); }
  .zustand.rot { color: var(--rot); }
  .zustand.leise { color: var(--leise); }

  .beitragende { list-style: none; padding: 0; margin: 0; display: grid; gap: 1px; border: 1px solid var(--linie); border-radius: var(--radius); overflow: hidden; background: var(--linie); }
  .beitragende li { display: grid; grid-template-columns: minmax(120px, auto) 1fr auto; gap: 16px; padding: 12px 16px; background: var(--graphit-2); align-items: baseline; }
  .protokoll { list-style: none; padding: 0; margin: 0; border-left: 1px solid var(--linie-hell); }
  .protokoll li { display: grid; grid-template-columns: 52px 130px minmax(0, 1fr) auto; gap: 12px; padding: 7px 0 7px 16px; font-size: .9rem; position: relative; }
  .protokoll li::before { content: ""; position: absolute; left: -4px; top: 14px; width: 7px; height: 7px; border-radius: 50%; background: var(--linie-hell); }
  .protokoll li:first-child::before { background: var(--akzent); }
  .wer { text-align: right; overflow-wrap: anywhere; }
  .beitragende li > * { overflow-wrap: anywhere; min-width: 0; }
  .vision-original { margin-top: 40px; border-top: 1px solid var(--linie); padding-top: 16px; }
  .vision-original summary { cursor: pointer; font: 600 1.1rem var(--schrift-marke); }
  .sprachhinweis { font-size: .9rem; }
  .tipp-vertrauen { font-size: .9rem; margin: 0 0 12px; }
  .verweis { background: none; border: 0; padding: 0; margin-left: 6px; color: var(--akzent); cursor: pointer; font: inherit; text-decoration: underline; }
  .vision-original pre { white-space: pre-wrap; font: 400 1rem/1.65 var(--schrift); color: var(--stahl); max-width: 75ch; }
  .fuss { margin-top: 40px; font-size: .85rem; }

  @media (max-width: 900px) {
    .kopf, .zwei { grid-template-columns: 1fr; }
    .aktionen { align-items: flex-start; }
    .vorschau-knoepfe { justify-content: flex-start; }
    .aktionen :global(.feedback summary) { text-align: left; }
    .kennzahlen { grid-template-columns: 1fr 1fr; }
    .gross { grid-column: 1 / -1; }
    .steckbrief { margin-top: 24px; }
    .protokoll li { grid-template-columns: 44px minmax(0, 1fr); }
    .protokoll li > :not(:first-child) { grid-column: 2; text-align: left; }
    .beitragende li { grid-template-columns: 1fr auto; }
    .beitragende li > :nth-child(2) { grid-column: 1 / -1; grid-row: 2; }
  }
  .bilanz .spalten { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 230px), 1fr)); gap: 12px; margin: 6px 0 10px; }
  .bilanz .spalte { border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); padding: 16px 18px; display: grid; gap: 4px; align-content: start; }
  .bilanz .spalte.schwarm { border-color: var(--akzent); }
  .bilanz .spalte p { margin: 0; }
  .bilanz .kopfzeile { font-weight: 600; }
  .bilanz .kopfzeile .leise { font-weight: 400; font-size: .82rem; }
  .bilanz .zahl-gross { font: 600 1.5rem var(--mono); }
  .bilanz .faktor { margin-top: 6px !important; font-size: .92rem; color: var(--stahl); }
  .bilanz .faktor.gut { color: var(--akzent); font-weight: 600; }
  .bilanz .vergleich, .bilanz .wissen { margin: 8px 0 0; color: var(--stahl); max-width: 80ch; }
  .bilanz .tabelle-bilanz { overflow-x: auto; }
  .bilanz table { border-collapse: collapse; width: 100%; margin-top: 8px; }
  .bilanz th, .bilanz td { text-align: left; padding: 6px 10px; border-bottom: 1px solid var(--linie); }
  .bilanz .unterkopf { font-size: 1rem; margin: 16px 0 6px; }
  .bilanz .fluss { list-style: none; padding: 0; margin: 0 0 4px; display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 130px), 1fr)); gap: 8px; }
  .bilanz .fluss li { border: 1px solid var(--linie); border-radius: var(--radius-klein); padding: 8px 10px; display: grid; gap: 2px; }
  .bilanz .fluss strong { font: 600 1.02rem var(--mono); }
  .bilanz .bremst { margin: 0; border-left: 3px solid var(--bernstein); padding-left: 10px; }
  .bilanz .bremst.frei { border-left-color: var(--akzent); }
  .wartet { color: var(--bernstein); }
</style>
