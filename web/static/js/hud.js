// Top bar, the tavern wall, floating numbers, banners and the game over screen.
import { W, H, TOP, LEFT, CELL, COLS, ROWS, toPx, BIOMES, FONT_UI, FONT_PIXEL, nickColor, RANKS } from './layout.js';
import { ENEMIES } from './sprites.js';

const { Container, Graphics, Text } = PIXI;

const ui = (size, fill = 0xf2e6c9, extra = {}) => ({
  fontFamily: FONT_UI, fontSize: size, fontWeight: '700', fill, stroke: { color: 0x000000, width: 5 }, ...extra,
});

const PHASE = { build: 'Стройка', wave: 'Волна идёт', gameover: 'Таверна пала' };

const ABILITY = {
  bone_knight: 'щит поднят!',
  cult_leader: 'ритуал!',
  spider_queen: 'паутина!',
  cave_troll: 'камнепад!',
  lich: 'восстаньте!',
  ghost_king: 'сквозь завесу!',
  ice_golem: 'вечный холод!',
  frost_demon: 'ледяная тропа!',
};

const DISABLED = { spider_queen: 'опутана', cave_troll: 'оглушена', ice_golem: 'заморожена', sdf: 'в шоке' };

// What ЭссДиЭф does with each burp or fart.
const SDF_TRICK = {
  disable: 'башни в шоке',
  destroy: 'постройки снесены',
  offroad: 'сошёл с тропы',
  tavern: 'грабит таверну',
};
const FIELD_CX = LEFT + (COLS * CELL) / 2;
const FIELD_CY = TOP + (ROWS * CELL) / 2;

export class Hud {
  constructor() {
    this.world = new Container(); // the wall, lit by the lightmap
    this.ui = new Container(); // everything that must stay readable
    this.floaters = [];

    this.walls = new Graphics();
    this.world.addChild(this.walls);
    this.fire = new Graphics();
    this.world.addChild(this.fire);
    this.wallRatio = 1;
    this.hitFlash = 0;
    this.lastWalls = null;
    this.t = 0;

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
    this.wallsText = this.text('', ui(24), W / 2, 78, 0.5);
    this.biomeText = this.text('', ui(30), W - 32, 18, 1);
    this.infoText = this.text('', ui(22, 0xc9b98f), W - 32, 66, 1);

    // While a boss is on the field the top bar holds two bars: the walls
    // move up and the boss's health goes under them, clear of the field.
    this.bossBar = new Graphics();
    this.bossBar.position.set(W / 2 - 360, 66);
    this.ui.addChild(this.bossBar);
    this.bossText = this.text('', ui(20, 0xffd6d6), W / 2, 67, 0.5);

    this.floatLayer = new Container();
    this.blasts = new Graphics();
    this.rings = [];
    this.ui.addChild(this.blasts, this.floatLayer);
    this.banner = this.text('', ui(84, 0xf2e6c9, { stroke: { color: 0x000000, width: 10 } }), FIELD_CX, TOP + 380, 0.5);
    this.subBanner = this.text('', ui(36, 0xc9b98f), FIELD_CX, TOP + 470, 0.5);
    this.bannerTime = 0;
    // Achievements: a line under the column letters, one after another.
    this.toast = this.text('', ui(28, 0xffd24a), FIELD_CX, TOP + 40, 0.5);
    this.toasts = [];
    this.toastTime = 0;

    this.summary = new SummaryPanel();
    // A white flash over the field when a reward rains arrows from the sky.
    this.flash = new Graphics();
    this.flash.rect(LEFT, TOP, COLS * CELL, ROWS * CELL).fill(0xfff4d6);
    this.flash.alpha = 0;
    this.ui.addChild(this.flash);
    this.treasury = 0;
    this.ui.addChild(this.summary.root);
  }

  text(s, style, x, y, ax = 0) {
    const t = new Text({ text: s, style });
    t.anchor.set(ax, 0);
    t.position.set(x, y);
    this.ui.addChild(t);
    return t;
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
    if (this.lastWalls !== null && st.walls < this.lastWalls) this.hitFlash = 0.35;
    this.lastWalls = st.walls;
    this.treasury = st.treasury ?? 0;
    const boss = st.enemies.find((e) => e.boss);
    this.withBoss = !!boss;
    this.drawWalls(st.walls, st.wallsMax);
    this.drawBoss(boss);
    if (st.phase === 'gameover' && st.summary) this.summary.show(st.summary, st.timer);
    else this.summary.hide();
    for (const ev of st.events ?? []) this.onEvent(ev);
  }

  drawWalls(hp, maxHp) {
    const f = Math.max(0, hp / maxHp);
    this.wallRatio = f;
    const color = f > 0.5 ? 0x6fbf5a : f > 0.25 ? 0xe0b03a : 0xe04a3a;
    const b = this.wallsBar;
    b.clear();
    b.roundRect(0, 0, 720, 28, 6).fill(0x000000).stroke({ width: 2, color: 0x5a4a35 });
    b.roundRect(3, 3, 714 * f, 22, 4).fill(color);
    const pot = this.treasury > 0 ? `  ·  казна ${this.treasury} з` : '';
    this.wallsText.text = `Стены таверны ${Math.ceil(hp)} / ${maxHp}${pot}`;
    // With a boss the walls' caption goes inside their bar to make room.
    b.y = this.withBoss ? 22 : 46;
    this.wallsText.y = this.withBoss ? 23 : 78;
    this.wallsText.style.fontSize = this.withBoss ? 20 : 24;

    // The wall itself, on the border between tavern and field, cracks as it drops.
    const w = this.walls;
    w.clear();
    w.rect(LEFT - 14, TOP, 14, H - TOP).fill(0x4a4038);
    for (let y = TOP; y < H; y += 24) w.rect(LEFT - 14, y, 14, 2).fill(0x2a2420);
    if (this.gateY !== undefined) {
      const gy = TOP + this.gateY * CELL;
      w.rect(LEFT - 18, gy - 40, 22, 80).fill(0x5a3a1f).stroke({ width: 3, color: 0x2a2420 });
    }
    const cracks = Math.floor((1 - f) * 10);
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
    b.roundRect(0, 0, 720, 28, 6).fill(0x000000).stroke({ width: 2, color: 0x7a1d1d });
    b.roundRect(3, 3, 714 * f, 22, 4).fill(0xc23232);
    this.bossText.text = `${boss.title || 'Босс'}  ${Math.ceil(boss.hp)} / ${Math.ceil(boss.maxHp)}`;
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
      case 'built':
      case 'upgraded':
        this.floatAt(ev.x, ev.y - 0.5, ev.kind === 'built' ? 'готово' : `ур. ${ev.amount}`, 0xfff1c2, FONT_UI, 22);
        break;
      case 'stun':
        this.floatAt(ev.x, ev.y - 0.6, 'оглушён!', 0xff9a6a, FONT_UI, 22);
        break;
      case 'waveStart':
        if (ev.text === 'sdf') this.showBanner('ЭссДиЭф', 'стройте клоунов: !b C4 clown', 6);
        else if (ev.text) this.showBanner(ENEMIES[ev.text]?.name ?? 'Босс', `волна ${ev.amount} · босс приближается`, 4);
        else this.showBanner(`Волна ${ev.amount}`);
        break;
      case 'ability':
        if (ev.text === 'sdf') {
          // Every trick comes with the catchphrase and what he just did.
          this.floatAt(ev.x, ev.y - 1.4, 'УЗРИТЕ ВИЖН!', 0x9dff3a, FONT_UI, 40, 2.4);
          this.floatAt(ev.x, ev.y - 0.9, SDF_TRICK[ev.user] ?? '', 0xffd6a0, FONT_UI, 24, 2.4);
        } else {
          this.floatAt(ev.x, ev.y - 0.8, ABILITY[ev.text] ?? '', 0xffd6a0, FONT_UI, 28);
        }
        break;
      case 'achievement':
        this.toasts.push({ text: `🏆 ${ev.user}: «${ev.text}»  +${ev.amount} душ`, color: 0xffd24a });
        break;
      case 'rank':
        this.toasts.push({ text: `⬆ ${ev.user} теперь ${ev.text}`, color: RANKS[ev.amount]?.color ?? 0xffd24a });
        break;
      case 'destroyed':
        this.floatAt(ev.x, ev.y - 0.4, `${ev.text}: снесено`, 0xff6a5a, FONT_UI, 20);
        this.rings.push({ ...toPx(ev.x, ev.y), life: 0.35 });
        break;
      case 'disabled':
        this.floatAt(ev.x, ev.y - 0.5, DISABLED[ev.text] ?? 'выведена', 0xbfe8ff, FONT_UI, 20);
        break;
      case 'trap':
        this.floatAt(ev.x, ev.y - 0.3, 'щёлк', 0xe8e8e8, FONT_UI, 18);
        break;
      case 'wallBroken':
        this.floatAt(ev.x, ev.y - 0.4, 'баррикада пала', 0xff9a6a, FONT_UI, 22);
        break;
      case 'donation':
        this.showBanner(`${ev.user}: ${ev.text}`, `+${ev.amount ?? 0} золота`, 5);
        break;
      case 'support':
        this.showBanner(`${ev.user}: ${ev.text}`, `+${ev.amount ?? 0} золота`, 4);
        break;
      case 'reward':
        this.showBanner(ev.text, `награду взял ${ev.user}`, 3);
        break;
      case 'strike':
        this.flash.alpha = 0.55;
        break;
      case 'blast':
        this.rings.push({ ...toPx(ev.x, ev.y), life: 0.35 });
        break;
      case 'waveCleared':
        this.showBanner('Волна отбита', 'наёмникам выплачена награда', 2.2);
        break;
      case 'newGame':
        this.showBanner(BIOMES[ev.text]?.name ?? 'Новая карта', 'новая карта, стройте оборону', 4);
        break;
      case 'resumed':
        this.showBanner(`Продолжаем: волна ${ev.amount}`, BIOMES[ev.text]?.name ?? '', 4);
        break;
    }
  }

  floatAt(x, y, s, color, font = FONT_PIXEL, size = 18, life = 1.3) {
    const px = toPx(x, y);
    const t = new Text({ text: s, style: { fontFamily: font, fontSize: size, fontWeight: '700', fill: color, stroke: { color: 0x000000, width: 5 } } });
    t.anchor.set(0.5);
    t.position.set(Math.max(px.x, t.width / 2 + 8), px.y - 20); // keep it on screen at the left edge
    this.floatLayer.addChild(t);
    this.floaters.push({ t, life });
  }

  showBanner(main, sub = '', secs = 3) {
    this.banner.text = main;
    this.subBanner.text = sub;
    this.bannerTime = secs;
  }

  frame(dt) {
    this.t += dt;
    this.floaters = this.floaters.filter((f) => {
      f.life -= dt;
      f.t.y -= 40 * dt;
      f.t.alpha = Math.min(1, f.life * 2);
      if (f.life <= 0) f.t.destroy();
      return f.life > 0;
    });
    this.toastTime -= dt;
    if (this.toastTime <= 0 && this.toasts.length) {
      const next = this.toasts.shift();
      this.toast.text = next.text;
      this.toast.style.fill = next.color;
      this.toastTime = 4;
    }
    this.toast.alpha = Math.max(0, Math.min(1, this.toastTime * 2));
    this.bannerTime -= dt;
    const a = Math.max(0, Math.min(1, this.bannerTime * 2));
    this.banner.alpha = this.subBanner.alpha = a;
    this.hitFlash = Math.max(0, this.hitFlash - dt);
    this.flash.alpha = Math.max(0, this.flash.alpha - dt * 1.5);
    this.walls.tint = this.hitFlash > 0 ? 0xff8080 : 0xffffff;
    this.drawFire();
    // Fire blasts: a ring that expands and fades where a fireball landed.
    this.blasts.clear();
    this.rings = this.rings.filter((r) => {
      r.life -= dt;
      if (r.life <= 0) return false;
      const k = 1 - r.life / 0.35;
      this.blasts.circle(r.x, r.y, 20 + 70 * k).stroke({ width: 1 + 6 * (1 - k), color: 0xff8a3a, alpha: 1 - k });
      return true;
    });
  }

  // Below a quarter of its hit points the wall burns: flames grow as it drops.
  drawFire() {
    const g = this.fire;
    g.clear();
    if (this.wallRatio >= 0.25) return;
    const strength = 1 - this.wallRatio / 0.25;
    for (let i = 0; i < 9; i++) {
      const y = TOP + 70 + i * 104;
      const h = (18 + 22 * strength) * (0.75 + 0.25 * Math.sin(this.t * 9 + i * 1.7));
      g.poly([LEFT - 20, y, LEFT - 7, y - h, LEFT + 6, y]).fill({ color: 0xff6a1a, alpha: 0.9 });
      g.poly([LEFT - 14, y, LEFT - 7, y - h * 0.6, LEFT, y]).fill({ color: 0xffd36a, alpha: 0.95 });
    }
  }

  // Light sources for the lighting layer: the burning wall glows.
  lights() {
    if (this.wallRatio >= 0.25) return [];
    const out = [];
    for (let i = 0; i < 9; i += 2) out.push({ x: LEFT - 7, y: TOP + 60 + i * 104, r: 150, color: 0xff7a2a, a: 0.8 });
    return out;
  }
}

// SummaryPanel is the game over screen: how far the tavern held and the
// three best by damage, kills and mugs drunk.
class SummaryPanel {
  constructor() {
    this.root = new Container();
    this.root.visible = false;
    const pw = 1000;
    const ph = 520;
    this.root.position.set(FIELD_CX - pw / 2, FIELD_CY - ph / 2);
    const bg = new Graphics();
    bg.roundRect(0, 0, pw, ph, 14).fill({ color: 0x0d0b09, alpha: 0.92 }).stroke({ width: 4, color: 0x7a1d1d });
    this.root.addChild(bg);
    const add = (style, x, y, ax = 0.5) => {
      const t = new Text({ text: '', style });
      t.anchor.set(ax, 0);
      t.position.set(x, y);
      this.root.addChild(t);
      return t;
    };
    this.title = add(ui(64, 0xe8c9a0), pw / 2, 26);
    this.sub = add(ui(28, 0xc9b98f), pw / 2, 110);
    this.cols = ['Урон', 'Убийства', 'Выпито'].map((name, i) => {
      const x = 180 + i * 320;
      add(ui(30, 0xf2e6c9), x, 180).text = name;
      return [0, 1, 2].map((r) => ({
        n: add(ui(24, 0xffffff), x, 236 + r * 70),
        v: add({ fontFamily: FONT_PIXEL, fontSize: 14, fill: 0xffd24a, stroke: { color: 0x000000, width: 4 } }, x, 268 + r * 70),
      }));
    });
    this.next = add(ui(24, 0xc9b98f), pw / 2, ph - 54);
  }

  show(s, timer) {
    this.root.visible = true;
    this.title.text = 'Таверна пала';
    this.sub.text = `продержались ${s.wave} ${plural(s.wave, 'волну', 'волны', 'волн')} · ${BIOMES[s.biome]?.name ?? ''}`;
    [s.damage, s.kills, s.drinks].forEach((list, i) => {
      this.cols[i].forEach((cell, r) => {
        const e = list?.[r];
        cell.n.text = e ? `${['I', 'II', 'III'][r]}. ${e.name}` : r === 0 ? '—' : '';
        cell.n.style.fill = e ? nickColor(e.name) : 0x777777;
        cell.v.text = e ? String(e.value) : '';
      });
    });
    this.next.text = `новая карта через ${Math.ceil(Math.max(0, timer))} с`;
  }

  hide() {
    this.root.visible = false;
  }
}

function plural(n, one, few, many) {
  const m10 = n % 10;
  const m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return one;
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return few;
  return many;
}

function fmtTime(s) {
  s = Math.max(0, Math.ceil(s));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}
