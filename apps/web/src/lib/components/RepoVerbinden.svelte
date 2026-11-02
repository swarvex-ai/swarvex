<script lang="ts">
  // Repo eines Projekts über die swarvex GitHub App verbinden (apps/api/src/github.ts): App installieren, dann prüfen und verbinden.
  // Im Assistenten „Projekt starten“ und auf der Kontoseite; nach der Installation schickt GitHub zurück (?app=installed).
  import { onMount } from "svelte";
  import { sprache, tx } from "$lib/texte";

  let { slug, repo = "", von = "starten", onverbunden }: { slug: string; repo?: string; von?: "starten" | "konto"; onverbunden?: (url: string) => void } = $props();
  const s = $derived(sprache());
  const t = $derived(tx().starten.repo);
  const SPEICHER = "swarvex-repo";

  let app = $state<boolean | null>(null);
  let adresse = $state("");
  let arbeitet = $state(false);
  let meldung = $state("");
  let verbunden = $state("");

  onMount(async () => {
    adresse = repo;
    try { const r = await fetch("/api/account/github"); app = r.ok && (await r.json()).app === true; } catch { app = false; }
    // Zurück von GitHub: die vorher eingegebene Adresse gleich verbinden
    let gemerkt: { slug?: string; adresse?: string } = {};
    try { gemerkt = JSON.parse(sessionStorage.getItem(SPEICHER) ?? "{}"); } catch { /* ohne Speicher */ }
    if (app && new URL(location.href).searchParams.get("app") === "installed" && gemerkt.slug === slug) {
      adresse = gemerkt.adresse || adresse;
      if (adresse) await verbinden();
    }
  });

  function merken() { try { sessionStorage.setItem(SPEICHER, JSON.stringify({ slug, adresse: adresse.trim() })); } catch { /* ohne Speicher */ } }

  async function verbinden() {
    arbeitet = true; meldung = "";
    try {
      const r = await fetch(`/api/account/projects/${slug}/repo`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ repo_url: adresse.trim() }) });
      const j = await r.json().catch(() => ({}));
      if (r.ok) { verbunden = j.repo_url; onverbunden?.(j.repo_url); try { sessionStorage.removeItem(SPEICHER); } catch { /* egal */ } }
      else meldung = t.fehler[j.error] ?? j.error ?? t.sonst;
    } catch { meldung = t.sonst; } finally { arbeitet = false; }
  }
</script>

{#if verbunden}
  <p class="ok">✓ {t.verbunden} <a href={verbunden} rel="noopener">{verbunden.replace("https://", "")}</a></p>
{:else if app === false}
  <p class="leise">{t.ohneApp}</p>
{:else if app}
  <div class="repo">
    <p class="leise">{t.text}</p>
    <label>{t.adresse}<input type="url" maxlength="200" bind:value={adresse} oninput={merken} placeholder="https://github.com/…" /></label>
    <div class="knoepfe">
      <a class="knopf" href={`/api/account/github/install?lang=${s}&from=${von}`} onclick={merken} data-sveltekit-reload>{t.installieren}</a>
      <button class="knopf visionaer" disabled={arbeitet || !adresse.trim()} onclick={verbinden}>{arbeitet ? t.verbindet : t.verbinden}</button>
    </div>
    <p class="fehler" role="alert">{meldung}</p>
    <p class="leise klein">{t.keinRepo}</p>
  </div>
{/if}

<style>
  .repo { display: grid; gap: 12px; }
  .repo p { margin: 0; }
  label { display: grid; gap: 4px; font-size: .92rem; }
  input { min-height: 40px; padding: 8px 10px; border-radius: 6px; border: 1px solid var(--linie-hell); background: var(--graphit-2); color: var(--text);
          font: 400 .95rem var(--schrift); width: 100%; box-sizing: border-box; }
  .knoepfe { display: flex; flex-wrap: wrap; gap: 10px; }
  button.knopf[disabled] { opacity: .5; cursor: not-allowed; }
  .ok { color: var(--akzent); margin: 0; overflow-wrap: anywhere; }
  .fehler { color: var(--rot); }
  .fehler:empty { display: none; }
  .klein { font-size: .88rem; }
</style>
