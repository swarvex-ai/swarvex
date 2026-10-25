import { seiten } from "$lib/server/doku";

// Die ganze Doku als ein Textdokument, z. B. für Agenten: curl https://swarvex.ai/en/llms-full.txt
export const prerender = true;
export const entries = () => [{ lang: undefined }, { lang: "en" as const }];
export const GET = ({ params }) => {
  const en = params.lang === "en";
  const basis = `https://swarvex.ai${en ? "/en" : ""}/docs/`;
  const text = seiten(en ? "en" : "de").map((s) => `# ${s.titel}\n\n> ${s.beschreibung}\n> ${en ? "Source" : "Quelle"}: ${basis}${s.slug}\n\n${s.markdown.trim()}\n`).join("\n\n---\n\n");
  const kopf = en ? "# swarvex.ai docs" : "# swarvex.ai Doku";
  return new Response(`${kopf}\n\nthe coordination layer for agent-built software\n\n${text}`, { headers: { "content-type": "text/plain; charset=utf-8" } });
};
