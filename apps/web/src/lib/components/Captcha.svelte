<script lang="ts">
  // Captcha ohne Drittanbieter (Proof of Work im ALTCHA-Format, Gegenstück: apps/api/src/captcha.ts).
  // Startet von selbst, sobald das Formular sichtbar ist; ein Klick ist nur nach einem Fehler nötig.
  import { onMount } from "svelte";
  import { sha256Hex } from "$lib/sha256";

  let { texte, onloesung }: { texte: { laedt: string; prueft: string; fertig: string; fehler: string }; onloesung: (nutzlast: string) => void } = $props();

  let zustand = $state<"laedt" | "prueft" | "fertig" | "fehler">("laedt");
  let fortschritt = $state(0);

  async function loesen() {
    zustand = "laedt";
    fortschritt = 0;
    try {
      const r = await fetch("/api/reports/challenge", { cache: "no-store" });
      if (!r.ok) throw new Error(String(r.status));
      const a: { challenge: string; salt: string; algorithm: string; maxnumber: number } = await r.json();
      if (a.algorithm !== "SHA-256") throw new Error(a.algorithm);
      zustand = "prueft";
      for (let n = 0; n <= a.maxnumber; n++) {
        if (n % 5000 === 0) {
          fortschritt = Math.round((n / a.maxnumber) * 100);
          await new Promise((weiter) => setTimeout(weiter, 0)); // Seite bleibt bedienbar
        }
        if (sha256Hex(a.salt + n) === a.challenge) {
          zustand = "fertig";
          onloesung(btoa(JSON.stringify({ challenge: a.challenge, salt: a.salt, number: n, algorithm: a.algorithm })));
          return;
        }
      }
      throw new Error("unsolved");
    } catch {
      zustand = "fehler";
    }
  }

  onMount(loesen);
</script>

<button type="button" class="captcha {zustand}" disabled={zustand !== "fehler"} onclick={loesen} aria-live="polite">
  <span class="zeichen" aria-hidden="true">{zustand === "fertig" ? "✓" : zustand === "fehler" ? "!" : ""}</span>
  <span class="text">
    {zustand === "prueft" ? `${texte.prueft} ${fortschritt} %` : texte[zustand]}
    {#if zustand === "prueft"}<span class="balken" aria-hidden="true"><span style="width: {fortschritt}%"></span></span>{/if}
  </span>
  <span class="leise marke">ALTCHA</span>
</button>

<style>
  .captcha {
    display: flex; align-items: center; gap: 12px; width: 100%; min-height: 48px; padding: 10px 14px; text-align: left;
    font: inherit; color: var(--text); background: var(--metall); border: 1px solid var(--linie-hell); border-radius: var(--radius-klein);
  }
  .captcha:disabled { cursor: default; }
  .captcha.fehler { border-color: var(--rot); cursor: pointer; }
  .captcha.fertig { border-color: var(--akzent); }
  .zeichen {
    flex: none; width: 20px; height: 20px; border-radius: 50%; border: 2px solid var(--linie-hell);
    display: grid; place-items: center; font-size: .8rem; font-weight: 700;
  }
  .laedt .zeichen, .prueft .zeichen { border-top-color: var(--akzent); animation: drehen 0.8s linear infinite; }
  .fertig .zeichen { border-color: var(--akzent); color: var(--akzent); }
  .fehler .zeichen, .fehler .text { border-color: var(--rot); color: var(--rot); }
  .text { flex: 1; display: grid; gap: 6px; font-size: .95rem; }
  .balken { height: 4px; border-radius: 2px; background: var(--linie); overflow: hidden; }
  .balken span { display: block; height: 100%; background: var(--akzent); }
  .marke { font: 500 .75rem var(--mono); letter-spacing: .04em; }
  @keyframes drehen { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { .laedt .zeichen, .prueft .zeichen { animation: none; } }
</style>
