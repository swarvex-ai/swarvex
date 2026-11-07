import { describe, expect, it } from "vitest";
import { SHAPES } from "../../src/content/shapes";

describe("content", () => {
  it("every kind is complete and unique", () => {
    expect(new Set(SHAPES.map((s) => s.id)).size).toBe(SHAPES.length);
    for (const s of SHAPES) {
      expect(s.radius).toBeGreaterThan(0);
      expect(s.speed).toBeGreaterThan(0);
      expect(s.color).toMatch(/^#[0-9a-f]{6}$/);
    }
  });
});
