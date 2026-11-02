// GitHub App (src/github.ts): Adressen, Token-Header für git, App-JWT; ohne Netz.
import { generateKeyPairSync, createVerify } from "node:crypto";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const { privateKey, publicKey } = generateKeyPairSync("rsa", { modulusLength: 2048 });
const ordner = mkdtempSync(join(tmpdir(), "gh-"));
writeFileSync(join(ordner, "key.pem"), privateKey.export({ type: "pkcs8", format: "pem" }));
process.env.GITHUB_APP_ID = "123";
process.env.GITHUB_APP_NAME = "swarvex-test";
process.env.GITHUB_APP_KEY_FILE = join(ordner, "key.pem");
const gh = await import("../src/github.ts");

describe("GitHub App", () => {
  it("liest nur GitHub-Adressen", () => {
    expect(gh.repoAus("https://github.com/nevs84/mein-spiel")).toEqual({ owner: "nevs84", repo: "mein-spiel" });
    expect(gh.repoAus("https://github.com/nevs84/mein.spiel.git")).toEqual({ owner: "nevs84", repo: "mein.spiel" });
    expect(gh.repoAus("https://codeberg.org/a/b")).toBeNull();
    expect(gh.repoAus("https://github.com/a/b/../c")).toBeNull();
    expect(gh.repoAus("git@github.com:a/b.git")).toBeNull();
  });
  it("gibt den Token als Header für github.com mit, nicht in der Adresse", () => {
    const u = gh.gitTokenUmgebung("ghs_abc");
    expect(u.GIT_CONFIG_KEY_0).toBe("http.https://github.com/.extraheader");
    expect(Buffer.from(u.GIT_CONFIG_VALUE_0.split(" ").pop()!, "base64").toString()).toBe("x-access-token:ghs_abc");
  });
  it("ist bereit und baut ein gültiges App-JWT", async () => {
    expect(gh.appBereit()).toBe(true);
    expect(gh.installierenUrl("de.starten")).toBe("https://github.com/apps/swarvex-test/installations/new?state=de.starten");
    let jwt = "";
    const alt = globalThis.fetch;
    globalThis.fetch = (async (_u: string, init: RequestInit) => { jwt = String((init.headers as Record<string, string>).authorization).replace("Bearer ", "");
      return new Response("{}", { status: 404 }); }) as typeof fetch;
    try { expect(await gh.repoPruefen("https://github.com/a/b", "x")).toEqual({ fehler: "app_not_installed" }); } finally { globalThis.fetch = alt; }
    const [k, i, sig] = jwt.split(".");
    expect(createVerify("RSA-SHA256").update(`${k}.${i}`).verify(publicKey, sig, "base64url")).toBe(true);
    const inhalt = JSON.parse(Buffer.from(i, "base64url").toString());
    expect(inhalt.iss).toBe("123");
    expect(inhalt.exp - inhalt.iat).toBeLessThanOrEqual(600);
  });
});
