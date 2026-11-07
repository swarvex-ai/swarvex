// Start: the normal game loop, or the test mode for scene tests (?test): fixed steps on request, no animation loop.
import { createWorld, step, STEP } from "./core/world";
import { createView, type FrameInfo } from "./render/view";

declare global {
  interface Window { __swarvex?: { step(n: number): FrameInfo; tick(): number } }
}

const params = new URLSearchParams(location.search);
const testMode = params.has("test");
const canvas = document.querySelector("canvas")!;
let world = createWorld(Number(params.get("seed") ?? 1));
const view = await createView(canvas, world, { forceWebGL: testMode || params.has("webgl"), antialias: !testMode });
view.resize(canvas.clientWidth, canvas.clientHeight);

if (testMode) {
  window.__swarvex = {
    step(n: number) {
      for (let i = 0; i < n; i++) world = step(world);
      view.sync(world);
      return view.render();
    },
    tick: () => world.tick,
  };
  document.body.dataset.ready = "1";
} else {
  addEventListener("resize", () => view.resize(canvas.clientWidth, canvas.clientHeight));
  let rest = 0, last = performance.now();
  view.renderer.setAnimationLoop((now: number) => {
    rest += Math.min(0.25, (now - last) / 1000);
    last = now;
    while (rest >= STEP) { world = step(world); rest -= STEP; }
    view.sync(world);
    view.render();
  });
}
