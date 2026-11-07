// The simulation: pure TypeScript, no three.js, no DOM, no clock. Same seed and same steps give the same world,
// so tests check it in Node in milliseconds and every machine in the swarm sees the same result.
import { rng } from "./rng";
import { SHAPES } from "../content/shapes";

export const STEP = 1 / 60;

export type Body = { kind: string; x: number; y: number; z: number; angle: number; spin: number };
export type World = { tick: number; bodies: Body[] };

export function createWorld(seed: number, count = 12): World {
  const r = rng(seed);
  const bodies: Body[] = [];
  for (let i = 0; i < count; i++) {
    const kind = SHAPES[Math.floor(r() * SHAPES.length)];
    bodies.push({ kind: kind.id, x: (r() - 0.5) * 7, y: (r() - 0.5) * 3, z: (r() - 0.5) * 3, angle: r() * Math.PI * 2, spin: kind.speed * (0.5 + r()) });
  }
  return { tick: 0, bodies };
}

/** One fixed step of 1/60 s. Pure: it returns a new world and changes nothing else. */
export function step(w: World): World {
  return { tick: w.tick + 1, bodies: w.bodies.map((b) => ({ ...b, angle: (b.angle + b.spin * STEP) % (Math.PI * 2) })) };
}
