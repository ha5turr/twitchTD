// Screen layout at 1920x1080 and colour palettes. Game coordinates are in
// cells; toPx converts them to screen pixels.

export const W = 1920;
export const H = 1080;
export const CELL = 96;
export const TOP = 120; // top bar height
export const LEFT = 384; // tavern width
export const COLS = 16;
export const ROWS = 10;

export const toPx = (x, y) => ({ x: LEFT + x * CELL, y: TOP + y * CELL });

export const cellName = (x, y) => String.fromCharCode(65 + x) + (ROWS - y);

export const FONT_UI = 'Philosopher, sans-serif';
export const FONT_PIXEL = '"Press Start 2P", monospace';

// Ambient is the light level where no source shines (see lighting.js).
export const BIOMES = {
  ruins: {
    name: 'Руины', ground: 0x2a2f2a, groundAlt: 0x262b26, path: 0x4a4436, pathEdge: 0x5d5543,
    obstacle: 0x5b5f5a, ambient: 0x7c88a8, light: 0xb8c4e0,
    pathTint: 0xd8ccb0, groundTint: 0x6a7a5e, // mossy stone
  },
  dungeon: {
    name: 'Подземелье', ground: 0x24211f, groundAlt: 0x211e1c, path: 0x3f3a35, pathEdge: 0x534c44,
    obstacle: 0x14110f, ambient: 0x6e6468, light: 0xffa64d,
    pathTint: 0xffffff, groundTint: 0x6e6460,
  },
  crypt: {
    name: 'Крипта', ground: 0x1f2421, groundAlt: 0x1c201e, path: 0x37403a, pathEdge: 0x48544b,
    obstacle: 0x4a4f4c, ambient: 0x627568, light: 0x7dff9a,
    pathTint: 0xc0d0c4, groundTint: 0x5a6a60,
  },
  ice: {
    name: 'Ледяные пещеры', ground: 0x1f2a33, groundAlt: 0x1c262e, path: 0x5a7385, pathEdge: 0x7f9bb0,
    obstacle: 0x9fc4dc, ambient: 0x7890aa, light: 0x8fd4ff,
    pathTint: 0xc8e4ff, groundTint: 0x6a88a8,
  },
};

// Loot rarity colours by tower level 1..5.
export const RARITY = [0xe8e8e8, 0x4cd964, 0x3d8bff, 0xb05cff, 0xff9a2e];

// Stable per-nick colour, the same everywhere a nick appears.
export function nickColor(name) {
  let h = 0;
  for (const ch of name.toLowerCase()) h = (h * 31 + ch.codePointAt(0)) >>> 0;
  const hue = h % 360;
  return hslToHex(hue, 0.65, 0.65);
}

function hslToHex(h, s, l) {
  const k = (n) => (n + h / 30) % 12;
  const a = s * Math.min(l, 1 - l);
  const f = (n) => l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));
  return (Math.round(f(0) * 255) << 16) | (Math.round(f(8) * 255) << 8) | Math.round(f(4) * 255);
}
