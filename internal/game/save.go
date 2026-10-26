package game

import (
	"math"
	"slices"
	"strings"
)

// Save is what survives a server restart or a pause between stream breaks:
// the map is regenerated from its seed, enemies in flight are dropped and the
// interrupted wave is replayed after a build phase.
type Save struct {
	Seed     uint64       `json:"seed"`
	Biome    Biome        `json:"biome"`
	Curve    *float64     `json:"curve,omitempty"` // the map's pathCurve; absent in saves before 0.13
	Wave     int          `json:"wave"`
	Walls    float64      `json:"walls"`
	Over     bool         `json:"over"`
	Towers   []SavedTower `json:"towers"`
	Chatters []SavedChatter
	// Session is each player's state in this game. It rides in the game's
	// record: the chatters table keeps only what lasts between games.
	Session []SessionChatter `json:"session"`

	Treasury  int               `json:"treasury"`
	Banned    map[string]bool   `json:"banned"`
	Links     map[string]string `json:"links"`
	LinkCodes map[string]string `json:"linkCodes"`
	Memes     []*MemeRequest    `json:"memes"`

	RewardUses map[string]int `json:"rewardUses,omitempty"`

	MatchElapsed float64 `json:"matchElapsed,omitempty"` // seconds of the match played
	FinalDue     bool    `json:"finalDue,omitempty"`

	StreamerLogin string `json:"streamerLogin,omitempty"`
}

// wantCell is the kind of cell a building stands on.
func wantCell(s TowerSpec) CellKind {
	if s.OnPath() {
		return CellPath
	}
	return CellBuild
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
	Granted     int    `json:"granted,omitempty"`
	Ammo        string `json:"ammo,omitempty"`
}

type SavedChatter struct {
	Login  string
	Name   string
	Gold   int
	Kills  int
	Drinks int
	Class  string

	Progress Progress
}

// SessionChatter is a player's part in the current game, restored with the
// board, not by LoadChatters.
type SessionChatter struct {
	Login       string  `json:"login"`
	Grant       int     `json:"grant,omitempty"`
	Arrived     bool    `json:"arrived,omitempty"`     // joined this game: no grant on a return
	ClassLocked bool    `json:"classLocked,omitempty"` // the class was changed this game
	JailLeft    float64 `json:"jailLeft,omitempty"`    // still behind bars
	JailedOnce  bool    `json:"jailedOnce,omitempty"`
	Ransom      int     `json:"ransom,omitempty"`
}

func (g *Game) Export() Save {
	s := Save{
		Seed: g.Map.Seed, Biome: g.Map.Biome, Curve: &g.Map.Curve, Wave: g.Wave, Walls: g.Walls, Over: g.Phase == PhaseGameOver,
		Treasury: g.Treasury, Links: g.Links, LinkCodes: g.LinkCodes, Banned: g.Banned, Memes: g.Memes, RewardUses: g.RewardUses,
		MatchElapsed: g.matchElapsed, FinalDue: g.finalDue, StreamerLogin: g.streamerLogin,
	}
	for _, t := range g.Towers {
		s.Towers = append(s.Towers, SavedTower{t.Kind, t.Cell.X, t.Cell.Y, t.Owner, t.Level, t.Invested, t.Built, t.PendingCost, t.Granted, t.Ammo})
	}
	escrow := g.diceEscrow() // stakes on dice tables count as the players' gold
	for key, c := range g.Chatters {
		s.Chatters = append(s.Chatters, SavedChatter{key, c.Name, c.Gold + escrow[key], c.Kills, c.Drinks, c.Class, c.progress()})
		if c.Grant > 0 || c.arrived || c.classLocked || c.jailedOnce {
			s.Session = append(s.Session, SessionChatter{key, c.Grant, c.arrived, c.classLocked, c.jailLeft, c.jailedOnce, c.ransom})
		}
	}
	slices.SortFunc(s.Session, func(a, b SessionChatter) int { return strings.Compare(a.Login, b.Login) })
	if s.Session == nil {
		s.Session = []SessionChatter{} // saved, if empty: older saves have none at all
	}
	return s
}

// LoadEconomy restores what outlives any single game: the treasury and the
// donation links.
func (g *Game) LoadEconomy(s Save) {
	g.Treasury = s.Treasury
	g.loadMemes(s.Memes)
	for k, v := range s.Links {
		g.Links[k] = v
	}
	for k := range s.Banned {
		g.Banned[k] = true
	}
	for k, v := range s.LinkCodes {
		g.LinkCodes[k] = v
	}
}

// LoadChatters restores wallets and stats without touching the board.
func (g *Game) LoadChatters(cs []SavedChatter) {
	for _, sc := range cs {
		c := g.chatter(sc.Login, sc.Name)
		c.Gold, c.Kills, c.Drinks = sc.Gold, sc.Kills, sc.Drinks
		c.setProgress(sc.Progress)
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
	curve := 0.0 // older saves were generated before the setting existed
	if s.Curve != nil {
		curve = *s.Curve
	}
	g.Map = GenerateMap(s.Seed, s.Biome, curve, cam, hasCam)
	g.buildRoute()
	g.Phase = PhaseBuild
	g.Timer = g.Cfg.FirstBuildPhaseSec
	g.Wave = max(s.Wave-1, 0) // an interrupted wave is played again
	g.RewardUses = s.RewardUses
	g.streamerLogin = s.StreamerLogin
	g.matchElapsed, g.finalDue, g.finalWave = s.MatchElapsed, s.FinalDue, false // an interrupted final is played again
	if g.Cfg.BossEvery > 0 {                                                    // the rewards grown by the boss waves already held
		g.rewardMult = math.Pow(1+g.Cfg.RewardGrowth, float64(g.Wave/g.Cfg.BossEvery))
	}
	g.Walls = s.Walls
	g.Enemies, g.Projectiles, g.queue = nil, nil, nil
	g.Towers = map[Pos]*Tower{}
	g.Workers = map[string]*Worker{}
	for _, st := range s.Towers {
		p := Pos{st.X, st.Y}
		_, known := g.Cfg.Towers[st.Kind]
		refund := 0
		switch {
		case !p.In() || !known || g.Map.Grid.At(p) != wantCell(g.Cfg.Towers[st.Kind]):
			refund = st.Invested // the webcam zone or config changed
		case !st.Built, st.Kind == clownKind: // clowns only live while ЭссДиЭф does
			refund = st.Invested
		default:
			refund = st.PendingCost
			t := &Tower{ID: g.id(), Kind: st.Kind, Cell: p, Owner: st.Owner, Level: st.Level, Invested: st.Invested - st.PendingCost, Built: true}
			t.Granted = min(st.Granted, t.Invested)
			t.Ammo = st.Ammo
			g.Towers[p] = t
			g.readyTower(t)
		}
		if c := g.Chatters[st.Owner]; c != nil && refund > 0 && st.Owner != StreamerName {
			g.addGold(c, refund)
		}
	}
	if s.Session == nil {
		// A save from before 0.18.2 lost who had joined: count everyone
		// known as arrived rather than hand the whole tavern a grant.
		for _, c := range g.Chatters {
			c.arrived = true
		}
	}
	for _, sc := range s.Session {
		if c := g.Chatters[sc.Login]; c != nil {
			c.Grant, c.arrived = sc.Grant, sc.Arrived
			c.classLocked = sc.ClassLocked                                             // a restart mid-game must not free a second change
			c.jailLeft, c.jailedOnce, c.ransom = sc.JailLeft, sc.JailedOnce, sc.Ransom // nor open the jail
		}
	}
	g.mapChanged = true
	g.events = nil
	g.emit(Event{Kind: "resumed", Amount: g.Wave + 1, Text: string(s.Biome)})
}

// Progress is what a chatter keeps between games: souls, achievements and
// the lifetime counters behind them.
type Progress struct {
	Souls int            `json:"souls,omitempty"`
	Ach   []string       `json:"ach,omitempty"`
	Stats map[string]int `json:"stats,omitempty"`
}

func (c *Chatter) progress() Progress {
	p := Progress{Souls: c.Souls, Stats: c.Stats}
	for id := range c.Ach {
		p.Ach = append(p.Ach, id)
	}
	slices.Sort(p.Ach)
	return p
}

func (c *Chatter) setProgress(p Progress) {
	c.Souls, c.Stats = p.Souls, p.Stats
	if c.earned() < c.Souls { // souls from before ranks existed
		c.count("souls", c.Souls-c.earned())
	}
	c.Ach = map[string]bool{}
	for _, id := range p.Ach {
		c.Ach[id] = true
	}
}
