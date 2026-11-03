<script lang="ts">
  // Live-Ansicht unter der Hero-Grafik: die letzten öffentlichen Ereignisse aller Projekte (/api/activity), ohne Namen.
  // Lädt alle 15 s neu, solange der Tab sichtbar ist; ohne Ereignisse der letzten sieben Tage bleibt sie leer.
  import { onMount } from "svelte";
  import { pfad, sprache, tx } from "$lib/texte";

  type Ereignis = { seq: number; time: string; type: string; project: string; name: string; verdict: string | null };
  const s = $derived(sprache());
  const t = $derived(tx().start.liveTicker);
  let liste = $state<Ereignis[]>([]);
  let jetzt = $state(Date.now());

  async function laden() {
    if (document.hidden) return;
    try {
      const r = await fetch("/api/activity", { headers: { accept: "application/json" } });
      if (r.ok) liste = ((await r.json()) as Ereignis[]).slice(0, 5);
      jetzt = Date.now();
    } catch { /* bleibt, wie es ist */ }
  }
  onMount(() => { laden(); const z = setInterval(laden, 15_000); return () => clearInterval(z); });

  const vor = (zeit: string) => {
    const min = Math.max(0, Math.round((jetzt - new Date(zeit).getTime()) / 60_000));
    return min < 1 ? t.geradeEben : min < 60 ? t.minuten(min) : min < 1440 ? t.stunden(Math.round(min / 60)) : t.tage(Math.round(min / 1440));
  };
  const text = (e: Ereignis) => (e.type === "reviewed" ? t.arten[`reviewed_${e.verdict}`] : e.type === "peer_review" ? t.arten[`peer_review_${e.verdict}`] : t.arten[e.type]) ?? e.type;
</script>

{#if liste.length}
  <div class="ticker" aria-live="polite">
    <p class="kopf"><i class="punkt" aria-hidden="true"></i>{t.h}</p>
    <ul>
      {#each liste as e (e.seq)}
        <li><span class="zeit">{vor(e.time)}</span> <a href={pfad(s, `${tx().projekte.pfad}/${e.project}`)}>{e.name}</a> <span class="was">{text(e)}</span></li>
      {/each}
    </ul>
  </div>
{/if}

<style>
  .ticker { margin-top: 14px; border: 1px solid var(--linie); border-radius: var(--radius-klein); background: var(--metall); padding: 10px 14px; font-size: .86rem; }
  .kopf { margin: 0 0 6px; color: var(--leise); font: 500 .78rem var(--mono); text-transform: uppercase; letter-spacing: .04em; display: flex; align-items: center; gap: 8px; }
  .punkt { width: 7px; height: 7px; border-radius: 50%; background: var(--akzent, #34d399); display: inline-block; }
  ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 4px; }
  li { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .zeit { color: var(--leise); font: .8rem var(--mono); display: inline-block; min-width: 76px; }
  .was { color: var(--stahl); }
</style>
