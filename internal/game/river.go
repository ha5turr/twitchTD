package game

import "slices"

// Patch 0.22: the river camp. The path is a river, and the last cells before
// the tavern are its shore and pier. Traps on the water are anchor nets
// (a milder snap, no bleeding, a stronger slow) and barricades are booms
// (fewer hit points). Fire goes out on swimmers, while boats burn half as
// hard again. Pirates sit in boats that take the blows, with armour, until
// they sink; deep ones dive now and then where nothing can touch them;
// the drowned rise from the riverbed a third of the way down. Pirates
// leave their boats at the pier and walk the last cells. Its bosses: the grim
// octopus sets its young on buildings, which stay silent until the
// neighbours shoot the octopling off, and the leviathan raises the tide,
// drowning the buildings on the banks around it while it swims deep.

const (
	pierCells = 2 // the last path cells before the tavern are dry: the shore and the pier

	netDmgMult  = 0.5  // an anchor net hits this hard next to a trap's snap
	netSlowMult = 0.7  // and slows to this share of a trap's slow (lower is slower)
	netSecMult  = 1.5  // for this much longer
	boomHPMult  = 0.75 // a boom's hit points next to a barricade's
	boatBurn    = 1.5  // boats burn this much harder
	boatArmor   = 0.4  // the boat's planks: cut of physical damage while it floats

	diveEvery = 6.0 // a deep one dives this often
	diveSec   = 2.0 // and stays under this long

	riseFrom, riseTo = 0.25, 0.40 // the drowned rise this far down the river (share of the route)

	clingEvery  = 9.0  // the octopus sets an octopling on a building this often
	clingRadius = 4.0  // within this many cells of it
	clingSec    = 12.0 // and an octopling lets go by itself after this long

	tideEvery  = 15.0 // the leviathan raises the tide this often
	tideSec    = 5.0  // it lasts this long
	tideRadius = 5.0  // and drowns the bank within this many cells of it
)

// river reports whether this map is the river camp.
func (g *Game) river() bool { return g.Map.Biome == BiomeRiver }

// pierStart is the route distance where the river gives way to the shore.
func (g *Game) pierStart() float64 {
	n := len(g.cellDist)
	if n <= pierCells {
		return 0
	}
	return g.cellDist[n-pierCells] - 0.5
}

// wet reports whether a path cell is water rather than the pier.
func (g *Game) wet(p Pos) bool {
	if !g.river() {
		return false
	}
	i := slices.Index(g.Map.Path, p)
	return i >= 0 && i < len(g.Map.Path)-pierCells
}

// swimming reports whether an enemy is in the water, out of a boat: fire
// does not catch on it.
func (g *Game) swimming(e *Enemy) bool {
	if !g.river() || e.cling != nil || e.boatHP > 0 || g.Cfg.Enemies[e.Kind].Fly {
		return false
	}
	return e.Dist < g.pierStart()
}

// towerName is a building's name on this map: traps and barricades on the
// river are nets and booms.
func (g *Game) towerName(kind string) string {
	if g.river() {
		switch kind {
		case "trap":
			return "Якорная сеть"
		case "wall":
			return "Бон"
		}
	}
	return g.Cfg.Towers[kind].Name
}

// submerged reports whether an enemy is under water, out of every reach.
func (e *Enemy) submerged() bool { return e.diveLeft > 0 }

// targetable reports whether a building can aim at the enemy.
func targetable(e *Enemy) bool { return !e.dead && !e.submerged() }

// launch gives a newly spawned enemy its river kit: a boat, a dive timer.
func (g *Game) launch(e *Enemy, spec EnemySpec) {
	if spec.Boat > 0 {
		e.boatHP = spec.Boat * e.MaxHP
		e.boatMax = e.boatHP
	}
	if spec.Dive {
		e.diveCD = diveEvery / 2
	}
}

// riseDist is where a drowned climbs out of the riverbed.
func (g *Game) riseDist() float64 {
	return g.total * (riseFrom + (riseTo-riseFrom)*g.rng.Float64())
}

// hitBoat lets a pirate's boat take a hit while it floats. It reports the
// damage the boat took; the pirate is untouched by this hit.
func (g *Game) hitBoat(e *Enemy, dmg float64, physical bool) float64 {
	if physical {
		dmg *= 1 - boatArmor
	}
	hit := min(dmg, e.boatHP)
	e.boatHP -= hit
	if e.boatHP <= 0 {
		e.boatHP = 0
		e.burnLeft = 0 // the fire sinks with the boat
		g.emit(Event{Kind: "sink", X: e.Pos.X, Y: e.Pos.Y})
	}
	return hit
}

// ashore leaves a pirate's boat at the pier: he walks the rest of the way.
func (g *Game) ashore(e *Enemy) {
	if e.boatHP <= 0 || !g.river() || e.Dist < g.pierStart() {
		return
	}
	e.boatHP = 0
	e.burnLeft = 0 // the burning boat stays behind
	g.emit(Event{Kind: "ashore", X: e.Pos.X, Y: e.Pos.Y})
}

// tickDive takes a deep one under now and then.
func (g *Game) tickDive(e *Enemy, dt float64) {
	if e.diveLeft > 0 {
		e.diveLeft = max(0, e.diveLeft-dt)
		return
	}
	if e.diveCD -= dt; e.diveCD <= 0 && e.Dist < g.pierStart() {
		e.diveCD, e.diveLeft = diveEvery, diveSec
		e.netLeft, e.burnLeft, e.blocked = 0, 0, nil // under the net and the boom
		g.emit(Event{Kind: "dive", X: e.Pos.X, Y: e.Pos.Y})
	}
}

// cling (grim octopus) sets an octopling on the best building near it: the
// building stays silent until the octopling is shot off or lets go.
func (g *Game) cling(boss *Enemy) {
	taken := map[*Tower]bool{}
	for _, o := range g.Enemies {
		if o.cling != nil {
			taken[o.cling] = true
		}
	}
	var at *Tower
	for _, t := range g.Towers {
		d := center(t.Cell).sub(boss.Pos).len()
		if !t.Built || taken[t] || d > clingRadius || g.Cfg.Towers[t.Kind].Role == RoleTrap || g.Cfg.Towers[t.Kind].Role == RoleWall {
			continue
		}
		if at == nil || t.Level > at.Level || t.Level == at.Level && d < center(at.Cell).sub(boss.Pos).len() {
			at = t
		}
	}
	if at == nil {
		return
	}
	hp := boss.MaxHP * g.Cfg.Enemies["octopling"].HPMult
	e := &Enemy{ID: g.id(), Kind: "octopling", HP: hp, MaxHP: hp, Dist: g.total, Pos: center(at.Cell), slowMult: 1, summoned: true}
	e.cling, e.clingLeft = at, clingSec
	g.Enemies = append(g.Enemies, e)
	at.Disabled = max(at.Disabled, clingSec)
	g.emit(Event{Kind: "cling", X: e.Pos.X, Y: e.Pos.Y, Text: g.towerName(at.Kind)})
}

// tickCling keeps an octopling on its building; it lets go after a while
// or when the building is gone, so a wave never waits on it for good.
func (g *Game) tickCling(e *Enemy, dt float64) {
	e.clingLeft -= dt
	if g.Towers[e.cling.Cell] != e.cling || e.clingLeft <= 0 {
		e.dead = true // slips back into the water: no bounty
		g.release(e)
	}
}

// release frees an octopling's building.
func (g *Game) release(e *Enemy) {
	if e.cling == nil {
		return
	}
	e.cling.Disabled = 0
	cp := center(e.cling.Cell)
	g.emit(Event{Kind: "unclung", X: cp.X, Y: cp.Y})
	e.cling = nil
}

// tide (leviathan) floods the banks around it: buildings next to the
// river there go under and fall silent, and it swims deep the while.
func (g *Game) tide(boss *Enemy) {
	boss.diveLeft = tideSec
	boss.netLeft, boss.burnLeft, boss.blocked = 0, 0, nil
	wet := map[Pos]bool{}
	for _, p := range g.Map.Path {
		if g.wet(p) {
			wet[p] = true
		}
	}
	n := 0
	for _, t := range g.Towers {
		if !t.Built || wet[t.Cell] || center(t.Cell).sub(boss.Pos).len() > tideRadius {
			continue
		}
		bank := false
		for _, d := range dirs8 {
			if wet[Pos{t.Cell.X + d.X, t.Cell.Y + d.Y}] {
				bank = true
				break
			}
		}
		if bank {
			t.Disabled = max(t.Disabled, tideSec)
			t.floodLeft = tideSec
			n++
		}
	}
	g.emit(Event{Kind: "tide", X: boss.Pos.X, Y: boss.Pos.Y, Amount: n})
}

// dirs8 are the eight neighbouring cells.
var dirs8 = []Pos{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}

// boatLeft is the share of a pirate's boat still afloat.
func boatLeft(e *Enemy) float64 {
	if e.boatMax <= 0 {
		return 0
	}
	return e.boatHP / e.boatMax
}
