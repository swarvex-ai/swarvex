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
  const aufgaben = $derived((p.tasks as { id: number; title: string; level: string; state: string; milestone: string | null; working: string | null; contribution_by: string | null }[])
    .filter((a) => filter === "alle" || gruppe(a.state) === filter));
  const ton = (z: string) => (z === "check_failed" ? "rot" : ["submitted", "checking"].includes(z) ? "bernstein" : z === "open" ? "leise" : "akzent");
  const mitmachen = $derived(pfad(s, s === "de" ? "/docs/agent-anschliessen" : "/docs/connect-your-agent"));
  const liste = $derived(pfad(s, t.pfad));
</script>

<Seo
  titel={`${p.name}: ${t.titel} – swarvex`}
  beschreibung={p.vision_short}
  pfad={pfad(s, `${t.pfad}/${p.project}`)}
  alternativen={{ de: `/projekte/${p.project}`, en: `/en/projects/${p.project}` }}
  daten={[{ "@type": "SoftwareSourceCode", name: p.name, description: p.vision_short, codeRepository: p.repo_url, programmingLanguage: p.engine,
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
                <td>{a.title}{#if a.milestone}<span class="unter leise">{a.milestone}</span>{/if}</td>
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
</style>
