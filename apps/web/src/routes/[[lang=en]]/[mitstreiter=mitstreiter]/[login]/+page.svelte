<script lang="ts">
  // Öffentliches Beitragsprofil (apps/api/src/profil.ts): nur, wenn die Person es im Konto eingeschaltet hat.
  import Seo from "$lib/components/Seo.svelte";
  import { SEITE } from "$lib/seo";
  import { pfad, sprache, tx } from "$lib/texte";

  type Profil = { login: string; since: string; trust_level: number; accepted: number; peer_reviews: number;
                  projects: { project: string; name: string; accepted: number }[]; kinds: { kind: string; count: number }[]; badges: string[] };
  let { data } = $props();
  const s = $derived(sprache());
  const t = $derived(tx().mitstreiter);
  const p = $derived(data.profil as Profil);
  const projektPfad = $derived(tx().projekte.pfad);
  const monat = $derived(new Intl.DateTimeFormat(s === "de" ? "de-DE" : "en-US", { month: "long", year: "numeric" }).format(new Date(`${p.since}-01T12:00:00Z`)));
  const person = $derived({ "@type": "ProfilePage", mainEntity: { "@type": "Person", name: p.login, identifier: p.login, url: SEITE + pfad(s, `${t.pfad}/${p.login}`) } });
</script>

<Seo titel={t.titel(p.login)} beschreibung={t.beschreibung(p.login, p.accepted)} pfad={pfad(s, `${t.pfad}/${p.login}`)} typ="profile" daten={[person]}
  alternativen={{ de: `/mitstreiter/${p.login}`, en: `/en/contributors/${p.login}` }} />

<section class="huelle seite">
  <header>
    <h1>{p.login}</h1>
    <p class="leise">{t.seit} {monat} · {t.stufe} {p.trust_level}</p>
  </header>

  <dl class="zahlen">
    <div><dt>{t.angenommen}</dt><dd>{p.accepted}</dd></div>
    <div><dt>{t.pruefungen}</dt><dd>{p.peer_reviews}</dd></div>
    <div><dt>{t.projekte}</dt><dd>{p.projects.length}</dd></div>
  </dl>

  {#if p.badges.length}
    <section aria-labelledby="t-abzeichen">
      <h2 id="t-abzeichen">{t.abzeichen}</h2>
      <ul class="abzeichen">
        {#each p.badges as b}<li><strong>{t.marken[b]?.[0] ?? b}</strong><span class="leise">{t.marken[b]?.[1] ?? ""}</span></li>{/each}
      </ul>
    </section>
  {/if}

  <section aria-labelledby="t-projekte">
    <h2 id="t-projekte">{t.projekte}</h2>
    {#if p.projects.length}
      <ul class="projekte">
        {#each p.projects as x (x.project)}<li><a href={pfad(s, `${projektPfad}/${x.project}`)}>{x.name}</a><span class="zahl">{x.accepted}</span></li>{/each}
      </ul>
    {:else}<p class="leise">{t.keine}</p>{/if}
  </section>

  {#if p.kinds.length}
    <section aria-labelledby="t-arten">
      <h2 id="t-arten">{t.arten}</h2>
      <ul class="arten">{#each p.kinds as k}<li>{t.sorten[k.kind] ?? k.kind} <span class="leise">{k.count}</span></li>{/each}</ul>
    </section>
  {/if}
</section>

<style>
  .seite { padding-top: clamp(48px, 7vw, 88px); display: grid; gap: 28px; max-width: 880px; }
  .seite > * { min-width: 0; }
  h1 { font-size: clamp(2rem, 4.5vw, 3rem); margin: 0 0 8px; overflow-wrap: anywhere; }
  h2 { font-size: 1.2rem; margin: 0 0 10px; }
  header p { margin: 0; }
  .zahlen { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; margin: 0; }
  .zahlen div { border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); padding: 14px 16px; }
  .zahlen dt { color: var(--leise); font-size: .85rem; }
  .zahlen dd { margin: 4px 0 0; font: 500 1.8rem var(--mono); }
  .abzeichen { list-style: none; padding: 0; margin: 0; display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 200px), 1fr)); gap: 10px; }
  .abzeichen li { border: 1px solid var(--vision); border-radius: var(--radius-klein); padding: 10px 14px; display: grid; gap: 2px; }
  .abzeichen span { font-size: .85rem; }
  .projekte { list-style: none; padding: 0; margin: 0; display: grid; gap: 6px; }
  .projekte li { display: flex; justify-content: space-between; gap: 12px; border-bottom: 1px solid var(--linie); padding: 6px 0; }
  .projekte .zahl { font: 500 .95rem var(--mono); }
  .arten { list-style: none; padding: 0; margin: 0; display: flex; flex-wrap: wrap; gap: 8px 18px; }
</style>
