// The tavern on the left of the screen: every active chatter's mercenary sits
// at a long table with its nick and gold above it, walks to the bar to drink,
// and disappears through the gate while it works on the field.
//
// Layout, top to bottom: fireplace, tables with seats right under it, the
// bar counter with the barkeeper, and the fame board in the bottom left
// corner, clear of the default webcam (bottom right of the screen).
import { H, TOP, LEFT, CELL, FONT_UI, FONT_PIXEL, nickColor } from './layout.js';
import { drawFigure, CLASSES } from './figures.js';
import { HERO_ART } from './sprites.js';
import { hasTileset, sprite, frames } from './tileset.js';

const { Container, Graphics, Text } = PIXI;

const COLS_X = [64, 192, 320];
const ROW_Y0 = TOP + 172; // first row of feet: its labels clear the fireplace
const ROW_DY = 80;
const ROWS = 6;
const SEATS = COLS_X.length * ROWS;
const BAR_Y = H - 274; // counter top edge; the keeper and the board fit below
const SPEED = 170; // px per second

const DRINK_NAMES = { ale: 'эль', mead: 'медовуха', potion: 'зелье' };

export class Tavern {
  constructor() {
    this.world = new Container(); // lit by the lightmap
    this.ui = new Container(); // labels, board and card stay readable
    this.interior = new Graphics();
    this.world.addChild(this.interior);
    this.drawInterior();
    this.figures = new Container();
    this.world.addChild(this.figures);
    this.labels = new Container();
    this.ui.addChild(this.labels);

    this.mercs = new Map(); // name -> Merc
    this.seats = new Array(SEATS).fill(null);
    this.gateY = TOP + (H - TOP) / 2;
    this.limits = { cap: 0, towers: 0 };

    this.board = new FameBoard();
    this.ui.addChild(this.board.root);
    this.overflow = new Text({ text: '', style: label(18, 0xc9b98f) });
    this.overflow.anchor.set(0.5, 0);
    this.overflow.position.set(LEFT / 2, ROW_Y0 + ROWS * ROW_DY - 24);
    this.ui.addChild(this.overflow);
    this.card = new Card();
    this.ui.addChild(this.card.root);
  }

  drawInterior() {
    const g = this.interior;
    g.rect(0, TOP, LEFT, H - TOP).fill(0x3a2716);
    for (let y = TOP; y < H; y += 32) g.rect(0, y, LEFT, 2).fill(0x2a1b0f); // planks
    for (let x = 0; x < LEFT; x += 96) g.rect(x, TOP, 2, H - TOP).fill({ color: 0x2a1b0f, alpha: 0.6 });
    // Fireplace.
    g.rect(LEFT / 2 - 70, TOP + 8, 140, 70).fill(0x2b2b2b);
    g.rect(LEFT / 2 - 50, TOP + 28, 100, 50).fill(0x120c08);
    g.ellipse(LEFT / 2, TOP + 66, 34, 14).fill(0xff8a3a);
    g.ellipse(LEFT / 2, TOP + 62, 18, 10).fill(0xffd27a);
    // Two long tables between the seat columns.
    for (const x of [128, 256]) {
      g.roundRect(x - 20, ROW_Y0 - 40, 40, ROWS * ROW_DY - 8, 8).fill(0x5a3a1f).stroke({ width: 3, color: 0x2a1b0f });
      for (let r = 0; r < ROWS; r++) {
        const y = ROW_Y0 - 16 + r * ROW_DY;
        const flask = hasTileset() && sprite(['flask_red', 'flask_yellow', 'flask_green', 'flask_blue'][r % 4], 2);
        if (flask) {
          flask.position.set(x, y);
          this.world.addChild(flask);
        } else {
          g.rect(x - 8, y - 10, 16, 10).fill(0x8a6a3a); // mugs
        }
      }
    }
    // Bar counter with the barkeeper behind it: a dwarf, as taverns go.
    const keeper = hasTileset() ? sprite('dwarf_m_idle', 2, 0.1) : new Graphics();
    if (!hasTileset()) drawFigure(keeper, 0x7a3a2a, '');
    keeper.position.set(LEFT / 2, BAR_Y + 62);
    this.world.addChild(keeper);
    g.rect(16, BAR_Y, LEFT - 48, 30).fill(0x5a3a1f).stroke({ width: 3, color: 0x2a1b0f });
    g.rect(16, BAR_Y, LEFT - 48, 6).fill(0x7a5230);
    for (let i = 0; i < 6; i++) {
      const bottle = hasTileset() && sprite(i % 2 ? 'flask_big_red' : 'flask_big_yellow', 2);
      if (bottle) {
        bottle.position.set(37 + i * 52, BAR_Y + 2);
        this.world.addChild(bottle);
      } else {
        g.rect(30 + i * 52, BAR_Y - 14, 14, 14).fill(0x8a6a3a);
      }
    }
  }

  setMap(map) {
    this.gateY = TOP + map.route[map.route.length - 1][1] * CELL;
  }

  update(st) {
    this.limits = { cap: st.walletCap, towers: st.towerLimit };
    const outside = new Set((st.workers ?? []).map((w) => w.name));
    const active = st.chatters.filter((c) => c.active || c.towers > 0 || outside.has(c.name));
    const seen = new Set();
    let hidden = 0;
    for (const c of active) {
      seen.add(c.name);
      let m = this.mercs.get(c.name);
      if (!m) {
        const seat = this.seats.indexOf(null);
        if (seat < 0) {
          hidden++;
          continue;
        }
        m = new Merc(c, seat, this);
        this.seats[seat] = c.name;
        this.mercs.set(c.name, m);
      }
      m.set(c, outside.has(c.name));
    }
    for (const [name, m] of this.mercs) {
      if (!seen.has(name)) {
        this.seats[m.seat] = null;
        m.destroy();
        this.mercs.delete(name);
      }
    }
    this.overflow.text = hidden ? `и ещё ${hidden} в зале` : '';
    this.board.update(active);
    for (const ev of st.events ?? []) this.onEvent(ev);
  }

  onEvent(ev) {
    const m = this.mercs.get(ev.user);
    if (ev.kind === 'drink' && m) m.drink(DRINK_NAMES[ev.text] ?? ev.text);
    if (ev.kind === 'card') this.card.show(ev.user, this.mercs.get(ev.user), this.limits, ev.amount ?? 0);
    if (ev.kind === 'class' && m) m.redraw = true;
  }

  frame(dt) {
    for (const m of this.mercs.values()) m.frame(dt);
    this.board.frame(dt);
    this.card.frame(dt);
  }
}

function label(size, fill, font = FONT_UI) {
  return { fontFamily: font, fontSize: size, fontWeight: '700', fill, stroke: { color: 0x000000, width: 4 } };
}

function seatPos(i) {
  const col = i % COLS_X.length;
  const row = Math.floor(i / COLS_X.length);
  return { x: COLS_X[col], y: ROW_Y0 + row * ROW_DY };
}

class Merc {
  constructor(c, seat, tavern) {
    this.tavern = tavern;
    this.seat = seat;
    this.name = c.name;
    this.cls = c.class;
    this.color = nickColor(c.name);
    this.root = new Container();
    this.makeBody();
    this.mug = new Graphics();
    this.mug.roundRect(8, -20, 10, 12, 2).fill(0xc8a165).stroke({ width: 2, color: 0x000000 });
    this.mug.rect(9, -22, 8, 4).fill(0xfff6d8);
    this.mug.visible = false;
    this.root.addChild(this.mug);
    tavern.figures.addChild(this.root);

    this.nameText = new Text({ text: short(c.name), style: label(16, this.color) });
    this.nameText.anchor.set(0.5, 1);
    this.goldText = new Text({ text: '', style: label(11, 0xffd24a, FONT_PIXEL) });
    this.goldText.anchor.set(0.5, 1);
    this.bubble = new Text({ text: '', style: label(15, 0xfff1c2) });
    this.bubble.anchor.set(0.5, 1);
    tavern.labels.addChild(this.nameText, this.goldText, this.bubble);

    const p = seatPos(seat);
    this.root.position.set(p.x, p.y);
    this.mode = 'seat';
    this.t = Math.random() * 10;
    this.drunk = 0;
    this.gold = -1;
    this.out = false;
  }

  set(c, out) {
    if (c.gold !== this.gold) {
      this.gold = c.gold;
      this.goldText.text = `${c.gold} з`;
    }
    this.drunk = c.drunk;
    this.kills = c.kills;
    this.towers = c.towers;
    this.classKey = c.class;
    if (c.class !== this.cls || this.redraw) {
      this.cls = c.class;
      this.redraw = false;
      this.makeBody();
    }
    if (out && !this.out) {
      this.mode = 'out';
    } else if (!out && this.out) {
      // Back through the gate, then home to the seat.
      this.root.position.set(LEFT - 24, this.tavern.gateY);
      this.mode = 'seat';
    }
    this.out = out;
  }

  // makeBody draws the class's hero from the pack, or the vector figure.
  makeBody() {
    if (this.body) this.body.destroy();
    const hero = hasTileset() && HERO_ART[this.cls];
    if (hero) {
      this.idle = frames(`${hero}_idle`);
      this.run = frames(`${hero}_run`) ?? this.idle;
      this.body = sprite(`${hero}_idle`, 2, 0.12);
      this.height = this.body.height;
    } else {
      this.idle = this.run = null;
      this.body = new Graphics();
      drawFigure(this.body, this.color, this.cls);
      this.height = 46;
    }
    this.root.addChildAt(this.body, 0);
  }

  drink(what) {
    if (this.out) return;
    this.mode = 'bar';
    this.drinkName = what;
  }

  target() {
    if (this.mode === 'bar' || this.mode === 'drinking') {
      return { x: 40 + (this.seat % 6) * 52, y: BAR_Y - 6 };
    }
    return seatPos(this.seat);
  }

  frame(dt) {
    this.t += dt;
    const r = this.root;
    const visible = this.mode !== 'out';
    r.visible = this.nameText.visible = this.goldText.visible = visible;
    if (!visible) {
      this.bubble.visible = false;
      return;
    }
    const tgt = this.target();
    const dx = tgt.x - r.x;
    const dy = tgt.y - r.y;
    const dist = Math.hypot(dx, dy);
    const walking = dist > 2;
    if (walking) {
      const s = Math.min(dist, SPEED * dt);
      r.x += (dx / dist) * s;
      r.y += (dy / dist) * s;
    } else if (this.mode === 'bar') {
      this.mode = 'drinking';
      this.drinkTime = 2.5;
    }
    if (this.mode === 'drinking') {
      this.drinkTime -= dt;
      if (this.drinkTime <= 0) this.mode = 'seat';
    }

    // Drunk mercenaries sway, very drunk ones sing.
    const sway = Math.min(this.drunk, 6);
    this.body.rotation = sway ? Math.sin(this.t * 2.5) * 0.05 * sway : 0;
    if (this.idle) {
      const want = walking ? this.run : this.idle;
      if (this.body.textures !== want) {
        this.body.textures = want;
        this.body.play();
      }
      if (walking && Math.abs(dx) > 1) this.body.scale.x = Math.abs(this.body.scale.x) * (dx < 0 ? -1 : 1);
    } else {
      this.body.y = walking ? -Math.abs(Math.sin(this.t * 10)) * 3 : Math.sin(this.t * 1.5) * 0.8;
    }
    this.mug.visible = this.mode === 'drinking';
    this.mug.rotation = this.mode === 'drinking' ? -0.4 - Math.abs(Math.sin(this.t * 4)) * 0.6 : 0;

    let bubble = '';
    if (this.mode === 'drinking') bubble = `${this.drinkName}!`;
    else if (this.drunk >= 4) bubble = Math.floor(this.t * 2) % 2 ? '♪ ♫' : '♫ ♪';
    this.bubble.text = bubble;
    this.bubble.visible = !!bubble;
    this.bubble.position.set(r.x, r.y - this.height - 32);
    this.nameText.position.set(r.x, r.y - this.height - 14);
    this.goldText.position.set(r.x, r.y - this.height);
  }

  destroy() {
    this.root.destroy({ children: true });
    this.nameText.destroy();
    this.goldText.destroy();
    this.bubble.destroy();
  }
}

function short(name) {
  return name.length > 13 ? name.slice(0, 12) + '…' : name;
}

// FameBoard rotates three top-5 lists: richest, deadliest, thirstiest.
class FameBoard {
  constructor() {
    this.root = new Container();
    this.root.position.set(16, H - 204); // bottom left, under the bar
    const bg = new Graphics();
    bg.roundRect(0, 0, LEFT - 46, 196, 8).fill(0xd9c49a).stroke({ width: 4, color: 0x5a3a1f });
    bg.roundRect(6, 6, LEFT - 58, 184, 6).stroke({ width: 1, color: 0x8a6a3a });
    this.root.addChild(bg);
    this.title = new Text({ text: '', style: { fontFamily: FONT_UI, fontSize: 24, fontWeight: '700', fill: 0x3a2210 } });
    this.title.anchor.set(0.5, 0);
    this.title.position.set((LEFT - 46) / 2, 10);
    this.root.addChild(this.title);
    this.rows = [];
    for (let i = 0; i < 5; i++) {
      const n = new Text({ text: '', style: { fontFamily: FONT_UI, fontSize: 19, fontWeight: '700', fill: 0x2a1a0c } });
      const v = new Text({ text: '', style: { fontFamily: FONT_PIXEL, fontSize: 13, fill: 0x5a2a0c } });
      n.position.set(18, 44 + i * 29);
      v.anchor.set(1, 0);
      v.position.set(LEFT - 66, 49 + i * 29);
      this.root.addChild(n, v);
      this.rows.push({ n, v });
    }
    this.pages = [
      { title: 'Богачи', key: 'gold', unit: 'з' },
      { title: 'Убийцы', key: 'kills', unit: '' },
      { title: 'Пьяницы', key: 'drinks', unit: 'кр.' },
    ];
    this.page = 0;
    this.time = 0;
    this.chatters = [];
  }

  update(chatters) {
    this.chatters = chatters;
    this.render();
  }

  render() {
    // Skip empty pages so the board never shows a blank list.
    for (let tries = 0; tries < this.pages.length; tries++) {
      const p = this.pages[this.page];
      const top = this.chatters.filter((c) => c[p.key] > 0).sort((a, b) => b[p.key] - a[p.key]).slice(0, 5);
      if (top.length || tries === this.pages.length - 1) {
        this.title.text = p.title;
        this.rows.forEach((r, i) => {
          const c = top[i];
          r.n.text = c ? `${i + 1}. ${short(c.name)}` : '';
          r.v.text = c ? `${c[p.key]} ${p.unit}`.trim() : '';
        });
        return;
      }
      this.page = (this.page + 1) % this.pages.length;
    }
  }

  frame(dt) {
    this.time += dt;
    if (this.time > 8) {
      this.time = 0;
      this.page = (this.page + 1) % this.pages.length;
      this.render();
    }
  }
}

// Card is the parchment !me shows next to the mercenary for a few seconds.
class Card {
  constructor() {
    this.root = new Container();
    this.root.visible = false;
    const bg = new Graphics();
    bg.roundRect(0, 0, 250, 132, 8).fill(0xe6d3a8).stroke({ width: 4, color: 0x5a3a1f });
    this.root.addChild(bg);
    const st = (size, fill = 0x2a1a0c) => ({ fontFamily: FONT_UI, fontSize: size, fontWeight: '700', fill });
    this.name = new Text({ text: '', style: st(22) });
    this.cls = new Text({ text: '', style: st(16, 0x6a4a2a) });
    this.lines = new Text({ text: '', style: { ...st(17), lineHeight: 24 } });
    this.name.position.set(14, 8);
    this.cls.position.set(14, 36);
    this.lines.position.set(14, 60);
    this.root.addChild(this.name, this.cls, this.lines);
    this.time = 0;
  }

  show(user, merc, limits, towers) {
    this.name.text = short(user);
    this.cls.text = CLASSES[merc?.classKey]?.name ?? '';
    this.lines.text = `золото ${merc?.gold ?? '?'} / ${limits.cap}\nпостроек ${towers} / ${limits.towers} · убийств ${merc?.kills ?? 0}`;
    const x = merc && !merc.out ? merc.root.x : LEFT / 2;
    const y = merc && !merc.out ? merc.root.y : TOP + 360;
    this.root.position.set(Math.max(8, Math.min(LEFT + 120, x + 24)), Math.max(TOP + 8, y - 150));
    this.root.visible = true;
    this.time = 5;
  }

  frame(dt) {
    if (!this.root.visible) return;
    this.time -= dt;
    this.root.alpha = Math.min(1, this.time * 2);
    if (this.time <= 0) this.root.visible = false;
  }
}
