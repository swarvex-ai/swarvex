<script lang="ts">
  import Seo from "$lib/components/Seo.svelte";
  import { sprache } from "$lib/texte";
  let { data } = $props();
</script>

<Seo titel={`${data.titel} – swarvex`} beschreibung={data.beschreibung} pfad={data.alternativen[sprache()]}
     alternativen={data.alternativen} />

<article class="huelle recht">
  <h1>{data.titel}</h1>
  <div class="text">{@html data.html}</div>
</article>

<style>
  .recht { padding-top: clamp(48px, 7vw, 88px); max-width: 860px; }
  h1 { font-size: clamp(2.2rem, 4.5vw, 3.2rem); margin: 0 0 24px; hyphens: auto; overflow-wrap: break-word; }
  .text { color: var(--stahl); line-height: 1.65; }
  .text :global(h2) { color: var(--text); font-size: 1.3rem; margin: 40px 0 10px; }
  .text :global(p), .text :global(ul) { margin: 0 0 14px; max-width: 72ch; }
  .text :global(li) { margin-bottom: 6px; }
  .text :global(strong) { color: var(--text); }
  .text :global(a) { color: var(--akzent); }
  .text :global(code) { font: .88em var(--mono); background: var(--metall); border: 1px solid var(--linie); border-radius: 4px; padding: 1px 5px; }
  .text :global(table) { width: 100%; border-collapse: collapse; margin: 8px 0 18px; font-size: .93rem; display: block; overflow-x: auto; }
  .text :global(th), .text :global(td) { text-align: left; padding: 8px 12px; border-bottom: 1px solid var(--linie); vertical-align: top; }
  .text :global(th) { color: var(--leise); font-weight: 500; font-size: .85rem; }
  .text :global(mark.fehlt) { background: color-mix(in srgb, var(--bernstein) 25%, transparent); color: var(--text); padding: 0 4px; border-radius: 3px; font: .85em var(--mono); }
</style>
