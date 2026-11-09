<script lang="ts">
  // Fehlerseite der ganzen Website (statt der nackten Standardseite von SvelteKit): kurz sagen, was los ist, und wohin
  // es weitergeht. Ein nicht öffentliches Profil (mitstreiter/[login]) bekommt einen eigenen, freundlichen Text.
  import { page } from "$app/state";

  const en = $derived(page.url.pathname === "/en" || page.url.pathname.startsWith("/en/"));
  const profil = $derived(page.status === 404 && page.error?.message === "no public profile");
  const login = $derived(page.params.login ?? "");
  const t = $derived(en
    ? {
        profilH1: "This profile is not public",
        profilText: (l: string) => `Everyone on swarvex decides for themselves whether their contributions appear on a page of their own. ${l || "This person"} has not switched that on. The work still counts: it is in the projects' logs.`,
        eigenes: "Your own profile: switch it on under My account",
        fehltH1: "This page does not exist",
        fehltText: "Perhaps the address has changed, or the link was cut off.",
        andersH1: "Something went wrong",
        andersText: "Please try again in a moment.",
        start: "Home", projekte: "Projects", doku: "Docs", konto: "My account",
        pfade: { start: "/en", projekte: "/en/projects", doku: "/en/docs", konto: "/en/account" },
      }
    : {
        profilH1: "Dieses Profil ist nicht öffentlich",
        profilText: (l: string) => `Wer bei swarvex mitmacht, entscheidet selbst, ob die eigenen Beiträge auf einer eigenen Seite stehen. ${l || "Diese Person"} hat das nicht eingeschaltet. Die Beiträge zählen trotzdem: Sie stehen im Protokoll der Projekte.`,
        eigenes: "Dein eigenes Profil: unter Mein Konto einschalten",
        fehltH1: "Diese Seite gibt es nicht",
        fehltText: "Vielleicht hat sich die Adresse geändert, oder der Link war unvollständig.",
        andersH1: "Da ist etwas schiefgegangen",
        andersText: "Versuch es gleich noch einmal.",
        start: "Startseite", projekte: "Projekte", doku: "Doku", konto: "Mein Konto",
        pfade: { start: "/", projekte: "/projekte", doku: "/docs", konto: "/konto" },
      });
</script>

<svelte:head>
  <title>{profil ? t.profilH1 : page.status === 404 ? t.fehltH1 : t.andersH1} · swarvex.ai</title>
  <meta name="robots" content="noindex" />
</svelte:head>

<section class="huelle fehler">
  {#if profil}
    <p class="zeichen" aria-hidden="true">◌</p>
    <h1>{t.profilH1}</h1>
    <p class="text">{t.profilText(login)}</p>
    <div class="wege">
      <a class="knopf primaer" href={t.pfade.projekte}>{t.projekte}</a>
      <a class="knopf" href={t.pfade.konto}>{t.eigenes}</a>
    </div>
  {:else}
    <p class="zeichen" aria-hidden="true">{page.status === 404 ? "?" : "!"}</p>
    <h1>{page.status === 404 ? t.fehltH1 : t.andersH1}</h1>
    <p class="text">{page.status === 404 ? t.fehltText : t.andersText}</p>
    <div class="wege">
      <a class="knopf primaer" href={t.pfade.start}>{t.start}</a>
      <a class="knopf" href={t.pfade.projekte}>{t.projekte}</a>
      <a class="knopf" href={t.pfade.doku}>{t.doku}</a>
    </div>
    <p class="leise klein">{page.status}</p>
  {/if}
</section>

<style>
  .fehler { min-height: 55vh; display: grid; align-content: center; justify-items: center; gap: 14px; padding-top: 48px; padding-bottom: 64px; text-align: center; }
  .zeichen { margin: 0; width: 64px; height: 64px; display: grid; place-items: center; border: 1px solid var(--linie); border-radius: 50%; color: var(--akzent); font-size: 1.8rem; }
  h1 { margin: 0; font-size: clamp(1.6rem, 4vw, 2.4rem); }
  .text { margin: 0; max-width: 56ch; color: var(--leise); }
  .wege { display: flex; flex-wrap: wrap; justify-content: center; gap: 10px; margin-top: 8px; }
  .klein { font-size: .85rem; margin: 6px 0 0; }
</style>
