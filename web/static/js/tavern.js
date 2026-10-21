// The tavern on the left of the screen: every active chatter's mercenary sits
// at a long table with its nick and gold above it, walks to the bar to drink,
// and disappears through the gate while it works on the field.
//
// Layout, top to bottom: fireplace, tables with seats right under it, two
// dice tables, the bar counter with the barkeeper, and the fame board in the
// bottom left corner, clear of the default webcam (bottom right).
import { H, TOP, LEFT, CELL, FONT_UI, FONT_PIXEL, nickColor, RANKS } from './layout.js';
import { drawFigure, CLASSES } from './figures.js';
import { HERO_ART } from './sprites.js';
import { hasTileset, sprite, frames, texture, PX } from './tileset.js';

const { Container, Graphics, Text } = PIXI;

const COLS_X = [64, 192, 320];
const ROW_Y0 = TOP + 172; // first row of feet: its labels clear the fireplace
const ROW_DY = 80;
const ROWS = 5;
const SEATS = COLS_X.length * ROWS;
const BAR_Y = H - 274; // counter top edge; the keeper and the board fit below
const DICE_Y = ROW_Y0 + ROWS * ROW_DY + 20; // feet of dice players, between seats and bar
const DICE_X = [96, 288]; // dice tables; players sit 48 px to either side
const SPEED = 170; // px per second
const WOOD = 0xa89078; // tint for Kenney's light oak, so it sits in the gloom

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
    this.overflow.anchor.set(1, 0.5); // beside the barkeeper
    this.overflow.position.set(LEFT - 24, BAR_Y + 46);
    this.ui.addChild(this.overflow);
    this.card = new Card();
    this.ui.addChild(this.card.root);
    this.dice = new DiceTables();
    this.ui.addChild(this.dice.root);
  }

  drawInterior() {
    const g = this.interior;
    g.rect(0, TOP, LEFT, H - TOP).fill(0x3a2716);
    for (let y = TOP; y < H; y += 32) g.rect(0, y, LEFT, 2).fill(0x2a1b0f); // planks
    for (let x = 0; x < LEFT; x += 96) g.rect(x, TOP, 2, H - TOP).fill({ color: 0x2a1b0f, alpha: 0.6 });
    if (hasTileset() && texture('k_bar')) this.drawFurniture();
    else this.drawVectorFurniture();
    // The barkeeper: a dwarf, as taverns go.
    const keeper = hasTileset() ? sprite('dwarf_m_idle', 2, 0.1) : new Graphics();
    if (!hasTileset()) drawFigure(keeper, 0x7a3a2a, '');
    keeper.position.set(LEFT / 2, BAR_Y + 62);
    this.world.addChild(keeper);
    // Everyone knows the barkeeper by name.
    const bob = new Text({ text: 'Боб', style: label(15, 0xf2d48a) });
    bob.anchor.set(0.5, 1);
    bob.position.set(LEFT / 2, BAR_Y + 62 - (hasTileset() ? keeper.height : 46) - 2);
    this.ui.addChild(bob);
  }

  // drawFurniture builds the hall from sprites: the 0x72 back wall with a
  // hearth, and Kenney's tables, chairs and bar counter, darkened to fit.
  drawFurniture() {
    const put = (name, x, y, scale = PX, speed) => {
      const s = sprite(name, scale, speed);
      if (s) {
        s.position.set(x, y);
        if (name.startsWith('k_')) s.tint = WOOD;
        this.world.addChild(s);
      }
      return s;
    };
    const T = 16 * PX;
    // Back wall: a cap row and a brick row, banners and shields on it.
    for (let x = T / 2; x < LEFT; x += T) {
      put('wall_top_mid', x, TOP + T);
      put('wall_mid', x, TOP + 2 * T);
    }
    put('wall_banner_red', T * 1.5, TOP + 2 * T);
    put('wall_banner_green', LEFT - T * 1.5, TOP + 2 * T);
    put('k_shield', T / 2, TOP + 2 * T - 6, 2.5);
    put('k_shield', LEFT - T / 2, TOP + 2 * T - 6, 2.5);
    // Hearth: a stone surround with a mantel and a fire in a dark mouth.
    const hearth = new Graphics();
    this.world.addChild(hearth);
    const cx = LEFT / 2;
    hearth.rect(cx - 64, TOP + 22, 128, 2 * T - 22).fill(0x3a3532).stroke({ width: 3, color: 0x141010 });
    for (let y = TOP + 40; y < TOP + 2 * T; y += 14) hearth.rect(cx - 62, y, 124, 2).fill(0x24201e); // courses
    hearth.roundRect(cx - 44, TOP + 44, 88, 2 * T - 44, 18).fill(0x0c0806);
    hearth.rect(cx - 72, TOP + 14, 144, 12).fill(0x5a4632).stroke({ width: 3, color: 0x141010 }); // mantel
    put('campfire', cx, TOP + 2 * T + 2, 1.7, 0.15) ?? hearth.ellipse(cx, TOP + 84, 26, 10).fill(0xff8a3a);
    put('k_candles', cx - 50, TOP + 18, 2);
    put('k_candles', cx + 50, TOP + 18, 2);
    // Two long tables between the seat columns, a chair under every seat.
    const top = ROW_Y0 - 40;
    const len = ROWS * ROW_DY - 8;
    const TS = 2.5; // table scale: 40 px wide
    for (const x of [128, 256]) {
      put('k_table_top', x, top + 16 * TS, TS);
      const mid = put('k_table_mid', x, top + len - 16 * TS, TS);
      if (mid) mid.scale.y = (len - 32 * TS) / 2;
      put('k_table_bot', x, top + len, TS);
    }
    for (let i = 0; i < SEATS; i++) {
      const p = seatPos(i);
      put(i % COLS_X.length ? 'k_chair_l' : 'k_chair_r', p.x, p.y + 4, TS);
    }
    this.drawFlasks(new Graphics());
    // Dice tables: an oval table with a green felt and a stool either side.
    const felt = new Graphics();
    for (const x of DICE_X) {
      put('k_chair_r', x - 48, DICE_Y + 4, TS);
      put('k_chair_l', x + 48, DICE_Y + 4, TS);
      put('k_oval_l', x - 24, DICE_Y - 2);
      put('k_oval_r', x + 24, DICE_Y - 2);
      felt.ellipse(x, DICE_Y - 30, 38, 11).fill(0x2f5a32).stroke({ width: 2, color: 0x1a2a1a });
    }
    this.world.addChild(felt);
    // The bar counter: cabinets and bottle racks, flasks on top.
    const bar = ['k_bar', 'k_bar_bottles', 'k_bar_drawers', 'k_bar_bottles2', 'k_bar_drawers', 'k_bar_bottles', 'k_bar'];
    bar.forEach((name, i) => put(name, 16 + T / 2 + i * T, BAR_Y + T));
    for (let i = 0; i < 4; i++) put(i % 2 ? 'flask_big_red' : 'flask_big_yellow', 48 + i * 96, BAR_Y + 12, 2);
  }

  drawFlasks(g) {
    for (const x of [128, 256]) {
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
  }

  // drawVectorFurniture is the plain fallback while the tileset is missing.
  drawVectorFurniture() {
    const g = this.interior;
    // Fireplace.
    g.rect(LEFT / 2 - 70, TOP + 8, 140, 70).fill(0x2b2b2b);
    g.rect(LEFT / 2 - 50, TOP + 28, 100, 50).fill(0x120c08);
    g.ellipse(LEFT / 2, TOP + 66, 34, 14).fill(0xff8a3a);
    g.ellipse(LEFT / 2, TOP + 62, 18, 10).fill(0xffd27a);
    // Two long tables between the seat columns.
    for (const x of [128, 256]) {
      g.roundRect(x - 20, ROW_Y0 - 40, 40, ROWS * ROW_DY - 8, 8).fill(0x5a3a1f).stroke({ width: 3, color: 0x2a1b0f });
    }
    this.drawFlasks(g);
    // Dice tables: a round table with green felt and two stools.
    for (const x of DICE_X) {
      g.ellipse(x - 48, DICE_Y + 2, 14, 6).fill(0x2a1b0f);
      g.ellipse(x + 48, DICE_Y + 2, 14, 6).fill(0x2a1b0f);
      g.ellipse(x, DICE_Y - 18, 40, 20).fill(0x5a3a1f).stroke({ width: 3, color: 0x2a1b0f });
      g.ellipse(x, DICE_Y - 20, 32, 14).fill(0x2f5a32);
    }
    // Bar counter.
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
    // Mercenaries out building, and druids out hunting as panthers, leave the tavern.
    const outside = new Set([...(st.workers ?? []).filter((w) => w.state !== 'rest'), ...(st.panthers ?? []), ...(st.runs ?? [])].map((w) => w.name));
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
    // Players at a dice table leave their seats for its stools.
    const spots = this.dice.update(st.dice ?? []);
    for (const m of this.mercs.values()) m.diceSpot = spots.get(m.name) ?? null;
    for (const ev of st.events ?? []) this.onEvent(ev);
  }

  onEvent(ev) {
    const m = this.mercs.get(ev.user);
    if (ev.kind === 'drink' && m) m.drink(DRINK_NAMES[ev.text] ?? ev.text);
    if (ev.kind === 'card') this.card.show(ev.user, this.mercs.get(ev.user), this.limits, ev.amount ?? 0);
    if (ev.kind === 'class' && m) m.redraw = true;
    if (ev.kind === 'diceRoll') this.dice.result(ev);
    if (ev.kind === 'robbed' && m) m.shout(`−${ev.amount} з, ЭссДиЭф!`, 4);
    if (ev.kind === 'loot' && m) m.shout(`+${ev.amount} з вернулись`, 4);
    if (ev.kind === 'pickpocket') {
      // A rogue lifts a few coins from someone at the next table.
      m?.shout(`+${ev.amount} з, хе-хе`, 3);
      this.mercs.get(ev.text)?.shout(`−${ev.amount} з?!`, 3);
    }
  }

  frame(dt) {
    for (const m of this.mercs.values()) m.frame(dt);
    this.board.frame(dt);
    this.card.frame(dt);
    this.dice.frame(dt);
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
    this.out = false; // out on the field
    this.asleep = false;
  }

  set(c, out) {
    this.music = !!c.music;
    if ((c.rank ?? 0) !== this.rank) {
      // In the tavern the nick shows the rank colour; out on the field it
      // keeps the personal colour, so buildings stay easy to tell apart.
      this.rank = c.rank ?? 0;
      this.nameText.style.fill = RANKS[this.rank]?.color ?? RANKS[0].color;
    }
    if (c.gold !== this.gold) {
      this.gold = c.gold;
      this.goldText.text = `${c.gold} з`;
    }
    this.drunk = c.drunk;
    this.asleep = !!c.out; // passed out under the table (not the same as out on the field)
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

  // shout shows a line over the mercenary for a few seconds.
  shout(text, secs = 3) {
    this.shoutText = text;
    this.shoutLeft = secs;
  }

  drink(what) {
    if (this.out) return;
    this.mode = 'bar';
    this.drinkName = what;
  }

  target() {
    if (this.diceSpot) return this.diceSpot; // at a dice table, nothing else
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

    // Drunk mercenaries sway, very drunk ones sing, the overdrunk lie
    // under the table snoring.
    const sway = Math.min(this.drunk, 6);
    if (this.asleep) this.body.rotation = -Math.PI / 2 + Math.sin(this.t * 1.2) * 0.04;
    else this.body.rotation = sway ? Math.sin(this.t * 2.5) * 0.05 * sway : 0;
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
    this.shoutLeft = Math.max(0, (this.shoutLeft ?? 0) - dt);
    if (this.shoutLeft > 0) bubble = this.shoutText;
    else if (this.asleep) bubble = ['z', 'zZ', 'zZz'][Math.floor(this.t * 1.5) % 3];
    else if (this.mode === 'drinking') bubble = `${this.drinkName}!`;
    else if (this.drunk >= 4) bubble = Math.floor(this.t * 2) % 2 ? '♪ ♫' : '♫ ♪';
    this.bubble.text = bubble;
    this.bubble.visible = !!bubble;
    this.bubble.position.set(r.x, r.y - this.height - 32);
    this.nameText.position.set(r.x, r.y - this.height - 14);
    this.goldText.position.set(r.x, r.y - this.height);
    this.playNotes(dt, r);
  }

  // playNotes: while a bard plays, notes hop up from him and fade.
  playNotes(dt, r) {
    this.notes = this.notes ?? [];
    this.noteIn = (this.noteIn ?? 0) - dt;
    if (this.music && this.noteIn <= 0) {
      this.noteIn = 0.3;
      const t = new Text({ text: Math.random() < 0.5 ? '♪' : '♫', style: { fontSize: 20, fontWeight: '700', fill: 0xffd24a, stroke: { color: 0x2a1a0c, width: 4 } } });
      t.anchor.set(0.5);
      this.tavern.labels.addChild(t);
      this.notes.push({ t, life: 1.4, dx: (Math.random() - 0.5) * 40 });
    }
    this.notes = this.notes.filter((n) => {
      n.life -= dt;
      if (n.life <= 0) {
        n.t.destroy();
        return false;
      }
      const k = 1 - n.life / 1.4;
      // Up and a little aside, bouncing on the beat, fading at the top.
      n.t.position.set(r.x + n.dx * k, r.y - this.height - 10 - 60 * k - Math.abs(Math.sin(k * 12)) * 8);
      n.t.alpha = Math.min(1, n.life * 2);
      return true;
    });
  }

  destroy() {
    this.root.destroy({ children: true });
    this.nameText.destroy();
    this.goldText.destroy();
    this.bubble.destroy();
    for (const n of this.notes ?? []) n.t.destroy();
  }
}

function short(name) {
  return name.length > 13 ? name.slice(0, 12) + '…' : name;
}

// HELP pages remind the chat of its commands between the lists.
const HELP = [
  {
    title: 'Команды: стройка',
    rows: [
      ['!b C4 arrow', 'построить'],
      ['!up C4', 'улучшить'],
      ['!play', 'войти в игру'],
      ['!помощь', 'все команды'],
      ['!me', 'мой наёмник'],
    ],
  },
  {
    title: 'Команды: таверна',
    rows: [
      ['!drink', 'выпить эля'],
      ['!кости 50', 'ставка в кости'],
      ['!кости принять', 'сыграть'],
      ['!class', 'сменить класс'],
      ['!души', 'души и ачивки'],
    ],
  },
  {
    title: 'Постройки',
    rows: [
      ['arrow', 'лучники'],
      ['magic/curse', 'магия, порча'],
      ['fire/shrine', 'огонь, храм'],
      ['drums/totem', 'ритм, гроза'],
      ['trap/wall', 'на дорогу'],
    ],
  },
];

// FameBoard rotates three top-5 lists (richest, deadliest, thirstiest) with
// a page of commands between every two of them.
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
    this.help = 0; // which HELP page comes next
    this.onHelp = false; // every other turn of the board is a page of commands
    this.time = 0;
    this.chatters = [];
  }

  update(chatters) {
    this.chatters = chatters;
    this.render();
  }

  render() {
    if (this.onHelp) {
      this.show(HELP[this.help].title, HELP[this.help].rows);
      return;
    }
    // Skip empty lists; with every list empty the board teaches commands.
    for (let tries = 0; tries < this.pages.length; tries++) {
      const p = this.pages[this.page];
      const top = this.chatters.filter((c) => c[p.key] > 0).sort((a, b) => b[p.key] - a[p.key]).slice(0, 5);
      if (top.length) {
        this.show(p.title, top.map((c, i) => [`${i + 1}. ${short(c.name)}`, `${c[p.key]} ${p.unit}`.trim()]));
        return;
      }
      this.page = (this.page + 1) % this.pages.length;
    }
    this.show(HELP[this.help].title, HELP[this.help].rows);
  }

  show(title, rows) {
    this.title.text = title;
    this.rows.forEach((r, i) => {
      r.n.text = rows[i]?.[0] ?? '';
      r.v.text = rows[i]?.[1] ?? '';
    });
  }

  frame(dt) {
    this.time += dt;
    if (this.time > 8) {
      this.time = 0;
      if (this.onHelp) {
        this.help = (this.help + 1) % HELP.length;
        this.page = (this.page + 1) % this.pages.length;
      }
      this.onHelp = !this.onHelp;
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
    // The rank, in its colour, on the class line at the right.
    this.rank = new Text({ text: '', style: { ...st(15), stroke: { color: 0x1a0f08, width: 4 } } });
    this.rank.anchor.set(1, 0);
    this.rank.position.set(238, 37);
    this.name.position.set(14, 8);
    this.cls.position.set(14, 36);
    this.lines.position.set(14, 60);
    this.root.addChild(this.name, this.cls, this.lines, this.rank);
    this.time = 0;
  }

  show(user, merc, limits, towers) {
    this.name.text = short(user);
    const rank = RANKS[merc?.rank ?? 0] ?? RANKS[0];
    this.rank.text = rank.name;
    this.rank.style.fill = rank.color;
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

// DiceTables shows what lies on each dice table: the stake and the time
// left while the host waits, a flicker of numbers while the dice roll, and
// the two results with the winner's pot afterwards.
class DiceTables {
  constructor() {
    this.root = new Container();
    this.felt = DICE_X.map((x) => {
      const t = new Text({ text: '', style: label(13, 0xfff1c2, FONT_PIXEL) });
      t.anchor.set(0.5);
      t.position.set(x, DICE_Y - 20);
      this.root.addChild(t);
      return t;
    });
    this.tables = []; // last known games, to place a result on its table
    this.shown = DICE_X.map(() => null); // result on display: { text, life }
    this.t = 0;
  }

  // update returns where each player sits: name -> { x, y }.
  update(games) {
    this.tables = games;
    const spots = new Map();
    for (const d of games) {
      const x = DICE_X[d.table] ?? DICE_X[0];
      spots.set(d.host, { x: x - 48, y: DICE_Y });
      if (d.guest) spots.set(d.guest, { x: x + 48, y: DICE_Y });
    }
    // Keep players on their stools while the result is on display.
    this.shown.forEach((s, i) => {
      if (s && s.life > 0) {
        spots.set(s.host, { x: DICE_X[i] - 48, y: DICE_Y });
        spots.set(s.guest, { x: DICE_X[i] + 48, y: DICE_Y });
      }
    });
    return spots;
  }

  result(ev) {
    const [host, a, guest, b] = (ev.text ?? '').split(':');
    const i = this.lastTables?.find((d) => d.host === host && d.guest === guest)?.table ?? 0;
    this.shown[i] = { text: `${a} : ${b}`, life: 4, host, guest, winner: ev.user, pot: ev.amount };
  }

  frame(dt) {
    this.t += dt;
    this.lastTables = this.tables.length ? this.tables : this.lastTables;
    DICE_X.forEach((_, i) => {
      const t = this.felt[i];
      const s = this.shown[i];
      if (s && s.life > 0) {
        s.life -= dt;
        t.text = `${s.text}\n+${s.pot}`;
        t.style.fill = 0xffd24a;
        return;
      }
      const d = this.tables.find((g) => g.table === i);
      if (!d) {
        t.text = '';
      } else if (!d.guest) {
        t.text = `${d.stake}\n${Math.ceil(d.wait)}с`; // waiting for an opponent
        t.style.fill = 0xfff1c2;
      } else {
        // The dice are rolling: numbers tumble until the server throws.
        const r = () => 1 + Math.floor(Math.random() * 20);
        const tick = Math.floor(this.t * 12); // 12 changes a second
        if (tick !== this.flip) {
          this.flip = tick;
          t.text = `${r()} : ${r()}`;
        }
        t.style.fill = 0xffffff;
      }
    });
  }
}
