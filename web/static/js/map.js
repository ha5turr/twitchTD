// Static map layer: ground, path, obstacles, torches and the chess labels.
import { BIOMES, CELL, COLS, ROWS, TOP, LEFT, toPx, FONT_UI } from './layout.js';

const { Container, Graphics, Text } = PIXI;

export function buildMap(map) {
  const pal = BIOMES[map.biome] ?? BIOMES.ruins;
  const root = new Container();
  const g = new Graphics();
  root.addChild(g);

  for (let y = 0; y < ROWS; y++) {
    for (let x = 0; x < COLS; x++) {
      const c = map.cells[y][x];
      const p = toPx(x, y);
      const ground = (x + y) % 2 ? pal.ground : pal.groundAlt;
      g.rect(p.x, p.y, CELL, CELL).fill(c === '#' ? pal.path : ground);
    }
  }
  drawPathEdges(g, map, pal);
  // Faint grid so the chess coordinates are easy to follow.
  for (let x = 0; x <= COLS; x++) g.moveTo(LEFT + x * CELL, TOP).lineTo(LEFT + x * CELL, TOP + ROWS * CELL);
  for (let y = 0; y <= ROWS; y++) g.moveTo(LEFT, TOP + y * CELL).lineTo(LEFT + COLS * CELL, TOP + y * CELL);
  g.stroke({ width: 1, color: 0xffffff, alpha: 0.05 });

  for (let y = 0; y < ROWS; y++) {
    for (let x = 0; x < COLS; x++) {
      if (map.cells[y][x] === 'o') drawObstacle(g, map.biome, pal, x, y);
    }
  }
  for (const [x, y] of map.torches) {
    const t = torchSpot(map, x, y);
    drawTorch(g, t.x, t.y);
  }
  return root;
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
