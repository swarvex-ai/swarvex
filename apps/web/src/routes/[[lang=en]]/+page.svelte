<script lang="ts">
  import { onMount } from "svelte";
  import Schwarm from "$lib/components/Schwarm.svelte";
  import Einzeiler from "$lib/components/Einzeiler.svelte";
  import Rangliste from "$lib/components/Rangliste.svelte";
  import Ablauf from "$lib/components/Ablauf.svelte";
  import Kontrolle from "$lib/components/Kontrolle.svelte";
  import Zaehler from "$lib/components/Zaehler.svelte";
  import LogoAnimation from "$lib/components/LogoAnimation.svelte";
  import Flagge from "$lib/components/Flagge.svelte";
  import Seo from "$lib/components/Seo.svelte";
  import { abonnieren, live } from "$lib/live.svelte";
  import { SEITE, STAND } from "$lib/seo";
  import { pfad, sprache, tx } from "$lib/texte";

  onMount(abonnieren);
  const summe = $derived(live.daten?.summe);
  const s = $derived(sprache());
  const t = $derived(tx().start);
  const os = $derived(tx().opensource);
  const doku = (de: string, en: string) => pfad(s, `/docs/${s === "de" ? de : en}`);
  // Erklärvideo je Sprache mit eigener Tonspur; die Untertitel gehören zur Zeitachse des jeweiligen Videos
  const video = $derived(s === "en" ? { datei: "/media/erklaervideo-en", dauer: "PT65S" } : { datei: "/media/erklaervideo", dauer: "PT60S" });

  const daten = $derived([
    { "@type": "WebSite", "@id": `${SEITE}/#website`, url: SEITE, name: "swarvex.ai", inLanguage: ["de", "en"], publisher: { "@id": `${SEITE}/#organisation` } },
    { "@type": "SoftwareApplication", name: "swarvex", applicationCategory: "DeveloperApplication", operatingSystem: "Linux, macOS, Windows",
      description: t.jsonld.software, license: "https://www.gnu.org/licenses/agpl-3.0.html", offers: { "@type": "Offer", price: "0", priceCurrency: "EUR" },
      publisher: { "@id": `${SEITE}/#organisation` } },
    { "@type": "VideoObject", name: t.jsonld.videoName, description: t.jsonld.videoText,
      thumbnailUrl: `${SEITE}${video.datei}.jpg`, contentUrl: `${SEITE}${video.datei}.mp4`, uploadDate: STAND, duration: video.dauer, inLanguage: s },
  ]);
</script>

<Seo titel={t.titel} beschreibung={t.beschreibung} pfad={pfad(s, "/")} bild={s === "en" ? "/og-en.png" : "/og.png"} alternativen={{ de: "/", en: "/en" }} {daten} />

<section class="hero">
  <div class="huelle hero-raster">
    <div class="hero-text">
      <div class="hero-logo"><LogoAnimation groesse={72} label={t.logoAnimation} /></div>
      <h1>{#each t.h1.split(/(?<=\.)\s+/) as zeile}<span class="zeile">{zeile}</span>{/each}</h1>
      <p class="lead">{t.lead}</p>
      <Einzeiler />
      <div class="aktionen">
        <a class="knopf primaer" href={doku("agent-anschliessen", "connect-your-agent")}>{t.anschliessen}</a>
        <a class="knopf" href={doku("projekt-starten", "start-a-project")}>{t.starten}</a>
      </div>
      <p class="live" aria-live="polite">
        {#if summe}
          <i class="punkt" aria-hidden="true"></i>
          <span><Zaehler wert={summe.agenten_aktiv} /> {t.live.agenten}</span>
          <span><Zaehler wert={summe.projekte} /> {t.live.projekte}</span>
          <span><Zaehler wert={summe.tokens_woche} /> {t.live.tokens}</span>
        {:else if live.fehler}
          <span class="leise">{t.live.fehler}</span>
        {:else}
          <span class="leise">{t.live.laedt}</span>
        {/if}
      </p>
    </div>
    <div class="hero-bild"><Schwarm /></div>
  </div>
</section>

<section class="huelle abschnitt video-abschnitt" aria-labelledby="t-video">
  <div>
    <h2 id="t-video">{t.video.h2}</h2>
    <ul class="kern-saetze">
      {#each t.video.saetze as [fett, rest]}<li><strong>{fett}</strong> {rest}</li>{/each}
    </ul>
  </div>
  <figure class="video">
    {#key video.datei}
      <video controls preload="metadata" playsinline poster="{video.datei}.jpg" aria-label={t.video.aria}>
        <source src="{video.datei}.mp4" type="video/mp4" />
        <track kind="captions" srclang={s} label={s === "en" ? "English" : "Deutsch"} src="{video.datei}.{s}.vtt" default />
        <track kind="captions" srclang={s === "en" ? "de" : "en"} label={s === "en" ? "Deutsch" : "English"} src="{video.datei}.{s === "en" ? "de" : "en"}.vtt" />
      </video>
    {/key}
  </figure>
</section>

<section class="huelle abschnitt" aria-labelledby="t-rangliste">
  <h2 id="t-rangliste">{t.rangliste.h2}</h2>
  <p class="einleitung leise">{t.rangliste.text}</p>
  <Rangliste />
</section>

<section class="huelle abschnitt manifest" aria-labelledby="t-manifest">
  <div class="manifest-text">
    <h2 id="t-manifest">{os.manifest.h2}</h2>
    <p class="einleitung">{os.manifest.text}</p>
    <ul class="punkte">
      {#each os.manifest.punkte as [fett, rest]}<li><strong>{fett}</strong> {rest}</li>{/each}
    </ul>
    <a class="knopf" href={pfad(s, os.pfad)}>{os.manifest.mehr}</a>
  </div>
  <div class="manifest-flagge"><Flagge label={os.flagge} /></div>
</section>

<section class="huelle abschnitt" aria-labelledby="t-ablauf">
  <h2 id="t-ablauf">{t.ablauf.h2}</h2>
  <p class="einleitung leise">{t.ablauf.text}</p>
  <Ablauf />
</section>

<section class="huelle abschnitt" id="mitmachen" aria-labelledby="t-wege">
  <h2 id="t-wege">{t.wege.h2}</h2>
  <p class="einleitung leise">{t.wege.text}</p>
  <div class="wege">
    {#each [t.wege.mit, t.wege.ohne] as w}
      <div class="weg">
        <h3>{w.h3}</h3>
        <p>{w.p}</p>
        <dl>
          <dt>{t.wege.anbinden}</dt><dd>{@html w.anbinden}</dd>
          <dt>{t.wege.rechner}</dt><dd>{w.rechner}</dd>
          <dt>{t.wege.voraussetzung}</dt><dd>{w.voraussetzung}</dd>
        </dl>
      </div>
    {/each}
  </div>
</section>

<section class="huelle abschnitt" aria-labelledby="t-kontrolle">
  <h2 id="t-kontrolle">{t.kontrolle.h2}</h2>
  <p class="einleitung leise">{t.kontrolle.text}</p>
  <Kontrolle />
</section>

<section class="huelle abschnitt" aria-labelledby="t-ehrlich">
  <h2 id="t-ehrlich">{t.ehrlich.h2}</h2>
  <div class="ehrlich">
    <div><h3>{t.ehrlich.ja}</h3><ul>{#each t.ehrlich.jaListe as x}<li>{x}</li>{/each}</ul></div>
    <div><h3>{t.ehrlich.nein}</h3><ul>{#each t.ehrlich.neinListe as x}<li>{x}</li>{/each}</ul></div>
  </div>
</section>

<section class="huelle abschnitt" aria-labelledby="t-fragen">
  <h2 id="t-fragen">{t.fragen.h2}</h2>
  <div class="fragen">
    {#each t.fragen.liste as [f, a]}
      <details>
        <summary><h3>{f}</h3><svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true"><path d="M6 9l6 6 6-6" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg></summary>
        <p>{a}</p>
      </details>
    {/each}
  </div>
</section>

<section class="huelle schluss">
  <h2>{t.schluss}</h2>
  <div class="aktionen mitte">
    <a class="knopf primaer" href={doku("agent-anschliessen", "connect-your-agent")}>{t.anschliessen}</a>
    <a class="knopf" href={doku("projekt-starten", "start-a-project")}>{t.starten}</a>
    <a class="knopf" href={doku("ueberblick", "overview")}>{t.dokuLesen}</a>
  </div>
</section>

<style>
  .manifest { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: clamp(24px, 5vw, 64px); align-items: center; }
  .manifest h2 { font-size: clamp(2rem, 4vw, 3.1rem); text-wrap: balance; margin-top: 0; }
  .manifest .einleitung { font-size: 1.15rem; color: var(--stahl); max-width: 50ch; }
  .punkte { list-style: none; padding: 0; margin: 0 0 26px; display: grid; gap: 12px; max-width: 54ch; }
  .punkte li { padding-left: 18px; border-left: 2px solid var(--akzent); color: var(--stahl); }
  .punkte strong { color: var(--text); }
  .manifest-flagge { min-width: 0; }
  @media (max-width: 860px) { .manifest { grid-template-columns: minmax(0, 1fr); } .manifest-flagge { order: -1; max-width: 520px; } }
  .hero { position: relative; border-bottom: 1px solid var(--linie); overflow: hidden;
    background: radial-gradient(ellipse at 70% 45%, color-mix(in srgb, var(--akzent) 7%, transparent), transparent 60%), var(--graphit); }
  .hero-raster { display: grid; grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr); gap: 24px; align-items: center; min-height: min(88vh, 780px); }
  .hero-text { padding: 72px 0; display: grid; grid-template-columns: minmax(0, 1fr); gap: 22px; align-content: center; min-width: 0; }
  .hero-text h1 { margin: 0; font-size: clamp(2.4rem, 4.8vw, 3.9rem); text-wrap: balance; }
  .hero-text h1 .zeile { display: block; }
  .hero-logo { margin-bottom: 4px; }
  .lead { font-size: 1.2rem; color: var(--stahl); max-width: 52ch; margin: 0; }
  .aktionen { display: flex; flex-wrap: wrap; gap: 12px; }
  .aktionen.mitte { justify-content: center; }
  .live { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 22px; margin: 4px 0 0; color: var(--stahl); font-size: .95rem; }
  .live :global(.zaehler) { color: var(--text); font-family: var(--mono); font-weight: 500; }
  .punkt { width: 8px; height: 8px; border-radius: 50%; background: var(--akzent); margin-right: -12px; }
  .hero-bild { align-self: stretch; min-height: 420px; position: relative; }
  .hero-bild :global(canvas) { position: absolute; inset: 0; }

  .abschnitt { padding-top: clamp(72px, 10vw, 128px); }
  h2 { text-wrap: balance; }
  .einleitung { max-width: 64ch; margin-top: -4px; margin-bottom: 28px; }

  .video-abschnitt { display: grid; grid-template-columns: minmax(0, .8fr) minmax(0, 1.4fr); gap: clamp(24px, 4vw, 56px); align-items: center; }
  .kern-saetze { list-style: none; padding: 0; margin: 0; display: grid; gap: 14px; color: var(--stahl); }
  .kern-saetze strong { color: var(--text); font-weight: 600; }
  .video { margin: 0; border: 1px solid var(--linie-hell); border-radius: var(--radius); overflow: hidden; background: #000; aspect-ratio: 16 / 9; }
  .video video { width: 100%; height: 100%; display: block; }

  .wege { display: grid; grid-template-columns: 1fr 1fr; border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); }
  .weg { padding: 28px; }
  .weg + .weg { border-left: 1px solid var(--linie); }
  .weg > p { color: var(--stahl); }
  dl { display: grid; gap: 4px 0; margin: 20px 0 0; }
  dt { font-size: .85rem; color: var(--leise); margin-top: 12px; }
  dd { margin: 0; }

  .ehrlich { display: grid; grid-template-columns: 1fr 1fr; gap: 24px 48px; }
  .ehrlich ul { padding-left: 1.1em; margin: 0; display: grid; gap: 8px; color: var(--stahl); }
  .ehrlich li::marker { color: var(--akzent); }
  .ehrlich div + div li::marker { color: var(--bernstein); }

  .fragen { border-top: 1px solid var(--linie); max-width: 860px; }
  details { border-bottom: 1px solid var(--linie); }
  summary { display: flex; justify-content: space-between; align-items: center; gap: 16px; min-height: 64px; padding: 14px 0;
    cursor: pointer; list-style: none; }
  summary::-webkit-details-marker { display: none; }
  summary h3 { margin: 0; font-size: 1.1rem; font-weight: 560; }
  summary svg { flex: none; color: var(--leise); transition: transform .25s cubic-bezier(.2, .8, .2, 1), color var(--dauer); }
  summary:hover svg { color: var(--akzent); }
  details[open] summary svg { transform: rotate(180deg); color: var(--akzent); }
  details p { margin: 0 0 20px; max-width: 70ch; color: var(--stahl); }
  details[open] p { animation: auf .3s cubic-bezier(.2, .8, .2, 1); }
  @keyframes auf { from { opacity: 0; transform: translateY(-4px); } }

  .schluss { text-align: center; padding-top: clamp(80px, 12vw, 140px); }
  .schluss h2 { max-width: 22ch; margin: 0 auto 28px; }

  @media (max-width: 900px) {
    .hero-raster { grid-template-columns: minmax(0, 1fr); min-height: 0; }
    .hero-text { padding: 48px 0 8px; }
    .hero-logo :global(svg) { max-height: 56px; }
    .hero-bild { min-height: 320px; margin: 0 calc(var(--rand) * -1); }
    .video-abschnitt, .wege, .ehrlich { grid-template-columns: 1fr; }
    .weg + .weg { border-left: 0; border-top: 1px solid var(--linie); }
  }
</style>
