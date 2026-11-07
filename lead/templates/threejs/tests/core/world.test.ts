import { describe, expect, it } from "vitest";
import { createWorld, step } from "../../src/core/world";

describe("world", () => {
  it("is the same for the same seed, on every machine", () => {
    expect(createWorld(7)).toEqual(createWorld(7));
    expect(createWorld(7)).not.toEqual(createWorld(8));
  });
  it("steps deterministically and keeps angles in one turn", () => {
    let a = createWorld(3), b = createWorld(3);
    for (let i = 0; i < 600; i++) { a = step(a); b = step(b); }
    expect(a).toEqual(b);
    expect(a.tick).toBe(600);
    for (const body of a.bodies) expect(body.angle).toBeLessThan(Math.PI * 2);
  });
});
