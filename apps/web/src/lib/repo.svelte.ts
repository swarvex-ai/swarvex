// Sterne des Quellcode-Repos: einmal pro Seitenaufruf von unserer API (/api/repo), nie direkt von GitHub.
export const repo = $state<{ stars: number | null }>({ stars: null });

let geladen = false;
export function repoLaden() {
  if (geladen) return;
  geladen = true;
  fetch("/api/repo")
    .then((r) => (r.ok ? r.json() : null))
    .then((d) => { if (d && typeof d.stars === "number") repo.stars = d.stars; })
    .catch(() => {});
}
