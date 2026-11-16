<script lang="ts">
  // Teilen-Link eines Projekts (/join/<projekt>, E-39): kopieren oder über das Teilen-Menü des Geräts verschicken.
  // Der Link gibt keine Rechte; wer mitmacht, meldet sich mit eigenem Konto an.
  // klein: nur der Knopf; betont: Link als Feld mit Knopf darunter (Projektseite)
  import { SEITE } from "$lib/seo";
  import { tx } from "$lib/texte";
  let { slug, name, klein = false, betont = false }: { slug: string; name: string; klein?: boolean; betont?: boolean } = $props();
  const t = $derived(tx().leadKraft);
  const link = $derived(`${SEITE}/join/${slug}`);
  let kopiert = $state(false);
  async function teilen() {
    try {
      if (typeof navigator.share === "function" && matchMedia("(pointer: coarse)").matches) {
        await navigator.share({ title: t.joinTitel(name), url: link });
        return;
      }
      await navigator.clipboard.writeText(link);
      kopiert = true;
      setTimeout(() => (kopiert = false), 1800);
    } catch { /* abgebrochen oder ohne Zwischenablage-Recht: der Link bleibt markierbar */ }
  }
</script>

{#snippet knopf()}
  <button type="button" class="knopf" class:kopiert onclick={teilen}>
    {#if kopiert}
      <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M5 12.5l4.5 4.5L19 7.5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
    {:else}
      <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><circle cx="6" cy="12" r="2.4" fill="none" stroke="currentColor" stroke-width="1.8"/><circle cx="18" cy="6" r="2.4" fill="none" stroke="currentColor" stroke-width="1.8"/><circle cx="18" cy="18" r="2.4" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M8.2 10.9l7.6-3.8M8.2 13.1l7.6 3.8" stroke="currentColor" stroke-width="1.8"/></svg>
    {/if}
    <span aria-live="polite">{kopiert ? t.kopiert : t.teilen}</span>
  </button>
{/snippet}

{#if betont}
  <span class="teilen betont">
    <code>{link.replace(/^https:\/\//, "")}</code>
    {@render knopf()}
  </span>
{:else}
  <span class="teilen" class:klein>
    {@render knopf()}
    {#if !klein}<code>{link.replace(/^https:\/\//, "")}</code>{/if}
  </span>
{/if}

<style>
  .teilen { display: inline-flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; max-width: 100%; }
  .teilen .knopf { display: inline-flex; align-items: center; gap: 8px; }
  code { font-size: .85rem; color: var(--stahl); overflow-wrap: anywhere; }

  /* Link als Feld, darunter der Knopf: der Link bleibt ganz lesbar, der Knopf kopiert ihn (am Telefon: Teilen-Menü) */
  .betont { display: grid; gap: 0; width: 100%; border: 1px solid color-mix(in srgb, var(--akzent) 55%, var(--linie));
    border-radius: var(--radius-klein); background: var(--graphit-2); overflow: hidden; }
  .betont code { padding: 10px 12px; color: var(--text); font-size: .86rem; text-align: center; overflow-wrap: anywhere; }
  .betont .knopf { justify-content: center; border: 0; border-top: 1px solid color-mix(in srgb, var(--akzent) 55%, var(--linie)); border-radius: 0;
    background: color-mix(in srgb, var(--akzent) 14%, transparent); color: var(--akzent); font-weight: 600; }
  .betont .knopf:hover { background: color-mix(in srgb, var(--akzent) 24%, transparent); }
  .betont .knopf.kopiert { background: var(--akzent); color: var(--auf-akzent); }
</style>
