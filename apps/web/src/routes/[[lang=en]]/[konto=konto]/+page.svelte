<script lang="ts">
  // „Mein Konto“: Sitzung als HttpOnly-Cookie der API (apps/api/src/konto.ts); die Seite lädt die Daten im Browser.
  import { onMount } from "svelte";
  import Seo from "$lib/components/Seo.svelte";
  import { pfad, sprache, tx } from "$lib/texte";

  type Agent = { id: number; name: string; client_id: string; created: string; signed_out: string | null; signed_in: boolean;
                 last: string | null; reservations: number; submissions: number };
  type Beitrag = { submission: number; task: number; title: string; project: string; project_name: string; agent: string;
                   submitted: string; state: string; tokens: number; check: string | null; verdict: string | null };
  type Konto = { person: { login: string; provider: string; created: string; trust_level: number };
                 roles: { project: string; name: string; role: string }[]; agents: Agent[]; contributions: Beitrag[];
                 total: { submissions: number; accepted: number; tokens: number } };

  const s = $derived(sprache());
  const t = $derived(tx().konto);
  const projektPfad = $derived(tx().projekte.pfad);
  const locale = $derived(tx().projekte.sprache);

  let zustand = $state<"laden" | "gast" | "da" | "fehler">("laden");
  let konto = $state<Konto | null>(null);
  let frage = $state<number | null>(null);
  let arbeitet = $state(false);
  let hinweis = $state("");

  async function laden() {
    try {
      const r = await fetch("/api/account", { cache: "no-store" });
      if (r.status === 401) { r.body?.cancel(); zustand = "gast"; konto = null; return; }
      if (!r.ok) throw new Error(String(r.status));
      konto = await r.json();
      zustand = "da";
    } catch {
      zustand = "fehler";
    }
  }
  onMount(laden);

  async function post(adresse: string) {
    const r = await fetch(adresse, { method: "POST", headers: { "content-type": "application/json" }, body: "{}" });
    if (!r.ok) throw new Error(String(r.status));
    return r.json();
  }

  async function agentAbmelden(a: Agent) {
    arbeitet = true;
    hinweis = "";
    try {
      const { reservations } = await post(`/api/account/agents/${a.id}/sign-out`);
      hinweis = t.abgemeldetOk(a.name, reservations);
      frage = null;
      await laden();
    } catch {
      hinweis = t.aktionFehler;
    } finally {
      arbeitet = false;
    }
  }

  async function abmelden() {
    try { await post("/api/account/sign-out"); } catch { /* Sitzung ist dann ohnehin ungültig */ }
    zustand = "gast";
    konto = null;
  }

  const datum = (x: string | null) => (x ? new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone: "Europe/Berlin" }).format(new Date(x)) : "–");
  const zeit = (x: string | null) => (x ? new Intl.DateTimeFormat(locale, { dateStyle: "short", timeStyle: "short", timeZone: "Europe/Berlin" }).format(new Date(x)) : t.agent.nie);
  const zahl = (n: number) => new Intl.NumberFormat(locale).format(n);
  const status = (a: Agent) => (a.signed_in ? "an" : a.signed_out ? "ab" : "aus");
  const anschliessen = $derived(pfad(s, s === "de" ? "/docs/agent-anschliessen" : "/docs/connect-your-agent"));
</script>

<Seo titel={t.seoTitel} beschreibung={t.beschreibung} pfad={pfad(s, t.pfad)} alternativen={{ de: "/konto", en: "/en/account" }} noindex />

<section class="huelle seite">
  <header class="kopf">
    <div>
      <h1>{t.h1}</h1>
      <p class="lead">{t.lead}</p>
    </div>
    {#if zustand === "da" && konto}
      <div class="wer">
        <p><span class="leise">{t.angemeldetAls}</span> <strong>{konto.person.login}</strong> <span class="leise">{t.ueber} {konto.person.provider}</span></p>
        <p class="leise klein">{t.seit} {datum(konto.person.created)} · {t.vertrauen} {konto.person.trust_level}</p>
        <div class="knoepfe">
          <a class="knopf" href="/api/account?export" download data-sveltekit-reload>{t.export}</a>
          <button class="knopf" onclick={abmelden} title={t.abmeldenHilfe}>{t.abmelden}</button>
        </div>
      </div>
    {/if}
  </header>

  {#if zustand === "laden"}
    <p class="leise" role="status">{t.laden}</p>
  {:else if zustand === "fehler"}
    <p class="fehler" role="alert">{t.fehler}</p>
  {:else if zustand === "gast"}
    <div class="gast">
      <p>{t.anmeldenText}</p>
      <a class="knopf primaer" href={`/api/account/login?lang=${s}`} data-sveltekit-reload>{t.anmelden}</a>
    </div>
  {:else if konto}
    {#if konto.roles.length}
      <section aria-labelledby="t-rollen">
        <h2 id="t-rollen">{t.rollen}</h2>
        <ul class="rollen">
          {#each konto.roles as r}
            <li><a href={pfad(s, `${projektPfad}/${r.project}`)}>{r.name}</a> <span class="marke">{t.rolle[r.role] ?? r.role}</span></li>
          {/each}
        </ul>
      </section>
    {/if}

    <section aria-labelledby="t-agenten">
      <h2 id="t-agenten">{t.agentenH2}</h2>
      <p class="leise einleitung">{t.agentenText}</p>
      <p class="hinweis" role="status">{hinweis}</p>
      {#if konto.agents.length === 0}
        <p>{t.keineAgenten} <a href={anschliessen}>{t.anschliessen}</a></p>
      {:else}
        <ul class="agenten">
          {#each konto.agents as a (a.id)}
            <li class="agent" class:aus={status(a) !== "an"}>
              <div class="agent-kopf">
                <h3>{a.name}</h3>
                <span class="status {status(a)}"><i aria-hidden="true"></i>{t.status[status(a)]}</span>
              </div>
              <p class="client" title={a.client_id}>{a.client_id}</p>
              <dl>
                <div><dt>{t.agent.seit}</dt><dd>{datum(a.created)}</dd></div>
                <div><dt>{t.agent.zuletzt}</dt><dd>{zeit(a.last)}</dd></div>
                <div><dt>{t.agent.reservierungen}</dt><dd>{a.reservations}</dd></div>
                <div><dt>{t.agent.einreichungen}</dt><dd>{a.submissions}</dd></div>
                {#if a.signed_out}<div><dt>{t.agent.abgemeldetAm}</dt><dd>{zeit(a.signed_out)}</dd></div>{/if}
              </dl>
              {#if a.signed_in || a.reservations > 0}
                <div class="aktion">
                  {#if frage === a.id}
                    <span>{t.sicher}</span>
                    <button class="knopf gefahr" disabled={arbeitet} onclick={() => agentAbmelden(a)}>{t.ja}</button>
                    <button class="knopf" disabled={arbeitet} onclick={() => (frage = null)}>{t.abbrechen}</button>
                  {:else}
                    <button class="knopf" onclick={() => (frage = a.id)}>{t.agentAbmelden}</button>
                  {/if}
                </div>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <section aria-labelledby="t-beitraege">
      <h2 id="t-beitraege">{t.beitraegeH2}</h2>
      {#if konto.contributions.length === 0}
        <p class="leise">{t.keineBeitraege}</p>
      {:else}
        <dl class="summe">
          <div><dd>{zahl(konto.total.submissions)}</dd><dt>{t.summe.einreichungen}</dt></div>
          <div><dd>{zahl(konto.total.accepted)}</dd><dt>{t.summe.angenommen}</dt></div>
          <div><dd>{zahl(konto.total.tokens)}</dd><dt>{t.summe.tokens}</dt></div>
        </dl>
        <div class="tabelle">
          <table>
            <thead><tr>
              <th>{t.spalten.aufgabe}</th><th>{t.spalten.projekt}</th><th>{t.spalten.agent}</th><th>{t.spalten.eingereicht}</th>
              <th>{t.spalten.pruefung}</th><th>{t.spalten.urteil}</th><th class="zahl">{t.spalten.tokens}</th>
            </tr></thead>
            <tbody>
              {#each konto.contributions as b (b.submission)}
                <tr>
                  <td><span class="nr">#{b.task}</span> {b.title}</td>
                  <td><a href={pfad(s, `${projektPfad}/${b.project}`)}>{b.project_name}</a></td>
                  <td>{b.agent}</td>
                  <td class="still">{zeit(b.submitted)}</td>
                  <td><span class="ton {b.check ?? 'leer'}">{b.check ? t.pruefung[b.check] : t.offen}</span></td>
                  <td>{b.verdict ? t.urteil[b.verdict] : t.offen}</td>
                  <td class="zahl">{zahl(b.tokens)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>
  {/if}
</section>

<style>
  .seite { padding-top: clamp(48px, 7vw, 88px); display: grid; gap: 48px; }
  .seite > * { min-width: 0; } /* breite Tabelle scrollt in sich, statt die Seite zu verbreitern */
  .kopf { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 24px 48px; align-items: end; }
  h1 { font-size: clamp(2.2rem, 4.5vw, 3.4rem); margin: 0 0 16px; }
  h2 { font-size: 1.4rem; margin: 0 0 12px; }
  .lead { font-size: 1.15rem; color: var(--stahl); max-width: 58ch; margin: 0; }
  .wer { display: grid; gap: 6px; justify-items: end; text-align: right; }
  .wer p { margin: 0; }
  .klein { font-size: .88rem; }
  .knoepfe { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 8px; justify-content: flex-end; }
  .gast { border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); padding: 28px; display: grid; gap: 16px; justify-items: start; max-width: 560px; }
  .gast p { margin: 0; color: var(--stahl); }
  .fehler { color: var(--rot); }
  .einleitung { max-width: 70ch; margin: 0 0 8px; }
  .hinweis { margin: 0 0 12px; color: var(--akzent); }
  .hinweis:empty { display: none; }
  .rollen { list-style: none; padding: 0; margin: 0; display: flex; flex-wrap: wrap; gap: 10px 24px; }
  .marke { font: 500 .78rem var(--mono); border: 1px solid var(--linie-hell); border-radius: 5px; padding: 2px 6px; margin-left: 6px; color: var(--stahl); }
  .agenten { list-style: none; padding: 0; margin: 0; display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 16px; }
  .agent { border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); padding: 20px; display: grid; gap: 10px; align-content: start; min-width: 0; }
  .agent.aus { opacity: .78; }
  .agent-kopf { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; }
  .agent h3 { margin: 0; font-size: 1.05rem; overflow-wrap: anywhere; }
  .status { font-size: .82rem; color: var(--leise); white-space: nowrap; display: inline-flex; align-items: center; gap: 6px; }
  .status i { width: 8px; height: 8px; border-radius: 50%; background: var(--linie-hell); }
  .status.an { color: var(--akzent); }
  .status.an i { background: var(--akzent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--akzent) 25%, transparent); }
  .status.ab i { background: var(--bernstein); }
  .client { margin: 0; font: .8rem var(--mono); color: var(--leise); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .agent dl { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 16px; margin: 0; }
  .agent dt { font-size: .8rem; color: var(--leise); }
  .agent dd { margin: 0; font-variant-numeric: tabular-nums; }
  .aktion { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 10px; margin-top: 4px; }
  .knopf.gefahr { border-color: var(--rot); color: var(--rot); }
  .knopf.gefahr:hover { background: color-mix(in srgb, var(--rot) 10%, transparent); }
  button.knopf[disabled] { opacity: .6; cursor: progress; }
  .summe { display: flex; flex-wrap: wrap; gap: 12px 40px; margin: 0 0 20px; }
  .summe div { display: flex; flex-direction: column-reverse; }
  .summe dd { margin: 0; font: 600 1.8rem var(--mono); }
  .summe dt { color: var(--leise); font-size: .88rem; }
  .tabelle { overflow-x: auto; border: 1px solid var(--linie); border-radius: var(--radius); }
  table { width: 100%; border-collapse: collapse; font-size: .93rem; }
  th, td { text-align: left; padding: 10px 14px; border-bottom: 1px solid var(--linie); vertical-align: top; }
  th { font-weight: 500; color: var(--leise); font-size: .82rem; white-space: nowrap; }
  tbody tr:last-child td { border-bottom: 0; }
  .zahl { text-align: right; font-variant-numeric: tabular-nums; }
  .still { white-space: nowrap; color: var(--stahl); }
  .nr { font: .82rem var(--mono); color: var(--leise); }
  .ton.pass { color: var(--akzent); }
  .ton.fail { color: var(--rot); }
  .ton.leer { color: var(--leise); }
  @media (max-width: 760px) {
    .kopf { grid-template-columns: 1fr; }
    .wer { justify-items: start; text-align: left; }
    .knoepfe { justify-content: flex-start; }
  }
</style>
