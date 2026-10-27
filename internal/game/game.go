package game

import (
	"math"
	"math/rand/v2"
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

type Chatter struct {
	Name     string // display name
	Gold     int
	Kills    int // all time
	Drinks   int // all time
	Class    string
	Drunk    float64 // mugs in the blood, wears off over time
	out      float64 // seconds left passed out under the table
	snored   float64 // seconds until the next "zzz" reply, so chat is not flooded
	LastSeen time.Time
	lastPaid time.Time

	// Grant is gold for buildings and upgrades only, given once to a late
	// newcomer; it sits outside the wallet and its cap.
	Grant   int
	arrived bool // joined this game at least once: no grant on a later return

	// This game only; shown on the game over screen.
	GameDamage  float64
	GameKills   int
	GameDrinks  int
	classLocked bool // the class was changed this game

	// The jail (jail.go): seconds left behind bars, the ransom that frees
	// earlier, whether this game already locked him up once, and until when
	// a failed vote shields him from the next one.
	jailLeft    float64
	jailedOnce  bool
	ransom      int
	jailVoteAt  time.Time
	saidAt      time.Time // last !s line
	kickedUntil time.Time // thrown out of the tavern by a moderator: no way back till then
	dancedAt    time.Time // last dance on the table

	// Kept between games (progress.go).
	Souls      int
	Ach        map[string]bool // achievements earned
	Stats      map[string]int  // lifetime counters behind achievements
	diceStreak int

	// Drinks (drinks.go): the buildings hit buff times as hard for buffLeft
	// seconds; the ale then switches to nextBuff for nextSec seconds.
	buff, buffLeft    float64
	nextBuff, nextSec float64
	passiveCD         float64 // rogue and druid: seconds to the next trick (rogues.go)
	skillCD           float64 // class skill (heroes.go): seconds to the next use
	skill2CD          float64 // a class's second skill (blood pact, gate brazier, feast, wolves)
	veilWave          int     // rogue: the wave his veil was spent in (workers.go)
	musicLeft         float64 // bard: seconds of music left
	wasActive         bool    // chatted since the last AFK sweep (economy.go)
	Joined            bool    // took part this session: !play or any game command (commands.go)
}

// Event is a one-off happening the overlay animates (floating text, sounds).
type Event struct {
	Kind   string            `json:"kind"`
	User   string            `json:"user,omitempty"`
	Text   string            `json:"text,omitempty"`
	Emotes map[string]string `json:"emotes,omitempty"` // a !s line's emotes: name -> image URL
	X      float64           `json:"x,omitempty"`
	Y      float64           `json:"y,omitempty"`
	Amount int               `json:"amount,omitempty"`
	X2     float64           `json:"x2,omitempty"` // the far end of a beam (zap, curse)
	Y2     float64           `json:"y2,omitempty"`
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

	// Treasury is the common pot: unmatched donations and gold that did not
	// fit a wallet. It repairs the walls and the streamer can hand it out.
	Treasury    int
	treasuryAcc float64
	Links       map[string]string // donation name (lower case) -> chatter login
	LinkCodes   map[string]string // "4821" -> login, from !link
	Banned      map[string]bool   // chatters the streamer forbade to build
	Dice        []*DiceGame       // games of dice in the tavern
	Memes       []*MemeRequest    // memecoin requests waiting for the streamer
	RewardUses  map[string]int    // channel point rewards taken this game, by "title|login"
	pactLeft    float64           // a warlock's blood pact: seconds every building still hits harder
	gateFires   []*GateFire       // pyromancers' braziers on the tavern wall
	storms      []*Storm          // artifact totems' lightning charges on the path
	wallShield  float64           // artifact shrines' shield over the tavern walls
	pactBoost   float64
	Panthers    []*Panther // druids on the hunt
	Runs        []*Run     // charging warriors, rogues in a shadow-step
	meteors     []*meteor  // a mage's meteor shower, still falling

	// Admin reaches the overlay settings (sound) for mod chat commands.
	Admin SettingsAdmin
	// Ops runs !tdupdate and !tdrollback; nil when the server has none.
	Ops Operator

	queue      []spawn
	spawnTimer float64
	nextID     int
	rng        *rand.Rand
	events     []Event
	mapChanged bool
	// The match (match.go).
	streamerLogin string          // the broadcaster's Twitch login, learnt from their chat, saved
	matchElapsed  float64         // seconds played this game, saved
	finalDue      bool            // the time is up: the next wave is the final
	finalWave     bool            // the wave on the field is the final
	ready         map[string]bool // !готов this build phase
	readyGo       bool            // enough are ready: the countdown runs
	tempoSaved    float64         // build phase seconds the ready vote cut
	emoteIDs      []string        // channels whose emotes the overlay loads (SetEmoteChannels)
	summary       *Summary
	// The King in Yellow (king.go).
	introLeft   float64              // seconds of the scene before the final
	signs       []*Sign              // Yellow Signs on buildings
	signSupport bool                 // the next sign goes on a support
	maskLeft    float64              // seconds the auras still work the other way
	songLeft    float64              // seconds Cassilda's song still breaks skills
	brokenLinks map[int]bool         // soul links the king tore (crypt)
	nameWarned  bool                 // the chat was told not to say his name
	cellDist    []float64            // route distance to the centre of each path cell
	waveHP      float64              // base enemy HP of the current wave
	rushLeft    float64              // seconds of the frost demon's speed-up left
	assist      []string             // drummers behind the shot being resolved
	rewardMult  float64              // grows after every boss wave: kills, assists and barricades pay more
	smallKill   float64              // a small party's extra share of every bounty this wave (0.5 is x1.5)
	afkCheck    float64              // seconds to the next AFK sweep
	helpAt      map[string]time.Time // last !помощь answer per topic
	jailVote    *JailVote            // the tavern voting to lock someone up
	mapVote     *MapVote             // the tavern voting for a new map (newmap.go)
	mapVoteAt   time.Time            // a failed vote: no new one until then
	nextMeme    int
}

func New(cfg Config, seed uint64) *Game {
	g := &Game{
		Cfg:       cfg,
		Now:       time.Now,
		Chatters:  map[string]*Chatter{},
		Links:     map[string]string{},
		LinkCodes: map[string]string{},
		Banned:    map[string]bool{},
		helpAt:    map[string]time.Time{},
		rng:       rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
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
	g.Map = GenerateMap(g.rng.Uint64()%1_000_000, biome, g.Cfg.PathCurve, cam, hasCam)
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
	g.summary = nil
	g.rewardMult = 1
	g.smallKill = 0
	g.RewardUses = nil
	g.pactLeft = 0
	if !g.Cfg.KeepTreasury {
		g.Treasury, g.treasuryAcc = 0, 0
	}
	g.Panthers, g.Runs, g.meteors, g.gateFires, g.storms = nil, nil, nil, nil, nil
	g.wallShield = 0
	// Dice stakes go back before wallets reset, so a new game never eats them.
	for _, d := range g.Dice {
		g.refundDice(d)
	}
	g.Dice = nil
	g.jailVote, g.mapVote = nil, nil
	g.matchElapsed, g.finalDue, g.finalWave = 0, false, false
	g.ready, g.readyGo, g.tempoSaved = nil, false, 0
	g.clearKing()
	g.nameWarned = false
	for _, c := range g.Chatters {
		if !g.Cfg.KeepGold {
			c.Gold = g.Cfg.StartGold
		}
		c.GameDamage, c.GameKills, c.GameDrinks, c.classLocked = 0, 0, 0, false
		c.Grant, c.arrived = 0, c.Joined // those still in the tavern start this game with it
		c.out, c.veilWave = 0, 0
		c.jailLeft, c.jailedOnce, c.ransom, c.jailVoteAt = 0, false, 0, time.Time{}
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
	g.cellDist = g.cellDist[:0]
	g.total = 0
	for i := 1; i < len(g.route); i++ {
		l := g.route[i].sub(g.route[i-1]).len()
		g.segLen = append(g.segLen, l)
		g.total += l
		if i <= len(p) {
			g.cellDist = append(g.cellDist, g.total) // route[i] is the centre of path[i-1]
		}
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

// SetEmoteChannels sets the channels whose emotes the overlay loads and
// sends them out with the map.
func (g *Game) SetEmoteChannels(ids []string) {
	g.emoteIDs = ids
	g.mapChanged = true
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
		if key != StreamerName && c.Joined && c.LastSeen.After(cutoff) {
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
	g.soberUp(dt)
	g.tickDrinks(dt)
	g.tickRogues(dt)
	g.sweepAFK(dt)
	g.tickTreasury(dt)
	g.tickDice(dt)
	g.tickJail(dt)
	g.tickMapVote(dt)
	g.tickMatch(dt)
	switch g.Phase {
	case PhaseBuild:
		g.Timer -= dt
		if g.Timer <= 0 {
			g.StartWave()
		}
	case PhaseGameOver:
		if g.Cfg.GameOverSec > 0 { // otherwise the game waits for !нк
			if g.Timer -= dt; g.Timer <= 0 {
				g.NewGame("")
			}
		}
		return
	case PhaseWave:
		if g.introLeft > 0 {
			if g.introLeft -= dt; g.introLeft <= 0 { // the king's scene: the final waits
				g.emit(Event{Kind: "kingGo"})
			}
		} else {
			g.tickSpawns(dt)
		}
		g.tickSigns(dt)
		if len(g.queue) == 0 && len(g.Enemies) == 0 {
			g.endWave()
		}
	}
	g.tickEnemies(dt)
	g.tickDruids(dt)
	g.tickHeroes(dt)
	g.tickWorkers(dt)
	g.tickTowers(dt)
	g.tickGateFires(dt)
	g.tickStorms(dt)
	g.tickSoulLinks()
	g.tickWallShield()
	g.tickProjectiles(dt)
	g.sweepDead()
	if g.Walls <= 0 && g.Phase != PhaseGameOver {
		g.Walls = 0
		g.endMatch(false)
	}
}

// addGold credits gold up to the wallet cap and returns what was actually
// added. The overflow goes to the common treasury.
func (g *Game) addGold(c *Chatter, amount int) int {
	room := g.Cfg.WalletCap - c.Gold
	add := min(amount, max(room, 0))
	c.Gold += add
	if over := amount - add; over > 0 {
		g.toTreasury(over)
	}
	return add
}

// chatter returns the chatter for key, creating one with starting gold.
func (g *Game) chatter(key, display string) *Chatter {
	c := g.Chatters[key]
	if c == nil {
		c = &Chatter{Name: display, Gold: g.Cfg.StartGold, Class: Classes[g.rng.IntN(len(Classes))].Key}
		g.Chatters[key] = c
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
	c.wasActive = true
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
		if c := g.Chatters[strings.ToLower(l)]; c != nil && c.Joined {
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
