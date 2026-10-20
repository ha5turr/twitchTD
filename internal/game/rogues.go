package game

import (
	"fmt"
	"math"
	"slices"
)

// Class passives (hotfix 2).
//   - Разбойник: every RogueEverySec a rogue in the tavern lifts a little gold
//     from a random other mercenary sitting there.
//   - Друид: no building bonus; instead, during a wave, the mercenary turns
//     into a black panther every DruidEverySec, leaves the tavern and mauls
//     enemies for DruidPantherSec, each one once.

// Panther is a druid on the hunt.
type Panther struct {
	Owner string
	Name  string
	Pos   vec
	left  float64
	hit   map[int]bool // enemies already mauled
}

// tavernMates lists the active mercenaries sitting in the tavern, sorted.
func (g *Game) tavernMates() []string {
	cut := g.Now().Add(-activeWindow(g.Cfg))
	var out []string
	for key, c := range g.Chatters {
		if key != StreamerName && c.Joined && c.LastSeen.After(cut) && g.Workers[key] == nil && g.panther(key) == nil && g.runOf(key) == nil {
			out = append(out, key)
		}
	}
	slices.Sort(out)
	return out
}

func (g *Game) tickRogues(dt float64) {
	if g.Phase == PhaseGameOver {
		return
	}
	mates := g.tavernMates()
	for _, key := range mates {
		c := g.Chatters[key]
		if c.Class != "rogue" {
			continue
		}
		if c.passiveCD == 0 { // a rogue just sat down: the first theft comes later
			c.passiveCD = g.Cfg.RogueEverySec
		}
		c.passiveCD -= dt
		if c.passiveCD > 0 {
			continue
		}
		c.passiveCD = g.Cfg.RogueEverySec
		var marks []string
		for _, k := range mates {
			if k != key && g.Chatters[k].Gold > 0 {
				marks = append(marks, k)
			}
		}
		if len(marks) == 0 {
			continue
		}
		v := g.Chatters[marks[g.rng.IntN(len(marks))]]
		take := min(v.Gold, g.Cfg.RogueStealMax, max(1, int(math.Round(float64(v.Gold)*g.Cfg.RogueStealPct))))
		v.Gold -= take
		got := g.addGold(c, take)
		g.emit(Event{Kind: "pickpocket", User: c.Name, Text: v.Name, Amount: got})
		g.reply(v.Name, fmt.Sprintf("%s стащил у тебя %d золота", c.Name, take))
	}
}

func (g *Game) panther(key string) *Panther {
	i := slices.IndexFunc(g.Panthers, func(p *Panther) bool { return p.Owner == key })
	if i < 0 {
		return nil
	}
	return g.Panthers[i]
}

// tickDruids turns druids sitting in the tavern into panthers during a
// wave, and runs the hunt.
func (g *Game) tickDruids(dt float64) {
	if g.Phase == PhaseWave {
		for _, key := range g.tavernMates() {
			c := g.Chatters[key]
			if c.Class != "druid" {
				continue
			}
			c.passiveCD -= dt
			if c.passiveCD > 0 || len(g.Enemies) == 0 {
				continue
			}
			c.passiveCD = g.Cfg.DruidEverySec
			g.Panthers = append(g.Panthers, &Panther{Owner: key, Name: c.Name, Pos: g.gate(), left: g.Cfg.DruidPantherSec, hit: map[int]bool{}})
			g.emit(Event{Kind: "panther", User: c.Name})
		}
	}
	alive := g.Panthers[:0]
	for _, p := range g.Panthers {
		p.left -= dt
		var prey *Enemy
		if p.left > 0 {
			for _, e := range g.Enemies {
				if !e.dead && !p.hit[e.ID] && (prey == nil || e.Pos.sub(p.Pos).len() < prey.Pos.sub(p.Pos).len()) {
					prey = e
				}
			}
		}
		if prey == nil { // the hunt is over: back to the tavern
			if g.step2(&p.Pos, g.gate(), g.Cfg.DruidPantherSpeed*dt) {
				continue
			}
			alive = append(alive, p)
			continue
		}
		if g.step2(&p.Pos, prey.Pos, g.Cfg.DruidPantherSpeed*dt) {
			p.hit[prey.ID] = true
			g.damage(prey, g.skillHP()*g.Cfg.DruidMaul, p.Owner, false)
			g.emit(Event{Kind: "maul", User: p.Name, X: p.Pos.X, Y: p.Pos.Y})
		}
		alive = append(alive, p)
	}
	g.Panthers = alive
}

// step2 moves a point toward target and reports whether it arrived.
func (g *Game) step2(pos *vec, target vec, s float64) bool {
	d := target.sub(*pos)
	if d.len() <= s {
		*pos = target
		return true
	}
	*pos = vec{pos.X + d.X/d.len()*s, pos.Y + d.Y/d.len()*s}
	return false
}
