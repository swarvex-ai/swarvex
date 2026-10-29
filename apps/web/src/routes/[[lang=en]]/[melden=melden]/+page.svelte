<script lang="ts">
  // Meldeformular (Notice and Takedown): schickt an POST /api/reports, Betreiber bearbeiten Meldungen über die CLI.
  import { tick, untrack } from "svelte";
  import Seo from "$lib/components/Seo.svelte";
  import Captcha from "$lib/components/Captcha.svelte";
  import { pfad, sprache, tx } from "$lib/texte";

  let { data } = $props();
  const s = $derived(sprache());
  const t = $derived(tx().melden);
  const f = $derived(t.felder);

  let art = $state("infringement");
  let projekt = $state(untrack(() => data.projekt)); // Vorbelegung aus ?project=, danach frei editierbar
  let bezug = $state(untrack(() => data.bezug));
  let beschreibung = $state("");
  let name = $state("");
  let email = $state("");
  let rechteinhaber = $state(false);
  let versichert = $state(false);
  let website = $state("");
  let altcha = $state("");
  let captchaRunde = $state(0); // neue Runde = neue Aufgabe (nach Ablauf oder Ablehnung)

  let sendet = $state(false);
  let fehler = $state<Record<string, string>>({});
  let meldung = $state("");
  let nummer = $state<number | null>(null);
  let dankeEl = $state<HTMLElement>();

  function pruefen() {
    const e: Record<string, string> = {};
    // Reihenfolge wie im Formular: der Fokus springt ins erste fehlerhafte Feld
    if (beschreibung.trim().length < 20) e.beschreibung = t.fehler.kurz;
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) e.email = t.fehler.email;
    if (!versichert) e.versichert = t.fehler.versichert;
    return e;
  }

  async function absenden(ev: SubmitEvent) {
    ev.preventDefault();
    meldung = "";
    fehler = pruefen();
    if (Object.keys(fehler).length) {
      meldung = t.fehler.felder;
      await tick();
      document.getElementById(`m-${Object.keys(fehler)[0]}`)?.focus();
      return;
    }
    if (!altcha) {
      meldung = t.fehler.captcha;
      return;
    }
    sendet = true;
    try {
      const r = await fetch("/api/reports", {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ kind: art, project: projekt.trim(), location: bezug.trim(), description: beschreibung, name, email, rights_holder: rechteinhaber, confirmed: versichert, website, altcha }),
      });
      if (r.status === 201) {
        nummer = (await r.json()).id;
        await tick();
        dankeEl?.focus();
      } else if (r.status === 429) {
        meldung = t.fehler.zuViele;
      } else if (r.status === 400 && (await r.clone().json()).error === "captcha") {
        meldung = t.fehler.captcha;
        altcha = "";
        captchaRunde++;
      } else if (r.status === 400) {
        // Felder der API (englisch) → Formularfelder und Texte
        const felder = ((await r.json()).fields ?? []) as { field: string }[];
        const zuordnung: Record<string, [string, string]> = { email: ["email", t.fehler.email], description: ["beschreibung", t.fehler.kurz], confirmed: ["versichert", t.fehler.versichert] };
        fehler = Object.fromEntries(felder.map((x) => zuordnung[x.field] ?? [x.field, t.fehler.felder]));
        meldung = t.fehler.felder;
      } else {
        meldung = t.fehler.allgemein;
      }
    } catch {
      meldung = t.fehler.allgemein;
    } finally {
      sendet = false;
    }
  }
</script>

<Seo titel={t.seoTitel} beschreibung={t.beschreibung} pfad={pfad(s, t.pfad)} alternativen={{ de: "/melden", en: "/en/report" }} />

<section class="huelle seite">
  <header>
    <h1>{t.h1}</h1>
    <p class="lead">{t.lead}</p>
  </header>

  <div class="raster">
    {#if nummer !== null}
      <div class="danke" tabindex="-1" bind:this={dankeEl} role="status">
        <h2>{t.danke}</h2>
        <p class="nr"><span class="leise">{t.nummer}</span> <strong>#{nummer}</strong></p>
        <p>{t.dankeText}</p>
      </div>
    {:else}
      <form onsubmit={absenden} novalidate>
        <div class="feld">
          <label for="m-art">{f.art}</label>
          <select id="m-art" bind:value={art}>
            {#each Object.entries(t.arten) as [wert, text]}<option value={wert}>{text}</option>{/each}
          </select>
          {#if art === "security"}
            <p class="hilfe">{t.luecke} <a href={pfad(s, s === "de" ? "/docs/sicherheit" : "/docs/security")}>{t.lueckeLink}</a></p>
          {/if}
        </div>

        <div class="zwei">
          <div class="feld">
            <label for="m-projekt">{f.projekt} <span class="leise">({f.optional})</span></label>
            <input id="m-projekt" bind:value={projekt} maxlength="80" autocomplete="off" aria-describedby="h-projekt" />
            <p class="hilfe" id="h-projekt">{f.projektHilfe}</p>
          </div>
          <div class="feld">
            <label for="m-bezug">{f.bezug} <span class="leise">({f.optional})</span></label>
            <input id="m-bezug" bind:value={bezug} maxlength="500" inputmode="url" autocomplete="off" aria-describedby="h-bezug" />
            <p class="hilfe" id="h-bezug">{f.bezugHilfe}</p>
          </div>
        </div>

        <div class="feld">
          <label for="m-beschreibung">{f.beschreibung}</label>
          <textarea id="m-beschreibung" bind:value={beschreibung} rows="7" maxlength="5000" required
                    aria-invalid={!!fehler.beschreibung} aria-describedby="h-beschreibung"></textarea>
          {#if fehler.beschreibung}<p class="fehler">{fehler.beschreibung}</p>{/if}
          <p class="hilfe" id="h-beschreibung">{f.beschreibungHilfe}</p>
        </div>

        <div class="zwei">
          <div class="feld">
            <label for="m-name">{f.name} <span class="leise">({f.optional})</span></label>
            <input id="m-name" bind:value={name} maxlength="200" autocomplete="name" />
          </div>
          <div class="feld">
            <label for="m-email">{f.email}</label>
            <input id="m-email" type="email" bind:value={email} maxlength="200" autocomplete="email" required
                   aria-invalid={!!fehler.email} aria-describedby="h-email" />
            {#if fehler.email}<p class="fehler">{fehler.email}</p>{/if}
            <p class="hilfe" id="h-email">{f.emailHilfe}</p>
          </div>
        </div>

        <div class="honig" aria-hidden="true">
          <label for="m-website">Website</label>
          <input id="m-website" name="website" bind:value={website} tabindex="-1" autocomplete="off" />
        </div>

        <label class="haken"><input type="checkbox" bind:checked={rechteinhaber} /> <span>{f.rechteinhaber}</span></label>
        <label class="haken" class:falsch={!!fehler.versichert}>
          <input id="m-versichert" type="checkbox" bind:checked={versichert} aria-invalid={!!fehler.versichert} />
          <span>{f.versichert}</span>
        </label>
        {#if fehler.versichert}<p class="fehler">{fehler.versichert}</p>{/if}

        {#key captchaRunde}<Captcha texte={t.captcha} onloesung={(n) => (altcha = n)} />{/key}

        <div class="absenden">
          <button class="knopf primaer" type="submit" disabled={sendet}>{sendet ? t.sendet : t.senden}</button>
          <p class="meldung" role="alert">{meldung}</p>
        </div>
        <p class="daten leise">{t.daten}</p>
      </form>
    {/if}

    <aside aria-labelledby="t-ablauf">
      <h2 id="t-ablauf">{t.ablaufH2}</h2>
      <ol>
        {#each t.ablauf as [schritt, text], i}
          <li><span class="zahl" aria-hidden="true">{i + 1}</span><div><h3>{schritt}</h3><p>{text}</p></div></li>
        {/each}
      </ol>
    </aside>
  </div>
</section>

<style>
  .seite { padding-top: clamp(48px, 7vw, 88px); }
  h1 { font-size: clamp(2.2rem, 4.5vw, 3.4rem); margin: 0 0 16px; }
  .lead { font-size: 1.15rem; color: var(--stahl); max-width: 58ch; margin: 0; }
  .raster { display: grid; grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr); gap: 32px 56px; margin-top: 48px; align-items: start; }
  form, .danke { display: grid; gap: 22px; }
  .zwei { display: grid; grid-template-columns: 1fr 1fr; gap: 22px; }
  .feld { display: grid; gap: 6px; min-width: 0; align-content: start; }
  label { font-weight: 500; }
  input:not([type="checkbox"]), select, textarea {
    width: 100%; font: inherit; color: var(--text); background: var(--metall); border: 1px solid var(--linie-hell);
    border-radius: var(--radius-klein); padding: 10px 12px; min-height: 44px; transition: border-color var(--dauer);
  }
  textarea { resize: vertical; line-height: 1.5; }
  input:focus-visible, select:focus-visible, textarea:focus-visible { outline: 2px solid var(--akzent); outline-offset: 1px; border-color: var(--akzent); }
  [aria-invalid="true"] { border-color: var(--rot) !important; }
  .hilfe { margin: 0; font-size: .88rem; color: var(--leise); }
  .fehler { margin: 0; font-size: .9rem; color: var(--rot); }
  .haken { display: flex; gap: 12px; align-items: flex-start; font-weight: 400; cursor: pointer; }
  .haken input { width: 20px; height: 20px; margin: 2px 0 0; flex: none; accent-color: var(--akzent); }
  .haken.falsch span { color: var(--rot); }
  .honig { position: absolute; left: -9999px; width: 1px; height: 1px; overflow: hidden; }
  .absenden { display: flex; flex-wrap: wrap; gap: 12px 20px; align-items: center; }
  .absenden button[disabled] { opacity: .6; cursor: progress; }
  .meldung { margin: 0; color: var(--rot); }
  .meldung:empty { display: none; }
  .daten { margin: 0; font-size: .88rem; max-width: 62ch; }
  .danke { border: 1px solid var(--akzent); border-radius: var(--radius); padding: 28px; background: color-mix(in srgb, var(--akzent) 6%, var(--graphit)); }
  .danke:focus { outline: none; }
  .danke h2 { margin: 0; font-size: 1.5rem; }
  .danke p { margin: 0; }
  .nr strong { font: 600 1.4rem var(--mono); color: var(--akzent); }
  aside { border: 1px solid var(--linie); border-radius: var(--radius); padding: 24px; background: var(--metall); }
  aside h2 { font-size: 1.15rem; margin: 0 0 18px; }
  aside ol { list-style: none; margin: 0; padding: 0; display: grid; gap: 18px; }
  aside li { display: flex; gap: 14px; }
  .zahl { flex: none; width: 28px; height: 28px; border-radius: 50%; border: 1px solid var(--linie-hell); display: grid; place-items: center; font: 500 .85rem var(--mono); color: var(--akzent); }
  aside h3 { margin: 2px 0 4px; font-size: 1rem; }
  aside p { margin: 0; color: var(--stahl); font-size: .93rem; }
  @media (max-width: 900px) { .raster { grid-template-columns: 1fr; } }
  @media (max-width: 600px) { .zwei { grid-template-columns: 1fr; } }
</style>
