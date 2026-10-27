package game

import "math"

// Class patch (0.14):
//   - Жрец: his pillar of light hits harder and stuns longer for every shrine
//     he owns;
//   - Следопыт: his ballistas reach a cell further and may land critical hits,
//     likelier after every boss wave held;
//   - Пиромант: the burning path hurts more, and a second skill: while he sits
//     in the tavern he mans a brazier on the wall by the door for a while, a
//     fire building that grows with the boss waves held and takes every buff
//     a building would;
//   - Друид: a bear form besides the panther: it runs down the path into the
//     first enemy and holds the line there, mauling everyone it fights; if the
//     wave already reached the door, it shoves the enemies back a cell and
//     fights them at the door, bosses too. One form at a time;
//   - Маг: a feast on the tavern tables: everyone gets the black potion's
//     boost, without getting drunk;
//   - Шаман: two spectral wolves race from the tavern down the path, through
//     the enemies, biting each once.

// bossesHeld is how many boss waves the tavern has held so far.
func (g *Game) bossesHeld() int {
	held := g.Wave
	if g.Phase == PhaseWave {
		held-- // the wave in progress is not held yet
	}
	if g.Cfg.BossEvery <= 0 || held <= 0 {
		return 0
	}
	return held / g.Cfg.BossEvery
}

// towerRange is how far t reaches: a ranger's ballistas a cell further.
func (g *Game) towerRange(t *Tower, spec TowerSpec) float64 {
	if t.Kind == "arrow" && g.classBonus(t.Owner, t.Kind) > 1 {
		return spec.Range + g.Cfg.RangerRangeBonus
	}
	return spec.Range
}

// critChance is the chance a ranger's ballista lands a critical hit.
func (g *Game) critChance(t *Tower) float64 {
	if t.Kind != "arrow" || g.classBonus(t.Owner, t.Kind) <= 1 {
		return 0
	}
	return math.Min(g.Cfg.CritMax, g.Cfg.CritBase+g.Cfg.CritPerBoss*float64(g.bossesHeld()))
}

// stormGrowth is how much harder a totem's lightning hits for the boss
// waves held, so it keeps up with the waves' growing HP late in the game.
func (g *Game) stormGrowth() float64 {
	return 1 + math.Min(g.Cfg.TotemBossMax, g.Cfg.TotemPerBoss*float64(g.bossesHeld()))
}

// tickSecond runs a class's second skill on its own timer, the first use
// halfway through like every skill. fire reports whether it went off; if
// not, it tries again in a few seconds.
func (g *Game) tickSecond(c *Chatter, every, dt float64, worth bool, fire func() bool) {
	if every <= 0 {
		return
	}
	if c.skill2CD == 0 {
		c.skill2CD = every / 2
	}
	c.skill2CD -= dt
	if c.skill2CD > 0 || !worth {
		return
	}
	if fire() {
		c.skill2CD = every
	} else {
		c.skill2CD = 3
	}
}

// GateFire is a pyromancer's brazier on the tavern wall by the door.
type GateFire struct {
	t    *Tower
	left float64
}

// gateCell is the cell just left of the field by the door, where the
// pyromancer's brazier stands on the tavern wall.
func (g *Game) gateCell() Pos {
	return Pos{-1, g.Map.Path[len(g.Map.Path)-1].Y}
}

// manGate (pyromancer) lights the brazier on the wall when enemies come
// near the door; until then it waits.
func (g *Game) manGate(key string, c *Chatter) bool {
	for _, f := range g.gateFires {
		if f.t.Owner == key {
			return true // already at the wall
		}
	}
	near := false
	for _, e := range g.Enemies {
		if !e.dead && (e.AtGate || e.Dist >= g.total-g.Cfg.Towers["fire"].Range-0.5) {
			near = true
			break
		}
	}
	if !near {
		return false
	}
	lvl := min(1+g.bossesHeld(), g.Cfg.Towers["fire"].MaxLevel)
	t := &Tower{ID: g.id(), Kind: "fire", Cell: g.gateCell(), Owner: key, Level: lvl, Built: true}
	g.gateFires = append(g.gateFires, &GateFire{t: t, left: g.Cfg.PyroGateSec})
	g.emit(Event{Kind: "gateFire", User: c.Name, Amount: lvl})
	return true
}

func (g *Game) tickGateFires(dt float64) {
	alive := g.gateFires[:0]
	for _, f := range g.gateFires {
		f.left -= dt
		if f.left <= 0 || g.Phase != PhaseWave {
			continue
		}
		g.shoot(f.t, g.Cfg.Towers[f.t.Kind], dt)
		alive = append(alive, f)
	}
	g.gateFires = alive
}

// feast (mage): dishes on every tavern table, and everyone in the game gets
// the black potion's boost to their buildings, without the drink in the blood.
func (g *Game) feast(c *Chatter) bool {
	d := g.Cfg.Drinks["potion"]
	cut := g.Now().Add(-activeWindow(g.Cfg))
	n := 0
	for key, o := range g.Chatters {
		if key == StreamerName || !o.Joined || !o.LastSeen.After(cut) {
			continue
		}
		o.buff, o.buffLeft, o.nextBuff = d.Boost, d.Sec, 0
		n++
	}
	g.emit(Event{Kind: "feast", User: c.Name, Amount: int(d.Sec)})
	return n > 0
}

// spiritWolves (shaman): two spectral wolves race from the door to the
// entrance through every enemy on the path, biting each once.
func (g *Game) spiritWolves(key string, c *Chatter) bool {
	if len(g.Enemies) == 0 {
		return false
	}
	for i := range 2 {
		g.Runs = append(g.Runs, &Run{Owner: key, Name: c.Name, Kind: "wolf", Pos: g.gate(), dist: g.total + 0.3*float64(i), hit: map[int]bool{}, lane: 0.2 - 0.4*float64(i)})
	}
	g.emit(Event{Kind: "wolves", User: c.Name})
	return true
}

// bearForm (druid): if the wave already reached the door, the bear shoves
// the enemies there a cell back and holds the door; otherwise it runs down
// the path into the first enemy and holds the line there.
func (g *Game) bearForm(key string, c *Chatter) bool {
	if len(g.Enemies) == 0 {
		return false
	}
	r := &Run{Owner: key, Name: c.Name, Kind: "bear", Pos: g.gate(), dist: g.total, hit: map[int]bool{}}
	atDoor := false
	for _, e := range g.Enemies {
		if !e.dead && !g.Cfg.Enemies[e.Kind].Fly && (e.AtGate || e.Dist >= g.total-0.6) {
			atDoor = true
		}
	}
	if atDoor {
		r.dist = g.total - 0.6
		for _, e := range g.Enemies {
			spec := g.Cfg.Enemies[e.Kind]
			if e.dead || spec.Fly || e.tavernLeft > 0 || e.Dist < g.total-1.2 || e.Kind == kingKind {
				continue // nothing shoves the king
			}
			// Bosses too: the bear is the one thing that moves them at the door.
			e.Dist = max(0, g.total-1.2-0.1*float64(len(r.hit)))
			e.AtGate, e.blocked = false, nil
			e.Pos = g.routePos(e.Dist)
			r.hit[e.ID] = true
			g.emit(Event{Kind: "shove", User: c.Name, X: e.Pos.X, Y: e.Pos.Y})
		}
		r.hold, r.left = true, g.Cfg.BearFightSec
	}
	r.Pos = g.routePos(r.dist)
	g.Runs = append(g.Runs, r)
	g.emit(Event{Kind: "bear", User: c.Name, Amount: map[bool]int{true: 1, false: 0}[atDoor]})
	return true
}

// tickBear runs the bear down the path until it meets an enemy, then holds
// the line there for BearFightSec, mauling everyone within reach. It reports
// whether the bear is still out.
func (g *Game) tickBear(r *Run, dt float64) bool {
	if !r.hold {
		r.dist -= g.Cfg.BearSpeed * dt
		for _, e := range g.Enemies {
			spec := g.Cfg.Enemies[e.Kind]
			if !e.dead && !spec.Fly && !spec.Ghost && e.tavernLeft <= 0 && e.Dist >= r.dist-0.2 {
				r.hold, r.left = true, g.Cfg.BearFightSec
				r.dist = max(0, min(r.dist, e.Dist)+0.5) // stand in front of it
				break
			}
		}
		if r.dist <= 0 {
			return false // ran the whole path: nobody to fight
		}
		r.Pos = g.routePos(r.dist)
		return true
	}
	r.left -= dt
	if r.left <= 0 {
		g.emit(Event{Kind: "bearDone", User: r.Name, X: r.Pos.X, Y: r.Pos.Y})
		return false
	}
	for _, e := range g.Enemies {
		if !e.dead && e.Pos.sub(r.Pos).len() <= g.Cfg.BearReach {
			g.damage(e, g.skillHP()*g.Cfg.BearDPS*dt, r.Owner, false)
		}
	}
	return true
}

// heldByBear stops an enemy walking into a bear that holds the line, like a
// barricade with no strength to lose; flyers and ghosts pass.
func (g *Game) heldByBear(e *Enemy, spec EnemySpec, to float64) float64 {
	if spec.Fly || spec.Ghost {
		return to
	}
	for _, r := range g.Runs {
		if r.Kind != "bear" || !r.hold {
			continue
		}
		if edge := r.dist - 0.4; e.Dist <= edge+0.01 && to > edge {
			to = edge
		}
	}
	return to
}

// tickWolf races a spectral wolf down the path, biting each enemy once.
func (g *Game) tickWolf(r *Run, dt float64) bool {
	from := r.dist
	r.dist -= g.Cfg.WolfSpeed * dt
	for _, e := range g.Enemies {
		if e.dead || r.hit[e.ID] || e.tavernLeft > 0 || e.Dist < r.dist-0.3 || e.Dist > from+0.3 {
			continue
		}
		r.hit[e.ID] = true
		g.damage(e, g.skillHP()*g.Cfg.WolfDmg, r.Owner, true)
		g.emit(Event{Kind: "wolfBite", User: r.Name, X: e.Pos.X, Y: e.Pos.Y})
	}
	if r.dist <= 0 {
		return false
	}
	p := g.routePos(r.dist)
	r.Pos = vec{p.X, p.Y + r.lane}
	return true
}
