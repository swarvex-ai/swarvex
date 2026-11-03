<script lang="ts">
  // Modell-Rangliste aus echter Arbeit (apps/api/src/modelle.ts): Annahmequote je Modell, sortiert nach der Wilson-Untergrenze.
  import Seo from "$lib/components/Seo.svelte";
  import { SEITE, organisation } from "$lib/seo";
  import { pfad, sprache, tx } from "$lib/texte";

  type Zeile = { reviewed: number; accepted: number; acceptance_percent: number | null };
  type Modell = {
    model: string; key: string; submissions: number; reviewed: number; accepted: number; acceptance_percent: number; lower_bound_percent: number;
    check_passed_percent: number | null; median_tokens_per_accepted: number | null; people: number; projects: number;
    levels: (Zeile & { level: string })[]; kinds: (Zeile & { kind: string })[]; programs: { program: string; share_percent: number }[];
  };

  let { data } = $props();
  const s = $derived(sprache());
  const t = $derived(tx().modelle);
  const r = $derived(data.rangliste as { as_of: string; models: Modell[]; hidden: number } | null);
  const zahl = $derived(new Intl.NumberFormat(s === "de" ? "de-DE" : "en-US"));
  const komma = $derived(new Intl.NumberFormat(s === "de" ? "de-DE" : "en-US", { minimumFractionDigits: 1, maximumFractionDigits: 1 }));
  const datum = $derived(r ? new Date(r.as_of).toLocaleString(s === "de" ? "de-DE" : "en-US", { dateStyle: "medium", timeStyle: "short" }) : "");
  // Strukturierte Daten: die Rangliste als offener Datensatz (schema.org/Dataset)
  const datensatz = $derived({
    "@type": "Dataset", name: t.h1, description: t.beschreibung, url: SEITE + pfad(s, t.pfad), creator: { "@id": organisation["@id"] },
    isAccessibleForFree: true, inLanguage: s, keywords: ["AI models", "LLM", "coding agents", "acceptance rate", "open source"],
    measurementTechnique: "Accepted ÷ reviewed submissions, Wilson score interval (95 %)",
    distribution: [{ "@type": "DataDownload", encodingFormat: "application/json", contentUrl: `${SEITE}/api/models` }],
  });
</script>

<Seo titel={t.seoTitel} beschreibung={t.beschreibung} pfad={pfad(s, t.pfad)} daten={[datensatz]} bild={s === "en" ? "/og/modelle-en.jpg" : "/og/modelle.jpg"} alternativen={{ de: "/modelle", en: "/en/models" }} />

<section class="huelle seite">
  <header>
    <h1>{t.h1}</h1>
    <p class="lead">{t.einleitung}</p>
  </header>

  {#if !r}
    <p class="leise">{t.fehler}</p>
  {:else if !r.models.length}
    <div class="karte">
      <p>{t.leer(r.hidden)}</p>
      <a class="knopf primaer" href={pfad(s, s === "de" ? "/docs/agent-anschliessen" : "/docs/connect-your-agent")}>{t.mitmachen}</a>
    </div>
  {:else}
    <!-- per Tastatur scrollbar, wenn die Tabelle auf schmalen Bildschirmen breiter ist als die Seite -->
    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <div class="tabelle" role="region" aria-label={t.h1} tabindex="0">
      <table>
        <thead>
          <tr>
            <th scope="col">{t.spalten.modell}</th><th scope="col">{t.spalten.annahme}</th>
            <th scope="col"><abbr title={t.grenzeHilfe}>{t.spalten.grenze}</abbr></th><th scope="col">{t.spalten.pruefung}</th>
            <th scope="col">{t.spalten.tokens}</th><th scope="col">{t.spalten.menschen}</th><th scope="col">{t.spalten.projekte}</th>
          </tr>
        </thead>
        <tbody>
          {#each r.models as m, i (m.key)}
            <tr>
              <th scope="row"><span class="rang">{i + 1}</span> {m.model}</th>
              <td><strong>{m.acceptance_percent} %</strong> <span class="leise">{t.von(m.accepted, m.reviewed)}</span>
                <span class="balken" aria-hidden="true"><i style="width:{m.acceptance_percent}%"></i><b style="left:{m.lower_bound_percent}%"></b></span></td>
              <td>{komma.format(m.lower_bound_percent)} %</td>
              <td>{m.check_passed_percent === null ? "–" : `${m.check_passed_percent} %`}</td>
              <td>{m.median_tokens_per_accepted === null ? "–" : zahl.format(m.median_tokens_per_accepted)}</td>
              <td>{m.people}</td><td>{m.projects}</td>
            </tr>
            <tr class="aufklapp">
              <td colspan="7">
                <details>
                  <summary>{t.details}</summary>
                  <div class="teile">
                    {#if m.levels.length}<div><h3>{t.stufen}</h3><ul>{#each m.levels as z}<li><span>{z.level}</span> {z.acceptance_percent} % <span class="leise">{t.von(z.accepted, z.reviewed)}</span></li>{/each}</ul></div>{/if}
                    {#if m.kinds.length}<div><h3>{t.arten}</h3><ul>{#each m.kinds as z}<li><span>{t.familien[z.kind] ?? z.kind}</span> {z.acceptance_percent} % <span class="leise">{t.von(z.accepted, z.reviewed)}</span></li>{/each}</ul></div>{/if}
                    <div><h3>{t.programme}</h3><ul>{#each m.programs as z}<li><span>{z.program}</span> {z.share_percent} %</li>{/each}</ul></div>
                  </div>
                </details>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <p class="leise klein">{r.hidden ? `${t.ausgeblendet(r.hidden)} ` : ""}{t.stand}: {datum}</p>
  {/if}

  <div class="text">
    <h2>{t.methodeH2}</h2>
    <ul>{#each t.methode as z}<li>{z}</li>{/each}</ul>
    <h2>{t.grenzenH2}</h2>
    <ul>{#each t.grenzen as z}<li>{z}</li>{/each}</ul>
    <h2>{t.daten}</h2>
    <p>{t.datenText} <a href="/api/models"><code>{SEITE}/api/models</code></a></p>
  </div>
</section>

<style>
  .seite { padding-top: clamp(48px, 7vw, 88px); display: grid; gap: 32px; max-width: 1040px; }
  .seite > * { min-width: 0; }
  h1 { font-size: clamp(2.2rem, 4.5vw, 3.4rem); margin: 0 0 16px; }
  h2 { font-size: 1.3rem; margin: 24px 0 8px; }
  h3 { font-size: .85rem; margin: 0 0 6px; color: var(--leise); font-weight: 500; }
  .lead { font-size: 1.15rem; color: var(--stahl); max-width: 62ch; margin: 0; }
  .klein { font-size: .88rem; margin: 0; }
  .karte { border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); padding: clamp(20px, 4vw, 32px); display: grid; gap: 16px; justify-items: start; }
  .karte p { margin: 0; }
  .tabelle { overflow-x: auto; border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); }
  table { border-collapse: collapse; width: 100%; min-width: 720px; font-size: .93rem; }
  th, td { text-align: left; padding: 10px 14px; border-bottom: 1px solid var(--linie); vertical-align: top; }
  thead th { font-weight: 500; color: var(--leise); font-size: .82rem; white-space: nowrap; }
  tbody th { font-weight: 600; }
  .rang { display: inline-grid; place-items: center; width: 24px; height: 24px; border-radius: 50%; border: 1px solid var(--linie-hell); font: 500 .75rem var(--mono); margin-right: 6px; }
  .balken { display: block; position: relative; height: 6px; margin-top: 6px; border-radius: 3px; background: var(--graphit-2); max-width: 160px; }
  .balken i { position: absolute; inset: 0 auto 0 0; border-radius: 3px; background: var(--vision); }
  .balken b { position: absolute; top: -3px; width: 2px; height: 12px; background: var(--text); }
  .aufklapp td { padding-top: 0; }
  details summary { cursor: pointer; color: var(--stahl); font-size: .85rem; }
  .teile { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px 24px; padding: 10px 0 4px; }
  .teile ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 4px; }
  .teile li span:first-child { display: inline-block; min-width: 72px; }
  abbr { text-decoration: underline dotted; cursor: help; }
  .text { max-width: 72ch; }
  .text ul { padding-left: 20px; display: grid; gap: 8px; color: var(--stahl); }
  code { font: .85rem var(--mono); overflow-wrap: anywhere; }
</style>
