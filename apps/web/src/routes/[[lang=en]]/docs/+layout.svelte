<script lang="ts">
  import { page } from "$app/state";
  import Suche from "$lib/components/Suche.svelte";
  import { pfad, sprache, tx } from "$lib/texte";
  const t = $derived(tx());
  const s = $derived(sprache());
  let { data, children } = $props();
  let suche: Suche;
  let navOffen = $state(false);
</script>

<div class="doku huelle">
  <aside class="seitenleiste" class:offen={navOffen}>
    <button type="button" class="suchknopf" onclick={() => suche.oeffnen()}>
      <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><circle cx="11" cy="11" r="6.5" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M16 16l4 4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
      <span>{t.suche.knopf}</span><kbd>⌘K</kbd>
    </button>
    <a class="als-text" href={pfad(s, "/llms-full.txt")}>{t.doku.alsText}</a>
    <nav aria-label={t.doku.nav}>
      {#each data.nav as g}
        <p class="gruppe">{g.gruppe}</p>
        <ul>
          {#each g.seiten as seite}
            <li><a href={pfad(s, `/docs/${seite.slug}`)} aria-current={page.params.slug === seite.slug ? "page" : undefined} onclick={() => (navOffen = false)}>{seite.titel}</a></li>
          {/each}
        </ul>
      {/each}
    </nav>
  </aside>
  <button type="button" class="nav-knopf" aria-expanded={navOffen} onclick={() => (navOffen = !navOffen)}>
    {navOffen ? t.doku.navAuf : t.doku.navZu}
  </button>
  <div class="inhalt">{@render children()}</div>
</div>
<Suche bind:this={suche} />

<style>
  .doku { display: grid; grid-template-columns: 240px minmax(0, 1fr); gap: 48px; padding-top: 32px; }
  .seitenleiste { position: sticky; top: 88px; align-self: start; max-height: calc(100vh - 110px); overflow: auto; padding-bottom: 24px; scrollbar-width: thin; }
  .suchknopf { display: flex; align-items: center; gap: 8px; width: 100%; min-height: 40px; padding: 0 10px; border: 1px solid var(--linie-hell);
    border-radius: 7px; background: var(--graphit-2); color: var(--leise); font: 400 .9rem var(--schrift); cursor: pointer; }
  .suchknopf span { flex: 1; text-align: left; }
  .suchknopf:hover { border-color: var(--stahl); color: var(--stahl); }
  kbd { font-size: .72rem; padding: 1px 5px; border: 1px solid var(--linie-hell); border-radius: 4px; }
  .als-text { display: block; margin: 10px 0 18px; font-size: .85rem; color: var(--leise); text-decoration: none; }
  .als-text:hover { color: var(--akzent); }
  .gruppe { margin: 18px 0 6px; font: 600 .82rem var(--schrift); color: var(--text); }
  ul { list-style: none; margin: 0; padding: 0; border-left: 1px solid var(--linie); }
  li a { display: block; padding: 5px 12px; margin-left: -1px; border-left: 1px solid transparent; color: var(--stahl); text-decoration: none; font-size: .92rem; }
  li a:hover { color: var(--text); border-left-color: var(--linie-hell); }
  li a[aria-current="page"] { color: var(--akzent); border-left-color: var(--akzent); }
  .nav-knopf { display: none; }
  @media (max-width: 900px) {
    .doku { grid-template-columns: 1fr; gap: 0; }
    .seitenleiste { display: none; position: static; max-height: none; }
    .seitenleiste.offen { display: block; }
    .nav-knopf { display: block; order: -1; min-height: 44px; margin-bottom: 16px; border: 1px solid var(--linie-hell); border-radius: 7px;
      background: var(--graphit-2); color: var(--text); font: 500 .95rem var(--schrift); }
    .seitenleiste { order: 0; }
  }
</style>
