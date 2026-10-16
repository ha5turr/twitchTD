// Top bar, tavern, floating numbers and banners.
import { W, H, TOP, LEFT, CELL, toPx, BIOMES, FONT_UI, FONT_PIXEL, nickColor } from './layout.js';

const { Container, Graphics, Text } = PIXI;

const ui = (size, fill = 0xf2e6c9, extra = {}) => ({
  fontFamily: FONT_UI, fontSize: size, fontWeight: '700', fill, stroke: { color: 0x000000, width: 5 }, ...extra,
});

const PHASE = { build: 'Стройка', wave: 'Волна идёт', gameover: 'Таверна пала' };

export class Hud {
  constructor() {
    this.world = new Container(); // tavern interior, lit by the lightmap
    this.ui = new Container(); // everything that must stay readable
    this.floaters = [];
    this.plates = new Map();

    this.tavern = new Graphics();
    this.world.addChild(this.tavern);
    this.drawTavern();
    this.walls = new Graphics();
    this.world.addChild(this.walls);

    const bar = new Graphics();
    bar.rect(0, 0, W, TOP).fill(0x15130f);
    bar.rect(0, TOP - 6, W, 6).fill(0x3a2f22);
    bar.rect(0, TOP - 2, W, 2).fill(0x000000);
    this.ui.addChild(bar);

    this.waveText = this.text('', ui(44), 32, 18);
    this.phaseText = this.text('', ui(28, 0xc9b98f), 34, 70);
    this.wallsBar = new Graphics();
    this.wallsBar.position.set(W / 2 - 360, 46);
    this.ui.addChild(this.wallsBar);
    this.wallsText = this.text('', ui(24), W / 2, 22, 0.5);
    this.biomeText = this.text('', ui(30), W - 32, 18, 1);
    this.infoText = this.text('', ui(22, 0xc9b98f), W - 32, 66, 1);

    this.bossBar = new Graphics();
    this.bossBar.position.set(LEFT + 368, TOP + 16);
    this.bossText = this.text('', ui(22, 0xffd6d6), LEFT + 768, TOP + 14, 0.5);
    this.ui.addChild(this.bossBar);

    this.platesLayer = new Container();
    this.ui.addChild(this.platesLayer);
    this.floatLayer = new Container();
    this.ui.addChild(this.floatLayer);
    this.banner = this.text('', ui(84, 0xf2e6c9, { stroke: { color: 0x000000, width: 10 } }), LEFT + 768, TOP + 380, 0.5);
    this.subBanner = this.text('', ui(36, 0xc9b98f), LEFT + 768, TOP + 470, 0.5);
    this.bannerTime = 0;
  }

  text(s, style, x, y, ax = 0) {
    const t = new Text({ text: s, style });
    t.anchor.set(ax, 0);
    t.position.set(x, y);
    this.ui.addChild(t);
    return t;
  }

  drawTavern() {
    const g = this.tavern;
    g.rect(0, TOP, LEFT, H - TOP).fill(0x3a2716);
    for (let y = TOP; y < H; y += 32) g.rect(0, y, LEFT, 2).fill(0x2a1b0f); // planks
    for (let x = 0; x < LEFT; x += 96) g.rect(x, TOP, 2, H - TOP).fill({ color: 0x2a1b0f, alpha: 0.6 });
    // Fireplace at the top, bar counter at the bottom: both stay clear of a
    // webcam placed in the middle of the left edge.
    g.rect(LEFT / 2 - 70, TOP + 8, 140, 70).fill(0x2b2b2b);
    g.rect(LEFT / 2 - 50, TOP + 28, 100, 50).fill(0x120c08);
    g.ellipse(LEFT / 2, TOP + 66, 34, 14).fill(0xff8a3a);
    g.ellipse(LEFT / 2, TOP + 62, 18, 10).fill(0xffd27a);
    g.rect(24, H - 110, LEFT - 72, 34).fill(0x5a3a1f);
    g.rect(24, H - 110, LEFT - 72, 6).fill(0x7a5230);
    for (let i = 0; i < 5; i++) g.rect(44 + i * 58, H - 70, 26, 34).fill(0x2a1b0f); // stools
  }

  setMap(map) {
    this.map = map;
    this.biomeText.text = (BIOMES[map.biome] ?? BIOMES.ruins).name;
    this.gateY = map.route[map.route.length - 1][1];
  }

  update(st) {
    this.waveText.text = st.wave ? `Волна ${st.wave}` : 'Подготовка';
    let phase = PHASE[st.phase] ?? st.phase;
    if (st.phase === 'build' || st.phase === 'gameover') phase += `  ${fmtTime(st.timer)}`;
    if (st.paused) phase = 'Пауза';
    this.phaseText.text = phase;
    this.infoText.text = `наёмников ${st.active}   seed ${this.map?.seed ?? ''}`;
    this.drawWalls(st.walls, st.wallsMax);
    this.drawBoss(st.enemies.find((e) => e.kind === 'boss'));
    this.updatePlates(st.chatters);
    for (const ev of st.events ?? []) this.onEvent(ev);
  }

  drawWalls(hp, maxHp) {
    const f = Math.max(0, hp / maxHp);
    const color = f > 0.5 ? 0x6fbf5a : f > 0.25 ? 0xe0b03a : 0xe04a3a;
    const b = this.wallsBar;
    b.clear();
    b.roundRect(0, 0, 720, 28, 6).fill(0x000000).stroke({ width: 2, color: 0x5a4a35 });
    b.roundRect(3, 3, 714 * f, 22, 4).fill(color);
    this.wallsText.text = `Стены таверны ${Math.ceil(hp)} / ${maxHp}`;
    this.wallsText.y = 78;

    // The wall itself, on the border between tavern and field, cracks as it drops.
    const w = this.walls;
    w.clear();
    w.rect(LEFT - 14, TOP, 14, H - TOP).fill(0x4a4038);
    for (let y = TOP; y < H; y += 24) w.rect(LEFT - 14, y, 14, 2).fill(0x2a2420);
    if (this.gateY !== undefined) {
      const gy = TOP + this.gateY * CELL;
      w.rect(LEFT - 18, gy - 40, 22, 80).fill(0x5a3a1f).stroke({ width: 3, color: 0x2a2420 });
    }
    const cracks = Math.floor((1 - f) * 8);
    for (let i = 0; i < cracks; i++) {
      const y = TOP + 60 + ((i * 397) % (H - TOP - 120));
      w.moveTo(LEFT - 14, y).lineTo(LEFT - 4, y + 14).lineTo(LEFT - 10, y + 28);
    }
    w.stroke({ width: 2, color: 0x000000 });
  }

  drawBoss(boss) {
    const b = this.bossBar;
    b.clear();
    if (!boss) {
      this.bossText.text = '';
      return;
    }
    const f = Math.max(0, boss.hp / boss.maxHp);
    b.roundRect(0, 18, 800, 22, 6).fill({ color: 0x000000, alpha: 0.75 }).stroke({ width: 2, color: 0x7a1d1d });
    b.roundRect(3, 21, 794 * f, 16, 4).fill(0xc23232);
    this.bossText.text = 'Босс';
  }

  // Nameplates for active chatters: name, then gold. Workers that walk to
  // build sites come with stage 2; for now each chatter sits at a table.
  updatePlates(chatters) {
    const active = chatters.filter((c) => c.active);
    const seen = new Set();
    active.forEach((c, i) => {
      seen.add(c.name);
      let p = this.plates.get(c.name);
      if (!p) {
        p = new Plate(c.name);
        this.plates.set(c.name, p);
        this.platesLayer.addChild(p.root);
      }
      const col = i % 2;
      const row = Math.floor(i / 2);
      p.root.position.set(20 + col * 176, TOP + 104 + row * 66);
      p.root.visible = row < 11;
      p.setGold(c.gold);
    });
    for (const [name, p] of this.plates) {
      if (!seen.has(name)) {
        p.root.destroy({ children: true });
        this.plates.delete(name);
      }
    }
  }

  plateOf(name) {
    return this.plates.get(name);
  }

  onEvent(ev) {
    switch (ev.kind) {
      case 'kill':
        if (ev.amount > 0) this.floatAt(ev.x, ev.y, `+${ev.amount}`, 0xffd24a);
        break;
      case 'build':
      case 'upgrade':
        this.floatAt(ev.x, ev.y - 0.4, `${ev.amount}`, 0xff7a6a);
        break;
      case 'sell':
        this.floatAt(ev.x, ev.y - 0.4, `+${ev.amount}`, 0xffd24a);
        break;
      case 'waveStart':
        this.showBanner(ev.text === 'boss' ? 'Босс приближается' : `Волна ${ev.amount}`, ev.text === 'boss' ? 'каждая пятая волна' : '');
        break;
      case 'waveCleared':
        this.showBanner('Волна отбита', 'наёмникам выплачена награда', 2.2);
        break;
      case 'gameOver':
        this.showBanner('Таверна пала', `продержались ${ev.amount} волн`, 10);
        break;
      case 'newGame':
        this.showBanner(BIOMES[ev.text]?.name ?? 'Новая карта', 'новая карта, стройте оборону', 4);
        break;
      case 'card': {
        const p = this.plateOf(ev.user);
        if (p) p.flash();
        break;
      }
    }
  }

  floatAt(x, y, s, color) {
    const px = toPx(x, y);
    const t = new Text({ text: s, style: { fontFamily: FONT_PIXEL, fontSize: 18, fill: color, stroke: { color: 0x000000, width: 5 } } });
    t.anchor.set(0.5);
    t.position.set(px.x, px.y - 20);
    this.floatLayer.addChild(t);
    this.floaters.push({ t, life: 1.3 });
  }

  showBanner(main, sub = '', secs = 3) {
    this.banner.text = main;
    this.subBanner.text = sub;
    this.bannerTime = secs;
  }

  frame(dt) {
    this.floaters = this.floaters.filter((f) => {
      f.life -= dt;
      f.t.y -= 40 * dt;
      f.t.alpha = Math.min(1, f.life * 2);
      if (f.life <= 0) f.t.destroy();
      return f.life > 0;
    });
    this.bannerTime -= dt;
    const a = Math.max(0, Math.min(1, this.bannerTime * 2));
    this.banner.alpha = this.subBanner.alpha = a;
    for (const p of this.plates.values()) p.frame(dt);
  }
}

class Plate {
  constructor(name) {
    this.root = new Container();
    const color = nickColor(name);
    const body = new Graphics();
    body.circle(14, 26, 12).fill(color).stroke({ width: 2, color: 0x000000 });
    body.rect(6, 36, 16, 14).fill(color).stroke({ width: 2, color: 0x000000 });
    this.root.addChild(body);
    this.glow = new Graphics();
    this.glow.roundRect(-4, 4, 172, 56, 8).stroke({ width: 3, color: 0xffd24a });
    this.glow.alpha = 0;
    this.root.addChild(this.glow);
    const short = name.length > 14 ? name.slice(0, 13) + '…' : name;
    const n = new Text({ text: short, style: { fontFamily: FONT_UI, fontSize: 18, fontWeight: '700', fill: color, stroke: { color: 0x000000, width: 4 } } });
    n.position.set(32, 8);
    this.root.addChild(n);
    this.gold = new Text({ text: '', style: { fontFamily: FONT_PIXEL, fontSize: 13, fill: 0xffd24a, stroke: { color: 0x000000, width: 4 } } });
    this.gold.position.set(32, 34);
    this.root.addChild(this.gold);
    this.value = -1;
    this.flashTime = 0;
  }

  setGold(v) {
    if (v !== this.value) {
      this.value = v;
      this.gold.text = `${v} з`;
    }
  }

  flash() {
    this.flashTime = 5;
  }

  frame(dt) {
    this.flashTime = Math.max(0, this.flashTime - dt);
    this.glow.alpha = Math.min(1, this.flashTime);
  }
}

function fmtTime(s) {
  s = Math.max(0, Math.ceil(s));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}
