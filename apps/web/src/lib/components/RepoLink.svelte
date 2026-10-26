<script lang="ts">
  // Link zum Quellcode auf GitHub mit Sternzahl (ab dem ersten Stern).
  import { onMount } from "svelte";
  import { QUELLCODE } from "$lib/seo";
  import { repo, repoLaden } from "$lib/repo.svelte";
  import { tx } from "$lib/texte";

  let { text, klasse = "" }: { text: string; klasse?: string } = $props();
  const t = $derived(tx().nav);
  const zahl = (n: number) => (n >= 1000 ? `${(n / 1000).toLocaleString("en-US", { maximumFractionDigits: 1 })}k` : String(n));
  onMount(repoLaden);
</script>

<a href={QUELLCODE} class="repo {klasse}" rel="noopener"
   aria-label={repo.stars ? `${text} (GitHub, ${repo.stars} ${t.sterne})` : `${text} (GitHub)`}>
  <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill="currentColor" d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
  <span>{text}</span>
  {#if repo.stars}
    <span class="sterne" aria-hidden="true"><svg viewBox="0 0 16 16" width="12" height="12"><path fill="currentColor" d="M8 .25l2.36 4.78 5.28.77-3.82 3.72.9 5.25L8 12.27l-4.72 2.5.9-5.25L.36 5.8l5.28-.77z"/></svg>{zahl(repo.stars)}</span>
  {/if}
</a>

<style>
  .repo { display: inline-flex; align-items: center; gap: 7px; }
  .sterne { display: inline-flex; align-items: center; gap: 3px; font: 500 .78rem var(--mono); border: 1px solid var(--linie-hell);
            border-radius: 999px; padding: 1px 7px; color: var(--stahl); }
</style>
