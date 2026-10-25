// Speech bubbles for !s lines: HTML over the canvas rather than Pixi text, so
// a line sits on a readable backing and its emotes, animated GIF and WebP
// included, play as the browser draws them. Owners place their bubble in
// canvas pixels every frame; the layer scales with the canvas.
import { W, H } from './layout.js';
import { emoteURL } from './emotes.js';

const MAX_EMOTES = 8; // pictures per line

export class Bubbles {
  constructor(canvas) {
    this.canvas = canvas;
    this.layer = document.createElement('div');
    Object.assign(this.layer.style, { position: 'fixed', left: '0', top: '0', width: `${W}px`, height: `${H}px`, transformOrigin: '0 0', pointerEvents: 'none', overflow: 'hidden' });
    document.body.appendChild(this.layer);
    const style = document.createElement('style');
    style.textContent = `
      .say { position: absolute; transform: translate(-50%, -100%); white-space: nowrap;
        font: 700 16px Philosopher, sans-serif; color: #fff1c2; line-height: 28px;
        background: rgba(22, 14, 8, 0.86); border: 2px solid #c9a86a; border-radius: 10px;
        padding: 2px 10px; box-shadow: 0 2px 6px rgba(0, 0, 0, 0.6); }
      .say::after { content: ''; position: absolute; left: 50%; bottom: -8px; margin-left: -7px;
        border: 7px solid transparent; border-top-color: #c9a86a; border-bottom: 0; }
      .say img { height: 28px; vertical-align: middle; margin: 0 1px; }`;
    document.head.appendChild(style);
    this.items = new Map(); // id -> { el, left }
    this.fit();
    window.addEventListener('resize', () => this.fit());
  }

  // fit lays the layer over the canvas as CSS shows it.
  fit() {
    const r = this.canvas.getBoundingClientRect();
    this.layer.style.transform = `translate(${r.left}px, ${r.top}px) scale(${r.width / W})`;
  }

  // show puts a line up under id for secs, replacing the one there.
  show(id, text, emotes, secs = 5) {
    this.hide(id);
    const el = document.createElement('div');
    el.className = 'say';
    let pics = 0;
    text.split(' ').forEach((word, i) => {
      if (i > 0) el.appendChild(document.createTextNode(' '));
      const url = pics < MAX_EMOTES && emoteURL(word, emotes);
      if (url) {
        const img = document.createElement('img');
        img.src = url;
        img.alt = word;
        el.appendChild(img);
        pics++;
      } else {
        el.appendChild(document.createTextNode(word)); // text, never markup
      }
    });
    this.layer.appendChild(el);
    this.items.set(id, { el, left: secs });
  }

  // place moves id's bubble so its tail points at x, y (canvas pixels),
  // kept inside [minX, maxX] when given. It reports whether one is up.
  place(id, x, y, minX = 0, maxX = W) {
    const b = this.items.get(id);
    if (!b) return false;
    const half = b.el.offsetWidth / 2 + 4;
    b.el.style.left = `${Math.min(Math.max(x, minX + half), maxX - half)}px`;
    b.el.style.top = `${y}px`;
    return true;
  }

  hide(id) {
    this.items.get(id)?.el.remove();
    this.items.delete(id);
  }

  frame(dt) {
    this.fit();
    for (const [id, b] of this.items) {
      b.left -= dt;
      if (b.left <= 0) this.hide(id);
      else b.el.style.opacity = String(Math.min(1, b.left / 0.4)); // a soft fade at the end
    }
  }
}
