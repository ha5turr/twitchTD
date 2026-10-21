package game

import (
	"maps"
	"math"
	"slices"
	"strconv"
)

// Patch 12: event guests. Now and then (EventChance per ordinary wave from
// EventFromWave on) a guest joins the wave instead of a boss:
//   - the tax collector walks to the gate for the treasury: at the gate he
//     takes half of it and leaves; killed, he adds a bonus to it;
//   - the golden golem attacks nothing and runs fast: killed, it bursts into
//     gold for everyone who hit it, by their share of the damage; if it gets
//     away, nothing is lost but the gold.

const (
	taxKind   = "tax_collector"
	golemKind = "golden_golem"
)

var eventKinds = []string{taxKind, golemKind}

// isGuest reports whether an enemy is an event guest.
func isGuest(kind string) bool { return slices.Contains(eventKinds, kind) }

// rollGuest decides whether this wave brings a guest and slips it into the
// middle of the queue. Boss waves never do.
func (g *Game) rollGuest(bossWave bool) string {
	if bossWave || g.Wave < g.Cfg.EventFromWave || g.rng.Float64() >= g.Cfg.EventChance {
		return ""
	}
	kind := eventKinds[g.rng.IntN(len(eventKinds))]
	s := spawn{kind, g.waveHP * g.Cfg.BossHPMult * g.Cfg.Enemies[kind].HPMult}
	at := len(g.queue) / 2
	g.queue = slices.Insert(g.queue, at, s)
	return kind
}

// guestAtGate handles a guest that reached the tavern: the tax collector takes
// his cut, the golem simply runs off. Either way it is gone, unpaid.
func (g *Game) guestAtGate(e *Enemy) {
	e.dead = true
	switch e.Kind {
	case taxKind:
		took := g.Treasury / 2
		g.Treasury -= took
		g.emit(Event{Kind: "taxed", Amount: took})
		g.reply("", "Сборщик податей добрался до таверны и унёс половину казны")
	case golemKind:
		g.emit(Event{Kind: "golemGone"})
	}
}

// guestKilled pays out what a slain guest leaves behind.
func (g *Game) guestKilled(e *Enemy) {
	switch e.Kind {
	case taxKind:
		bonus := int(math.Round(float64(30+10*g.Wave) * g.rewardMult))
		g.toTreasury(bonus)
		g.emit(Event{Kind: "taxKilled", X: e.Pos.X, Y: e.Pos.Y, Amount: bonus})
	case golemKind:
		g.goldRain(e)
	}
}

// goldRain shares the golem's gold among everyone who hurt it, by their
// part of the damage; everyone who hit it gets at least one coin.
func (g *Game) goldRain(e *Enemy) {
	pool := float64(40+15*g.Wave) * g.rewardMult
	total := 0.0
	for _, d := range e.hurtBy {
		total += d
	}
	paid, n := 0, 0
	if total > 0 {
		for _, key := range slices.Sorted(maps.Keys(e.hurtBy)) {
			c := g.Chatters[key]
			if c == nil || key == StreamerName {
				continue
			}
			got := g.addGold(c, max(1, int(math.Round(pool*e.hurtBy[key]/total))))
			paid += got
			n++
		}
	}
	g.emit(Event{Kind: "goldRain", X: e.Pos.X, Y: e.Pos.Y, Amount: paid, Text: strconv.Itoa(n)})
}

// tracksDamage reports whether an enemy remembers who hurt it and how much.
func tracksDamage(kind string) bool { return kind == sdfKind || kind == golemKind }
