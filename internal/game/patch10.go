package game

import "math"

// Patch 10: the warlock and the shaman with their buildings and skills, and
// two enemies that break habits: the goblin bomber (against barricades) and
// the shieldbearer (against arrows alone).

// guardCut is how much physical damage a shieldbearer's cover stops.
const guardCut = 0.5

// guarded reports whether a living shieldbearer covers e (itself included).
func (g *Game) guarded(e *Enemy) bool {
	for _, o := range g.Enemies {
		if r := g.Cfg.Enemies[o.Kind].Guard; r > 0 && !o.dead && o.Pos.sub(e.Pos).len() <= r {
			return true
		}
	}
	return false
}

// blowUp sets off a bomber at the barricade it reached: the barricade loses
// Bomb of its strength and the goblin is gone, with no bounty for anyone.
func (g *Game) blowUp(e *Enemy, spec EnemySpec) {
	t := e.blocked
	e.blocked = nil
	g.emit(Event{Kind: "boom", X: e.Pos.X, Y: e.Pos.Y})
	g.damage(e, e.HP+1, "", true)
	if g.Towers[t.Cell] != t {
		return
	}
	t.HP -= spec.Bomb * t.MaxHP
	if t.HP <= 0 {
		g.breakWall(t)
	}
}

// tickCurse lets a warlock's altar curse the toughest enemy in range that is
// not cursed yet. Faster and stronger with every level.
func (g *Game) tickCurse(t *Tower, spec TowerSpec, dt float64) {
	t.curseCD -= dt
	if t.curseCD > 0 {
		return
	}
	tc := center(t.Cell)
	var at *Enemy
	for _, e := range g.Enemies {
		if !e.dead && e.curseLeft <= 0 && e.Pos.sub(tc).len() <= spec.Range && (at == nil || e.HP > at.HP) {
			at = e
		}
	}
	if at == nil {
		return // keeps the curse ready until someone comes
	}
	lv := 1 + 0.25*float64(t.Level-1)
	t.curseCD = spec.CurseEvery / lv
	amp := spec.CurseAmp * lv * g.classBonus(t.Owner, t.Kind)
	if g.Cfg.Enemies[at.Kind].Boss {
		amp /= 2
	}
	at.curseLeft, at.curseAmp, at.curseOwner = spec.CurseSec, amp, t.Owner
	g.emit(Event{Kind: "curse", X: tc.X, Y: tc.Y, X2: at.Pos.X, Y2: at.Pos.Y})
}

// payCurse gives the altar's owner a soul shard for a cursed enemy's death:
// part of its bounty and, now and then, a soul.
func (g *Game) payCurse(e *Enemy) {
	if e.curseLeft <= 0 || e.summoned || e.curseOwner == "" || e.curseOwner == StreamerName {
		return
	}
	c := g.Chatters[e.curseOwner]
	if c == nil {
		return
	}
	gold := int(math.Round(float64(g.Cfg.Enemies[e.Kind].Bounty) * g.rewardMult * g.Cfg.CurseShardPct))
	got := 0
	if gold > 0 {
		got = g.addGold(c, gold)
	}
	if g.rng.Float64() < g.Cfg.CurseSoulChance {
		g.giveSouls(e.curseOwner, c, 1)
	}
	g.emit(Event{Kind: "shard", User: c.Name, X: e.Pos.X, Y: e.Pos.Y, Amount: got})
}

// chainZap strikes target with lightning that jumps on to the nearest enemies
// not hit yet, weaker with every jump.
func (g *Game) chainZap(t *Tower, spec TowerSpec, target *Enemy, dmg float64) {
	from := center(t.Cell)
	jumps := spec.Chain + (t.Level-1)/2
	hit := map[*Enemy]bool{}
	g.assist = g.drummers(t)
	for cur := target; cur != nil && len(hit) <= jumps; {
		p := cur.Pos
		g.emit(Event{Kind: "zap", X: from.X, Y: from.Y, X2: p.X, Y2: p.Y})
		hit[cur] = true
		g.damage(cur, dmg, t.Owner, spec.Magic)
		falloff := spec.ChainFalloff
		if g.artifact(t) {
			falloff = g.Cfg.TotemArtFalloff
		}
		from, dmg = p, dmg*falloff
		var next *Enemy
		for _, e := range g.Enemies {
			if !e.dead && !hit[e] && e.Pos.sub(p).len() <= spec.ChainRange && (next == nil || e.Pos.sub(p).len() < next.Pos.sub(p).len()) {
				next = e
			}
		}
		cur = next
	}
	g.assist = nil
}

// massCurse (warlock): every enemy on the map is cursed for a while: no
// armor and WarlockCurseAmp more damage taken (bosses half). A stronger curse
// from an altar is kept; mass curses pay no soul shards.
func (g *Game) massCurse(c *Chatter) {
	n := 0
	for _, e := range g.Enemies {
		if e.dead {
			continue
		}
		amp := g.Cfg.WarlockCurseAmp
		if g.Cfg.Enemies[e.Kind].Boss {
			amp /= 2
		}
		if e.curseLeft <= 0 || e.curseAmp < amp {
			e.curseAmp, e.curseOwner = amp, ""
		}
		e.curseLeft = max(e.curseLeft, g.Cfg.WarlockCurseSec)
		n++
	}
	if n > 0 {
		g.emit(Event{Kind: "massCurse", User: c.Name, Amount: n})
	}
}

// bloodPact (warlock): the walls pay PactWallsPct of their strength and every
// building on the map hits PactBoost harder for PactSec. Never while the
// walls are low: it reports false then and nothing happens.
func (g *Game) bloodPact(c *Chatter) bool {
	if g.Walls < g.Cfg.PactMinWalls*g.Cfg.WallsHP {
		return false
	}
	cost := g.Cfg.PactWallsPct * g.Cfg.WallsHP
	g.Walls -= cost
	g.pactLeft, g.pactBoost = g.Cfg.PactSec, g.Cfg.PactBoost
	g.emit(Event{Kind: "bloodPact", User: c.Name, Amount: int(math.Round(cost))})
	return true
}

// earthquake (shaman): every enemy on the ground is stunned for a moment;
// ghosts float above it, bosses shake it off twice as fast.
func (g *Game) earthquake(c *Chatter) {
	for _, e := range g.Enemies {
		spec := g.Cfg.Enemies[e.Kind]
		if e.dead || spec.Ghost || e.tavernLeft > 0 {
			continue
		}
		stun := g.Cfg.QuakeStunSec
		if spec.Boss {
			stun /= 2
		}
		e.stunLeft = max(e.stunLeft, stun)
	}
	g.emit(Event{Kind: "quake", User: c.Name})
}
