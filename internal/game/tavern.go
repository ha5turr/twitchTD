package game

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// DrinkSpec is one item on the tavern menu. Drinking is purely cosmetic: it
// costs gold, makes the mercenary tipsy and counts toward the drinkers' board.
type DrinkSpec struct {
	Name  string `json:"name"`
	Price int    `json:"price"`
	Mugs  int    `json:"mugs"` // how much it adds to drunkenness
	// Effect on the drinker's buildings, see drinks.go.
	Boost float64 `json:"boost"`
	After float64 `json:"after"`
	Sec   float64 `json:"sec"`
}

var drinkAliases = map[string]string{
	"ale": "ale", "эль": "ale", "пиво": "ale",
	"mead": "mead", "медовуха": "mead", "мёд": "mead", "мед": "mead",
	"potion": "potion", "зелье": "potion", "чёрное": "potion",
}

func (g *Game) cmdDrink(c *Chatter, key string, args []string) {
	if g.busyAtDice(c, key) {
		return
	}
	kind := "ale"
	if len(args) > 0 {
		k, ok := drinkAliases[strings.ToLower(args[0])]
		if !ok {
			g.reply(c.Name, "в меню: эль, медовуха, зелье")
			return
		}
		kind = k
	}
	d := g.Cfg.Drinks[kind]
	if g.Workers[key] != nil {
		g.reply(c.Name, "твой наёмник сейчас в поле")
		return
	}
	if c.Gold < d.Price && key != StreamerName {
		g.reply(c.Name, fmt.Sprintf("%s стоит %d золота, у тебя %d", d.Name, d.Price, c.Gold))
		return
	}
	if key != StreamerName {
		c.Gold -= d.Price
	}
	g.reply(c.Name, strings.ToLower(d.Name)+": "+g.serve(key, c, kind))
}

// serve pours a drink: drunkenness, the drinkers' board and its effect,
// which it returns as words for the chat.
func (g *Game) serve(key string, c *Chatter, kind string) string {
	c.Drunk += float64(g.Cfg.Drinks[kind].Mugs)
	c.Drinks++
	c.GameDrinks++
	if c.Drinks >= drinksNeeded {
		g.unlock(key, "drunkard")
	}
	g.emit(Event{Kind: "drink", User: c.Name, Text: kind, Amount: int(math.Ceil(c.Drunk))})
	effect := g.drinkEffect(key, c, kind)
	if g.Cfg.PassOutMugs > 0 && math.Ceil(c.Drunk) >= g.Cfg.PassOutMugs && c.out <= 0 && key != StreamerName {
		g.passOut(key, c)
	}
	return effect
}

// drunkStatus tells !me how drunk the mercenary is.
func (g *Game) drunkStatus(c *Chatter) string {
	mugs := int(math.Ceil(c.Drunk))
	switch {
	case c.out > 0:
		return fmt.Sprintf("храпит под столом ещё %d с", int(math.Ceil(c.out)))
	case mugs == 0:
		return "трезв"
	case g.Cfg.PassOutMugs > 0:
		return fmt.Sprintf("пьян: %d из %d кружек до отключки", mugs, int(g.Cfg.PassOutMugs))
	}
	return fmt.Sprintf("пьян: %d кружек", mugs)
}

// passOut puts an overdrunk mercenary under the table: he hears no
// commands until he sleeps it off.
func (g *Game) passOut(key string, c *Chatter) {
	c.out = g.Cfg.PassOutSec
	g.emit(Event{Kind: "passOut", User: c.Name})
	g.reply(c.Name, fmt.Sprintf("перебрал и храпит под столом: %d с не слышит команд", int(g.Cfg.PassOutSec)))
	g.unlock(key, "zabuldyga")
}

// asleep tells a passed out chatter (now and then) why nothing happens.
func (g *Game) asleep(c *Chatter) bool {
	if c.out <= 0 {
		return false
	}
	if c.snored <= 0 {
		c.snored = 20
		g.reply(c.Name, fmt.Sprintf("храпит под столом… ещё %d с", int(math.Ceil(c.out))))
	}
	return true
}

func (g *Game) cmdClass(c *Chatter, args []string) {
	cur, _ := classByKey(c.Class)
	if len(args) == 0 {
		names := make([]string, len(Classes))
		for i, cl := range Classes {
			names[i] = strings.ToLower(cl.Name)
		}
		g.reply(c.Name, fmt.Sprintf("твой класс: %s. Сменить: !class %s", cur.Name, strings.Join(names, "/")))
		return
	}
	next, ok := classByAlias(args[0])
	switch {
	case !ok:
		g.reply(c.Name, "нет такого класса, список: !class")
	case next.Key == c.Class:
		g.reply(c.Name, "ты уже "+strings.ToLower(next.Name))
	case c.classLocked:
		g.reply(c.Name, "класс можно сменить один раз за игру")
	default:
		c.Class = next.Key
		c.classLocked = true
		g.emit(Event{Kind: "class", User: c.Name, Text: next.Key})
	}
}

// soberUp lets the drink wear off.
func (g *Game) soberUp(dt float64) {
	for _, c := range g.Chatters {
		if c.Drunk > 0 {
			c.Drunk = max(0, c.Drunk-dt/g.Cfg.DrunkDecaySec)
		}
		if c.out > 0 {
			if c.out -= dt; c.out <= 0 {
				g.emit(Event{Kind: "wakeUp", User: c.Name})
			}
		}
		c.snored = max(0, c.snored-dt)
	}
}

// Summary is the game over screen: how far the tavern held and who shone.
type Summary struct {
	Wave   int    `json:"wave"`
	Biome  Biome  `json:"biome"`
	Damage []Rank `json:"damage"`
	Kills  []Rank `json:"kills"`
	Drinks []Rank `json:"drinks"`
}

type Rank struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func (g *Game) makeSummary() *Summary {
	top := func(val func(*Chatter) int) []Rank {
		var out []Rank
		for key, c := range g.Chatters {
			if v := val(c); v > 0 && key != StreamerName {
				out = append(out, Rank{c.Name, v})
			}
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Value != out[j].Value {
				return out[i].Value > out[j].Value
			}
			return out[i].Name < out[j].Name
		})
		return out[:min(3, len(out))]
	}
	return &Summary{
		Wave:   max(g.Wave-1, 0), // the wave that broke through does not count
		Biome:  g.Map.Biome,
		Damage: top(func(c *Chatter) int { return int(c.GameDamage) }),
		Kills:  top(func(c *Chatter) int { return c.GameKills }),
		Drinks: top(func(c *Chatter) int { return c.GameDrinks }),
	}
}
