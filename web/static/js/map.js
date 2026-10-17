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
  props.sortableChildren = true; // drawn by their feet, so lower ones overlap
  root.addChild(props);
  if (pixel) drawObstacles(props, map, pal);
  else {
    for (let y = 0; y < ROWS; y++) {
      for (let x = 0; x < COLS; x++) if (map.cells[y][x] === 'o') drawObstacle(g, map.biome, pal, x, y);
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
      s.zIndex = s.y;
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

// drawObstacles turns the obstacle cells into scenery. Touching cells form a
// group: a large group with room for a 2x2 block (and every group in the
// crypt) becomes a fenced graveyard, the rest become ruins.
function drawObstacles(layer, map, pal) {
  const isObs = (x, y) => y >= 0 && y < ROWS && x >= 0 && x < COLS && map.cells[y][x] === 'o';
  const seen = new Set();
  for (let y = 0; y < ROWS; y++) {
    for (let x = 0; x < COLS; x++) {
      if (!isObs(x, y) || seen.has(`${x},${y}`)) continue;
      const group = [];
      const stack = [[x, y]];
      seen.add(`${x},${y}`);
      while (stack.length) {
        const [cx, cy] = stack.pop();
        group.push([cx, cy]);
        for (const [dx, dy] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
          const k = `${cx + dx},${cy + dy}`;
          if (isObs(cx + dx, cy + dy) && !seen.has(k)) {
            seen.add(k);
            stack.push([cx + dx, cy + dy]);
          }
        }
      }
      const cells = new Set(group.map(([gx, gy]) => `${gx},${gy}`));
      const block = group.some(([gx, gy]) => cells.has(`${gx + 1},${gy}`) && cells.has(`${gx},${gy + 1}`) && cells.has(`${gx + 1},${gy + 1}`));
      if (map.biome === 'crypt' || (group.length >= 4 && block)) graveyard(layer, group, cells, map.biome);
      else for (const [gx, gy] of group) ruin(layer, map.biome, pal, gx, gy);
    }
  }
}

// putter places sprites on their feet, ordered by depth.
const putter = (layer) => (name, x, y, scale = PX, tint = 0xffffff) => {
  const s = sprite(name, scale);
  if (!s) return null;
  s.position.set(x, y);
  s.tint = tint;
  s.zIndex = y;
  layer.addChild(s);
  return s;
};

// ruin fills a cell with what is left of a building: a crumbling stretch of
// wall, a column, or a heap of rubble. Walls in neighbouring cells line up.
function ruin(layer, biome, pal, x, y) {
  const put = putter(layer);
  const p = toPx(x, y);
  const cx = p.x + CELL / 2;
  const bottom = p.y + CELL - 6;
  const half = CELL / 2;
  const ice = biome === 'ice';
  const stone = ice ? 0x9fd0f4 : biome === 'ruins' ? pal.pathTint : 0xffffff;
  const cap = ice ? 0xd8f0ff : stone;
  const rock = (i, rx, ry, scale = 2.5) => put(`rock_${(ice ? 3 : 0) + (cellHash(x, y, i) % 3)}`, rx, ry, scale, ice ? 0xffffff : 0x9a9088);
  const h = cellHash(x, y);
  const v = biome === 'ruins' ? (h % 10) - 3 : h % 10; // open ruins favour columns
  if (v < 5) {
    // A wall, each half standing full height, broken off low, or fallen.
    for (let i = 0; i < 2; i++) {
      const hi = cellHash(x, y, i + 1);
      const wx = p.x + i * half + half / 2;
      if (hi < 750) {
        put(hi < 60 && !ice ? 'wall_banner_red' : 'wall_mid', wx, bottom, PX, stone);
        if (hi < 380) put('wall_top_mid', wx, bottom - half, PX, cap);
      } else {
        rock(i, wx, bottom, 2.2);
      }
    }
    if (h % 3 === 0) rock(5, p.x + 18 + (h % 60), bottom + 4, 1.8);
  } else if (v < 8) {
    put('column', cx - 8, bottom - 2, 2, ice ? 0xb8e4ff : stone);
    rock(6, cx + 26, bottom + 2, 2);
  } else {
    rock(7, cx - 18, bottom - 22, 2.4);
    rock(8, cx + 16, bottom - 8, 2.8);
    if (h % 2) rock(9, cx - 22, bottom + 2, 2);
    if (h < 400) put('skull', cx + 4, bottom + 2, 2);
  }
}

// graveyard fences a group of cells with iron railings and fills it with
// graves, a dead tree here and there and a mausoleum where 2x2 cells fit.
function graveyard(layer, group, cells, biome) {
  const put = putter(layer);
  const ice = biome === 'ice';
  const stoneTint = ice ? 0xd8f0ff : 0xc8d0c8;
  const back = new Graphics(); // railings behind the graves
  const front = new Graphics(); // the near railing overlaps them
  back.zIndex = -1;
  front.zIndex = 100000;
  layer.addChild(back, front);
  const has = (x, y) => cells.has(`${x},${y}`);
  const used = new Set();
  let tombs = 0;
  // Mausoleums: one per graveyard, two in a big one.
  for (const [x, y] of group) {
    if (tombs >= (group.length >= 10 ? 2 : 1)) break;
    const block = [[x, y], [x + 1, y], [x, y + 1], [x + 1, y + 1]];
    if (block.some(([bx, by]) => !has(bx, by) || used.has(`${bx},${by}`))) continue;
    block.forEach(([bx, by]) => used.add(`${bx},${by}`));
    const p = toPx(x, y);
    put(ice ? 'mausoleum_pale' : 'mausoleum', p.x + CELL, p.y + 2 * CELL - 16, 4, ice ? 0xd8f0ff : 0xffffff);
    tombs++;
  }
  // The gate: a gap in the near railing of one cell.
  const near = group.filter(([x, y]) => !has(x, y + 1));
  const gate = near.length > 1 ? near[cellHash(group[0][0], group[0][1]) % near.length] : null;
  for (const [x, y] of group) {
    const p = toPx(x, y);
    const cx = p.x + CELL / 2;
    const bottom = p.y + CELL;
    if (!used.has(`${x},${y}`)) {
      const h = cellHash(x, y);
      if (texture('tomb_0')) {
        put(`tomb_${h % 8}`, cx - 20, bottom - 44, PX, stoneTint);
        if (h < 220) put(ice ? 'dead_tree_pale' : 'dead_tree', cx + 22, bottom - 12, 3.4);
        else put(`tomb_${(h >> 3) % 8}`, cx + 22, bottom - 14, PX, stoneTint);
      } else {
        put('crate', cx - 10, bottom - 16, PX, 0xb0b8b0);
      }
      if (h % 2) put('skull', cx - 26, bottom - 8, 2);
    }
    if (!has(x, y - 1)) railing(back, p.x, p.x + CELL, p.y + 14);
    if (!has(x, y + 1) && !(gate && gate[0] === x && gate[1] === y)) railing(front, p.x, p.x + CELL, bottom - 4);
    if (!has(x - 1, y)) post(back, p.x + 6, p.y + 14, bottom - 4);
    if (!has(x + 1, y)) post(back, p.x + CELL - 6, p.y + 14, bottom - 4);
  }
  back.fill(IRON);
  front.fill(IRON);
}

const IRON = 0x2a2327;

// railing draws a run of spiked iron bars standing on y.
function railing(g, x0, x1, y) {
  g.rect(x0, y - 16, x1 - x0, 3).rect(x0, y - 6, x1 - x0, 3);
  for (let x = x0 + 4; x < x1; x += 12) g.rect(x, y - 20, 3, 20).poly([x - 1, y - 20, x + 1.5, y - 25, x + 4, y - 20]);
}

// post draws a side railing seen from above: bars along a line going down.
function post(g, x, y0, y1) {
  g.rect(x - 1, y0 - 16, 3, y1 - y0 + 10);
  for (let y = y0; y <= y1; y += 14) g.rect(x - 2, y - 18, 5, 18).poly([x - 2, y - 18, x + 0.5, y - 23, x + 3, y - 18]);
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
