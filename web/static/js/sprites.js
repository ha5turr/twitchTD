// Placeholder art drawn with vector shapes until real sprites arrive: every
// enemy and building has its own silhouette so viewers can tell them apart
// on a compressed stream.
import { CELL } from './layout.js';

const INK = 0x111111;

export const ENEMIES = {
  skeleton: { name: 'Скелет', color: 0xd8d2c0, r: 0.22 },
  ghoul: { name: 'Гуль', color: 0x7f9a6a, r: 0.19 },
  cultist: { name: 'Культист', color: 0x8a1f2a, r: 0.21 },
  spider: { name: 'Паук', color: 0x26222a, r: 0.2 },
  rat: { name: 'Крыса', color: 0x7a5a42, r: 0.14 },
  troll: { name: 'Тролль', color: 0x5d7a4a, r: 0.3 },
  mummy: { name: 'Мумия', color: 0xc9b98f, r: 0.24 },
  ghost: { name: 'Призрак', color: 0xe6f0ff, r: 0.22 },
  necromancer: { name: 'Некромант', color: 0x4a2a6a, r: 0.21 },
  iceskel: { name: 'Ледяной скелет', color: 0xa8d8f0, r: 0.24 },
  frostspirit: { name: 'Инеистый дух', color: 0x9fe8ff, r: 0.2 },

  bone_knight: { name: 'Костяной Рыцарь', color: 0xd8d2c0, r: 0.42, boss: true },
  cult_leader: { name: 'Вожак Культа', color: 0xa8202a, r: 0.42, boss: true },
  spider_queen: { name: 'Королева Пауков', color: 0x2e1f33, r: 0.44, boss: true },
  cave_troll: { name: 'Пещерный Тролль', color: 0x4f6a3e, r: 0.48, boss: true },
  lich: { name: 'Лич', color: 0x5a3a8a, r: 0.42, boss: true },
  ghost_king: { name: 'Призрачный Король', color: 0xdff0ff, r: 0.44, boss: true },
  ice_golem: { name: 'Ледяной Голем', color: 0x8fc8e8, r: 0.48, boss: true },
  frost_demon: { name: 'Демон Стужи', color: 0x5ab0e0, r: 0.44, boss: true },
  sdf: { name: 'ЭссДиЭф', color: 0x8a9a3a, r: 0.5, boss: true },
};

export const TOWERS = {
  arrow: { name: 'Пост лучников', light: 0xffb36b },
  magic: { name: 'Тёмный обелиск', light: 0xb58cff },
  fire: { name: 'Жаровня', light: 0xff7a2a },
  shrine: { name: 'Святилище', light: 0xffe08a },
  trap: { name: 'Капканы' },
  drums: { name: 'Барабаны войны' },
  wall: { name: 'Баррикада' },
  clown: { name: 'Клоун', light: 0xff5ab0 },
};

const eyes = (g, r, color = 0x1a0f0f) =>
  g.circle(-r * 0.35, -r * 0.15, Math.max(2, r * 0.15)).circle(r * 0.35, -r * 0.15, Math.max(2, r * 0.15)).fill(color);

const crown = (g, r, color) => {
  for (let i = -2; i <= 2; i++) g.poly([i * 12 - 7, -r + 6, i * 12, -r - 16 - (i === 0 ? 8 : 0), i * 12 + 7, -r + 6]);
  g.fill(color);
};

// drawEnemy draws an enemy centred on (0, 0).
export function drawEnemy(g, kind) {
  const st = ENEMIES[kind] ?? ENEMIES.skeleton;
  const r = st.r * CELL;
  const c = st.color;
  const base = kind.replace(/_.*/, '');
  if (st.boss) crown(g, r, kind.startsWith('ice') || kind.startsWith('frost') ? 0xbfe8ff : 0x6e1d1d);
  switch (kind) {
    case 'spider':
    case 'spider_queen':
      for (let i = 0; i < 4; i++) {
        const a = -0.9 + i * 0.6;
        g.moveTo(0, 0).lineTo(-r * 1.5, Math.sin(a) * r * 1.2).moveTo(0, 0).lineTo(r * 1.5, Math.sin(a) * r * 1.2);
      }
      g.stroke({ width: 3, color: INK });
      g.ellipse(0, 0, r, r * 0.8).fill(c).stroke({ width: 2, color: 0x6a3a8a });
      eyes(g, r, 0xff3b3b);
      return;
    case 'rat':
      g.ellipse(0, 0, r * 1.3, r * 0.8).fill(c).stroke({ width: 2, color: INK });
      g.moveTo(r * 1.2, 0).quadraticCurveTo(r * 2.2, -r, r * 2.4, r * 0.4).stroke({ width: 2, color: 0xc89a8a });
      g.circle(-r * 0.8, -r * 0.3, 2).fill(INK);
      return;
    case 'ghost':
    case 'frostspirit':
    case 'ghost_king':
      g.circle(0, -r * 0.2, r).fill({ color: c, alpha: 0.7 });
      g.poly([-r, -r * 0.2, -r, r, -r * 0.5, r * 0.7, 0, r, r * 0.5, r * 0.7, r, r, r, -r * 0.2]).fill({ color: c, alpha: 0.7 });
      eyes(g, r, 0x223355);
      return;
  }
  // Humanoids: a round body with details per kind.
  g.circle(0, 0, r).fill(c).stroke({ width: 3, color: INK });
  eyes(g, r, kind === 'necromancer' || kind === 'lich' ? 0x7dff9a : 0x1a0f0f);
  switch (base) {
    case 'mummy':
      for (let i = -1; i <= 1; i++) g.rect(-r * 0.9, i * r * 0.45 + r * 0.1, r * 1.8, 3).fill(0x8a7a5a);
      break;
    case 'iceskel':
    case 'ice':
      g.poly([-r * 0.6, -r * 0.8, 0, -r * 1.4, r * 0.6, -r * 0.8]).fill(0xe0f6ff);
      break;
    case 'cultist':
    case 'cult':
    case 'necromancer':
    case 'lich':
      g.arc(0, 0, r, Math.PI * 1.05, Math.PI * 1.95).stroke({ width: 5, color: 0x1a0a14 }); // hood
      break;
    case 'troll':
    case 'cave':
      g.poly([-r * 0.3, r * 0.3, -r * 0.15, r * 0.65, 0, r * 0.3]).poly([r * 0.3, r * 0.3, r * 0.15, r * 0.65, 0, r * 0.3]).fill(0xf2e6c9); // tusks
      break;
    case 'bone':
      g.rect(-r * 0.9, -r * 0.1, r * 1.8, r * 0.3).fill(0x6a6f7a); // visor
      break;
    case 'frost':
      g.poly([-r * 0.9, -r * 0.5, -r * 1.3, -r * 1.3, -r * 0.4, -r * 0.8]).poly([r * 0.9, -r * 0.5, r * 1.3, -r * 1.3, r * 0.4, -r * 0.8]).fill(0xe0f6ff); // horns
      break;
    case 'ghoul':
      g.poly([-r, 0, -r * 1.4, r * 0.6, -r * 0.6, r * 0.3]).poly([r, 0, r * 1.4, r * 0.6, r * 0.6, r * 0.3]).fill(c); // claws
      break;
  }
}

// drawTower draws a building centred on (0, 0) at full health.
const STONE = 0x1b1a1f;
const STONE_EDGE = 0x4a4238;

// qualityDots shows a building's quality: one dot per level in the loot
// rarity colour (white, green, blue, purple, orange), stacked bottom up in a
// thin column at the right edge of the cell. There it clears the sprite in
// the middle, the owner's nick below and the progress bar above.
const DOTS_X = 41; // between the stone base (34) and the cell edge (48)
export function qualityDots(g, rarity, level) {
  const gap = 10;
  const top = ((level - 1) * gap) / 2; // the column is centred on the cell
  g.roundRect(DOTS_X - 7, -top - 7, 14, (level - 1) * gap + 14, 7).fill({ color: 0x000000, alpha: 0.6 });
  for (let i = 0; i < level; i++) {
    g.circle(DOTS_X, top - i * gap, 4).fill(rarity).stroke({ width: 1.5, color: 0x000000 });
  }
}

// drawTower draws a building centred on (0, 0). pixel: the tileset is
// loaded, so buildings stand on the floor with no stone plate behind them;
// art: the building has sprites, and this only adds the quality dots.
export function drawTower(g, kind, rarity, level, { pixel = false, art = false } = {}) {
  if (art) {
    if (kind === 'drums') {
      // A war drum at the bard's side, in the pack's dark outline style.
      g.rect(10, 6, 22, 22).fill(0x8a3a2a).stroke({ width: 3, color: 0x1a0f0f });
      g.ellipse(21, 6, 11, 5).fill(0xe8d4a8).stroke({ width: 3, color: 0x1a0f0f });
      g.moveTo(12, 14).lineTo(30, 24).moveTo(30, 14).lineTo(12, 24).stroke({ width: 2, color: 0xd8b070 });
      g.moveTo(26, -10).lineTo(20, 2).stroke({ width: 3, color: 0x5a3a1f });
    }
    qualityDots(g, rarity, level);
    return;
  }
  if (!pixel) g.roundRect(-34, -34, 68, 68, 10).fill(STONE).stroke({ width: 3, color: STONE_EDGE });
  switch (kind) {
    case 'trap':
      for (let i = 0; i < 3; i++) {
        for (let j = 0; j < 3; j++) g.poly([-24 + i * 20, 16 - j * 16, -18 + i * 20, 4 - j * 16, -12 + i * 20, 16 - j * 16]);
      }
      g.fill(0x9aa0a8);
      break;
    case 'wall':
      for (let i = -1; i <= 1; i++) g.rect(-30, i * 16 - 6, 60, 12).fill(0x7a5230).stroke({ width: 2, color: 0x2a1b0f });
      break;
    case 'magic':
      g.poly([0, -26, 14, 0, 0, 26, -14, 0]).fill(0x9b6bff);
      g.circle(0, 0, 6).fill(0xffffff);
      break;
    case 'fire':
      g.rect(-18, 4, 36, 14).fill(0x3a2a1a);
      g.poly([-14, 4, -6, -22, 0, -6, 6, -26, 14, 4]).fill(0xff6a1a);
      g.poly([-6, 4, 0, -12, 6, 4]).fill(0xffd36a);
      break;
    case 'shrine':
      g.rect(-16, 2, 32, 18).fill(0x8a8070);
      g.rect(-4, -24, 8, 28).fill(0xffe08a);
      g.rect(-12, -16, 24, 7).fill(0xffe08a);
      break;
    case 'clown':
      g.circle(0, -4, 22).fill(0xfff0f6).stroke({ width: 3, color: 0x2a1b0f });
      g.circle(0, 0, 6).fill(0xe02a2a);
      g.circle(-8, -10, 3).circle(8, -10, 3).fill(0x2a1b0f);
      break;
    case 'drums':
      g.ellipse(-10, 4, 12, 16).fill(0x8a3a2a).stroke({ width: 2, color: 0x2a1b0f });
      g.ellipse(12, 6, 10, 13).fill(0x7a5a2a).stroke({ width: 2, color: 0x2a1b0f });
      g.moveTo(-20, -20).lineTo(-6, -8).moveTo(20, -22).lineTo(10, -10).stroke({ width: 3, color: 0xe8c9a0 });
      break;
    default: // arrow
      g.rect(-18, -10, 36, 26).fill(0x5a4630);
      g.poly([-22, -10, 0, -28, 22, -10]).fill(0xc8a165);
      g.rect(-3, -2, 6, 12).fill(STONE);
  }
  qualityDots(g, rarity, level);
}

// drawProjectile draws a shot of the given building kind.
export function drawProjectile(g, kind) {
  if (kind === 'magic') g.circle(0, 0, 8).fill(0xc9a6ff).circle(0, 0, 4).fill(0xffffff);
  else if (kind === 'fire') g.circle(0, 0, 9).fill(0xff6a1a).circle(0, 0, 5).fill(0xffd36a);
  else g.circle(0, 0, 4).fill(0xffe3a8);
}

// Pixel art, mostly from the 0x72 pack; the vector drawings above stay as
// the fallback while a sheet is missing.
export const ENEMY_ART = {
  skeleton: { name: 'skelet_run', scale: 3 },
  ghoul: { name: 'tiny_zombie_run', scale: 3 },
  cultist: { name: 'orc_shaman_run', scale: 3 },
  troll: { name: 'ogre_run', scale: 2.4 },
  mummy: { name: 'zombie', scale: 3.2 },
  necromancer: { name: 'necromancer', scale: 3 },
  iceskel: { name: 'ice_zombie', scale: 3.2 },
  bone_knight: { name: 'big_zombie_run', scale: 3.6 },
  cult_leader: { name: 'masked_orc_run', scale: 4.4 },
  cave_troll: { name: 'ogre_run', scale: 3.6, tint: 0xb8b8a0 },
  lich: { name: 'necromancer', scale: 5, tint: 0xd0a8ff },
  ice_golem: { name: 'ice_zombie', scale: 6.4 },
  frost_demon: { name: 'big_demon_run', scale: 3.6, tint: 0xa8dcff },
  // From other CC0 sheets. float: hovers up and down; hop: a still sprite
  // that bounces as it runs.
  ghost: { name: 'soul', scale: 3, float: true, alpha: 0.85 },
  frostspirit: { name: 'tc_ice_spirit', scale: 2.6, float: true, alpha: 0.9 },
  ghost_king: { name: 'tc_wraith', scale: 5, float: true },
  rat: { name: 'tc_rat', scale: 2.2, hop: true },
  spider: { name: 'spider_dark', scale: 3.6 },
  spider_queen: { name: 'spider_purple', scale: 5.5 },
  // dirs: walk cycles by heading (side ones face right); acts: the burp
  // and the fart he stands still for.
  sdf: {
    name: 'sdf_side', scale: 1.8, speed: 0.08,
    dirs: { side: 'sdf_side', down: 'sdf_front', up: 'sdf_back' },
    acts: { 1: 'sdf_burp', 2: 'sdf_fart' },
  },
};

// Building icons from the pack, drawn over the stone base.

// Mercenaries look like the pack's heroes, one per class.
export const HERO_ART = {
  ranger: 'elf_m', mage: 'wizzard_m', pyro: 'wizzard_f', cleric: 'angel',
  rogue: 'rogue', bard: 'elf_f', warrior: 'knight_m', druid: 'druid',
};
// Building art: each entry is a list of sprite layers drawn bottom to top,
// tried in order of preference so a missing extra sheet falls back to the
// 0x72 pack. aim: the sprite turns toward the nearest enemy (ballista).
export const TOWER_ART = {
  arrow: [[{ name: 'ballista_left', scale: 3, y: 26, aim: 'ballista' }], [{ name: 'weapon_bow', scale: 2.2, y: 26 }]],
  magic: [
    [{ name: 'stone_tower', scale: 3, y: 26, tint: 0xc8b8ff }, { name: 'weapon_green_magic_staff', scale: 1.4, y: 6 }],
    [{ name: 'weapon_green_magic_staff', scale: 2.2, y: 32 }],
  ],
  fire: [
    [{ name: 'campfire', scale: 2, y: 32, speed: 0.15 }],
    [{ name: 'torch_floor', scale: 2.4, y: 30, speed: 0.18 }],
    [{ name: 'wall_fountain_mid_red', scale: 3, y: 24 }],
  ],
  shrine: [[{ name: 'chapel', scale: 3, y: 26 }], [{ name: 'wall_fountain_mid_blue', scale: 3, y: 24 }]],
  // A tower with a clown on top; text layers are emoji drawn by the font.
  clown: [
    [{ name: 'stone_tower', scale: 3, y: 26, tint: 0xffb8e0 }, { text: '🤡', size: 40, y: -30 }],
    [{ text: '🤡', size: 56, y: 0 }],
  ],
  // The bard is the pack's elf heroine; drawTower adds her drum.
  drums: [[{ name: 'elf_f_idle', scale: 2.6, x: -8, y: 32, speed: 0.1 }]],
};

// Path buildings stay small so the trench and the enemies in it stay visible.
const PATH_ART = {
  // Set low in the cell, clear of the cliff face along the trench's top edge.
  trap: [[{ name: 'floor_spikes', scale: 3, y: 28, speed: 0.08 }]],
  wall: [[{ name: 'spear_wall', scale: 3, y: 30 }], [{ name: 'crate', scale: 2, x: -16, y: 30 }, { name: 'crate', scale: 2, x: 16, y: 30 }]],
};

// emoji makes a centred emoji, drawn by the system's colour emoji font.
export function emoji(s, size, x = 0, y = 0) {
  const t = new PIXI.Text({ text: s, style: { fontSize: size, fontFamily: 'Segoe UI Emoji, Apple Color Emoji, Noto Color Emoji, sans-serif' } });
  t.anchor.set(0.5);
  t.position.set(x, y);
  return t;
}

// towerArt builds a building's sprites, or null to keep the vector drawing.
export function towerArt(kind, makeSprite) {
  for (const layers of TOWER_ART[kind] ?? PATH_ART[kind] ?? []) {
    const sprites = layers.map((l) => {
      if (l.text) return emoji(l.text, l.size, l.x ?? 0, l.y);
      const s = makeSprite(l.name, l.scale, l.speed ?? 0.12);
      if (!s) return null;
      s.position.set(l.x ?? 0, l.y);
      if (l.tint) s.tint = l.tint;
      if (l.aim) s.aim = l.aim;
      return s;
    });
    if (sprites.every(Boolean)) {
      const c = new PIXI.Container();
      c.addChild(...sprites);
      c.aimSprite = sprites.find((s) => s.aim);
      return c;
    }
  }
  return null;
}
