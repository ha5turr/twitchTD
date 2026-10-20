package game

import (
	"math"
	"slices"
)

// ЭссДиЭф is a guest boss of any biome (SdfChance per boss wave) and is
// unfair on purpose: very slow, a huge pool of hit points, and every few
// seconds he stops to burp or fart (taking turns) and plays a random trick:
// switches the buildings around him off, flattens the nearest ones, strays
// off the path, or pops into the tavern to rob every mercenary present.
// Clown buildings, one per chatter and only while he is out, are the answer:
// they reach him anywhere and hurt nothing else.

const (
	sdfKind      = "sdf"
	clownKind    = "clown"
	sdfEvery     = 9.0 // seconds between tricks
	sdfPause     = 2.0 // he stands still while he burps or farts
	sdfSteal     = 50  // gold taken from each mercenary in the tavern
	sdfTavernSec = 4.0 // how long the robbery takes
	sdfOffSec    = 7.0 // how long he strays off the path
	sdfOffSpeed  = 0.8 // share of the way off (or back) per second
	sdfDisable   = 3.0 // radius of the switch-off, cells
	sdfDisSec    = 5.0 // how long buildings stay off
	sdfFlatten   = 1.6 // radius of the flattening, cells
)

// sdfTricks lists the tricks with their weights: the tavern robbery costs
// chat real gold, so it comes up only one time in ten.
var sdfTricks = []weighted{{"disable", 0, 3}, {"destroy", 0, 3}, {"offroad", 0, 3}, {"tavern", 0, 1}}

// sdfTavern is where he stands while robbing: the middle of the tavern,
// left of the field (grid x is negative there).
var sdfTavern = vec{-2, GridH / 2}

// sdfTrick plays one trick and returns the event that announces it:
// Amount is the act (1 burp, 2 fart), User the trick.
func (g *Game) sdfTrick(e *Enemy) Event {
	return g.sdfPlay(e, pickEnemy(g.rng.Float64(), sdfTricks, 0))
}

func (g *Game) sdfPlay(e *Enemy, trick string) Event {
	e.tricks++
	e.pause = sdfPause
	e.act = 2 - e.tricks%2 // a burp first, then a fart, and so on
	ev := Event{Kind: "ability", Text: sdfKind, User: trick, Amount: e.act, X: e.Pos.X, Y: e.Pos.Y}
	switch trick {
	case "disable":
		for _, t := range g.towersNear(e.Pos, sdfDisable) {
			if t.Built {
				t.Disabled = max(t.Disabled, sdfDisSec)
				cp := center(t.Cell)
				g.emit(Event{Kind: "disabled", Text: sdfKind, X: cp.X, Y: cp.Y})
			}
		}
	case "destroy": // no refund: that is the joke
		for _, t := range g.towersNear(e.Pos, sdfFlatten) {
			g.removeTower(t)
			g.unlock(t.Owner, "farted_down")
			for _, o := range g.Enemies {
				if o.blocked == t {
					o.blocked = nil
				}
			}
			cp := center(t.Cell)
			g.emit(Event{Kind: "destroyed", Text: g.Cfg.Towers[t.Kind].Name, X: cp.X, Y: cp.Y})
		}
	case "offroad":
		a := g.rng.Float64() * 2 * math.Pi
		r := 1.5 + g.rng.Float64()
		e.offset = vec{math.Cos(a) * r, math.Sin(a) * r}
		e.offLeft = sdfOffSec
	case "tavern":
		e.tavernLeft = sdfTavernSec
		g.robTavern(e)
	}
	return ev
}

// towersNear lists the buildings within r of p, nearest first. Clowns are
// immune to ЭссДиЭф's tricks.
func (g *Game) towersNear(p vec, r float64) []*Tower {
	var out []*Tower
	for _, t := range g.Towers {
		if t.Kind != clownKind && center(t.Cell).sub(p).len() <= r {
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b *Tower) int {
		return int(math.Copysign(1, center(a.Cell).sub(p).len()-center(b.Cell).sub(p).len()))
	})
	return out
}

// robTavern takes up to sdfSteal gold from every mercenary present. He
// carries it until he dies; then everyone gets their gold back.
func (g *Game) robTavern(e *Enemy) {
	cut := g.Now().Add(-activeWindow(g.Cfg))
	for key, c := range g.Chatters {
		if key == StreamerName || !c.Joined || !c.LastSeen.After(cut) || c.Gold <= 0 {
			continue
		}
		take := min(sdfSteal, c.Gold)
		c.Gold -= take
		if e.loot == nil {
			e.loot = map[string]int{}
		}
		e.loot[key] += take
		g.unlock(key, "robbed")
		g.emit(Event{Kind: "robbed", User: c.Name, Amount: take})
	}
}

// returnLoot pays back what a fallen ЭссДиЭф stole.
func (g *Game) returnLoot(e *Enemy) {
	for key, n := range e.loot {
		if c := g.Chatters[key]; c != nil {
			g.emit(Event{Kind: "loot", User: c.Name, Amount: g.addGold(c, n)})
		}
	}
	e.loot = nil
}

// tickSdf runs his timers and places him: in the tavern, off the path, or
// on it. Movement along the route itself is the usual one.
func (g *Game) tickSdf(e *Enemy, dt float64) {
	if e.pause > 0 {
		e.pause -= dt
		if e.pause <= 0 {
			e.act = 0
		}
	}
	if e.tavernLeft > 0 {
		e.tavernLeft -= dt
		e.Pos = sdfTavern
		return
	}
	if e.offLeft > 0 {
		e.offLeft -= dt
		e.offAmt = min(1, e.offAmt+sdfOffSpeed*dt)
	} else {
		e.offAmt = max(0, e.offAmt-sdfOffSpeed*dt)
	}
	p := g.routePos(e.Dist)
	if e.offAmt > 0 {
		p.X = clamp(p.X+e.offset.X*e.offAmt, 0.5, GridW-0.5)
		p.Y = clamp(p.Y+e.offset.Y*e.offAmt, 0.5, GridH-0.5)
	}
	e.Pos = p
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// sdfOut reports whether ЭссДиЭф is on the field.
func (g *Game) sdfOut() bool {
	return slices.ContainsFunc(g.Enemies, func(e *Enemy) bool { return e.Kind == sdfKind && !e.dead })
}

// clownsOf counts the clown buildings a chatter has.
func (g *Game) clownsOf(owner string) int {
	n := 0
	for _, t := range g.Towers {
		if t.Owner == owner && t.Kind == clownKind {
			n++
		}
	}
	return n
}

// packClowns takes the clowns down after a wave, refunding them in full:
// they only exist for ЭссДиЭф.
func (g *Game) packClowns() {
	for _, t := range g.Towers {
		if t.Kind != clownKind {
			continue
		}
		g.removeTower(t)
		if c := g.Chatters[t.Owner]; c != nil && t.Owner != StreamerName {
			g.addGold(c, t.Invested)
		}
		cp := center(t.Cell)
		g.emit(Event{Kind: "sell", User: t.Owner, X: cp.X, Y: cp.Y, Amount: t.Invested})
	}
}

// crownClown gives the top damage dealer against a fallen ЭссДиЭф his title.
func (g *Game) crownClown(e *Enemy) {
	best, most := "", 0.0
	for key, d := range e.hurtBy {
		if d > most || d == most && key < best {
			best, most = key, d
		}
	}
	if best != "" {
		g.unlock(best, "clown_king")
	}
}
