package game

import (
	"math"
	"slices"
)

// Class skills (hotfix 4). During a wave a mercenary sitting in the tavern
// now and then acts on its own, by class:
//   - Воитель: charges down the path from the gate to the spawn, shoving
//     every enemy he passes two cells back; barricades and traps do not stop
//     him (and enemies never stun him while he builds, see tickWorkers);
//   - Маг: calls a meteor shower over the whole field: moderate damage and a
//     burn that also halves healing;
//   - Бард: plays music for a while: the aura of his drums covers the whole
//     map, and his drums shoot notes at enemies;
//   - Разбойник: shadow-steps to the boss (or into a crowd), stabs it with a
//     bleeding, slowing cut, fans blades around if no boss is near and the
//     crowd is thick, and steps back.
// The druid's panther and the rogue's pickpocketing live in rogues.go.

// Run is a mercenary out on the field for a skill: a charging Воитель or a
// rogue in a shadow-step. The overlay draws it; the tavern lets it go.
type Run struct {
	Owner string
	Name  string
	Kind  string // "charge" | "step"
	Pos   vec
	dist  float64      // charge: route distance, falling to 0
	left  float64      // step: seconds left on the field
	hit   map[int]bool // charge: enemies already shoved
}

type meteor struct {
	pos   vec
	delay float64
	owner string
}

func (g *Game) runOf(key string) *Run {
	i := slices.IndexFunc(g.Runs, func(r *Run) bool { return r.Owner == key })
	if i < 0 {
		return nil
	}
	return g.Runs[i]
}

// tickHeroes runs the class skills of everyone in the tavern, the music,
// the charges, the shadow-steps and the meteors already falling.
func (g *Game) tickHeroes(dt float64) {
	for _, c := range g.Chatters {
		c.musicLeft = max(0, c.musicLeft-dt)
	}
	if g.Phase == PhaseWave && len(g.Enemies) > 0 {
		for _, key := range g.tavernMates() {
			c := g.Chatters[key]
			if c.Class == "warlock" {
				g.tickPact(key, c, dt) // the warlock's second skill keeps its own time
			}
			every := map[string]float64{
				"warrior": g.Cfg.WarriorEverySec, "mage": g.Cfg.MageEverySec,
				"bard": g.Cfg.BardEverySec, "rogue": g.Cfg.RogueStepEverySec,
				"cleric": g.Cfg.ClericEverySec, "pyro": g.Cfg.PyroEverySec, "ranger": g.Cfg.RangerEverySec,
				"warlock": g.Cfg.WarlockEverySec, "shaman": g.Cfg.ShamanEverySec,
			}[c.Class]
			if every <= 0 {
				continue
			}
			if c.skillCD == 0 { // the first time comes halfway through
				c.skillCD = every / 2
			}
			c.skillCD -= dt
			if c.skillCD > 0 {
				continue
			}
			c.skillCD = every
			switch c.Class {
			case "warrior":
				g.Runs = append(g.Runs, &Run{Owner: key, Name: c.Name, Kind: "charge", Pos: g.gate(), dist: g.total, hit: map[int]bool{}})
				g.emit(Event{Kind: "charge", User: c.Name})
			case "mage":
				g.meteorShower(key, c)
			case "bard":
				c.musicLeft = g.Cfg.BardMusicSec
				g.emit(Event{Kind: "music", User: c.Name})
			case "rogue":
				g.shadowStep(key, c)
			case "cleric":
				g.pillarOfLight(key, c)
			case "pyro":
				g.setPathAlight(key, c)
			case "ranger":
				g.arrowRain(key, c)
			case "warlock":
				g.massCurse(c)
			case "shaman":
				g.earthquake(c)
			}
		}
	}
	g.pactLeft = max(0, g.pactLeft-dt)
	g.tickRuns(dt)
	g.tickMeteors(dt)
}

// tickRuns moves charging warriors and ends shadow-steps.
func (g *Game) tickRuns(dt float64) {
	alive := g.Runs[:0]
	for _, r := range g.Runs {
		switch r.Kind {
		case "charge":
			from := r.dist
			r.dist -= g.Cfg.WarriorSpeed * dt
			for _, e := range g.Enemies {
				// Bosses are too heavy to shove: pushed back again and again a slow
				// boss would never reach the gate nor die, and the wave would drag on.
				if e.dead || r.hit[e.ID] || e.tavernLeft > 0 || g.Cfg.Enemies[e.Kind].Boss || e.Dist < r.dist || e.Dist > from+0.5 {
					continue
				}
				r.hit[e.ID] = true
				e.Dist = max(0, e.Dist-g.Cfg.WarriorPush)
				e.blocked, e.AtGate = nil, false
				e.Pos = g.routePos(e.Dist)
				g.emit(Event{Kind: "shove", User: r.Name, X: e.Pos.X, Y: e.Pos.Y})
			}
			if r.dist <= 0 {
				continue // reached the spawn: back in the tavern
			}
			r.Pos = g.routePos(r.dist)
		case "step":
			r.left -= dt
			if r.left <= 0 {
				g.emit(Event{Kind: "shadowstep", User: r.Name, X: r.Pos.X, Y: r.Pos.Y})
				continue
			}
		}
		alive = append(alive, r)
	}
	g.Runs = alive
}

// meteorShower drops MeteorCount meteors over the field: half on enemies,
// half anywhere, one after another.
func (g *Game) meteorShower(key string, c *Chatter) {
	for i := 0; i < g.Cfg.MeteorCount; i++ {
		p := vec{0.5 + g.rng.Float64()*(GridW-1), 0.5 + g.rng.Float64()*(GridH-1)}
		if i%2 == 0 && len(g.Enemies) > 0 {
			p = g.Enemies[g.rng.IntN(len(g.Enemies))].Pos
		}
		g.meteors = append(g.meteors, &meteor{pos: p, delay: 0.3 + float64(i)*0.25, owner: key})
	}
	g.emit(Event{Kind: "meteorShower", User: c.Name})
}

func (g *Game) tickMeteors(dt float64) {
	alive := g.meteors[:0]
	for _, m := range g.meteors {
		m.delay -= dt
		if m.delay > 0 {
			alive = append(alive, m)
			continue
		}
		for _, e := range g.Enemies {
			if !e.dead && e.Pos.sub(m.pos).len() <= g.Cfg.MeteorRadius {
				e.burnLeft, e.burnDPS, e.burnOwner = g.Cfg.MeteorBurnSec, g.skillHP()*g.Cfg.MeteorBurn, m.owner
				g.damage(e, g.skillHP()*g.Cfg.MeteorDmg, m.owner, true)
			}
		}
		g.emit(Event{Kind: "meteor", X: m.pos.X, Y: m.pos.Y})
	}
	g.meteors = alive
}

// shadowStep sends a rogue to the boss, or else to the enemy furthest
// along: a bleeding, slowing stab, and a fan of blades when no boss is near
// and more than two enemies are.
func (g *Game) shadowStep(key string, c *Chatter) {
	var at *Enemy
	for _, e := range g.Enemies {
		if e.dead || e.tavernLeft > 0 {
			continue
		}
		boss := g.Cfg.Enemies[e.Kind].Boss
		switch {
		case at == nil,
			boss && !g.Cfg.Enemies[at.Kind].Boss,
			boss == g.Cfg.Enemies[at.Kind].Boss && e.Dist > at.Dist:
			at = e
		}
	}
	if at == nil {
		return
	}
	p := at.Pos
	g.Runs = append(g.Runs, &Run{Owner: key, Name: c.Name, Kind: "step", Pos: p, left: g.Cfg.RogueStepSec})
	g.emit(Event{Kind: "shadowstep", User: c.Name, X: p.X, Y: p.Y})
	at.bleedLeft, at.bleedDPS, at.bleedOwner = g.Cfg.TrapBleedSec, g.skillHP()*g.Cfg.RogueStepDmg*g.Cfg.TrapBleed, key
	at.slowMult, at.slowLeft = 0.5, 2
	g.damage(at, g.skillHP()*g.Cfg.RogueStepDmg, key, false)
	bossNear, crowd := false, 0
	for _, e := range g.Enemies {
		if !e.dead && e.Pos.sub(p).len() <= g.Cfg.RogueFanRadius {
			crowd++
			bossNear = bossNear || g.Cfg.Enemies[e.Kind].Boss
		}
	}
	if bossNear || crowd <= 2 {
		return
	}
	for _, e := range g.Enemies {
		if !e.dead && e.Pos.sub(p).len() <= g.Cfg.RogueFanRadius {
			g.damage(e, g.skillHP()*g.Cfg.RogueFanDmg, key, false)
		}
	}
	g.emit(Event{Kind: "bladefan", User: c.Name, X: p.X, Y: p.Y})
}

// playing reports whether the owner of a building is playing music now.
func (g *Game) playing(owner string) bool {
	c := g.Chatters[owner]
	return c != nil && c.musicLeft > 0
}

// tickNotes lets a bard's drums shoot notes while he plays: slow, single
// target, moderate damage, nothing between songs.
func (g *Game) tickNotes(t *Tower, dt float64) {
	if !g.playing(t.Owner) {
		return
	}
	t.cooldown -= dt
	if t.cooldown > 0 {
		return
	}
	tc := center(t.Cell)
	var target *Enemy
	for _, e := range g.Enemies {
		if !e.dead && e.Pos.sub(tc).len() <= g.Cfg.NoteRange && (target == nil || e.Dist > target.Dist) {
			target = e
		}
	}
	if target == nil {
		return
	}
	t.cooldown = 1 / g.Cfg.NoteRate
	dmg := g.skillHP() * g.Cfg.NoteDmg * g.levelMult(t) * g.classBonus(t.Owner, t.Kind) * g.drinkMult(t)
	g.Projectiles = append(g.Projectiles, &Projectile{
		ID: g.id(), Kind: "note", Pos: tc, Target: target.ID, Aim: target.Pos,
		Damage: dmg, Magic: true, Speed: 5, Owner: t.Owner,
	})
}

// auraRange is how far an aura reaches. A shrine's damage buff covers the
// whole map, always (a design rule: do not shrink it in balance passes); a
// bard's drums do too while he plays.
func (g *Game) auraRange(a *Tower, spec TowerSpec) float64 {
	if spec.AuraDmg > 0 || spec.AuraRate > 0 && g.playing(a.Owner) {
		return math.Inf(1)
	}
	return spec.AuraRange
}

// skillHP is what class skills measure their damage in: a plain enemy's HP
// at this wave, without the boost for a big chat. Skills multiply with the
// number of mercenaries, so they must not also grow with the chat's size.
func (g *Game) skillHP() float64 {
	return g.Cfg.BaseHP * math.Pow(g.Cfg.HPGrowth, float64(max(g.Wave, 1)))
}

// Hotfix 5 skills: the cleric's pillar of light, the pyromancer's burning
// path and the ranger's rain of arrows.

// crowdSpot is the enemy with the most others within r of it: where an
// area skill hits hardest. With a single enemy it is that one.
func (g *Game) crowdSpot(r float64) (vec, bool) {
	var at *Enemy
	best := 0
	for _, e := range g.Enemies {
		if e.dead || e.tavernLeft > 0 {
			continue
		}
		n := 0
		for _, o := range g.Enemies {
			if !o.dead && o.Pos.sub(e.Pos).len() <= r {
				n++
			}
		}
		if n > best || n == best && at != nil && e.Dist > at.Dist {
			at, best = e, n
		}
	}
	if at == nil {
		return vec{}, false
	}
	return at.Pos, true
}

// pillarOfLight stuns every enemy in the crowdest cell and hurts them.
func (g *Game) pillarOfLight(key string, c *Chatter) {
	p, ok := g.crowdSpot(g.Cfg.PillarRadius)
	if !ok {
		return
	}
	for _, e := range g.Enemies {
		if !e.dead && e.Pos.sub(p).len() <= g.Cfg.PillarRadius {
			e.stunLeft = max(e.stunLeft, g.Cfg.PillarStunSec)
			g.damage(e, g.skillHP()*g.Cfg.PillarDmg, key, true)
		}
	}
	g.emit(Event{Kind: "pillar", User: c.Name, X: p.X, Y: p.Y})
}

// setPathAlight sets the whole path on fire: a hit and a burn for every
// enemy on it (burning halves healing, see heal).
func (g *Game) setPathAlight(key string, c *Chatter) {
	for _, e := range g.Enemies {
		if e.dead || e.tavernLeft > 0 || e.offAmt > 0.5 {
			continue // only those on the path
		}
		e.burnLeft, e.burnDPS, e.burnOwner = g.Cfg.PathFireBurnSec, g.skillHP()*g.Cfg.PathFireBurn, key
		g.damage(e, g.skillHP()*g.Cfg.PathFireDmg, key, true)
	}
	g.emit(Event{Kind: "pathFire", User: c.Name})
}

// arrowRain looses a volley over the thickest crowd: everyone within
// ArrowRainRadius is hit.
func (g *Game) arrowRain(key string, c *Chatter) {
	p, ok := g.crowdSpot(g.Cfg.ArrowRainRadius)
	if !ok {
		return
	}
	for _, e := range g.Enemies {
		if !e.dead && e.Pos.sub(p).len() <= g.Cfg.ArrowRainRadius {
			g.damage(e, g.skillHP()*g.Cfg.ArrowRainDmg, key, false)
		}
	}
	g.emit(Event{Kind: "arrowRain", User: c.Name, X: p.X, Y: p.Y})
}
