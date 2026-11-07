// The view: builds and updates the three.js scene from the world state. It never changes the world.
import { AmbientLight, Color, DirectionalLight, Mesh, PerspectiveCamera, Scene } from "three";
import { WebGPURenderer } from "three/webgpu";
import { SHAPES } from "../content/shapes";
import type { World } from "../core/world";
import { crystal, ring } from "../assets/procedural/shapes";

const BUILDERS = { crystal, ring } as const;

export type FrameInfo = { drawCalls: number; triangles: number };

export async function createView(canvas: HTMLCanvasElement, world: World, opts: { forceWebGL: boolean; antialias: boolean }) {
  // WebGPU where the browser has it, otherwise WebGL 2; scene tests force WebGL 2 for stable software rendering
  const renderer = new WebGPURenderer({ canvas, antialias: opts.antialias, forceWebGL: opts.forceWebGL });
  await renderer.init();
  const scene = new Scene();
  scene.background = new Color("#0b0d10");
  scene.add(new AmbientLight("#ffffff", 0.6));
  const sun = new DirectionalLight("#ffffff", 2.5);
  sun.position.set(3, 5, 4);
  scene.add(sun);
  const camera = new PerspectiveCamera(50, 16 / 9, 0.1, 100);
  camera.position.set(0, 1.5, 9);
  camera.lookAt(0, 0, 0);

  const meshes = world.bodies.map((b) => {
    const kind = SHAPES.find((s) => s.id === b.kind)!;
    const { geometry, material } = BUILDERS[kind.id](kind.radius, kind.color);
    const mesh = new Mesh(geometry, material);
    mesh.position.set(b.x, b.y, b.z);
    scene.add(mesh);
    return mesh;
  });

  return {
    renderer,
    sync(w: World) {
      w.bodies.forEach((b, i) => { meshes[i].rotation.set(b.angle * 0.5, b.angle, 0); });
    },
    render(): FrameInfo {
      renderer.render(scene, camera);
      return { drawCalls: renderer.info.render.drawCalls, triangles: renderer.info.render.triangles };
    },
    resize(width: number, height: number) {
      renderer.setSize(width, height, false);
      camera.aspect = width / height;
      camera.updateProjectionMatrix();
    },
  };
}
