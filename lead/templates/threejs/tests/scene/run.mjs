// Scene tests in headless Chromium: the build (dist/) is served locally, each scene runs a fixed number of steps in test
// mode (?test), then its draw calls and triangles are checked against the budget and its image against the baseline.
// No network, no GPU: WebGL 2 through SwiftShader. A missing baseline is written and the test fails, so it gets committed
// on purpose. Results and diffs: tests/scene/out/.
import { createServer } from "node:http";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { extname, join, normalize } from "node:path";
import { chromium } from "playwright-core";
import { PNG } from "pngjs";
import pixelmatch from "pixelmatch";

const ordner = new URL(".", import.meta.url).pathname;
const dist = join(ordner, "../../dist");
const { viewport, scenes } = JSON.parse(readFileSync(join(ordner, "scenes.json"), "utf8"));
const browser = [process.env.CHROME_PATH, "/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome"].find((p) => p && existsSync(p));
if (!browser) { console.error("No Chromium found: set CHROME_PATH (the swarvex threejs environment has it)."); process.exit(2); }
if (!existsSync(join(dist, "index.html"))) { console.error("No build: run `vite build` first."); process.exit(2); }

const TYPES = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".json": "application/json", ".png": "image/png", ".wasm": "application/wasm", ".glb": "model/gltf-binary" };
const server = createServer((req, res) => {
  const pfad = normalize(decodeURIComponent(new URL(req.url, "http://x").pathname)).replace(/^(\.\.[/\\])+/, "");
  const datei = join(dist, pfad.endsWith("/") ? pfad + "index.html" : pfad);
  if (!datei.startsWith(dist) || !existsSync(datei)) { res.writeHead(404).end(); return; }
  res.writeHead(200, { "content-type": TYPES[extname(datei)] ?? "application/octet-stream" }).end(readFileSync(datei));
});
await new Promise((r) => server.listen(0, "127.0.0.1", r));
const basis = `http://127.0.0.1:${server.address().port}/`;

const chrome = await chromium.launch({ executablePath: browser, args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist", "--disable-dev-shm-usage"] });
const out = join(ordner, "out");
mkdirSync(out, { recursive: true });
mkdirSync(join(ordner, "baseline"), { recursive: true });
let fehler = 0;
for (const s of scenes) {
  const page = await chrome.newPage({ viewport, deviceScaleFactor: 1 });
  const meldungen = [];
  page.on("pageerror", (e) => meldungen.push(`page error: ${e.message}`));
  page.on("console", (m) => { if (m.type() === "error") meldungen.push(`console: ${m.text()}`); });
  const bericht = (text) => { console.log(`${s.name}: ${text}`); };
  try {
    await page.goto(`${basis}?test&${s.query ?? ""}`);
    await page.waitForFunction(() => document.body.dataset.ready === "1", null, { timeout: 60_000 });
    const info = await page.evaluate((n) => window.__swarvex.step(n), s.steps ?? 1);
    const bild = PNG.sync.read(await page.locator("canvas").screenshot());
    if (meldungen.length) { fehler++; bericht(`FAIL errors in the page\n  ${meldungen.join("\n  ")}`); }
    if (info.drawCalls > s.maxDrawCalls) { fehler++; bericht(`FAIL ${info.drawCalls} draw calls, budget ${s.maxDrawCalls}`); }
    if (info.triangles > s.maxTriangles) { fehler++; bericht(`FAIL ${info.triangles} triangles, budget ${s.maxTriangles}`); }
    writeFileSync(join(out, `${s.name}.png`), PNG.sync.write(bild));
    const basisDatei = join(ordner, "baseline", `${s.name}.png`);
    if (!existsSync(basisDatei)) {
      writeFileSync(basisDatei, PNG.sync.write(bild));
      fehler++;
      bericht("FAIL no baseline yet: written to tests/scene/baseline; check it and commit it");
    } else {
      const soll = PNG.sync.read(readFileSync(basisDatei));
      if (soll.width !== bild.width || soll.height !== bild.height) { fehler++; bericht("FAIL image size differs from the baseline"); }
      else {
        const diff = new PNG({ width: bild.width, height: bild.height });
        const n = pixelmatch(soll.data, bild.data, diff.data, bild.width, bild.height, { threshold: 0.15 });
        const prozent = (100 * n) / (bild.width * bild.height);
        writeFileSync(join(out, `${s.name}.diff.png`), PNG.sync.write(diff));
        if (prozent > (s.maxDiffPercent ?? 2)) { fehler++; bericht(`FAIL ${prozent.toFixed(2)} % of pixels differ from the baseline (allowed ${s.maxDiffPercent ?? 2} %), see tests/scene/out`); }
        else bericht(`ok · ${info.drawCalls} draw calls · ${info.triangles} triangles · ${prozent.toFixed(2)} % changed`);
      }
    }
  } catch (e) {
    fehler++;
    bericht(`FAIL ${e.message}${meldungen.length ? `\n  ${meldungen.join("\n  ")}` : ""}`);
  }
  await page.close();
}
await chrome.close();
server.close();
process.exit(fehler ? 1 : 0);
