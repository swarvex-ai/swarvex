<script lang="ts">
  import { tx } from "$lib/texte";
  let { befehl = "curl -fsSL https://swarvex.ai/install.sh | sh" }: { befehl?: string } = $props();
  const t = $derived(tx().einzeiler);
  let kopiert = $state(false);
  async function kopieren() {
    try { await navigator.clipboard.writeText(befehl); kopiert = true; setTimeout(() => (kopiert = false), 1800); }
    catch { /* ohne Zwischenablage-Recht bleibt der Text markierbar */ }
  }
</script>

<div class="einzeiler">
  <code><span class="prompt" aria-hidden="true">$</span> {befehl}</code>
  <button type="button" onclick={kopieren} aria-label={t.aria}>
    {#if kopiert}
      <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path d="M5 12.5l4.5 4.5L19 7.5" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/></svg>
      <span>{t.kopiert}</span>
    {:else}
      <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><rect x="8" y="8" width="11" height="11" rx="2" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M5 15V6a1 1 0 0 1 1-1h9" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
      <span>{t.kopieren}</span>
    {/if}
  </button>
</div>

<style>
  .einzeiler {
    display: flex; align-items: center; gap: 12px; max-width: 100%;
    border: 1px solid var(--linie-hell); border-radius: var(--radius-klein); background: var(--metall);
    padding: 6px 6px 6px 16px;
  }
  code { flex: 1; min-width: 0; overflow-x: auto; white-space: nowrap; color: var(--text); font-size: .9rem; scrollbar-width: none; }
  .prompt { color: var(--akzent); margin-right: .4em; }
  button {
    display: inline-flex; align-items: center; gap: 6px; min-height: 40px; padding: 0 12px;
    border: 1px solid var(--linie); border-radius: 5px; background: var(--graphit-3); color: var(--stahl);
    font: 500 .85rem var(--schrift); cursor: pointer; transition: color var(--dauer), border-color var(--dauer);
  }
  button:hover { color: var(--akzent); border-color: var(--akzent); }
  @media (max-width: 520px) { button span { display: none; } }
</style>
