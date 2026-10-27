// The King in Yellow's powers, played big over the field (king.go emits the
// events): the Yellow Sign shown close up before it flies to its building,
// Cassilda's song as twin suns circling, his throne with the retinue when
// the second phase calls it, and the arrow that pounds the shrines and the
// drums when the mask comes off. The sign plays at once; the rest wait their
// turn so two of them never share the screen.
import { LEFT, TOP, COLS, ROWS, CELL, toPx } from './layout.js';
import { drawDrum, towerArt, softGlow as glow } from './sprites.js';
import { hasTileset, sprite } from './tileset.js';
import { SIGN_H } from './entities.js';

const { Container, Graphics, Sprite, AnimatedSprite, Texture, Rectangle, Assets } = PIXI;

const FIELD_W = COLS * CELL;
const FIELD_H = ROWS * CELL;
const CX = LEFT + FIELD_W / 2;
const CY = TOP + FIELD_H * 0.45;
const DIR = 'assets/king';

const clamp01 = (v) => Math.max(0, Math.min(1, v));
const smooth = (a, b, v) => {
  const k = clamp01((v - a) / (b - a));
  return k * k * (3 - 2 * k);
};
const easeInOut = (k) => (k < 0.5 ? 4 * k * k * k : 1 - Math.pow(-2 * k + 2, 3) / 2);
const easeIn = (k) => k * k * k;
const easeOutBack = (k) => 1 + 2.70158 * Math.pow(k - 1, 3) + 1.70158 * Math.pow(k - 1, 2);

// sheet cuts a strip or a grid of square frames out of a texture.
function sheet(tex, size) {
  const out = [];
  for (let y = 0; y + size <= tex.height; y += size) {
    for (let x = 0; x + size <= tex.width; x += size) out.push(new Texture({ source: tex.source, frame: new Rectangle(x, y, size, size) }));
  }
  return out;
}

// paleSun repaints the red sun of the sheet in the pale gold of Carcosa's
// suns: its brightness picks a colour from deep ochre to almost white.
function paleSun(tex) {
  const img = tex.source.resource;
  const c = document.createElement('canvas');
  c.width = tex.width;
  c.height = tex.height;
  const ctx = c.getContext('2d');
  ctx.drawImage(img, 0, 0);
  const d = ctx.getImageData(0, 0, c.width, c.height);
  const stops = [[110, 60, 10], [240, 190, 40], [255, 248, 200]];
  for (let i = 0; i < d.data.length; i += 4) {
    const l = Math.min(1, ((d.data[i] * 0.6 + d.data[i + 1] * 0.4) / 255) * 1.3);
    const [a, b, k] = l < 0.5 ? [stops[0], stops[1], l * 2] : [stops[1], stops[2], l * 2 - 1];
    for (let j = 0; j < 3; j++) d.data[i + j] = a[j] + (b[j] - a[j]) * k;
  }
  ctx.putImageData(d, 0, 0);
  const t = Texture.from(c);
  t.source.scaleMode = 'nearest';
  return t;
}

export class KingFx {
  constructor() {
    this.layer = new Container();
    this.shade = new Graphics();
    this.shade.rect(LEFT, TOP, FIELD_W, FIELD_H).fill(0x000000);
    this.shade.alpha = 0;
    this.stage = new Container(); // the cinematic playing now
    this.signs = new Container(); // Yellow Signs on their way
    this.layer.addChild(this.shade, this.stage, this.signs);
    this.queue = [];
    this.now = null;
    this.flying = [];
    this.tex = {};
    const load = (name, file, prep) =>
      Assets.load(`${DIR}/${file}`)
        .then((t) => (this.tex[name] = prep ? prep(t) : t))
        .catch(() => {});
    const pixel = (t) => ((t.source.scaleMode = 'nearest'), t);
    load('sign', 'sign/yellow_sign.jpg');
    load('throne', 'king_throne.png', pixel);
    load('sun', 'fx/sun.png', (t) => sheet(paleSun(t), 64));
    load('orb', 'fx/abyss_orb.png', (t) => sheet(pixel(t), 16));
    load('arrow', 'fx/arrow_down.png', pixel);
  }

  // onEvent starts the animation of a power. dying: the king is near death,
  // and the long ones are not worth his last seconds.
  onEvent(ev, dying) {
    switch (ev.kind) {
      case 'sign':
        this.sign(ev);
        break;
      case 'song':
        if (!dying) this.play(() => this.song());
        break;
      case 'kingPhase':
        if (ev.amount === 2) this.play(() => this.retinue());
        break;
      case 'unmask':
        if (!dying) this.play(() => this.unmask());
        break;
    }
  }

  // play queues a cinematic: a builder returning its length and a step
  // function that draws it at time t.
  play(build) {
    if (this.queue.length >= 2) this.queue.shift(); // a backlog is stale
    this.queue.push(build);
  }

  // clear stops everything at once: the king fell or the game is over.
  clear() {
    this.queue = [];
    this.endNow();
    for (const f of this.flying) f.s.destroy();
    this.flying = [];
  }

  endNow() {
    this.now = null;
    this.stage.removeChildren().forEach((c) => c.destroy({ children: true }));
    this.shade.alpha = 0;
  }

  frame(dt) {
    this.flySigns(dt);
    if (!this.now && this.queue.length) {
      const c = this.queue.shift()();
      if (c) this.now = { ...c, t: 0 };
    }
    const n = this.now;
    if (!n) return;
    n.t += dt;
    if (n.t >= n.len) return this.endNow();
    n.step(n.t, dt);
  }

  // sign: the Yellow Sign fills the field, holds for a second, then shrinks
  // onto the building it marks.
  sign(ev) {
    const tex = this.tex.sign;
    if (!tex) return;
    const s = new Sprite(tex);
    s.anchor.set(0.5);
    s.blendMode = 'add'; // the black around it is see-through
    s.position.set(CX, CY);
    s.alpha = 0;
    const to = toPx(ev.x, ev.y - 0.08);
    const ring = new Graphics();
    this.signs.addChild(s, ring);
    this.flying.push({ s, ring, to, t: 0, big: (FIELD_H * 0.62) / tex.height, small: SIGN_H / tex.height });
  }

  flySigns(dt) {
    this.flying = this.flying.filter((f) => {
      f.t += dt;
      const { s, t } = f;
      if (t < 0.25) {
        const k = t / 0.25;
        s.alpha = k;
        s.scale.set(f.big * (1.35 - 0.35 * easeOutBack(k)));
        s.rotation = (1 - k) * -0.25;
      } else if (t < 1.1) {
        s.alpha = 1;
        s.rotation = 0;
        s.scale.set(f.big * (1 + 0.025 * Math.sin((t - 0.25) * 9)));
      } else if (t < 1.6) {
        const k = easeIn((t - 1.1) / 0.5);
        s.position.set(CX + (f.to.x - CX) * k, CY + (f.to.y - CY) * k);
        s.scale.set(f.big + (f.small - f.big) * k);
        s.rotation = k * Math.PI * 2;
      } else if (t < 2.1) {
        const k = (t - 1.6) / 0.5;
        s.position.set(f.to.x, f.to.y);
        s.scale.set(f.small);
        s.rotation = 0;
        s.alpha = 1 - k;
        f.ring.clear().circle(f.to.x, f.to.y, 20 + 60 * k).stroke({ width: 6 * (1 - k) + 1, color: 0xffd23a, alpha: 1 - k });
      } else {
        s.destroy();
        f.ring.destroy();
        return false;
      }
      return true;
    });
  }

  // song: one sun rises, splits in two, and the twins race round an oval,
  // faster and faster to the middle, then slow down, meet again and burn out.
  song() {
    const frames = this.tex.sun;
    if (!frames) return null;
    const len = 3.2;
    const turns = 2.25;
    const rx = FIELD_W * 0.27;
    const ry = FIELD_H * 0.13;
    const tail = 12;
    const trail = [0, 1].map(() => Array.from({ length: tail }, () => glow(0xffb02a)));
    const halos = [0, 1].map(() => glow(0xffe08a));
    const suns = [0, 1].map(() => {
      const s = new AnimatedSprite(frames);
      s.anchor.set(0.5);
      s.animationSpeed = 0.5;
      s.play();
      return s;
    });
    const sky = new Container(); // the near sun is drawn over the far one
    sky.sortableChildren = true;
    sky.addChild(...suns);
    const flash = new Graphics();
    flash.blendMode = 'add';
    this.stage.addChild(...trail.flat(), ...halos, sky, flash);
    const past = [[], []];
    return {
      len,
      step: (t) => {
        const u = t / len;
        this.shade.alpha = 0.45 * smooth(0, 0.12, u) * (1 - smooth(0.88, 1, u));
        const spread = smooth(0.1, 0.28, u) * (1 - smooth(0.7, 0.88, u));
        const a = -Math.PI / 2 + turns * Math.PI * 2 * easeInOut(smooth(0.1, 0.88, u));
        const grow = easeOutBack(clamp01(u / 0.12));
        const fade = 1 - smooth(0.9, 1, u);
        suns.forEach((s, i) => {
          const ai = a + i * Math.PI;
          const depth = Math.sin(ai); // the near side of the oval is lower and larger
          s.position.set(CX + Math.cos(ai) * rx * spread, CY + depth * ry * spread);
          s.scale.set(2.2 * grow * (1 + 0.18 * depth * spread) * (1 + 0.5 * smooth(0.9, 1, u)));
          s.alpha = fade;
          s.zIndex = depth;
          past[i].unshift({ x: s.x, y: s.y });
          past[i].length = Math.min(past[i].length, tail + 1);
          trail[i].forEach((g, j) => {
            const p = past[i][j + 1] ?? past[i][0];
            const k = 1 - j / tail;
            g.position.set(p.x, p.y);
            g.scale.set(s.scale.x * 0.55 * k);
            g.alpha = 0.5 * k * spread * fade;
          });
          halos[i].position.set(s.x, s.y);
          halos[i].scale.set(s.scale.x * 1.6);
          halos[i].alpha = 0.55 * fade;
        });
        // Meeting again they flare once.
        const k = smooth(0.86, 1, u);
        flash.clear();
        if (k > 0) flash.circle(CX, CY, 80 + 260 * k).stroke({ width: 14 * (1 - k) + 1, color: 0xfff0b0, alpha: 1 - k });
      },
    };
  }

  // retinue: the king on his throne, three cultists kneeling before it and
  // three orbs of the abyss over his head, the middle one a little higher.
  retinue() {
    const throne = this.tex.throne;
    if (!throne) return null;
    const len = 4.2;
    const ts = 3;
    const base = TOP + FIELD_H * 0.8;
    const top = base - throne.height * ts;
    const king = new Sprite(throne);
    king.anchor.set(0.5, 1);
    king.scale.set(ts);
    king.position.set(CX, base);
    const orbs = (this.tex.orb ? [[-230, 70, 0.7], [0, 0, 0.85], [230, 70, 1]] : []).map(([x, y, at]) => {
      const s = new AnimatedSprite(this.tex.orb);
      s.anchor.set(0.5);
      s.animationSpeed = 0.15;
      s.play();
      return { s, halo: glow(0x8a3aff), x: CX + x, y: top - 40 + y, at };
    });
    // The outer two face the middle.
    const cultists = (hasTileset() ? [[-200, 1.4, 1], [0, 1.6, 1], [200, 1.8, -1]] : [])
      .map(([x, at, dir]) => {
        const s = sprite('orc_shaman_idle', 7, 0.12);
        if (s) s.scale.x *= dir;
        return s && { s, x: CX + x, y: base + 60, at };
      })
      .filter(Boolean);
    this.stage.addChild(king, ...orbs.flatMap((o) => [o.halo, o.s]), ...cultists.map((c) => c.s));
    return {
      len,
      step: (t) => {
        const out = 1 - smooth(len - 0.6, len, t);
        this.shade.alpha = 0.7 * smooth(0, 0.4, t) * out;
        const k = smooth(0, 0.6, t);
        king.alpha = k * out;
        king.y = base + 40 * (1 - k);
        for (const o of orbs) {
          const p = clamp01((t - o.at) / 0.35);
          o.s.alpha = Math.min(1, p * 2) * out;
          o.s.scale.set(5.5 * easeOutBack(p));
          o.s.position.set(o.x, o.y + Math.sin(t * 2.4 + o.x) * 8);
          o.halo.position.copyFrom(o.s.position);
          o.halo.scale.set(1.9 * easeOutBack(p));
          o.halo.alpha = 0.9 * o.s.alpha * (0.8 + 0.2 * Math.sin(t * 6 + o.x));
        }
        for (const c of cultists) {
          const p = clamp01((t - c.at) / 0.4);
          c.s.alpha = p * out;
          c.s.position.set(c.x, c.y + 30 * (1 - p));
        }
      },
    };
  }

  // unmask: shrines and drums line up, a violet arrow over each pounds down
  // on it twice, they shudder at each blow and vanish.
  unmask() {
    const arrowTex = this.tex.arrow;
    if (!arrowTex || !hasTileset()) return null;
    const len = 3.6;
    const y = TOP + FIELD_H * 0.75;
    const gap = 280;
    const size = 2; // the buildings drawn twice their size on the field
    const kinds = ['shrine', 'drums', 'shrine', 'drums'];
    const items = kinds
      .map((kind, i) => {
        const c = towerArt(kind, sprite);
        if (!c) return null;
        if (kind === 'drums') {
          const g = new Graphics();
          drawDrum(g);
          c.addChild(g);
        }
        const x = CX + (i - (kinds.length - 1) / 2) * gap;
        const top = y + c.getLocalBounds().y * size; // the arrow's tip stops here
        const arrow = new Sprite(arrowTex);
        arrow.anchor.set(0.5, 1);
        arrow.tint = 0xb05aff;
        const d = i * 0.08; // the blows fall one after another along the row
        return { c, x, top, arrow, light: glow(0x9a4aff), at: 0.1 + i * 0.12, hits: [1.2 + d, 1.75 + d] };
      })
      .filter(Boolean);
    const waves = new Graphics();
    this.stage.addChild(...items.map((it) => it.c), ...items.flatMap((it) => [it.light, it.arrow]), waves);
    return {
      len,
      step: (t) => {
        const out = 1 - smooth(len - 0.5, len, t);
        this.shade.alpha = 0.7 * smooth(0, 0.35, t) * out;
        const gone = smooth(2.15, 2.7, t);
        waves.clear();
        for (const it of items) {
          const [h1, h2] = it.hits;
          // The arrow drops from above, bounces off the first blow and falls again.
          let lift;
          if (t < h1) lift = 230 * (1 - easeIn(clamp01((t - (h1 - 0.45)) / 0.45)));
          else if (t < h2) lift = 80 * Math.sin(((t - h1) / (h2 - h1)) * Math.PI);
          else lift = 0;
          const struck = (t >= h1 && t < h1 + 0.08) || (t >= h2 && t < h2 + 0.08);
          it.arrow.position.set(it.x, it.top + 12 - lift); // the sheet leaves two empty rows under the tip
          it.arrow.scale.set(8, 8 * (struck ? 0.78 : 1));
          it.arrow.alpha = smooth(h1 - 0.6, h1 - 0.4, t) * (1 - smooth(2.6, 3.0, t));
          it.light.position.set(it.x, it.arrow.y - 64);
          it.light.scale.set(1.8, 2.1);
          it.light.alpha = 0.8 * it.arrow.alpha;
          // A shudder after each blow, then the building sinks and fades.
          let shake = 0;
          for (const h of it.hits) {
            const k = (t - h) / 0.4;
            if (k < 0 || k > 1) continue;
            shake = Math.max(shake, 1 - k);
            waves.ellipse(it.x, y + 40, 60 + 110 * k, 16 + 26 * k).stroke({ width: 8 * (1 - k) + 1, color: 0xc08aff, alpha: 1 - k });
          }
          const p = clamp01((t - it.at) / 0.3);
          it.c.alpha = p * (1 - gone) * out;
          it.c.scale.set(size * easeOutBack(p) * (1 - 0.4 * gone));
          it.c.position.set(it.x + Math.sin(t * 70 + it.x) * 9 * shake, y + 30 * gone);
          it.c.tint = shake > 0.3 ? 0xd0a0ff : 0xffffff;
        }
      },
    };
  }
}
