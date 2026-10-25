<script lang="ts">
  // „Du behältst die Kontrolle“ zum Ausprobieren: Jede Einstellung ändert sofort den Satz darunter,
  // der beschreibt, was dein Agent darf. Der Runner setzt genau diese Grenzen lokal durch.
  import { tx } from "$lib/texte";
  const t = $derived(tx().kontrolle);
  // Auswahl als Indizes, damit sie beim Sprachwechsel erhalten bleibt
  let projekte = $state([0]);
  let code = $state(true);
  let design = $state(false);
  let modell = $state(0);
  let budget = $state(2);
  let pausiert = $state(false);

  // Zeitfenster: Wochentage (Mo = 0 … So = 6) und Uhrzeit, frei wählbar oder per Vorlage
  let tage = $state([true, true, true, true, true, true, true]);
  let von = $state("22:00");
  let bis = $state("07:00");
  let immer = $state(false);
  const vorlagen = {
    immer: () => { immer = true; tage = Array(7).fill(true); von = "00:00"; bis = "24:00"; },
    nachts: () => { immer = false; tage = Array(7).fill(true); von = "22:00"; bis = "07:00"; },
    abends: () => { immer = false; tage = [true, true, true, true, true, false, false]; von = "18:00"; bis = "23:00"; },
    wochenende: () => { immer = false; tage = [false, false, false, false, false, true, true]; von = "09:00"; bis = "24:00"; },
  };
  const aktiveVorlage = $derived(
    immer ? "immer"
    : tage.every(Boolean) && von === "22:00" && bis === "07:00" ? "nachts"
    : tage.join() === "true,true,true,true,true,false,false" && von === "18:00" && bis === "23:00" ? "abends"
    : tage.join() === "false,false,false,false,false,true,true" && von === "09:00" && bis === "24:00" ? "wochenende" : null);
  const stunden = Array.from({ length: 25 }, (_, h) => `${String(h).padStart(2, "0")}:00`);

  // Tage zu kurzen Bereichen zusammenfassen: [mo..fr] → „mo-fr“, [sa, so] → „sa,so“
  function tageKurz(namen: string[], trenner: string) {
    if (tage.every(Boolean)) return null;
    const teile: string[] = [];
    for (let i = 0; i < 7; i++) {
      if (!tage[i]) continue;
      let j = i;
      while (j + 1 < 7 && tage[j + 1]) j++;
      teile.push(j - i >= 2 ? `${namen[i]}${trenner}${namen[j]}` : j > i ? `${namen[i]},${namen[j]}` : namen[i]);
      i = j;
    }
    return teile.join(",");
  }
  const zeitText = $derived.by(() => {
    if (immer) return t.rund;
    if (!tage.some(Boolean)) return t.keineTage;
    const tg = tageKurz(t.tage, "–")?.replaceAll(",", ", ") ?? t.taeglich;
    return `${tg} ${von.replace(/^0/, "")}–${bis.replace(/^0/, "")}${t.uhr}`;
  });
  const runnerZeile = $derived(immer || !tage.some(Boolean) ? "" : `--zeitfenster "${tageKurz(t.tagCodes, "-") ?? t.taeglich} ${von}-${bis}"`);

  const umschalten = (i: number) => (projekte = projekte.includes(i) ? projekte.filter((x) => x !== i) : [...projekte, i].sort());
  const liste = (a: string[]) => (a.length < 2 ? a.join("") : `${a.slice(0, -1).join(", ")} ${t.und} ${a.at(-1)}`);
  const arten = $derived(liste([code && t.code, design && t.design].filter(Boolean) as string[]) || t.keine);
</script>

<div class="panel" class:pausiert>
  <div class="einstellungen">
    <fieldset>
      <legend>{t.projekte}</legend>
      <div class="chips">
        {#each t.alleProjekte as p, i}
          <button type="button" class="chip" aria-pressed={projekte.includes(i)} onclick={() => umschalten(i)}>{p}</button>
        {/each}
      </div>
    </fieldset>
    <fieldset>
      <legend>{t.aufgaben}</legend>
      <div class="chips">
        <button type="button" class="chip" aria-pressed={code} onclick={() => (code = !code)}>{t.code}</button>
        <button type="button" class="chip" aria-pressed={design} onclick={() => (design = !design)}>{t.design}</button>
      </div>
    </fieldset>
    <label class="feld">
      <span>{t.modell}</span>
      <select bind:value={modell}>{#each t.modelle as m, i}<option value={i}>{m}</option>{/each}</select>
    </label>
    <fieldset class="wann">
      <legend>{t.wann}</legend>
      <div class="chips">
        {#each Object.entries(vorlagen) as [name, setzen]}
          <button type="button" class="chip" aria-pressed={aktiveVorlage === name} onclick={setzen}>{t.vorlagen[name as keyof typeof vorlagen]}</button>
        {/each}
      </div>
      {#if !immer}
        <div class="tage" role="group" aria-label={t.wann}>
          {#each t.tage as tag, i}
            <button type="button" class="tag" aria-pressed={tage[i]} onclick={() => (tage[i] = !tage[i])}>{tag}</button>
          {/each}
        </div>
        <div class="uhrzeit">
          <label>{t.von} <select bind:value={von}>{#each stunden.slice(0, 24) as h}<option>{h}</option>{/each}</select></label>
          <label>{t.bis} <select bind:value={bis}>{#each stunden.slice(1) as h}<option>{h}</option>{/each}</select></label>
        </div>
      {/if}
    </fieldset>
    <label class="feld">
      <span>{t.budget}: <strong class="mono">{budget} {t.mio}</strong></span>
      <input type="range" min="0.5" max="10" step="0.5" bind:value={budget} />
    </label>
  </div>

  <div class="ergebnis">
    <p class="satz" aria-live="polite">
      {#if pausiert}
        {@html t.angehalten}
      {:else if projekte.length === 0 || !(code || design)}
        {t.wartet}
      {:else}
        {@html t.satz(liste(projekte.map((i) => t.alleProjekte[i])), arten, t.modelle[modell], zeitText, budget)}
      {/if}
    </p>
    <button type="button" class="knopf pause" aria-pressed={pausiert} onclick={() => (pausiert = !pausiert)}>
      {#if pausiert}
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M8 5l11 7-11 7z" fill="currentColor"/></svg>{t.weiter}
      {:else}
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M7 5h3v14H7zM14 5h3v14h-3z" fill="currentColor"/></svg>{t.anhalten}
      {/if}
    </button>
    {#if runnerZeile && !pausiert}
      <p class="runner"><span class="leise">{t.runner}:</span> <code>{runnerZeile}</code></p>
    {/if}
    <p class="leise hinweis">{t.hinweis}</p>
  </div>
</div>

<style>
  .panel { display: grid; grid-template-columns: 1.1fr 1fr; border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); overflow: hidden; }
  .einstellungen { padding: 24px; display: grid; gap: 18px; border-right: 1px solid var(--linie); transition: opacity .3s; }
  .pausiert .einstellungen { opacity: .4; }
  fieldset { border: 0; margin: 0; padding: 0; }
  legend, .feld > span { display: block; margin-bottom: 8px; color: var(--stahl); font-size: .92rem; }
  .chips { display: flex; flex-wrap: wrap; gap: 8px; }
  .chip { min-height: 40px; padding: 0 14px; border-radius: 6px; border: 1px solid var(--linie-hell); background: var(--graphit-2);
    color: var(--stahl); font: 500 .9rem var(--schrift); cursor: pointer; transition: all var(--dauer); }
  .chip[aria-pressed="true"] { border-color: var(--akzent); color: var(--akzent); background: color-mix(in srgb, var(--akzent) 8%, transparent); }
  select { min-height: 42px; width: 100%; max-width: 260px; padding: 0 10px; border-radius: 6px; border: 1px solid var(--linie-hell);
    background: var(--graphit-2); color: var(--text); font: 400 .95rem var(--schrift); }
  .tage { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 12px; }
  .tag { min-width: 44px; min-height: 40px; border-radius: 6px; border: 1px solid var(--linie-hell); background: var(--graphit-2);
    color: var(--leise); font: 500 .85rem var(--schrift); cursor: pointer; transition: all var(--dauer); }
  .tag[aria-pressed="true"] { border-color: var(--akzent); color: var(--akzent); background: color-mix(in srgb, var(--akzent) 8%, transparent); }
  .uhrzeit { display: flex; flex-wrap: wrap; gap: 16px; margin-top: 12px; color: var(--stahl); font-size: .92rem; }
  .uhrzeit select { width: auto; min-width: 96px; margin-left: 6px; font-family: var(--mono); }
  .runner { margin: 0; font-size: .85rem; display: flex; flex-wrap: wrap; gap: 6px; align-items: baseline; }
  .runner code { padding: 3px 8px; border: 1px solid var(--linie); border-radius: 5px; background: var(--graphit-2); color: var(--text); font-size: .82rem; }
  input[type="range"] { width: 100%; accent-color: var(--bernstein); }
  .mono { font-family: var(--mono); font-weight: 500; color: var(--bernstein); }
  .ergebnis { padding: 24px; display: flex; flex-direction: column; justify-content: center; gap: 18px; }
  .satz { font-size: 1.15rem; line-height: 1.55; margin: 0; min-height: 5.5em; }
  .satz :global(strong) { color: var(--akzent); font-weight: 500; }
  .pausiert .satz :global(strong) { color: var(--rot); }
  .pause { align-self: flex-start; }
  .pause[aria-pressed="true"] { border-color: var(--rot); color: var(--rot); }
  .hinweis { font-size: .88rem; margin: 0; }
  @media (max-width: 820px) { .panel { grid-template-columns: 1fr; } .einstellungen { border-right: 0; border-bottom: 1px solid var(--linie); } }
</style>
