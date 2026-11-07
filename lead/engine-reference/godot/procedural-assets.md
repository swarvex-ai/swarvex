# Godot — Assets in Code

Last verified: 2026-11-07 against Godot 4.7.2 (every snippet ran headless) | Engine: Godot 4.7

Sounds, music, pixel sprites and simple meshes can be GDScript functions instead of files or generator credits:
reviewable as a diff, testable headless, tunable by parameters. Generate them once at start and keep them.

## Models from the shared catalogue

- **Formwerk** (https://github.com/swarvex-ai/formwerk) is the swarvex catalogue of parametric assets: furniture,
  architecture, nature, vehicles and props, CC0. Its gallery exports `.glb` with baked textures; movable parts arrive
  as named nodes with their pivot at the hinge, and physics hints (body, mass, collider) as glTF extras.
- Need a model it lacks (a tank, a market stall)? Send Formwerk a work request (`request_work`) instead of buying
  generator credits: requests from other projects come first, and the result is in the catalogue for everyone.

## Meshes in code

`PrimitiveMesh` types (`BoxMesh`, `CylinderMesh`, `SphereMesh`, `PrismMesh`) for blockouts; `SurfaceTool`
(`begin(Mesh.PRIMITIVE_TRIANGLES)`, `set_uv()`, `add_vertex()`, `generate_normals()`, `commit()`) for your own
shapes; `MultiMeshInstance3D` for many copies.

## Sound and music

Compute samples, then wrap them in an `AudioStreamWAV`:

```gdscript
class_name SoundCode

const RATE := 48000

## A short hit: a falling tone with noise. Same seed, same samples.
static func hit(rng_seed: int, pitch := 220.0) -> PackedFloat32Array:
	var rng := RandomNumberGenerator.new()
	rng.seed = rng_seed
	var out := PackedFloat32Array()
	out.resize(int(0.4 * RATE))
	var phase := 0.0
	for i in out.size():
		var t := float(i) / RATE
		phase += TAU * (pitch - 120.0 * t) / RATE
		out[i] = (sin(phase) * 0.6 + rng.randf_range(-0.25, 0.25)) * exp(-t * 18.0) * minf(1.0, t / 0.002)
	return out

## Samples to a stream for AudioStreamPlayer, 16-bit mono.
static func to_stream(samples: PackedFloat32Array, loop := false) -> AudioStreamWAV:
	var bytes := PackedByteArray()
	bytes.resize(samples.size() * 2)
	for i in samples.size():
		bytes.encode_s16(i * 2, int(clampf(samples[i], -1.0, 1.0) * 32767.0))
	var wav := AudioStreamWAV.new()
	wav.format = AudioStreamWAV.FORMAT_16_BITS
	wav.mix_rate = RATE
	wav.data = bytes
	if loop:
		wav.loop_mode = AudioStreamWAV.LOOP_FORWARD
		wav.loop_end = samples.size()
	return wav
```

- **Cost:** one second of a tone with noise takes about 7 ms in GDScript (measured on 4.7.2). A long music loop with
  several voices takes a second or more: build it with `WorkerThreadPool.add_task()` or behind the loading screen.
- **Building blocks:** oscillators as a phase accumulator (sine, square, saw, triangle from the phase), noise from a
  seeded `RandomNumberGenerator`, envelopes (an attack of at least 2 ms, or the first sample clicks), simple filters
  (one-pole low-pass, a biquad from the Audio EQ Cookbook formulas), pitch slides.
- **Live sound** that follows the game (an engine hum, wind by speed): `AudioStreamGenerator` (`mix_rate`,
  `buffer_length`) on an `AudioStreamPlayer`; every frame, push `Vector2(left, right)` frames into
  `get_stream_playback()` with `push_frame()` while `get_frames_available()` is above zero.
- **Music as data:** a pattern of notes (step, pitch, length, instrument) rendered into exactly
  `bars * beats_per_bar * 60.0 / bpm * RATE` samples. Add the tails of notes that ring past the end to the start, so
  the loop has no click at the seam. Layers (drums, bass, melody) as separate loops in an `AudioStreamSynchronized`
  play in sync; fade a layer with `set_sync_stream_volume()` for music that follows the game.
- **Checks** in a headless test (see `current-best-practices.md`, Command Line): length; peak below −1 dBFS (0.89);
  no DC offset; first and last samples near zero for one-shots, a seamless seam for loops; loudness (RMS) in the range
  of its kind; same seed, same samples. `RandomNumberGenerator` gives the same numbers everywhere, `sin()` may differ
  in the last bits between platforms: compare with a stored reference within a tolerance, not by hash.
- **Agents cannot listen.** `wav.save_to_wav("user://out/<id>.wav")` in the test, so a reviewer can play what
  changed, and listen to new sounds yourself before accepting them.
- **No melodies of known games, films or songs**, not even rebuilt from notes: a melody is protected however it is
  synthesised.

## Pixel sprites and characters

Animated 3D characters are the one asset that is hard to make in code. Pixel-art sprites in a lit 3D world (2.5D),
or a 2D game, avoid it: a character is a small image per frame, and code can draw those images.

```gdscript
class_name SpriteCode

## One sheet of a pixel sprite; draw into the RGBA bytes with integer coordinates.
static func sheet_image(w: int, h: int, pixels: PackedByteArray) -> Image:
	return Image.create_from_data(w, h, false, Image.FORMAT_RGBA8, pixels)

## A character in a lit 3D world: lit, sharp pixels, origin at the feet.
static func character(frames: SpriteFrames, frame_height: int) -> AnimatedSprite3D:
	var s := AnimatedSprite3D.new()
	s.sprite_frames = frames
	s.pixel_size = 1.0 / 32.0
	s.billboard = BaseMaterial3D.BILLBOARD_FIXED_Y
	s.shaded = true
	s.alpha_cut = SpriteBase3D.ALPHA_CUT_DISCARD
	s.texture_filter = BaseMaterial3D.TEXTURE_FILTER_NEAREST
	s.offset.y = frame_height / 2.0
	return s
```

- **Drawing:** into a `PackedByteArray` of `w * h * 4` bytes with integer coordinates, then `Image.create_from_data()`:
  much faster than `set_pixel()` per pixel, and the same bit for bit on every machine. `Image.create_empty()` gives an
  empty image.
- **Characters from parts:** a body template per direction (4 or 8), layers on top (clothes, hair, the held item),
  each material a ramp of 3 to 5 shades from one palette, then a one-pixel dark outline around the silhouette.
  Animations are offsets of the parts per frame (a walk: 4 frames, legs and arms shifted, the body bobbing one pixel).
  Parameters and a seed make a whole village from one builder.
- **Frames:** one sheet per character (rows = animation × direction, columns = frames),
  `ImageTexture.create_from_image()`, one `AtlasTexture` (`atlas`, `region`) per frame in a `SpriteFrames`
  (`add_animation()`, `set_animation_speed()`, `add_frame()`), played by `AnimatedSprite2D` or `AnimatedSprite3D`.
- **2D:** `texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST`, or for the whole project
  `rendering/textures/canvas_textures/default_texture_filter` set to Nearest.
- **2.5D (above):** one pixel density for everything (`pixel_size = 1.0 / 32.0`: 32 pixels per metre), lit by the
  scene (`shaded`), `ALPHA_CUT_DISCARD` for sharp pixel edges without sorting problems between overlapping sprites,
  and `offset.y` at half the frame height so the origin is at the feet (checked: a 24-pixel frame spans 0 to 0.75 m).
  A billboard also faces the camera in the shadow pass, so its shadow is the silhouette the player sees; with several
  cameras that is undefined (Godot docs, `SpriteBase3D.billboard`).
- **Checks:** a headless test saves every sheet (`img.save_png("user://out/<id>.png")`) and compares its bytes
  (`get_data()`) with the baseline in the repo.

## When a file is the better choice

3D characters with skeletal animation, or scanned objects: glTF (`.glb`) with a free licence, the licence next to it
(`assets/LICENSES.md`: source, author, licence). Good CC0 sources: Kenney, Poly Haven, ambientCG, Quaternius. If the
game's look allows it, pixel sprites (above) avoid the animated 3D character altogether.
