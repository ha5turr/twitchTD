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
    this.workers = new Map();
  }

  clear() {
    for (const m of [this.enemies, this.towers, this.proj, this.workers]) {
      for (const v of m.values()) v.destroy();
      m.clear();
    }
  }

  update(state) {
    sync(this.enemies, state.enemies, (e) => new EnemyView(e, this), (v, e) => v.set(e));
    sync(this.towers, state.towers, (t) => new TowerView(t, this), (v, t) => v.set(t));
    sync(this.proj, state.proj, (p) => new ProjView(p, this), (v, p) => v.set(p));
    const workers = (state.workers ?? []).map((w) => ({ ...w, id: w.name }));
    sync(this.workers, workers, (w) => new WorkerView(w, this), (v, w) => v.set(w));
  }

  // Names of mercenaries currently out of the tavern.
  outside() {
    return new Set(this.workers.keys());
  }

  frame(dt) {
    const k = Math.min(1, dt * 14);
    for (const v of this.enemies.values()) v.frame(k);
    for (const v of this.proj.values()) v.frame(Math.min(1, dt * 30));
    for (const v of this.workers.values()) v.frame(k, dt);
  }

  // Light sources in screen pixels, consumed by the lighting layer.
  lights() {
    const out = [];
    for (const t of this.towers.values()) {
      if (t.built) out.push({ x: t.cx, y: t.cy, r: 120 + 20 * t.level, color: TOWER_STYLE[t.kind]?.light ?? 0xffffff, a: 0.75 });
    }
    // Each mercenary carries a lantern, so it is visible on dark ground.
    for (const w of this.workers.values()) out.push({ x: w.root.x, y: w.root.y - 10, r: 80, color: 0xffcf8a, a: 0.7 });
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
    this.bar = new Graphics();
    this.bar.position.set(this.cx, this.cy - 44);
    layer.labels.addChild(this.bar);
    this.level = 0;
    this.built = null;
    this.progress = -1;
    this.set(t);
  }

  set(t) {
    if (t.progress !== this.progress) {
      // Construction or upgrade under way: a progress bar above the cell.
      this.progress = t.progress ?? 0;
      this.bar.clear();
      if (this.progress > 0) {
        this.bar.rect(-30, 0, 60, 7).fill({ color: 0x000000, alpha: 0.75 });
        this.bar.rect(-30, 0, 60 * this.progress, 7).fill(0xffcf5a);
      }
    }
    if (t.lvl === this.level && t.built === this.built) return;
    this.level = t.lvl;
    this.built = t.built;
    const g = this.root;
    g.clear();
    if (!t.built) {
      // Construction site: scaffolding and a ghost of the future tower.
      g.roundRect(-34, -34, 68, 68, 10).fill({ color: 0x1b1a1f, alpha: 0.5 }).stroke({ width: 3, color: 0x8a7a5a, alpha: 0.9 });
      g.moveTo(-28, 28).lineTo(28, -28).moveTo(-28, -28).lineTo(28, 28).stroke({ width: 3, color: 0x6b5638, alpha: 0.8 });
      g.alpha = 0.85;
      return;
    }
    g.alpha = 1;
    const rarity = RARITY[Math.min(t.lvl, 5) - 1];
    const st = TOWER_STYLE[this.kind] ?? TOWER_STYLE.arrow;
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
    this.bar.destroy();
  }
}

// A chatter's mercenary on the field: a small figure in the nick's colour
// with the nick above it. It bobs while walking, swings a hammer while
// building and shows stars while stunned.
class WorkerView {
  constructor(w, layer) {
    const color = nickColor(w.name);
    this.root = new Container();
    this.body = new Graphics();
    this.body.circle(0, -18, 9).fill(color).stroke({ width: 2, color: 0x000000 });
    this.body.roundRect(-8, -10, 16, 18, 4).fill(color).stroke({ width: 2, color: 0x000000 });
    this.body.rect(9, -14, 4, 12).fill(0x6b4a2a); // lantern pole
    this.body.circle(11, -16, 4).fill(0xffd27a);
    this.root.addChild(this.body);
    this.tool = new Graphics();
    this.tool.rect(-2, -16, 4, 16).fill(0x7a5a3a).rect(-6, -18, 12, 5).fill(0x9aa0a8);
    this.tool.position.set(-10, -6);
    this.root.addChild(this.tool);
    layer.world.addChild(this.root);

    this.stars = new Text({ text: '✦ ✦', style: { fontFamily: FONT_UI, fontSize: 16, fill: 0xffe066, stroke: { color: 0x000000, width: 3 } } });
    this.stars.anchor.set(0.5);
    this.name = new Text({
      text: w.name,
      style: { fontFamily: FONT_UI, fontSize: 16, fontWeight: '700', fill: color, stroke: { color: 0x000000, width: 4 } },
    });
    this.name.anchor.set(0.5, 1);
    layer.labels.addChild(this.name, this.stars);

    const p = toPx(w.x, w.y);
    this.root.position.set(p.x, p.y);
    this.target = p;
    this.t = Math.random() * 10;
    this.set(w);
  }

  set(w) {
    this.target = toPx(w.x, w.y);
    this.state = w.state;
  }

  frame(k, dt) {
    this.t += dt;
    const r = this.root;
    r.x += (this.target.x - r.x) * k;
    r.y += (this.target.y - r.y) * k;
    const moving = this.state === 'walk' || this.state === 'return' || this.state === 'flee';
    this.body.y = moving ? -Math.abs(Math.sin(this.t * 10)) * 4 : 0;
    this.tool.rotation = this.state === 'build' ? Math.sin(this.t * 14) * 0.9 : 0.3;
    this.stars.visible = this.state === 'flee' || this.state === 'rest';
    this.stars.position.set(r.x, r.y - 52);
    this.stars.rotation = Math.sin(this.t * 6) * 0.3;
    this.name.position.set(r.x, r.y - 32);
  }

  destroy() {
    this.root.destroy({ children: true });
    this.name.destroy();
    this.stars.destroy();
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
