<script lang="ts">
  // Die Open-Source-Flagge: ein Tuch am Mast, das im Wind weht. Das Motiv (Logo-Symbol und
  // „OPEN SOURCE“) wird einmal in ein Offscreen-Canvas gemalt und dann in schmalen Streifen mit
  // einer Welle versetzt gezeichnet; Licht und Schatten folgen der Neigung der Welle.
  // Pausiert außerhalb des Sichtbereichs; bei reduzierter Bewegung ein ruhiges Standbild.
  import { onMount } from "svelte";

  let { label = "Open Source", zeile1 = "OPEN", zeile2 = "SOURCE" }: { label?: string; zeile1?: string; zeile2?: string } = $props();

  let leinwand: HTMLCanvasElement;
  let huelle: HTMLDivElement;

  const STROEME: [number, string][] = [[-45, "#22d3ee"], [-135, "#ec4899"], [135, "#fbbf24"], [45, "#34d399"]];

  function motiv(b: number, h: number, dpr: number): HTMLCanvasElement {
    const c = document.createElement("canvas");
    c.width = Math.round(b * dpr);
    c.height = Math.round(h * dpr);
    const g = c.getContext("2d")!;
    g.scale(dpr, dpr);
    // Tuch: tiefes Graphit mit leichtem Verlauf, feine Kante
    const v = g.createLinearGradient(0, 0, b, h);
    v.addColorStop(0, "#151b22");
    v.addColorStop(1, "#0b0f14");
    g.fillStyle = v;
    g.fillRect(0, 0, b, h);
    // Webstruktur: sehr feine Linien
    g.globalAlpha = 0.018;
    g.fillStyle = "#ffffff";
    for (let y = 0; y < h; y += 3) g.fillRect(0, y, b, 1);
    g.globalAlpha = 1;
    // Akzentstreifen am Mast
    const s = g.createLinearGradient(0, h, 0, 0);
    s.addColorStop(0, "#8b5cf6");
    s.addColorStop(1, "#22d3ee");
    g.fillStyle = s;
    g.fillRect(0, 0, b * 0.035, h);
    // Logo-Symbol: Sechseck mit vier Strömen
    const mx = b * 0.24, my = h * 0.5, r = h * 0.105;
    g.save();
    g.translate(mx, my);
    for (const [w, farbe] of STROEME) {
      const rad = (w * Math.PI) / 180;
      [[1.72, 0.36], [2.31, 0.29], [2.83, 0.23], [3.28, 0.17]].forEach(([d, pr]) => {
        g.beginPath();
        g.arc(Math.cos(rad) * r * d, -Math.sin(rad) * r * d, r * pr, 0, Math.PI * 2);
        g.fillStyle = farbe;
        g.fill();
      });
    }
    const hg = g.createLinearGradient(-r, r, r, -r);
    hg.addColorStop(0, "#8b5cf6");
    hg.addColorStop(1, "#22d3ee");
    g.beginPath();
    for (let i = 0; i < 6; i++) {
      const a = Math.PI / 6 + (i * Math.PI) / 3;
      g[i ? "lineTo" : "moveTo"](Math.cos(a) * r, Math.sin(a) * r);
    }
    g.closePath();
    g.fillStyle = hg;
    g.fill();
    g.restore();
    // Schrift
    const schrift = getComputedStyle(document.documentElement).getPropertyValue("--schrift-marke") || "system-ui";
    let groesse = h * 0.25;
    g.font = `700 ${groesse}px ${schrift}`;
    const platz = b * 0.5;
    const breit = Math.max(g.measureText(zeile1).width, g.measureText(zeile2).width);
    if (breit > platz) groesse *= platz / breit;
    g.font = `700 ${groesse}px ${schrift}`;
    g.fillStyle = "#eef0ff";
    g.textBaseline = "alphabetic";
    g.fillText(zeile1, b * 0.43, h * 0.5 - groesse * 0.08);
    g.fillStyle = "#3ee6d0";
    g.fillText(zeile2, b * 0.43, h * 0.5 + groesse * 0.92);
    return c;
  }

  onMount(() => {
    const ctx = leinwand.getContext("2d")!;
    const ruhig = matchMedia("(prefers-reduced-motion: reduce)").matches;
    let bild: HTMLCanvasElement | null = null;
    let b = 0, h = 0, dpr = 1, laeuft = false, sichtbar = false, rahmen = 0, beendet = false;
    let t = Math.random() * 10, zuletzt = 0;

    // Maße: Mast links, Tuch im Verhältnis 3:2
    const mast = () => Math.max(6, b * 0.016);
    const tuch = () => ({ x: mast() + b * 0.01, y: h * 0.12, b: b - mast() - b * 0.08, h: (b - mast() - b * 0.08) / 1.5 });

    function groesse() {
      const r = huelle.getBoundingClientRect();
      dpr = Math.min(2, devicePixelRatio || 1);
      b = r.width;
      h = r.height;
      leinwand.width = Math.round(b * dpr);
      leinwand.height = Math.round(h * dpr);
      const f = tuch();
      bild = motiv(f.b, f.h, dpr);
      zeichnen();
    }

    function zeichnen() {
      if (!bild) return;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, b, h);
      const f = tuch();
      // Mast mit Knauf
      const mg = ctx.createLinearGradient(0, 0, mast(), 0);
      mg.addColorStop(0, "#5b6877");
      mg.addColorStop(0.5, "#c3ccd5");
      mg.addColorStop(1, "#4b5866");
      ctx.fillStyle = mg;
      ctx.fillRect(0, f.y - h * 0.06, mast(), h);
      ctx.beginPath();
      ctx.arc(mast() / 2, f.y - h * 0.06, mast() * 0.9, 0, Math.PI * 2);
      ctx.fillStyle = "#c3ccd5";
      ctx.fill();
      // Tuch in Streifen: Welle wächst vom Mast nach außen, Böen ändern die Stärke
      const boe = 0.75 + 0.25 * Math.sin(t * 0.37) + 0.12 * Math.sin(t * 1.3);
      const amp = f.h * 0.075 * boe;
      const streifen = Math.max(2, Math.round(b / 260));
      const quelleB = bild.width / f.b;
      for (let x = 0; x < f.b; x += streifen) {
        const u = x / f.b;
        const phase = u * 7.5 - t * 3.1;
        const welle = Math.sin(phase) + 0.35 * Math.sin(phase * 2.1 + 1.3);
        const dy = amp * Math.pow(u, 0.85) * welle;
        const neigung = Math.cos(phase) + 0.7 * Math.cos(phase * 2.1 + 1.3);
        const stauchung = 1 - 0.04 * Math.pow(u, 0.85) * Math.abs(welle);
        const hoehe = f.h * stauchung;
        ctx.drawImage(bild, x * quelleB, 0, streifen * quelleB + 1, bild.height,
          f.x + x, f.y + dy + (f.h - hoehe) / 2, streifen + 0.6, hoehe);
        // Licht und Schatten der Falten
        const licht = neigung * Math.pow(u, 0.6) * 0.16 * boe;
        ctx.fillStyle = licht > 0 ? `rgba(255,255,255,${licht})` : `rgba(0,0,0,${-licht * 1.6})`;
        ctx.fillRect(f.x + x, f.y + dy + (f.h - hoehe) / 2, streifen + 0.6, hoehe);
      }
    }

    function schritt(jetzt: number) {
      if (beendet) return;
      const dt = Math.min(0.05, (jetzt - (zuletzt || jetzt)) / 1000);
      zuletzt = jetzt;
      t += dt;
      zeichnen();
      if (laeuft) rahmen = requestAnimationFrame(schritt);
    }

    function steuern() {
      const soll = sichtbar && !ruhig && !document.hidden;
      if (soll && !laeuft) { laeuft = true; zuletzt = 0; rahmen = requestAnimationFrame(schritt); }
      if (!soll && laeuft) { laeuft = false; cancelAnimationFrame(rahmen); }
    }

    const beobachter = new ResizeObserver(groesse);
    const blick = new IntersectionObserver(([e]) => { sichtbar = e.isIntersecting; steuern(); });
    document.addEventListener("visibilitychange", steuern);
    // Erst zeichnen, wenn die Markenschrift geladen ist, sonst steht die Ersatzschrift auf der Flagge
    document.fonts.load('700 40px "Sora Variable"').catch(() => {}).finally(() => {
      if (beendet) return;
      beobachter.observe(huelle);
      blick.observe(huelle);
    });
    return () => { beendet = true; cancelAnimationFrame(rahmen); beobachter.disconnect(); blick.disconnect(); document.removeEventListener("visibilitychange", steuern); };
  });
</script>

<div class="flagge" bind:this={huelle} role="img" aria-label={label}>
  <canvas bind:this={leinwand}></canvas>
</div>

<style>
  .flagge { position: relative; width: 100%; aspect-ratio: 4 / 3; }
  canvas { position: absolute; inset: 0; width: 100%; height: 100%; display: block; }
</style>
