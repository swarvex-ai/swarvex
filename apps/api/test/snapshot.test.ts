// Clean Room: Der Arbeitsstand für Bau-Aufgaben enthält nur die freigegebene Spezifikation.
import { execFileSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { snapshotPfade, githubHttps } from "../src/snapshot.ts";

describe("Snapshot im Clean Room", () => {
  it("lässt Entwürfe, Prüfrunden und offene Fragen für den Bau weg", () => {
    const repo = mkdtempSync(join(tmpdir(), "swx-snap-"));
    const git = (...a: string[]) => execFileSync("git", a, { cwd: repo, encoding: "utf8" });
    git("init", "-q");
    for (const f of ["src/main.ts", "specification/README.md", "specification/approved/obligations/save.json", "specification/drafts/save/x.json",
                     "specification/checks/round-1.md", "specification/unknowns.md"]) {
      mkdirSync(join(repo, f, ".."), { recursive: true });
      writeFileSync(join(repo, f), "x\n");
    }
    git("add", "-A");
    git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "t");
    const dateien = (raum: string | null): string[] =>
      execFileSync("sh", ["-c", `git archive --format=tar HEAD ${snapshotPfade(raum).map((x) => `'${x}'`).join(" ")} | tar -t`], { cwd: repo, encoding: "utf8" })
        .split("\n").filter((x) => x && !x.endsWith("/"));
    const bau = dateien("bau");
    expect(bau).toContain("src/main.ts");
    expect(bau).toContain("specification/approved/obligations/save.json");
    expect(bau).toContain("specification/README.md");
    expect(bau.some((x) => x.startsWith("specification/drafts") || x.startsWith("specification/checks") || x === "specification/unknowns.md")).toBe(false);
    expect(dateien("analyse")).toContain("specification/drafts/save/x.json");
    expect(dateien(null)).toContain("specification/unknowns.md");
  });
});

describe("githubHttps", () => {
  it("macht aus SSH-Adressen von GitHub öffentliche HTTPS-Adressen", () => {
    expect(githubHttps("ssh://git@github.com/swarvex-ai/rend-and-run.git")).toBe("https://github.com/swarvex-ai/rend-and-run.git");
    expect(githubHttps("git@github.com:swarvex-ai/rend-and-run.git")).toBe("https://github.com/swarvex-ai/rend-and-run.git");
    expect(githubHttps("git@github.com:o/r")).toBe("https://github.com/o/r.git");
    expect(githubHttps("ssh://git@gitlab.com/o/r.git")).toBeNull();
    expect(githubHttps("ssh://git@github.com/o/r.git; rm -rf /")).toBeNull();
  });
});
