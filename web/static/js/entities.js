// Dynamic entities. The server sends positions 20 times a second; sprites
// glide toward the latest position so motion stays smooth at 60 fps.
import { CELL, toPx, RARITY, FONT_UI, FONT_PIXEL, nickColor } from './layout.js';
import { drawFigure } from './figures.js';
import { ENEMIES, TOWERS, ENEMY_ART, HERO_ART, towerArt, drawEnemy, drawTower, drawProjectile } from './sprites.js';
import { hasTileset, sprite, frames, texture } from './tileset.js';

const { Container, Graphics, Text } = PIXI;

export class Entities {
  constructor() {
    this.world = new Container(); // affected by lighting
    this.labels = new Container(); // nameplates and HP bars, always readable
    this.enemies = new Map();
    this.towers = new Map();
    this.proj = new Map();
    this.workers = new Map();
    this.t = 0;
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
    this.t += dt;
    const k = Math.min(1, dt * 14);
    for (const v of this.enemies.values()) v.frame(k, this.t);
    for (const v of this.proj.values()) v.frame(Math.min(1, dt * 30));
    for (const v of this.workers.values()) v.frame(k, dt);
    for (const v of this.towers.values()) v.frame(this.t, this.enemies);
  }

  // Light sources in screen pixels, consumed by the lighting layer.
  lights() {
    const out = [];
    for (const t of this.towers.values()) {
      const light = TOWERS[t.kind]?.light;
      if (t.built && light && !t.disabled) out.push({ x: t.cx, y: t.cy, r: 120 + 20 * t.level, color: light, a: 0.75 });
    }
    // Each mercenary carries a lantern, so it is visible on dark ground.
    for (const w of this.workers.values()) out.push({ x: w.root.x, y: w.root.y - 10, r: 80, color: 0xffcf8a, a: 0.7 });
    for (const p of this.proj.values()) {
      if (p.kind === 'magic') out.push({ x: p.root.x, y: p.root.y, r: 70, color: 0xb58cff, a: 0.8 });
      if (p.kind === 'fire') out.push({ x: p.root.x, y: p.root.y, r: 90, color: 0xff7a2a, a: 0.9 });
    }
    for (const e of this.enemies.values()) {
      if (e.boss) out.push({ x: e.root.x, y: e.root.y, r: 160, color: 0xff3b3b, a: 0.6 });
      else if (e.burn) out.push({ x: e.root.x, y: e.root.y, r: 60, color: 0xff7a2a, a: 0.7 });
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
    this.boss = !!e.boss;
    const st = ENEMIES[e.kind] ?? ENEMIES.skeleton;
    this.r = st.r * CELL;
    this.root = new Container();
    const art = hasTileset() && ENEMY_ART[e.kind];
    if (art && frames(art.name)) {
      // A pixel sprite centred on the enemy's point, feet below it.
      this.body = sprite(art.name, art.scale, 0.15);
      this.body.y = this.footY = this.body.height / 2;
      this.motion = art.float ? 'float' : art.hop ? 'hop' : null;
      this.phase = Math.random() * 6;
      if (art.alpha) this.body.alpha = art.alpha;
      this.baseTint = art.tint ?? 0xffffff;
      this.body.tint = this.baseTint;
      this.r = this.body.height / 2;
    } else {
      this.body = new Graphics();
      drawEnemy(this.body, e.kind);
      this.baseTint = 0xffffff;
    }
    // Status effects: a frost ring while slowed, a shield arc for the bone knight.
    this.fx = new Graphics();
    this.root.addChild(this.body, this.fx);
    layer.world.addChild(this.root);

    this.bar = new Graphics();
    layer.labels.addChild(this.bar);
    const p = toPx(e.x, e.y);
    this.root.position.set(p.x, p.y);
    this.target = p;
    this.hp = -1;
    this.flags = '';
    this.set(e);
  }

  set(e) {
    this.target = toPx(e.x, e.y);
    this.burn = !!e.burn;
    if (e.hp !== this.hp) {
      this.hp = e.hp;
      const w = Math.max(36, this.r * 2);
      const f = Math.max(0, e.hp / e.maxHp);
      this.bar.clear();
      this.bar.rect(-w / 2, 0, w, 6).fill({ color: 0x000000, alpha: 0.7 });
      this.bar.rect(-w / 2, 0, w * f, 6).fill(f > 0.5 ? 0x5fd35f : f > 0.25 ? 0xe0b03a : 0xe04a3a);
    }
    const flags = (e.slow ? 's' : '') + (e.shield ? 'h' : '');
    if (flags !== this.flags) {
      this.flags = flags;
      const r = this.r;
      this.fx.clear();
      if (e.slow) this.fx.ellipse(0, r * 0.8, r * 1.1, r * 0.35).stroke({ width: 3, color: 0x8fd4ff });
      if (e.shield) this.fx.arc(0, 0, r + 8, -2.2, 2.2).stroke({ width: 5, color: 0xffe08a });
    }
  }

  frame(k, t) {
    const r = this.root;
    const dx = this.target.x - r.x;
    r.x += dx * k;
    r.y += (this.target.y - r.y) * k;
    if (Math.abs(dx) > 0.5 && this.body.anchor) {
      // Sprites face the way they walk.
      this.body.scale.x = Math.abs(this.body.scale.x) * (dx < 0 ? -1 : 1);
    }
    if (this.motion === 'float') this.body.y = this.footY - 6 + Math.sin(t * 3 + this.phase) * 5;
    else if (this.motion === 'hop') this.body.y = this.footY - Math.abs(Math.sin(t * 12 + this.phase)) * 5;
    // Burning enemies flicker orange.
    this.body.tint = this.burn && Math.sin(t * 20) > 0 ? 0xffa060 : this.baseTint;
    this.bar.position.set(r.x, r.y - this.r - (this.boss ? 34 : 14));
  }

  destroy() {
    this.root.destroy({ children: true });
    this.bar.destroy();
  }
}

class TowerView {
  constructor(t, layer) {
    this.kind = t.kind;
    const p = toPx(t.x, t.y);
    this.cx = p.x + CELL / 2;
    this.cy = p.y + CELL / 2;
    // Graphics cannot hold children in Pixi 8, so the base and the pack's
    // sprites sit side by side in a container.
    this.root = new Container();
    this.g = new Graphics();
    this.root.addChild(this.g);
    this.art = null;
    this.root.position.set(this.cx, this.cy);
    // A web of silk or frost drawn while a boss keeps the building disabled.
    this.web = new Graphics();
    this.web.position.set(this.cx, this.cy);
    this.web.visible = false;
    for (let i = 0; i < 4; i++) {
      const dx = Math.cos(i * 0.8) * 40;
      const dy = Math.sin(i * 0.8) * 40;
      this.web.moveTo(-dx, -dy).lineTo(dx, dy);
    }
    this.web.circle(0, 0, 16).circle(0, 0, 30);
    this.web.stroke({ width: 2, color: 0xe8f4ff, alpha: 0.85 });
    layer.world.addChild(this.root, this.web);

    this.name = new Text({
      text: t.owner,
      style: { fontFamily: FONT_UI, fontSize: 15, fontWeight: '700', fill: nickColor(t.owner), stroke: { color: 0x000000, width: 4 } },
    });
    this.name.anchor.set(0.5, 0);
    this.name.position.set(this.cx, this.cy + 30);
    this.bar = new Graphics();
    this.bar.position.set(this.cx, this.cy - 44);
    this.charges = new Text({ text: '', style: { fontFamily: FONT_PIXEL, fontSize: 11, fill: 0xe8e8e8, stroke: { color: 0x000000, width: 4 } } });
    this.charges.anchor.set(0, 1); // top left: the quality dots take the right edge
    this.charges.position.set(this.cx - 44, this.cy - 26);
    layer.labels.addChild(this.name, this.bar, this.charges);
    this.level = 0;
    this.built = null;
    this.barKey = '';
    this.set(t);
  }

  set(t) {
    this.disabled = !!t.disabled;
    // One bar above the cell: construction progress, or a barricade's health.
    const key = `${t.progress ?? 0}|${t.hp ?? 0}`;
    if (key !== this.barKey) {
      this.barKey = key;
      this.bar.clear();
      if (t.progress > 0) {
        this.bar.rect(-30, 0, 60, 7).fill({ color: 0x000000, alpha: 0.75 });
        this.bar.rect(-30, 0, 60 * t.progress, 7).fill(0xffcf5a);
      } else if (t.maxHp > 0) {
        const f = Math.max(0, t.hp / t.maxHp);
        this.bar.rect(-30, 0, 60, 7).fill({ color: 0x000000, alpha: 0.75 });
        this.bar.rect(-30, 0, 60 * f, 7).fill(f > 0.5 ? 0xb0a080 : 0xe04a3a);
      }
    }
    const trap = this.kind === 'trap' && t.built;
    this.charges.text = trap ? `×${t.charges ?? 0}` : '';
    this.empty = trap && !t.charges; // a spent trap waits for the next wave
    if (t.lvl === this.level && t.built === this.built) return;
    this.level = t.lvl;
    this.built = t.built;
    const g = this.g;
    g.clear();
    if (!t.built) {
      // Construction site: scaffolding and a ghost of the future building.
      g.roundRect(-34, -34, 68, 68, 10).fill({ color: 0x1b1a1f, alpha: 0.5 }).stroke({ width: 3, color: 0x8a7a5a, alpha: 0.9 });
      g.moveTo(-28, 28).lineTo(28, -28).moveTo(-28, -28).lineTo(28, 28).stroke({ width: 3, color: 0x6b5638, alpha: 0.8 });
      return;
    }
    if (!this.art && hasTileset()) {
      this.art = towerArt(this.kind, sprite);
      if (this.art) this.root.addChildAt(this.art, this.kind === 'trap' || this.kind === 'wall' ? 0 : 1);
    }
    drawTower(g, this.kind, RARITY[Math.min(t.lvl, 5) - 1], t.lvl, { pixel: hasTileset(), art: !!this.art });
  }

  frame(t, enemies) {
    // A disabled building turns grey, the colour the design keeps for that.
    this.root.tint = this.disabled ? 0x777777 : 0xffffff;
    this.root.alpha = !this.built ? 0.85 : this.empty ? 0.45 : 1;
    this.web.visible = this.disabled;
    this.web.rotation = Math.sin(t) * 0.05;
    if (this.art?.aimSprite && enemies) this.aim(enemies);
  }

  // aim turns the ballista toward the nearest enemy in reach, one of four
  // directions as the sheet draws it.
  aim(enemies) {
    let best = null;
    let bestD = 300 * 300;
    for (const e of enemies.values()) {
      const dx = e.root.x - this.cx;
      const dy = e.root.y - this.cy;
      const d = dx * dx + dy * dy;
      if (d < bestD) {
        bestD = d;
        best = [dx, dy];
      }
    }
    if (!best) return;
    const [dx, dy] = best;
    const dir = Math.abs(dx) > Math.abs(dy) ? (dx > 0 ? 'right' : 'left') : dy > 0 ? 'down' : 'up';
    const s = this.art.aimSprite;
    const t = texture(`${s.aim}_${dir}`);
    if (t && s.texture !== t) s.texture = t;
  }

  destroy() {
    this.root.destroy({ children: true });
    this.web.destroy();
    this.name.destroy();
    this.bar.destroy();
    this.charges.destroy();
  }
}


// A chatter's mercenary on the field: a small figure in the nick's colour
// with the nick above it. It bobs while walking, swings a hammer while
// building and shows stars while stunned.
class WorkerView {
  constructor(w, layer) {
    const color = nickColor(w.name);
    this.root = new Container();
    const hero = hasTileset() && HERO_ART[w.class];
    if (hero) {
      // The pack's hero for the class; it runs while walking.
      this.idle = frames(`${hero}_idle`);
      this.run = frames(`${hero}_run`) ?? this.idle;
      this.body = sprite(`${hero}_idle`, 2.5, 0.15);
      this.height = this.body.height;
    } else {
      this.body = new Graphics();
      drawFigure(this.body, color, w.class);
      this.height = 46;
    }
    const lantern = new Graphics();
    lantern.circle(14, -8, 4).fill(0xffd27a).stroke({ width: 1.5, color: 0x000000 });
    this.root.addChild(this.body, lantern);
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
    const dx = this.target.x - r.x;
    r.x += dx * k;
    r.y += (this.target.y - r.y) * k;
    const moving = this.state === 'walk' || this.state === 'return' || this.state === 'flee';
    if (this.idle) {
      const want = moving ? this.run : this.idle;
      if (this.body.textures !== want) {
        this.body.textures = want;
        this.body.play();
      }
      if (Math.abs(dx) > 0.5) this.body.scale.x = Math.abs(this.body.scale.x) * (dx < 0 ? -1 : 1);
    } else {
      this.body.y = moving ? -Math.abs(Math.sin(this.t * 10)) * 4 : 0;
    }
    this.tool.rotation = this.state === 'build' ? Math.sin(this.t * 14) * 0.9 : 0.3;
    this.stars.visible = this.state === 'flee' || this.state === 'rest';
    this.stars.position.set(r.x, r.y - this.height - 24);
    this.stars.rotation = Math.sin(this.t * 6) * 0.3;
    this.name.position.set(r.x, r.y - this.height - 2);
  }

  destroy() {
    this.root.destroy({ children: true });
    this.name.destroy();
    this.stars.destroy();
  }
}

class ProjView {
  constructor(p, layer) {
    this.kind = p.kind;
    this.root = new Graphics();
    drawProjectile(this.root, p.kind);
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
