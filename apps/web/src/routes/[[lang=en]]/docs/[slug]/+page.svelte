<script lang="ts">
  import { onMount } from "svelte";
  import Seo from "$lib/components/Seo.svelte";
  import { SEITE, STAND } from "$lib/seo";
  import { pfad, sprache, tx } from "$lib/texte";
  const t = $derived(tx().doku);
  const s = $derived(sprache());
  let { data } = $props();
  let aktiv = $state("");

  onMount(() => {
    // Kopieren-Knöpfe an Codeblöcken
    const klick = async (e: MouseEvent) => {
      const k = (e.target as HTMLElement).closest("[data-kopieren]");
      if (!k) return;
      const code = k.parentElement?.querySelector("pre")?.innerText ?? "";
      try { await navigator.clipboard.writeText(code); k.textContent = t.kopiert; setTimeout(() => (k.textContent = t.kopieren), 1500); } catch {}
    };
    document.addEventListener("click", klick);
    return () => document.removeEventListener("click", klick);
  });

  // Inhaltsverzeichnis: markiert den Abschnitt, der gerade gelesen wird
  $effect(() => {
    void data.slug;
    const ziele = data.ueberschriften.map((u) => document.getElementById(u.id)).filter(Boolean) as HTMLElement[];
    const io = new IntersectionObserver((e) => { const s = e.find((x) => x.isIntersecting); if (s) aktiv = s.target.id; }, { rootMargin: "-80px 0px -70% 0px" });
    ziele.forEach((z) => io.observe(z));
    return () => io.disconnect();
  });
</script>

<Seo
  titel="{data.titel}: {t.titelZusatz}"
  beschreibung="{data.beschreibung} {t.beschreibungZusatz}"
  pfad={pfad(s, `/docs/${data.slug}`)}
  alternativen={data.alternativen}
  typ="article"
  daten={[
    { "@type": "TechArticle", headline: data.titel, description: data.beschreibung, inLanguage: s, dateModified: STAND,
      url: SEITE + pfad(s, `/docs/${data.slug}`), publisher: { "@id": `${SEITE}/#organisation` } },
    { "@type": "BreadcrumbList", itemListElement: [
      { "@type": "ListItem", position: 1, name: t.startseite, item: SEITE + pfad(s, "/docs") },
      { "@type": "ListItem", position: 2, name: data.gruppe },
      { "@type": "ListItem", position: 3, name: data.titel, item: SEITE + pfad(s, `/docs/${data.slug}`) } ] },
  ]}
/>

<div class="seite">
  <article>
    <p class="pfad leise">{data.gruppe}</p>
    <h1>{data.titel}</h1>
    <p class="beschreibung">{data.beschreibung}</p>
    {#if data.eingebunden}<p class="quelle leise">{@html t.eingebunden(data.eingebunden)}</p>{/if}
    <div class="prosa">{@html data.html}</div>
    <footer>
      <a class="leise bearbeiten" href={data.bearbeiten} rel="noopener">{t.bearbeiten}</a>
      <nav class="blaettern" aria-label={t.blaettern}>
        {#if data.zurueck}<a href={pfad(s, `/docs/${data.zurueck.slug}`)}><span class="leise">{t.zurueck}</span>{data.zurueck.titel}</a>{:else}<span></span>{/if}
        {#if data.weiter}<a class="weiter" href={pfad(s, `/docs/${data.weiter.slug}`)}><span class="leise">{t.weiter}</span>{data.weiter.titel}</a>{/if}
      </nav>
    </footer>
  </article>
  {#if data.ueberschriften.length > 1}
    <aside class="toc" aria-label={t.aufSeite}>
      <p>{t.aufSeite}</p>
      <ul>
        {#each data.ueberschriften as u}
          <li class:tief={u.ebene === 3}><a href="#{u.id}" aria-current={aktiv === u.id ? "location" : undefined}>{u.text}</a></li>
        {/each}
      </ul>
    </aside>
  {/if}
</div>

<style>
  .seite { display: grid; grid-template-columns: minmax(0, 1fr) 200px; gap: 48px; }
  article { min-width: 0; max-width: 780px; }
  .pfad { margin: 0 0 6px; font-size: .88rem; }
  h1 { font-size: clamp(2rem, 4vw, 2.6rem); }
  .beschreibung { font-size: 1.15rem; color: var(--stahl); margin-bottom: 24px; }
  .quelle { font-size: .85rem; border-left: 2px solid var(--linie-hell); padding-left: 10px; }
  .toc { position: sticky; top: 88px; align-self: start; font-size: .88rem; }
  .toc p { margin: 0 0 8px; font-weight: 600; }
  .toc ul { list-style: none; padding: 0; margin: 0; display: grid; gap: 6px; }
  .toc li.tief { padding-left: 12px; }
  .toc a { color: var(--leise); text-decoration: none; }
  .toc a:hover, .toc a[aria-current] { color: var(--akzent); }
  footer { margin-top: 56px; padding-top: 20px; border-top: 1px solid var(--linie); display: grid; gap: 20px; }
  .bearbeiten { font-size: .88rem; text-decoration: none; }
  .bearbeiten:hover { color: var(--akzent); }
  .blaettern { display: flex; justify-content: space-between; gap: 16px; }
  .blaettern a { display: grid; gap: 2px; padding: 12px 16px; min-width: 40%; border: 1px solid var(--linie); border-radius: 8px; color: var(--text); text-decoration: none; }
  .blaettern a:hover { border-color: var(--akzent); }
  .blaettern .weiter { text-align: right; }
  .blaettern span { font-size: .8rem; }

  /* Prosa aus Markdown */
  .prosa :global(h2) { margin-top: 2.2em; font-size: 1.55rem; }
  .prosa :global(h3) { margin-top: 1.8em; }
  .prosa :global(h2), .prosa :global(h3) { position: relative; scroll-margin-top: 90px; }
  .prosa :global(.anker) { position: absolute; left: -1.1em; color: var(--leise); text-decoration: none; opacity: 0; transition: opacity var(--dauer); }
  .prosa :global(h2:hover .anker), .prosa :global(h3:hover .anker), .prosa :global(.anker:focus) { opacity: 1; }
  .prosa :global(p), .prosa :global(li) { color: color-mix(in srgb, var(--text) 88%, var(--stahl)); }
  .prosa :global(ul), .prosa :global(ol) { padding-left: 1.3em; }
  .prosa :global(li) { margin: .25em 0; }
  .prosa :global(li::marker) { color: var(--leise); }
  .prosa :global(:not(pre) > code) { padding: .1em .35em; border-radius: 4px; background: var(--graphit-3); border: 1px solid var(--linie); font-size: .86em; }
  .prosa :global(.code) { position: relative; margin: 1.2em 0; }
  .prosa :global(pre) { margin: 0; padding: 16px 18px; border: 1px solid var(--linie); border-radius: 8px; overflow-x: auto; font-size: .88rem; line-height: 1.55;
    background: var(--graphit-2) !important; }
  .prosa :global(pre span) { color: var(--shiki-dark); }
  :global(:root[data-thema="hell"]) .prosa :global(pre span) { color: var(--shiki-light); }
  @media (prefers-color-scheme: light) { .prosa :global(pre span) { color: var(--shiki-light); } }
  .prosa :global(.kopieren) { position: absolute; top: 8px; right: 8px; min-height: 30px; padding: 0 10px; border: 1px solid var(--linie-hell); border-radius: 5px;
    background: var(--graphit-3); color: var(--leise); font: 500 .78rem var(--schrift); cursor: pointer; opacity: 0; transition: opacity var(--dauer); }
  .prosa :global(.code:hover .kopieren), .prosa :global(.kopieren:focus) { opacity: 1; }
  .prosa :global(.tabelle) { overflow-x: auto; margin: 1.2em 0; border: 1px solid var(--linie); border-radius: 8px; }
  .prosa :global(table) { width: 100%; border-collapse: collapse; font-size: .93rem; }
  .prosa :global(th), .prosa :global(td) { padding: 9px 12px; text-align: left; vertical-align: top; border-bottom: 1px solid var(--linie); }
  .prosa :global(th) { background: var(--graphit-2); font-weight: 600; color: var(--stahl); }
  .prosa :global(tr:last-child td) { border-bottom: 0; }
  .prosa :global(.hinweis) { margin: 1.4em 0; padding: 12px 16px; border: 1px solid color-mix(in srgb, var(--akzent) 35%, var(--linie)); border-radius: 6px; background: color-mix(in srgb, var(--akzent) 5%, var(--graphit-2)); }
  .prosa :global(.hinweis.achtung), .prosa :global(.hinweis.wichtig) { border-color: color-mix(in srgb, var(--bernstein) 40%, var(--linie)); }
  .prosa :global(.hinweis p:last-child) { margin: 0; }
  .prosa :global(hr) { border: 0; border-top: 1px solid var(--linie); margin: 2em 0; }

  @media (max-width: 1100px) { .seite { grid-template-columns: 1fr; } .toc { display: none; } }
</style>
