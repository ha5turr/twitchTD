import { W, H, BIOMES } from './layout.js';
import { buildMap, buildLabels } from './map.js';
import { Entities } from './entities.js';
import { Lighting } from './lighting.js';
import { Hud } from './hud.js';
import { connect } from './net.js';

const { Application, Container } = PIXI;

async function start() {
  // Text is measured when created, so the fonts must be ready first.
  // The sample text pulls in the Cyrillic subsets as well as the Latin ones.
  const sample = 'AZaz09 АЯаяЁё';
  await Promise.all([
    document.fonts.load('700 20px Philosopher', sample),
    document.fonts.load('400 20px Philosopher', sample),
    document.fonts.load('16px "Press Start 2P"', sample),
  ]).catch(() => {});

  const app = new Application();
  await app.init({ width: W, height: H, background: 0x07080b, antialias: true });
  document.getElementById('stage').appendChild(app.canvas);

  // Draw order: world (lit) -> lightmap (multiply) -> UI (always readable).
  const world = new Container();
  const mapLayer = new Container();
  const hud = new Hud();
  const ents = new Entities();
  world.addChild(hud.world, mapLayer, ents.world);
  const lighting = new Lighting(app.renderer);
  const ui = new Container();
  ui.addChild(buildLabels(), ents.labels, hud.ui);
  app.stage.addChild(world, lighting.sprite, ui);

  window.td = { app, hud, ents, lighting }; // for poking at the overlay from devtools
  let bossAlive = false;
  connect({
    map(m) {
      mapLayer.removeChildren().forEach((c) => c.destroy({ children: true }));
      mapLayer.addChild(buildMap(m));
      ents.clear();
      hud.setMap(m);
      lighting.setMap(m, BIOMES[m.biome] ?? BIOMES.ruins);
    },
    state(s) {
      ents.update(s);
      hud.update(s);
      bossAlive = s.enemies.some((e) => e.kind === 'boss');
    },
  });

  app.ticker.add((t) => {
    const dt = t.deltaMS / 1000;
    ents.frame(dt);
    hud.frame(dt);
    lighting.alarm += ((bossAlive ? 0.55 : 0) - lighting.alarm) * Math.min(1, dt * 2);
    lighting.render(dt, ents.lights());
  });
}

start();
