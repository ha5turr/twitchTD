package game

// Save is what survives a server restart or a pause between stream breaks:
// the map is regenerated from its seed, enemies in flight are dropped and the
// interrupted wave is replayed after a build phase.
type Save struct {
	Seed     uint64       `json:"seed"`
	Biome    Biome        `json:"biome"`
	Wave     int          `json:"wave"`
	Walls    float64      `json:"walls"`
	Over     bool         `json:"over"`
	Towers   []SavedTower `json:"towers"`
	Chatters []SavedChatter
}

type SavedTower struct {
	Kind        string `json:"kind"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	Owner       string `json:"owner"`
	Level       int    `json:"level"`
	Invested    int    `json:"invested"`
	Built       bool   `json:"built"`
	PendingCost int    `json:"pendingCost"`
}

type SavedChatter struct {
	Login  string
	Name   string
	Gold   int
	Kills  int
	Drinks int
	Class  string
}

func (g *Game) Export() Save {
	s := Save{Seed: g.Map.Seed, Biome: g.Map.Biome, Wave: g.Wave, Walls: g.Walls, Over: g.Phase == PhaseGameOver}
	for _, t := range g.Towers {
		s.Towers = append(s.Towers, SavedTower{t.Kind, t.Cell.X, t.Cell.Y, t.Owner, t.Level, t.Invested, t.Built, t.PendingCost})
	}
	for key, c := range g.Chatters {
		s.Chatters = append(s.Chatters, SavedChatter{key, c.Name, c.Gold, c.Kills, c.Drinks, c.Class})
	}
	return s
}

// LoadChatters restores wallets and stats without touching the board.
func (g *Game) LoadChatters(cs []SavedChatter) {
	for _, sc := range cs {
		c := g.chatter(sc.Login, sc.Name)
		c.Gold, c.Kills, c.Drinks = sc.Gold, sc.Kills, sc.Drinks
		if _, ok := classByKey(sc.Class); ok {
			c.Class = sc.Class
		}
	}
	g.events = nil // no "join" banners for people who are not here yet
}

// Restore rebuilds the saved game. Unfinished work is refunded because the
// mercenaries doing it are gone.
func (g *Game) Restore(s Save) {
	if s.Over || s.Seed == 0 && s.Biome == "" {
		return
	}
	if _, ok := profiles[s.Biome]; !ok {
		return
	}
	cam, hasCam := g.Cfg.WebcamRect()
	g.Map = GenerateMap(s.Seed, s.Biome, cam, hasCam)
	g.buildRoute()
	g.Phase = PhaseBuild
	g.Timer = g.Cfg.FirstBuildPhaseSec
	g.Wave = max(s.Wave-1, 0) // an interrupted wave is played again
	g.Walls = s.Walls
	g.Enemies, g.Projectiles, g.queue = nil, nil, nil
	g.Towers = map[Pos]*Tower{}
	g.Workers = map[string]*Worker{}
	for _, st := range s.Towers {
		p := Pos{st.X, st.Y}
		_, known := g.Cfg.Towers[st.Kind]
		refund := 0
		switch {
		case !p.In() || !known || g.Map.Grid.At(p) != CellBuild:
			refund = st.Invested // the webcam zone or config changed
		case !st.Built:
			refund = st.Invested
		default:
			refund = st.PendingCost
			g.Towers[p] = &Tower{ID: g.id(), Kind: st.Kind, Cell: p, Owner: st.Owner, Level: st.Level, Invested: st.Invested - st.PendingCost, Built: true}
		}
		if c := g.Chatters[st.Owner]; c != nil && refund > 0 && st.Owner != StreamerName {
			g.addGold(c, refund)
		}
	}
	g.mapChanged = true
	g.events = nil
	g.emit(Event{Kind: "resumed", Amount: g.Wave + 1, Text: string(s.Biome)})
}
