<script lang="ts">
  import { onMount } from "svelte";
  import { flip } from "svelte/animate";
  import { abonnieren, live, type Projekt } from "$lib/live.svelte";
  import Zaehler from "./Zaehler.svelte";
  import { pfad, sprache, tx } from "$lib/texte";
  const t = $derived(tx().rangliste);

  const ordnungen = {
    aktiv: { wert: (p: Projekt) => p.agenten_aktiv * 3 + p.woche.angenommen * 2 + p.woche.einreichungen },
    tokens: { wert: (p: Projekt) => p.woche.tokens },
    agenten: { wert: (p: Projekt) => p.woche.agenten },
  } as const;
  let ordnung = $state<keyof typeof ordnungen>("aktiv");

  onMount(abonnieren);

  const sortiert = $derived(
    [...(live.daten?.projekte ?? [])].sort((a, b) => ordnungen[ordnung].wert(b) - ordnungen[ordnung].wert(a)).slice(0, 8),
  );
</script>

<div class="kopfzeile">
  <div role="tablist" aria-label={t.sortieren}>
    {#each Object.entries(ordnungen) as [schluessel, o]}
      <button role="tab" aria-selected={ordnung === schluessel} onclick={() => (ordnung = schluessel as keyof typeof ordnungen)}>{t.ordnungen[schluessel as keyof typeof ordnungen]}</button>
    {/each}
  </div>
  <p class="leise stand" aria-live="polite">
    {#if live.daten}{t.live}{:else if live.fehler}{t.fehler}{:else}{t.laedt}{/if}
  </p>
</div>

{#if live.daten && sortiert.length}
  <div class="tabelle" role="table" aria-label={t.tabelle}>
    <div class="zeile kopf" role="row">
      <span role="columnheader">{t.projekt}</span>
      <span role="columnheader" class="zahl">{t.jetzt}</span>
      <span role="columnheader" class="zahl">{t.tokens}</span>
      <span role="columnheader" class="zahl">{t.angenommen}</span>
      <span role="columnheader">{t.fortschritt}</span>
    </div>
    {#each sortiert as p, i (p.projekt)}
      <a class="zeile" class:spitze={i === 0} role="row" href={pfad(sprache(), `${tx().projekte.pfad}/${p.projekt}`)} animate:flip={{ duration: 400 }}>
        <span role="cell" class="projekt">
          <span class="rang" aria-label="{t.rang} {i + 1}">{i + 1}</span>
          <span>
            <strong>{p.name}</strong>
            <span class="leise unter">{p.engine}{#if p.lead_modell && p.lead_modell !== "-"}<span class="lead">{t.lead} {p.lead_modell}</span>{/if}</span>
          </span>
        </span>
        <span role="cell" class="zahl" class:an={p.agenten_aktiv > 0}>
          {#if p.agenten_aktiv > 0}<i class="punkt" aria-hidden="true"></i>{/if}<Zaehler wert={p.agenten_aktiv} />
        </span>
        <span role="cell" class="zahl"><Zaehler wert={p.woche.tokens} /></span>
        <span role="cell" class="zahl"><Zaehler wert={p.woche.angenommen} /></span>
        <span role="cell" class="fortschritt">
          <span class="balken"><i style="width:{p.fortschritt_prozent}%"></i></span>
          <span class="zahl klein">{p.fortschritt_prozent} %</span>
        </span>
      </a>
    {/each}
  </div>
{:else if live.daten}
  <p class="leer">{t.leer} <a href={pfad(sprache(), sprache() === "de" ? "/docs/projekt-starten" : "/docs/start-a-project")}>{t.leerLink}</a></p>
{:else}
  <div class="tabelle skelett" aria-hidden="true">{#each Array(3) as _}<div class="zeile"></div>{/each}</div>
{/if}

<style>
  .kopfzeile { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 16px; }
  [role="tablist"] { display: inline-flex; padding: 3px; border: 1px solid var(--linie); border-radius: 8px; background: var(--graphit-2); }
  [role="tab"] { min-height: 38px; padding: 0 14px; border: 0; border-radius: 6px; background: none; color: var(--stahl);
    font: 500 .9rem var(--schrift); cursor: pointer; transition: background var(--dauer), color var(--dauer); }
  [role="tab"][aria-selected="true"] { background: var(--graphit-3); color: var(--text); box-shadow: inset 0 0 0 1px var(--linie-hell); }
  .stand { margin: 0; font-size: .88rem; }
  .tabelle { border: 1px solid var(--linie); border-radius: var(--radius); overflow: hidden; background: var(--metall); }
  .zeile { display: grid; grid-template-columns: minmax(0, 2.6fr) 1fr 1fr 1fr 1.3fr; gap: 16px; align-items: center;
    padding: 14px 20px; border-top: 1px solid var(--linie); color: var(--text); text-decoration: none; transition: background var(--dauer); }
  a.zeile:hover { background: color-mix(in srgb, var(--akzent) 5%, transparent); }
  .zeile.kopf { border-top: 0; padding-top: 12px; padding-bottom: 12px; color: var(--leise); font-size: .85rem; background: var(--graphit-2); }
  .projekt { display: flex; align-items: center; gap: 14px; min-width: 0; }
  .projekt strong { display: block; font-family: var(--schrift-marke); font-weight: 600; }
  .unter { display: block; font-size: .85rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .unter .lead { margin-left: 14px; }
  .zeile.spitze { background: color-mix(in srgb, var(--akzent) 6%, transparent); }
  .spitze .rang { border-color: var(--akzent); color: var(--akzent); }
  .rang { flex: none; width: 28px; height: 28px; display: grid; place-items: center; border: 1px solid var(--linie-hell);
    border-radius: 6px; font: 500 .85rem var(--mono); color: var(--stahl); }
  .zahl { font-family: var(--mono); font-variant-numeric: tabular-nums; text-align: right; }
  .kopf .zahl { font-family: var(--schrift); }
  .an { color: var(--akzent); }
  .punkt { display: inline-block; width: 7px; height: 7px; border-radius: 50%; background: var(--akzent); margin-right: 8px;
    }
  @keyframes puls { 0% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--akzent) 60%, transparent); } 100% { box-shadow: 0 0 0 8px transparent; } }
  .fortschritt { display: flex; align-items: center; gap: 10px; }
  .balken { flex: 1; height: 6px; border-radius: 3px; background: var(--graphit-3); overflow: hidden; }
  .balken i { display: block; height: 100%; background: var(--akzent); border-radius: 3px; transition: width .6s; }
  .klein { font-size: .85rem; min-width: 3.2em; }
  .leer { padding: 28px; border: 1px dashed var(--linie-hell); border-radius: var(--radius); color: var(--stahl); }
  .skelett .zeile { height: 64px; background: linear-gradient(90deg, transparent, var(--graphit-3), transparent) 0 0 / 200% 100%; animation: schimmer 1.4s infinite; }
  @keyframes schimmer { to { background-position: -200% 0; } }
  @media (max-width: 820px) {
    .zeile { grid-template-columns: 1fr auto; }
    .zeile.kopf { display: none; }
    .zeile > :nth-child(3), .zeile > :nth-child(4) { display: none; }
    .fortschritt { grid-column: 1 / -1; }
  }
</style>
