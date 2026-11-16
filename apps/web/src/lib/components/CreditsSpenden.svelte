<script lang="ts">
  // „Credits spenden?“ auf der Projektseite: fragt Schritt für Schritt nach Dienst, Menge und Verfallsdatum und legt die
  // Zusage an (POST /api/account/pledges, Weg „Website“, spenden.ts). Ohne Anmeldung bleibt der Entwurf in der Sitzung
  // und ist nach der Anmeldung wieder da. Der Weg über den Runner wird nur erklärt: Ihn legt `swarvex setup` an.
  import { onMount, tick } from "svelte";
  import { invalidate } from "$app/navigation";
  import { pfad, sprache, tx } from "$lib/texte";

  let { slug, name }: { slug: string; name: string } = $props();
  const s = $derived(sprache());
  const t = $derived(tx().projekte.detail.spenden);
  const dienstName = (d: string) => tx().konto.spende.dienste[d] ?? d;
  // Reihenfolge nach Häufigkeit verfallender Credits; „other“ verlangt den Namen in der Notiz
  const DIENSTE = ["meshy", "elevenlabs", "suno", "midjourney", "leonardo", "higgsfield", "runway", "kling", "other"];

  const iso = (x: Date) => `${x.getFullYear()}-${String(x.getMonth() + 1).padStart(2, "0")}-${String(x.getDate()).padStart(2, "0")}`;
  const heute = () => new Date();
  const monatsende = () => { const h = heute(); return iso(new Date(h.getFullYear(), h.getMonth() + 1, 0)); };
  const inTagen = (n: number) => { const h = heute(); return iso(new Date(h.getFullYear(), h.getMonth(), h.getDate() + n)); };

  let offen = $state(false);
  let schritt = $state<"dienst" | "menge" | "anmelden" | "fertig">("dienst");
  let dienst = $state("");
  let credits = $state<number | null>(null);
  let verfaellt = $state("");
  let notiz = $state("");
  let fehler = $state("");
  let arbeitet = $state(false);
  let panel = $state<HTMLDivElement>();
  let ausloeser = $state<HTMLButtonElement>();
  const ENTWURF = $derived(`credits-entwurf:${slug}`);

  async function fokus() { await tick(); panel?.querySelector<HTMLElement>("[data-fokus]")?.focus(); }
  function umschalten() { offen = !offen; if (offen) { if (schritt === "fertig") neu(); fokus(); } }
  function neu() { schritt = "dienst"; dienst = ""; credits = null; notiz = ""; fehler = ""; verfaellt = monatsende(); }
  function waehlen(d: string) { dienst = d; verfaellt ||= monatsende(); schritt = "menge"; fehler = ""; fokus(); }

  async function zusagen(e: SubmitEvent) {
    e.preventDefault();
    fehler = "";
    if (dienst === "other" && !notiz.trim()) { fehler = t.welcherDienst; return; }
    arbeitet = true;
    try {
      const r = await fetch("/api/account/pledges", { method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ project: slug, service: dienst, credits: Number(credits), expires: verfaellt, note: notiz.trim() }) });
      if (r.status === 401) {
        try { sessionStorage.setItem(ENTWURF, JSON.stringify({ dienst, credits, verfaellt, notiz })); } catch { /* ohne Speicher: nach der Anmeldung neu ausfüllen */ }
        schritt = "anmelden"; fokus(); return;
      }
      const j = await r.json().catch(() => ({}));
      if (!r.ok) { fehler = j.error ?? t.fehler; return; }
      try { sessionStorage.removeItem(ENTWURF); } catch { /* egal */ }
      schritt = "fertig"; fokus();
      invalidate("app:projekt");
    } catch { fehler = t.fehler; } finally { arbeitet = false; }
  }

  // Zurück von der Anmeldung: Entwurf wiederherstellen und das Menü offen zeigen, abschicken bleibt beim Menschen
  onMount(() => {
    verfaellt = monatsende();
    let roh: string | null = null;
    try { roh = sessionStorage.getItem(ENTWURF); } catch { /* kein Speicher */ }
    if (!roh) return;
    try {
      const e = JSON.parse(roh);
      if (DIENSTE.includes(e.dienst)) { dienst = e.dienst; credits = e.credits ?? null; verfaellt = e.verfaellt && e.verfaellt >= iso(heute()) ? e.verfaellt : monatsende(); notiz = e.notiz ?? ""; schritt = "menge"; offen = true; }
    } catch { /* kaputter Entwurf */ }
  });

  const schnell = $derived([{ name: t.monatsende, wert: monatsende() }, { name: t.inTagen(7), wert: inTagen(7) }, { name: t.inTagen(30), wert: inTagen(30) }]);
  const spendenDoku = $derived(pfad(s, s === "de" ? "/docs/spenden#medien-werkzeuge-über-den-runner" : "/docs/donations#media-tools-through-the-runner"));
  const konto = $derived(pfad(s, s === "de" ? "/konto#t-spenden" : "/en/account#t-spenden"));
</script>

<!-- Escape schließt das Menü, wenn der Fokus darin steht, und gibt den Fokus an den Knopf zurück -->
<svelte:window onkeydown={(e) => { if (e.key === "Escape" && offen && panel?.contains(document.activeElement)) { offen = false; ausloeser?.focus(); } }} />

<div class="spenden" class:offen>
  <button type="button" class="knopf ausloeser" bind:this={ausloeser} aria-expanded={offen} aria-controls="credits-spenden" onclick={umschalten}>
    <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><ellipse cx="12" cy="7" rx="7" ry="3" fill="none" stroke="currentColor" stroke-width="1.7"/><path d="M5 7v5c0 1.7 3.1 3 7 3s7-1.3 7-3V7M5 12v5c0 1.7 3.1 3 7 3s7-1.3 7-3v-5" fill="none" stroke="currentColor" stroke-width="1.7"/></svg>
    <span>{t.knopf}</span>
    <svg class="pfeil" viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M6 9l6 6 6-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
  </button>

  {#if offen}
    <div class="panel" id="credits-spenden" bind:this={panel} role="region" aria-label={t.knopf}>
      {#if schritt === "dienst"}
        <p class="frage" tabindex="-1" data-fokus>{t.frageDienst}</p>
        <p class="leise klein">{t.erklaerung}</p>
        <div class="dienste">
          {#each DIENSTE as d (d)}
            <button type="button" class="dienst" onclick={() => waehlen(d)}>
              <strong>{dienstName(d)}</strong><span class="leise">{t.macht[d]}</span>
            </button>
          {/each}
        </div>
      {:else if schritt === "menge"}
        <form onsubmit={zusagen}>
          <p class="frage" tabindex="-1" data-fokus>{t.frageMenge(dienstName(dienst))}</p>
          {#if dienst === "other"}
            <label>{t.welcherDienstLabel}<input type="text" maxlength="500" bind:value={notiz} placeholder={t.welcherDienstHilfe} required /></label>
          {/if}
          <label>{t.wieViele}<input type="number" inputmode="numeric" min="1" max="1000000" step="1" bind:value={credits} placeholder={t.wieVieleHilfe} required /></label>
          <fieldset>
            <legend>{t.bisWann}</legend>
            <div class="chips">
              {#each schnell as c (c.name)}
                <button type="button" class="chip" aria-pressed={verfaellt === c.wert} onclick={() => (verfaellt = c.wert)}>{c.name}</button>
              {/each}
            </div>
            <input type="date" aria-label={t.datum} min={iso(heute())} bind:value={verfaellt} required />
          </fieldset>
          {#if dienst !== "other"}
            <label>{t.notiz}<input type="text" maxlength="500" bind:value={notiz} placeholder={t.notizHilfe} /></label>
          {/if}
          {#if fehler}<p class="fehler" role="alert">{fehler}</p>{/if}
          <button class="knopf primaer" disabled={arbeitet}>{arbeitet ? t.sendet : t.zusagen}</button>
          <p class="leise klein">{t.soGehtsWeiter}</p>
          {#if dienst !== "other"}<p class="leise klein">{t.runner(dienstName(dienst))} <a href={spendenDoku}>{t.runnerLink}</a></p>{/if}
          <button type="button" class="zurueck leise" onclick={() => { schritt = "dienst"; fehler = ""; fokus(); }}>← {t.andererDienst}</button>
        </form>
      {:else if schritt === "anmelden"}
        <p class="frage" tabindex="-1" data-fokus>{t.anmeldenH}</p>
        <p class="leise klein">{t.anmeldenText}</p>
        <a class="knopf primaer" href={`/api/account/login?lang=${s}&next=projekt-${slug}`} data-sveltekit-reload>{t.anmelden}</a>
        <button type="button" class="zurueck leise" onclick={() => { schritt = "menge"; fokus(); }}>← {t.zurueck}</button>
      {:else}
        <p class="frage danke" tabindex="-1" data-fokus role="status">{t.danke(Number(credits), dienstName(dienst === "other" ? notiz.trim() || "other" : dienst), name)}</p>
        <p class="leise klein">{t.dankeText} <a href={konto}>{t.zumKonto}</a></p>
      {/if}
    </div>
  {/if}
</div>

<style>
  .spenden { display: grid; }
  .ausloeser { justify-content: center; gap: 8px; }
  .ausloeser .pfeil { transition: transform var(--dauer); }
  .offen .ausloeser { border-color: color-mix(in srgb, var(--bernstein) 60%, var(--linie)); border-bottom-left-radius: 0; border-bottom-right-radius: 0; }
  .offen .ausloeser .pfeil { transform: rotate(180deg); }
  .panel { display: grid; gap: 10px; padding: 14px; border: 1px solid color-mix(in srgb, var(--bernstein) 60%, var(--linie)); border-top: 0;
    border-radius: 0 0 var(--radius-klein) var(--radius-klein); background: var(--graphit-2); animation: auf var(--dauer) ease-out; }
  @keyframes auf { from { opacity: 0; transform: translateY(-4px); } }
  @media (prefers-reduced-motion: reduce) { .panel { animation: none; } .ausloeser .pfeil { transition: none; } }
  form { display: grid; gap: 10px; }
  .frage { margin: 0; font-weight: 600; font-size: .95rem; outline: none; }
  .klein { margin: 0; font-size: .82rem; line-height: 1.45; }
  .dienste { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; }
  .dienst { display: grid; gap: 2px; padding: 8px 10px; text-align: left; border: 1px solid var(--linie); border-radius: var(--radius-klein);
    background: var(--graphit); color: var(--text); font: inherit; cursor: pointer; min-height: 48px; }
  .dienst:hover, .dienst:focus-visible { border-color: var(--bernstein); }
  .dienst strong { font-size: .9rem; font-weight: 600; }
  .dienst span { font-size: .76rem; line-height: 1.3; }
  label, fieldset { display: grid; gap: 4px; font-size: .84rem; color: var(--stahl); }
  fieldset { margin: 0; padding: 0; border: 0; }
  legend { padding: 0; margin-bottom: 4px; }
  input { width: 100%; min-width: 0; padding: 8px 10px; border: 1px solid var(--linie-hell); border-radius: var(--radius-klein); background: var(--graphit);
    color: var(--text); font: inherit; font-size: .9rem; }
  .chips { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 6px; }
  .chip { padding: 4px 10px; border: 1px solid var(--linie-hell); border-radius: 999px; background: transparent; color: var(--stahl); font: inherit;
    font-size: .8rem; cursor: pointer; min-height: 32px; }
  .chip[aria-pressed="true"] { border-color: var(--bernstein); color: var(--text); background: color-mix(in srgb, var(--bernstein) 14%, transparent); }
  .fehler { margin: 0; color: var(--rot); font-size: .84rem; }
  .zurueck { justify-self: start; padding: 4px 0; border: 0; background: none; font: inherit; font-size: .82rem; cursor: pointer; }
  .zurueck:hover { color: var(--text); }
  .danke { color: var(--text); }
  .panel .knopf { justify-content: center; }
</style>
