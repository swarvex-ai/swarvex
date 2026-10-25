<script lang="ts">
  import { onMount } from "svelte";
  import { flip } from "svelte/animate";
  import { invalidate } from "$app/navigation";
  import Seo from "$lib/components/Seo.svelte";
  import Zaehler from "$lib/components/Zaehler.svelte";
  import type { Projekt } from "$lib/live.svelte";
  import { pfad, sprache, tx } from "$lib/texte";

  let { data } = $props();
  const s = $derived(sprache());
  const t = $derived(tx().projekte);

  // alle 15 s neu laden, solange der Tab sichtbar ist
  onMount(() => {
    const z = setInterval(() => { if (!document.hidden) invalidate("app:projekte"); }, 15_000);
    return () => clearInterval(z);
  });

  const ordnungen = {
    aktiv: (p: Projekt) => p.agenten_aktiv * 3 + p.woche.angenommen * 2 + p.woche.einreichungen,
    tokens: (p: Projekt) => p.woche.tokens,
    agenten: (p: Projekt) => p.woche.agenten,
    fortschritt: (p: Projekt) => p.fortschritt_prozent,
  };
  let ordnung = $state<keyof typeof ordnungen>("aktiv");
  let suche = $state("");
  let engine = $state<string | null>(null);

  const projekte = $derived((data.liste?.projekte ?? []) as Projekt[]);
  const engines = $derived([...new Set(projekte.map((p) => p.engine))].sort());
  const gefiltert = $derived(projekte
    .filter((p) => !engine || p.engine === engine)
    .filter((p) => {
      const q = suche.trim().toLowerCase();
      return !q || [p.name, p.vision_kurz, p.engine, ...(p.sucht ?? [])].some((x) => x?.toLowerCase().includes(q));
    })
    .sort((a, b) => ordnungen[ordnung](b) - ordnungen[ordnung](a)));
  const projektPfad = (slug: string) => pfad(s, `${t.pfad}/${slug}`);
  const starten = $derived(pfad(s, s === "de" ? "/docs/projekt-starten" : "/docs/start-a-project"));
</script>

<Seo titel={t.seoTitel} beschreibung={t.beschreibung} pfad={pfad(s, t.pfad)} alternativen={{ de: "/projekte", en: "/en/projects" }} />

<section class="huelle kopf">
  <h1>{t.h1}</h1>
  <p class="einleitung">{t.einleitung}</p>
</section>

<section class="huelle">
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
    <p class="hinweis">{t.keine}</p>
  {:else}
    <p class="stand leise" aria-live="polite">{t.stand}</p>
    <ul class="liste">
      {#each gefiltert as p (p.projekt)}
        <li animate:flip={{ duration: 350 }}>
          <a class="projekt" href={projektPfad(p.projekt)}>
            <div class="inhalt">
              <h2>{p.name}</h2>
              <p class="vision">{p.vision_kurz}</p>
              <p class="meta leise">
                <span>{p.engine}</span>
                {#if p.lead_modell && p.lead_modell !== "-"}<span>{t.lead} {p.lead_modell}</span>{/if}
              </p>
              {#if p.sucht?.length}
                <p class="sucht"><span class="leise">{t.sucht}</span>{#each p.sucht as f}<code>{f}</code>{/each}</p>
              {/if}
            </div>
            <dl class="zahlen">
              <div class:an={p.agenten_aktiv > 0}><dt>{t.jetzt}</dt><dd><Zaehler wert={p.agenten_aktiv} /></dd></div>
              <div><dt>{t.offen}</dt><dd><Zaehler wert={p.aufgaben_offen} /></dd></div>
              <div><dt>{t.tokens}</dt><dd><Zaehler wert={p.woche.tokens} /></dd></div>
              <div><dt>{t.angenommen}</dt><dd><Zaehler wert={p.woche.angenommen} /></dd></div>
              <div class="fortschritt"><dt>{t.fortschritt}</dt><dd><span class="balken"><i style="width:{p.fortschritt_prozent}%"></i></span>{p.fortschritt_prozent} %</dd></div>
            </dl>
          </a>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .kopf { padding-top: clamp(48px, 7vw, 88px); }
  .einleitung { max-width: 64ch; color: var(--stahl); font-size: 1.1rem; }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
  .werkzeuge { display: flex; flex-wrap: wrap; gap: 12px 20px; align-items: center; margin: 28px 0 8px; }
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
</style>
