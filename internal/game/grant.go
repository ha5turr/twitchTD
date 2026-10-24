package game

import (
	"fmt"
	"math"
	"slices"
)

// Late newcomers: a player who first joins a game from GrantFromWave on gets
// a one-off grant to catch up, and every player raises the difficulty only in
// proportion to what they have built, so a fresh mercenary does not make the
// waves harder than he can help with.

// spend takes cost from a chatter's grant first, then from the wallet, and
// reports how much came from the grant. Nothing is taken when the two
// together fall short.
func (g *Game) spend(c *Chatter, cost int) (granted int, ok bool) {
	if c.Grant+c.Gold < cost {
		return 0, false
	}
	granted = min(c.Grant, cost)
	c.Grant -= granted
	c.Gold -= cost - granted
	return granted, true
}

// refund pays back gold from building t: the share bought with a grant goes
// to the treasury, the rest to the owner. It returns what the owner got.
func (g *Game) refund(t *Tower, amount int) int {
	if amount <= 0 {
		return 0
	}
	fromGrant := 0
	if t.Granted > 0 && t.Invested > 0 {
		fromGrant = amount * min(t.Granted, t.Invested) / t.Invested
	}
	g.toTreasury(fromGrant)
	c := g.Chatters[t.Owner]
	if c == nil || t.Owner == StreamerName {
		return 0
	}
	return g.addGold(c, amount-fromGrant)
}

// investedBy sums what every player has put into buildings, clowns aside.
func (g *Game) investedBy() map[string]int {
	m := map[string]int{}
	for _, t := range g.Towers {
		if t.Kind != clownKind && t.Owner != StreamerName {
			m[t.Owner] += t.Invested
		}
	}
	return m
}

// othersMedian is the median investment of the active players other than key.
func (g *Game) othersMedian(inv map[string]int, key string) float64 {
	cut := g.Now().Add(-activeWindow(g.Cfg))
	var xs []int
	for k, c := range g.Chatters {
		if k != key && k != StreamerName && c.Joined && c.LastSeen.After(cut) {
			xs = append(xs, inv[k])
		}
	}
	if len(xs) == 0 {
		return 0
	}
	slices.Sort(xs)
	n := len(xs)
	if n%2 == 1 {
		return float64(xs[n/2])
	}
	return float64(xs[n/2-1]+xs[n/2]) / 2
}

// difficultyChatters is how many players a wave is sized for. Each weighs in
// by what they have built, in full from half the others' median investment.
func (g *Game) difficultyChatters() float64 {
	inv := g.investedBy()
	cut := g.Now().Add(-activeWindow(g.Cfg))
	n := 0.0
	for k, c := range g.Chatters {
		if k == StreamerName || !c.Joined || !c.LastSeen.After(cut) {
			continue
		}
		n += g.contribution(inv, k)
	}
	return n
}

// contribution is how fully a player takes part, 0 to 1: what he has built
// against half the others' median investment. With nobody else building
// everyone counts in full.
func (g *Game) contribution(inv map[string]int, key string) float64 {
	need := g.othersMedian(inv, key) / 2
	if need <= 0 {
		return 1
	}
	return min(1, float64(inv[key])/need)
}

// arrive runs on a player's first join in a game and hands a late newcomer
// the grant.
func (g *Game) arrive(key string, c *Chatter) {
	if c.arrived {
		return
	}
	c.arrived = true
	if g.Wave < g.Cfg.GrantFromWave || g.Cfg.GrantPct <= 0 {
		return
	}
	n := int(math.Round(g.Cfg.GrantPct * g.othersMedian(g.investedBy(), key)))
	if n <= 0 {
		return
	}
	c.Grant += n
	g.emit(Event{Kind: "grant", User: c.Name, Amount: n})
	g.reply(c.Name, fmt.Sprintf("таверна скинулась новичку: %d золота подъёмных на стройку и улучшения", n))
}
