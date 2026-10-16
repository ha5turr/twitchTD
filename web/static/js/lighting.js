// Lightmap: a half-resolution texture cleared to the biome's ambient colour,
// with soft light blobs added on top, then multiplied over the world. Where
// the map is white the scene is untouched; where it is dark the scene dims.
import { W, H, TOP, LEFT } from './layout.js';
import { torchSpot } from './map.js';

const { Container, RenderTexture, Sprite, Texture } = PIXI;
const SCALE = 0.5;

function radialTexture() {
  const size = 256;
  const c = document.createElement('canvas');
  c.width = c.height = size;
  const ctx = c.getContext('2d');
  const grad = ctx.createRadialGradient(size / 2, size / 2, 0, size / 2, size / 2, size / 2);
  grad.addColorStop(0, 'rgba(255,255,255,1)');
  grad.addColorStop(0.4, 'rgba(255,255,255,0.55)');
  grad.addColorStop(1, 'rgba(255,255,255,0)');
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, size, size);
  return Texture.from(c);
}

export class Lighting {
  constructor(renderer) {
    this.renderer = renderer;
    this.rt = RenderTexture.create({ width: W * SCALE, height: H * SCALE });
    this.sprite = new Sprite(this.rt);
    this.sprite.scale.set(1 / SCALE);
    this.sprite.blendMode = 'multiply';
    this.lights = new Container();
    this.lights.scale.set(SCALE);
    this.tex = radialTexture();
    this.pool = [];
    this.ambient = 0x404050;
    this.torches = [];
    this.time = 0;
    this.alarm = 0; // 0..1, reddens the ambient during boss waves
  }

  setMap(map, pal) {
    this.ambient = pal.ambient;
    this.torchColor = pal.light;
    this.torches = map.torches.map(([x, y]) => ({ ...torchSpot(map, x, y), seed: x * 13 + y * 7 }));
  }

  blob(i) {
    let s = this.pool[i];
    if (!s) {
      s = new Sprite(this.tex);
      s.anchor.set(0.5);
      s.blendMode = 'add';
      this.pool[i] = s;
      this.lights.addChild(s);
    }
    s.visible = true;
    return s;
  }

  render(dt, dynamic) {
    this.time += dt;
    const all = [
      // The tavern is the one warm, fully lit place on screen.
      { x: LEFT / 2, y: TOP + 260, r: 420, color: 0xffc27a, a: 1 },
      { x: LEFT / 2, y: H - 260, r: 420, color: 0xffc27a, a: 1 },
      { x: LEFT / 2, y: TOP + 60, r: 160, color: 0xff8a3a, a: 0.9 + 0.1 * Math.sin(this.time * 11) }, // fireplace
      ...this.torches.map((t) => ({
        x: t.x, y: t.y, color: this.torchColor,
        r: 230 + 14 * Math.sin(this.time * 7 + t.seed) + 8 * Math.sin(this.time * 13 + t.seed * 2),
        a: 0.85 + 0.1 * Math.sin(this.time * 9 + t.seed) * (1 - this.alarm),
      })),
      ...dynamic,
    ];
    all.forEach((l, i) => {
      const s = this.blob(i);
      s.position.set(l.x, l.y);
      s.width = s.height = l.r * 2;
      s.tint = l.color;
      s.alpha = l.a;
    });
    for (let i = all.length; i < this.pool.length; i++) this.pool[i].visible = false;

    const amb = mix(this.ambient, 0x5a1414, this.alarm);
    this.renderer.render({
      container: this.lights,
      target: this.rt,
      clear: true,
      clearColor: [((amb >> 16) & 255) / 255, ((amb >> 8) & 255) / 255, (amb & 255) / 255, 1],
    });
  }
}

function mix(a, b, t) {
  const ch = (s) => Math.round(((a >> s) & 255) * (1 - t) + ((b >> s) & 255) * t);
  return (ch(16) << 16) | (ch(8) << 8) | ch(0);
}
