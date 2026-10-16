// Short-lived effects: shards when an enemy dies, dust when a building is
// finished, and a frame on the cell a chatter just ordered built, so they
// see at once that the command was understood.
import { CELL, toPx } from './layout.js';
import { ENEMIES } from './sprites.js';

const { Graphics } = PIXI;

export class Particles {
  constructor() {
    this.g = new Graphics();
    this.bits = [];
    this.marks = [];
  }

  onEvent(ev) {
    switch (ev.kind) {
      case 'kill': {
        const p = toPx(ev.x, ev.y);
        const color = ENEMIES[ev.text]?.color ?? 0xd8d2c0;
        const n = ENEMIES[ev.text]?.boss ? 40 : 10;
        for (let i = 0; i < n; i++) {
          const a = Math.random() * Math.PI * 2;
          const v = 60 + Math.random() * (n > 10 ? 260 : 140);
          this.bits.push({ x: p.x, y: p.y, vx: Math.cos(a) * v, vy: Math.sin(a) * v - 60, life: 0.6, max: 0.6, r: 2 + Math.random() * 3, color });
        }
        break;
      }
      case 'built':
      case 'upgraded': {
        const p = toPx(ev.x, ev.y);
        for (let i = 0; i < 14; i++) {
          const a = (i / 14) * Math.PI * 2;
          this.bits.push({ x: p.x, y: p.y + 20, vx: Math.cos(a) * 90, vy: Math.sin(a) * 30 - 20, life: 0.7, max: 0.7, r: 5, color: 0xb8a888, drag: 3 });
        }
        break;
      }
      case 'build': {
        const p = toPx(ev.x - 0.5, ev.y - 0.5); // ev is the cell centre
        this.marks.push({ x: p.x, y: p.y, life: 1.4 });
        break;
      }
    }
  }

  frame(dt) {
    const g = this.g;
    g.clear();
    this.bits = this.bits.filter((b) => {
      b.life -= dt;
      if (b.life <= 0) return false;
      if (b.drag) {
        b.vx *= 1 - b.drag * dt;
        b.vy *= 1 - b.drag * dt;
      } else {
        b.vy += 420 * dt; // shards fall
      }
      b.x += b.vx * dt;
      b.y += b.vy * dt;
      g.circle(b.x, b.y, b.r).fill({ color: b.color, alpha: b.life / b.max });
      return true;
    });
    this.marks = this.marks.filter((m) => {
      m.life -= dt;
      if (m.life <= 0) return false;
      const a = Math.min(1, m.life);
      const pad = 4 + (1.4 - m.life) * 6;
      g.roundRect(m.x + pad, m.y + pad, CELL - pad * 2, CELL - pad * 2, 8).stroke({ width: 4, color: 0xffd36a, alpha: a });
      return true;
    });
  }
}
