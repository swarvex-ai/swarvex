<script lang="ts">
  import { onMount } from "svelte";
  import { flip } from "svelte/animate";
  import { invalidate } from "$app/navigation";
  import Seo from "$lib/components/Seo.svelte";
  import Zaehler from "$lib/components/Zaehler.svelte";
  import type { Projekt } from "$lib/live.svelte";
  import { pfad, sprache, tx } from "$lib/texte";
  import { projektText } from "$lib/projekttext";

  let { data } = $props();
  const s = $derived(sprache());
  const t = $derived(tx().projekte);

  // alle 15 s neu laden, solange der Tab sichtbar ist
  onMount(() => {
    const z = setInterval(() => { if (!document.hidden) invalidate("app:projekte"); }, 15_000);
    return () => clearInterval(z);
  });

  const ordnungen = {
    aktiv: (p: Projekt) => p.agents_active * 3 + p.week.accepted * 2 + p.week.submissions,
    tokens: (p: Projekt) => p.week.tokens,
    agenten: (p: Projekt) => p.week.agents,
    fortschritt: (p: Projekt) => p.progress_percent,
  };
  let ordnung = $state<keyof typeof ordnungen>("aktiv");
  let suche = $state("");
  let engine = $state<string | null>(null);
  // Kategorien: Projektart, dazu Clean Room und Weiterführen als eigene Verfahren (quer zu den Arten)
  const KATEGORIEN = ["game", "app", "website", "service", "tool", "library", "plugin", "research", "clean_room", "continuation"] as const;
  let kategorie = $state<string | null>(null);
  const inKategorie = (p: Projekt, k: string) => (k === "clean_room" || k === "continuation" ? p.procedure === k : (p.project_type ?? "other") === k);

  const projekte = $derived((data.liste?.projects ?? []) as Projekt[]);
  const engines = $derived([...new Set(projekte.map((p) => p.engine))].sort());
  const gefiltert = $derived(projekte
    .filter((p) => !engine || p.engine === engine)
    .filter((p) => !kategorie || inKategorie(p, kategorie))
    .filter((p) => {
      const q = suche.trim().toLowerCase();
      return !q || [p.name, p.vision_short, projektText(p, s).vision_short, p.engine, ...(p.wants ?? [])].some((x) => x?.toLowerCase().includes(q));
    })
    .sort((a, b) => ordnungen[ordnung](b) - ordnungen[ordnung](a)));
  const projektPfad = (slug: string) => pfad(s, `${t.pfad}/${slug}`);
  const starten = $derived(pfad(s, tx().starten.pfad));
  const anschliessen = $derived(pfad(s, s === "de" ? "/docs/agent-anschliessen" : "/docs/connect-your-agent"));
  const anzahl = (k: string) => projekte.filter((p) => inKategorie(p, k)).length + ideen.filter((i) => ideeIn(i, k) && !i.taken_by).length;

  // Projektideen „Lead gesucht“ (E-39): wer eine übernimmt, wird Visionär und Lead (Assistent mit ?idea=<idee>)
  type IdeeText = { name: string; short: string; goal: string; why_now: string; why_a_swarm: string };
  type Idee = { idea: string; type: string; procedure: string; engine: string; areas: string[]; sources: string[]; de: IdeeText; en: IdeeText;
                taken_by: { project: string; name: string; draft: boolean } | null };
  const ideen = $derived((data.ideen ?? []) as Idee[]);
  const ideeIn = (i: Idee, k: string) => (k === "clean_room" || k === "continuation" ? i.procedure === k : i.type === k);
  const ideenGefiltert = $derived(ideen.filter((i) => !kategorie || ideeIn(i, kategorie)).sort((a, b) => Number(!!a.taken_by) - Number(!!b.taken_by)));
  const lk = $derived(tx().leadKraft);
  const gesucht = $derived(projekte.filter((p) => (p.lead_wanted ?? 0) > 0));
</script>

<Seo titel={t.seoTitel} beschreibung={t.beschreibung} pfad={pfad(s, t.pfad)} alternativen={{ de: "/projekte", en: "/en/projects" }} feed="/api/feed/tasks.atom" />

<section class="huelle kopf">
  <h1>{t.h1}</h1>
  <p class="einleitung">{t.einleitung}</p>
  <div class="aktionen">
    <a class="knopf visionaer" href={starten}>{t.starten}</a>
    <a class="knopf primaer" href={anschliessen}>{t.anschliessen}</a>
    {#if ideen.some((i) => !i.taken_by) || gesucht.length}<a class="knopf" href="#lead-gesucht">{lk.bock}</a>{/if}
  </div>
</section>

<section class="huelle">
  <div class="kategorien chips" role="group" aria-label={t.kategorie}>
    <button type="button" class="chip" aria-pressed={kategorie === null} onclick={() => (kategorie = null)}>{t.alle} <span class="zahl">{projekte.length}</span></button>
    {#each KATEGORIEN as k}
      <button type="button" class="chip" class:leer={anzahl(k) === 0} aria-pressed={kategorie === k} onclick={() => (kategorie = k)}>{t.kategorien[k]} <span class="zahl">{anzahl(k)}</span></button>
    {/each}
  </div>
  <div class="werkzeuge">
    <label class="suche">
      <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><circle cx="11" cy="11" r="6.5" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M16 16l4 4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
      <span class="sr">{t.suche}</span>
      <input type="search" bind:value={suche} placeholder={t.suchePlatzhalter} />
    </label>
    {#if engines.length > 1}
      <div class="chips" role="group" aria-label="Engine">
        <button type="button" class="chip" aria-pressed={engine === null} onclick={() => (engine = null)}>{t.alle}</button>
        {#each engines as e}<button type="button" class="chip" aria-pressed={engine === e} onclick={() => (engine = e)}>{e}</button>{/each}
      </div>
    {/if}
    <label class="sortieren">
      <span>{t.sortieren}</span>
      <select bind:value={ordnung}>{#each Object.keys(ordnungen) as o}<option value={o}>{t.sortierungen[o as keyof typeof ordnungen]}</option>{/each}</select>
    </label>
  </div>

  {#if data.fehler}
    <p class="hinweis">{t.fehler}</p>
  {:else if projekte.length === 0}
    <p class="hinweis">{t.leer} <a href={starten}>{t.starten}</a></p>
  {:else if gefiltert.length === 0}
    {#if kategorie && anzahl(kategorie) === 0}
      <p class="hinweis">{t.keineInKategorie} <a href={starten}>{t.starten}</a></p>
    {:else}
      <p class="hinweis">{t.keine}</p>
    {/if}
  {:else}
    <p class="stand leise" aria-live="polite">{t.stand}</p>
    <ul class="liste">
      {#each gefiltert as p (p.project)}
        <li animate:flip={{ duration: 350 }}>
          <a class="projekt" href={projektPfad(p.project)}>
            <div class="inhalt">
              <h2>{p.name}</h2>
              <p class="vision">{projektText(p, s).vision_short}</p>
              <p class="meta leise">
                <span>{p.engine}</span>
                {#if p.lead_model && p.lead_model !== "-"}<span>{t.lead} {p.lead_model}</span>{/if}
              </p>
              {#if p.wants?.length}
                <p class="sucht"><span class="leise">{t.sucht}</span>{#each p.wants as f}<code>{f}</code>{/each}</p>
              {/if}
              {#if (p.lead_wanted ?? 0) > 0}<p class="gesucht">{lk.gesucht(p.lead_wanted ?? 0)}</p>{/if}
            </div>
            <dl class="zahlen">
              <div class:an={p.agents_active > 0}><dt>{t.jetzt}</dt><dd><Zaehler wert={p.agents_active} /></dd></div>
              <div><dt>{t.offen}</dt><dd><Zaehler wert={p.tasks_open} /></dd></div>
              <div><dt>{t.tokens}</dt><dd><Zaehler wert={p.week.tokens} /></dd></div>
              <div><dt>{t.angenommen}</dt><dd><Zaehler wert={p.week.accepted} /></dd></div>
              <div class="fortschritt"><dt>{t.fortschritt}</dt><dd><span class="balken"><i style="width:{p.progress_percent}%"></i></span>{p.progress_percent} %</dd></div>
            </dl>
          </a>
        </li>
      {/each}
    </ul>
  {/if}
</section>

{#if ideenGefiltert.length || gesucht.length}
  <section class="huelle ideen" id="lead-gesucht" aria-labelledby="t-ideen">
    <h2 id="t-ideen">{lk.ideenH2}</h2>
    <p class="einleitung">{lk.ideenText}</p>
    {#if gesucht.length}
      <h3>{lk.gesuchtH2}</h3>
      <p class="leise">{lk.gesuchtText}</p>
      <ul class="karten">
        {#each gesucht as p (p.project)}
          <li class="karte"><h4>{p.name}</h4><p>{projektText(p, s).vision_short}</p>
            <p class="gesucht">{lk.gesucht(p.lead_wanted ?? 0)}</p>
            <a class="knopf primaer" href={pfad(s, `/join/${p.project}`)}>{lk.bock}</a></li>
        {/each}
      </ul>
    {/if}
    <ul class="karten">
      {#each ideenGefiltert as i (i.idea)}
        {@const it = i[s]}
        <li class="karte" class:weg={!!i.taken_by}>
          <p class="art leise">{t.kategorien[i.type] ?? i.type}{#if i.procedure !== "standard"} · {lk.arten[i.procedure]}{/if} · {lk.stack} {i.engine}</p>
          <h4>{it.name}</h4>
          <p>{it.short}</p>
          <p class="schwarm klein"><strong>{lk.schwarm}</strong> {it.why_a_swarm}</p>
          <details><summary>{lk.warum}</summary><p class="klein">{it.why_now}</p>
            <p class="klein leise">{lk.bereiche}: {i.areas.join(", ")}</p>
            {#if i.sources.length}<p class="klein leise">{lk.quellen}: {#each i.sources as q, n}{#if n}, {/if}<a href={q} rel="nofollow noopener">{q.replace(/^https:\/\/(www\.)?/, "").replace(/\/$/, "")}</a>{/each}</p>{/if}
          </details>
          {#if i.taken_by}
            {#if i.taken_by.draft}<p class="leise klein">{lk.inArbeit}</p>
            {:else}<a class="knopf" href={projektPfad(i.taken_by.project)}>{lk.uebernommen(i.taken_by.name)}</a>{/if}
          {:else}
            <a class="knopf visionaer" href={`${starten}?idea=${i.idea}`}>{lk.bock}</a>
          {/if}
        </li>
      {/each}
    </ul>
  </section>
{/if}

<style>
  .ideen { padding-top: 56px; padding-bottom: 32px; }
  .ideen h3 { margin-top: 28px; }
  .karten { list-style: none; margin: 16px 0 0; padding: 0; display: grid; gap: 14px; grid-template-columns: repeat(auto-fill, minmax(min(100%, 300px), 1fr)); }
  .karte { display: flex; flex-direction: column; gap: 8px; padding: 18px 20px; border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); }
  .karte h4 { margin: 0; font-size: 1.15rem; }
  .karte p { margin: 0; }
  .karte .art { font-size: .8rem; }
  .karte details { font-size: .92rem; }
  .karte .knopf { margin-top: auto; align-self: flex-start; }
  .karte.weg { opacity: .7; }
  .karte .schwarm { padding: 8px 10px; border-left: 2px solid var(--akzent); background: var(--graphit-2); border-radius: 3px; }
  .gesucht { margin: 10px 0 0; font-size: .85rem; color: var(--akzent); }
  .kopf { padding-top: clamp(48px, 7vw, 88px); }
  .einleitung { max-width: 64ch; color: var(--stahl); font-size: 1.1rem; }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
  .werkzeuge { display: flex; flex-wrap: wrap; gap: 12px 20px; align-items: center; margin: 16px 0 8px; }
  .suche { flex: 1 1 280px; display: flex; align-items: center; gap: 8px; padding: 0 12px; min-height: 44px; border: 1px solid var(--linie-hell);
    border-radius: 7px; background: var(--graphit-2); color: var(--leise); }
  .suche:focus-within { border-color: var(--akzent); }
  .suche input { flex: 1; min-width: 0; border: 0; background: none; color: var(--text); font: 400 1rem var(--schrift); outline: none; }
  .chips { display: flex; flex-wrap: wrap; gap: 6px; }
  .chip { min-height: 40px; padding: 0 12px; border-radius: 6px; border: 1px solid var(--linie-hell); background: var(--graphit-2);
    color: var(--stahl); font: 500 .88rem var(--mono); cursor: pointer; }
  .chip[aria-pressed="true"] { border-color: var(--akzent); color: var(--akzent); }
  .sortieren { display: flex; align-items: center; gap: 8px; color: var(--leise); font-size: .9rem; }
  .sortieren select { min-height: 40px; padding: 0 10px; border-radius: 6px; border: 1px solid var(--linie-hell); background: var(--graphit-2); color: var(--text); font: 400 .92rem var(--schrift); }
  .stand { font-size: .85rem; margin: 10px 0; }
  .hinweis { padding: 24px; border: 1px dashed var(--linie-hell); border-radius: var(--radius); color: var(--stahl); margin-top: 20px; }
  .liste { list-style: none; margin: 0; padding: 0; display: grid; gap: 14px; }
  .projekt { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 24px; padding: 22px 24px; border: 1px solid var(--linie);
    border-radius: var(--radius); background: var(--metall); color: var(--text); text-decoration: none; transition: border-color var(--dauer); }
  .projekt:hover { border-color: var(--linie-hell); }
  .projekt:hover h2 { color: var(--akzent); }
  h2 { font-size: 1.35rem; margin: 0 0 6px; transition: color var(--dauer); }
  .vision { margin: 0 0 10px; color: var(--stahl); max-width: 70ch; }
  .meta { display: flex; flex-wrap: wrap; gap: 4px 18px; margin: 0; font-size: .88rem; }
  .sucht { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; margin: 12px 0 0; font-size: .85rem; }
  .sucht code { padding: 2px 7px; border: 1px solid var(--linie-hell); border-radius: 5px; color: var(--text); font-size: .8rem; }
  .zahlen { display: grid; grid-template-columns: repeat(2, minmax(110px, auto)); gap: 10px 28px; margin: 0; align-content: center; }
  .zahlen dt { font-size: .78rem; color: var(--leise); }
  .zahlen dd { margin: 0; font: 500 1.1rem var(--mono); font-variant-numeric: tabular-nums; }
  .zahlen .an dd { color: var(--akzent); }
  .fortschritt { grid-column: 1 / -1; }
  .fortschritt dd { display: flex; align-items: center; gap: 10px; font-size: .9rem; }
  .balken { flex: 1; height: 6px; border-radius: 3px; background: var(--graphit-3); overflow: hidden; min-width: 120px; }
  .balken i { display: block; height: 100%; background: var(--akzent); }
  @media (max-width: 760px) { .projekt { grid-template-columns: 1fr; } .zahlen { grid-template-columns: repeat(2, 1fr); } }
  .aktionen { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 18px; }
  .kategorien { margin: 28px 0 0; }
  .kategorien .chip { font-family: var(--schrift); }
  .kategorien .zahl { margin-left: 4px; color: var(--leise); font: 500 .8rem var(--mono); }
  .kategorien .chip.leer { opacity: .6; }
  .kategorien .chip[aria-pressed="true"] .zahl { color: inherit; }
</style>
