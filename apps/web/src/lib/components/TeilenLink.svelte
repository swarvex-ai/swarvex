<script lang="ts">
  // Teilen-Link eines Projekts (/join/<projekt>, E-39): kopieren oder über das Teilen-Menü des Geräts verschicken.
  // Der Link gibt keine Rechte; wer mitmacht, meldet sich mit eigenem Konto an.
  import { SEITE } from "$lib/seo";
  import { tx } from "$lib/texte";
  let { slug, name, klein = false }: { slug: string; name: string; klein?: boolean } = $props();
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

<span class="teilen" class:klein>
  <button type="button" class="knopf" onclick={teilen}>
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><circle cx="6" cy="12" r="2.4" fill="none" stroke="currentColor" stroke-width="1.8"/><circle cx="18" cy="6" r="2.4" fill="none" stroke="currentColor" stroke-width="1.8"/><circle cx="18" cy="18" r="2.4" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M8.2 10.9l7.6-3.8M8.2 13.1l7.6 3.8" stroke="currentColor" stroke-width="1.8"/></svg>
    {kopiert ? t.kopiert : t.teilen}
  </button>
  {#if !klein}<code>{link.replace(/^https:\/\//, "")}</code>{/if}
</span>

<style>
  .teilen { display: inline-flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; max-width: 100%; }
  .teilen .knopf { display: inline-flex; align-items: center; gap: 8px; }
  code { font-size: .85rem; color: var(--stahl); overflow-wrap: anywhere; }
</style>
