package game

import (
	"fmt"
	"math"
)

// Barricades: they pay their owner a little for every enemy they hold,
// give part of their price back when broken, can be mended cheaply with
// !чинить (once in wallRepairCdSec), and shrines mend all of them a bit after every wave.

// payHold credits the owner for each second an enemy spends hacking at t.
func (g *Game) payHold(t *Tower, dt float64) {
	c := g.Chatters[t.Owner]
	if c == nil || t.Owner == StreamerName {
		return
	}
	t.holdGold += g.Cfg.WallHoldGold * g.rewardMult * dt
	if n := int(t.holdGold); n > 0 {
		t.holdGold -= float64(n)
		g.addGold(c, n)
	}
}

// wallRefund pays back part of a broken barricade: more to the Воитель,
// whose signature building it is. It returns the gold paid.
func (g *Game) wallRefund(t *Tower) int {
	c := g.Chatters[t.Owner]
	if c == nil || t.Owner == StreamerName {
		return 0
	}
	pct := g.Cfg.WallRefundPct
	if g.classBonus(t.Owner, t.Kind) > 1 {
		pct = g.Cfg.WallRefundClass
	}
	return g.addGold(c, t.Invested*pct/100)
}

// shrinesMendWalls runs after a wave: every shrine mends every barricade on
// the map, whoever owns it, by a share of its hit points per shrine level.
func (g *Game) shrinesMendWalls() {
	share := 0.0
	for _, s := range g.Towers {
		if g.Cfg.Towers[s.Kind].Role == RoleAura && g.Cfg.Towers[s.Kind].AuraDmg > 0 && s.Built {
			share += g.Cfg.ShrineWallRepair * auraMult(s) * g.classBonus(s.Owner, s.Kind)
		}
	}
	if share <= 0 {
		return
	}
	for _, t := range g.Towers {
		if t.Built && g.Cfg.Towers[t.Kind].Role == RoleWall && t.HP < t.MaxHP {
			t.HP = min(t.MaxHP, t.HP+share*t.MaxHP)
			cp := center(t.Cell)
			g.emit(Event{Kind: "mended", X: cp.X, Y: cp.Y})
		}
	}
}

// repairCost is the price of mending t to full: a share of its last
// price, in proportion to the hit points it lost.
func (g *Game) repairCost(t *Tower) int {
	lost := 1 - t.HP/t.MaxHP
	return max(1, int(math.Ceil(lost*float64(g.Cfg.TowerCost(t.Kind, t.Level))*g.Cfg.WallRepairShare)))
}

func (g *Game) cmdRepair(c *Chatter, key string, args []string) {
	if g.busyAtDice(c, key) {
		return
	}
	t := g.ownTower(c, key, args)
	if t == nil {
		return
	}
	switch {
	case g.Cfg.Towers[t.Kind].Role != RoleWall:
		g.reply(c.Name, "чинить можно только баррикады")
		return
	case !t.Built || t.PendingCost > 0 || t.repairing:
		g.reply(c.Name, t.Cell.Name()+": наёмник ещё работает над ней")
		return
	case t.HP >= t.MaxHP:
		g.reply(c.Name, t.Cell.Name()+": баррикада цела")
		return
	}
	streamer := g.isStreamer(key)
	if !streamer && t.repairCD > 0 {
		g.reply(c.Name, fmt.Sprintf("%s: баррикаду недавно чинили, снова можно через %d с", t.Cell.Name(), int(math.Ceil(t.repairCD))))
		return
	}
	if !streamer && !g.canQueue(key) {
		g.reply(c.Name, fmt.Sprintf("у наёмника уже %d приказа, подожди", g.Cfg.MaxOrders))
		return
	}
	cost := g.repairCost(t)
	if !streamer {
		if c.Gold < cost {
			g.reply(c.Name, fmt.Sprintf("починка стоит %d золота, у тебя %d", cost, c.Gold))
			return
		}
		c.Gold -= cost
	}
	cp := center(t.Cell)
	g.emit(Event{Kind: "upgrade", User: c.Name, X: cp.X, Y: cp.Y, Amount: -cost})
	if streamer {
		t.HP = t.MaxHP
		return
	}
	t.repairing = true
	g.order(key, c.Name, t.Cell)
}
