// Third-party chat emotes for !s lines. Streamer.bot sends the emotes it
// resolved with each line; on top of that the overlay loads the global 7TV,
// BTTV and FFZ sets, and those of the channels in config.json
// (twitchChannelId, emoteChannels), by their Twitch IDs. A failed request
// just leaves fewer emotes.

let channelSets = []; // name -> image URL, one map per channel in config order
const globalSet = new Map(); // and the global ones, which they all override
let loadedFor = null;

// emoteURL is the picture for word: the line's own emotes first, then the
// channels' in their order, then the globals.
export function emoteURL(word, lineEmotes) {
  if (lineEmotes?.[word]) return lineEmotes[word];
  for (const set of channelSets) if (set.has(word)) return set.get(word);
  return globalSet.get(word) ?? null;
}

// loadEmotes fetches the sets once per list of channels.
export function loadEmotes(channels = []) {
  const key = channels.join(',') || '-';
  if (loadedFor === key) return;
  const globalsToo = loadedFor === null;
  loadedFor = key;
  channelSets = channels.map(() => new Map());
  const adder = (set) => (name, url) => {
    if (name && url && !set.has(name)) set.set(name, url.startsWith('//') ? `https:${url}` : url);
  };
  const get = (url) => fetch(url).then((r) => (r.ok ? r.json() : null)).catch(() => null);
  const seventv = (add) => (set) => {
    for (const e of set?.emotes ?? []) {
      const host = e.data?.host;
      if (host?.url) add(e.name, `${host.url}/2x.webp`);
    }
  };
  const bttv = (add) => (list) => {
    for (const e of list ?? []) add(e.code, `https://cdn.betterttv.net/emote/${e.id}/2x`);
  };
  const ffz = (add) => (sets) => {
    for (const set of Object.values(sets ?? {})) {
      for (const e of set.emoticons ?? []) add(e.name, e.animated?.['2'] ?? e.animated?.['1'] ?? e.urls?.['2'] ?? e.urls?.['1']);
    }
  };
  channels.forEach((channel, i) => {
    const mine = adder(channelSets[i]);
    const id = encodeURIComponent(channel);
    get(`https://7tv.io/v3/users/twitch/${id}`).then((u) => seventv(mine)(u?.emote_set));
    get(`https://api.betterttv.net/3/cached/users/twitch/${id}`).then((u) => {
      bttv(mine)(u?.channelEmotes);
      bttv(mine)(u?.sharedEmotes);
    });
    get(`https://api.frankerfacez.com/v1/room/id/${id}`).then((r) => ffz(mine)(r?.sets));
  });
  if (!globalsToo) return;
  const all = adder(globalSet);
  get('https://7tv.io/v3/emote-sets/global').then(seventv(all));
  get('https://api.betterttv.net/3/cached/emotes/global').then(bttv(all));
  get('https://api.frankerfacez.com/v1/set/global').then((r) => ffz(all)(r?.sets));
}
