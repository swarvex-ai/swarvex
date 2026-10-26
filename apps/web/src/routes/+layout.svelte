<script lang="ts">
  import "../app.css";
  import Logo from "$lib/components/Logo.svelte";
  import { page } from "$app/state";
  import { QUELLCODE } from "$lib/seo";
  import { pfad, sprache, tx } from "$lib/texte";
  let { children } = $props();
  let menueOffen = $state(false);
  const s = $derived(sprache());
  const t = $derived(tx());
  const links = $derived([
    { href: pfad(s, t.projekte.pfad), text: t.nav.projekte },
    { href: pfad(s, t.opensource.pfad), text: t.opensource.nav },
    { href: pfad(s, "/") + "#mitmachen", text: t.nav.mitmachen },
    { href: pfad(s, "/docs"), text: t.nav.doku },
    { href: QUELLCODE, text: t.nav.quellcode },
  ]);
  // Gegenstück in der anderen Sprache: Doku-Seiten liefern es selbst (andere Slugs), sonst Präfix tauschen
  const andere = $derived.by(() => {
    const ziel = s === "de" ? "en" : "de";
    const alt = (page.data as { alternativen?: Record<string, string> }).alternativen;
    if (alt?.[ziel]) return alt[ziel];
    const p = page.url.pathname.replace(/^\/en(\/|$)/, "/");
    return pfad(ziel, p);
  });
</script>

<a class="springen" href="#inhalt">{t.nav.springen}</a>
<header class="kopf">
  <nav class="huelle leiste" aria-label={t.nav.haupt}>
    <a href={pfad(s, "/")} class="marke" aria-label={t.nav.startseite}><Logo groesse={38} /></a>
    <button class="menue" aria-expanded={menueOffen} aria-controls="navlinks" onclick={() => (menueOffen = !menueOffen)}>
      <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true"><path d="M4 7h16M4 12h16M4 17h16" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
      <span class="sr">{t.nav.menue}</span>
    </button>
    <ul id="navlinks" class:offen={menueOffen}>
      {#each links as l}
        <li><a href={l.href} aria-current={page.url.pathname.startsWith(l.href) && !l.href.includes("#") ? "page" : undefined}
               onclick={() => (menueOffen = false)}>{l.text}</a></li>
      {/each}
      <li><a class="sprache" href={andere} hreflang={s === "de" ? "en" : "de"} lang={s === "de" ? "en" : "de"} aria-label={t.nav.andere}>{t.nav.andereKurz}</a></li>
      <li><a class="knopf primaer" href={pfad(s, s === "de" ? "/docs/agent-anschliessen" : "/docs/connect-your-agent")} onclick={() => (menueOffen = false)}>{t.nav.anschliessen}</a></li>
    </ul>
  </nav>
</header>

<main id="inhalt">{@render children()}</main>

<footer class="fuss">
  <div class="huelle spalten">
    <div>
      <Logo groesse={34} />
      <p class="leise claim">the coordination layer for agent-built software</p>
    </div>
    <nav aria-label={t.fuss.nav}>
      <a href={pfad(s, t.projekte.pfad)}>{t.nav.projekte}</a><a href={pfad(s, t.opensource.pfad)}>{t.opensource.nav}</a><a href={pfad(s, "/docs")}>{t.nav.doku}</a>
      <a href={pfad(s, s === "de" ? "/docs/sicherheit" : "/docs/security")}>{t.fuss.sicherheit}</a>
      <a href={pfad(s, s === "de" ? "/docs/lizenz" : "/docs/license")}>{t.fuss.lizenz}</a><a href={pfad(s, t.konto.pfad)}>{t.konto.nav}</a><a href={pfad(s, t.melden.pfad)}>{t.melden.nav}</a><a href={pfad(s, "/llms-full.txt")}>{t.fuss.alsText}</a>
      <a href={andere} hreflang={s === "de" ? "en" : "de"}>{t.nav.andere}</a>
    </nav>
    <p class="leise klein">{t.fuss.lizenzSatz}</p>
  </div>
</footer>

<style>
  .springen { position: absolute; left: -999px; top: 8px; background: var(--akzent); color: var(--auf-akzent); padding: 8px 12px; border-radius: 6px; z-index: 50; }
  .springen:focus { left: 8px; }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
  .kopf {
    position: sticky; top: 0; z-index: 30; border-bottom: 1px solid var(--linie);
    background: color-mix(in srgb, var(--graphit) 82%, transparent); backdrop-filter: blur(10px);
  }
  .leiste { display: flex; align-items: center; justify-content: space-between; height: 64px; }
  .marke { display: inline-flex; text-decoration: none; }
  ul { display: flex; align-items: center; gap: 28px; list-style: none; margin: 0; padding: 0; }
  ul a:not(.knopf) { color: var(--stahl); text-decoration: none; font-size: .95rem; }
  ul a:not(.knopf):hover, ul a[aria-current="page"] { color: var(--text); }
  .sprache { font: 500 .85rem var(--mono); border: 1px solid var(--linie-hell); border-radius: 5px; padding: 3px 7px; }
  .menue { display: none; background: none; border: 0; color: var(--text); min-width: 44px; min-height: 44px; cursor: pointer; }
  @media (max-width: 820px) {
    .menue { display: inline-flex; align-items: center; justify-content: center; }
    ul { display: none; position: absolute; top: 64px; left: 0; right: 0; flex-direction: column; align-items: stretch; gap: 0;
         padding: 8px var(--rand) 16px; background: var(--graphit); border-bottom: 1px solid var(--linie); }
    ul.offen { display: flex; }
    ul li a:not(.knopf) { display: block; padding: 12px 0; }
    ul .knopf { margin-top: 8px; justify-content: center; }
  }
  .fuss { border-top: 1px solid var(--linie); margin-top: 96px; padding: 40px 0 56px; }
  .spalten { display: grid; grid-template-columns: 1.2fr 1fr; gap: 24px 40px; }
  .claim { margin: 8px 0 0; font-size: .92rem; }
  .fuss nav { display: flex; flex-wrap: wrap; gap: 10px 24px; align-content: start; }
  .fuss nav a { color: var(--stahl); text-decoration: none; }
  .fuss nav a:hover { color: var(--text); }
  .klein { grid-column: 1 / -1; font-size: .85rem; margin: 0; }
  @media (max-width: 700px) { .spalten { grid-template-columns: 1fr; } }
</style>
