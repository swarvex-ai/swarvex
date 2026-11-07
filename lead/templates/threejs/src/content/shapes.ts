/** Content is data: one entry per kind. Adding a kind is a small task with its own test (tests/core/content.test.ts). */
export type ShapeKind = { id: "crystal" | "ring"; radius: number; speed: number; color: string };

export const SHAPES: ShapeKind[] = [
  { id: "crystal", radius: 0.5, speed: 1.2, color: "#7dd3fc" },
  { id: "ring", radius: 0.55, speed: 0.8, color: "#fbbf24" },
];
