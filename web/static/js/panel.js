// Streamer panel for an OBS Custom Dock: the same game seen from above, with
// controls. Everything goes through the game WebSocket as the streamer.
import { connect } from './net.js';

const $ = (id) => document.getElementById(id);
const COLS = 16;
const ROWS = 10;
const CELL = 96;

const TOOLS = [
  ['arrow', 'Лучники', '#c8a165'],
  ['magic', 'Обелиск', '#9b6bff'],
  ['fire', 'Жаровня', '#ff6a1a'],
  ['shrine', 'Святилище', '#ffe08a'],
  ['trap', 'Капканы', '#9aa0a8'],
  ['drums', 'Барабаны', '#8a3a2a'],
  ['wall', 'Баррикада', '#7a5230'],
  ['clown', 'Клоун 🤡', '#e04aa8'],
  ['remove', 'Снести', '#d0574a'],
];
const BOSSES = [
  ['bone_knight', 'Костяной Рыцарь'], ['cult_leader', 'Вожак Культа'], ['spider_queen', 'Королева Пауков'],
  ['cave_troll', 'Пещерный Тролль'], ['lich', 'Лич'], ['ghost_king', 'Призрачный Король'],
  ['ice_golem', 'Ледяной Голем'], ['frost_demon', 'Демон Стужи'], ['sdf', 'ЭссДиЭф'],
];
const CLASSES = { ranger: 'Следопыт', mage: 'Маг', pyro: 'Пиромант', cleric: 'Жрец', rogue: 'Разбойник', bard: 'Бард', warrior: 'Воитель', druid: 'Друид' };
const SETTINGS = [
  ['towerLimit', 'Лимит построек', 'int'], ['wallRepairCdSec', 'Кд починки баррикады, с', 'num'], ['walletCap', 'Потолок кошелька', 'int'], ['startGold', 'Стартовое золото', 'int'],
  ['keepGold', 'Хранить золото между играми', 'bool'], ['chatGold', 'Золото за сообщение', 'int'], ['bitsGold', 'Золото за бит', 'num'],
  ['subGold', 'Золото за подписку', 'int'], ['donationGoldPerRub', 'Золото за 1 ₽', 'num'], ['buildPhaseSec', 'Фаза стройки, с', 'num'],
  ['bossEvery', 'Босс каждые N волн', 'int'], ['sdfChance', 'Шанс ЭссДиЭфа на волне босса (0–1)', 'num'], ['sdfFromWave', 'ЭссДиЭф не раньше волны', 'int'],['baseHP', 'Базовое ХП врагов', 'num'], ['hpGrowth', 'Рост ХП за волну', 'num'],
  ['chatterHPCoef', 'ХП от числа чатерсов', 'num'], ['webcamFrom', 'Вебка: от клетки', 'text'], ['webcamTo', 'Вебка: до клетки', 'text'],
];

let tool = 'arrow';
let map = null;
let state = null;
let hover = null;
let values = {};

const net = connect({
  map(m) { map = m; draw(); },
  state(s) { state = s; renderStatus(); renderChatters(); draw(); },
  settings(s) { values = s.values; renderSettings(); },
  settingsError(e) {
    message(e.error, true);
    document.activeElement?.blur();
    renderSettings(); // put the values the game kept back into the form
  },
});

const say = (text) => net.send({ t: 'chat', user: 'streamer', mod: true, text });
const cellName = (x, y) => String.fromCharCode(65 + x) + (ROWS - y);

// Tools.
for (const [key, label, color] of TOOLS) {
  const b = document.createElement('button');
  b.textContent = label;
  b.dataset.tool = key;
  b.style.borderLeft = `4px solid ${color}`;
  b.onclick = () => selectTool(key);
  $('tools').appendChild(b);
}
function selectTool(key) {
  tool = key;
  document.querySelectorAll('[data-tool]').forEach((b) => b.classList.toggle('on', b.dataset.tool === key));
}
selectTool('arrow');

for (const [key, label] of BOSSES) $('spawn').add(new Option(label, key));
document.querySelectorAll('[data-cmd]').forEach((b) => (b.onclick = () => say(b.dataset.cmd)));
$('newmap').onclick = () => {
  if (confirm('Начать новую игру на новой карте? Текущая партия закончится.')) say(`!tdnew ${$('biome').value}`.trim());
};
$('spawnBtn').onclick = () => say(`!tdspawn ${$('spawn').value}`);
$('setWalls').onclick = () => say(`!tdwalls ${$('wallsHp').value}`);
$('give').onclick = () => $('tNick').value && say(`!tdtreasury ${$('tNick').value} ${$('tAmount').value}`);

// Map.
const canvas = $('map');
const ctx = canvas.getContext('2d');
const scale = 1536 / COLS / CELL; // canvas units per game pixel, here 1
function cellAt(ev) {
  const r = canvas.getBoundingClientRect();
  const x = Math.floor(((ev.clientX - r.left) / r.width) * COLS);
  const y = Math.floor(((ev.clientY - r.top) / r.height) * ROWS);
  return x >= 0 && x < COLS && y >= 0 && y < ROWS ? { x, y } : null;
}
canvas.addEventListener('mousemove', (ev) => {
  hover = cellAt(ev);
  const t = hover && state?.towers.find((t) => t.x === hover.x && t.y === hover.y);
  $('hover').textContent = hover ? `${cellName(hover.x, hover.y)}${t ? ` · ${t.kind} ур. ${t.lvl} · ${t.owner}` : ''}` : '';
  draw();
});
canvas.addEventListener('mouseleave', () => { hover = null; $('hover').textContent = ''; draw(); });
canvas.addEventListener('click', (ev) => {
  const c = cellAt(ev);
  if (!c) return;
  const name = cellName(c.x, c.y);
  say(tool === 'remove' ? `!tdremove ${name}` : `!build ${name} ${tool}`);
});

const COLORS = { '.': '#2a2a26', '#': '#5a5040', o: '#151412', w: '#1a2230' };
const TOWER_COLORS = Object.fromEntries(TOOLS.map(([k, , c]) => [k, c]));
const RARITY = ['#e8e8e8', '#4cd964', '#3d8bff', '#b05cff', '#ff9a2e'];

function draw() {
  if (!map) return;
  const s = CELL * scale;
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  for (let y = 0; y < ROWS; y++) {
    for (let x = 0; x < COLS; x++) {
      ctx.fillStyle = COLORS[map.cells[y][x]] ?? '#222';
      ctx.fillRect(x * s, y * s, s - 2, s - 2);
    }
  }
  // The canvas is 1536 px wide but shown at dock width, so labels are big.
  ctx.font = 'bold 40px system-ui';
  ctx.fillStyle = 'rgba(236,228,212,.6)';
  for (let x = 0; x < COLS; x++) ctx.fillText(String.fromCharCode(65 + x), x * s + 8, 40);
  for (let y = 0; y < ROWS; y++) ctx.fillText(String(ROWS - y), 8, y * s + s - 10);
  if (state) {
    for (const t of state.towers) {
      const px = t.x * s;
      const py = t.y * s;
      ctx.globalAlpha = t.built ? 1 : 0.45;
      ctx.fillStyle = TOWER_COLORS[t.kind] ?? '#888';
      ctx.fillRect(px + 14, py + 14, s - 30, s - 30);
      ctx.lineWidth = 6;
      ctx.strokeStyle = t.disabled ? '#777' : RARITY[Math.min(t.lvl, 5) - 1];
      ctx.strokeRect(px + 14, py + 14, s - 30, s - 30);
      ctx.globalAlpha = 1;
    }
    for (const e of state.enemies) {
      ctx.fillStyle = e.boss ? '#ff3b3b' : '#e6dcc8';
      ctx.beginPath();
      ctx.arc(e.x * s, e.y * s, e.boss ? 22 : 10, 0, Math.PI * 2);
      ctx.fill();
    }
    for (const w of state.workers ?? []) {
      ctx.fillStyle = '#ffd27a';
      ctx.beginPath();
      ctx.arc(w.x * s, w.y * s, 8, 0, Math.PI * 2);
      ctx.fill();
    }
  }
  if (hover) {
    ctx.lineWidth = 4;
    ctx.strokeStyle = tool === 'remove' ? '#d0574a' : '#d9a441';
    ctx.strokeRect(hover.x * s + 2, hover.y * s + 2, s - 6, s - 6);
  }
}

// Status.
const PHASE = { build: 'стройка', wave: 'идёт волна', gameover: 'таверна пала' };
function renderStatus() {
  const s = state;
  $('wave').textContent = s.wave ? `Волна ${s.wave}` : 'Подготовка';
  $('phase').textContent = s.paused ? 'пауза' : `${PHASE[s.phase] ?? s.phase}${s.phase !== 'wave' ? ` · ${Math.ceil(s.timer)} с` : ''}`;
  $('pause').classList.toggle('on', s.paused);
  const f = Math.max(0, s.walls / s.wallsMax);
  $('walls').firstElementChild.style.width = `${f * 100}%`;
  $('walls').firstElementChild.style.background = f > 0.5 ? 'var(--ok)' : f > 0.25 ? 'var(--accent)' : 'var(--danger)';
  $('wallsText').textContent = `стены ${Math.ceil(s.walls)} / ${s.wallsMax} · наёмников ${s.active}`;
  $('treasury').textContent = s.treasury ?? 0;
}

// Chatters: rebuilt at most twice a second, the table is cheap but not free.
let lastTable = 0;
function renderChatters() {
  const now = performance.now();
  if (now - lastTable < 500) return;
  lastTable = now;
  const rows = [...state.chatters].sort((a, b) => b.gold - a.gold);
  const body = $('chatters');
  body.replaceChildren(...rows.map((c) => {
    const tr = document.createElement('tr');
    const cell = (text, cls) => { const td = document.createElement('td'); td.textContent = text; if (cls) td.className = cls; return td; };
    tr.append(cell(c.name), cell(CLASSES[c.class] ?? ''), cell(c.gold, 'num'), cell(c.souls ?? 0, 'num'), cell(c.towers, 'num'), cell(c.kills, 'num'));
    const act = document.createElement('td');
    const gold = document.createElement('button');
    gold.textContent = '+100';
    gold.title = 'Выдать 100 золота';
    gold.onclick = () => say(`!tdgold ${c.name} 100`);
    const ban = document.createElement('button');
    ban.textContent = c.banned ? 'Разрешить' : 'Запретить стройку';
    ban.className = c.banned ? 'on' : '';
    ban.onclick = () => say(`${c.banned ? '!tdunban' : '!tdban'} ${c.name}`);
    act.append(gold, ' ', ban);
    tr.append(act);
    return tr;
  }));
  renderMemes();
}

// Memecoin requests: the streamer credits them in MemeAlerts by hand.
let memesKey = null;
function renderMemes() {
  const list = state.memes ?? [];
  const key = list.map((r) => r.id).join(',');
  if (key === memesKey) return; // keep buttons stable between updates
  memesKey = key;
  const cell = (text, cls) => { const td = document.createElement('td'); td.textContent = text; if (cls) td.className = cls; return td; };
  const rows = list.map((r) => {
    const tr = document.createElement('tr');
    tr.append(cell(`#${r.id}`), cell(r.name), cell(r.coins, 'num'), cell(r.souls, 'num'));
    const act = document.createElement('td');
    const done = document.createElement('button');
    done.textContent = 'Выдано';
    done.className = 'on';
    done.onclick = () => say(`!tdmeme done ${r.id}`);
    const reject = document.createElement('button');
    reject.textContent = 'Отказать';
    reject.onclick = () => say(`!tdmeme reject ${r.id}`);
    act.append(done, ' ', reject);
    tr.append(act);
    return tr;
  });
  if (!rows.length) {
    const tr = document.createElement('tr');
    const td = cell('заявок нет', 'muted');
    td.colSpan = 5;
    tr.append(td);
    rows.push(tr);
  }
  $('memes').replaceChildren(...rows);
}

// Settings.
let built = false;
function renderSettings() {
  const form = $('settings');
  if (!built) {
    for (const [key, label, type] of SETTINGS) {
      const l = document.createElement('label');
      const input = document.createElement('input');
      input.name = key;
      input.type = type === 'bool' ? 'checkbox' : type === 'text' ? 'text' : 'number';
      if (type === 'num') input.step = 'any';
      if (type === 'text') input.size = 4;
      l.append(label, input);
      form.append(l);
    }
    built = true;
  }
  for (const [key, , type] of SETTINGS) {
    const input = form.elements[key];
    if (document.activeElement === input) continue; // do not fight the streamer's typing
    if (type === 'bool') input.checked = !!values[key];
    else input.value = values[key] ?? '';
  }
  renderMixer();
}

$('save').onclick = () => {
  const form = $('settings');
  const out = {};
  for (const [key, , type] of SETTINGS) {
    const input = form.elements[key];
    const v = type === 'bool' ? input.checked : type === 'text' ? input.value.trim().toUpperCase() : Number(input.value);
    if (v !== values[key]) out[key] = v;
  }
  if (!Object.keys(out).length) return message('нет изменений');
  net.send({ t: 'settings', values: out });
  message('применено');
};
// Sound mixer: master, ambience and one row per sound group. Moving a slider
// shows the level at once and applies it on release; ▶ plays a sample in
// the overlay so the streamer hears it through OBS.
const GROUPS = [
  ['combat', 'Бой'], ['building', 'Стройка'], ['alerts', 'Волны и боссы'],
  ['support', 'Донаты и награды'], ['tavern', 'Таверна'], ['walls', 'Стены и наёмники'],
];
const pctText = (v) => `${Math.round((v ?? 0) * 100)}%`;
const sendSettings = (v) => net.send({ t: 'settings', values: v });

for (const [key, label] of GROUPS) {
  const row = document.createElement('div');
  row.className = 'mix';
  row.dataset.group = key;
  const name = document.createElement('span');
  name.textContent = label;
  const range = document.createElement('input');
  range.type = 'range';
  range.min = 0;
  range.max = 1;
  range.step = 0.05;
  range.setAttribute('aria-label', `Громкость: ${label}`);
  const out = document.createElement('output');
  const test = document.createElement('button');
  test.textContent = '▶';
  test.dataset.test = key;
  test.title = `Прослушать: ${label}`;
  range.oninput = () => { out.textContent = pctText(Number(range.value)); row.classList.toggle('off', Number(range.value) === 0); };
  range.onchange = () => {
    // Send the whole map: an override replaces the setting as a whole.
    sendSettings({ soundGroups: { ...(values.soundGroups ?? {}), [key]: Number(range.value) } });
  };
  row.append(name, range, out, test);
  $('groups').append(row);
}

document.querySelectorAll('[data-test]').forEach((b) => (b.onclick = () => net.send({ t: 'soundTest', text: b.dataset.test })));
$('volume').oninput = () => ($('volumeOut').textContent = pctText(Number($('volume').value)));
$('volume').onchange = () => sendSettings({ volume: Number($('volume').value) });
$('muted').onclick = () => sendSettings({ muted: !values.muted });
$('ambient').onchange = () => sendSettings({ ambient: $('ambient').checked });
$('ambientVolume').oninput = () => ($('ambientVolumeOut').textContent = pctText(Number($('ambientVolume').value)));
$('ambientVolume').onchange = () => sendSettings({ ambientVolume: Number($('ambientVolume').value), ambient: true });

function renderMixer() {
  const busy = (el) => document.activeElement === el; // never move a slider under the cursor
  if (!busy($('volume'))) $('volume').value = values.volume ?? 0.6;
  $('volumeOut').textContent = pctText(values.volume);
  $('muted').classList.toggle('on', !!values.muted);
  $('muted').textContent = values.muted ? 'Звук выключен' : 'Без звука';
  $('ambient').checked = !!values.ambient;
  if (!busy($('ambientVolume'))) $('ambientVolume').value = values.ambientVolume ?? 0.7;
  $('ambientVolumeOut').textContent = values.ambient ? pctText(values.ambientVolume) : 'выкл';
  for (const row of document.querySelectorAll('#groups .mix')) {
    const v = values.soundGroups?.[row.dataset.group] ?? 1;
    const range = row.querySelector('input');
    if (!busy(range)) range.value = v;
    row.querySelector('output').textContent = pctText(v);
    row.classList.toggle('off', v === 0);
  }
}

function message(text, err = false) {
  $('msg').textContent = text;
  $('msg').className = err ? 'err' : '';
}

// DonationAlerts status.
const pollDA = () => fetch('/da/status').then((r) => r.json()).then((s) => ($('da').textContent = s.status)).catch(() => {});
pollDA();
setInterval(pollDA, 5000);
