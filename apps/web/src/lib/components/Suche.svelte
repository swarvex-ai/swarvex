<script lang="ts">
  // Doku-Suche (⌘K / Strg+K): lädt den vorgerenderten Index und sucht in Titeln und Abschnitten.
  import { goto } from "$app/navigation";
  import { pfad, sprache, tx } from "$lib/texte";
  const t = $derived(tx().suche);

  type Treffer = { slug: string; seite: string; gruppe: string; id: string; titel: string; auszug: string; punkte: number };
  type Index = { slug: string; titel: string; gruppe: string; abschnitte: { id: string; titel: string; text: string }[] }[];

  let dialog: HTMLDialogElement;
  let eingabe = $state<HTMLInputElement>();
  let anfrage = $state("");
  let index: Index | null = null;
  let auswahl = $state(0);
  let treffer = $state<Treffer[]>([]);

  export async function oeffnen() {
    dialog.showModal();
    eingabe?.focus();
    index ??= await (await fetch(pfad(sprache(), "/docs/suche.json"))).json();
    suchen();
  }

  function suchen() {
    const woerter = anfrage.toLowerCase().split(/\s+/).filter(Boolean);
    if (!index || !woerter.length) { treffer = []; return; }
    const ergebnis: Treffer[] = [];
    for (const s of index) for (const a of s.abschnitte) {
      const titel = a.titel.toLowerCase(), text = a.text.toLowerCase();
      let punkte = 0;
      for (const w of woerter) {
        if (titel.includes(w)) punkte += 5;
        else if (text.includes(w)) punkte += 1;
        else { punkte = 0; break; }
      }
      if (punkte) {
        const stelle = Math.max(0, text.indexOf(woerter[0]) - 40);
        ergebnis.push({ slug: s.slug, seite: s.titel, gruppe: s.gruppe, id: a.id, titel: a.titel, punkte,
                        auszug: (stelle ? "… " : "") + a.text.slice(stelle, stelle + 140) + "…" });
      }
    }
    treffer = ergebnis.sort((a, b) => b.punkte - a.punkte).slice(0, 12);
    auswahl = 0;
  }

  function springen(z: Treffer) { dialog.close(); goto(pfad(sprache(), `/docs/${z.slug}`) + (z.id ? "#" + z.id : "")); }

  function taste(e: KeyboardEvent) {
    if (e.key === "ArrowDown") { auswahl = Math.min(auswahl + 1, treffer.length - 1); e.preventDefault(); }
    if (e.key === "ArrowUp") { auswahl = Math.max(auswahl - 1, 0); e.preventDefault(); }
    if (e.key === "Enter" && treffer[auswahl]) springen(treffer[auswahl]);
  }
</script>

<svelte:window onkeydown={(e) => { if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); oeffnen(); } }} />

<dialog bind:this={dialog} aria-label={t.titel} onclick={(e) => { if (e.target === dialog) dialog.close(); }}>
  <div class="kasten">
    <label class="feld">
      <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><circle cx="11" cy="11" r="6.5" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M16 16l4 4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
      <input bind:this={eingabe} bind:value={anfrage} oninput={suchen} onkeydown={taste} placeholder={t.platzhalter} aria-label={t.begriff} />
      <kbd>Esc</kbd>
    </label>
    {#if treffer.length}
      <ul role="listbox" aria-label={t.treffer}>
        {#each treffer as z, i}
          <li role="option" aria-selected={i === auswahl}>
            <button type="button" onclick={() => springen(z)} onmouseenter={() => (auswahl = i)}>
              <span class="ort leise">{z.gruppe} / {z.seite}</span>
              <strong>{z.titel}</strong>
              <span class="auszug leise">{z.auszug}</span>
            </button>
          </li>
        {/each}
      </ul>
    {:else if anfrage}
      <p class="leer leise">{t.leer} <a href={pfad(sprache(), sprache() === "de" ? "/docs/ueberblick" : "/docs/overview")}>{t.ueberblick}</a>.</p>
    {/if}
  </div>
</dialog>

<style>
  dialog { padding: 0; border: 1px solid var(--linie-hell); border-radius: 12px; background: var(--graphit-2); color: var(--text);
    width: min(640px, calc(100vw - 32px)); margin-top: 12vh; }
  dialog::backdrop { background: rgba(5, 8, 12, .6); backdrop-filter: blur(3px); }
  .feld { display: flex; align-items: center; gap: 10px; padding: 0 14px; border-bottom: 1px solid var(--linie); color: var(--leise); }
  input { flex: 1; min-height: 54px; border: 0; background: none; color: var(--text); font: 400 1.05rem var(--schrift); outline: none; }
  kbd { font-size: .75rem; padding: 2px 6px; border: 1px solid var(--linie-hell); border-radius: 4px; }
  ul { list-style: none; margin: 0; padding: 6px; max-height: 60vh; overflow: auto; }
  li button { display: grid; gap: 2px; width: 100%; text-align: left; padding: 10px 12px; border: 0; border-radius: 8px; background: none;
    color: var(--text); font: inherit; cursor: pointer; }
  li[aria-selected="true"] button { background: var(--graphit-3); box-shadow: inset 2px 0 0 var(--akzent); }
  .ort { font-size: .8rem; }
  strong { font-weight: 600; }
  .auszug { font-size: .88rem; overflow: hidden; display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; }
  .leer { padding: 18px; margin: 0; }
</style>
