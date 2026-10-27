import { W, H, BIOMES } from './layout.js';
import { buildMap, buildLabels } from './map.js';
import { Entities } from './entities.js';
import { Lighting } from './lighting.js';
import { Hud } from './hud.js';
import { Tavern } from './tavern.js';
import { connect } from './net.js';
import { Sound, EVENT_SOUNDS } from './audio.js';
import { Particles } from './particles.js';
import { loadTileset } from './tileset.js';
import { Bubbles } from './bubbles.js';
import { loadEmotes } from './emotes.js';
import { KingScene } from './king.js';

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
  await loadTileset();

  const app = new Application();
  await app.init({ width: W, height: H, background: 0x07080b, antialias: true });
  document.getElementById('stage').appendChild(app.canvas);

  // Draw order: world (lit) -> lightmap (multiply) -> UI (always readable).
  const world = new Container();
  const mapLayer = new Container();
  const hud = new Hud();
  const ents = new Entities();
  const tavern = new Tavern();
  world.addChild(tavern.world, hud.world, mapLayer, hud.ground, ents.world);
  const lighting = new Lighting(app.renderer);
  const ui = new Container();
  const fx = new Particles();
  ui.addChild(buildLabels(), fx.g, ents.labels, tavern.ui, hud.ui);
  app.stage.addChild(world, lighting.sprite, ui);

  const bubbles = new Bubbles(app.canvas);
  tavern.bubbles = ents.bubbles = bubbles;
  window.td = { app, hud, ents, tavern, lighting, bubbles }; // for poking at the overlay from devtools
  const sound = new Sound();
  window.td.sound = sound;
  const king = new KingScene(sound, () => net.send({ t: 'kingDone' })); // the scene is over: let the king in
  ui.addChild(king.layer);
  window.td.king = king;
  let finalOn = false;
  let bossAlive = false;
  let shots = new Set();
  let walls = null;
  const net = connect({
    map(m) {
      king.endMusic(1); // a new map: the King's theme stops
      mapLayer.removeChildren().forEach((c) => c.destroy({ children: true }));
      mapLayer.addChild(buildMap(m));
      ents.clear();
      hud.setMap(m);
      tavern.setMap(m);
      lighting.setMap(m, BIOMES[m.biome] ?? BIOMES.ruins);
      sound.setBiome(m.biome);
      loadEmotes(m.channels ?? []);
    },
    state(s) {
      ents.update(s);
      hud.update(s);
      tavern.update(s);
      bossAlive = s.enemies.some((e) => e.boss);
      playSounds(s);
      king.update(s); // his health first: near death his powers keep quiet
      for (const ev of s.events ?? []) {
        fx.onEvent(ev);
        king.onEvent(ev, finalOn);
      }
      finalOn = !!s.match?.final && s.phase === 'wave';
      if (finalOn && s.enemies.some((e) => e.kind === 'king')) king.keepMusic(); // the overlay reloaded mid-final
    },
    settings(s) {
      sound.configure(s.values ?? {});
    },
    soundTest(m) {
      sound.test(m.group);
    },
  });

  function playSounds(s) {
    for (const ev of s.events ?? []) {
      if (ev.kind === 'waveStart') sound.play(ev.text ? 'horn' : 'bell');
      else if (EVENT_SOUNDS[ev.kind]) sound.play(EVENT_SOUNDS[ev.kind]);
    }
    // New projectiles are shots: one sound per kind, throttled in Sound.
    const now = new Set();
    for (const p of s.proj) {
      now.add(p.id);
      if (!shots.has(p.id)) sound.play(p.kind);
    }
    shots = now;
    if (walls !== null && s.walls < walls) sound.play('wallHit');
    walls = s.walls;
  }

  app.ticker.add((t) => {
    const dt = t.deltaMS / 1000;
    ents.frame(dt);
    hud.frame(dt);
    tavern.frame(dt);
    fx.frame(dt);
    bubbles.frame(dt);
    king.frame(dt);
    lighting.alarm += ((bossAlive ? 0.55 : 0) - lighting.alarm) * Math.min(1, dt * 2);
    lighting.render(dt, [...ents.lights(), ...hud.lights()]);
  });
}

start();
