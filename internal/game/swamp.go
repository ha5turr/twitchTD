package game

import "fmt"

// Patch 11: the swamp. Slimes split when they die, bats fly straight to the
// tavern over every bend, goblin shamans wash their neighbours clean of
// nets, burns and curses. Its bosses: the swamp witch turns buildings into
// frogs for a while, and the hydra grows a head at every quarter of its
// health it loses, unless the stump is burning.

const (
	hydraHeads = 3   // heads the hydra is born with
	hexSec     = 8.0 // seconds a building stays a frog
	hexRadius  = 3.5 // the witch reaches this far
	dispelSec  = 4.0 // a goblin shaman cleans his neighbours this often
)

// flyPos is where a flyer is: on the straight line from the spawn point to
// the gate, as far along as its Dist is along the route.
func (g *Game) flyPos(e *Enemy) vec {
	a, b := g.route[0], g.gate()
	k := min(1, max(0, e.Dist/g.total))
	return vec{a.X + (b.X-a.X)*k, a.Y + (b.Y-a.Y)*k}
}

// fly moves a flyer: step cells along its straight line, which is shorter
// than the route, so its Dist grows faster. Barricades and traps stay below.
func (g *Game) fly(e *Enemy, step float64) {
	a, b := g.route[0], g.gate()
	line := max(b.sub(a).len(), 1)
	e.Dist = min(e.Dist+step*g.total/line, g.total)
	e.Pos = g.flyPos(e)
	if e.Dist >= g.total {
		e.AtGate = true
	}
}

// split spawns a dying slime's young around where it fell.
func (g *Game) split(e *Enemy, spec EnemySpec) {
	for i := 0; i < spec.SplitN; i++ {
		off := 0.2 * float64(2*i-spec.SplitN+1)
		c := g.spawnEnemy(spec.Split, e.MaxHP*g.Cfg.Enemies[spec.Split].HPMult, max(0, e.Dist+off))
		c.summoned = e.summoned
	}
	g.emit(Event{Kind: "split", X: e.Pos.X, Y: e.Pos.Y})
}

// tickDispel lets a goblin shaman wash his neighbours clean of nets, burns,
// curses, bleeding and slows every few seconds.
func (g *Game) tickDispel(e *Enemy, spec EnemySpec, dt float64) {
	e.ability -= dt
	if e.ability > 0 {
		return
	}
	e.ability = dispelSec
	n := 0
	for _, o := range g.Enemies {
		if o.dead || o.Pos.sub(e.Pos).len() > spec.Dispel {
			continue
		}
		if o.netLeft > 0 || o.burnLeft > 0 || o.curseLeft > 0 || o.bleedLeft > 0 || o.slowLeft > 0 || o.healCut > 0 {
			n++
		}
		o.netLeft, o.burnLeft, o.curseLeft, o.bleedLeft, o.slowLeft, o.healCut = 0, 0, 0, 0, 0, 0
		o.slowMult = 1
	}
	if n > 0 {
		g.emit(Event{Kind: "dispel", X: e.Pos.X, Y: e.Pos.Y, Amount: n})
	}
}

// hex (swamp witch) turns the best building near her into a frog: it stops
// working for a while.
func (g *Game) hex(e *Enemy) {
	var at *Tower
	for _, t := range g.Towers {
		d := center(t.Cell).sub(e.Pos).len()
		if !t.Built || t.hexLeft > 0 || d > hexRadius {
			continue
		}
		if at == nil || t.Level > at.Level || t.Level == at.Level && d < center(at.Cell).sub(e.Pos).len() {
			at = t
		}
	}
	if at == nil {
		return
	}
	at.Disabled = max(at.Disabled, hexSec)
	at.hexLeft = hexSec
	cp := center(at.Cell)
	g.emit(Event{Kind: "hex", X: cp.X, Y: cp.Y, Text: g.Cfg.Towers[at.Kind].Name})
}

// tickHydra grows a head each time the hydra loses another quarter of its
// health, unless it is burning then: fire seals the stump.
func (g *Game) tickHydra(e *Enemy) {
	if e.heads == 0 {
		e.heads, e.nextCut = hydraHeads, 0.75
		e.Title = g.hydraTitle(e)
	}
	for e.nextCut > 0 && e.HP <= e.nextCut*e.MaxHP {
		e.nextCut -= 0.25
		if e.burnLeft > 0 {
			g.emit(Event{Kind: "cauterize", X: e.Pos.X, Y: e.Pos.Y})
			continue
		}
		e.heads++
		e.Title = g.hydraTitle(e)
		g.emit(Event{Kind: "headGrow", X: e.Pos.X, Y: e.Pos.Y, Amount: e.heads})
	}
}

func (g *Game) hydraTitle(e *Enemy) string {
	return fmt.Sprintf("%s · голов: %d", g.Cfg.Enemies[e.Kind].Name, e.heads)
}

// fury is how much faster and harder an enemy hits than its spec says: a
// hydra with extra heads.
func (e *Enemy) fury() (speed, blows float64) {
	if extra := e.heads - hydraHeads; extra > 0 {
		return 1 + 0.15*float64(extra), 1 + 0.3*float64(extra)
	}
	return 1, 1
}
