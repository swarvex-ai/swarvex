// Procedural assets: geometry and material in code. No model files, no generator credits, reviewable as a diff.
import { IcosahedronGeometry, TorusGeometry } from "three";
import { MeshStandardNodeMaterial } from "three/webgpu";
import { color, float, mix, normalWorld } from "three/tsl";

export function crystal(radius: number, hex: string) {
  const geometry = new IcosahedronGeometry(radius, 0);
  const material = new MeshStandardNodeMaterial({ roughness: 0.25, metalness: 0.1, flatShading: true });
  // lighter towards the top: a TSL node, compiled to WGSL (WebGPU) or GLSL (WebGL 2)
  material.colorNode = mix(color(hex), color("#ffffff"), normalWorld.y.mul(0.5).add(0.5).mul(float(0.35)));
  return { geometry, material };
}

export function ring(radius: number, hex: string) {
  const geometry = new TorusGeometry(radius, radius * 0.18, 12, 48);
  const material = new MeshStandardNodeMaterial({ roughness: 0.4, metalness: 0.6 });
  material.colorNode = color(hex);
  return { geometry, material };
}
