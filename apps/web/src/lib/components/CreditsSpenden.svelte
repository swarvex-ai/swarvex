<script lang="ts">
  // „Credits spenden?“ auf der Projektseite: Dienst, API-Schlüssel (geprüft mit Guthaben), Menge und Datum, dann die
  // Zusage (POST /api/account/pledges, Weg „schluessel“, spenden.ts). Die Plattform erzeugt die Aufträge des Leads selbst
  // (spendenwerk.ts); von Hand erzeugen und hochladen macht keiner (neVs 17.11.2026). Der Schlüssel bleibt nur im
  // Speicher dieser Seite, nie im Entwurf: Nach der Anmeldung wird er neu eingefügt.
  import { onMount, tick } from "svelte";
  import { invalidate } from "$app/navigation";
  import { pfad, sprache, tx } from "$lib/texte";
  let { slug, name }: { slug: string; name: string } = $props();
  const s = $derived(sprache());
  const t = $derived(tx().projekte.detail.spenden);
  const dienstName = (d: string) => tx().konto.spende.dienste[d] ?? d;
  const DIENSTE = ["meshy", "elevenlabs"];
  const iso = (x: Date) => `${x.getFullYear()}-${String(x.getMonth() + 1).padStart(2, "0")}-${String(x.getDate()).padStart(2, "0")}`;
  const heute = () => new Date();
  const monatsende = () => { const h = heute(); return iso(new Date(h.getFullYear(), h.getMonth() + 1, 0)); };
  const inTagen = (n: number) => { const h = heute(); return iso(new Date(h.getFullYear(), h.getMonth(), h.getDate() + n)); };
  // Der Dienst erneuert am Tag X: gespendet werden kann bis zum Tag davor
  const tagDavor = (d: string) => { const [j, m, t2] = d.split("-").map(Number); return iso(new Date(j, m - 1, t2 - 1)); };
  const datumText = (d: string) => new Date(`${d}T12:00:00`).toLocaleDateString(s === "de" ? "de-DE" : "en", { day: "numeric", month: "long", year: "numeric" });
  type Geprueft = { balance: number; expiring: number | null; renews: string | null };
  let offen = $state(false);
  let schritt = $state<"dienst" | "schluessel" | "menge" | "anmelden" | "fertig">("dienst");
  let dienst = $state("");
  let schluessel = $state("");
  let geprueft = $state<Geprueft | null>(null);
  let credits = $state<number | null>(null);
  let verfaellt = $state("");
  let notiz = $state("");
  let rechte = $state(false);
  let fehler = $state("");
  let arbeitet = $state(false);
  let panel = $state<HTMLDivElement>();
  let ausloeser = $state<HTMLButtonElement>();
  const ENTWURF = $derived(`credits-entwurf:${slug}`);
  async function fokus() { await tick(); panel?.querySelector<HTMLElement>("[data-fokus]")?.focus(); }
  function umschalten() { offen = !offen; if (offen) { if (schritt === "fertig") neu(); fokus(); } }
  function neu() { schritt = "dienst"; dienst = ""; schluessel = ""; geprueft = null; credits = null; notiz = ""; rechte = false; fehler = ""; verfaellt = monatsende(); }
  function waehlen(d: string) { dienst = d; schluessel = ""; geprueft = null; schritt = "schluessel"; fehler = ""; fokus(); }
  function anmelden() {
    try { sessionStorage.setItem(ENTWURF, JSON.stringify({ dienst })); } catch { /* ohne Speicher: nach der Anmeldung neu wählen */ }
    schluessel = ""; schritt = "anmelden"; fokus();
  }
  async function pruefen(e: SubmitEvent) {
    e.preventDefault();
    fehler = ""; arbeitet = true;
    try {
      const r = await fetch("/api/account/pledges/check-key", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ service: dienst, key: schluessel }) });
      if (r.status === 401) { anmelden(); return; }
      const j = await r.json().catch(() => ({}));
      if (!r.ok) { fehler = j.error ?? t.fehler; return; }
      geprueft = { balance: j.balance, expiring: j.expiring ?? null, renews: j.renews ?? null };
      credits = Math.max(1, Math.min(j.expiring ?? j.balance, j.balance));
      verfaellt = j.renews ? tagDavor(j.renews) : monatsende();
      if (verfaellt < iso(heute())) verfaellt = iso(heute());
      schritt = "menge"; fokus();
    } catch { fehler = t.fehler; } finally { arbeitet = false; }
  }
  async function zusagen(e: SubmitEvent) {
    e.preventDefault();
    fehler = ""; arbeitet = true;
    try {
      const r = await fetch("/api/account/pledges", { method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ project: slug, service: dienst, credits: Number(credits), expires: verfaellt, note: notiz.trim(), key: schluessel, rights_confirmed: rechte }) });
      if (r.status === 401) { anmelden(); return; }
      const j = await r.json().catch(() => ({}));
      if (!r.ok) { fehler = j.error ?? t.fehler; return; }
      try { sessionStorage.removeItem(ENTWURF); } catch { /* egal */ }
      schluessel = "";
      schritt = "fertig"; fokus();
      invalidate("app:projekt");
    } catch { fehler = t.fehler; } finally { arbeitet = false; }
  }
  // Zurück von der Anmeldung: Dienst wiederherstellen und das Menü offen zeigen; den Schlüssel fügt der Mensch neu ein
  onMount(() => {
    verfaellt = monatsende();
    let roh: string | null = null;
    try { roh = sessionStorage.getItem(ENTWURF); } catch { /* kein Speicher */ }
    if (!roh) return;
    try {
      const e = JSON.parse(roh);
      if (DIENSTE.includes(e.dienst)) { dienst = e.dienst; schritt = "schluessel"; offen = true; }
    } catch { /* kaputter Entwurf */ }
  });

  const schnell = $derived([
    ...(geprueft?.renews ? [{ name: t.wennVerfallen, wert: tagDavor(geprueft.renews) }] : [{ name: t.monatsende, wert: monatsende() }]),
    { name: t.inTagen(7), wert: inTagen(7) }, { name: t.inTagen(30), wert: inTagen(30) }]);
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
        <p class="leise klein">{t.andere} <a href={spendenDoku}>{t.runnerLink}</a></p>
      {:else if schritt === "schluessel"}
        <form onsubmit={pruefen}>
          <p class="frage" tabindex="-1" data-fokus>{t.frageSchluessel(dienstName(dienst))}</p>
          <p class="leise klein">{t.hilfe[dienst]} <a href={t.hilfeLink[dienst]} target="_blank" rel="noopener noreferrer">{t.hilfeLinkText}</a></p>
          <label>{t.schluessel}<input type="password" autocomplete="off" spellcheck="false" minlength="16" maxlength="200" bind:value={schluessel} required /></label>
          <p class="leise klein">{t.sicher}</p>
          {#if fehler}<p class="fehler" role="alert">{fehler}</p>{/if}
          <button class="knopf primaer" disabled={arbeitet}>{arbeitet ? t.prueft : t.pruefen}</button>
          <button type="button" class="zurueck leise" onclick={() => { schritt = "dienst"; schluessel = ""; fehler = ""; fokus(); }}>← {t.andererDienst}</button>
        </form>
      {:else if schritt === "menge" && geprueft}
        <form onsubmit={zusagen}>
          <p class="frage" tabindex="-1" data-fokus>{t.frageMenge}</p>
          <p class="klein">{t.guthaben(geprueft.balance, geprueft.expiring, geprueft.renews ? datumText(tagDavor(geprueft.renews)) : null)}</p>
          <label>{t.wieViele}<input type="number" inputmode="numeric" min="1" max={geprueft.balance} step="1" bind:value={credits} required /></label>
          <fieldset>
            <legend>{t.bisWann}</legend>
            <div class="chips">
              {#each schnell as c (c.name)}
                <button type="button" class="chip" aria-pressed={verfaellt === c.wert} onclick={() => (verfaellt = c.wert)}>{c.name}</button>
              {/each}
            </div>
            <input type="date" aria-label={t.datum} min={iso(heute())} bind:value={verfaellt} required />
          </fieldset>
          <label>{t.notiz}<input type="text" maxlength="500" bind:value={notiz} placeholder={t.notizHilfe} /></label>
          <label class="haken"><input type="checkbox" bind:checked={rechte} required /> <span>{t.rechte}</span></label>
          {#if fehler}<p class="fehler" role="alert">{fehler}</p>{/if}
          <button class="knopf primaer" disabled={arbeitet || !rechte}>{arbeitet ? t.sendet : t.zusagen}</button>
          <p class="leise klein">{t.soGehtsWeiter}</p>
          <button type="button" class="zurueck leise" onclick={() => { schritt = "schluessel"; fehler = ""; fokus(); }}>← {t.zurueck}</button>
        </form>
      {:else if schritt === "anmelden"}
        <p class="frage" tabindex="-1" data-fokus>{t.anmeldenH}</p>
        <p class="leise klein">{t.anmeldenText}</p>
        <a class="knopf primaer" href={`/api/account/login?lang=${s}&next=projekt-${slug}`} data-sveltekit-reload>{t.anmelden}</a>
        <button type="button" class="zurueck leise" onclick={() => { schritt = "schluessel"; fokus(); }}>← {t.zurueck}</button>
      {:else}
        <p class="frage danke" tabindex="-1" data-fokus role="status">{t.danke(Number(credits), dienstName(dienst), name)}</p>
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
  .haken { display: flex; gap: 8px; align-items: flex-start; color: var(--text); font-size: .8rem; line-height: 1.45; }
  .haken input { width: auto; margin-top: 3px; flex: none; }
  .panel .knopf { justify-content: center; }
</style>
