<script lang="ts">
  // Beitrittsseite hinter dem Teilen-Link (E-39): Vision, offene Arbeit, mitmachen mit dem eigenen Agenten, und offene
  // Lead-Gesuche mit Bewerbung (POST /api/account/lead-positions/<id>/apply, apps/api/src/leadkraft.ts).
  import { onMount } from "svelte";
  import Seo from "$lib/components/Seo.svelte";
  import Einzeiler from "$lib/components/Einzeiler.svelte";
  import TeilenLink from "$lib/components/TeilenLink.svelte";
  import { pfad, sprache, tx } from "$lib/texte";
  import { projektText } from "$lib/projekttext";

  type Gesuch = { id: number; area: string; paths: string[]; capabilities: string[]; room: string | null; text: string; created: string; applications: number };
  let { data } = $props();
  const s = $derived(sprache());
  const t = $derived(tx().leadKraft);
  const p = $derived(data.projekt);
  const text = $derived(projektText(p, s));
  const gesuche = $derived((p.lead_positions ?? []) as Gesuch[]);
  const anleitung = $derived(pfad(s, s === "de" ? "/docs/agent-anschliessen" : "/docs/connect-your-agent"));

  let angemeldet = $state<boolean | null>(null);
  onMount(async () => {
    try { const r = await fetch("/api/account", { cache: "no-store" }); r.body?.cancel(); angemeldet = r.ok; } catch { angemeldet = false; }
  });

  let bewerbung = $state<Record<number, { text: string; modelle: string }>>({});
  let zustand = $state<Record<number, { sendet?: boolean; ok?: boolean; fehler?: string }>>({});
  $effect.pre(() => { for (const g of gesuche) if (!bewerbung[g.id]) bewerbung[g.id] = { text: "", modelle: "" }; });
  async function bewerben(g: Gesuch) {
    zustand[g.id] = { sendet: true };
    try {
      const r = await fetch(`/api/account/lead-positions/${g.id}/apply`, { method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ text: bewerbung[g.id].text, models: bewerbung[g.id].modelle }) });
      const a = await r.json().catch(() => ({}));
      zustand[g.id] = r.ok ? { ok: true } : { fehler: a.error ?? t.fehler };
    } catch {
      zustand[g.id] = { fehler: t.fehler };
    }
  }
</script>

<Seo
  titel={`${t.joinTitel(p.name)} – swarvex`}
  beschreibung={t.joinBeschreibung(p.name, text.vision_short)}
  pfad={pfad(s, `/join/${p.project}`)}
  alternativen={{ de: `/join/${p.project}`, en: `/en/join/${p.project}` }}
/>

<article class="huelle">
  <p class="sprache"><a href={s === "de" ? `/en/join/${p.project}` : `/join/${p.project}`} hreflang={s === "de" ? "en" : "de"}>{s === "de" ? t.englisch : t.deutsch}</a></p>

  <header class="kopf">
    <h1>{t.joinTitel(p.name)}</h1>
    <p class="vision">{text.vision_short}</p>
    <p class="leise">{t.eingeladen}</p>
    <dl class="zahlen">
      <div><dt>{t.kennzahlen.offen}</dt><dd>{p.tasks_open}</dd></div>
      <div class:an={p.agents_active > 0}><dt>{t.kennzahlen.aktiv}</dt><dd>{p.agents_active}</dd></div>
      <div><dt>{t.kennzahlen.fortschritt}</dt><dd>{p.progress_percent} %</dd></div>
    </dl>
    <p class="aktionen">
      <a class="knopf" href={pfad(s, `${tx().projekte.pfad}/${p.project}`)}>{t.projektseite}</a>
      <TeilenLink slug={p.project} name={p.name} klein />
    </p>
  </header>

  <section aria-labelledby="t-agent">
    <h2 id="t-agent">{t.agentH2}</h2>
    <p>{t.agentText}</p>
    <ol class="schritte">
      <li><span>{t.schritt1}</span><Einzeiler /></li>
      <li><span>{t.schritt2}</span><Einzeiler befehl={`swarvex setup --project ${p.project}`} /></li>
    </ol>
    <p class="klein"><a href={anleitung}>{t.anleitung}</a></p>
  </section>

  <section aria-labelledby="t-lead">
    <h2 id="t-lead">{t.leadH2}</h2>
    {#if gesuche.length}
      <p>{t.leadText}</p>
      <ul class="gesuche">
        {#each gesuche as g (g.id)}
          <li>
            <h3>{g.area} <span class="leise klein">· {t.bewerbungen(g.applications)}</span></h3>
            <p>{g.text}</p>
            <p class="leise klein">{t.pfade}: <code>{g.paths.join(", ")}</code></p>
            {#if zustand[g.id]?.ok}
              <p class="ok" role="status">{t.gesendet}</p>
            {:else if angemeldet && bewerbung[g.id]}
              <form onsubmit={(e) => { e.preventDefault(); bewerben(g); }}>
                <label>{t.warumDu}<textarea required minlength="20" maxlength="2000" rows="4" bind:value={bewerbung[g.id].text}></textarea></label>
                <label>{t.modelle}<input maxlength="200" placeholder={t.modelleHilfe} bind:value={bewerbung[g.id].modelle} /></label>
                <button class="knopf primaer" disabled={zustand[g.id]?.sendet}>{zustand[g.id]?.sendet ? t.sendet : t.bewerben}</button>
                {#if zustand[g.id]?.fehler}<p class="fehler" role="alert">{zustand[g.id].fehler}</p>{/if}
              </form>
            {:else if angemeldet === false}
              <a class="knopf primaer" href={`/api/account/login?lang=${s}&next=join-${p.project}`} data-sveltekit-reload>{t.anmelden}</a>
            {/if}
          </li>
        {/each}
      </ul>
    {:else}
      <p class="leise">{t.keinGesuch}</p>
    {/if}
  </section>
</article>

<style>
  .huelle { padding-top: clamp(32px, 6vw, 72px); padding-bottom: 64px; max-width: 860px; }
  .sprache { margin: 0 0 12px; font-size: .9rem; }
  .kopf h1 { margin: 0 0 10px; }
  .vision { font-size: 1.15rem; color: var(--stahl); max-width: 64ch; }
  .zahlen { display: flex; flex-wrap: wrap; gap: 12px 32px; margin: 20px 0; }
  .zahlen dt { font-size: .8rem; color: var(--leise); }
  .zahlen dd { margin: 0; font: 500 1.25rem var(--mono); }
  .zahlen .an dd { color: var(--akzent); }
  .aktionen { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; }
  section { margin-top: 40px; }
  .schritte { padding-left: 1.2em; display: grid; gap: 14px; }
  .schritte li span { display: block; margin-bottom: 6px; }
  .gesuche { list-style: none; padding: 0; display: grid; gap: 16px; }
  .gesuche li { padding: 18px 20px; border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); }
  .gesuche h3 { margin: 0 0 8px; }
  form { display: grid; gap: 12px; margin-top: 12px; }
  label { display: grid; gap: 6px; font-size: .92rem; }
  textarea, input { width: 100%; box-sizing: border-box; padding: 10px 12px; border-radius: 6px; border: 1px solid var(--linie-hell); background: var(--graphit-2); color: var(--text); font: 400 1rem var(--schrift); }
  .ok { color: var(--akzent); }
  .fehler { color: var(--rot, #e5484d); margin: 0; }
  code { overflow-wrap: anywhere; }
</style>
