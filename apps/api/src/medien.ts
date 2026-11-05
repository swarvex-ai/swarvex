// Inhalt einer hochgeladenen Medien-Datei: Passt der Anfang zur Endung? Eine private Datei (Schlüssel, Notizen) kommt so
// nicht als „Modell“ oder „Bild“ durch, auch wenn ein manipulierter Auftrag den Agenten dazu bringt, sie abzugeben.
// Der Runner prüft dasselbe vor dem Hochladen (runner/medien.go).

/** Über den Runner (Zusage über den Runner) nur Typen mit prüfbarem Inhalt: keine Texte, Archive, SVG oder glTF-JSON. */
export const RUNNER_TYPEN = ["glb", "fbx", "obj", "png", "jpg", "jpeg", "webp", "mp4", "webm", "mov", "wav", "mp3", "ogg", "flac"];

const ascii = (kopf: Buffer, ab: number, text: string) => kopf.subarray(ab, ab + text.length).toString("latin1") === text;

const OBJ_ZEILE = /^\s*(#.*|(v|vt|vn|vp|f|l|p|o|g|s|usemtl|mtllib)(\s.*)?)?$/;

/** true: der Anfang der Datei (mindestens die ersten 64 KB) passt zur Endung; unbekannte Endungen: null (keine Prüfung). */
export function inhaltPasst(endung: string, kopf: Buffer): boolean | null {
  switch (endung) {
    case "glb": return ascii(kopf, 0, "glTF");
    case "fbx": return ascii(kopf, 0, "Kaydara FBX Binary") || ascii(kopf, 0, "; FBX");
    case "png": return kopf.subarray(0, 8).equals(Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]));
    case "jpg": case "jpeg": return kopf[0] === 0xff && kopf[1] === 0xd8 && kopf[2] === 0xff;
    case "webp": return ascii(kopf, 0, "RIFF") && ascii(kopf, 8, "WEBP");
    case "wav": return ascii(kopf, 0, "RIFF") && ascii(kopf, 8, "WAVE");
    case "ogg": return ascii(kopf, 0, "OggS");
    case "flac": return ascii(kopf, 0, "fLaC");
    case "mp3": return ascii(kopf, 0, "ID3") || (kopf[0] === 0xff && (kopf[1] & 0xe0) === 0xe0);
    case "webm": return kopf.subarray(0, 4).equals(Buffer.from([0x1a, 0x45, 0xdf, 0xa3]));
    case "mp4": return ascii(kopf, 4, "ftyp");
    case "mov": return ["ftyp", "moov", "mdat", "wide", "free", "skip", "pnot"].some((a) => ascii(kopf, 4, a));
    case "obj": {
      // Text aus OBJ-Anweisungen (Ecken, Flächen, Gruppen, Materialien); die letzte Zeile kann abgeschnitten sein
      const text = kopf.toString("utf8");
      if (text.includes("\u0000") || text.includes("�")) return false;
      const zeilen = text.split(/\r?\n/);
      if (kopf.length >= 65536) zeilen.pop();
      return zeilen.some((z) => /^\s*(v|f)\s/.test(z)) && zeilen.every((z) => OBJ_ZEILE.test(z));
    }
    default: return null;
  }
}
