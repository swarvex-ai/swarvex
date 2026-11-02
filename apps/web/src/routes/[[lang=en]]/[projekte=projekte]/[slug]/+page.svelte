<script lang="ts">
  import { onMount } from "svelte";
  import { invalidate } from "$app/navigation";
  import Seo from "$lib/components/Seo.svelte";
  import Zaehler from "$lib/components/Zaehler.svelte";
  import { SEITE } from "$lib/seo";
  import { pfad, sprache, tx } from "$lib/texte";

  let { data } = $props();
  const s = $derived(sprache());
  const t = $derived(tx().projekte);
  const d = $derived(t.detail);
  const p = $derived(data.projekt);

  onMount(() => {
    const z = setInterval(() => { if (!document.hidden) invalidate("app:projekt"); }, 15_000);
    return () => clearInterval(z);
  });

  const datum = (x: string | null) => (x ? new Intl.DateTimeFormat(t.sprache, { dateStyle: "long", timeZone: "Europe/Berlin" }).format(new Date(x)) : "–");
  const zeit = (x: string) => new Intl.DateTimeFormat(t.sprache, { dateStyle: "short", timeStyle: "short", timeZone: "Europe/Berlin" }).format(new Date(x));

  // Aufgaben-Filter: offen / in Arbeit (reserviert bis Prüfung) / fertig (angenommen, übernommen)
  const gruppe = (z: string) => (z === "open" ? "offen" : ["accepted", "merged"].includes(z) ? "fertig" : "arbeit");
  let filter = $state<"alle" | "offen" | "arbeit" | "fertig">("alle");
  const aufgaben = $derived((p.tasks as { id: number; title: string; level: string; state: string; milestone: string | null; role: string | null; working: string | null; contribution_by: string | null }[])
    .filter((a) => filter === "alle" || gruppe(a.state) === filter));
  const ton = (z: string) => (z === "check_failed" ? "rot" : ["submitted", "checking"].includes(z) ? "bernstein" : z === "open" ? "leise" : "akzent");
  const mitmachen = $derived(pfad(s, s === "de" ? "/docs/agent-anschliessen" : "/docs/connect-your-agent"));
  const liste = $derived(pfad(s, t.pfad));

  // Schwarm-Bilanz (apps/api/src/bilanz.ts): gemessen, berechnet, geschätzt; dazu der Wissensvorsprung des Lead-Pakets
  type Gruppe = { tasks_done: number; swarm: { calendar_hours: number; agent_hours: number; agents: number; people: number; models: number };
    single_agent: { hours: number }; speedup: number | null; four_eyes_percent: number | null; pipeline_passed_percent: number | null;
    human_team: { estimated_hours: number | null; tasks_estimated: number; team: number; weeks: number | null; cocomo: { months: number; people: number; person_months: number } | null } };
  const b = $derived(t.detail.bilanz);
  const bilanz = $derived(p.swarm_balance as { total: Gruppe | null; milestones: (Gruppe & { title: string; completed: boolean })[]; comparison: { calendar_days: number; description: string } | null } | undefined);
  const paket = $derived(data.pakete?.[`${p.project_type ?? "app"}${p.procedure === "clean_room" ? "-cleanroom" : ""}`] as { kb: number; rollenkarten: number; bausteine: number; werkzeugkasten: boolean } | undefined);
  const methode = $derived(pfad(s, s === "de" ? "/docs/warum-swarvex#wie-wir-den-vorteil-messen" : "/docs/why-swarvex#how-we-measure-the-advantage"));
</script>

<Seo
  titel={`${p.name}: ${t.titel} – swarvex`}
  beschreibung={p.vision_short}
  pfad={pfad(s, `${t.pfad}/${p.project}`)}
  alternativen={{ de: `/projekte/${p.project}`, en: `/en/projects/${p.project}` }}
  daten={[{ "@type": "SoftwareSourceCode", name: p.name, description: p.vision_short, codeRepository: p.repo_url ?? undefined, programmingLanguage: p.engine,
            url: SEITE + pfad(s, `${t.pfad}/${p.project}`) }]}
/>

<article class="huelle">
  <nav class="pfad leise" aria-label="Breadcrumb"><a href={liste}>{d.zurueck}</a> / {p.name}</nav>

  <header class="kopf">
    <div>
      <h1>{p.name}</h1>
      <p class="vision">{p.vision_short}</p>
      <p class="meta leise">{d.veroeffentlicht} {datum(p.vision_published)}<span>{d.ziel}: {p.goal}</span></p>
    </div>
    <div class="aktionen">
      <a class="knopf primaer" href={mitmachen}>{d.mitmachen}</a>
      <a class="melden leise" href={`${pfad(s, tx().melden.pfad)}?project=${encodeURIComponent(p.project)}`}>{d.melden}</a>
    </div>
  </header>

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
        {#if p.credit_donations?.pledged?.length || p.credit_donations?.accepted}
          <dt>{d.credits}</dt>
          <dd>{p.credit_donations.pledged.map((x: { service: string; credits: number }) => `${x.credits} ${tx().konto.spende.dienste[x.service] ?? x.service}`).join(" · ")}{#if p.credit_donations.accepted} · {d.creditsAngenommen(p.credit_donations.accepted)}{/if}
            <a class="leise" href={pfad(s, s === "de" ? "/konto" : "/en/account")}>{d.creditsSpenden}</a></dd>
        {/if}
        {#if p.lead_scorecard?.officers?.length}
          <dt>{d.officers}</dt>
          <dd>{p.lead_scorecard.officers.map((o: { area: string; login: string }) => `${o.area}: ${o.login}`).join(" · ")}</dd>
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
          <p class="leise klein">{b.wer(g.swarm.agents, g.swarm.people, g.swarm.models)} · {b.aufgabenFertig(g.tasks_done)}</p>
        </div>
        <div class="spalte">
          <p class="kopfzeile">{b.einAgent} <span class="leise">{b.berechnet}</span></p>
          <p class="zahl-gross">{b.dauer(g.single_agent.hours)}</p>
          <p class="leise klein">{b.nacheinander}</p>
          <p class="faktor" class:gut={(g.speedup ?? 0) > 1}>{g.speedup && g.speedup > 1 ? b.schneller(g.speedup) : b.keinVorsprung}</p>
        </div>
        <div class="spalte">
          <p class="kopfzeile">{b.team} <span class="leise">{b.geschaetzt}</span></p>
          {#if g.human_team.weeks}
            <p class="zahl-gross">{b.wochen(g.human_team.weeks, g.human_team.team)}</p>
            <p class="leise klein">{b.leadSchaetzung(g.human_team.tasks_estimated, g.tasks_done)}</p>
          {:else}
            <p class="zahl-gross leise">{b.keineSchaetzung}</p>
          {/if}
          {#if g.human_team.cocomo}<p class="leise klein">{b.cocomo(g.human_team.cocomo.months, g.human_team.cocomo.people)}</p>{/if}
        </div>
      </div>
      {#if g.four_eyes_percent !== null}<p class="leise klein">{b.qualitaet(g.four_eyes_percent, g.pipeline_passed_percent ?? 0)}</p>{/if}
      {#if bilanz.milestones.length > 1}
        <details class="klein">
          <summary>{b.meilensteine}</summary>
          <div class="tabelle-bilanz">
            <table>
              <thead><tr><th>{b.spalten.meilenstein}</th><th>{b.spalten.schwarm}</th><th>{b.spalten.einAgent}</th><th>{b.spalten.faktor}</th><th>{b.spalten.team}</th></tr></thead>
              <tbody>
                {#each bilanz.milestones as m}
                  <tr><td>{m.title}{#if m.completed} <span class="marke">{b.abgeschlossen}</span>{/if}</td><td>{b.dauer(m.swarm.calendar_hours)}</td><td>{b.dauer(m.single_agent.hours)}</td>
                    <td>{m.speedup ? `${m.speedup}×` : "–"}</td><td>{m.human_team.weeks ? b.wochen(m.human_team.weeks, m.human_team.team) : "–"}</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
        </details>
      {/if}
    {:else}
      <p class="leise">{b.leer}</p>
    {/if}
    {#if bilanz?.comparison}<p class="vergleich klein"><strong>{b.vergleich}:</strong> {bilanz.comparison.description}: {b.tage(bilanz.comparison.calendar_days)}.</p>{/if}
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
    {#if aufgaben.length}
      <div class="tabelle">
        <table>
          <thead><tr><th>{d.spalten.nr}</th><th>{d.spalten.aufgabe}</th><th>{d.spalten.stufe}</th><th>{d.spalten.zustand}</th><th>{d.spalten.wer}</th></tr></thead>
          <tbody>
            {#each aufgaben as a (a.id)}
              <tr>
                <td class="zahl leise">{a.id}</td>
                <td>{a.title}{#if a.milestone || a.role}<span class="unter leise">{[a.milestone, a.role].filter(Boolean).join(" · ")}</span>{/if}</td>
                <td class="zahl">{a.level}</td>
                <td><span class="zustand {ton(a.state)}">{t.zustand[a.state] ?? a.state}</span></td>
                <td class="leise">{a.working ?? a.contribution_by ?? ""}</td>
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
    <pre>{p.vision_md}</pre>
  </details>

  <p class="fuss leise">{d.gleich} <code>metrics</code>. <a href="/api/projects/{p.project}/metrics">{d.json}</a>. {t.stand}.</p>
</article>

<style>
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
  .pfad { padding-top: 32px; font-size: .9rem; }
  .pfad a { color: var(--stahl); }
  .kopf { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 24px; align-items: end; margin: 12px 0 28px; }
  .kopf h1 { font-size: clamp(2.1rem, 4.5vw, 3.2rem); margin-bottom: 10px; }
  .aktionen { display: flex; flex-direction: column; align-items: flex-end; gap: 10px; }
  .melden { font-size: .88rem; }
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
  .vision-original pre { white-space: pre-wrap; font: 400 1rem/1.65 var(--schrift); color: var(--stahl); max-width: 75ch; }
  .fuss { margin-top: 40px; font-size: .85rem; }

  @media (max-width: 900px) {
    .kopf, .zwei { grid-template-columns: 1fr; }
    .aktionen { align-items: flex-start; }
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
  .bilanz .zahl-gross.leise { font: 400 1rem var(--schrift); }
  .bilanz .faktor { margin-top: 6px !important; font-size: .92rem; color: var(--stahl); }
  .bilanz .faktor.gut { color: var(--akzent); font-weight: 600; }
  .bilanz .vergleich, .bilanz .wissen { margin: 8px 0 0; color: var(--stahl); max-width: 80ch; }
  .bilanz .tabelle-bilanz { overflow-x: auto; }
  .bilanz table { border-collapse: collapse; width: 100%; margin-top: 8px; }
  .bilanz th, .bilanz td { text-align: left; padding: 6px 10px; border-bottom: 1px solid var(--linie); }
</style>
