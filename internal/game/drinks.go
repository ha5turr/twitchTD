package game

import (
	"fmt"
	"math"
)

// Drinks are no longer cosmetic (hotfix 2): each changes how hard the
// drinker's buildings hit for a while.
//   - ale: +Boost for Sec seconds, then the hangover: After for as long;
//   - mead: one random building hits Boost times as hard, another one
//     After times, for Sec seconds;
//   - potion: +Boost for Sec seconds, nothing after.
// Damage from shots, traps and lightning follows drinkMult.

// drinkMult is how a building's damage is changed by its owner's drinks.
func (g *Game) drinkMult(t *Tower) float64 {
	m := 1.0
	if c := g.Chatters[t.Owner]; c != nil && c.buffLeft > 0 {
		m *= c.buff
	}
	if t.drinkLeft > 0 {
		m *= t.drink
	}
	if t.warLeft > 0 {
		m *= 1 + g.Cfg.DrumWarBoost // an artifact drum's war beat
	}
	if g.pactLeft > 0 {
		m *= 1 + g.pactBoost // a warlock's blood pact: every building on the map
	}
	return m
}

// tickDrinks runs the drink timers: the ale's high turns into its hangover.
func (g *Game) tickDrinks(dt float64) {
	for _, c := range g.Chatters {
		if c.buffLeft <= 0 {
			continue
		}
		c.buffLeft -= dt
		if c.buffLeft <= 0 && c.nextBuff > 0 {
			c.buff, c.buffLeft, c.nextBuff = c.nextBuff, c.nextSec, 0
		}
	}
	for _, t := range g.Towers {
		t.drinkLeft = max(0, t.drinkLeft-dt)
	}
}

// drinkEffect applies a drink to its drinker's buildings and describes it
// for the chat.
func (g *Game) drinkEffect(key string, c *Chatter, kind string) string {
	d := g.Cfg.Drinks[kind]
	pct := func(m float64) string { return fmt.Sprintf("%+d%%", int(math.Round((m-1)*100))) }
	switch kind {
	case "ale":
		c.buff, c.buffLeft, c.nextBuff, c.nextSec = d.Boost, d.Sec, d.After, d.Sec
		return fmt.Sprintf("постройки %s на %d с, потом похмелье %s на столько же", pct(d.Boost), int(d.Sec), pct(d.After))
	case "potion":
		c.buff, c.buffLeft, c.nextBuff = d.Boost, d.Sec, 0
		return fmt.Sprintf("постройки %s на %d с", pct(d.Boost), int(d.Sec))
	case "mead":
		var mine []*Tower
		for _, t := range g.Towers {
			if t.Owner == key && t.Built && g.Cfg.Towers[t.Kind].Role != RoleAura && g.Cfg.Towers[t.Kind].Role != RoleWall {
				mine = append(mine, t)
			}
		}
		if len(mine) == 0 {
			return "но бить пока нечему"
		}
		sortTowers(mine)
		up := mine[g.rng.IntN(len(mine))]
		up.drink, up.drinkLeft = d.Boost, d.Sec
		cp := center(up.Cell)
		g.emit(Event{Kind: "drinkTower", X: cp.X, Y: cp.Y, Amount: 1})
		out := fmt.Sprintf("%s бьёт в %.0f раза сильнее", up.Cell.Name(), d.Boost)
		if len(mine) > 1 {
			var rest []*Tower
			for _, t := range mine {
				if t != up {
					rest = append(rest, t)
				}
			}
			down := rest[g.rng.IntN(len(rest))]
			down.drink, down.drinkLeft = d.After, d.Sec
			cp := center(down.Cell)
			g.emit(Event{Kind: "drinkTower", X: cp.X, Y: cp.Y, Amount: -1})
			out += fmt.Sprintf(", %s вдвое слабее", down.Cell.Name())
		}
		return out + fmt.Sprintf(" на %d с", int(d.Sec))
	}
	return ""
}

// sortTowers orders buildings by cell so a seeded game picks the same ones.
func sortTowers(ts []*Tower) {
	for i := 1; i < len(ts); i++ {
		for j := i; j > 0 && less(ts[j].Cell, ts[j-1].Cell); j-- {
			ts[j], ts[j-1] = ts[j-1], ts[j]
		}
	}
}

func less(a, b Pos) bool { return a.Y < b.Y || a.Y == b.Y && a.X < b.X }

// drinkKinds is the menu a round picks from.
var drinkKinds = []string{"ale", "mead", "potion"}
