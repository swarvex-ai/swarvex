<script lang="ts">
  // Manifest, Clean-Room-Verfahren und Dankesliste. Die Liste kommt aus docs/dank.json (auch Quelle von DANK.md).
  import Flagge from "$lib/components/Flagge.svelte";
  import Seo from "$lib/components/Seo.svelte";
  import { SEITE } from "$lib/seo";
  import { pfad, sprache, tx } from "$lib/texte";
  import dank from "../../../../../../docs/dank.json";

  const s = $derived(sprache());
  const t = $derived(tx().opensource);
  type Eintrag = { name: string; url: string; lizenz: string; de: string; en: string; gruppe: string };
  const gruppen = $derived(Object.entries(t.dank.gruppen).map(([k, titel]) => ({ k, titel, eintraege: (dank as Eintrag[]).filter((e) => e.gruppe === k) })));
  const daten = $derived([{ "@type": "WebPage", "@id": `${SEITE}${pfad(s, t.pfad)}`, name: t.h1, description: t.beschreibung, inLanguage: s,
    isPartOf: { "@id": `${SEITE}/#website` } }]);
</script>

<Seo titel={t.seoTitel} beschreibung={t.beschreibung} pfad={pfad(s, t.pfad)} bild={s === "en" ? "/og-en.png" : "/og.png"}
     alternativen={{ de: t.pfad, en: `/en${t.pfad}` }} {daten} />

<section class="kopf-bereich">
  <div class="huelle kopf-raster">
    <div class="kopf-text">
      <h1>{t.h1}</h1>
      <p class="lead">{t.lead}</p>
    </div>
    <div class="kopf-flagge"><Flagge label={t.flagge} /></div>
  </div>
</section>

<section class="huelle abschnitt" aria-labelledby="t-thesen">
  <h2 id="t-thesen">{t.thesenH2}</h2>
  <ol class="thesen">
    {#each t.thesen as [these, text], i}
      <li><span class="nr" aria-hidden="true">{String(i + 1).padStart(2, "0")}</span><h3>{these}</h3><p>{text}</p></li>
    {/each}
  </ol>
</section>

<section class="huelle abschnitt" aria-labelledby="t-clean">
  <h2 id="t-clean">{t.clean.h2}</h2>
  <p class="einleitung leise">{t.clean.text}</p>
  <div class="raeume">
    {#each t.clean.raeume as [raum, text], i}
      <div class="raum" class:bau={i === 2}>
        <h3>{raum}</h3>
        <p>{text}</p>
      </div>
      {#if i === 0}<div class="wand" aria-hidden="true"><span>{t.clean.wand}</span></div>{/if}
    {/each}
  </div>
  <h3 class="regeln-titel">{t.clean.regelnH3}</h3>
  <ul class="regeln">{#each t.clean.regeln as r}<li>{r}</li>{/each}</ul>
  <p class="hinweis">{t.clean.hinweis}</p>
  <a class="knopf" href={pfad(s, "/docs/clean-room")}>{t.clean.doku}</a>
</section>

<section class="huelle abschnitt" aria-labelledby="t-dank">
  <h2 id="t-dank">{t.dank.h2}</h2>
  <p class="einleitung leise">{t.dank.text}</p>
  {#each gruppen as g}
    <h3 class="gruppe">{g.titel}</h3>
    <div class="tabelle-huelle">
      <table>
        <thead><tr><th scope="col">{t.dank.projekt}</th><th scope="col">{t.dank.lizenz}</th><th scope="col">{t.dank.wofuer}</th></tr></thead>
        <tbody>
          {#each g.eintraege as e}
            <tr><td><a href={e.url} rel="noopener">{e.name}</a></td><td class="lizenz">{e.lizenz}</td><td>{s === "en" ? e.en : e.de}</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/each}
  <p class="leise klein">{t.dank.dienste}</p>
</section>

<style>
  .kopf-bereich { border-bottom: 1px solid var(--linie); background: radial-gradient(ellipse at 75% 40%, color-mix(in srgb, var(--akzent) 7%, transparent), transparent 60%), var(--graphit); }
  .kopf-raster { display: grid; grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr); gap: clamp(24px, 5vw, 64px); align-items: center; padding-block: clamp(40px, 8vw, 96px); }
  .abschnitt { padding-top: clamp(64px, 9vw, 112px); }
  .abschnitt:last-of-type { padding-bottom: clamp(64px, 9vw, 112px); }
  h1 { font-size: clamp(2.4rem, 5vw, 4rem); margin: 0 0 20px; text-wrap: balance; }
  .lead { font-size: 1.2rem; color: var(--stahl); max-width: 56ch; margin: 0; }
  .kopf-flagge { min-width: 0; }

  .thesen { list-style: none; padding: 0; margin: 0; display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 300px), 1fr)); gap: 16px; }
  .thesen li { border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); padding: 22px; }
  .thesen .nr { font-family: var(--mono); color: var(--akzent); font-size: .9rem; }
  .thesen h3 { margin: 8px 0 8px; font-size: 1.2rem; }
  .thesen p { margin: 0; color: var(--stahl); }

  .raeume { display: grid; grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr) minmax(0, 1fr); gap: 14px; align-items: stretch; margin: 26px 0; }
  .raum { border: 1px solid var(--linie); border-radius: var(--radius); padding: 20px; background: var(--graphit-2); }
  .raum h3 { margin: 0 0 8px; font-family: var(--mono); font-size: 1rem; color: var(--bernstein); }
  .raum.bau h3 { color: var(--akzent); }
  .raum p { margin: 0; color: var(--stahl); }
  .wand { display: grid; place-items: center; width: 34px; border-inline: 2px dashed var(--linie-hell); }
  .wand span { writing-mode: vertical-rl; transform: rotate(180deg); font-family: var(--mono); font-size: .78rem; background: var(--graphit); padding: 8px 2px; color: var(--text); letter-spacing: .08em; text-transform: uppercase; }
  .regeln-titel { margin: 8px 0 10px; }
  .regeln { margin: 0 0 18px; padding-left: 1.2em; display: grid; gap: 6px; color: var(--stahl); max-width: 62ch; }
  .hinweis { border-left: 3px solid var(--bernstein); padding: 4px 0 4px 16px; color: var(--stahl); max-width: 62ch; margin: 0 0 22px; }
  .einleitung { max-width: 62ch; }

  .gruppe { margin: 30px 0 10px; font-size: 1.05rem; }
  .tabelle-huelle { overflow-x: auto; border: 1px solid var(--linie); border-radius: var(--radius); }
  table { width: 100%; border-collapse: collapse; font-size: .95rem; }
  th, td { text-align: left; padding: 10px 14px; border-bottom: 1px solid var(--linie); vertical-align: top; }
  th { font-weight: 500; color: var(--leise); font-size: .85rem; }
  tbody tr:last-child td { border-bottom: 0; }
  td a { color: var(--text); text-decoration-color: var(--linie-hell); overflow-wrap: anywhere; }
  .lizenz { font-family: var(--mono); font-size: .85rem; color: var(--stahl); white-space: nowrap; }
  .klein { font-size: .9rem; margin-top: 18px; }

  @media (max-width: 860px) {
    .kopf-raster { grid-template-columns: minmax(0, 1fr); }
    .kopf-flagge { order: -1; max-width: 520px; }
    .raeume { grid-template-columns: minmax(0, 1fr); }
    .wand { width: auto; height: 34px; border-inline: 0; border-block: 2px dashed var(--linie-hell); }
    .wand span { writing-mode: horizontal-tb; transform: none; padding: 2px 8px; }
    .lizenz { white-space: normal; }
  }
</style>
