// WebSocket connection to the game server with automatic reconnect: OBS keeps
// the overlay open for hours and the server may restart in between.
export function connect(handlers) {
  let ws;
  let version = null; // the server release this page was loaded from
  const open = () => {
    ws = new WebSocket(`ws://${location.host}/ws`);
    ws.onmessage = (e) => {
      const msg = JSON.parse(e.data);
      if (msg.t === 'map' && msg.version) {
        // After !tdupdate the server comes back on a new release: reload so
        // OBS runs its scripts and art, not the ones it loaded before.
        if (version && msg.version !== version) {
          location.reload();
          return;
        }
        version = msg.version;
      }
      handlers[msg.t]?.(msg);
    };
    ws.onclose = () => setTimeout(open, 1000);
  };
  open();
  return {
    send(obj) {
      if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify(obj));
    },
  };
}
