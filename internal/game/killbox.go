package game

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
)

// The killbox demo (the server's -killbox flag, for development): finals
// against the King in Yellow one after another, the way a strong chat
// plays them. Every team of five builds all its limit at the artifact
// level around one turn of the path: shooters on the cells that see the
// most of it, the warrior's barricades and the rogue's traps on it, the
// warlock's altars in pairs across it. In the fight the players drink
// (potions, ale, mead, never past passing out), redeem the rain of arrows,
// a round and wall repairs for channel points, wipe the Yellow Signs,
// mend their barricades. When the final is over the result goes to the
// log and the chat, and the next team takes the next biome.

// KillboxTeams are the teams the demo cycles through by default.
var KillboxTeams = [][]string{
	{"ranger", "warlock", "warrior", "mage", "pyro"},
	{"ranger", "warlock", "warrior", "cleric", "bard"},
	{"ranger", "ranger", "warlock", "warrior", "shaman"},
	{"ranger", "rogue", "warlock", "mage", "druid"},
	{"ranger", "warrior", "pyro", "shaman", "cleric"},
	{"ranger", "warlock", "bard", "mage", "rogue"},
}

const (
	killboxWindow = 5    // path cells on each side of the turn that make the killbox
	killboxPause  = 12.0 // seconds the summary stays up before the next final
	killboxGold   = 3000 // each player's gold for drinks and repairs
	killboxSupMax = 2    // a cleric or bard builds this many of their own, the rest are archers
)

// killbox runs the demo across finals.
type killbox struct {
	teams  [][]string
	biome  Biome // "" for every biome in turn
	wave   int   // the waves held before the final (the tower limit and the king's health follow it)
	report func(string)
	points bool // the players redeem channel point rewards
	run    int
	wait   float64 // seconds of the summary left
	f      *kbFight
	tally  map[string]*kbTally // by team
}

type kbTally struct {
	games, wins int
	fights      []float64
}

// kbFight is one final.
type kbFight struct {
	team    []string
	biome   Biome
	members []kbMember
	clock   float64
	start   float64 // clock when the king stepped in, -1 before
	phase   [4]float64
	seen    bool // the king has been on the field
	killed  bool
	hpLeft  float64
	signs   map[*Sign]map[string]float64 // each player's reaction to each sign
	signN   int
	wiped   int
	drinks  int
	strikes int
	rounds  int
	repairs int
	wallFix float64   // clock of the last walls reward
	falls   []float64 // seconds into the fight he fell at the end of each act but the last
	carries int
}

type kbMember struct {
	key, name, class string
	strikeAt, chatAt float64
}

// KillboxDemo starts the demo: teams in turn (KillboxTeams when empty), in
// biome or every biome in turn, the final after this many waves held; with
// points the players also redeem channel point rewards.
func (g *Game) KillboxDemo(biome Biome, teams [][]string, wave int, points bool, report func(string)) {
	if len(teams) == 0 {
		teams = KillboxTeams
	}
	if report == nil {
		report = func(string) {}
	}
	g.killbox = &killbox{teams: teams, biome: biome, wave: wave, points: points, report: report, tally: map[string]*kbTally{}}
	g.killbox.next(g)
}

// next sets up the next final.
func (k *killbox) next(g *Game) {
	team := k.teams[k.run%len(k.teams)]
	b := k.biome
	if b == "" {
		all := []Biome{BiomeRuins, BiomeDungeon, BiomeCrypt, BiomeIce, BiomeSwamp, BiomeRiver}
		b = all[(k.run+k.run/len(k.teams))%len(all)] // every team meets every biome
	}
	k.run++
	k.f = g.killboxSetup(b, team, k.wave)
	k.report(fmt.Sprintf("killbox #%d: %s, %s, limit %d", k.run, strings.Join(team, ","), b, g.TowerLimit()))
}

// killboxSetup builds the team's killbox and lets the final come.
func (g *Game) killboxSetup(biome Biome, team []string, wave int) *kbFight {
	g.NewGame(biome)
	f := &kbFight{team: team, biome: biome, start: -1, phase: [4]float64{-1, -1, -1, -1}, signs: map[*Sign]map[string]float64{}}
	used := map[string]int{}
	for _, cl := range team {
		name := demoNames[cl]
		if used[cl]++; used[cl] > 1 {
			name = fmt.Sprintf("%s%d", name, used[cl])
		}
		g.HandleChat(ChatMessage{User: name, Text: "!play"})
		key, c := g.findChatter(name)
		if c == nil {
			continue
		}
		c.Class, c.Gold, c.Drunk = cl, killboxGold, 0 // sober, as a new player would come
		f.members = append(f.members, kbMember{key: key, name: name, class: cl, chatAt: 20 + g.rng.Float64()*30})
	}
	g.Wave = wave
	limit := g.TowerLimit()
	path := g.Map.Path
	turn := g.killboxTurn()
	lo, hi := max(1, turn-killboxWindow), min(len(path)-2, turn+killboxWindow)
	box := path[lo : hi+1]
	seen := func(p Pos, r float64) int { // the killbox's path cells within r of p
		n := 0
		for _, q := range box {
			if center(q).sub(center(p)).len() <= r {
				n++
			}
		}
		return n
	}
	free := func(p Pos) bool {
		return p.X >= 0 && p.Y >= 0 && p.X < GridW && p.Y < GridH && g.Map.Grid.At(p) == CellBuild && g.Towers[p] == nil
	}
	put := func(m kbMember, kind string, at Pos, art bool) {
		spec := g.Cfg.Towers[kind]
		lvl := spec.MaxLevel
		if art {
			lvl++
		}
		t := &Tower{ID: g.id(), Kind: kind, Cell: at, Owner: m.key, Level: lvl, Built: true}
		g.Towers[at] = t
		g.readyTower(t)
		if kind == "arrow" && g.artifact(t) {
			t.Ammo = []string{ammoFire, ammoPoison, ammoBlast}[g.rng.IntN(3)] // as a player would pick
		}
	}
	// best is the free build cell that sees most of the killbox from r,
	// the nearest to the turn among equals.
	best := func(r float64) (Pos, bool) {
		var at Pos
		n, d := 0, math.Inf(1)
		for y := 0; y < GridH; y++ {
			for x := 0; x < GridW; x++ {
				p := Pos{x, y}
				if !free(p) {
					continue
				}
				s, l := seen(p, r), center(p).sub(center(path[turn])).len()
				if s > n || s == n && s > 0 && l < d {
					at, n, d = p, s, l
				}
			}
		}
		if n == 0 { // nothing left near the turn: the cell that sees most of the rest of the path, the nearest among equals
			for y := 0; y < GridH; y++ {
				for x := 0; x < GridW; x++ {
					p := Pos{x, y}
					if !free(p) {
						continue
					}
					s := 0
					for _, q := range path {
						if center(q).sub(center(p)).len() <= r {
							s++
						}
					}
					if l := center(p).sub(center(path[turn])).len(); s > n || s == n && l < d {
						at, n, d = p, s, l
					}
				}
			}
		}
		return at, !math.IsInf(d, 1)
	}
	// What each player builds: their own building as artifacts, but a
	// cleric or a bard only a couple and archers after; a druid has none.
	var orders []kbOrder
	for _, m := range f.members {
		spec, _ := classByKey(m.class)
		for i := 0; i < limit; i++ {
			switch {
			case spec.Tower == "" || (spec.Tower == "shrine" || spec.Tower == "drums") && i >= killboxSupMax:
				orders = append(orders, kbOrder{m, "arrow", false})
			default:
				orders = append(orders, kbOrder{m, spec.Tower, true})
			}
		}
	}
	// On the path first: barricades from the far end of the killbox back,
	// so he hacks at them inside it, then traps on what is left of it.
	onPath := slices.Clone(box)
	slices.Reverse(onPath)
	for _, o := range orders {
		role := g.Cfg.Towers[o.kind].Role
		if role != RoleWall && role != RoleTrap {
			continue
		}
		for _, p := range onPath {
			if g.Towers[p] == nil {
				put(o.m, o.kind, p, o.art)
				break
			}
		}
	}
	// Altars in pairs across the killbox, so their soul links net it.
	for _, o := range orders {
		if o.kind != "curse" || g.kindOf(o.m.key, "curse") >= limit {
			continue
		}
		done := false
		for _, p := range box {
			for _, d := range []Pos{{1, 0}, {0, 1}} {
				a, b := Pos{p.X - d.X, p.Y - d.Y}, Pos{p.X + d.X, p.Y + d.Y}
				if free(a) && free(b) && g.kindOf(o.m.key, "curse")+2 <= limit {
					put(o.m, "curse", a, o.art)
					put(o.m, "curse", b, o.art)
					done = true
					break
				}
			}
			if done {
				break
			}
		}
	}
	// Then shooters, the artifact ones first, then auras, then archers.
	rank := func(o kbOrder) int {
		switch {
		case o.art && g.Cfg.Towers[o.kind].Role == RoleShooter:
			return 0
		case o.kind == "curse":
			return 1
		case o.art:
			return 2
		}
		return 3
	}
	// Within a rank the shorter reach goes first and takes the cells nearest
	// the turn: a brazier must stand close, a ballista reaches from further.
	reach := func(o kbOrder) float64 { return g.Cfg.Towers[o.kind].Range }
	slices.SortStableFunc(orders, func(a, b kbOrder) int {
		if r := rank(a) - rank(b); r != 0 {
			return r
		}
		return cmp.Compare(reach(a), reach(b))
	})
	for _, o := range orders {
		role := g.Cfg.Towers[o.kind].Role
		if role == RoleWall || role == RoleTrap {
			continue
		}
		if g.kindOf(o.m.key, o.kind) >= countOrders(orders, o.m.key, o.kind) {
			continue // altars already placed in pairs
		}
		r := g.Cfg.Towers[o.kind].Range
		if r <= 0 {
			r = max(g.Cfg.Towers[o.kind].AuraRange, 1.6)
		}
		if at, ok := best(r); ok {
			put(o.m, o.kind, at, o.art)
		}
	}
	g.finalDue = true
	g.matchElapsed = g.Cfg.MatchMinutes * 60
	g.Phase, g.Timer = PhaseBuild, 5
	return f
}

// kbOrder is a building a player of the demo is to put up.
type kbOrder struct {
	m    kbMember
	kind string
	art  bool
}

// countOrders is how many buildings of kind key is to build.
func countOrders(orders []kbOrder, key, kind string) int {
	n := 0
	for _, o := range orders {
		if o.m.key == key && o.kind == kind {
			n++
		}
	}
	return n
}

// killboxTurn is the path cell, away from both ends, with the most build
// cells around it that see the most of the path near it: the turn a chat
// would wall in.
func (g *Game) killboxTurn() int {
	path := g.Map.Path
	bestI, bestN := len(path)/2, -1
	for i := len(path) / 4; i < len(path)*3/4; i++ {
		n := 0
		for y := 0; y < GridH; y++ {
			for x := 0; x < GridW; x++ {
				p := Pos{x, y}
				if g.Map.Grid.At(p) != CellBuild {
					continue
				}
				for j := max(0, i-killboxWindow); j <= min(len(path)-1, i+killboxWindow); j++ {
					if center(path[j]).sub(center(p)).len() <= 2.6 {
						n++
					}
				}
			}
		}
		if n > bestN {
			bestI, bestN = i, n
		}
	}
	return bestI
}

// tickKillbox plays the demo's players and moves on to the next final.
func (g *Game) tickKillbox(dt float64) {
	k := g.killbox
	f := k.f
	if g.Phase == PhaseGameOver {
		if k.wait <= 0 {
			k.finish(g)
			k.wait = killboxPause
		}
		g.Timer = max(g.Timer, 1) // the demo starts the next game itself
		if k.wait -= dt; k.wait <= 0 {
			k.next(g)
		}
		return
	}
	g.killboxFight(f, k.points, dt)
}

// killboxFight plays the team in the final: they chat, drink, redeem the
// rewards (with points), wipe the Yellow Signs, mend barricades and carry
// their buildings off burnt ground.
func (g *Game) killboxFight(f *kbFight, points bool, dt float64) {
	f.clock += dt
	for i := range f.members {
		m := &f.members[i]
		if f.clock >= m.chatAt { // people chat now and then, staying in the tavern
			g.HandleChat(ChatMessage{User: m.name, Text: []string{"gg", "держим поворот", "пьём!", "+"}[g.rng.IntN(4)]})
			m.chatAt = f.clock + 40 + g.rng.Float64()*30
		}
	}
	if g.kingRest != nil { // between his acts: carry off the burnt ground
		if len(f.falls) < g.kingRest.kingAct {
			f.falls = append(f.falls, f.clock-f.start)
		}
		g.killboxCarry(f, g.kingRest)
		return
	}
	king := g.king()
	if king == nil {
		if f.seen && !f.killed && g.Phase == PhaseWave {
			f.killed = true
			f.phase[0] = f.clock - f.start
		}
		return
	}
	if !f.seen {
		f.seen, f.start = true, f.clock
		for i := range f.members { // the chat notices him one by one
			f.members[i].strikeAt = f.clock + 2 + g.rng.Float64()*12
		}
	}
	f.hpLeft = g.kingShare(king)
	g.killboxCarry(f, king)
	for p := 2; p <= king.phase; p++ {
		if f.phase[p] < 0 {
			f.phase[p] = f.clock - f.start
		}
	}
	g.killboxSigns(f)
	for i := range f.members {
		m := &f.members[i]
		_, c := g.findChatter(m.name)
		if c == nil {
			continue
		}
		g.killboxDrink(f, m, c)
		if f.clock >= m.strikeAt && g.strikeKey != g.strikeNow() { // the chat sees whether it fell this act
			m.strikeAt = f.clock + 30 + g.rng.Float64()*25
			if t := g.rewardTitle("strike", points); t != "" {
				if !g.Reward(m.key, m.name, t) {
					f.strikes++
				}
			}
		}
		if m.class == "warrior" {
			g.killboxMend(f, m, c)
		}
	}
	if f.rounds == 0 && f.clock-f.start > 25 {
		if t := g.rewardTitle("round", points); t != "" {
			m := f.members[g.rng.IntN(len(f.members))]
			g.Reward(m.key, m.name, t)
			f.rounds++
		}
	}
	if g.Walls < 0.4*g.Cfg.WallsHP && f.clock-f.wallFix > 30 {
		if t := g.rewardTitle("repair", points); t != "" {
			m := f.members[g.rng.IntN(len(f.members))]
			g.Reward(m.key, m.name, t)
			f.wallFix = f.clock
		}
	}
}

// killboxDrink keeps a player's buildings boosted: a potion while it will
// not knock them out, ale while it will not either, a mead now and then.
func (g *Game) killboxDrink(f *kbFight, m *kbMember, c *Chatter) {
	if c.out > 0 || g.Workers[m.key] != nil || c.buffLeft > 0 && c.buff >= 1 {
		return
	}
	room := g.Cfg.PassOutMugs - 1 - math.Ceil(c.Drunk)
	if g.Cfg.PassOutMugs <= 0 {
		room = 99
	}
	kind := ""
	switch {
	case room >= float64(g.Cfg.Drinks["potion"].Mugs):
		kind = "зелье"
	case room >= float64(g.Cfg.Drinks["mead"].Mugs) && g.rng.IntN(3) == 0:
		kind = "медовуха"
	case room >= float64(g.Cfg.Drinks["ale"].Mugs) && c.buffLeft <= 0:
		kind = "эль"
	}
	if kind != "" {
		before := c.Drunk
		g.HandleChat(ChatMessage{User: m.name, Text: "!выпить " + kind})
		if c.Drunk > before { // served, not refused
			f.drinks++
		}
	}
}

// killboxMend sends the warrior to a barricade that is half gone.
func (g *Game) killboxMend(f *kbFight, m *kbMember, c *Chatter) {
	if g.Workers[m.key] != nil {
		return
	}
	for _, t := range g.Towers {
		if t.Owner == m.key && t.Built && !t.repairing && t.repairCD <= 0 && g.Cfg.Towers[t.Kind].Role == RoleWall && t.HP < 0.5*t.MaxHP {
			g.HandleChat(ChatMessage{User: m.name, Text: "!чинить " + t.Cell.Name()})
			if t.repairing {
				f.repairs++
			}
			return
		}
	}
}

// killboxSigns wipes the Yellow Signs: every player sees a sign after a
// few seconds of their own and goes to wipe it.
func (g *Game) killboxSigns(f *kbFight) {
	live := map[*Sign]bool{}
	for _, s := range slices.Clone(g.signs) { // a wipe takes the sign out of g.signs
		if s == nil {
			continue
		}
		live[s] = true
		r := f.signs[s]
		if r == nil {
			r = map[string]float64{}
			for _, m := range f.members {
				r[m.name] = f.clock + 2 + g.rng.Float64()*8
			}
			f.signs[s] = r
			f.signN++
		}
		for name, at := range r {
			if at > 0 && f.clock >= at {
				g.HandleChat(ChatMessage{User: name, Text: "!рз " + s.Tower.Cell.Name()})
				r[name] = 0
			}
		}
	}
	for s := range f.signs {
		if !live[s] {
			if s.Left > 0 {
				f.wiped++
			}
			delete(f.signs, s)
		}
	}
}

// rewardTitle is the title of the first channel point reward with this action,
// "" when there is none or the players play without points.
func (g *Game) rewardTitle(action string, points bool) string {
	if !points {
		return ""
	}
	for _, r := range g.Cfg.Rewards {
		if r.Action == action {
			return r.Title
		}
	}
	return ""
}

// finish reports the final that just ended.
func (k *killbox) finish(g *Game) {
	f := k.f
	won := g.Won()
	name := strings.Join(f.team, ",")
	t := k.tally[name]
	if t == nil {
		t = &kbTally{}
		k.tally[name] = t
	}
	t.games++
	fight := f.phase[0]
	if !f.killed && f.start >= 0 {
		fight = f.clock - f.start
	}
	if won {
		t.wins++
		t.fights = append(t.fights, fight)
	}
	at := func(s float64) string {
		if s < 0 {
			return "—"
		}
		return fmt.Sprintf("%.0f с", s)
	}
	acts := ""
	for i, s := range f.falls {
		acts += fmt.Sprintf("акт %d пал на %.0f с; ", i+1, s)
	}
	res := "поражение, у Короля " + fmt.Sprintf("%.0f%%", 100*f.hpLeft)
	if won {
		res = "победа"
	}
	line := fmt.Sprintf("Килбокс #%d (%s, %s): %s за %.0f с; %sфаза II %s, III %s; стены %.0f%%; выпито %d, дождей %d, знаков %d (стёрто %d), починок %d, переносов %d. Счёт команды: %d/%d",
		k.run, name, f.biome, res, fight, acts, at(f.phase[2]), at(f.phase[3]), 100*g.Walls/g.Cfg.WallsHP,
		f.drinks, f.strikes, f.signN, f.wiped, f.repairs, f.carries, t.wins, t.games)
	if len(t.fights) > 0 {
		s := slices.Clone(t.fights)
		slices.Sort(s)
		line += fmt.Sprintf(", медиана победного боя %.0f с", s[len(s)/2])
	}
	k.report(line)
	g.reply("", line)
}

// killboxCarry has every player whose mercenary is free carry their best
// building off burnt ground to the free cell that sees most of the route
// still ahead of the king.
func (g *Game) killboxCarry(f *kbFight, king *Enemy) {
	if len(g.burned) == 0 {
		return
	}
	for i := range f.members {
		m := &f.members[i]
		if g.Workers[m.key] != nil {
			continue
		}
		var from *Tower
		for _, t := range g.Towers {
			if t.Owner != m.key || !g.burned[t.Cell] || !t.Built || t.moveTo != nil || t.packed || g.Cfg.Towers[t.Kind].OnPath() {
				continue
			}
			if from == nil || t.Level > from.Level || t.Level == from.Level && t.ID < from.ID {
				from = t
			}
		}
		if from == nil {
			continue
		}
		r := g.Cfg.Towers[from.Kind].Range
		if r <= 0 {
			r = max(g.Cfg.Towers[from.Kind].AuraRange, 1.6)
		}
		var to Pos
		best := 0
		for y := 0; y < GridH; y++ {
			for x := 0; x < GridW; x++ {
				p := Pos{x, y}
				if g.Map.Grid.At(p) != CellBuild || g.Towers[p] != nil || g.burned[p] || g.carriedTo(p) != nil {
					continue
				}
				n := 0
				for j, q := range g.Map.Path {
					if g.cellDist[j] >= king.Dist && center(q).sub(center(p)).len() <= r {
						n++
					}
				}
				if n > best {
					to, best = p, n
				}
			}
		}
		if best == 0 {
			continue
		}
		g.HandleChat(ChatMessage{User: m.name, Text: "!перенести " + from.Cell.Name() + " " + to.Name()})
		if from.moveTo != nil {
			f.carries++
		}
	}
}
