package game

import (
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"time"
)

type Phase string

const (
	PhaseBuild    Phase = "build"
	PhaseWave     Phase = "wave"
	PhaseGameOver Phase = "gameover"
)

// StreamerName owns towers placed from the streamer panel; it has no limits.
const StreamerName = "streamer"

type vec struct{ X, Y float64 }

func (a vec) sub(b vec) vec        { return vec{a.X - b.X, a.Y - b.Y} }
func (a vec) len() float64         { return math.Hypot(a.X, a.Y) }
func center(p Pos) vec             { return vec{float64(p.X) + 0.5, float64(p.Y) + 0.5} }
func lerp(a, b vec, t float64) vec { return vec{a.X + (b.X-a.X)*t, a.Y + (b.Y-a.Y)*t} }

type Enemy struct {
	ID     int
	Kind   string
	HP     float64
	MaxHP  float64
	Dist   float64 // distance travelled along the route, in cells
	Pos    vec
	AtGate bool
}

type Tower struct {
	ID       int
	Kind     string
	Cell     Pos
	Owner    string
	Level    int
	Invested int
	// Built is false while the tower is a construction site. PendingCost is
	// the paid price of an upgrade a mercenary has not finished yet.
	Built       bool
	PendingCost int
	Progress    float64 // 0..1 of the current construction or upgrade
	cooldown    float64
}

type Projectile struct {
	ID     int
	Pos    vec
	Target int
	Aim    vec // last known target position
	Damage float64
	Magic  bool
	Speed  float64
	Owner  string
}

type Chatter struct {
	Name     string // display name
	Gold     int
	Kills    int
	LastSeen time.Time
	lastPaid time.Time
}

// Event is a one-off happening the overlay animates (floating text, sounds).
type Event struct {
	Kind   string  `json:"kind"`
	User   string  `json:"user,omitempty"`
	Text   string  `json:"text,omitempty"`
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Amount int     `json:"amount,omitempty"`
}

type spawn struct {
	kind string
	hp   float64
}

type Game struct {
	Cfg Config
	Now func() time.Time

	Map    *MapData
	route  []vec
	segLen []float64
	total  float64

	Phase  Phase
	Timer  float64 // seconds left in the build / game over phase
	Wave   int
	Walls  float64
	Paused bool

	Enemies     []*Enemy
	Towers      map[Pos]*Tower
	Projectiles []*Projectile
	Chatters    map[string]*Chatter // keyed by lower-case login
	Workers     map[string]*Worker  // mercenaries out of the tavern, by owner

	queue      []spawn
	spawnTimer float64
	nextID     int
	rng        *rand.Rand
	events     []Event
	mapChanged bool
}

func New(cfg Config, seed uint64) *Game {
	g := &Game{
		Cfg:      cfg,
		Now:      time.Now,
		Chatters: map[string]*Chatter{},
		rng:      rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
	}
	g.NewGame("")
	return g
}

// NewGame starts a fresh game on a new random map. A non-empty biome forces it.
func (g *Game) NewGame(biome Biome) {
	last := Biome("")
	if g.Map != nil {
		last = g.Map.Biome
	}
	if biome == "" {
		for {
			biome = AllBiomes[g.rng.IntN(len(AllBiomes))]
			if biome != last {
				break
			}
		}
	}
	cam, hasCam := g.Cfg.WebcamRect()
	g.Map = GenerateMap(g.rng.Uint64()%1_000_000, biome, cam, hasCam)
	g.buildRoute()

	g.Phase = PhaseBuild
	g.Timer = g.Cfg.FirstBuildPhaseSec
	g.Wave = 0
	g.Walls = g.Cfg.WallsHP
	g.Enemies = nil
	g.Towers = map[Pos]*Tower{}
	g.Workers = map[string]*Worker{}
	g.Projectiles = nil
	g.queue = nil
	for _, c := range g.Chatters {
		if !g.Cfg.KeepGold {
			c.Gold = g.Cfg.StartGold
		}
	}
	g.mapChanged = true
	g.emit(Event{Kind: "newGame", Text: string(biome)})
}

// buildRoute turns the path cells into a polyline from beyond the right edge
// to the tavern gate left of column A.
func (g *Game) buildRoute() {
	p := g.Map.Path
	g.route = g.route[:0]
	g.route = append(g.route, vec{GridW + 0.6, float64(p[0].Y) + 0.5})
	for _, c := range p {
		g.route = append(g.route, center(c))
	}
	g.route = append(g.route, vec{-0.3, float64(p[len(p)-1].Y) + 0.5})
	g.segLen = g.segLen[:0]
	g.total = 0
	for i := 1; i < len(g.route); i++ {
		l := g.route[i].sub(g.route[i-1]).len()
		g.segLen = append(g.segLen, l)
		g.total += l
	}
}

func (g *Game) routePos(d float64) vec {
	for i, l := range g.segLen {
		if d <= l {
			return lerp(g.route[i], g.route[i+1], d/l)
		}
		d -= l
	}
	return g.route[len(g.route)-1]
}

func (g *Game) id() int { g.nextID++; return g.nextID }

func (g *Game) emit(e Event) { g.events = append(g.events, e) }

// TakeEvents returns and clears events accumulated since the last call.
func (g *Game) TakeEvents() []Event {
	ev := g.events
	g.events = nil
	return ev
}

// TakeMapChanged reports whether the map changed since the last call.
func (g *Game) TakeMapChanged() bool {
	ch := g.mapChanged
	g.mapChanged = false
	return ch
}

// ActiveChatters counts people who wrote in chat within the active window.
func (g *Game) ActiveChatters() int {
	cutoff := g.Now().Add(-activeWindow(g.Cfg))
	n := 0
	for key, c := range g.Chatters {
		if key != StreamerName && c.LastSeen.After(cutoff) {
			n++
		}
	}
	return n
}

func activeWindow(c Config) time.Duration {
	return time.Duration(c.ActiveWindowMn) * time.Minute
}

func (g *Game) Tick(dt float64) {
	if g.Paused {
		return
	}
	switch g.Phase {
	case PhaseBuild:
		g.Timer -= dt
		if g.Timer <= 0 {
			g.StartWave()
		}
	case PhaseGameOver:
		g.Timer -= dt
		if g.Timer <= 0 {
			g.NewGame("")
		}
		return
	case PhaseWave:
		g.tickSpawns(dt)
		if len(g.queue) == 0 && len(g.Enemies) == 0 {
			g.endWave()
		}
	}
	g.tickEnemies(dt)
	g.tickWorkers(dt)
	g.tickTowers(dt)
	g.tickProjectiles(dt)
	if g.Walls <= 0 && g.Phase != PhaseGameOver {
		g.Walls = 0
		g.Phase = PhaseGameOver
		g.Timer = g.Cfg.GameOverSec
		g.emit(Event{Kind: "gameOver", Amount: g.Wave})
	}
}

// StartWave skips whatever is left of the build phase.
func (g *Game) StartWave() {
	if g.Phase != PhaseBuild {
		return
	}
	g.Wave++
	g.Phase = PhaseWave
	g.Timer = 0
	n := max(g.ActiveChatters(), 1)
	sq := math.Sqrt(float64(n))
	hp := g.Cfg.BaseHP * math.Pow(g.Cfg.HPGrowth, float64(g.Wave)) * (1 + g.Cfg.ChatterHPCoef*sq)
	count := g.Cfg.BaseCount + g.Cfg.CountPerWave*g.Wave + int(sq)

	g.queue = g.queue[:0]
	for i := 0; i < count; i++ {
		kind := "normal"
		roll := g.rng.Float64()
		switch {
		case g.Wave >= 6 && roll < 0.2:
			kind = "armored"
		case g.Wave >= 3 && roll < 0.45:
			kind = "fast"
		}
		g.queue = append(g.queue, spawn{kind, hp * g.Cfg.Enemies[kind].HPMult})
	}
	boss := g.Cfg.BossEvery > 0 && g.Wave%g.Cfg.BossEvery == 0
	if boss {
		g.queue = append(g.queue, spawn{"boss", hp * g.Cfg.BossHPMult})
	}
	g.spawnTimer = 0
	g.emit(Event{Kind: "waveStart", Amount: g.Wave, Text: map[bool]string{true: "boss"}[boss]})
}

func (g *Game) endWave() {
	g.Phase = PhaseBuild
	g.Timer = g.Cfg.BuildPhaseSec
	bonus := 10 + 2*g.Wave
	owners := map[string]bool{}
	for _, t := range g.Towers {
		owners[t.Owner] = true
	}
	for name := range owners {
		if c := g.Chatters[name]; c != nil {
			g.addGold(c, bonus)
		}
	}
	g.emit(Event{Kind: "waveCleared", Amount: g.Wave})
}

func (g *Game) tickSpawns(dt float64) {
	if len(g.queue) == 0 {
		return
	}
	g.spawnTimer -= dt
	if g.spawnTimer > 0 {
		return
	}
	s := g.queue[0]
	g.queue = g.queue[1:]
	g.spawnTimer = g.Cfg.SpawnIntervalSec
	g.Enemies = append(g.Enemies, &Enemy{ID: g.id(), Kind: s.kind, HP: s.hp, MaxHP: s.hp, Pos: g.route[0]})
}

func (g *Game) speedMult() float64 {
	if g.Map.Biome == BiomeIce {
		return 1.15
	}
	return 1
}

func (g *Game) tickEnemies(dt float64) {
	for _, e := range g.Enemies {
		spec := g.Cfg.Enemies[e.Kind]
		if e.AtGate {
			g.Walls -= spec.WallDPS * dt
			continue
		}
		e.Dist += spec.Speed * g.speedMult() * dt
		if e.Dist >= g.total {
			e.Dist = g.total
			e.AtGate = true
		}
		e.Pos = g.routePos(e.Dist)
	}
}

func (g *Game) tickTowers(dt float64) {
	for _, t := range g.Towers {
		if !t.Built {
			continue
		}
		spec := g.Cfg.Towers[t.Kind]
		t.cooldown -= dt
		if t.cooldown > 0 {
			continue
		}
		tc := center(t.Cell)
		var target *Enemy
		for _, e := range g.Enemies {
			if e.Pos.sub(tc).len() <= spec.Range && (target == nil || e.Dist > target.Dist) {
				target = e
			}
		}
		if target == nil {
			continue
		}
		t.cooldown = 1 / spec.Rate
		g.Projectiles = append(g.Projectiles, &Projectile{
			ID: g.id(), Pos: tc, Target: target.ID, Aim: target.Pos,
			Damage: spec.Damage * math.Pow(spec.UpgMult, float64(t.Level-1)),
			Magic:  spec.Magic, Speed: spec.ProjSpd, Owner: t.Owner,
		})
	}
}

func (g *Game) enemy(id int) *Enemy {
	for _, e := range g.Enemies {
		if e.ID == id {
			return e
		}
	}
	return nil
}

func (g *Game) tickProjectiles(dt float64) {
	alive := g.Projectiles[:0]
	for _, p := range g.Projectiles {
		target := g.enemy(p.Target)
		if target != nil {
			p.Aim = target.Pos
		}
		d := p.Aim.sub(p.Pos)
		step := p.Speed * dt
		if d.len() > step {
			p.Pos = vec{p.Pos.X + d.X/d.len()*step, p.Pos.Y + d.Y/d.len()*step}
			alive = append(alive, p)
			continue
		}
		if target != nil {
			g.hit(target, p)
		}
	}
	g.Projectiles = alive
}

func (g *Game) hit(e *Enemy, p *Projectile) {
	dmg := p.Damage
	if !p.Magic {
		dmg *= 1 - g.Cfg.Enemies[e.Kind].Armor
	}
	e.HP -= dmg
	if e.HP > 0 {
		return
	}
	g.Enemies = slices.DeleteFunc(g.Enemies, func(x *Enemy) bool { return x == e })
	bounty := g.Cfg.Enemies[e.Kind].Bounty
	if c := g.Chatters[p.Owner]; c != nil {
		c.Kills++
		bounty = g.addGold(c, bounty)
	}
	g.emit(Event{Kind: "kill", User: p.Owner, X: e.Pos.X, Y: e.Pos.Y, Amount: bounty, Text: e.Kind})
}

// addGold credits gold up to the wallet cap and returns what was actually
// added. The overflow repairs the tavern walls.
func (g *Game) addGold(c *Chatter, amount int) int {
	room := g.Cfg.WalletCap - c.Gold
	add := min(amount, max(room, 0))
	c.Gold += add
	if over := amount - add; over > 0 {
		g.Walls = min(g.Walls+float64(over)*g.Cfg.OverflowWallsPerGold, g.Cfg.WallsHP)
	}
	return add
}

// chatter returns the chatter for key, creating one with starting gold.
func (g *Game) chatter(key, display string) *Chatter {
	c := g.Chatters[key]
	if c == nil {
		c = &Chatter{Name: display, Gold: g.Cfg.StartGold}
		g.Chatters[key] = c
		g.emit(Event{Kind: "join", User: display})
	}
	if display != "" && key != StreamerName {
		c.Name = display // display names can change between streams
	}
	return c
}

// Touch registers chat activity and pays for it (with a cooldown).
func (g *Game) Touch(key, display string) *Chatter {
	c := g.chatter(key, display)
	now := g.Now()
	c.LastSeen = now
	if now.Sub(c.lastPaid) >= time.Duration(g.Cfg.ChatCooldownSec*float64(time.Second)) {
		c.lastPaid = now
		g.addGold(c, g.Cfg.ChatGold)
	}
	return c
}

// Present pays lurkers from the Present Viewers list. Only chatters who
// already have a mercenary are paid, and it does not count as chat activity,
// so silent viewers never raise the difficulty.
func (g *Game) Present(logins []string) {
	for _, l := range logins {
		if c := g.Chatters[strings.ToLower(l)]; c != nil {
			g.addGold(c, g.Cfg.PresentGold)
		}
	}
}

func (g *Game) towersOf(owner string) int {
	n := 0
	for _, t := range g.Towers {
		if t.Owner == owner {
			n++
		}
	}
	return n
}
