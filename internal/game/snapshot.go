package game

import (
	"math"
	"sort"
)

// MapMsg is sent on connect and whenever a new map is generated.
type MapMsg struct {
	T       string       `json:"t"` // "map"
	Seed    uint64       `json:"seed"`
	Biome   Biome        `json:"biome"`
	W       int          `json:"w"`
	H       int          `json:"h"`
	Cells   []string     `json:"cells"` // one string per row, see cellChars
	Path    [][2]int     `json:"path"`
	Route   [][2]float64 `json:"route"`
	Torches [][2]int     `json:"torches"`
}

// One character per cell keeps the message small and readable in devtools.
var cellChars = map[CellKind]byte{CellBuild: '.', CellPath: '#', CellObstacle: 'o', CellWebcam: 'w'}

func (g *Game) MapMessage() MapMsg {
	m := MapMsg{T: "map", Seed: g.Map.Seed, Biome: g.Map.Biome, W: GridW, H: GridH}
	for y := 0; y < GridH; y++ {
		row := make([]byte, GridW)
		for x := 0; x < GridW; x++ {
			row[x] = cellChars[g.Map.Grid.Cells[y][x]]
		}
		m.Cells = append(m.Cells, string(row))
	}
	for _, p := range g.Map.Path {
		m.Path = append(m.Path, [2]int{p.X, p.Y})
	}
	for _, v := range g.route {
		m.Route = append(m.Route, [2]float64{v.X, v.Y})
	}
	for _, p := range g.Map.Torches {
		m.Torches = append(m.Torches, [2]int{p.X, p.Y})
	}
	return m
}

type StateMsg struct {
	T        string        `json:"t"` // "state"
	Phase    Phase         `json:"phase"`
	Timer    float64       `json:"timer"`
	Wave     int           `json:"wave"`
	Walls    float64       `json:"walls"`
	WallsMax float64       `json:"wallsMax"`
	Paused   bool          `json:"paused"`
	Cap      int           `json:"walletCap"`
	Limit    int           `json:"towerLimit"`
	Active   int           `json:"active"`
	Enemies  []EnemyView   `json:"enemies"`
	Towers   []TowerView   `json:"towers"`
	Proj     []ProjView    `json:"proj"`
	Chatters []ChatterView `json:"chatters"`
	Workers  []WorkerView  `json:"workers"`
	Events   []Event       `json:"events,omitempty"`
	Summary  *Summary      `json:"summary,omitempty"` // only on the game over screen
}

type EnemyView struct {
	ID    int     `json:"id"`
	Kind  string  `json:"kind"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	HP    float64 `json:"hp"`
	MaxHP float64 `json:"maxHp"`
}

type TowerView struct {
	ID       int     `json:"id"`
	Kind     string  `json:"kind"`
	X        int     `json:"x"`
	Y        int     `json:"y"`
	Level    int     `json:"lvl"`
	Owner    string  `json:"owner"`
	Built    bool    `json:"built"`
	Progress float64 `json:"progress,omitempty"`
}

type WorkerView struct {
	Name  string      `json:"name"`
	Class string      `json:"class"`
	X     float64     `json:"x"`
	Y     float64     `json:"y"`
	State WorkerState `json:"state"`
}

type ProjView struct {
	ID    int     `json:"id"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Magic bool    `json:"magic,omitempty"`
}

type ChatterView struct {
	Name   string `json:"name"`
	Gold   int    `json:"gold"`
	Towers int    `json:"towers"`
	Kills  int    `json:"kills"`
	Drinks int    `json:"drinks"`
	Class  string `json:"class"`
	Drunk  int    `json:"drunk"`
	Active bool   `json:"active"`
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

// StateMessage builds a snapshot and drains pending events into it.
func (g *Game) StateMessage() StateMsg {
	s := StateMsg{
		T: "state", Phase: g.Phase, Timer: round2(g.Timer), Wave: g.Wave,
		Walls: math.Ceil(g.Walls), WallsMax: g.Cfg.WallsHP, Paused: g.Paused, Cap: g.Cfg.WalletCap, Limit: g.Cfg.TowerLimit,
		Active: g.ActiveChatters(), Events: g.TakeEvents(),
		Enemies: []EnemyView{}, Towers: []TowerView{}, Proj: []ProjView{}, Chatters: []ChatterView{},
		Workers: []WorkerView{},
	}
	for _, e := range g.Enemies {
		s.Enemies = append(s.Enemies, EnemyView{e.ID, e.Kind, round2(e.Pos.X), round2(e.Pos.Y), math.Ceil(e.HP), math.Ceil(e.MaxHP)})
	}
	owners := map[string]int{}
	for _, t := range g.Towers {
		owners[t.Owner]++
		name := t.Owner
		if c := g.Chatters[t.Owner]; c != nil {
			name = c.Name
		}
		s.Towers = append(s.Towers, TowerView{t.ID, t.Kind, t.Cell.X, t.Cell.Y, t.Level, name, t.Built, round2(t.Progress)})
	}
	sort.Slice(s.Towers, func(i, j int) bool { return s.Towers[i].ID < s.Towers[j].ID })
	for _, p := range g.Projectiles {
		s.Proj = append(s.Proj, ProjView{p.ID, round2(p.Pos.X), round2(p.Pos.Y), p.Magic})
	}
	activeCut := g.Now().Add(-activeWindow(g.Cfg))
	for key, c := range g.Chatters {
		active := c.LastSeen.After(activeCut)
		// Wallets loaded from the database stay out of the snapshot until
		// their owner shows up, so the tavern holds only people present.
		if key == StreamerName || !active && owners[key] == 0 && g.Workers[key] == nil {
			continue
		}
		s.Chatters = append(s.Chatters, ChatterView{
			c.Name, c.Gold, owners[key], c.Kills, c.Drinks, c.Class, int(math.Ceil(c.Drunk)), active,
		})
	}
	if g.Phase == PhaseGameOver {
		s.Summary = g.summary
	}
	sort.Slice(s.Chatters, func(i, j int) bool { return s.Chatters[i].Name < s.Chatters[j].Name })
	for _, w := range g.Workers {
		class := ""
		if c := g.Chatters[w.Owner]; c != nil {
			class = c.Class
		}
		s.Workers = append(s.Workers, WorkerView{w.Name, class, round2(w.Pos.X), round2(w.Pos.Y), w.State})
	}
	sort.Slice(s.Workers, func(i, j int) bool { return s.Workers[i].Name < s.Workers[j].Name })
	return s
}
