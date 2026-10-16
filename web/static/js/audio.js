// Sound effects synthesised with WebAudio, so the overlay needs no audio
// files. OBS plays browser sources without a click; a normal browser tab
// starts the sound on the first click anywhere.

const AMBIENT = {
  ruins: { base: 55, cutoff: 260, wind: 0.05 },
  dungeon: { base: 49, cutoff: 200, wind: 0.02 },
  crypt: { base: 46.25, cutoff: 180, wind: 0.03 },
  ice: { base: 61.7, cutoff: 420, wind: 0.07 },
};

export class Sound {
  constructor() {
    this.ctx = new (window.AudioContext || window.webkitAudioContext)();
    this.master = this.ctx.createGain();
    this.master.connect(this.ctx.destination);
    this.volume = 0.6;
    this.muted = false;
    this.ambientOn = true;
    this.last = {};
    this.voices = 0;
    this.noiseBuf = this.makeNoise();
    this.applyVolume();
    const unlock = () => this.ctx.resume();
    window.addEventListener('pointerdown', unlock);
    window.addEventListener('keydown', unlock);
  }

  configure({ volume, muted, ambient }) {
    if (typeof volume === 'number') this.volume = volume;
    if (typeof muted === 'boolean') this.muted = muted;
    if (typeof ambient === 'boolean') this.ambientOn = ambient;
    this.applyVolume();
    if (!this.ambientOn) this.stopAmbient();
    else if (this.biome) this.setBiome(this.biome);
  }

  applyVolume() {
    this.master.gain.setTargetAtTime(this.muted ? 0 : this.volume, this.ctx.currentTime, 0.05);
  }

  makeNoise() {
    const len = this.ctx.sampleRate;
    const buf = this.ctx.createBuffer(1, len, this.ctx.sampleRate);
    const d = buf.getChannelData(0);
    for (let i = 0; i < len; i++) d[i] = Math.random() * 2 - 1;
    return buf;
  }

  // gate keeps one sound from piling up when many things happen at once.
  gate(name, gap) {
    const now = this.ctx.currentTime;
    if (this.ctx.state !== 'running' || this.voices > 24) return false;
    if (now - (this.last[name] ?? -1) < gap) return false;
    this.last[name] = now;
    return true;
  }

  env(gainNode, t, attack, decay, peak) {
    const g = gainNode.gain;
    g.setValueAtTime(0.0001, t);
    g.exponentialRampToValueAtTime(peak, t + attack);
    g.exponentialRampToValueAtTime(0.0001, t + attack + decay);
  }

  tone(type, freq, { at = 0, attack = 0.005, decay = 0.2, peak = 0.3, to, filter } = {}) {
    const t = this.ctx.currentTime + at;
    const o = this.ctx.createOscillator();
    const g = this.ctx.createGain();
    o.type = type;
    o.frequency.setValueAtTime(freq, t);
    if (to) o.frequency.exponentialRampToValueAtTime(to, t + attack + decay);
    let node = o;
    if (filter) {
      const f = this.ctx.createBiquadFilter();
      f.type = 'lowpass';
      f.frequency.value = filter;
      o.connect(f);
      node = f;
    }
    node.connect(g).connect(this.master);
    this.env(g, t, attack, decay, peak);
    this.voices++;
    o.onended = () => this.voices--;
    o.start(t);
    o.stop(t + attack + decay + 0.05);
  }

  noise({ at = 0, attack = 0.003, decay = 0.1, peak = 0.3, type = 'bandpass', freq = 1000, q = 1 } = {}) {
    const t = this.ctx.currentTime + at;
    const src = this.ctx.createBufferSource();
    src.buffer = this.noiseBuf;
    const f = this.ctx.createBiquadFilter();
    f.type = type;
    f.frequency.value = freq;
    f.Q.value = q;
    const g = this.ctx.createGain();
    src.connect(f).connect(g).connect(this.master);
    this.env(g, t, attack, decay, peak);
    this.voices++;
    src.onended = () => this.voices--;
    src.start(t, Math.random() * 0.5);
    src.stop(t + attack + decay + 0.05);
  }

  play(name) {
    switch (name) {
      case 'arrow':
        if (this.gate(name, 0.07)) this.noise({ decay: 0.06, peak: 0.12, freq: 2400, q: 3 });
        break;
      case 'magic':
        if (this.gate(name, 0.1)) this.tone('sine', 700, { decay: 0.16, peak: 0.08, to: 1500 });
        break;
      case 'fire':
        if (this.gate(name, 0.12)) this.noise({ attack: 0.02, decay: 0.25, peak: 0.14, type: 'lowpass', freq: 900 });
        break;
      case 'blast':
        if (this.gate(name, 0.15)) {
          this.noise({ attack: 0.005, decay: 0.4, peak: 0.25, type: 'lowpass', freq: 380 });
          this.tone('sine', 90, { decay: 0.3, peak: 0.2, to: 45 });
        }
        break;
      case 'kill':
        if (this.gate(name, 0.06)) {
          this.tone('triangle', 160, { decay: 0.12, peak: 0.12, to: 70 });
          this.noise({ decay: 0.05, peak: 0.08, freq: 3000, q: 2 });
        }
        break;
      case 'coin':
        if (this.gate(name, 0.09)) {
          this.tone('triangle', 1760, { decay: 0.07, peak: 0.06 });
          this.tone('triangle', 2350, { at: 0.06, decay: 0.12, peak: 0.06 });
        }
        break;
      case 'build':
        if (this.gate(name, 0.15)) {
          for (let i = 0; i < 3; i++) {
            this.noise({ at: i * 0.13, decay: 0.05, peak: 0.16, freq: 1400, q: 4 });
            this.tone('square', 220, { at: i * 0.13, decay: 0.04, peak: 0.04, filter: 900 });
          }
        }
        break;
      case 'trap':
        if (this.gate(name, 0.1)) {
          this.tone('square', 1300, { decay: 0.03, peak: 0.07, filter: 3000 });
          this.noise({ at: 0.02, decay: 0.08, peak: 0.12, freq: 4000, q: 6 });
        }
        break;
      case 'wallHit':
        if (this.gate(name, 0.35)) this.noise({ decay: 0.18, peak: 0.2, type: 'lowpass', freq: 220 });
        break;
      case 'stun':
        if (this.gate(name, 0.3)) this.tone('sine', 520, { decay: 0.2, peak: 0.12, to: 220 });
        break;
      case 'drink':
        if (this.gate(name, 0.2)) {
          for (let i = 0; i < 3; i++) this.tone('sine', 320, { at: i * 0.16, decay: 0.12, peak: 0.1, to: 160 });
        }
        break;
      case 'bell': // a wave begins
        if (this.gate(name, 1)) {
          for (const [f, p] of [[523, 0.16], [784, 0.09], [1046, 0.06], [1318, 0.03]]) this.tone('sine', f, { decay: 2.2, peak: p });
        }
        break;
      case 'horn': // a boss arrives
        if (this.gate(name, 2)) {
          this.tone('sawtooth', 98, { attack: 0.35, decay: 1.6, peak: 0.16, to: 92, filter: 700 });
          this.tone('sawtooth', 147, { attack: 0.4, decay: 1.5, peak: 0.1, to: 140, filter: 700 });
        }
        break;
      case 'ability':
        if (this.gate(name, 0.6)) this.tone('sawtooth', 220, { attack: 0.05, decay: 0.5, peak: 0.12, to: 110, filter: 600 });
        break;
      case 'cleared':
        if (this.gate(name, 1)) [392, 494, 587].forEach((f, i) => this.tone('triangle', f, { at: i * 0.12, decay: 0.5, peak: 0.1 }));
        break;
      case 'fanfare': // donations and support
        if (this.gate(name, 0.8)) [523, 659, 784, 1046].forEach((f, i) => this.tone('square', f, { at: i * 0.11, decay: 0.35, peak: 0.07, filter: 2500 }));
        break;
      case 'strike':
        if (this.gate(name, 1)) for (let i = 0; i < 10; i++) this.noise({ at: i * 0.05, decay: 0.05, peak: 0.1, freq: 2600, q: 3 });
        break;
      case 'gameover':
        if (this.gate(name, 3)) for (let i = 0; i < 3; i++) this.tone('sine', 110, { at: i * 1.1, decay: 2.5, peak: 0.22 });
        break;
      case 'newgame':
        if (this.gate(name, 2)) [262, 330, 392].forEach((f) => this.tone('triangle', f, { attack: 0.05, decay: 1.2, peak: 0.08 }));
        break;
    }
  }

  // setBiome starts a quiet drone with a little wind in the biome's key.
  setBiome(biome) {
    this.biome = biome;
    this.stopAmbient();
    if (!this.ambientOn) return;
    const a = AMBIENT[biome] ?? AMBIENT.ruins;
    const ctx = this.ctx;
    const out = ctx.createGain();
    out.gain.value = 0;
    out.gain.setTargetAtTime(1, ctx.currentTime, 2);
    out.connect(this.master);
    const lp = ctx.createBiquadFilter();
    lp.type = 'lowpass';
    lp.frequency.value = a.cutoff;
    lp.connect(out);
    const nodes = [out];
    for (const [mult, detune] of [[1, -6], [1.5, 4], [2, 7]]) {
      const o = ctx.createOscillator();
      o.type = 'sawtooth';
      o.frequency.value = a.base * mult;
      o.detune.value = detune;
      const g = ctx.createGain();
      g.gain.value = 0.018;
      o.connect(g).connect(lp);
      o.start();
      nodes.push(o);
    }
    const wind = ctx.createBufferSource();
    wind.buffer = this.noiseBuf;
    wind.loop = true;
    const bp = ctx.createBiquadFilter();
    bp.type = 'bandpass';
    bp.frequency.value = 500;
    bp.Q.value = 0.7;
    const wg = ctx.createGain();
    wg.gain.value = a.wind;
    const lfo = ctx.createOscillator();
    lfo.frequency.value = 0.07;
    const lfoGain = ctx.createGain();
    lfoGain.gain.value = 300;
    lfo.connect(lfoGain).connect(bp.frequency);
    wind.connect(bp).connect(wg).connect(out);
    wind.start();
    lfo.start();
    nodes.push(wind, lfo);
    this.ambient = nodes;
  }

  stopAmbient() {
    if (!this.ambient) return;
    const [out, ...srcs] = this.ambient;
    out.gain.setTargetAtTime(0, this.ctx.currentTime, 0.5);
    setTimeout(() => srcs.forEach((s) => s.stop()), 2500);
    this.ambient = null;
  }
}

// EVENT_SOUNDS maps game events to sounds.
export const EVENT_SOUNDS = {
  kill: 'kill', build: 'coin', upgrade: 'coin', sell: 'coin', built: 'build', upgraded: 'build',
  trap: 'trap', blast: 'blast', stun: 'stun', drink: 'drink', waveCleared: 'cleared',
  donation: 'fanfare', support: 'fanfare', reward: 'fanfare', strike: 'strike',
  gameOver: 'gameover', newGame: 'newgame', ability: 'ability', wallBroken: 'blast', disabled: 'ability',
};
