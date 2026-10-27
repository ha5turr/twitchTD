// The King in Yellow (king.go): the scene before the final and his lines
// during it. The cat talks to the chat in his yellow cloak, turns into the
// grim cloaked figure and is gone; then the final wave starts. Art comes
// from assets/king/ and voice lines from assets/king/voice/<line>.ogg (or
// .mp3) when they are there; without them the scene draws its own stand-ins
// and plays in silence with the subtitles.
import { LEFT, TOP, COLS, ROWS, CELL, FONT_UI, nickColor } from './layout.js';
import { HERO_ART } from './sprites.js';
import { hasTileset, sprite } from './tileset.js';

const { Container, Graphics, Text, Sprite, Assets } = PIXI;

const FIELD_X = LEFT;
const FIELD_W = COLS * CELL;
const FIELD_H = ROWS * CELL;
const ART = ['king_cat_idle', 'king_cat_talk', 'king_cat_grin', 'king_cat_point', 'king_transform', 'king_dark'];
const LINE_SEC = 3.2; // a line stays this long without a voice to time it
// The King in Yellow's theme, from the end of his scene to the end of the
// final, dipping to MUSIC_LINE while he speaks in the fight.
const MUSIC = 'assets/king/music/lost_carcosa.ogg';
const MUSIC_LINE = 0.55;
// The king stands on the right, the player he talks about on the left.
const KING_X = FIELD_X + FIELD_W * 0.75;
const CARD_X = FIELD_X + FIELD_W * 0.05;

const plural = (n, one, few, many) => {
  const m10 = n % 10;
  const m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return one;
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return few;
  return many;
};

// lines picks what the cat says about this game (the design document's
// scene table), each with its voice file and the pose to show.
function introLines(d) {
  const out = [];
  out.push(d.players <= 3
    ? { text: 'Так вот кто держит эту таверну… Горстка смельчаков.', voice: 'king_hello_few', pose: 'king_cat_idle' }
    : { text: 'Сколько вас тут набилось… Вы правда думали, что я не замечу?', voice: 'king_hello_many', pose: 'king_cat_idle' });
  const n = d.towers ?? 0;
  const towers = `${n} ${plural(n, 'постройка', 'постройки', 'построек')}…`;
  if (n < 15) out.push({ text: `${towers} Так мало.`, voice: 'king_towers_few', pose: 'king_cat_grin' });
  else if (n <= 40) out.push({ text: `${towers} Как мило.`, voice: 'king_towers_some', pose: 'king_cat_grin' });
  else out.push({ text: `${towers} Вы и правда старались.`, voice: 'king_towers_many', pose: 'king_cat_grin' });
  // Players he singles out: the one who hit hardest, then a couple of the
  // others he has something to say about. The card beside him shows who
  // and for what, so the voice never has to say a name.
  const spots = Object.fromEntries((d.spots ?? []).map((s) => [s.kind, s]));
  if (spots.damage) out.push(spotLine(spots.damage));
  const extra = ['drinks', 'dances', 'repairs'].filter((k) => spots[k]).sort(() => Math.random() - 0.5).slice(0, 2);
  for (const k of extra) out.push(spotLine(spots[k]));
  if ((d.drinks ?? 0) >= 20 && !extra.includes('drinks')) {
    out.push({ text: `${d.drinks} ${plural(d.drinks, 'кружка', 'кружки', 'кружек')} эля. Вы встречаете короля пьяными?`, voice: 'king_drunk', pose: 'king_cat_grin' });
  } else {
    const w = d.waves ?? 0;
    out.push({ text: `${w} ${plural(w, 'волна', 'волны', 'волн')} за час. Неплохо… для смертных.`, voice: 'king_waves', pose: 'king_cat_idle' });
  }
  out.push({ text: 'А теперь… снимем маски.', voice: 'king_transform', pose: 'king_transform', transform: true });
  out.push({ text: 'Узрите Короля в Жёлтом.', voice: 'king_dark', pose: 'king_dark', dark: true });
  return out;
}

// spotLine is what the king says to a player he singles out, and the card
// that shows them.
function spotLine(s) {
  const n = s.value;
  switch (s.kind) {
    case 'damage':
      return { text: `${s.name}… Это ты бил больше всех? Я запомню твоё имя.`, voice: 'king_top', pose: 'king_cat_point', spot: s, stat: `урон за партию: ${n.toLocaleString('ru-RU')}` };
    case 'drinks':
      return { text: `Был у Боба ${n} ${plural(n, 'раз', 'раза', 'раз')}… ${s.name}. Теперь понятно по твоему лицу…`, voice: 'king_bob', pose: 'king_cat_grin', spot: s, stat: `у Боба: ${n} ${plural(n, 'кружка', 'кружки', 'кружек')}` };
    case 'dances':
      return { text: `Ты, ${s.name}, танцевал до упаду, пока остальные пахали… НЕ СИЛЬНО ЗАПАХАЛСЯ?`, voice: 'king_dance', pose: 'king_cat_grin', spot: s, stat: `танцев: ${n}` };
    default:
      return { text: `Кто реально запахался, так это ${s.name}: ${n} ${plural(n, 'починка', 'починки', 'починок')} баррикад?!`, voice: 'king_repair', pose: 'king_cat_point', spot: s, stat: `починок баррикад: ${n}` };
  }
}

// Lines during the fight, by the event that calls them.
const FIGHT = {
  phase2: { text: 'Мои слуги уже здесь.', voice: 'king_phase2' },
  song: { text: 'Слышите? Это песнь Кассильды.', voice: 'king_song' },
  unmask: { text: 'Маска? На мне нет маски.', voice: 'king_mask' },
  named: { text: 'Ты звал меня?', voice: 'king_name' },
  signOff: { text: 'Знак… стёрт. Ненадолго.', voice: 'king_sign_lost' },
  fall: { text: 'Это… ещё… не конец.', voice: 'king_fall' },
  rules: { text: 'Король правит. Таверна моя.', voice: 'king_rules' },
};

export class KingScene {
  constructor(sound, onDone) {
    this.sound = sound;
    this.root = new Container();
    this.root.visible = false;
    this.dark = new Graphics();
    this.dark.rect(FIELD_X, TOP, FIELD_W, FIELD_H).fill(0x000000);
    this.figure = new Container();
    this.figure.position.set(KING_X, TOP + FIELD_H);
    this.sub = new Text({
      text: '',
      style: {
        fontFamily: FONT_UI, fontSize: 38, fontWeight: '700', fill: 0xffe08a, align: 'center',
        wordWrap: true, wordWrapWidth: FIELD_W * 0.56, stroke: { color: 0x000000, width: 8 },
      },
    });
    this.sub.anchor.set(0.5, 1);
    this.sub.position.set(FIELD_X + FIELD_W * 0.33, TOP + FIELD_H - 60);
    // The card of the player he is talking about: their mercenary, nick and deed.
    this.card = new Container();
    this.card.position.set(CARD_X, TOP + FIELD_H * 0.3);
    this.card.visible = false;
    this.root.addChild(this.dark, this.figure, this.card, this.sub);
    // A line over the field during the fight, without the darkness.
    this.fightLine = new Text({ text: '', style: { ...this.sub.style, fontSize: 32 } });
    this.fightLine.anchor.set(0.5, 1);
    this.fightLine.position.set(FIELD_X + FIELD_W / 2, TOP + FIELD_H - 24);
    this.fightLine.alpha = 0;
    this.fightLeft = 0;
    this.layer = new Container(); // what main.js puts over the UI
    this.layer.addChild(this.root, this.fightLine);
    this.namedAt = -1e9;
    this.t = 0;
    this.art = {};
    this.buffers = {}; // line -> promise of its audio
    this.loaded = {}; // line -> its audio, once decoded
    this.onDone = onDone;
    for (const name of ART) {
      Assets.load(`assets/king/${name}.png`).then((tex) => (this.art[name] = tex)).catch(() => {});
    }
    this.scene = null;
  }

  // onEvent starts the scene or a fight line.
  onEvent(ev, finalOn) {
    switch (ev.kind) {
      case 'kingIntro': {
        let d = {};
        try {
          d = JSON.parse(ev.text);
        } catch {}
        this.start(introLines(d));
        break;
      }
      case 'kingSkip':
        this.stop();
        break;
      case 'kingPhase':
        if (ev.amount === 2) this.say(FIGHT.phase2);
        break;
      case 'song':
        this.say(FIGHT.song);
        break;
      case 'unmask':
        this.say(FIGHT.unmask);
        break;
      case 'named':
        if (this.t - this.namedAt > 20) {
          this.namedAt = this.t;
          this.say(FIGHT.named);
        }
        break;
      case 'signOff':
        this.say(FIGHT.signOff);
        break;
      case 'kill':
        // His last words when he falls, over whatever he was saying.
        if (ev.text === 'king') {
          this.stopVoice();
          this.say(FIGHT.fall, true);
        }
        break;
      case 'victory':
        this.endMusic(6); // the theme fades after his last words
        break;
      case 'gameOver':
        if (finalOn) {
          this.stopVoice();
          this.say(FIGHT.rules, true);
        }
        this.endMusic(6);
        break;
    }
  }

  // start plays the scene over secs: darkness first, then the lines.
  // start plays the scene: darkness first, then each line as long as its
  // recording lasts (or as it reads, without one). When it is over the
  // overlay tells the server, which lets the king in.
  start(lines) {
    this.scene = { lines, at: -1.5, index: -1, lineAt: 0, lineEnd: 0, loading: true };
    Promise.all(lines.map((l) => this.loadVoice(l.voice))).then(() => this.scene && (this.scene.loading = false));
    for (const l of Object.values(FIGHT)) this.loadVoice(l.voice); // the fight's lines are ready before he speaks them
    this.root.visible = true;
    this.dark.alpha = 0;
    this.sound?.ambientBus?.gain.setTargetAtTime(0.1, this.sound.ctx.currentTime, 0.5);
  }

  // startMusic begins his theme from the top.
  startMusic() {
    const s = this.sound;
    if (!s?.ctx) return;
    if (!this.music) {
      this.music = new Audio(MUSIC);
      this.music.loop = true;
      this.music.preload = 'auto';
      this.musicGain = s.ctx.createGain();
      this.musicGain.gain.value = 0;
      s.ctx.createMediaElementSource(this.music).connect(this.musicGain);
      this.musicGain.connect(s.master);
    }
    clearTimeout(this.musicStop);
    this.music.currentTime = 0;
    this.music.play().catch(() => {});
    this.musicOn = true;
    this.musicLevel(1, 1.5);
  }

  // keepMusic starts the theme at full level if the king is on the field
  // and it is not playing (the overlay was reloaded after his scene).
  keepMusic() {
    if (this.musicOn || this.scene) return;
    this.startMusic();
  }

  // musicLevel glides the theme to a level over about secs.
  musicLevel(level, secs) {
    if (!this.musicGain) return;
    const g = this.musicGain.gain;
    g.cancelScheduledValues(this.sound.ctx.currentTime);
    g.setTargetAtTime(level, this.sound.ctx.currentTime, Math.max(0.05, secs / 3));
  }

  // endMusic fades the theme out and stops it: the final is over, or a new map.
  endMusic(secs = 4) {
    if (!this.musicOn) return;
    this.musicOn = false;
    this.musicLevel(0, secs);
    clearTimeout(this.musicStop);
    this.musicStop = setTimeout(() => this.music?.pause(), secs * 1000 + 500);
    this.sound?.ambientBus?.gain.setTargetAtTime(0.7, this.sound.ctx.currentTime, 1);
  }

  stop() {
    const played = !!this.scene;
    this.scene = null;
    this.root.visible = false;
    this.figure.removeChildren();
    this.card.visible = false;
    this.sub.text = '';
    // The dialogue is over (or skipped): his theme starts, the ambience stays low under it.
    if (played) this.startMusic();
    this.sound?.ambientBus?.gain.setTargetAtTime(this.musicOn ? 0.3 : 0.7, this.sound.ctx.currentTime, 0.8);
  }

  say(line, force = false) {
    if (!force && (this.kingShare ?? 1) < 0.05) return; // he is about to fall: only his last words
    this.fightLine.text = line.text;
    this.fightLeft = LINE_SEC;
    this.fightLeft = Math.max(LINE_SEC, this.voice(line.voice) + 0.5); // the subtitle stays while he talks
    this.fightSec = this.fightLeft;
    if (this.musicOn && !this.scene) {
      // The theme dips under his voice and comes back.
      this.musicLevel(MUSIC_LINE, 0.4);
      clearTimeout(this.musicDip);
      this.musicDip = setTimeout(() => this.musicOn && !this.scene && this.musicLevel(1, 1.5), this.fightLeft * 1000);
    }
  }

  // loadVoice fetches a recorded line once; it resolves to the audio, or
  // null when there is no recording.
  loadVoice(name) {
    const s = this.sound;
    if (!s?.ctx || !name) return Promise.resolve(null);
    this.buffers[name] ??= (async () => {
      for (const ext of ['ogg', 'mp3']) {
        try {
          const r = await fetch(`assets/king/voice/${name}.${ext}`);
          if (!r.ok) continue;
          const buf = await s.ctx.decodeAudioData(await r.arrayBuffer());
          this.loaded[name] = buf;
          return buf;
        } catch {}
      }
      return null;
    })();
    return this.buffers[name];
  }

  // voice plays a recorded line now if it is loaded and returns its length
  // in seconds, 0 without one; a line not loaded yet is played when it is.
  voice(name) {
    const buf = this.loaded[name];
    if (!buf) {
      this.loadVoice(name).then((b) => b && this.fightLeft > 0 && this.play(b));
      return 0;
    }
    this.play(buf);
    return buf.duration;
  }

  play(buf) {
    const s = this.sound;
    const src = s.ctx.createBufferSource();
    src.buffer = buf;
    src.connect(s.master);
    src.start();
    this.speaking = src;
    src.onended = () => this.speaking === src && (this.speaking = null);
  }

  // stopVoice cuts off the line he is saying.
  stopVoice() {
    try {
      this.speaking?.stop();
    } catch {}
    this.speaking = null;
  }

  // update follows the king's health: near death his powers say nothing,
  // so his last words are not lost under a spell's line.
  update(s) {
    const k = s.enemies.find((e) => e.kind === 'king');
    this.kingShare = k ? k.hp / k.maxHp : 1;
  }

  // pose shows the cat or the dark form: the art when it is loaded, a
  // stand-in drawn here otherwise.
  pose(line) {
    this.figure.removeChildren();
    this.figure.alpha = 1;
    // A cat pose not drawn yet is the plain cat (the transformation too:
    // the frame darkens it), so two pictures are enough for the scene.
    const cat = line.pose.startsWith('king_cat') || line.transform;
    let tex = this.art[line.pose];
    if (!tex && cat) tex = this.art.king_cat_idle;
    this.figure.talks = tex && tex === this.art.king_cat_idle; // its mouth can move
    if (tex) {
      const s = new Sprite(tex);
      s.anchor.set(0.5, 1);
      s.scale.set(Math.min(1, (FIELD_H * 0.92) / tex.height));
      this.figure.addChild(s);
      this.figure.sprite = s;
      return;
    }
    this.figure.sprite = null;
    const g = new Graphics();
    if (line.dark) {
      // A tall figure in tattered yellow with a pale blank mask.
      g.poly([-120, 0, -90, -560, 0, -640, 90, -560, 120, 0, 80, -30, 40, 0, 0, -40, -40, 0, -80, -30]).fill(0x8a7420);
      g.ellipse(0, -520, 46, 58).fill(0xe8e0c8);
      for (let i = -2; i <= 2; i++) g.moveTo(i * 40, -10).quadraticCurveTo(i * 60, 40, i * 50 + 20, 80).stroke({ width: 8, color: 0x111111 });
    } else {
      // The cat in his cloak: a yellow hood and robe, ears, the black cloth over his eyes.
      g.poly([-150, 0, -110, -420, 110, -420, 150, 0]).fill(0xd8c020);
      g.circle(0, -480, 95).fill(0xd8c020);
      g.poly([-80, -540, -60, -620, -20, -560]).poly([80, -540, 60, -620, 20, -560]).fill(0xc87a3a);
      g.ellipse(0, -470, 70, 64).fill(0xd8884a);
      g.roundRect(-70, -530, 140, 60, 26).fill(0x111111);
      g.circle(-28, -500, 7).circle(28, -500, 7).fill(0xffe08a);
      g.ellipse(0, -430, 30, 18).fill(0xf4e8dc);
    }
    this.figure.addChild(g);
  }

  // showCard puts up (or takes down) the singled-out player's card.
  showCard(line) {
    this.card.removeChildren().forEach((c) => c.destroy({ children: true }));
    this.card.visible = !!line.spot;
    if (!line.spot) return;
    const w = 520;
    const h = 210;
    const bg = new Graphics();
    bg.roundRect(0, 0, w, h, 16).fill({ color: 0x14110a, alpha: 0.92 }).stroke({ width: 4, color: 0xd8b020 });
    bg.roundRect(18, 18, 174, 174, 12).fill({ color: 0x2a2414 }).stroke({ width: 2, color: 0x6a5a28 });
    this.card.addChild(bg);
    const hero = hasTileset() && HERO_ART[line.spot.class] && sprite(`${HERO_ART[line.spot.class]}_idle`, 5, 0.12);
    if (hero) {
      hero.position.set(105, 178);
      this.card.addChild(hero);
    }
    const name = new Text({
      text: line.spot.name,
      style: { fontFamily: FONT_UI, fontSize: 40, fontWeight: '700', fill: nickColor(line.spot.name), stroke: { color: 0x000000, width: 6 }, wordWrap: true, wordWrapWidth: w - 230 },
    });
    name.position.set(212, 34);
    const stat = new Text({
      text: line.stat,
      style: { fontFamily: FONT_UI, fontSize: 28, fontWeight: '700', fill: 0xffe08a, stroke: { color: 0x000000, width: 5 }, wordWrap: true, wordWrapWidth: w - 230 },
    });
    stat.position.set(212, 34 + name.height + 14);
    this.card.addChild(name, stat);
    this.card.alpha = 0;
  }

  frame(dt) {
    this.t += dt;
    if (this.card.visible) this.card.alpha = Math.min(1, this.card.alpha + dt * 4);
    if (this.fightLeft > 0) {
      this.fightLeft -= dt;
      this.fightLine.alpha = Math.min(1, this.fightLeft * 2, ((this.fightSec ?? LINE_SEC) - this.fightLeft) * 4);
    } else {
      this.fightLine.alpha = 0;
    }
    const sc = this.scene;
    if (!sc) return;
    // The darkness waits for the recordings, three seconds at most.
    if (!(sc.at >= 0 && sc.loading && sc.at < 3)) sc.at += dt;
    this.dark.alpha = Math.min(0.78, ((sc.at + 1.5) / 1.5) * 0.78);
    if (sc.at < 0 || (sc.index < 0 && sc.loading && sc.at < 3)) return;
    if (sc.index < 0 || sc.at >= sc.lineEnd) {
      sc.index++;
      if (sc.index >= sc.lines.length) {
        sc.endAt ??= sc.at;
      } else {
        const line = sc.lines[sc.index];
        this.pose(line);
        this.sub.text = line.text;
        this.showCard(line);
        const said = this.voice(line.voice);
        // Without a recording the mouth moves for about as long as the line reads.
        this.talking = said || 0.4 + line.text.length * 0.06;
        sc.lineAt = sc.at;
        sc.lineEnd = sc.at + (said ? said + 0.45 : Math.max(2.6, this.talking + 0.6));
      }
    }
    if (sc.endAt !== undefined) {
      sc.index = sc.lines.length;
      this.figure.alpha = Math.max(0, 1 - (sc.at - sc.endAt) / 0.8); // the dark form fades out
      if (this.figure.alpha <= 0) {
        this.stop();
        this.onDone?.();
      }
      return;
    }
    const line = sc.lines[sc.index];
    const k = sc.at - sc.lineAt;
    // The mouth: with both cat poses loaded it opens and closes while he talks.
    const s = this.figure.sprite;
    if (s && this.figure.talks && this.art.king_cat_talk) {
      const talking = k < this.talking;
      s.texture = talking && Math.floor(this.t * 8) % 2 ? this.art.king_cat_talk : this.art.king_cat_idle;
    }
    // A slow breath, and the transformation shudders.
    this.figure.scale.set(1 + 0.01 * Math.sin(this.t * 2));
    this.figure.x = KING_X + (line.transform ? Math.sin(this.t * 40) * 4 * Math.min(1, k) : 0);
    this.figure.tint = line.transform ? 0xffffff - Math.floor(Math.min(1, k / (sc.lineEnd - sc.lineAt)) * 0x60) * 0x10101 : 0xffffff;
  }
}
