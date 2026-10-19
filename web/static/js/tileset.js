// 0x72 DungeonTileset II (CC0, https://0x72.itch.io/dungeontileset-ii):
// one sheet plus a tile list of "name x y w h". Pixel art is drawn at 3x
// (a 16 px tile becomes 48 px, two per 96 px cell) with nearest-neighbour
// scaling so the pixels stay crisp.
const { Assets, Texture, Rectangle, Sprite, AnimatedSprite } = PIXI;

export const PX = 3; // screen pixels per art pixel

const tex = new Map(); // "floor_1" -> Texture
const anims = new Map(); // "skelet_run" -> [Texture...]
let ready = false;

export async function loadTileset(base = 'assets/0x72/') {
  try {
    const [sheet, list] = await Promise.all([
      Assets.load(base + '0x72_DungeonTilesetII_v1.7.png'),
      fetch(base + 'tile_list_v1.7').then((r) => (r.ok ? r.text() : Promise.reject(new Error(r.statusText)))),
    ]);
    sheet.source.scaleMode = 'nearest';
    const groups = new Map();
    for (const line of list.split('\n')) {
      const [name, x, y, w, h] = line.trim().split(/\s+/);
      if (!h) continue;
      const t = new Texture({ source: sheet.source, frame: new Rectangle(+x, +y, +w, +h) });
      tex.set(name, t);
      const m = name.match(/^(.*)_anim_f(\d+)$/);
      if (m) {
        if (!groups.has(m[1])) groups.set(m[1], []);
        groups.get(m[1]).push([+m[2], t]);
      }
    }
    for (const [name, frames] of groups) anims.set(name, frames.sort((a, b) => a[0] - b[0]).map(([, t]) => t));
    ready = true;
  } catch (err) {
    // Without the pack the game falls back to its vector placeholders.
    console.warn('tileset not loaded, using vector art:', err);
    return;
  }
  await Promise.all(EXTRA.map(loadExtra));
}

// row lists n frames of w x h laid out left to right from (x, y).
const row = (x, y, w, h, n) => Array.from({ length: n }, (_, i) => [x + i * w, y, w, h]);
const kt = (c, r) => [[c * 17, r * 17, 16, 16]];

// Extra CC0 sheets that fill gaps in the 0x72 pack (see each LICENSE.txt):
// torches and braziers from the Extended add-on, and buildings from
// MiniWorld Sprites. Frames are given by hand: these sheets have no list.
const EXTRA = [
  {
    file: 'assets/extended/dungeontileset-extended.png',
    defs: { flame: row(128, 336, 16, 16, 8), torch_wall: row(128, 352, 16, 24, 8), torch_floor: row(128, 400, 16, 24, 8) },
  },
  {
    file: 'assets/miniworld/Ballista.png', // rows face up, down, right, left
    defs: { ballista_up: [[0, 0, 16, 16]], ballista_down: [[0, 16, 16, 16]], ballista_right: [[0, 32, 16, 16]], ballista_left: [[0, 48, 16, 16]] },
  },
  { file: 'assets/miniworld/Tower2.png', defs: { stone_tower: [[0, 0, 16, 16]] } },
  { file: 'assets/miniworld/Chapels.png', defs: { chapel: [[16, 0, 16, 16]] } },
  { file: 'assets/miniworld/SpearWall.png', defs: { spear_wall: [[0, 0, 16, 16]] } },
  {
    file: 'assets/miniworld/Tombstones.png',
    defs: Object.fromEntries(Array.from({ length: 8 }, (_, i) => [`tomb_${i}`, [[(i % 4) * 16, Math.floor(i / 4) * 16, 16, 16]]])),
  },
  { file: 'assets/soul/soul.png', defs: { soul: row(0, 0, 16, 16, 4) } },
  {
    // Superdark's Elvenking in a green robe, the druid's human form. The
    // sheet has one 4-frame cycle, used for standing and running alike.
    file: 'assets/forest/druid.png',
    defs: { druid_idle: row(0, 0, 24, 24, 4), druid_run: row(0, 0, 24, 24, 4) },
  },
  {
    // Shepardskin's black cat, the druid's panther form: the run row, facing left.
    file: 'assets/cat/cat.png',
    defs: { panther_run: [[2, 34, 22, 16], [24, 34, 22, 16], [47, 34, 22, 16], [69, 34, 22, 16], [92, 34, 22, 16], [116, 34, 21, 16]] },
  },
  {
    // ЭссДиЭф: 56x75 cells, feet centred on the bottom edge; walks facing
    // right, toward the viewer and away from him, then the burp and the fart.
    file: 'assets/sdf/sdf.png',
    defs: Object.fromEntries(['side', 'front', 'back', 'burp', 'fart'].map((n, i) => [`sdf_${n}`, row(0, i * 75, 56, 75, 4)])),
  },
  {
    file: 'assets/spiders/spiders.png', // side walk cycles facing right
    defs: { spider_purple: row(0, 0, 16, 16, 4), spider_dark: row(0, 16, 16, 16, 4), spider_orange: row(0, 32, 16, 16, 4) },
  },
  {
    // Tiny Creatures: 16 px tiles with a 1 px gap, like Kenney's sheets.
    file: 'assets/tiny-creatures/creatures.png',
    defs: { tc_wraith: kt(9, 1), tc_ice_spirit: kt(9, 4), tc_rat: kt(4, 9) },
  },
  { file: 'assets/miniworld/Mausoleum.png', defs: { mausoleum: [[0, 0, 32, 32]], mausoleum_pale: [[32, 0, 32, 32]] } },
  {
    file: 'assets/miniworld/Rocks.png', // rows: grey, mossy, green, snowy
    defs: Object.fromEntries(Array.from({ length: 6 }, (_, i) => [`rock_${i}`, [[(i % 3) * 16, (i < 3 ? 0 : 3) * 16, 16, 16]]])),
  },
  { file: 'assets/miniworld/DeadTrees.png', defs: { dead_tree: [[32, 0, 16, 16]], dead_tree_pale: [[48, 0, 16, 16]] } },
  {
    // Idylwild's Chromatic Campfires: 48 px cells, four frames per colour.
    file: 'assets/campfires/campfires.png',
    defs: {
      campfire: Array.from({ length: 4 }, (_, i) => [i * 48 + 6, 144, 36, 40]),
      campfire_blue: Array.from({ length: 4 }, (_, i) => [i * 48 + 6, 0, 36, 40]),
    },
  },
  {
    // Kenney's Roguelike Indoors: 16 px tiles with a 1 px gap, so objects
    // wider than a tile are put together from single tiles.
    file: 'assets/kenney/roguelike-indoors.png',
    defs: {
      k_chair_r: kt(2, 4),
      k_chair_l: kt(3, 4),
      k_table_top: kt(5, 0),
      k_table_mid: [[85, 7, 16, 2]], // a plain strip, stretched to length
      k_table_bot: kt(5, 1),
      k_oval_l: kt(3, 0),
      k_oval_r: kt(4, 0),
      k_candles: kt(20, 0),
      k_torch: kt(21, 6),
      k_shield: kt(22, 9),
      k_bar: kt(0, 16),
      k_bar_drawers: kt(1, 16),
      k_bar_bottles: kt(5, 16),
      k_bar_bottles2: kt(6, 16),
    },
  },
];

async function loadExtra({ file, defs }) {
  try {
    const sheet = await Assets.load(file);
    sheet.source.scaleMode = 'nearest';
    for (const [name, rects] of Object.entries(defs)) {
      const frames = rects.map(([x, y, w, h]) => new Texture({ source: sheet.source, frame: new Rectangle(x, y, w, h) }));
      tex.set(name, frames[0]);
      if (frames.length > 1) anims.set(name, frames);
    }
  } catch (err) {
    console.warn(`extra sheet ${file} not loaded:`, err); // its sprites fall back
  }
}

export const hasTileset = () => ready;

export function texture(name) {
  return tex.get(name);
}

// frames returns an animation's textures, or the single texture of that name.
export function frames(name) {
  if (anims.has(name)) return anims.get(name);
  const t = tex.get(name);
  return t ? [t] : null;
}

// sprite makes a (possibly animated) sprite standing on its bottom centre.
export function sprite(name, scale = PX, speed = 0.12) {
  const f = frames(name);
  if (!f) return null;
  let s;
  if (f.length > 1) {
    s = new AnimatedSprite(f);
    s.animationSpeed = speed;
    s.gotoAndPlay(Math.floor(Math.random() * f.length));
  } else {
    s = new Sprite(f[0]);
  }
  s.anchor.set(0.5, 1);
  s.scale.set(scale);
  return s;
}
