<script lang="ts">
  // Lead starten mit einem Befehl (install.sh … lead <projekt>) und live sehen, ob er läuft: fragt das eigene Konto
  // (lead_services, apps/api/src/konto.ts) alle fünf Sekunden ab, bis der Lead-Dienst aktiv ist.
  import { onDestroy, onMount } from "svelte";
  import Einzeiler from "$lib/components/Einzeiler.svelte";
  import { tx, pfad, sprache } from "$lib/texte";

  let { slug, basis = "https://swarvex.ai" }: { slug: string; basis?: string } = $props();
  const t = $derived(tx().starten.leadStart);
  type Stand = "warten" | "angemeldet" | "laeuft";
  let stand: Stand = $state("warten");
  let uhr: ReturnType<typeof setInterval> | undefined;

  async function pruefen() {
    try {
      const r = await fetch("/api/account", { credentials: "same-origin", cache: "no-store" });
      if (!r.ok) return;
      const k = await r.json();
      const d = (k.lead_services ?? []).find((x: { project: string }) => x.project === slug);
      stand = !d ? "warten" : d.active ? "laeuft" : "angemeldet";
      if (stand === "laeuft" && uhr) { clearInterval(uhr); uhr = undefined; }
    } catch { /* offline: beim nächsten Mal */ }
  }
  onMount(() => { pruefen(); uhr = setInterval(pruefen, 5000); });
  onDestroy(() => { if (uhr) clearInterval(uhr); });
</script>

<Einzeiler befehl={`curl -fsSL ${basis}/install.sh | sh -s -- lead ${slug}`} />
<div class="stand {stand}" role="status" aria-live="polite">
  {#if stand === "laeuft"}
    <strong>{t.laeuft}</strong>
    <p>{t.laeuftText}</p>
    <Einzeiler befehl={`swarvex project lead attach ${slug}`} />
    <p><a href={pfad(sprache(), `${tx().projekte.pfad}/${slug}`)}>{t.projekt} →</a></p>
  {:else if stand === "angemeldet"}
    <strong><span class="punkt" aria-hidden="true"></span>{t.angemeldet}</strong>
    <p>{t.angemeldetText}</p>
  {:else}
    <strong><span class="punkt" aria-hidden="true"></span>{t.warten}</strong>
    <p>{t.wartenText}</p>
  {/if}
</div>

<style>
  .stand { margin-top: 12px; padding: 14px 16px; border: 1px solid var(--linie); border-radius: var(--radius-klein); background: var(--graphit-2, transparent); }
  .stand p { margin: 6px 0 0; color: var(--stahl); font-size: .92rem; }
  .stand.laeuft { border-color: var(--akzent); }
  .stand.laeuft strong { color: var(--akzent); }
  .stand :global(.einzeiler) { margin-top: 10px; }
  .punkt { display: inline-block; width: 8px; height: 8px; margin-right: 8px; border-radius: 50%; background: var(--stahl); vertical-align: middle; animation: puls 1.6s ease-in-out infinite; }
  @keyframes puls { 50% { opacity: .3; } }
  @media (prefers-reduced-motion: reduce) { .punkt { animation: none; } }
</style>
