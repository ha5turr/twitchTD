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
