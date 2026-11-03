// Vision, Kurzvision und Ziel in der Sprache der Seite: die Übersetzung, wenn das Original in der anderen Sprache
// geschrieben ist und Lead oder Visionär eine hinterlegt haben (set_translation); sonst das Original.
type Texte = { vision_md?: string; vision_short?: string; goal?: string };
export type MitUebersetzung = Texte & { language?: string; translations?: Record<string, Texte> };

export function projektText(p: MitUebersetzung, s: string) {
  const fremd = !!p.language && p.language !== s;
  const u = fremd ? p.translations?.[s] : undefined;
  return {
    vision_short: u?.vision_short ?? p.vision_short ?? "",
    vision_md: u?.vision_md ?? p.vision_md ?? "",
    goal: u?.goal ?? p.goal ?? "",
    /** die ausführliche Vision ist übersetzt (Hinweis und Link zum Original zeigen) */
    uebersetzt: !!u?.vision_md,
    /** Sprache des Originals, wenn sie nicht die der Seite ist */
    original: fremd ? p.language! : null,
  };
}
