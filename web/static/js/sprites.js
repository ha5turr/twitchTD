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
};

export const TOWERS = {
  arrow: { name: 'Пост лучников', light: 0xffb36b },
  magic: { name: 'Тёмный обелиск', light: 0xb58cff },
  fire: { name: 'Жаровня', light: 0xff7a2a },
  shrine: { name: 'Святилище', light: 0xffe08a },
  trap: { name: 'Капканы' },
  drums: { name: 'Барабаны войны' },
  wall: { name: 'Баррикада' },
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
export function drawTower(g, kind, rarity, level) {
  if (kind === 'trap') {
    // Spikes flush with the road, no stone base: it is a trap after all.
    for (let i = 0; i < 3; i++) {
      for (let j = 0; j < 3; j++) g.poly([-24 + i * 20, 16 - j * 16, -18 + i * 20, 4 - j * 16, -12 + i * 20, 16 - j * 16]);
    }
    g.fill(0x9aa0a8);
    g.circle(0, 0, 30).stroke({ width: 3, color: rarity, alpha: 0.9 });
    return;
  }
  if (kind === 'wall') {
    for (let i = -1; i <= 1; i++) g.rect(-36, i * 16 - 6, 72, 12).fill(0x7a5230).stroke({ width: 2, color: 0x2a1b0f });
    g.moveTo(-30, -30).lineTo(30, 30).moveTo(30, -30).lineTo(-30, 30).stroke({ width: 5, color: 0x5a3a1f });
    g.rect(-38, -28, 76, 56).stroke({ width: 3, color: rarity });
    return;
  }
  g.roundRect(-34, -34, 68, 68, 10).fill(0x1b1a1f).stroke({ width: 4, color: rarity });
  switch (kind) {
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
    case 'drums':
      g.ellipse(-10, 4, 12, 16).fill(0x8a3a2a).stroke({ width: 2, color: 0x2a1b0f });
      g.ellipse(12, 6, 10, 13).fill(0x7a5a2a).stroke({ width: 2, color: 0x2a1b0f });
      g.moveTo(-20, -20).lineTo(-6, -8).moveTo(20, -22).lineTo(10, -10).stroke({ width: 3, color: 0xe8c9a0 });
      break;
    default: // arrow
      g.rect(-18, -10, 36, 26).fill(0x5a4630);
      g.poly([-22, -10, 0, -28, 22, -10]).fill(0xc8a165);
      g.rect(-3, -2, 6, 12).fill(0x1b1a1f);
  }
  // Pips under the icon repeat the level for colour-blind viewers.
  for (let i = 0; i < level; i++) g.circle(-16 + i * 8, 26, 2.5).fill(rarity);
}

// drawProjectile draws a shot of the given building kind.
export function drawProjectile(g, kind) {
  if (kind === 'magic') g.circle(0, 0, 8).fill(0xc9a6ff).circle(0, 0, 4).fill(0xffffff);
  else if (kind === 'fire') g.circle(0, 0, 9).fill(0xff6a1a).circle(0, 0, 5).fill(0xffd36a);
  else g.circle(0, 0, 4).fill(0xffe3a8);
}
