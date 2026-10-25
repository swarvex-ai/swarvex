<script lang="ts">
  // Zahl, die bei neuen Live-Werten sichtbar hochzählt (Bewegung zeigt, was sich geändert hat).
  import { Tween } from "svelte/motion";
  import { cubicOut } from "svelte/easing";
  import { zahl } from "$lib/live.svelte";
  import { sprache } from "$lib/texte";
  let { wert }: { wert: number } = $props();
  const t = new Tween(0, { duration: 700, easing: cubicOut });
  $effect(() => { t.target = wert; });
</script>

<span class="zaehler">{zahl(Math.round(t.current), sprache())}</span>

<style>.zaehler { font-variant-numeric: tabular-nums; }</style>
