<script lang="ts">
  import { SEITE, organisation } from "$lib/seo";
  let { titel, beschreibung, pfad, bild = "/og.png", typ = "website", daten = [], alternativen = {}, noindex = false }:
    { titel: string; beschreibung: string; pfad: string; bild?: string; typ?: string; daten?: object[]; alternativen?: Record<string, string>; noindex?: boolean } = $props();
  const englisch = $derived(pfad === "/en" || pfad.startsWith("/en/"));
  const url = $derived(SEITE + pfad);
  const jsonld = $derived(JSON.stringify({ "@context": "https://schema.org", "@graph": [organisation, ...daten] }).replace(/</g, "\\u003c"));
</script>

<svelte:head>
  <title>{titel}</title>
  <meta name="description" content={beschreibung} />
  <link rel="canonical" href={url} />
  {#if noindex}<meta name="robots" content="noindex" />{/if}
  <meta property="og:type" content={typ} />
  <meta property="og:site_name" content="swarvex.ai" />
  <meta property="og:locale" content={englisch ? "en_US" : "de_DE"} />
  {#each Object.entries(alternativen) as [sp, p]}<link rel="alternate" hreflang={sp} href={SEITE + p} />{/each}
  {#if alternativen.en}<link rel="alternate" hreflang="x-default" href={SEITE + alternativen.en} />{/if}
  <meta property="og:title" content={titel} />
  <meta property="og:description" content={beschreibung} />
  <meta property="og:url" content={url} />
  <meta property="og:image" content={SEITE + bild} />
  <meta property="og:image:width" content="1200" />
  <meta property="og:image:height" content="630" />
  <meta name="twitter:card" content="summary_large_image" />
  <meta name="twitter:title" content={titel} />
  <meta name="twitter:description" content={beschreibung} />
  <meta name="twitter:image" content={SEITE + bild} />
  {@html `<script type="application/ld+json">${jsonld}</script>`}
</svelte:head>
