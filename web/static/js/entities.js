// Dynamic entities. The server sends positions 20 times a second; sprites
// glide toward the latest position so motion stays smooth at 60 fps.
import { W, CELL, LEFT, toPx, RARITY, FONT_UI, FONT_PIXEL, nickColor } from './layout.js';
import { drawFigure } from './figures.js';
import { ENEMIES, TOWERS, ENEMY_ART, HERO_ART, towerArt, drawEnemy, drawTower, drawProjectile, emoji, softGlow } from './sprites.js';
import { hasTileset, sprite, frames, texture } from './tileset.js';
import { isWater } from './map.js';

const { Container, Graphics, Text, Sprite, Assets } = PIXI;

// The Yellow Sign as the streamer drew it (yellow on black, so added onto
// the field), the one the king shows close up before it lands; SIGN_H is its
// height over a marked building. Until it loads the sign is drawn here.
export const SIGN_H = 64;
let signTex = null;
Assets.load('assets/king/sign/yellow_sign.jpg').then((t) => (signTex = t)).catch(() => {});

// Icons of an artifact ballista's arrows.
const AMMO_ICONS = { poison: '☠️', fire: '🔥', blast: '💥' };

// How long !me keeps the speaker's buildings lit.
const HIGHLIGHT_SEC = 6;

// A shieldbearer covers everyone within GUARD cells (enemy guard in config.go).
const GUARD = 1.2;
// His cover rolls out as a ring every GUARD_EVERY seconds, like the drums'.
const GUARD_EVERY = 1.6;

// Drums of war: their aura reaches DRUM_RANGE cells (the drums' auraRange in
// config.go), shown as a pulse every DRUM_EVERY seconds.
const DRUM_RANGE = 3;
const DRUM_EVERY = 5;

export class Entities {
  constructor() {
    this.world = new Container(); // affected by lighting
    this.labels = new Container(); // nameplates and HP bars, always readable
    this.enemies = new Map();
    this.towers = new Map();
    this.proj = new Map();
    this.workers = new Map();
    this.panthers = new Map(); // druids on the hunt
    this.runs = new Map(); // charging warriors, shadow-stepping rogues, bears, wolves
    this.gateFires = new Map(); // pyromancers' braziers on the tavern wall
    this.t = 0;
  }

  clear() {
    for (const m of [this.enemies, this.towers, this.proj, this.workers, this.panthers, this.runs, this.gateFires]) {
      for (const v of m.values()) v.destroy();
      m.clear();
    }
  }

  update(state) {
    sync(this.enemies, state.enemies, (e) => new EnemyView(e, this), (v, e) => v.set(e));
    // A building carried to another cell (!перенести) is drawn anew there.
    for (const t of state.towers) {
      const v = this.towers.get(t.id);
      if (v && (v.x !== t.x || v.y !== t.y)) {
        v.destroy();
        this.towers.delete(t.id);
      }
    }
    sync(this.towers, state.towers, (t) => new TowerView(t, this), (v, t) => v.set(t));
    sync(this.proj, state.proj, (p) => new ProjView(p, this), (v, p) => v.set(p));
    // A mercenary resting after a scare is back inside: it sits in the tavern,
    // not at the gate, even while enemies batter the gate till the wave ends.
    const workers = (state.workers ?? []).filter((w) => w.state !== 'rest').map((w) => ({ ...w, id: w.name }));
    sync(this.workers, workers, (w) => new WorkerView(w, this), (v, w) => v.set(w));
    for (const ev of state.events ?? []) if (ev.kind === 'say' && this.workers.has(ev.user)) this.bubbles?.show(`field:${ev.user}`, ev.text, ev.emotes, 5);
    const panthers = (state.panthers ?? []).map((p) => ({ ...p, id: p.name }));
    sync(this.panthers, panthers, (p) => new PantherView(p, this), (v, p) => v.set(p));
    // Two wolves share their shaman's name: key by name, kind and order.
    const runs = (state.runs ?? []).map((r, i) => ({ ...r, id: `${r.name}|${r.kind}|${i}` }));
    const fires = (state.gateFires ?? []).map((f) => ({ ...f, id: f.name }));
    sync(this.gateFires, fires, (f) => new GateFireView(f, this), (v, f) => v.set(f));
    sync(this.runs, runs, (r) => new RunView(r, this), (v, r) => v.set(r));
    // !me lights up the speaker's buildings for a few seconds.
    for (const ev of state.events ?? []) {
      if (ev.kind !== 'card') continue;
      for (const v of this.towers.values()) if (v.owner === ev.user) v.highlight(HIGHLIGHT_SEC);
    }
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
    for (const v of this.panthers.values()) v.frame(k, this.t);
    for (const v of this.runs.values()) v.frame(Math.min(1, dt * 20), this.t);
    for (const v of this.gateFires.values()) v.frame(this.t);
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
    for (const w of this.workers.values()) out.push({ x: w.root.x, y: w.root.y - 10, r: 80, color: 0xffcf8a, a: w.veil ? 0.15 : 0.7 }); // a veiled rogue shades it
    for (const p of this.panthers.values()) out.push({ x: p.root.x, y: p.root.y - 10, r: 70, color: 0xb0ff8a, a: 0.6 });
    // Spirit wolves give off a faint cold glow of their own.
    for (const r of this.runs.values()) if (r.kind === 'wolf') out.push({ x: r.root.x, y: r.root.y - 14, r: 60, color: 0x9ef0ff, a: 0.5 });
    for (const p of this.proj.values()) {
      if (p.kind === 'magic') out.push({ x: p.root.x, y: p.root.y, r: 70, color: 0xb58cff, a: 0.8 });
      if (p.kind === 'fire') out.push({ x: p.root.x, y: p.root.y, r: 90, color: 0xff7a2a, a: 0.9 });
    }
    for (const f of this.gateFires.values()) out.push({ x: f.cx, y: f.cy, r: 130, color: 0xff7a2a, a: 0.9 });
    for (const e of this.enemies.values()) {
      if (e.kind === 'golden_golem') out.push({ x: e.root.x, y: e.root.y, r: 170, color: 0xffd24a, a: 0.8 }); // it glitters
      else if (e.kind === 'king') out.push({ x: e.root.x, y: e.root.y + 40, r: 190 + 40 * (e.opened ?? 0), color: 0xfff2c0, a: 0.85 }); // a pale glow, so his yellow stays yellow; wider with the cloak open
      else if (e.boss) out.push({ x: e.root.x, y: e.root.y, r: 160, color: 0xff3b3b, a: 0.6 });
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
    let art = hasTileset() && ENEMY_ART[e.kind];
    if (art && !frames(art.name) && art.fallback) art = art.fallback; // the king's own sprite not drawn yet
    if (art && frames(art.name)) {
      // A pixel sprite centred on the enemy's point, feet below it.
      this.body = sprite(art.name, art.scale, art.speed ?? 0.15);
      this.body.y = this.footY = this.body.height / 2;
      this.motion = art.float ? 'float' : art.hop ? 'hop' : null;
      this.art = art;
      this.heading = 'side';
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
    if (this.art?.boat) {
      // A pirate stands in a rowboat, bow first; the boat shows its wounds.
      this.boat = sprite('boat_0', 1.5);
      if (this.boat) {
        this.boat.anchor.set(0.5);
        this.root.addChildAt(this.boat, 0);
        this.body.y = this.footY = this.body.height / 2 - 6;
        this.boatFrame = 'boat_0';
      }
    }
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
    // At the gate the route ends just past the wall: keep the whole body on
    // the field side, so big bosses do not stand inside the tavern. (Far
    // left of the wall is ЭссДиЭф robbing the tavern, that one stays.)
    if (e.x > -1) {
      const half = this.body.anchor ? Math.abs(this.body.width) / 2 : this.r;
      this.target.x = Math.max(this.target.x, LEFT + half * 0.8);
    }
    this.burn = !!e.burn;
    this.rage = !!e.rage;
    this.cursed = !!e.curse;
    this.act = e.act ?? 0;
    if (e.hp !== this.hp) {
      this.hp = e.hp;
      const w = Math.max(36, this.r * 2);
      const f = Math.max(0, e.hp / e.maxHp);
      this.bar.clear();
      this.bar.rect(-w / 2, 0, w, 6).fill({ color: 0x000000, alpha: 0.7 });
      this.bar.rect(-w / 2, 0, w * f, 6).fill(f > 0.5 ? 0x5fd35f : f > 0.25 ? 0xe0b03a : 0xe04a3a);
    }
    if (this.boat) {
      const name = e.sunk ? '' : e.boat > 0.6 ? 'boat_0' : e.boat > 0.3 ? 'boat_1' : 'boat_2';
      if (name !== this.boatFrame) {
        this.boatFrame = name;
        this.boat.visible = !!name;
        if (name) this.boat.texture = texture(name);
      }
    }
    this.dive = !!e.dive;
    this.bar.visible = !this.dive;
    if (e.open && !this.breath) {
      // The King in Yellow's cloak is open (nets and slows take now): it
      // breathes, a soft golden light swelling behind him and motes of it
      // drifting up the cloak (see frame).
      this.breath = softGlow(0xffc83a, this.r * 2.2);
      this.motes = new Graphics();
      this.motes.blendMode = 'add';
      this.root.addChildAt(this.breath, 0);
      // The cloak itself glows on each breath: the body again, added on top.
      if (this.body.texture) {
        this.shine = new PIXI.Sprite(this.body.texture);
        this.shine.anchor.copyFrom(this.body.anchor);
        this.shine.blendMode = 'add';
        this.shine.tint = 0xffb020;
        this.root.addChildAt(this.shine, this.root.getChildIndex(this.body) + 1);
      }
      this.root.addChild(this.motes);
      this.opened = 0;
    }
    this.open = !!e.open;
    const flags = (e.slow ? 's' : '') + (e.shield ? 'h' : '') + (e.net ? 'n' : '') + (e.stun ? 'z' : '') + (e.swim ? 'w' : '') + (e.dive ? 'd' : '') + (e.open ? 'o' : '');
    if (flags !== this.flags) {
      this.flags = flags;
      const r = this.r;
      this.fx.clear();
      if (e.swim || e.dive) {
        // Swimming: rings of ripples round the waist; under water only they show.
        this.fx.ellipse(0, r * 0.4, r * 1.1, r * 0.35).stroke({ width: 2, color: 0xbfe8ff, alpha: 0.7 });
        this.fx.ellipse(0, r * 0.4, r * 1.5, r * 0.5).stroke({ width: 2, color: 0xbfe8ff, alpha: 0.35 });
      }
      if (e.slow) this.fx.ellipse(0, r * 0.8, r * 1.1, r * 0.35).stroke({ width: 3, color: 0x8fd4ff });
      if (e.shield) this.fx.arc(0, 0, r + 8, -2.2, 2.2).stroke({ width: 5, color: 0xffe08a });
      if (e.stun) {
        // Dazed by holy light: a soft golden halo over the head.
        this.fx.ellipse(0, -r - 8, r * 0.6, 6).stroke({ width: 3, color: 0xffe08a, alpha: 0.9 });
      }
      if (e.net) {
        // A rope net thrown over the body: a diamond mesh inside a ring.
        const n = r + 6;
        const s = Math.SQRT1_2;
        for (let d = -n + 5; d < n; d += 10) {
          const h = Math.sqrt(n * n - d * d); // half the chord at distance d
          // Chords along both diagonals, so the mesh stays inside the ring.
          this.fx.moveTo(d * s - h * s, d * -s - h * s).lineTo(d * s + h * s, d * -s + h * s);
          this.fx.moveTo(d * s - h * s, d * s + h * s).lineTo(d * s + h * s, d * s - h * s);
        }
        this.fx.stroke({ width: 2, color: 0xd8c08a, alpha: 0.9 });
        this.fx.circle(0, 0, n).stroke({ width: 3, color: 0x8a6a3a });
      }
    }
  }

  frame(k, t) {
    const r = this.root;
    const dx = this.target.x - r.x;
    const dy = this.target.y - r.y;
    if (Math.hypot(dx, dy) > CELL * 3) {
      r.position.set(this.target.x, this.target.y); // a teleport, not a walk
    } else {
      r.x += dx * k;
      r.y += dy * k;
    }
    if (this.art?.dirs) {
      this.animate(dx, dy);
    } else if (Math.abs(dx) > 0.5 && this.body.anchor && !this.art?.front) {
      // Sprites face the way they walk.
      this.body.scale.x = Math.abs(this.body.scale.x) * (dx < 0 ? -1 : 1);
    }
    if (this.boat && Math.hypot(dx, dy) > 0.5) {
      // The bow leads, the pirate keeps facing us.
      const want = Math.atan2(dy, dx) + Math.PI / 2;
      let d = want - this.boat.rotation;
      d = Math.atan2(Math.sin(d), Math.cos(d));
      this.boat.rotation += d * 0.15;
      this.body.scale.x = Math.abs(this.body.scale.x);
    }
    this.body.alpha = this.dive ? 0.18 : this.art?.alpha ?? 1;
    if (this.motion === 'float') this.body.y = this.footY - 6 + Math.sin(t * 3 + this.phase) * 5;
    else if (this.motion === 'hop') this.body.y = this.footY - Math.abs(Math.sin(t * 12 + this.phase)) * 5;
    // Raging enemies throb red, burning ones flicker orange.
    if (this.rage) this.body.tint = Math.sin(t * 8) > 0 ? 0xff4a3a : 0xff9a8a;
    else if (this.cursed) this.body.tint = Math.sin(t * 5) > 0 ? 0xc07aff : 0xe0c0ff;
    else this.body.tint = this.burn && Math.sin(t * 20) > 0 ? 0xffa060 : this.baseTint;
    this.bar.position.set(r.x, r.y - this.r - (this.boss ? 34 : 14));
    if (this.breath) this.breathe(k, t);
    if (this.kind === 'shieldbearer') this.guardPulse(t);
  }

  // guardPulse shows the cover a shieldbearer gives: a faint ring rolling out
  // to GUARD cells every GUARD_EVERY seconds, as the drums' pulse does.
  guardPulse(t) {
    if (!this.guard) {
      this.guard = new Graphics();
      this.root.addChildAt(this.guard, 0);
      this.guardPhase = Math.random() * GUARD_EVERY;
    }
    const k = ((t + this.guardPhase) % GUARD_EVERY) / 1.2; // the ring lives 1.2 s
    this.guard.clear();
    if (k >= 1) return;
    this.guard.circle(0, 0, (0.25 + 0.75 * k) * GUARD * CELL).stroke({ width: 3, color: 0x9ac8ff, alpha: 0.35 * (1 - k) });
  }

  // breathe swells the open cloak's light in and out, fading it in as the
  // cloak opens and out as it closes.
  breathe(k, t) {
    this.opened += ((this.open ? 1 : 0) - this.opened) * Math.min(1, k * 0.5);
    const o = this.opened;
    const b = 0.5 + 0.5 * Math.sin(t * 3.2);
    const h = this.r * 2;
    const foot = this.footY ?? this.r;
    this.breath.position.set(0, foot - h * 0.5);
    this.breath.scale.set(((this.r * 1.7) / 64) * (0.85 + 0.3 * b), ((this.r * 2.3) / 64) * (0.85 + 0.3 * b));
    this.breath.alpha = o * (0.75 + 0.25 * b);
    if (this.shine) {
      this.shine.texture = this.body.texture;
      this.shine.position.copyFrom(this.body.position);
      this.shine.scale.copyFrom(this.body.scale);
      this.shine.alpha = o * 0.55 * b;
    }
    this.motes.clear();
    if (o < 0.02) return;
    for (let i = 0; i < 12; i++) {
      const p = (t * 0.45 + i / 12) % 1; // each mote rises from the hem and fades
      const x = Math.sin(i * 2.4 + t * 1.3) * this.r * 0.75;
      const y = foot - p * h * 1.05;
      this.motes.circle(x, y, 3 + 3 * (1 - p)).fill({ color: 0xffe08a, alpha: o * Math.sin(p * Math.PI) });
    }
  }

  // animate picks the walk cycle for the heading, or plays an act (the
  // burp or the fart) once over the time he stands still for it.
  animate(dx, dy) {
    const a = this.art;
    if (!this.act && Math.hypot(dx, dy) > 0.3) {
      this.heading = Math.abs(dx) >= Math.abs(dy) ? 'side' : dy > 0 ? 'down' : 'up';
      if (this.heading === 'side') this.facing = dx < 0 ? -1 : 1;
    }
    const name = this.act ? a.acts[this.act] : a.dirs[this.heading];
    const want = frames(name);
    if (want && this.body.textures !== want) {
      this.body.textures = want;
      this.body.loop = !this.act;
      this.body.animationSpeed = this.act ? want.length / (2 * 60) : a.speed ?? 0.15;
      this.body.gotoAndPlay(0);
    }
    // Side views face right in the sheet; front, back and the burp face us.
    const sideways = name === a.dirs.side || name === a.acts[2];
    this.body.scale.x = Math.abs(this.body.scale.x) * (sideways ? this.facing ?? 1 : 1);
  }

  destroy() {
    this.root.destroy({ children: true });
    this.bar.destroy();
  }
}

class TowerView {
  constructor(t, layer) {
    this.kind = t.kind;
    this.owner = t.owner;
    this.x = t.x;
    this.y = t.y;
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
    // The leviathan's tide over the building.
    this.wave = new Graphics();
    this.wave.position.set(this.cx, this.cy);
    this.wave.roundRect(-44, -44, 88, 88, 18).fill({ color: 0x2a6a9a, alpha: 0.8 });
    for (let i = -1; i <= 1; i++) this.wave.moveTo(-34, i * 20).quadraticCurveTo(-17, i * 20 - 8, 0, i * 20).quadraticCurveTo(17, i * 20 + 8, 34, i * 20);
    this.wave.stroke({ width: 3, color: 0xbfe8ff, alpha: 0.8 });
    this.wave.visible = false;
    // The Yellow Sign over a marked building: the drawn one once it loads
    // (see frame), three hooked arms round a ring till then.
    this.signMark = new Container();
    this.signMark.position.set(this.cx, this.cy - 8);
    const mark = new Graphics();
    this.signMark.addChild(mark);
    mark.circle(0, 0, 9).stroke({ width: 5, color: 0xffd23a });
    for (let i = 0; i < 3; i++) {
      const a = (i * Math.PI * 2) / 3 - Math.PI / 2;
      const x = Math.cos(a);
      const y = Math.sin(a);
      mark.moveTo(x * 12, y * 12).lineTo(x * 30, y * 30).quadraticCurveTo(x * 40 - y * 14, y * 40 + x * 14, x * 30 - y * 18, y * 30 + x * 18);
    }
    mark.stroke({ width: 5, color: 0xffd23a, cap: 'round' });
    this.signMark.visible = false;
    layer.world.addChild(this.root, this.web, this.wave);
    layer.labels.addChild(this.signMark);
    // The frog a swamp witch turns the building into for a while.
    this.frog = sprite('tc_frog', 4) ?? emoji('🐸', 40);
    this.frog.position.set(this.cx, this.cy + (this.frog.anchor?.y === 1 ? 24 : 0));
    this.frog.visible = false;
    layer.world.addChild(this.frog);
    // An artifact ballista's arrows, as an icon over its top left corner.
    this.ammo = '';
    this.ammoIcon = new Text({ text: '', style: { fontSize: 20, fontFamily: 'Segoe UI Emoji, Apple Color Emoji, Noto Color Emoji, sans-serif' } });
    this.ammoIcon.anchor.set(0.5);
    this.ammoIcon.position.set(this.cx - 32, this.cy - 30);
    layer.labels.addChild(this.ammoIcon);
    // The ring !me lights around the speaker's buildings, in his nick colour.
    this.glow = new Graphics();
    this.glow.position.set(this.cx, this.cy);
    this.glow.visible = false;
    this.glowLeft = 0;
    layer.labels.addChild(this.glow);

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
    this.hex = !!t.hex;
    this.flood = !!t.flood;
    this.signed = !!t.sign;
    this.burnt = !!t.burnt; // on ground the King in Yellow burned
    this.carry = !!t.carry; // being carried elsewhere by its mercenary
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
    this.charges.text = trap ? (t.artifact ? '×∞' : `×${t.charges ?? 0}`) : '';
    this.empty = trap && !t.charges && !t.artifact; // a spent trap waits for the next wave
    this.war = !!t.war;
    if ((t.ammo ?? '') !== this.ammo) {
      this.ammo = t.ammo ?? '';
      this.ammoIcon.text = AMMO_ICONS[this.ammo] ?? '';
    }
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
    const water = isWater(t.x, t.y);
    if (!this.art && hasTileset() && !water) {
      this.art = towerArt(this.kind, sprite);
      if (this.art) this.root.addChildAt(this.art, this.kind === 'trap' || this.kind === 'wall' ? 0 : 1);
    }
    drawTower(g, this.kind, RARITY[Math.min(t.lvl, 6) - 1], t.lvl, { pixel: hasTileset(), art: !!this.art, water });
  }

  highlight(secs) {
    this.glowLeft = secs;
  }

  frame(t, enemies) {
    this.drawGlow(t);
    // A disabled building turns grey, the colour the design keeps for that.
    this.root.tint = this.disabled ? 0x777777 : 0xffffff;
    this.root.alpha = !this.built ? 0.85 : this.carry ? 0.5 : this.empty ? 0.45 : 1;
    this.web.visible = this.disabled && !this.hex && !this.flood && !this.burnt && !this.carry; // silenced, not webbed
    // Under the tide the building sinks into the water.
    this.wave.visible = this.flood;
    this.signMark.visible = this.signed;
    if (this.signed) {
      if (signTex && !this.signMark.drawn) {
        const s = new Sprite(signTex);
        s.anchor.set(0.5);
        s.blendMode = 'add';
        s.scale.set(SIGN_H / signTex.height);
        const halo = new Graphics();
        for (let r = 40; r > 0; r -= 8) halo.circle(0, 0, r).fill({ color: 0xffd23a, alpha: 0.07 });
        this.signMark.removeChildren().forEach((c) => c.destroy());
        this.signMark.addChild(halo, s);
        this.signMark.drawn = true;
      }
      // It sways and glows, the drawn sign upright, the stand-in turning.
      this.signMark.rotation = this.signMark.drawn ? Math.sin(t * 1.5) * 0.12 : t * 0.6;
      this.signMark.scale.set(1 + 0.06 * Math.sin(t * 5));
      this.signMark.alpha = 0.8 + 0.2 * Math.sin(t * 5);
    }
    if (this.flood) this.wave.alpha = 0.55 + 0.15 * Math.sin(t * 4);
    this.web.rotation = Math.sin(t) * 0.05;
    this.frog.visible = this.hex;
    if (this.hex) this.frog.scale.y = Math.abs(this.frog.scale.y) * (1 + 0.06 * Math.sin(t * 6)); // croaking
    if (this.art?.aimSprite && enemies) this.aim(enemies);
    if (this.kind === 'drums') this.drumPulse(t);
  }

  // drawGlow pulses a ring in the owner's colour round the cell while !me
  // keeps the building lit, fading out over the last second.
  drawGlow(t) {
    const g = this.glow;
    const dt = t - (this.lastT ?? t);
    this.lastT = t;
    if (this.glowLeft <= 0) {
      // A war beat from artifact drums lights the building amber.
      g.clear();
      if (this.war) {
        g.roundRect(-44, -44, 88, 88, 12).stroke({ width: 4, color: 0xffb347, alpha: 0.8 });
        g.visible = true;
      } else g.visible = false;
      return;
    }
    this.glowLeft -= dt;
    const fade = Math.min(1, Math.max(0, this.glowLeft));
    const pulse = 0.6 + 0.4 * Math.sin(t * 6);
    const color = nickColor(this.owner);
    g.clear();
    g.roundRect(-44, -44, 88, 88, 12).fill({ color, alpha: 0.12 * fade }).stroke({ width: 5, color, alpha: pulse * fade });
    g.visible = true;
  }

  // drumPulse shows the reach of the drums' aura now and then: a faint warm
  // ring that rolls out to the edge of it every few seconds, like a beat.
  drumPulse(t) {
    if (!this.pulse) {
      this.pulse = new Graphics();
      this.root.addChildAt(this.pulse, 0);
      this.phase = Math.random() * DRUM_EVERY;
    }
    const k = ((t + this.phase) % DRUM_EVERY) / 1.4; // the ring lives 1.4 s
    this.pulse.clear();
    if (k >= 1 || !this.built || this.disabled) return;
    this.pulse.circle(0, 0, (0.25 + 0.75 * k) * DRUM_RANGE * CELL).stroke({ width: 3, color: 0xffb36b, alpha: 0.28 * (1 - k) });
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
    this.wave.destroy();
    this.signMark.destroy({ children: true });
    this.frog.destroy();
    this.glow.destroy();
    this.ammoIcon.destroy();
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
    this.layer = layer;
    this.bubbleId = `field:${w.name}`;
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
    this.veil = !!w.veil;
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
    // A veiled rogue: barely there, the nick too.
    this.root.alpha = this.veil ? 0.3 + 0.12 * Math.sin(this.t * 5) : 1;
    this.name.alpha = this.veil ? 0.55 : 1;
    this.stars.visible = this.state === 'flee' || this.state === 'rest';
    this.stars.position.set(r.x, r.y - this.height - 24);
    this.stars.rotation = Math.sin(this.t * 6) * 0.3;
    this.name.position.set(r.x, r.y - this.height - 2);
    this.layer.bubbles?.place(this.bubbleId, r.x, r.y - this.height - 22, LEFT, W);
  }

  destroy() {
    this.root.destroy({ children: true });
    this.layer.bubbles?.hide(this.bubbleId);
    this.name.destroy();
    this.stars.destroy();
  }
}

class ProjView {
  constructor(p, layer) {
    this.kind = p.kind;
    if (p.kind === 'clown') {
      this.root = emoji('🤡', 30); // clowns throw clowns
    } else if (p.kind === 'note') {
      // A bard's drums throw notes while the music plays.
      this.root = new Text({ text: Math.random() < 0.5 ? '♪' : '♫', style: { fontSize: 30, fontWeight: '700', fill: 0xffd24a, stroke: { color: 0x2a1a0c, width: 4 } } });
      this.root.anchor.set(0.5);
    } else {
      this.root = new Graphics();
      drawProjectile(this.root, p.kind);
    }
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

// A druid turned black panther, mauling enemies on the field: Shepardskin's
// black cat, scaled up, running.
class PantherView {
  constructor(p, layer) {
    this.root = new Container();
    this.body = hasTileset() && sprite('panther_run', 3.4, 0.25);
    if (!this.body) {
      this.body = new Graphics();
      this.body.ellipse(0, -10, 22, 11).fill(0x1a1a22).circle(18, -18, 8).fill(0x1a1a22).circle(21, -20, 2).fill(0xd8ff5a);
    }
    this.root.addChild(this.body);
    this.name = new Text({ text: p.name, style: { fontFamily: FONT_UI, fontSize: 14, fontWeight: '700', fill: nickColor(p.name), stroke: { color: 0x000000, width: 4 } } });
    this.name.anchor.set(0.5, 1);
    this.name.y = -this.body.height - 4;
    this.root.addChild(this.name);
    const px = toPx(p.x, p.y);
    this.root.position.set(px.x, px.y + 20);
    this.target = { x: this.root.x, y: this.root.y };
    this.t = Math.random() * 6;
    layer.world.addChild(this.root);
  }

  set(p) {
    const px = toPx(p.x, p.y);
    this.target = { x: px.x, y: px.y + 20 };
  }

  frame(k, t) {
    const dx = this.target.x - this.root.x;
    this.root.x += dx * k;
    this.root.y += (this.target.y - this.root.y) * k;
    // The cat runs to the left in its sheet: mirror it when heading right.
    if (Math.abs(dx) > 0.5 && this.body.anchor) this.body.scale.x = Math.abs(this.body.scale.x) * (dx < 0 ? 1 : -1);
  }

  destroy() {
    this.root.destroy({ children: true });
  }
}

// A mercenary out on a class skill: a Воитель charging down the path (the
// pack's knight, running), or a rogue in a shadow-step (the bandit, dark
// and flickering).
class RunView {
  constructor(r, layer) {
    this.kind = r.kind;
    this.root = new Container();
    const charge = r.kind === 'charge';
    if (r.kind === 'bear') this.body = hasTileset() && sprite('tc_bear', 4.2);
    else if (r.kind === 'wolf') {
      // A spectral wolf: a wolf's run, pale blue and see-through.
      this.body = hasTileset() && sprite('wolf_run', 1.3, 0.3);
      if (this.body) {
        this.body.tint = 0x9ef0ff;
        this.body.alpha = 0.7;
      }
    } else this.body = hasTileset() && sprite(charge ? 'knight_m_run' : 'rogue_idle', 2.6, 0.3);
    if (!this.body) {
      this.body = new Graphics();
      if (r.kind === 'bear') this.body.ellipse(0, -18, 26, 18).fill(0x6a4a2a);
      else if (r.kind === 'wolf') this.body.ellipse(0, -10, 20, 9).fill({ color: 0x9ef0ff, alpha: 0.7 });
      else drawFigure(this.body, charge ? 0x9aa0a8 : 0x4a2a6a, charge ? 'warrior' : 'rogue');
    }
    this.root.addChild(this.body);
    this.name = new Text({ text: r.name, style: { fontFamily: FONT_UI, fontSize: 14, fontWeight: '700', fill: nickColor(r.name), stroke: { color: 0x000000, width: 4 } } });
    this.name.anchor.set(0.5, 1);
    this.name.y = -(this.body.height || 46) - 4;
    this.root.addChild(this.name);
    const px = toPx(r.x, r.y);
    this.root.position.set(px.x, px.y + 20);
    this.target = { x: this.root.x, y: this.root.y };
    layer.world.addChild(this.root);
  }

  set(r) {
    const px = toPx(r.x, r.y);
    this.target = { x: px.x, y: px.y + 20 };
    this.hold = !!r.hold;
  }

  frame(k, t) {
    const dx = this.target.x - this.root.x;
    this.root.x += dx * k;
    this.root.y += (this.target.y - this.root.y) * k;
    // The wolf sheet runs left, the heroes right: mirror whichever way it goes.
    const faceLeft = this.kind === 'wolf' ? dx > 0 : dx < 0;
    if (Math.abs(dx) > 0.5 && this.body.anchor) this.body.scale.x = Math.abs(this.body.scale.x) * (faceLeft ? -1 : 1);
    if (this.kind === 'step') this.body.alpha = 0.55 + 0.35 * Math.sin(t * 30); // a shadow, not quite there
    if (this.kind === 'wolf') this.body.alpha = 0.55 + 0.2 * Math.sin(t * 12);
    // A bear holding the line rears and swipes.
    if (this.kind === 'bear') this.body.rotation = this.hold ? Math.sin(t * 9) * 0.12 : 0;
  }

  destroy() {
    this.root.destroy({ children: true });
  }
}

// A pyromancer at the brazier on the tavern wall by the door: a campfire
// with its level dots, burning while he shoots from it.
class GateFireView {
  constructor(f, layer) {
    this.root = new Container();
    this.g = new Graphics();
    this.body = hasTileset() && sprite('campfire', 1.6, 0.18);
    if (this.body) this.root.addChild(this.body);
    else this.g.ellipse(0, -8, 16, 10).fill(0xff6a1a);
    this.root.addChild(this.g);
    this.name = new Text({ text: '', style: { fontFamily: FONT_UI, fontSize: 13, fontWeight: '700', fill: nickColor(f.name), stroke: { color: 0x000000, width: 4 } } });
    this.name.anchor.set(0.5, 1);
    this.name.y = -44;
    this.root.addChild(this.name);
    const px = toPx(f.x, f.y);
    this.cx = px.x;
    this.cy = px.y;
    this.root.position.set(px.x, px.y + 24);
    layer.world.addChild(this.root);
    this.level = 0;
    this.set(f);
  }

  set(f) {
    this.name.text = `${f.name} · ${f.left} с`;
    if (f.lvl !== this.level) {
      this.level = f.lvl;
      this.g.clear();
      for (let i = 0; i < f.lvl; i++) this.g.circle(-((f.lvl - 1) * 8) / 2 + i * 8, 6, 3).fill(RARITY[Math.min(f.lvl, 6) - 1]);
    }
  }

  frame(t) {
    this.root.scale.set(1 + 0.04 * Math.sin(t * 7));
  }

  destroy() {
    this.root.destroy({ children: true });
  }
}
