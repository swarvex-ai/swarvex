<script lang="ts">
  // Symbol und Wortmarke kommen direkt aus medien/logo/export (eine Quelle für Video, Logo-Dateien und Website).
  import symbolRoh from "../../../../../medien/logo/export/swarvex_symbol.svg?raw";
  import wortRoh from "../../../../../medien/logo/export/swarvex_wortmarke_dunkel.svg?raw";

  let { groesse = 32, wort = true }: { groesse?: number; wort?: boolean } = $props();
  const uid = Math.random().toString(36).slice(2, 7);
  // eindeutige Verlaufs-ID, falls das Logo mehrfach auf der Seite steht
  const symbol = symbolRoh.replace(/id="v"/, `id="v${uid}"`).replace(/url\(#v\)/, `url(#v${uid})`);
  // Schriftfarbe folgt dem Thema (hell/dunkel)
  const wortmarke = wortRoh.replace('fill="#eef0ff"', 'fill="currentColor"').replace('fill="#8b90b8"', 'style="fill:var(--leise)"');
</script>

<span class="logo" style="--h:{groesse}px">
  <span class="symbol" aria-hidden="true">{@html symbol}</span>
  {#if wort}<span class="wort" aria-label="swarvex.ai">{@html wortmarke}</span>{/if}
</span>

<style>
  .logo { display: inline-flex; align-items: center; gap: calc(var(--h) * 0.18); color: var(--text); }
  .symbol, .wort { display: inline-flex; }
  .symbol :global(svg) { width: var(--h); height: var(--h); }
  .wort :global(svg) { height: calc(var(--h) * 0.62); width: auto; }
</style>
