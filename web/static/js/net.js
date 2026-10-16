// WebSocket connection to the game server with automatic reconnect: OBS keeps
// the overlay open for hours and the server may restart in between.
export function connect(handlers) {
  let ws;
  const open = () => {
    ws = new WebSocket(`ws://${location.host}/ws`);
    ws.onmessage = (e) => {
      const msg = JSON.parse(e.data);
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
