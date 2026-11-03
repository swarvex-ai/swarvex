<script lang="ts">
  // Rückmeldung zur spielbaren Vorschau (apps/api/src/feedback.ts): was gut war, was hakt, zum gespielten Stand.
  // Nur angemeldet; der Text geht an Lead und Officers des Projekts, öffentlich ist nur die Anzahl.
  import { sprache, tx } from "$lib/texte";

  let { slug, commit, anzahl = 0 }: { slug: string; commit: string; anzahl?: number } = $props();
  const s = $derived(sprache());
  const t = $derived(tx().projekte.detail.feedback);
  let gut = $state(""), hakt = $state(""), geraet = $state("");
  let zustand = $state<"" | "sendet" | "danke" | "anmelden" | "fehler">("");
  let meldung = $state("");

  async function senden(e: SubmitEvent) {
    e.preventDefault();
    if (!gut.trim() && !hakt.trim()) { zustand = "fehler"; meldung = t.leer; return; }
    zustand = "sendet";
    try {
      const r = await fetch("/api/account/preview-feedback", { method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ project: slug, commit, good: gut, problems: hakt, device: geraet }) });
      if (r.status === 401) { zustand = "anmelden"; return; }
      if (!r.ok) { zustand = "fehler"; meldung = (await r.json().catch(() => ({}))).error ?? t.fehler; return; }
      zustand = "danke"; gut = ""; hakt = ""; geraet = "";
    } catch { zustand = "fehler"; meldung = t.fehler; }
  }
</script>

<details class="feedback">
  <summary>{t.oeffnen}{#if anzahl} <span class="leise">· {t.anzahl(anzahl)}</span>{/if}</summary>
  {#if zustand === "danke"}
    <p role="status">{t.danke}</p>
  {:else}
    <form onsubmit={senden}>
      <p class="leise klein">{t.text(commit.slice(0, 7))}</p>
      <label>{t.gut}<textarea rows="2" maxlength="2000" bind:value={gut}></textarea></label>
      <label>{t.hakt}<textarea rows="2" maxlength="2000" bind:value={hakt}></textarea></label>
      <label>{t.geraet}<input type="text" maxlength="120" bind:value={geraet} placeholder={t.geraetHilfe} /></label>
      {#if zustand === "anmelden"}
        <p>{t.anmeldenText} <a href={`/api/account/login?lang=${s}&next=projekt-${slug}`} data-sveltekit-reload>{t.anmelden}</a></p>
      {:else if zustand === "fehler"}<p class="fehler" role="alert">{meldung}</p>{/if}
      <button class="knopf" disabled={zustand === "sendet"}>{zustand === "sendet" ? t.sendet : t.senden}</button>
    </form>
  {/if}
</details>

<style>
  .feedback { width: 100%; max-width: 360px; text-align: left; font-size: .9rem; }
  .feedback summary { cursor: pointer; color: var(--stahl); text-align: right; }
  form { display: grid; gap: 8px; margin-top: 10px; border: 1px solid var(--linie); border-radius: var(--radius-klein); background: var(--metall); padding: 12px 14px; }
  form p { margin: 0; }
  label { display: grid; gap: 4px; }
  textarea, input { padding: 6px 8px; border-radius: 6px; border: 1px solid var(--linie-hell); background: var(--graphit-2); color: var(--text); font: 400 .9rem var(--schrift); width: 100%; box-sizing: border-box; }
  .fehler { color: var(--rot); }
  .knopf { justify-self: start; }
</style>
