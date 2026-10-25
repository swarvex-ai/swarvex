<script lang="ts">
  // Das Logo wie am Ende des Erklärvideos: Auf Klick fliegen die vier Agenten-Ströme mit Spuren
  // ins Sechseck, das Sechseck entsteht mit Lichtring und Strahlen, dazu der Ton aus dem Video.
  // Geometrie wie medien/logo/bauen_logo.py; Ton aus den Geräuschen des Erklärvideos (Schwarm + Einschlag).
  import wortRoh from "../../../../../medien/logo/export/swarvex_wortmarke_dunkel.svg?raw";
  import tonUrl from "$lib/ton/logo.mp3?url";

  let { groesse = 72, label }: { groesse?: number; label: string } = $props();

  const uid = Math.random().toString(36).slice(2, 7);
  const wortmarke = wortRoh.replace('fill="#eef0ff"', 'fill="currentColor"').replace('fill="#8b90b8"', 'style="fill:var(--leise)"');
  const M = 50;
  const sechseck = Array.from({ length: 6 }, (_, i) => {
    const w = ((30 + 60 * i) * Math.PI) / 180;
    return `${(M + 14.5 * Math.cos(w)).toFixed(2)},${(M + 14.5 * Math.sin(w)).toFixed(2)}`;
  }).join(" ");
  const STROEME: [number, string][] = [[45, "#22d3ee"], [135, "#ec4899"], [225, "#fbbf24"], [315, "#34d399"]];
  const ABSTAENDE = [25, 33.5, 41, 47.5];
  const RADIEN = [5.2, 4.2, 3.3, 2.5];
  const punkte = STROEME.flatMap(([winkel, farbe], s) =>
    ABSTAENDE.map((d, i) => {
      const w = (winkel * Math.PI) / 180;
      return { s, i, farbe, x: M + d * Math.cos(w), y: M + d * Math.sin(w), r: RADIEN[i], d };
    }),
  );
  const strahlen = Array.from({ length: 16 }, (_, i) => {
    const w = (i / 16) * 2 * Math.PI;
    return { x1: M + 19 * Math.cos(w), y1: M + 19 * Math.sin(w), x2: M + 25 * Math.cos(w), y2: M + 25 * Math.sin(w), dx: Math.cos(w), dy: Math.sin(w) };
  });

  let svg: SVGSVGElement;
  let wort: HTMLSpanElement;
  let ton: HTMLAudioElement | null = null;
  let laeuft = $state(false);

  const ANFLUG = 650; // ms bis zum Einschlag, wie im Video
  const weich = "cubic-bezier(0.22, 1, 0.36, 1)";

  function abspielen() {
    if (laeuft) return;
    laeuft = true;
    try {
      ton ??= new Audio(tonUrl);
      ton.currentTime = 0;
      ton.volume = 0.7;
      void ton.play().catch(() => {});
    } catch { /* ohne Ton weiter */ }

    const ruhig = matchMedia("(prefers-reduced-motion: reduce)").matches;
    const alle: Animation[] = [];
    const sechs = svg.querySelector<SVGPolygonElement>(".sechs")!;

    if (ruhig) {
      alle.push(sechs.animate([{ opacity: 0.2 }, { opacity: 1 }], { duration: 600, easing: "ease-out" }));
    } else {
      // Ströme: von außen nach innen, mit zwei nachlaufenden Spuren je Punkt
      svg.querySelectorAll<SVGCircleElement>(".punkt, .spur").forEach((el) => {
        const p = punkte[Number(el.dataset.n)];
        const spur = Number(el.dataset.spur ?? 0);
        const weg = 6 * p.d;
        const w = (STROEME[p.s][0] * Math.PI) / 180;
        const von = `translate(${(Math.cos(w) * weg).toFixed(1)}px, ${(Math.sin(w) * weg).toFixed(1)}px)`;
        const verzug = (3 - p.i) * 26 + p.s * 18 + spur * 34;
        const dauer = ANFLUG - (3 - p.i) * 26 - p.s * 18;
        const bild = spur
          ? [{ transform: von, opacity: 0 }, { opacity: 0.45 / spur, offset: 0.5 }, { transform: "translate(0,0)", opacity: 0 }]
          : [{ transform: von, opacity: 0 }, { opacity: 1, offset: 0.35 }, { transform: "translate(0,0)", opacity: 1 }];
        alle.push(el.animate(bild, { duration: dauer, delay: verzug, easing: weich, fill: "backwards" }));
      });
      alle.push(sechs.animate([{ transform: "scale(0)", opacity: 0 }, { transform: "scale(1.18)", opacity: 1, offset: 0.6 }, { transform: "scale(1)" }],
        { duration: 420, delay: ANFLUG, easing: "ease-out", fill: "backwards" }));
      const ring = svg.querySelector<SVGCircleElement>(".ring")!;
      alle.push(ring.animate([{ transform: "scale(1)", opacity: 0.9 }, { transform: "scale(5.5)", opacity: 0 }],
        { duration: 700, delay: ANFLUG, easing: "cubic-bezier(0.16, 1, 0.3, 1)" }));
      svg.querySelectorAll<SVGLineElement>(".strahl").forEach((el, i) => {
        const s = strahlen[i];
        alle.push(el.animate([
          { transform: "translate(0,0)", opacity: 0 },
          { opacity: 1, offset: 0.25 },
          { transform: `translate(${s.dx * 14}px, ${s.dy * 14}px)`, opacity: 0 },
        ], { duration: 520, delay: ANFLUG, easing: "ease-out" }));
      });
      alle.push(wort.animate([{ transform: "translateX(-10px)", opacity: 0 }, { transform: "translateX(0)", opacity: 1 }],
        { duration: 380, delay: ANFLUG + 80, easing: weich, fill: "backwards" }));
    }
    Promise.all(alle.map((a) => a.finished)).finally(() => (laeuft = false));
  }
</script>

<button type="button" class="logo" class:laeuft style="--h:{groesse}px" onclick={abspielen} aria-label={label} title={label}>
  <svg bind:this={svg} class="symbol" viewBox="0 0 100 100" aria-hidden="true">
    <defs>
      <linearGradient id="v{uid}" x1="0" y1="1" x2="1" y2="0">
        <stop offset="0" stop-color="#8b5cf6" /><stop offset="1" stop-color="#22d3ee" />
      </linearGradient>
    </defs>
    <circle class="ring" cx={M} cy={M} r="14.5" fill="none" stroke="#22d3ee" stroke-width="1.5" vector-effect="non-scaling-stroke" />
    {#each strahlen as s}
      <line class="strahl" x1={s.x1} y1={s.y1} x2={s.x2} y2={s.y2} stroke="#22d3ee" stroke-width="2" stroke-linecap="round" />
    {/each}
    {#each punkte as p, n}
      {#each [2, 1] as spur}
        <circle class="spur" data-n={n} data-spur={spur} cx={p.x} cy={p.y} r={p.r * (1 - spur * 0.2)} fill={p.farbe} />
      {/each}
    {/each}
    <polygon class="sechs" points={sechseck} fill="url(#v{uid})" />
    {#each punkte as p, n}
      <circle class="punkt" data-n={n} cx={p.x} cy={p.y} r={p.r} fill={p.farbe} />
    {/each}
  </svg>
  <span class="wort" bind:this={wort}>{@html wortmarke}</span>
</button>

<style>
  .logo {
    display: inline-flex; align-items: center; gap: calc(var(--h) * 0.18); color: var(--text);
    background: none; border: 0; padding: 0; margin: 0; font: inherit; cursor: pointer; border-radius: 8px;
  }
  .logo:focus-visible { outline: 2px solid var(--akzent); outline-offset: 6px; }
  .symbol { width: var(--h); height: var(--h); overflow: visible; }
  .wort { display: inline-flex; }
  .wort :global(svg) { height: calc(var(--h) * 0.62); width: auto; }
  .sechs, .ring { transform-box: fill-box; transform-origin: center; }
  .ring, .strahl, .spur { opacity: 0; }
  .sechs { transition: filter 0.25s ease; }
  .logo:hover .sechs, .logo:focus-visible .sechs { filter: drop-shadow(0 0 6px rgba(34, 211, 238, 0.7)); }
  @media (hover: hover) and (prefers-reduced-motion: no-preference) {
    .logo:not(.laeuft):hover .punkt { animation: zucken 0.9s ease-in-out infinite; }
    .logo:not(.laeuft):hover .punkt:nth-of-type(4n + 2) { animation-delay: 0.1s; }
    .logo:not(.laeuft):hover .punkt:nth-of-type(4n + 3) { animation-delay: 0.2s; }
  }
  @keyframes zucken { 50% { opacity: 0.55; } }
</style>
