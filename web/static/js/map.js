// Static map layer: ground, path, obstacles, torches and the chess labels.
import { BIOMES, CELL, COLS, ROWS, TOP, LEFT, toPx, FONT_UI } from './layout.js';
import { hasTileset, texture, sprite, PX } from './tileset.js';

const { Container, Graphics, Text, Sprite } = PIXI;

export function buildMap(map) {
  const pal = BIOMES[map.biome] ?? BIOMES.ruins;
  const root = new Container();
  const tiles = new Container();
  root.addChild(tiles);
  const g = new Graphics();
  root.addChild(g);
  const pixel = hasTileset();

  for (let y = 0; y < ROWS; y++) {
    for (let x = 0; x < COLS; x++) {
      const c = map.cells[y][x];
      const p = toPx(x, y);
      if (pixel) {
        floorTiles(tiles, p, x, y, c === '#' ? pal.pathTint : pal.groundTint);
      } else {
        const ground = (x + y) % 2 ? pal.ground : pal.groundAlt;
        g.rect(p.x, p.y, CELL, CELL).fill(c === '#' ? pal.path : ground);
      }
    }
  }
  if (pixel) drawTrench(g, tiles, map, pal);
  else drawPathEdges(g, map, pal);
  // Faint grid so the chess coordinates are easy to follow.
  for (let x = 0; x <= COLS; x++) g.moveTo(LEFT + x * CELL, TOP).lineTo(LEFT + x * CELL, TOP + ROWS * CELL);
  for (let y = 0; y <= ROWS; y++) g.moveTo(LEFT, TOP + y * CELL).lineTo(LEFT + COLS * CELL, TOP + y * CELL);
  g.stroke({ width: 1, color: 0xffffff, alpha: 0.05 });

  const props = new Container(); // obstacles stand above the grid lines
  root.addChild(props);
  for (let y = 0; y < ROWS; y++) {
    for (let x = 0; x < COLS; x++) {
      if (map.cells[y][x] !== 'o') continue;
      if (!pixel || !obstacleSprites(props, map.biome, pal, x, y)) drawObstacle(g, map.biome, pal, x, y);
    }
  }
  for (const [x, y] of map.torches) {
    const t = torchSpot(map, x, y);
    // A wall torch on the path's edge, a standing one on an obstacle; the
    // flame sits where the lighting puts its glow.
    const onPath = map.cells[y][x] === '#';
    const s = pixel && sprite(onPath ? 'torch_wall' : 'torch_floor', 2, 0.2);
    if (s) {
      s.position.set(t.x, t.y + (onPath ? 34 : 38));
      props.addChild(s);
    } else {
      drawTorch(g, t.x, t.y);
    }
  }
  return root;
}

// cellHash gives each cell a stable pseudo-random number for tile variety.
const cellHash = (x, y, k = 0) => (((x * 73856093) ^ (y * 19349663) ^ (k * 83492791)) >>> 0) % 1000;

// floorTiles covers a cell with 2x2 floor tiles, mostly plain, sometimes cracked.
function floorTiles(layer, p, x, y, tint) {
  const half = CELL / 2;
  for (let j = 0; j < 2; j++) {
    for (let i = 0; i < 2; i++) {
      const h = cellHash(x * 2 + i, y * 2 + j);
      const name = h < 700 ? 'floor_1' : `floor_${2 + (h % 7)}`;
      const s = new Sprite(texture(name));
      s.scale.set(PX);
      s.position.set(p.x + i * half, p.y + j * half);
      s.tint = tint;
      layer.addChild(s);
    }
  }
}

// obstacleSprites draws an obstacle from the pack; false means "no art for
// this biome", and the caller draws the vector version instead.
function obstacleSprites(layer, biome, pal, x, y) {
  const p = toPx(x, y);
  const cx = p.x + CELL / 2;
  const bottom = p.y + CELL;
  const put = (name, px, py, scale = PX, tint = 0xffffff) => {
    const s = sprite(name, scale);
    s.position.set(px, py);
    s.tint = tint;
    layer.addChild(s);
    return s;
  };
  switch (biome) {
    case 'dungeon': { // a block of wall: top row on the upper half, face below
      const half = CELL / 2;
      for (let i = 0; i < 2; i++) {
        put('wall_top_mid', p.x + i * half + half / 2, p.y + half);
        put(cellHash(x, y, i) < 150 ? 'wall_banner_red' : 'wall_mid', p.x + i * half + half / 2, bottom);
      }
      return true;
    }
    case 'ruins': // a lone column, a little shorter so it stays in its cell
      put('column', cx, bottom - 4, 2, pal.pathTint);
      return true;
    case 'crypt': { // rows of graves; crates if the tombstone sheet is missing
      const h = cellHash(x, y);
      const tomb = `tomb_${h % 8}`;
      if (!texture(tomb)) {
        put('crate', cx - 10, bottom - 12, PX, 0xb0b8b0);
        return true;
      }
      put(tomb, cx - 20, bottom - 40, PX, 0xc8d0c8);
      put(`tomb_${(h >> 3) % 8}`, cx + 22, bottom - 6, PX, 0xb8c0b8);
      if (h < 500) put('skull', cx - 24, bottom - 2, 2);
      return true;
    }
    case 'ice': { // the pack has no ice, so stone is frozen with a tint
      const half = CELL / 2;
      if (cellHash(x, y) < 400) {
        put('column', cx, bottom - 4, 2, 0xb8e4ff); // a pillar of ice
        return true;
      }
      for (let i = 0; i < 2; i++) {
        put('wall_top_mid', p.x + i * half + half / 2, p.y + half, PX, 0xd8f0ff);
        put('wall_mid', p.x + i * half + half / 2, bottom, PX, 0x9fd0f4);
      }
      return true;
    }
  }
  return false;
}

// drawTrench sinks the enemy path below the building ground: a cliff face
// with a shadow under every edge that faces "north", a shaded west wall, a
// dark seam where the ground in front rises again, and a light lip along the
// raised ground. Light comes from the top left.
function drawTrench(g, layer, map, pal) {
  const isPath = (x, y) => y >= 0 && y < ROWS && x >= 0 && x < COLS && map.cells[y][x] === '#';
  const face = 24; // one row of small bricks
  for (const [x, y] of map.path) {
    const p = toPx(x, y);
    if (!isPath(x, y - 1)) {
      // The cliff face of the raised ground: real brickwork from the pack.
      for (let i = 0; i < 4; i++) {
        const s = new Sprite(texture('wall_mid'));
        s.scale.set(1.5);
        s.position.set(p.x + i * 24, p.y);
        s.tint = pal.cliffTint;
        layer.addChild(s);
      }
      g.rect(p.x, p.y + face, CELL, 12).fill({ color: 0x000000, alpha: 0.45 }); // shadow cast down
      g.rect(p.x, p.y + face + 12, CELL, 10).fill({ color: 0x000000, alpha: 0.2 });
      g.rect(p.x, p.y - 3, CELL, 3).fill({ color: 0xffffff, alpha: 0.22 }); // lip of the ground above
    }
    if (!isPath(x - 1, y) && x > 0) {
      g.rect(p.x, p.y, 16, CELL).fill({ color: 0x000000, alpha: 0.45 }); // west side in shade
      g.rect(p.x - 3, p.y, 3, CELL).fill({ color: 0xffffff, alpha: 0.16 });
    }
    if (!isPath(x + 1, y) && x < COLS - 1) {
      g.rect(p.x + CELL - 8, p.y, 8, CELL).fill({ color: 0x000000, alpha: 0.3 });
    }
    if (!isPath(x, y + 1)) {
      g.rect(p.x, p.y + CELL - 6, CELL, 6).fill({ color: 0x000000, alpha: 0.55 }); // ground rises again
    }
  }
}

function drawPathEdges(g, map, pal) {
  const isPath = (x, y) => y >= 0 && y < ROWS && x >= 0 && x < COLS && map.cells[y][x] === '#';
  for (const [x, y] of map.path) {
    const p = toPx(x, y);
    const e = 6;
    if (!isPath(x, y - 1)) g.rect(p.x, p.y, CELL, e);
    if (!isPath(x, y + 1)) g.rect(p.x, p.y + CELL - e, CELL, e);
    if (!isPath(x - 1, y) && x > 0) g.rect(p.x, p.y, e, CELL);
    if (!isPath(x + 1, y) && x < COLS - 1) g.rect(p.x + CELL - e, p.y, e, CELL);
  }
  g.fill(pal.pathEdge);
  // Stones scattered on the path, positioned from the cell so they are stable.
  for (const [x, y] of map.path) {
    const p = toPx(x, y);
    const h = (x * 73856093) ^ (y * 19349663);
    for (let i = 0; i < 3; i++) {
      const sx = p.x + 16 + ((h >> (i * 5)) & 63);
      const sy = p.y + 16 + ((h >> (i * 7 + 3)) & 63);
      g.circle(sx, sy, 2 + ((h >> i) & 3));
    }
  }
  g.fill({ color: 0x000000, alpha: 0.18 });
}

function drawObstacle(g, biome, pal, x, y) {
  const p = toPx(x, y);
  const cx = p.x + CELL / 2;
  const cy = p.y + CELL / 2;
  switch (biome) {
    case 'dungeon': // solid wall block with mortar lines
      g.rect(p.x + 2, p.y + 2, CELL - 4, CELL - 4).fill(pal.obstacle);
      g.rect(p.x + 2, p.y + 2, CELL - 4, CELL - 4).stroke({ width: 2, color: 0x2e2925 });
      g.moveTo(p.x + 2, cy).lineTo(p.x + CELL - 2, cy).stroke({ width: 2, color: 0x2e2925 });
      break;
    case 'crypt': // sarcophagus
      g.roundRect(cx - 22, cy - 38, 44, 76, 10).fill(pal.obstacle);
      g.roundRect(cx - 14, cy - 28, 28, 56, 6).stroke({ width: 2, color: 0x2a2f2c });
      break;
    case 'ice': // crystal
      g.poly([cx, cy - 38, cx + 22, cy, cx, cy + 38, cx - 22, cy]).fill({ color: pal.obstacle, alpha: 0.9 });
      g.poly([cx, cy - 38, cx + 22, cy, cx, cy]).fill({ color: 0xffffff, alpha: 0.25 });
      break;
    default: // ruins: broken column
      g.circle(cx, cy, 30).fill(pal.obstacle);
      g.circle(cx, cy, 30).stroke({ width: 3, color: 0x3d413c });
      g.poly([cx - 10, cy - 30, cx + 6, cy - 6, cx - 4, cy + 4]).fill(0x3d413c);
  }
}

// torchSpot returns the flame position: the middle of an obstacle, or the
// corner of a path cell, where it reads as a sconce and never blocks the way.
export function torchSpot(map, x, y) {
  const p = toPx(x, y);
  if (map.cells[y][x] === '#') return { x: p.x + 14, y: p.y + 18 };
  return { x: p.x + CELL / 2, y: p.y + CELL / 2 - 10 };
}

function drawTorch(g, x, y) {
  g.rect(x - 4, y + 4, 8, 22).fill(0x3b2a1a);
  g.circle(x, y, 9).fill(0xffa24a);
  g.circle(x, y - 2, 5).fill(0xffe2a0);
}

// Chess labels: letters along the top of the field, numbers down column A.
// Both edges stay clear of the default webcam corner (bottom right). They live
// above the lighting layer so darkness never hides them.
export function buildLabels() {
  const c = new Container();
  const style = {
    fontFamily: FONT_UI, fontSize: 26, fontWeight: '700', fill: 0xf2e6c9,
    stroke: { color: 0x000000, width: 5 },
  };
  for (let x = 0; x < COLS; x++) {
    const t = new Text({ text: String.fromCharCode(65 + x), style });
    t.anchor.set(0.5, 0);
    t.position.set(LEFT + x * CELL + CELL / 2, TOP + 2);
    t.alpha = 0.85;
    c.addChild(t);
  }
  for (let y = 0; y < ROWS; y++) {
    const t = new Text({ text: String(ROWS - y), style });
    t.anchor.set(0, 0.5);
    t.position.set(LEFT + 6, TOP + y * CELL + CELL / 2);
    t.alpha = 0.85;
    c.addChild(t);
  }
  return c;
}
