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
}

var drinkAliases = map[string]string{
	"ale": "ale", "эль": "ale", "пиво": "ale",
	"mead": "mead", "медовуха": "mead", "мёд": "mead", "мед": "mead",
	"potion": "potion", "зелье": "potion", "чёрное": "potion",
}

func (g *Game) cmdDrink(c *Chatter, key string, args []string) {
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
	c.Drunk += float64(d.Mugs)
	c.Drinks++
	c.GameDrinks++
	g.emit(Event{Kind: "drink", User: c.Name, Text: kind, Amount: int(math.Ceil(c.Drunk))})
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
