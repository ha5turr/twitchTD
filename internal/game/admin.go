package game

import "strings"

// SetConfig swaps balance settings while the game runs, e.g. after the
// streamer edits them in the panel or config.json changes on disk. The
// webcam zone applies from the next map.
func (g *Game) SetConfig(cfg Config) {
	g.Cfg = cfg
	g.Walls = min(g.Walls, cfg.WallsHP)
	for _, c := range g.Chatters {
		c.Gold = min(c.Gold, max(cfg.WalletCap, 0))
	}
}

// ban toggles whether a chatter may build. Banned chatters keep chatting,
// drinking and earning; their buildings stay unless the streamer removes them.
func (g *Game) ban(name string, banned bool) {
	key := strings.ToLower(strings.TrimPrefix(name, "@"))
	if key == "" {
		return
	}
	if banned {
		g.Banned[key] = true
	} else {
		delete(g.Banned, key)
	}
}
