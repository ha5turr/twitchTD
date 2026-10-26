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
// Soul links: how long one shows after an enemy crosses it, the last part
// of which it fades out, and how long !me shows all of a warlock's.
const LINK_FLASH = 1.2;
const LINK_FADE = 0.6;
const LINK_ME_SEC = 6;
const FIELD_CX = LEFT + (COLS * CELL) / 2;
const FIELD_CY = TOP + (ROWS * CELL) / 2;

export class Hud {
  constructor() {
    this.world = new Container(); // the wall, lit by the lightmap
    // Lightning charges on the ground: over the map, under the enemies, and
    // lit like the world, so a field of them never glares.
    this.ground = new Graphics();
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
    this.linkShow = new Map(); // soul link id -> seconds it still shows
    this.linkOwners = new Map(); // warlock's name -> seconds his links show after !me
    this.rings = [];
    this.bolts = [];
    this.quakes = []; // shamans' earthquakes
    this.coins = []; // a golden golem's gold rain
    this.fans = [];
    this.pillars = []; // clerics' pillars of light
    this.pathFires = []; // pyromancers setting the path alight
    this.volleys = []; // rangers' rain of arrows
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
    this.infoText.text = `наёмников ${st.active}   seed ${this.map?.seed ?? ''}   v${this.map?.version ?? ''}`;
    this.pact = st.pact ?? 0; // a warlock's blood pact: the field glows red while it lasts
    if (this.lastWalls !== null && st.walls < this.lastWalls) this.hitFlash = 0.35;
    this.lastWalls = st.walls;
    this.treasury = st.treasury ?? 0;
    const boss = st.enemies.find((e) => e.boss);
    this.withBoss = !!boss;
    this.shield = st.shield ?? 0;
    this.shieldMax = st.shieldMax ?? 0;
    this.storms = st.storms ?? [];
    this.links = st.links ?? [];
    for (const ev of st.events ?? []) {
      if (ev.kind === 'soulLink') this.linkShow.set(ev.amount, LINK_FLASH);
      if (ev.kind === 'card') this.linkOwners.set(ev.user, LINK_ME_SEC);
    }
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
    // An artifact shrine's shield: a pale blue band over the walls' bar.
    if (this.shield > 0) b.roundRect(3, 3, 714 * Math.min(1, this.shield / maxHp), 8, 3).fill({ color: 0x9ad8ff, alpha: 0.9 });
    const pot = this.treasury > 0 ? `  ·  казна ${this.treasury} з` : '';
    const shield = this.shieldMax > 0 ? `  ·  щит ${Math.ceil(this.shield)}` : '';
    this.wallsText.text = `Стены таверны ${Math.ceil(hp)} / ${maxHp}${shield}${pot}`;
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
        this.floatAt(ev.x, ev.y - 0.4, ev.amount ? `баррикада пала, +${ev.amount} назад` : 'баррикада пала', 0xff9a6a, FONT_UI, 22);
        break;
      case 'assist':
        if (ev.amount) this.floatAt(ev.x, ev.y + 0.3, `+${ev.amount} за помощь`, 0xffd24a, FONT_UI, 16);
        break;
      case 'lightning': {
        const p = toPx(ev.x, ev.y);
        const pts = [[p.x + 20, p.y - 260]];
        for (let i = 1; i < 6; i++) pts.push([p.x + (Math.random() - 0.5) * 40, p.y - 260 + (260 * i) / 6]);
        pts.push([p.x, p.y]);
        this.bolts.push({ x: p.x, y: p.y, pts, life: 0.3 });
        break;
      }
      case 'charge':
        this.toasts.push({ text: `⚔ ${ev.user} несётся по тропе`, color: 0xd8dce8 });
        break;
      case 'shove':
        this.floatAt(ev.x, ev.y - 0.5, 'отброшен!', 0xd8dce8, FONT_UI, 18);
        break;
      case 'meteorShower':
        this.toasts.push({ text: `☄ ${ev.user} вызывает метеоритный дождь`, color: 0xff9a4a });
        break;
      case 'meteor': {
        // A meteor streaks down at a slant and bursts.
        const p = toPx(ev.x, ev.y);
        this.bolts.push({ x: p.x, y: p.y, pts: [[p.x + 160, p.y - 240], [p.x, p.y]], life: 0.25, color: 0xffb36b, ring: 0xff6a1a });
        this.rings.push({ ...p, life: 0.35 });
        break;
      }
      case 'music':
        this.toasts.push({ text: `♪ ${ev.user} играет: барабаны звучат на всю карту`, color: 0xffd24a });
        break;
      case 'shadowstep':
        this.floatAt(ev.x, ev.y - 0.6, 'теневой прыжок', 0xb08aff, FONT_UI, 18);
        break;
      case 'bladefan':
        this.floatAt(ev.x, ev.y - 0.9, 'веер клинков!', 0xb08aff, FONT_UI, 22);
        this.fans.push({ ...toPx(ev.x, ev.y), life: 0.4 });
        break;
      case 'pillar':
        // A soft column of light: it swells and fades, no flash.
        this.pillars.push({ ...toPx(ev.x, ev.y), life: 1.8 });
        this.toasts.push({ text: `✨ ${ev.user} призывает столп света`, color: 0xfff1c2 });
        break;
      case 'pathFire':
        this.pathFires.push({ life: 1.8 });
        this.toasts.push({ text: `🔥 ${ev.user} поджигает тропу`, color: 0xff9a4a });
        break;
      case 'arrowRain': {
        const p = toPx(ev.x, ev.y);
        const arrows = Array.from({ length: 16 }, () => {
          const a = Math.random() * Math.PI * 2;
          const r = Math.sqrt(Math.random()) * 2 * CELL;
          return [Math.cos(a) * r, Math.sin(a) * r, Math.random() * 0.25];
        });
        this.volleys.push({ ...p, arrows, life: 0.7 });
        break;
      }
      case 'leave':
        this.toasts.push({ text: `🚪 ${ev.user} покинул таверну: ${ev.amount} з в казну`, color: 0xc9b98f });
        break;
      case 'afk':
        this.toasts.push({ text: `💤 ${ev.user} ушёл: ${ev.amount} з в казну`, color: 0xc9b98f });
        break;
      case 'net':
        this.floatAt(ev.x, ev.y - 0.6, 'сеть!', 0xd8c08a, FONT_UI, 20);
        break;
      case 'maul':
        this.floatAt(ev.x, ev.y - 0.4, 'кусь!', 0xb0ff8a, FONT_UI, 20);
        break;
      case 'panther':
        this.toasts.push({ text: `🐆 ${ev.user} обращается в пантеру`, color: 0xb0ff8a });
        break;
      case 'shrineGold':
        this.toasts.push({ text: `⛪ святилища ${ev.user} собрали ${ev.amount} з подношений`, color: 0xffe08a });
        break;
      case 'zap': {
        // Chain lightning: a short jagged cyan arc from link to link.
        const a = toPx(ev.x, ev.y);
        const b = toPx(ev.x2 ?? ev.x, ev.y2 ?? ev.y);
        this.bolts.push({ x: b.x, y: b.y, pts: jagged(a, b, 5, 14), life: 0.22, color: 0x9ef0ff, width: 3, noRing: true });
        break;
      }
      case 'curse': {
        const a = toPx(ev.x, ev.y);
        const b = toPx(ev.x2 ?? ev.x, ev.y2 ?? ev.y);
        this.bolts.push({ x: b.x, y: b.y, pts: jagged(a, b, 4, 6), life: 0.4, color: 0xb05aff, width: 2, ring: 0x8a3aff });
        break;
      }
      case 'veil':
        this.floatAt(ev.x, ev.y - 0.6, 'в тени', 0xb9a6ff, FONT_UI, 16);
        break;
      case 'mapVote':
        this.toasts.push({ text: `🗺 ${ev.user}: новая карта? Против — !против`, color: 0xd9c49a });
        break;
      case 'jailVote':
        this.toasts.push({ text: `⚖ ${ev.text}: ${ev.user} — в тюрьму? !за / !против`, color: 0xd9c49a });
        break;
      case 'jailed':
        this.toasts.push({ text: `⛓ ${ev.user} за решёткой, выкуп ${ev.amount} з`, color: 0xb0b8c4 });
        break;
      case 'artifact':
        this.toasts.push({ text: `🔴 ${ev.user}: артефактная постройка!`, color: 0xff6a5a });
        this.rings.push({ ...toPx(ev.x, ev.y), life: 0.35 });
        break;
      case 'ammo':
        this.floatAt(ev.x, ev.y - 0.6, { poison: 'яд ☠️', fire: 'огонь 🔥', blast: 'взрыв 💥' }[ev.text] ?? 'обычные', 0xffe0a0, FONT_UI, 18);
        break;
      case 'arcane':
        this.floatAt(ev.x, ev.y - 0.6, '×1,5', 0xc9a6ff, FONT_UI, 18);
        break;
      case 'warDrum':
        this.rings.push({ ...toPx(ev.x, ev.y), life: 0.35 });
        this.floatAt(ev.x, ev.y - 0.7, 'боевой ритм!', 0xffb347, FONT_UI, 20);
        break;
      case 'storm':
        this.floatAt(ev.x, ev.y - 0.5, 'заряд!', 0x9ef0ff, FONT_UI, 18);
        break;
      case 'crit':
        this.floatAt(ev.x, ev.y - 0.7, `КРИТ! ${ev.amount ?? ''}`, 0xffd24a, FONT_UI, 22);
        break;
      case 'gateFire':
        this.toasts.push({ text: `🔥 ${ev.user} встаёт к жаровне на стене таверны (ур. ${ev.amount ?? 1})`, color: 0xffa36a });
        break;
      case 'feast':
        this.toasts.push({ text: `🍗 ${ev.user} накрывает столы: все постройки +50% на ${ev.amount ?? 20} с`, color: 0xffd8a0 });
        break;
      case 'wolves':
        this.toasts.push({ text: `🐺 ${ev.user} призывает духов-волков`, color: 0x9ef0ff });
        break;
      case 'bear':
        this.toasts.push({ text: ev.amount ? `🐻 ${ev.user} в облике медведя встаёт у двери таверны!` : `🐻 ${ev.user} в облике медведя бросается в бой`, color: 0xc89a6a });
        break;
      case 'massCurse':
        this.toasts.push({ text: `🔮 ${ev.user} насылает порчу: проклято врагов — ${ev.amount ?? 0}`, color: 0xc07aff });
        this.curseFlash = 0.6;
        break;
      case 'bloodPact':
        this.toasts.push({ text: `🩸 ${ev.user} заключает кровавый пакт: стены −${ev.amount ?? 0}, постройки бьют сильнее`, color: 0xff6a6a });
        break;
      case 'quake':
        this.quakes.push({ life: 1 });
        this.toasts.push({ text: `🌋 ${ev.user} сотрясает землю`, color: 0xc8a070 });
        break;
      case 'boom':
        this.rings.push({ ...toPx(ev.x, ev.y), life: 0.35 });
        this.floatAt(ev.x, ev.y - 0.6, 'БУМ!', 0xff8a3a, FONT_UI, 24);
        break;
      case 'shard':
        if (ev.amount) this.floatAt(ev.x, ev.y - 0.5, `+${ev.amount} осколок`, 0xc07aff, FONT_UI, 16);
        break;
      case 'split':
        this.floatAt(ev.x, ev.y - 0.4, 'плюх!', 0x8aff9a, FONT_UI, 18);
        break;
      case 'dispel':
        this.rings.push({ ...toPx(ev.x, ev.y), life: 0.35 });
        this.floatAt(ev.x, ev.y - 0.6, 'очищено!', 0xb0ff8a, FONT_UI, 18);
        break;
      case 'hex':
        this.floatAt(ev.x, ev.y - 0.6, 'ква!', 0x8aff6a, FONT_UI, 22);
        break;
      case 'headGrow':
        this.floatAt(ev.x, ev.y - 0.8, `+голова! (${ev.amount})`, 0xff6a5a, FONT_UI, 22);
        this.toasts.push({ text: `🐍 У гидры отросла голова: их уже ${ev.amount}! Поджигайте раны`, color: 0xff8a7a });
        break;
      case 'cauterize':
        this.floatAt(ev.x, ev.y - 0.8, 'прижжено!', 0xffb36b, FONT_UI, 22);
        break;
      case 'guest':
        if (ev.text === 'tax_collector') this.showBanner('Сборщик податей', 'идёт за казной: не дайте ему дойти до таверны!', 5);
        else this.showBanner('Золотой голем', 'бейте все: золото достанется каждому, кто попал', 5);
        break;
      case 'taxed':
        this.toasts.push({ text: `💸 Сборщик податей унёс из казны ${ev.amount ?? 0} з`, color: 0xff8a7a });
        break;
      case 'taxKilled':
        this.floatAt(ev.x, ev.y - 0.8, `+${ev.amount} в казну`, 0xffd24a, FONT_UI, 24);
        this.toasts.push({ text: `🏦 Сборщик податей повержен: казна +${ev.amount}`, color: 0xffd24a });
        break;
      case 'goldRain': {
        const p = toPx(ev.x, ev.y);
        for (let i = 0; i < 40; i++) {
          const a = Math.random() * Math.PI * 2;
          const v = 120 + Math.random() * 260;
          this.coins.push({ x: p.x, y: p.y, vx: Math.cos(a) * v, vy: Math.sin(a) * v - 260, life: 1.2 + Math.random() * 0.5 });
        }
        this.toasts.push({ text: `💰 Золотой голем рассыпался: ${ev.text ?? 0} игроков поделили ${ev.amount ?? 0} з`, color: 0xffd24a });
        break;
      }
      case 'golemGone':
        this.toasts.push({ text: '🏃 Золотой голем убежал вместе со своим золотом', color: 0xc9b98f });
        break;
      case 'passOut':
        this.toasts.push({ text: `🍺💤 ${ev.user} перебрал и храпит под столом`, color: 0xe0b03a });
        break;
      case 'grant':
        this.toasts.push({ text: `🎒 ${ev.user} получает ${ev.amount} з подъёмных`, color: 0xb0ff8a });
        break;
      case 'rage':
        this.floatAt(ev.x, ev.y - 0.6, 'ярость!', 0xff5a4a, FONT_UI, 20);
        break;
      case 'limitUp':
        this.toasts.push({ text: `🏗️ Лимит построек вырос до ${ev.amount}`, color: 0xffd24a });
        break;
      case 'rewardUp':
        this.showBanner('Награды растут', `убийства, ассисты и баррикады платят +${ev.amount}%`, 4);
        break;
      case 'drinkTower':
        this.floatAt(ev.x, ev.y - 0.5, ev.amount > 0 ? 'медовуха ×2' : 'медовуха ×0.5', ev.amount > 0 ? 0xffd24a : 0x9a9a9a, FONT_UI, 18);
        break;
      case 'mended':
        this.floatAt(ev.x, ev.y - 0.4, 'починено', 0x9fe8a0, FONT_UI, 18);
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
        this.showBanner('Волна отбита', ev.text ? `малой команде награда ×${ev.text}` : 'наёмникам выплачена награда', 2.2);
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
    // Lightning from the obelisks: a jagged bolt from above and a blue ring.
    this.bolts = this.bolts.filter((b) => {
      b.life -= dt;
      if (b.life <= 0) return false;
      const a = Math.min(1, b.life / 0.15);
      this.blasts.moveTo(b.pts[0][0], b.pts[0][1]);
      for (const [x, y] of b.pts.slice(1)) this.blasts.lineTo(x, y);
      this.blasts.stroke({ width: b.width ?? 4, color: b.color ?? 0xd8e8ff, alpha: a });
      if (!b.noRing) this.blasts.circle(b.x, b.y, 60 * (1.3 - a * 0.3)).stroke({ width: 3, color: b.ring ?? 0x7ab8ff, alpha: a });
      return true;
    });
    // Blood pact: a red throb round the field while every building hits
    // harder; a mass curse: one violet flash.
    if (this.pact > 0) {
      const a = 0.25 + 0.15 * Math.sin(this.t * 5);
      this.blasts.rect(LEFT + 4, TOP + 4, COLS * CELL - 8, ROWS * CELL - 8).stroke({ width: 8, color: 0xd02a2a, alpha: a });
    }
    if (this.curseFlash > 0) {
      this.curseFlash -= dt;
      this.blasts.rect(LEFT, TOP, COLS * CELL, ROWS * CELL).fill({ color: 0x8a3aff, alpha: 0.18 * Math.max(0, this.curseFlash / 0.6) });
    }
    this.drawStorms();
    // Warlocks' soul links stay hidden: a faint violet thread shows for a
    // moment when an enemy crosses one, and all of a warlock's while his !me
    // card is up. No bright core, so a web of them never glares.
    for (const [id, left] of this.linkShow) {
      if (left - dt <= 0) this.linkShow.delete(id);
      else this.linkShow.set(id, left - dt);
    }
    for (const [name, left] of this.linkOwners) {
      if (left - dt <= 0) this.linkOwners.delete(name);
      else this.linkOwners.set(name, left - dt);
    }
    for (const l of this.links ?? []) {
      const flash = Math.min(1, (this.linkShow.get(l.id) ?? 0) / LINK_FADE);
      const shown = Math.min(1, this.linkOwners.get(l.owner) ?? 0);
      const k = Math.max(flash, shown);
      if (k <= 0) continue;
      const [x1, y1, x2, y2] = l.ends;
      const a = toPx(x1, y1);
      const b = toPx(x2, y2);
      this.blasts.moveTo(a.x, a.y).lineTo(b.x, b.y).stroke({ width: 2, color: 0x9a6ad0, alpha: 0.35 * k });
    }
    // Gold rain: coins burst out of a fallen golden golem and fall.
    this.coins = this.coins.filter((c) => {
      c.life -= dt;
      if (c.life <= 0) return false;
      c.vy += 700 * dt;
      c.x += c.vx * dt;
      c.y += c.vy * dt;
      const w = 7 * Math.abs(Math.cos(this.t * 12 + c.x)); // spinning
      this.blasts.ellipse(c.x, c.y, Math.max(1.5, w), 7).fill({ color: 0xffd24a, alpha: Math.min(1, c.life * 2) }).stroke({ width: 1.5, color: 0x8a5a10, alpha: Math.min(1, c.life * 2) });
      return true;
    });
    // Earthquake: dust rings shiver all along the path.
    this.quakes = this.quakes.filter((q) => {
      q.life -= dt;
      if (q.life <= 0 || !this.map) return false;
      const a = q.life;
      const route = this.map.route;
      for (let i = 1; i < route.length; i += 2) {
        const p = toPx(route[i][0], route[i][1]);
        const dx = Math.sin(this.t * 60 + i) * 6 * a;
        this.blasts.ellipse(p.x + dx, p.y + 18, 40 * (1.4 - a * 0.4), 12).stroke({ width: 4, color: 0x8a6a3a, alpha: 0.6 * a });
      }
      return true;
    });
    // Pillar of light: nested soft columns and a pool of light at the foot,
    // swelling in and out over the life so it never flashes.
    this.pillars = this.pillars.filter((p) => {
      p.life -= dt;
      if (p.life <= 0) return false;
      const a = Math.sin(Math.PI * (1 - p.life / 1.8)); // 0 → 1 → 0
      for (const [w, alpha] of [[70, 0.1], [46, 0.14], [24, 0.2]]) {
        this.blasts.rect(p.x - w / 2, p.y - 320, w, 320).fill({ color: 0xfff1c2, alpha: alpha * a });
      }
      this.blasts.ellipse(p.x, p.y, 56, 20).fill({ color: 0xffe08a, alpha: 0.25 * a });
      return true;
    });
    // The burning path: soft flames all along the route, flickering out.
    this.pathFires = this.pathFires.filter((f) => {
      f.life -= dt;
      if (f.life <= 0 || !this.map) return false;
      const a = Math.sin(Math.PI * (1 - f.life / 1.8));
      const route = this.map.route;
      for (let i = 1; i < route.length; i++) {
        const [x0, y0] = route[i - 1];
        const [x1, y1] = route[i];
        const n = Math.max(1, Math.round(Math.hypot(x1 - x0, y1 - y0) * 2));
        for (let j = 0; j < n; j++) {
          const p = toPx(x0 + ((x1 - x0) * j) / n, y0 + ((y1 - y0) * j) / n);
          const fl = 0.75 + 0.25 * Math.sin(this.t * 18 + i * 3 + j);
          this.blasts.circle(p.x, p.y, 26 * fl).fill({ color: 0xff6a1a, alpha: 0.22 * a });
          this.blasts.circle(p.x, p.y - 6, 12 * fl).fill({ color: 0xffc36a, alpha: 0.3 * a });
        }
      }
      return true;
    });
    // Rain of arrows: short shafts falling onto the area.
    this.volleys = this.volleys.filter((v) => {
      v.life -= dt;
      if (v.life <= 0) return false;
      const t = 0.7 - v.life;
      for (const [dx, dy, delay] of v.arrows) {
        const k = Math.min(1, Math.max(0, (t - delay) / 0.3));
        if (k <= 0 || k >= 1) continue;
        const y = v.y + dy - 200 * (1 - k);
        this.blasts.moveTo(v.x + dx + 6, y - 22).lineTo(v.x + dx, y);
      }
      this.blasts.stroke({ width: 3, color: 0xe8d8b0 });
      return true;
    });
    // A rogue's fan of blades: a violet ring out to two cells.
    this.fans = this.fans.filter((f) => {
      f.life -= dt;
      if (f.life <= 0) return false;
      const k = 1 - f.life / 0.4;
      this.blasts.circle(f.x, f.y, 2 * CELL * k).stroke({ width: 5, color: 0xb08aff, alpha: 1 - k });
      return true;
    });
  }

  // Below a quarter of its hit points the wall burns: flames grow as it drops.
  // drawStorms: an artifact totem's lightning charge is a faint wet stain on
  // the path with a thin rim and a couple of small sparks skittering over
  // it, fading out over its last seconds.
  drawStorms() {
    const g = this.ground;
    g.clear();
    const r = 0.45 * CELL;
    for (const [sx, sy, left] of this.storms ?? []) {
      const k = Math.min(1, (left ?? 1) / 0.1); // the last tenth of its time fades it
      const p = toPx(sx, sy);
      g.circle(p.x, p.y, r).fill({ color: 0x5ac8ff, alpha: 0.1 * k }).stroke({ width: 1.5, color: 0x9ef0ff, alpha: 0.25 * k });
      // Sparks jump to a new spot a few times a second.
      const tick = Math.floor(this.t * 6);
      for (let i = 0; i < 2; i++) {
        const seed = Math.sin((tick + i * 7.3) * 12.9898 + sx * 78.233 + sy * 37.719) * 43758.5453;
        const a = (seed - Math.floor(seed)) * Math.PI * 2;
        const d = r * (0.2 + 0.5 * ((seed * 7) % 1 + 1) % 1);
        let x = p.x + Math.cos(a) * d;
        let y = p.y + Math.sin(a) * d * 0.6;
        g.moveTo(x, y);
        for (let j = 0; j < 3; j++) {
          x += 5 + j * 2;
          y += j % 2 ? 4 : -4;
          g.lineTo(x, y);
        }
        g.stroke({ width: 1.5, color: 0xbff4ff, alpha: 0.45 * k });
      }
    }
  }

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

// jagged makes a zigzag line from a to b: n segments, sideways jitter up to amp.
function jagged(a, b, n, amp) {
  const pts = [[a.x, a.y]];
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const len = Math.hypot(dx, dy) || 1;
  for (let i = 1; i < n; i++) {
    const k = i / n;
    const off = (Math.random() - 0.5) * 2 * amp;
    pts.push([a.x + dx * k - (dy / len) * off, a.y + dy * k + (dx / len) * off]);
  }
  pts.push([b.x, b.y]);
  return pts;
}
