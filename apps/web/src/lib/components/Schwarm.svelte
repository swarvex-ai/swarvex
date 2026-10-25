<script lang="ts">
  // Die Grafik im Hero erzählt den Ablauf der Plattform im Kleinen, ruhig und in Schleife:
  // Der Lead gibt eine Aufgabe aus dem Projekt-Sechseck aus → ein Agent fliegt von außen heran und
  // dockt an → er arbeitet (Balken) → die Prüfung ist grün (Haken) → er bringt den Beitrag ins Projekt
  // (der Rand des Sechsecks füllt sich als Fortschritt, voll = Meilenstein) → er fliegt heim.
  // Sechs Plätze laufen versetzt. Dahinter ein leiser Matrix-Regen. Bei „weniger Bewegung“ steht das Bild.
  import { onMount } from "svelte";

  let leinwand: HTMLCanvasElement;

  onMount(() => {
    const ctx = leinwand.getContext("2d")!;
    const still = matchMedia("(prefers-reduced-motion: reduce)").matches;
    const stil = getComputedStyle(document.documentElement);
    const farbe = (n: string) => stil.getPropertyValue(n).trim();
    const AKZENT = farbe("--akzent"), STAHL = farbe("--stahl"), LINIE = farbe("--linie-hell"), GRUND = farbe("--graphit-3");
    const ZEICHEN = "0123456789ABCDEF{}[]<>/=+*#;:";
    const PLAETZE = 6;

    // Phasen eines Platzes mit Dauer in Sekunden (Spannen werden zufällig gewählt)
    type Phase = "pause" | "ausgabe" | "warten" | "anflug" | "arbeit" | "geprueft" | "abgabe" | "heimweg";
    const DAUER: Record<Phase, [number, number]> = {
      pause: [1.2, 3.5], ausgabe: [1.1, 1.1], warten: [0.8, 2.6], anflug: [2.6, 3.2],
      arbeit: [4.0, 6.5], geprueft: [0.9, 0.9], abgabe: [1.6, 1.6], heimweg: [2.4, 2.8],
    };
    const FOLGE: Record<Phase, Phase> = {
      pause: "ausgabe", ausgabe: "warten", warten: "anflug", anflug: "arbeit",
      arbeit: "geprueft", geprueft: "abgabe", abgabe: "heimweg", heimweg: "pause",
    };
    type Platz = { phase: Phase; t: number; dauer: number; heim: { x: number; y: number }; ton: number };

    let b = 0, h = 0, dpr = 1, laeuft = true, sichtbar = true, beendet = false;
    let regen: { x: number; y: number; v: number; z: string[] }[] = [];
    let plaetze: Platz[] = [];
    let fortschritt = 0.35, blitz = 0, puls = 0;

    const zufall = (a: number, z: number) => a + Math.random() * (z - a);
    const weich = (t: number) => (t < 0.5 ? 4 * t * t * t : 1 - (-2 * t + 2) ** 3 / 2); // ease-in-out cubic
    const mitte = () => ({ x: b * 0.5, y: h * 0.5 });
    const radius = () => Math.min(b, h);
    const platzPos = (i: number) => {
      const w = -Math.PI / 2 + Math.PI / PLAETZE + (i / PLAETZE) * Math.PI * 2;
      const m = mitte(), r = radius() * 0.36;
      return { x: m.x + Math.cos(w) * r * 1.2, y: m.y + Math.sin(w) * r * 0.95, w };
    };
    const heimFuer = (i: number) => {
      const p = platzPos(i), m = mitte(), w = p.w + zufall(-0.35, 0.35), r = Math.max(b, h) * 0.75;
      return { x: m.x + Math.cos(w) * r, y: m.y + Math.sin(w) * r };
    };
    const dockPos = (i: number) => {
      const p = platzPos(i), m = mitte(), dx = p.x - m.x, dy = p.y - m.y, l = Math.hypot(dx, dy) || 1;
      return { x: p.x + (dx / l) * 34, y: p.y + (dy / l) * 26 };
    };
    // Bahn als quadratische Kurve mit leichtem Bogen
    const bahn = (a: { x: number; y: number }, z: { x: number; y: number }, k: number, bogen = 0.15) => {
      const cx = (a.x + z.x) / 2 + (a.y - z.y) * bogen, cy = (a.y + z.y) / 2 - (a.x - z.x) * bogen;
      return { x: (1 - k) ** 2 * a.x + 2 * (1 - k) * k * cx + k * k * z.x, y: (1 - k) ** 2 * a.y + 2 * (1 - k) * k * cy + k * k * z.y };
    };

    function phaseSetzen(p: Platz, phase: Phase, i: number) {
      p.phase = phase; p.t = 0; p.dauer = zufall(...DAUER[phase]);
      if (phase === "anflug") { p.heim = heimFuer(i); p.ton = zufall(0.6, 1); }
    }

    function groesse() {
      const r = leinwand.getBoundingClientRect();
      dpr = Math.min(devicePixelRatio || 1, 2);
      b = r.width; h = r.height;
      leinwand.width = Math.round(b * dpr); leinwand.height = Math.round(h * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      regen = Array.from({ length: Math.floor(b / 24) }, (_, i) => ({ x: i * 24 + 12, y: Math.random() * h, v: zufall(6, 16),
        z: Array.from({ length: 12 }, () => ZEICHEN[Math.floor(Math.random() * ZEICHEN.length)]) })).filter(() => Math.random() < 0.28);
      // Startzustand: Plätze in unterschiedlichen Phasen, damit sofort etwas zu sehen ist
      const start: Phase[] = ["arbeit", "warten", "anflug", "pause", "arbeit", "heimweg"];
      plaetze = Array.from({ length: PLAETZE }, (_, i) => {
        const p: Platz = { phase: "pause", t: 0, dauer: 1, heim: heimFuer(i), ton: zufall(0.6, 1) };
        phaseSetzen(p, start[i % start.length], i);
        p.t = p.dauer * zufall(0.1, 0.7);
        return p;
      });
      if (still) zeichnen(0);
    }

    function sechseckPfad(x: number, y: number, r: number) {
      ctx.beginPath();
      for (let i = 0; i < 6; i++) { const w = -Math.PI / 2 + (i * Math.PI) / 3; ctx[i ? "lineTo" : "moveTo"](x + r * Math.cos(w), y + r * Math.sin(w)); }
      ctx.closePath();
    }
    // Rand des Sechsecks anteilig nachzeichnen (Projektfortschritt), Start oben, im Uhrzeigersinn
    function sechseckAnteil(x: number, y: number, r: number, anteil: number) {
      const ecken = Array.from({ length: 7 }, (_, i) => { const w = -Math.PI / 2 + (i * Math.PI) / 3; return { x: x + r * Math.cos(w), y: y + r * Math.sin(w) }; });
      let rest = anteil * 6;
      ctx.beginPath(); ctx.moveTo(ecken[0].x, ecken[0].y);
      for (let i = 0; i < 6 && rest > 0; i++) {
        const k = Math.min(1, rest);
        ctx.lineTo(ecken[i].x + (ecken[i + 1].x - ecken[i].x) * k, ecken[i].y + (ecken[i + 1].y - ecken[i].y) * k);
        rest -= 1;
      }
    }

    function karte(x: number, y: number, a: number, zustand: "offen" | "arbeit" | "geprueft", fort = 0) {
      ctx.globalAlpha = a;
      ctx.fillStyle = GRUND; ctx.strokeStyle = zustand === "offen" ? LINIE : AKZENT; ctx.lineWidth = 1.3;
      ctx.beginPath(); ctx.roundRect(x - 22, y - 14, 44, 28, 5); ctx.fill(); ctx.stroke();
      ctx.strokeStyle = STAHL; ctx.globalAlpha = a * 0.75; ctx.lineWidth = 1.5;
      ctx.beginPath(); ctx.moveTo(x - 14, y - 5); ctx.lineTo(x + 10, y - 5); ctx.moveTo(x - 14, y + 1); ctx.lineTo(x + 4, y + 1); ctx.stroke();
      if (zustand !== "offen") { // Fortschrittsbalken unten in der Karte
        ctx.globalAlpha = a * 0.35; ctx.fillStyle = STAHL; ctx.fillRect(x - 14, y + 7, 28, 2.5);
        ctx.globalAlpha = a; ctx.fillStyle = AKZENT; ctx.fillRect(x - 14, y + 7, 28 * fort, 2.5);
      }
      if (zustand === "geprueft") { // Haken über der Karte
        ctx.strokeStyle = AKZENT; ctx.lineWidth = 2.2; ctx.lineCap = "round";
        ctx.beginPath(); ctx.moveTo(x + 14, y - 24); ctx.lineTo(x + 18, y - 20); ctx.lineTo(x + 26, y - 29); ctx.stroke(); ctx.lineCap = "butt";
      }
      ctx.globalAlpha = 1;
    }

    function agent(x: number, y: number, a: number, ton: number, arbeitet = false, zeit = 0) {
      const s = 11, wipp = arbeitet ? Math.sin(zeit * 5) * 1.2 : 0;
      ctx.globalAlpha = a;
      ctx.fillStyle = `rgba(169,180,192,${ton})`;
      ctx.beginPath(); ctx.roundRect(x - s / 2, y - s * 0.4 + wipp, s, s * 0.8, 2.5); ctx.fill();
      ctx.fillStyle = AKZENT;
      ctx.fillRect(x - s * 0.24, y - s * 0.1 + wipp, 2, 2); ctx.fillRect(x + s * 0.1, y - s * 0.1 + wipp, 2, 2);
      ctx.globalAlpha = 1;
    }

    function zeichnen(dt: number) {
      ctx.clearRect(0, 0, b, h);
      const zeit = performance.now() / 1000;

      // Matrix-Regen, sehr leise und langsam
      ctx.font = "13px 'IBM Plex Mono', monospace"; ctx.textAlign = "center"; ctx.fillStyle = AKZENT;
      for (const r of regen) {
        r.y = (r.y + r.v * dt) % (h + 240);
        r.z.forEach((z, i) => {
          const y = r.y - i * 18;
          if (y < -10 || y > h + 10) return;
          ctx.globalAlpha = i === 0 ? 0.18 : 0.07 * (1 - i / r.z.length);
          ctx.fillText(z, r.x, y);
        });
        if (Math.random() < 0.004) r.z[Math.floor(Math.random() * r.z.length)] = ZEICHEN[Math.floor(Math.random() * ZEICHEN.length)];
      }
      ctx.globalAlpha = 1;

      const m = mitte(), hr = radius() * 0.13;

      // feine Verbindungslinien Projekt → Plätze
      ctx.strokeStyle = LINIE; ctx.globalAlpha = 0.35; ctx.lineWidth = 1; ctx.setLineDash([2, 6]);
      for (let i = 0; i < PLAETZE; i++) { const p = platzPos(i); ctx.beginPath(); ctx.moveTo(m.x, m.y); ctx.lineTo(p.x, p.y); ctx.stroke(); }
      ctx.setLineDash([]); ctx.globalAlpha = 1;

      // Plätze: Zustand weiterschalten und zeichnen
      for (let i = 0; i < PLAETZE; i++) {
        const p = plaetze[i], ort = platzPos(i), dock = dockPos(i);
        p.t += dt;
        if (p.t >= p.dauer) {
          if (p.phase === "abgabe") { puls = 1; fortschritt += 0.045; if (fortschritt >= 1) { blitz = 1; fortschritt = 0.08; } }
          phaseSetzen(p, FOLGE[p.phase], i);
        }
        const k = Math.min(1, p.t / p.dauer), w = weich(k);
        switch (p.phase) {
          case "ausgabe": { const q = bahn(m, ort, w, 0); karte(q.x, q.y, Math.min(1, k * 2), "offen"); break; }
          case "warten": karte(ort.x, ort.y, 1, "offen"); break;
          case "anflug": { karte(ort.x, ort.y, 1, "offen"); const q = bahn(p.heim, dock, w); agent(q.x, q.y, Math.min(1, k * 3), p.ton); break; }
          case "arbeit": karte(ort.x, ort.y, 1, "arbeit", w); agent(dock.x, dock.y, 1, p.ton, true, zeit + i); break;
          case "geprueft": karte(ort.x, ort.y, 1, "geprueft", 1); agent(dock.x, dock.y, 1, p.ton); break;
          case "abgabe": {
            const q = bahn(ort, m, w, 0.05), a = 1 - Math.max(0, (k - 0.75) / 0.25);
            karte(q.x, q.y, a, "geprueft", 1); agent(q.x + (dock.x - ort.x) * (1 - w), q.y + (dock.y - ort.y) * (1 - w), a, p.ton);
            break;
          }
          case "heimweg": { const q = bahn(m, p.heim, w, -0.12); agent(q.x, q.y, Math.min(1, (1 - k) * 2.5) * Math.min(1, k * 4), p.ton); break; }
        }
      }

      // Projekt-Sechseck: ruhiger Grund, Rand füllt sich als Fortschritt, Puls bei jeder Abgabe
      puls = Math.max(0, puls - dt * 1.4); blitz = Math.max(0, blitz - dt * 0.8);
      sechseckPfad(m.x, m.y, hr); ctx.fillStyle = `rgba(62,230,208,${0.04 + puls * 0.08 + blitz * 0.15})`; ctx.fill();
      ctx.strokeStyle = LINIE; ctx.lineWidth = 2; ctx.stroke();
      ctx.save(); ctx.shadowColor = AKZENT; ctx.shadowBlur = 10 + puls * 18 + blitz * 40;
      sechseckAnteil(m.x, m.y, hr, blitz > 0 ? 1 : fortschritt); ctx.strokeStyle = AKZENT; ctx.lineWidth = 2.5; ctx.stroke();
      ctx.restore();
      sechseckPfad(m.x, m.y, hr * 0.55); ctx.strokeStyle = STAHL; ctx.globalAlpha = 0.3; ctx.lineWidth = 1; ctx.stroke(); ctx.globalAlpha = 1;
      // der Lead in der Mitte (kleiner Kopf mit zwei Augen)
      ctx.fillStyle = STAHL; ctx.globalAlpha = 0.85; ctx.beginPath(); ctx.roundRect(m.x - 9, m.y - 7, 18, 14, 3.5); ctx.fill(); ctx.globalAlpha = 1;
      ctx.fillStyle = AKZENT; ctx.fillRect(m.x - 5, m.y - 2, 2.5, 2.5); ctx.fillRect(m.x + 2.5, m.y - 2, 2.5, 2.5);
    }

    let zuletzt = performance.now();
    function schleife(jetzt: number) {
      if (beendet) return;
      const dt = Math.min(0.05, (jetzt - zuletzt) / 1000); // Sprünge nach Tab-Wechsel begrenzen
      zuletzt = jetzt;
      if (laeuft && sichtbar) zeichnen(dt);
      requestAnimationFrame(schleife);
    }

    groesse();
    const ro = new ResizeObserver(groesse); ro.observe(leinwand);
    const io = new IntersectionObserver(([e]) => (laeuft = e.isIntersecting)); io.observe(leinwand);
    const sicht = () => (sichtbar = !document.hidden); document.addEventListener("visibilitychange", sicht);
    if (!still) requestAnimationFrame(schleife);
    return () => { ro.disconnect(); io.disconnect(); document.removeEventListener("visibilitychange", sicht); beendet = true; };
  });
</script>

<canvas bind:this={leinwand} aria-hidden="true"></canvas>

<style>
  canvas { display: block; width: 100%; height: 100%; }
</style>
