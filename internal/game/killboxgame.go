package game

import (
	"fmt"
	"math"
)

// Killbox players for whole games (cmd/balance -killbox): a team of five
// plays from the first wave the way a chat set on walling in one turn
// would. Each player takes a class and builds, as the limit grows, what the
// killbox demo gives them at once: their class building, but a cleric or a
// bard only a couple and archers after, a druid archers only; barricades and
// traps on the path inside the killbox from its far end back, altars in
// pairs across it, shooters on the cells that see most of it (a long reach
// leaving the cells nearest the turn to the short ones). With nothing to
// build they raise the lowest of their buildings, their class building up
// to the artifact level. On boss waves they drink and, with points, redeem
// the rain of arrows; with shrines every player also puts up shrines to
// their shrine limit, far from the turn (their buff reaches the whole map).
// In the final they play as in the demo (see
// killboxFight).

// KillboxResult is how a whole killbox game went.
type KillboxResult struct {
	Waves       int     // waves held
	Minutes     float64 // of the match
	Final, Won  bool
	KingLeft    float64   // share of his health (all acts) left when the walls fell
	Fight       float64   // seconds from his first step to his death or the walls'
	Falls       []float64 // seconds into the fight each act but the last ended
	Phase2      float64   // seconds into the fight phase II began, -1 never
	Phase3      float64
	Walls       float64 // share of the walls left
	Towers      int     // the team's buildings when he came
	Arts        int     // of them at the artifact level
	Level       float64 // their average level
	InBox       float64 // share of them by the killbox
	Drinks      int     // drunk in the final
	BossDrinks  int     // drunk on boss waves before it
	Strikes     int     // rains of arrows in the final
	BossStrikes int     // before it
	Signs       int
	Wiped       int
	Repairs     int
	Carries     int
	Acts        int // the King's acts in the final, 0 with none
	Advice      int // the acts advised for the team
}

type kbGame struct {
	team    []string
	points  bool
	shrines bool // every player also builds shrines up to their shrine limit
	f       *kbFight
	turn    int
	box     []Pos
	act     []float64 // each member's next move
	pair    []*Pos    // the cell of a member's second altar, still to build
	res     KillboxResult
	shot    bool // the team's buildings were counted when he came
	clock   float64
	strike  float64 // clock of the next rain on a boss wave
	vote    int     // with toVote: the acts each member votes for, the advice plus this
	toVote  bool
}

// KillboxGame lets the team of killbox players into a fresh game.
func (g *Game) KillboxGame(team []string, points, shrines bool) {
	k := &kbGame{team: team, points: points, shrines: shrines, f: &kbFight{team: team, start: -1, phase: [4]float64{-1, -1, -1, -1}, signs: map[*Sign]map[string]float64{}}}
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
		if spec, ok := classByKey(cl); ok && c.Class != cl {
			g.HandleChat(ChatMessage{User: name, Text: "!класс " + spec.Aliases[0]})
		}
		k.f.members = append(k.f.members, kbMember{key: key, name: name, class: cl, chatAt: g.rng.Float64() * 30})
		k.act = append(k.act, 3+g.rng.Float64()*10)
		k.pair = append(k.pair, nil)
	}
	k.turn = g.killboxTurn()
	path := g.Map.Path
	k.box = path[max(1, k.turn-killboxWindow) : min(len(path)-2, k.turn+killboxWindow)+1]
	g.kbGame = k
}

// KillboxReport is how the game of the killbox players went so far.
func (g *Game) KillboxReport() KillboxResult {
	k := g.kbGame
	if !k.shot { // he never came: the buildings as they stand now
		k.shot = true
		k.count(g)
	}
	r := k.res
	f := k.f
	r.Waves, r.Minutes = max(g.Wave-1, 0), k.clock/60
	r.Final, r.Won = g.FinalReached(), g.Won()
	if r.Final {
		r.Acts, r.Advice = g.kingActs(), g.actsAdvice
	}
	r.Fight = f.phase[0]
	if !f.killed && f.start >= 0 {
		r.Fight = f.clock - f.start
	}
	if !f.seen {
		r.Fight = 0
	}
	r.Falls = f.falls
	r.Phase2, r.Phase3 = f.phase[2], f.phase[3]
	if !r.Won && f.seen {
		r.KingLeft = f.hpLeft
	}
	r.Walls = g.Walls / g.Cfg.WallsHP
	r.Drinks, r.Strikes, r.Signs, r.Wiped, r.Repairs, r.Carries = f.drinks, f.strikes, f.signN, f.wiped, f.repairs, f.carries
	return r
}

// KillboxPlay plays the killbox players for dt seconds: call it before Tick.
func (g *Game) KillboxPlay(dt float64) {
	k := g.kbGame
	if k == nil || g.Phase == PhaseGameOver {
		return
	}
	k.clock += dt
	if g.finalWave {
		if !k.shot && g.king() != nil {
			k.shot = true
			k.count(g)
		}
		g.killboxFight(k.f, k.points, dt)
	} else {
		for i := range k.f.members {
			m := &k.f.members[i]
			if k.clock >= m.chatAt { // people chat now and then, staying in the tavern
				g.HandleChat(ChatMessage{User: m.name, Text: []string{"gg", "строим поворот", "+", "лол"}[g.rng.IntN(4)]})
				m.chatAt = k.clock + 40 + g.rng.Float64()*30
			}
		}
		if k.toVote && g.actVoteOpen() && g.Phase == PhaseBuild {
			k.toVote = false // once: they vote as the call goes out
			advice, _, _ := g.actsNow()
			for _, m := range k.f.members {
				g.HandleChat(ChatMessage{User: m.name, Text: fmt.Sprintf("!акты %d", advice+k.vote)})
			}
		}
		k.bossWave(g)
	}
	for i := range k.f.members {
		if k.clock >= k.act[i] {
			k.act[i] = k.clock + 4 + g.rng.Float64()*8
			k.spend(g, i)
		}
	}
}

// bossWave: with a boss on the field the players drink and one of them
// redeems the rain of arrows.
func (k *kbGame) bossWave(g *Game) {
	if g.Phase != PhaseWave {
		return
	}
	boss := false
	for _, e := range g.Enemies {
		if g.Cfg.Enemies[e.Kind].Boss && !e.dead {
			boss = true
		}
	}
	if !boss {
		k.strike = 0
		return
	}
	for i := range k.f.members {
		m := &k.f.members[i]
		if _, c := g.findChatter(m.name); c != nil {
			before := k.f.drinks
			g.killboxDrink(k.f, m, c)
			k.res.BossDrinks += k.f.drinks - before
			k.f.drinks = before
		}
	}
	if k.strike == 0 {
		k.strike = k.clock + 3 + g.rng.Float64()*10 // the chat notices the boss
	}
	if k.clock >= k.strike && g.strikeKey != g.strikeNow() {
		if t := g.rewardTitle("strike", k.points); t != "" {
			m := k.f.members[g.rng.IntN(len(k.f.members))]
			if !g.Reward(m.key, m.name, t) {
				k.res.BossStrikes++
			}
		}
	}
}

// count takes stock of the team's buildings when the king comes.
func (k *kbGame) count(g *Game) {
	levels := 0
	near := 0
	for _, t := range g.Towers {
		if !k.ours(t.Owner) {
			continue
		}
		k.res.Towers++
		levels += t.Level
		if t.Level > g.Cfg.Towers[t.Kind].MaxLevel {
			k.res.Arts++
		}
		for _, q := range k.box {
			if center(q).sub(center(t.Cell)).len() <= 2.6 {
				near++
				break
			}
		}
	}
	if k.res.Towers > 0 {
		k.res.Level = float64(levels) / float64(k.res.Towers)
		k.res.InBox = float64(near) / float64(k.res.Towers)
	}
}

func (k *kbGame) ours(key string) bool {
	for _, m := range k.f.members {
		if m.key == key {
			return true
		}
	}
	return false
}

// want is what player i builds as their n-th building.
func (k *kbGame) want(i, n int) string {
	spec, _ := classByKey(k.f.members[i].class)
	if spec.Tower == "" || (spec.Tower == "shrine" || spec.Tower == "drums") && n >= killboxSupMax {
		return "arrow"
	}
	return spec.Tower
}

// spend has player i build their next building or raise one.
func (k *kbGame) spend(g *Game, i int) {
	m := k.f.members[i]
	c := g.Chatters[m.key]
	if c == nil || c.out > 0 || !g.canQueue(m.key) {
		return
	}
	budget := c.Gold + c.Grant
	mine := 0
	for _, t := range g.Towers {
		if t.Owner != m.key {
			continue
		}
		mine++
		if t.Kind == "arrow" && g.artifact(t) && t.Ammo == "" {
			g.HandleChat(ChatMessage{User: m.name, Text: "!стрелы " + t.Cell.Name() + " " + []string{"огонь", "яд", "взрыв"}[g.rng.IntN(3)]})
		}
	}
	if m.class == "warrior" && g.king() == nil {
		g.killboxMend(k.f, &k.f.members[i], c)
	}
	if mine < g.TowerLimit() && !g.finalWave {
		kind := k.want(i, mine)
		if k.shrines && mine > 0 && kind != "shrine" && g.kindOf(m.key, "shrine") < g.shrineLimit(c) {
			kind = "shrine" // their buff reaches the whole map, and they share the bounty
		}
		at, ok := k.cell(g, i, kind)
		if !ok && kind != "arrow" { // no room for it left: an archer post instead
			kind = "arrow"
			at, ok = k.cell(g, i, kind)
		}
		if ok {
			if budget >= g.Cfg.TowerCost(kind, 1) {
				g.HandleChat(ChatMessage{User: m.name, Text: "!build " + at.Name() + " " + kind})
			}
			return // or saving up for it
		}
	}
	// The lowest of their buildings that can still grow, their class
	// building first among equals.
	spec, _ := classByKey(m.class)
	var best *Tower
	for _, t := range g.Towers {
		if t.Owner != m.key || !t.Built || t.PendingCost > 0 || t.repairing || t.moveTo != nil || t.Level >= g.maxLevel(t) {
			continue
		}
		if best == nil || t.Level < best.Level || t.Level == best.Level && t.Kind == spec.Tower && best.Kind != spec.Tower {
			best = t
		}
	}
	if best != nil && budget >= g.Cfg.TowerCost(best.Kind, best.Level+1) {
		g.HandleChat(ChatMessage{User: m.name, Text: "!up " + best.Cell.Name()})
	}
}

// cell is where player i puts a building of kind.
func (k *kbGame) cell(g *Game, i int, kind string) (Pos, bool) {
	free := func(p Pos) bool {
		return p.X >= 0 && p.Y >= 0 && p.X < GridW && p.Y < GridH && g.Map.Grid.At(p) == CellBuild && g.Towers[p] == nil && g.carriedTo(p) == nil
	}
	spec := g.Cfg.Towers[kind]
	if spec.OnPath() { // barricades and traps on the killbox from its far end back
		for j := len(k.box) - 1; j >= 0; j-- {
			if p := k.box[j]; g.Towers[p] == nil {
				return p, true
			}
		}
		return Pos{}, false
	}
	if kind == "curse" { // altars in pairs across the killbox
		if p := k.pair[i]; p != nil {
			k.pair[i] = nil
			if free(*p) {
				return *p, true
			}
		}
		for _, p := range k.box {
			for _, d := range []Pos{{1, 0}, {0, 1}} {
				a, b := Pos{p.X - d.X, p.Y - d.Y}, Pos{p.X + d.X, p.Y + d.Y}
				if free(a) && free(b) {
					k.pair[i] = &b
					return a, true
				}
			}
		}
	}
	if kind == "shrine" && k.shrines { // a shrine helps from anywhere: the free cell furthest from the turn
		var at Pos
		far := -1.0
		for y := 0; y < GridH; y++ {
			for x := 0; x < GridW; x++ {
				if p := (Pos{x, y}); free(p) && center(p).sub(center(g.Map.Path[k.turn])).len() > far {
					at, far = p, center(p).sub(center(g.Map.Path[k.turn])).len()
				}
			}
		}
		return at, far >= 0
	}
	r := spec.Range
	if r <= 0 {
		r = max(spec.AuraRange, 1.6)
	}
	long := r >= 2.4 // a long reach takes the further of the best cells
	turn := center(g.Map.Path[k.turn])
	var at Pos
	n, d := 0, math.Inf(1)
	for y := 0; y < GridH; y++ {
		for x := 0; x < GridW; x++ {
			p := Pos{x, y}
			if !free(p) {
				continue
			}
			s := 0
			for _, q := range k.box {
				if center(q).sub(center(p)).len() <= r {
					s++
				}
			}
			l := center(p).sub(turn).len()
			if long {
				l = -l
			}
			if s > n || s == n && s > 0 && l < d {
				at, n, d = p, s, l
			}
		}
	}
	if n > 0 {
		return at, true
	}
	// Nothing left by the turn: the cell that sees most of the path.
	for y := 0; y < GridH; y++ {
		for x := 0; x < GridW; x++ {
			p := Pos{x, y}
			if !free(p) {
				continue
			}
			s := 0
			for _, q := range g.Map.Path {
				if center(q).sub(center(p)).len() <= r {
					s++
				}
			}
			if s > n {
				at, n = p, s
			}
		}
	}
	return at, n > 0
}

// KillboxVote makes the team vote for the advised acts plus off when the
// vote opens (0 or no call: the advice plays as nobody votes).
func (g *Game) KillboxVote(off int) {
	if k := g.kbGame; k != nil {
		k.vote, k.toVote = off, true
	}
}
