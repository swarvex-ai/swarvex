<script lang="ts">
  // Assistent „Projekt starten“: Lead wählen, Projektart, Eckdaten, mit GitHub anmelden, anlegen (POST /api/account/projects, apps/api/src/projekte.ts),
  // dann die Befehle für genau dieses Lead-Paket (/lead/<paket>.md). Eingaben überleben die Anmeldung in sessionStorage.
  import { onMount } from "svelte";
  import Seo from "$lib/components/Seo.svelte";
  import Einzeiler from "$lib/components/Einzeiler.svelte";
  import { pfad, sprache, tx } from "$lib/texte";

  const s = $derived(sprache());
  const t = $derived(tx().starten);
  const ARTEN = ["game", "app", "website", "service", "tool", "library", "plugin"];
  const MODELL: Record<string, string> = { claude: "Claude Opus 5.5", codex: "", andere: "" };
  const STACKS: Record<string, string[]> = {
    game: ["godot-4", "unity-6", "bevy", "phaser", "love2d"], app: ["sveltekit", "react", "flutter", "tauri", "react-native"],
    website: ["sveltekit", "astro", "hugo", "eleventy"], service: ["node", "go", "python-fastapi", "rust-axum", "dotnet"],
    tool: ["rust", "go", "python", "node"], library: ["rust", "typescript", "python", "go", "c"], plugin: ["godot-plugin", "vscode-extension", "blender-addon", "obsidian-plugin"],
  };
  // Lizenz-Empfehlung je Art (der Visionär kann jede OSI- bzw. freie Lizenz wählen)
  const LIZENZ: Record<string, [string, string]> = {
    game: ["GPL-3.0-or-later", "CC BY-SA 4.0"], app: ["GPL-3.0-or-later", "CC BY-SA 4.0"], website: ["MIT", "CC BY 4.0"], service: ["AGPL-3.0-or-later", "CC BY-SA 4.0"],
    tool: ["GPL-3.0-or-later", "CC BY 4.0"], library: ["MIT", "CC BY 4.0"], plugin: ["GPL-3.0-or-later", "CC BY-SA 4.0"],
  };
  const CODE = ["MIT", "Apache-2.0", "MPL-2.0", "GPL-3.0-or-later", "AGPL-3.0-or-later", "LGPL-3.0-or-later", "EUPL-1.2", "BSD-3-Clause", "ISC", "0BSD", "Unlicense"];
  const ASSETS = ["CC BY-SA 4.0", "CC BY 4.0", "CC0 1.0", "OFL-1.1"];
  const SPEICHER = "swarvex-starten";

  type Daten = { agent: string; modell: string; type: string; cleanroom: boolean;
                 original: { name: string; version: string; acquisition: string; goal: string; legal_basis: string; inspect: string };
                 name: string; slug: string; slugFrei: boolean; vision_short: string; vision_md: string; engine: string;
                 license_code: string; license_assets: string; goal: string; repo_url: string };
  let d = $state<Daten>({ agent: "claude", modell: MODELL.claude, type: "", cleanroom: false,
    original: { name: "", version: "", acquisition: "", goal: "", legal_basis: "", inspect: "" },
    name: "", slug: "", slugFrei: false, vision_short: "", vision_md: "", engine: "open", license_code: "", license_assets: "", goal: "", repo_url: "" });
  let schritt = $state(0);
  let angemeldet = $state<boolean | null>(null);
  let arbeitet = $state(false);
  let fehler = $state("");
  let ergebnis = $state<{ slug: string; name: string; paket: string; type: string; cleanroom: boolean; agent: string } | null>(null);

  onMount(async () => {
    try {
      const alt = JSON.parse(sessionStorage.getItem(SPEICHER) ?? "null");
      if (alt?.d) { d = { ...d, ...alt.d }; schritt = alt.schritt ?? 0; ergebnis = alt.ergebnis ?? null; }
    } catch { /* ohne Speicher beginnt der Assistent von vorn */ }
    try { const r = await fetch("/api/account", { cache: "no-store" }); r.body?.cancel(); angemeldet = r.ok; } catch { angemeldet = false; }
  });
  $effect(() => {
    const stand = JSON.stringify({ d, schritt, ergebnis });
    try { sessionStorage.setItem(SPEICHER, stand); } catch { /* privates Fenster: dann eben ohne */ }
  });

  // Adresse aus dem Namen, bis sie jemand selbst ändert
  const ausName = (n: string) => n.toLowerCase().replace(/ä/g, "ae").replace(/ö/g, "oe").replace(/ü/g, "ue").replace(/ß/g, "ss")
    .normalize("NFKD").replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 40).replace(/-+$/, "");
  function nameGeaendert() { if (!d.slugFrei) d.slug = ausName(d.name); }
  function agentWaehlen(a: string) { d.agent = a; if (!d.modell || Object.values(MODELL).includes(d.modell)) d.modell = MODELL[a]; }
  function artWaehlen(a: string) {
    if (!d.license_code || d.license_code === LIZENZ[d.type]?.[0]) d.license_code = LIZENZ[a][0];
    if (!d.license_assets || d.license_assets === LIZENZ[d.type]?.[1]) d.license_assets = LIZENZ[a][1];
    d.type = a;
  }

  const AGENT_NAME: Record<string, string> = { claude: "Claude Code", codex: "Codex", andere: "" };
  const leadModell = $derived([AGENT_NAME[d.agent], d.modell.trim()].filter(Boolean).join(" · "));
  const slugOk = $derived(/^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$/.test(d.slug));
  const repoOk = $derived(!d.repo_url || /^https:\/\/(github\.com|codeberg\.org|gitlab\.com)\/[\w.-]+\/[\w.-]+\/?$/.test(d.repo_url.trim()));
  const fertig = $derived([
    leadModell.length > 0 && d.modell.trim().length > 0,
    !!d.type && (!d.cleanroom || !!(d.original.name.trim() && d.original.acquisition.trim() && d.original.legal_basis.trim())),
    !!d.name.trim() && slugOk && !!d.vision_short.trim() && !!d.engine.trim() && !!d.goal.trim() && repoOk,
  ]);

  function weiter() { fehler = ""; if (fertig[schritt] === false) { fehler = t.anlegen.pruefen; return; } schritt += 1; scrollTo({ top: 0 }); }
  function zurueck() { fehler = ""; schritt = Math.max(0, schritt - 1); }

  async function anlegen() {
    arbeitet = true; fehler = "";
    const o = d.original;
    const body = { slug: d.slug, name: d.name.trim(), vision_short: d.vision_short.trim(), vision_md: d.vision_md.trim(), engine: d.engine.trim(),
      license_code: d.license_code, license_assets: d.license_assets, repo_url: d.repo_url.trim().replace(/\/$/, "") || null, lead_model: leadModell,
      goal: d.goal.trim(), type: d.type, procedure: d.cleanroom ? "clean_room" : "standard",
      original: d.cleanroom ? Object.fromEntries(Object.entries(o).map(([k, v]) => [k, v.trim()]).filter(([, v]) => v)) : null };
    try {
      const r = await fetch("/api/account/projects", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
      const j = await r.json().catch(() => ({}));
      if (r.status === 401) { angemeldet = false; return; }
      if (!r.ok) { fehler = j.error ?? t.anlegen.fehler; return; }
      ergebnis = { slug: j.slug, name: body.name, paket: `${d.type}${d.cleanroom ? "-cleanroom" : ""}`, type: d.type, cleanroom: d.cleanroom, agent: d.agent };
      schritt = 4;
    } catch { fehler = t.anlegen.fehler; } finally { arbeitet = false; }
  }
  function neuesProjekt() { ergebnis = null; schritt = 0; d = { ...d, name: "", slug: "", slugFrei: false, vision_short: "", vision_md: "", goal: "", repo_url: "" }; }

  const anmeldeLink = $derived(`/api/account/login?lang=${s}&next=${s === "en" ? "start" : "starten"}`);
  const BASIS = "https://swarvex.ai";
  const ordner = $derived(ergebnis ? `mkdir -p ~/swarvex-lead/${ergebnis.slug} && cd ~/swarvex-lead/${ergebnis.slug}` : "");
  const auftrag = $derived(ergebnis ? t.auftrag(ergebnis.slug, t.art.arten[ergebnis.type][0], ergebnis.cleanroom) : "");
  let kopiert = $state(false);
  async function auftragKopieren() {
    try { await navigator.clipboard.writeText(auftrag); kopiert = true; setTimeout(() => (kopiert = false), 1800); } catch { /* Text bleibt markierbar */ }
  }
  const doku = (de: string, en: string) => pfad(s, `/docs/${s === "de" ? de : en}`);
</script>

<Seo titel={t.seoTitel} beschreibung={t.beschreibung} pfad={pfad(s, t.pfad)} alternativen={{ de: "/starten", en: "/en/start" }} />

<section class="huelle seite">
  <header>
    <h1>{t.h1}</h1>
    <p class="lead">{t.einleitung}</p>
  </header>

  <ol class="leiste" aria-label={t.h1}>
    {#each t.schritte as name, i}
      <li class:jetzt={i === schritt} class:erledigt={i < schritt} aria-current={i === schritt ? "step" : undefined}><span>{i + 1}</span>{name}</li>
    {/each}
  </ol>

  <div class="karte">
    {#if schritt === 0}
      <h2>{t.lead.h2}</h2>
      <p class="leise">{t.lead.text}</p>
      <div class="wahl" role="radiogroup" aria-label={t.lead.h2}>
        {#each Object.entries(t.lead.agenten) as [a, [name, text]]}
          <button type="button" role="radio" aria-checked={d.agent === a} class:an={d.agent === a} onclick={() => agentWaehlen(a)}>
            <strong>{name}{#if a === "claude"} <span class="marke">{t.lead.empfohlen}</span>{/if}</strong><small>{text}</small>
          </button>
        {/each}
      </div>
      <label>{t.lead.modell}<input type="text" maxlength="80" bind:value={d.modell} placeholder={t.lead.modellHilfe} /></label>
      <p class="tipp">{t.lead.frontier}</p>

    {:else if schritt === 1}
      <h2>{t.art.h2}</h2>
      <div class="wahl arten" role="radiogroup" aria-label={t.art.h2}>
        {#each ARTEN as a}
          <button type="button" role="radio" aria-checked={d.type === a} class:an={d.type === a} onclick={() => artWaehlen(a)}>
            <strong>{t.art.arten[a][0]}</strong><small>{t.art.arten[a][1]}</small>
          </button>
        {/each}
      </div>
      <p class="leise klein">{t.art.grundpaket}</p>
      <label class="schalter"><input type="checkbox" bind:checked={d.cleanroom} /> <strong>{t.art.cleanroom}</strong></label>
      <p class="leise klein">{t.art.cleanroomText} <a href={doku("clean-room", "clean-room")}>{t.art.cleanroomMehr}</a></p>
      {#if d.cleanroom}
        <div class="felder">
          <label>{t.art.original.name}<input type="text" maxlength="120" bind:value={d.original.name} required /></label>
          <label>{t.art.original.version}<input type="text" maxlength="60" bind:value={d.original.version} /></label>
          <label class="breit">{t.art.original.erwerb}<input type="text" maxlength="300" bind:value={d.original.acquisition} placeholder={t.art.original.erwerbHilfe} required /></label>
          <label class="breit">{t.art.original.rechtsgrundlage}<input type="text" maxlength="300" bind:value={d.original.legal_basis} placeholder={t.art.original.rechtsgrundlageHilfe} required /></label>
          <label class="breit">{t.art.original.ziel}<input type="text" maxlength="300" bind:value={d.original.goal} /></label>
          <label class="breit">{t.art.original.inspizieren}<input type="text" maxlength="300" bind:value={d.original.inspect} placeholder={t.art.original.inspizierenHilfe} /></label>
        </div>
      {/if}

    {:else if schritt === 2}
      <h2>{t.projekt.h2}</h2>
      <div class="felder">
        <label>{t.projekt.name}<input type="text" maxlength="80" bind:value={d.name} oninput={nameGeaendert} required /></label>
        <label>{t.projekt.adresse}
          <span class="adresse" class:falsch={d.slug && !slugOk}><span class="leise">swarvex.ai{pfad(s, tx().projekte.pfad)}/</span><input type="text" maxlength="40" bind:value={d.slug} oninput={() => (d.slugFrei = true)} required /></span>
          <small class="leise">{t.projekt.adresseHilfe}</small></label>
        <label class="breit">{t.projekt.kurz}<input type="text" maxlength="200" bind:value={d.vision_short} placeholder={t.projekt.kurzHilfe} required /></label>
        <label class="breit">{t.projekt.vision}<textarea rows="4" maxlength="20000" bind:value={d.vision_md} placeholder={t.projekt.visionHilfe}></textarea></label>
        <label>{t.projekt.engine}<input type="text" maxlength="60" list="stacks" bind:value={d.engine} required /><small class="leise">{t.projekt.engineHilfe}</small></label>
        <datalist id="stacks"><option value="open"></option>{#each STACKS[d.type] ?? [] as x}<option value={x}></option>{/each}</datalist>
        <label>{t.projekt.ziel}<input type="text" maxlength="200" bind:value={d.goal} placeholder={t.projekt.zielHilfe} required /></label>
        <label>{t.projekt.lizenzCode}<select bind:value={d.license_code}>{#each CODE as l}<option>{l}</option>{/each}</select></label>
        <label>{t.projekt.lizenzAssets}<select bind:value={d.license_assets}>{#each ASSETS as l}<option>{l}</option>{/each}</select></label>
        <p class="leise klein breit">{t.projekt.lizenzHilfe} <a href={doku("lizenz", "license")}>{t.projekt.lizenzMehr}</a></p>
        <label class="breit">{t.projekt.repo}<input type="url" maxlength="200" bind:value={d.repo_url} class:falsch={!repoOk} placeholder="https://github.com/…" /><small class="leise">{t.projekt.repoHilfe}</small></label>
      </div>

    {:else if schritt === 3}
      <h2>{t.anlegen.h2}</h2>
      <dl class="uebersicht">
        <div><dt>{t.anlegen.lead}</dt><dd>{leadModell} <button class="link" onclick={() => (schritt = 0)}>{t.anlegen.aendern}</button></dd></div>
        <div><dt>{t.anlegen.art}</dt><dd>{t.art.arten[d.type][0]}{#if d.cleanroom} · {t.anlegen.cleanroom} ({d.original.name}){/if} <button class="link" onclick={() => (schritt = 1)}>{t.anlegen.aendern}</button></dd></div>
        <div><dt>{t.projekt.name}</dt><dd><strong>{d.name}</strong> · {d.vision_short} <button class="link" onclick={() => (schritt = 2)}>{t.anlegen.aendern}</button></dd></div>
        <div><dt>{t.anlegen.adresse}</dt><dd><code>swarvex.ai{pfad(s, tx().projekte.pfad)}/{d.slug}</code></dd></div>
        <div><dt>{t.anlegen.engine}</dt><dd>{d.engine}</dd></div>
        <div><dt>{t.anlegen.lizenzen}</dt><dd>{d.license_code} · {d.license_assets}</dd></div>
        <div><dt>{t.anlegen.ziel}</dt><dd>{d.goal}</dd></div>
      </dl>
      <p class="leise klein">{t.anlegen.entwurf}</p>
      {#if angemeldet === false}
        <p>{t.anlegen.anmeldenText}</p>
        <a class="knopf visionaer" href={anmeldeLink} data-sveltekit-reload>{t.anlegen.anmelden}</a>
      {:else if angemeldet}
        <p class="leise klein">{t.anlegen.bedingungen} <a href={pfad(s, s === "de" ? "/nutzungsbedingungen" : "/terms")}>{t.anlegen.bedingungenLink}</a></p>
        <button class="knopf visionaer" disabled={arbeitet} onclick={anlegen}>{arbeitet ? t.anlegen.legtAn : t.anlegen.anlegen}</button>
      {/if}

    {:else if schritt === 4 && ergebnis}
      <h2>{t.los.h2(ergebnis.name)}</h2>
      <p>{t.los.text}</p>
      <ol class="befehle">
        <li><span>{t.los.ordner}</span><Einzeiler befehl={ordner} /></li>
        {#if ergebnis.agent === "claude"}
          <li><span>{t.los.mcp}</span><Einzeiler befehl={`claude mcp add --transport http swarvex ${BASIS}/mcp`} /></li>
          <li><span>{t.los.rollen}</span><Einzeiler befehl={`curl -fsSL ${BASIS}/lead/${ergebnis.paket}/agents.sh | sh`} /></li>
          <li><span>{t.los.starten}</span><Einzeiler befehl={`claude --append-system-prompt "$(curl -fsSL ${BASIS}/lead/${ergebnis.paket}.md)"`} /></li>
        {:else if ergebnis.agent === "codex"}
          <li><span>{t.los.paketSpeichern}</span><Einzeiler befehl={`curl -fsSL ${BASIS}/lead/${ergebnis.paket}.md -o AGENTS.md`} /></li>
          <li><span>{t.los.mcp}</span><Einzeiler befehl={`codex mcp add swarvex --url ${BASIS}/mcp`} /></li>
          <li><span>{t.los.codexStarten}</span><Einzeiler befehl="codex" /></li>
        {:else}
          <li><span>{t.los.paketSpeichern}</span><Einzeiler befehl={`curl -fsSL ${BASIS}/lead/${ergebnis.paket}.md -o AGENTS.md`} /></li>
          <li><span>{t.los.andereText} <a href={doku("agent-anschliessen", "connect-your-agent")}>{t.los.andereLink}</a></span></li>
        {/if}
        <li><span>{t.los.anmelden}</span></li>
        <li><span>{t.los.auftrag}</span>
          <div class="auftrag"><pre>{auftrag}</pre><button class="knopf" onclick={auftragKopieren}>{kopiert ? t.los.kopiert : t.los.kopieren}</button></div></li>
      </ol>
      <p class="klein"><a href={`/lead/${ergebnis.paket}.md`}>{t.los.paket}</a></p>
      <h3>{t.los.repoH3}</h3>
      <p class="leise">{t.los.repoText}</p>
      <p>{t.los.danach}</p>
      <div class="knoepfe">
        <a class="knopf primaer" href={pfad(s, tx().konto.pfad)}>{t.los.konto}</a>
        <button class="knopf" onclick={neuesProjekt}>{t.h1}</button>
      </div>
    {/if}

    <p class="fehler" role="alert">{fehler}</p>
    {#if schritt < 3}
      <div class="knoepfe">
        {#if schritt > 0}<button class="knopf" onclick={zurueck}>{t.zurueck}</button>{/if}
        <button class="knopf visionaer" onclick={weiter} disabled={!fertig[schritt]}>{t.weiter}</button>
      </div>
    {:else if schritt === 3}
      <div class="knoepfe"><button class="knopf" onclick={zurueck}>{t.zurueck}</button></div>
    {/if}
  </div>
</section>

<style>
  .seite { padding-top: clamp(48px, 7vw, 88px); display: grid; gap: 32px; max-width: 880px; }
  .seite > * { min-width: 0; }
  h1 { font-size: clamp(2.2rem, 4.5vw, 3.4rem); margin: 0 0 16px; }
  h2 { font-size: 1.4rem; margin: 0; }
  h3 { font-size: 1.1rem; margin: 12px 0 0; }
  .lead { font-size: 1.15rem; color: var(--stahl); max-width: 58ch; margin: 0; }
  .klein { font-size: .88rem; }
  .leiste { list-style: none; padding: 0; margin: 0; display: flex; flex-wrap: wrap; gap: 8px 20px; font-size: .92rem; color: var(--leise); }
  .leiste li { display: inline-flex; align-items: center; gap: 8px; }
  .leiste span { width: 26px; height: 26px; border-radius: 50%; border: 1px solid var(--linie-hell); display: inline-grid; place-items: center; font: 500 .8rem var(--mono); }
  .leiste .jetzt { color: var(--text); font-weight: 500; }
  .leiste .jetzt span { background: var(--vision); border-color: var(--vision); color: var(--auf-vision); }
  .leiste .erledigt span { border-color: var(--vision); color: var(--vision); }
  .karte { border: 1px solid var(--linie); border-radius: var(--radius); background: var(--metall); padding: clamp(20px, 4vw, 32px); display: grid; grid-template-columns: minmax(0, 1fr); gap: 16px; }
  .karte p { margin: 0; }
  .wahl { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr)); gap: 12px; }
  .wahl button { text-align: left; display: grid; gap: 6px; padding: 14px 16px; border-radius: var(--radius-klein); border: 1px solid var(--linie-hell);
                 background: var(--graphit-2); color: var(--text); font: inherit; cursor: pointer; }
  .wahl button:hover { border-color: var(--stahl); }
  .wahl button.an { border-color: var(--vision); box-shadow: 0 0 0 1px var(--vision) inset; }
  .wahl small { color: var(--stahl); font-size: .86rem; }
  .marke { font: 500 .72rem var(--mono); border: 1px solid var(--vision); color: var(--vision); border-radius: 5px; padding: 1px 6px; margin-left: 4px; vertical-align: 1px; }
  .tipp { border-left: 3px solid var(--vision); padding: 4px 0 4px 12px; color: var(--stahl); font-size: .93rem; }
  label { display: grid; gap: 4px; font-size: .92rem; }
  .schalter { display: flex; align-items: center; gap: 10px; font-size: 1rem; margin-top: 8px; }
  .felder { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 260px), 1fr)); gap: 14px 18px; }
  .breit { grid-column: 1 / -1; }
  input[type="text"], input[type="url"], select, textarea { min-height: 40px; padding: 8px 10px; border-radius: 6px; border: 1px solid var(--linie-hell);
    background: var(--graphit-2); color: var(--text); font: 400 .95rem var(--schrift); width: 100%; box-sizing: border-box; }
  .adresse { display: flex; align-items: center; gap: 2px; min-width: 0; }
  .adresse .leise { font: .85rem var(--mono); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; flex-shrink: 1; min-width: 0; }
  .adresse input { flex: 1 0 120px; }
  .falsch input, input.falsch { border-color: var(--rot); }
  .uebersicht { display: grid; gap: 10px; margin: 0; }
  .uebersicht div { display: grid; grid-template-columns: 130px minmax(0, 1fr); gap: 12px; }
  .uebersicht dt { color: var(--leise); font-size: .88rem; }
  .uebersicht dd { margin: 0; overflow-wrap: anywhere; }
  .link { background: none; border: 0; padding: 0; color: var(--vision); font: inherit; font-size: .85rem; cursor: pointer; text-decoration: underline; margin-left: 6px; }
  .befehle { margin: 0; padding-left: 22px; display: grid; grid-template-columns: minmax(0, 1fr); gap: 14px; }
  .befehle li { display: grid; grid-template-columns: minmax(0, 1fr); gap: 6px; min-width: 0; }
  .auftrag { display: grid; gap: 8px; justify-items: start; }
  .auftrag pre { margin: 0; white-space: pre-wrap; font: .85rem/1.5 var(--mono); border: 1px solid var(--linie-hell); border-radius: var(--radius-klein); background: var(--graphit-2); padding: 12px 14px; width: 100%; box-sizing: border-box; }
  .knoepfe { display: flex; flex-wrap: wrap; gap: 10px; }
  button.knopf[disabled] { opacity: .5; cursor: not-allowed; }
  .fehler { color: var(--rot); }
  .fehler:empty { display: none; }
  @media (max-width: 560px) { .uebersicht div { grid-template-columns: 1fr; gap: 2px; } }
</style>
