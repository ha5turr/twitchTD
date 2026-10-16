// Dynamic entities. The server sends positions 20 times a second; sprites
// glide toward the latest position so motion stays smooth at 60 fps.
import { CELL, toPx, ENEMY_STYLE, TOWER_STYLE, RARITY, FONT_UI, nickColor } from './layout.js';

const { Container, Graphics, Text } = PIXI;

export class Entities {
  constructor() {
    this.world = new Container(); // affected by lighting
    this.labels = new Container(); // nameplates and HP bars, always readable
    this.enemies = new Map();
    this.towers = new Map();
    this.proj = new Map();
  }

  clear() {
    for (const m of [this.enemies, this.towers, this.proj]) {
      for (const v of m.values()) v.destroy();
      m.clear();
    }
  }

  update(state) {
    sync(this.enemies, state.enemies, (e) => new EnemyView(e, this), (v, e) => v.set(e));
    sync(this.towers, state.towers, (t) => new TowerView(t, this), (v, t) => v.set(t));
    sync(this.proj, state.proj, (p) => new ProjView(p, this), (v, p) => v.set(p));
  }

  frame(dt) {
    const k = Math.min(1, dt * 14);
    for (const v of this.enemies.values()) v.frame(k);
    for (const v of this.proj.values()) v.frame(Math.min(1, dt * 30));
  }

  // Light sources in screen pixels, consumed by the lighting layer.
  lights() {
    const out = [];
    for (const t of this.towers.values()) {
      out.push({ x: t.cx, y: t.cy, r: 120 + 20 * t.level, color: TOWER_STYLE[t.kind]?.light ?? 0xffffff, a: 0.75 });
    }
    for (const p of this.proj.values()) {
      if (p.magic) out.push({ x: p.root.x, y: p.root.y, r: 70, color: 0xb58cff, a: 0.8 });
    }
    for (const e of this.enemies.values()) {
      if (e.kind === 'boss') out.push({ x: e.root.x, y: e.root.y, r: 140, color: 0xff3b3b, a: 0.6 });
    }
    return out;
  }
}

function sync(map, items, create, update) {
  const seen = new Set();
  for (const it of items) {
    seen.add(it.id);
    let v = map.get(it.id);
    if (!v) map.set(it.id, (v = create(it)));
    update(v, it);
  }
  for (const [id, v] of map) {
    if (!seen.has(id)) {
      v.destroy();
      map.delete(id);
    }
  }
}

class EnemyView {
  constructor(e, layer) {
    this.kind = e.kind;
    const st = ENEMY_STYLE[e.kind] ?? ENEMY_STYLE.normal;
    const r = st.r * CELL;
    this.r = r;
    this.root = new Graphics();
    const g = this.root;
    if (e.kind === 'boss') {
      // Crown of horns so the boss reads as a boss at a glance.
      for (let i = -2; i <= 2; i++) g.poly([i * 12 - 7, -r + 6, i * 12, -r - 16 - (i === 0 ? 8 : 0), i * 12 + 7, -r + 6]);
      g.fill(0x6e1d1d);
    }
    g.circle(0, 0, r).fill(st.color).stroke({ width: 3, color: 0x111111 });
    g.circle(-r * 0.35, -r * 0.15, Math.max(2, r * 0.16)).circle(r * 0.35, -r * 0.15, Math.max(2, r * 0.16)).fill(0x1a0f0f);
    if (e.kind === 'armored') g.rect(-r * 0.8, r * 0.1, r * 1.6, r * 0.35).fill(0x4d5566);
    layer.world.addChild(g);

    this.bar = new Graphics();
    layer.labels.addChild(this.bar);
    const p = toPx(e.x, e.y);
    g.position.set(p.x, p.y);
    this.target = p;
    this.hp = -1;
    this.set(e);
  }

  set(e) {
    this.target = toPx(e.x, e.y);
    if (e.hp !== this.hp) {
      this.hp = e.hp;
      const w = Math.max(36, this.r * 2);
      const f = Math.max(0, e.hp / e.maxHp);
      this.bar.clear();
      this.bar.rect(-w / 2, 0, w, 6).fill({ color: 0x000000, alpha: 0.7 });
      this.bar.rect(-w / 2, 0, w * f, 6).fill(f > 0.5 ? 0x5fd35f : f > 0.25 ? 0xe0b03a : 0xe04a3a);
    }
  }

  frame(k) {
    const g = this.root;
    g.x += (this.target.x - g.x) * k;
    g.y += (this.target.y - g.y) * k;
    this.bar.position.set(g.x, g.y - this.r - 14);
  }

  destroy() {
    this.root.destroy();
    this.bar.destroy();
  }
}

class TowerView {
  constructor(t, layer) {
    this.kind = t.kind;
    const p = toPx(t.x, t.y);
    this.cx = p.x + CELL / 2;
    this.cy = p.y + CELL / 2;
    this.root = new Graphics();
    this.root.position.set(this.cx, this.cy);
    layer.world.addChild(this.root);

    this.name = new Text({
      text: t.owner,
      style: { fontFamily: FONT_UI, fontSize: 15, fontWeight: '700', fill: nickColor(t.owner), stroke: { color: 0x000000, width: 4 } },
    });
    this.name.anchor.set(0.5, 0);
    this.name.position.set(this.cx, this.cy + 30);
    layer.labels.addChild(this.name);
    this.level = 0;
    this.set(t);
  }

  set(t) {
    if (t.lvl === this.level) return;
    this.level = t.lvl;
    const g = this.root;
    const rarity = RARITY[Math.min(t.lvl, 5) - 1];
    const st = TOWER_STYLE[this.kind] ?? TOWER_STYLE.arrow;
    g.clear();
    g.roundRect(-34, -34, 68, 68, 10).fill(0x1b1a1f).stroke({ width: 4, color: rarity });
    if (this.kind === 'magic') {
      g.poly([0, -26, 14, 0, 0, 26, -14, 0]).fill(st.color);
      g.circle(0, 0, 6).fill(0xffffff);
    } else {
      g.rect(-18, -10, 36, 26).fill(0x5a4630);
      g.poly([-22, -10, 0, -28, 22, -10]).fill(st.color);
      g.rect(-3, -2, 6, 12).fill(0x1b1a1f);
    }
    // Pips under the icon repeat the level for colour-blind viewers.
    for (let i = 0; i < t.lvl; i++) g.circle(-16 + i * 8, 26, 2.5).fill(rarity);
  }

  destroy() {
    this.root.destroy();
    this.name.destroy();
  }
}

class ProjView {
  constructor(p, layer) {
    this.magic = !!p.magic;
    this.root = new Graphics();
    if (this.magic) this.root.circle(0, 0, 8).fill(0xc9a6ff).circle(0, 0, 4).fill(0xffffff);
    else this.root.circle(0, 0, 4).fill(0xffe3a8);
    const px = toPx(p.x, p.y);
    this.root.position.set(px.x, px.y);
    this.target = px;
    layer.world.addChild(this.root);
  }

  set(p) {
    this.target = toPx(p.x, p.y);
  }

  frame(k) {
    this.root.x += (this.target.x - this.root.x) * k;
    this.root.y += (this.target.y - this.root.y) * k;
  }

  destroy() {
    this.root.destroy();
  }
}
