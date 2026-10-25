<script lang="ts">
  // „So funktioniert es“: links bleibt eine Grafik stehen, rechts drei Schritte. Der Schritt, der
  // gerade in der Bildmitte steht, bestimmt den Zustand der Grafik (Scroll erzählt die Abfolge).
  import { onMount } from "svelte";

  import { tx } from "$lib/texte";
  const schritte = $derived(tx().ablauf);
  const g = $derived(tx().grafik);
  let aktiv = $state(0);
  let elemente: HTMLElement[] = [];

  onMount(() => {
    const io = new IntersectionObserver(
      (eintraege) => { for (const e of eintraege) if (e.isIntersecting) aktiv = Number((e.target as HTMLElement).dataset.i); },
      { rootMargin: "-35% 0px -50% 0px" },
    );
    elemente.forEach((el) => io.observe(el));
    return () => io.disconnect();
  });

  // Aufgaben im Ring und Agenten-Bahnen für die Grafik
  const ring = Array.from({ length: 8 }, (_, i) => {
    const w = (i / 8) * Math.PI * 2 - Math.PI / 2;
    return { x: 200 + Math.cos(w) * 140, y: 200 + Math.sin(w) * 108 };
  });
  const ecken = [[20, 20], [380, 20], [380, 380], [20, 380]];
</script>

<div class="ablauf" data-schritt={aktiv}>
  <div class="buehne" aria-hidden="true">
    <svg viewBox="0 0 400 400">
      <!-- Raster -->
      <defs><pattern id="raster" width="20" height="20" patternUnits="userSpaceOnUse"><path d="M20 0H0V20" fill="none" stroke="var(--linie)" stroke-width=".6"/></pattern></defs>
      <rect width="400" height="400" fill="url(#raster)" opacity=".6"/>

      <!-- Agenten-Bahnen (Schritt 2) -->
      <g class="bahnen">
        {#each ring as p, i}
          {@const e = ecken[i % 4]}
          <path d="M{e[0]} {e[1]} Q {(e[0] + p.x) / 2 + 30} {(e[1] + p.y) / 2 - 20} {p.x} {p.y}" class="bahn" style="--v:{i * 0.25}s"/>
          <rect class="agent" width="9" height="7" rx="2" style="offset-path: path('M{e[0]} {e[1]} Q {(e[0] + p.x) / 2 + 30} {(e[1] + p.y) / 2 - 20} {p.x} {p.y}'); --v:{i * 0.35}s"/>
        {/each}
      </g>

      <!-- Aufgaben-Ring (ab Schritt 1) -->
      <g class="ring">
        {#each ring as p, i}
          <g class="aufgabe" style="--v:{i * 60}ms; transform-origin:{p.x}px {p.y}px">
            <rect x={p.x - 20} y={p.y - 12} width="40" height="24" rx="4" class:geprueft={i % 3 !== 2}/>
            <path d="M{p.x - 12} {p.y - 3}h22M{p.x - 12} {p.y + 4}h14"/>
            <path class="haken" d="M{p.x + 11} {p.y - 21}l4 4 8 -9"/>
          </g>
        {/each}
      </g>

      <!-- Prüfstraße (Schritt 3) -->
      <g class="pruefung">
        <path d="M150 330v-34h100v34" class="tor"/>
        <text x="200" y="352" text-anchor="middle">{g.pruefstrasse}</text>
      </g>

      <!-- Projekt-Sechseck und Lead -->
      <g class="kern">
        <polygon points="200,160 234.6,180 234.6,220 200,240 165.4,220 165.4,180" class="sechs"/>
        <polygon points="200,178 219,189 219,211 200,222 181,211 181,189" class="innen"/>
        <rect class="fuellung" x="170" y="252" width="60" height="5" rx="2.5"/>
        <rect class="fuellung-wert" x="170" y="252" height="5" rx="2.5"/>
      </g>
      <g class="lead">
        <rect x="250" y="180" width="26" height="20" rx="5"/><rect x="254" y="203" width="18" height="12" rx="3"/>
        <circle cx="258" cy="190" r="2"/><circle cx="268" cy="190" r="2"/>
        <text x="263" y="232" text-anchor="middle">{g.lead}</text>
      </g>
    </svg>
  </div>

  <ol class="schritte">
    {#each schritte as s, i}
      <li bind:this={elemente[i]} data-i={i} class:an={aktiv === i}>
        <span class="nummer" aria-hidden="true">{i + 1}</span>
        <h3>{s.titel}</h3>
        <p>{s.text}</p>
      </li>
    {/each}
  </ol>
</div>

<style>
  .ablauf { display: grid; grid-template-columns: 1fr 1fr; gap: clamp(24px, 5vw, 72px); }
  .buehne { position: sticky; top: max(88px, calc(50vh - 240px)); align-self: start; aspect-ratio: 1; max-width: 480px; width: 100%; border: 1px solid var(--linie);
    border-radius: var(--radius); background: var(--metall); overflow: hidden; }
  svg { display: block; width: 100%; height: 100%; }
  svg text { font: 500 11px var(--mono); fill: var(--stahl); }
  .schritte { list-style: none; margin: 0; padding: 0; display: grid; }
  /* Jeder Schritt ist hoch genug, dass die Grafik über alle drei stehen bleibt */
  .schritte li { min-height: 62vh; }
  .schritte li:last-child { min-height: 48vh; }
  .schritte li { position: relative; padding-left: 56px; color: var(--leise); transition: color .35s; }
  .schritte li h3 { color: var(--stahl); transition: color .35s; }
  .schritte li.an h3 { color: var(--text); }
  .nummer { position: absolute; left: 0; top: 0; width: 36px; height: 36px; display: grid; place-items: center;
    border: 1px solid var(--linie-hell); border-radius: 8px; font: 500 .95rem var(--mono); color: var(--stahl); transition: all .35s; }
  li.an .nummer { border-color: var(--akzent); color: var(--akzent); }
  .schritte p { color: inherit; max-width: 46ch; transition: color .35s; }
  .schritte li.an p { color: var(--stahl); }

  /* Grafik-Zustände */
  .sechs { fill: color-mix(in srgb, var(--akzent) 6%, transparent); stroke: var(--akzent); stroke-width: 2; }
  .innen { fill: none; stroke: var(--stahl); stroke-opacity: .4; }
  .lead rect { fill: var(--graphit-3); stroke: var(--stahl); stroke-width: 1.5; }
  .lead circle { fill: var(--akzent); }
  .aufgabe { opacity: 0; transform: scale(.4); transition: opacity .4s var(--v), transform .5s cubic-bezier(.16, 1, .3, 1) var(--v); }
  .aufgabe rect { fill: var(--graphit-3); stroke: var(--linie-hell); stroke-width: 1.2; transition: stroke .4s; }
  .aufgabe path { fill: none; stroke: var(--stahl); stroke-width: 1.5; stroke-linecap: round; }
  .aufgabe .haken { stroke: var(--akzent); stroke-width: 2.2; opacity: 0; transition: opacity .3s var(--v); }
  .bahn { fill: none; stroke: var(--stahl); stroke-dasharray: 3 5; opacity: 0; transition: opacity .4s; }
  .agent { fill: var(--stahl); opacity: 0; offset-rotate: 0deg; }
  .pruefung { opacity: 0; transition: opacity .4s; }
  .tor { fill: none; stroke: var(--akzent); stroke-width: 2; }
  .fuellung { fill: var(--graphit-3); }
  .fuellung-wert { fill: var(--akzent); width: 4px; transition: width .9s ease .2s; }

  [data-schritt="0"] .aufgabe, [data-schritt="1"] .aufgabe, [data-schritt="2"] .aufgabe { opacity: 1; transform: none; }
  [data-schritt="1"] .bahn { opacity: .5; }
  [data-schritt="1"] .agent { opacity: 1; animation: fliegen 2.8s cubic-bezier(.4, 0, .2, 1) var(--v) infinite; }
  [data-schritt="2"] .pruefung { opacity: 1; }
  [data-schritt="2"] .aufgabe rect.geprueft { stroke: var(--akzent); }
  [data-schritt="2"] .aufgabe .haken { opacity: 1; }
  [data-schritt="2"] .fuellung-wert { width: 38px; }
  @keyframes fliegen { from { offset-distance: 0%; } to { offset-distance: 100%; } }

  @media (max-width: 820px) {
    .ablauf { grid-template-columns: 1fr; }
    .buehne { position: relative; top: 0; max-width: 420px; }
    .schritte { padding: 0; gap: 28px; }
    .schritte li, .schritte li:last-child { min-height: 0; }
    .schritte li { opacity: 1; }
  }
</style>
