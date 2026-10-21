package game

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Souls are the currency that outlives a game. At game over everyone who
// fought earns some for the gold left in the wallet and for their part in
// the battle; achievements add a few more. Chat spends them on memecoins
// (manual requests, see memes.go).

// Achievement is a one-time goal; hidden ones are not listed until earned.
type Achievement struct {
	ID     string
	Name   string
	Souls  int
	Hidden bool
}

var Achievements = []Achievement{
	{"first_blood", "Первая кровь", 2, false},
	{"kills100", "Сотня", 3, false},
	{"kills1000", "Тысяча голов", 8, false},
	{"boss_slayer", "Цареубийца", 5, false},
	{"wave10", "Стойкий", 3, false},
	{"wave20", "Несломленный", 6, false},
	{"wave30", "Легенда таверны", 10, false},
	{"regular", "Завсегдатай", 4, false},
	{"architect", "Архитектор", 3, false},
	{"masterpiece", "Шедевр", 4, false},
	{"drunkard", "Пропойца", 3, false},
	{"zabuldyga", "Забулдыга", 2, false},
	{"lucky", "Везунчик", 4, false},
	{"high_roller", "Крупная ставка", 4, false},
	// Hidden ones, mostly about the guest boss: chat finds them by itself.
	{"nat1", "Один против двадцати", 6, true},
	{"robbed", "Обчищенный", 3, true},
	{"farted_down", "Снесло ветром", 3, true},
	{"clown_king", "Главный клоун", 8, true},
	{"clown_slayer", "Клоунада окончена", 10, true},
}

// Lifetime thresholds behind some achievements.
const (
	kills100     = 100
	kills1000    = 1000
	drinksNeeded = 50
	buildsNeeded = 50
	gamesNeeded  = 10
	streakNeeded = 3
	bigPot       = 500
)

func achievement(id string) (Achievement, bool) {
	i := slices.IndexFunc(Achievements, func(a Achievement) bool { return a.ID == id })
	if i < 0 {
		return Achievement{}, false
	}
	return Achievements[i], true
}

// unlock grants an achievement once and pays its souls.
func (g *Game) unlock(key, id string) {
	c := g.Chatters[key]
	a, ok := achievement(id)
	if c == nil || !ok || key == StreamerName || c.Ach[id] {
		return
	}
	if c.Ach == nil {
		c.Ach = map[string]bool{}
	}
	c.Ach[id] = true
	g.giveSouls(key, c, a.Souls)
	g.emit(Event{Kind: "achievement", User: c.Name, Text: a.Name, Amount: a.Souls})
	g.reply(c.Name, fmt.Sprintf("ачивка «%s», +%d душ", a.Name, a.Souls))
}

// count adds to a lifetime counter and returns the new value.
func (c *Chatter) count(stat string, n int) int {
	if c.Stats == nil {
		c.Stats = map[string]int{}
	}
	c.Stats[stat] += n
	return c.Stats[stat]
}

// onKill checks the kill achievements; boss tells a boss died.
func (g *Game) onKill(key string, c *Chatter, e *Enemy) {
	switch {
	case c.Kills >= kills1000:
		g.unlock(key, "kills1000")
		fallthrough
	case c.Kills >= kills100:
		g.unlock(key, "kills100")
		fallthrough
	default:
		g.unlock(key, "first_blood")
	}
	if g.Cfg.Enemies[e.Kind].Boss {
		g.unlock(key, "boss_slayer")
	}
	if e.Kind == sdfKind {
		g.unlock(key, "clown_slayer")
	}
}

// awardSouls runs at game over: souls for the gold left and for the fight,
// up to a cap, and the achievements for the waves held and games played.
// Only chatters who hurt an enemy this game take part, so chatting alone
// never earns souls.
func (g *Game) awardSouls(held int) {
	type earned struct {
		name  string
		souls int
	}
	var list []earned
	for key, c := range g.Chatters {
		if key == StreamerName || c.GameDamage <= 0 {
			continue
		}
		s := int(float64(c.Gold)*g.Cfg.SoulsGoldPct + c.GameDamage*g.Cfg.SoulsPerDamage + float64(c.GameKills)*g.Cfg.SoulsPerKill)
		s = min(max(s, 1), g.Cfg.SoulsGameCap)
		g.giveSouls(key, c, s)
		list = append(list, earned{c.Name, s})
		g.emit(Event{Kind: "souls", User: c.Name, Amount: s})
		if c.count("games", 1) >= gamesNeeded {
			g.unlock(key, "regular")
		}
		for _, w := range []struct {
			wave int
			id   string
		}{{10, "wave10"}, {20, "wave20"}, {30, "wave30"}} {
			if held >= w.wave {
				g.unlock(key, w.id)
			}
		}
	}
	if len(list) == 0 {
		return
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].souls != list[j].souls {
			return list[i].souls > list[j].souls
		}
		return list[i].name < list[j].name
	})
	parts := make([]string, 0, 5)
	for _, e := range list[:min(5, len(list))] {
		parts = append(parts, fmt.Sprintf("%s +%d", e.name, e.souls))
	}
	more := ""
	if len(list) > 5 {
		more = fmt.Sprintf(" и ещё %d", len(list)-5)
	}
	g.reply("", fmt.Sprintf("Таверна пала. Души за игру: %s%s. Баланс: !души", strings.Join(parts, ", "), more))
}

func (g *Game) cmdSouls(c *Chatter) {
	got := 0
	for _, a := range Achievements {
		if c.Ach[a.ID] {
			got++
		}
	}
	g.reply(c.Name, fmt.Sprintf("%s. Душ %d, ачивок %d из %d. Мемкоины: !мемы N (%d душ за мемкоин)", c.rankLine(), c.Souls, got, len(Achievements), g.Cfg.SoulsPerMemecoin))
}

func (g *Game) cmdAchievements(c *Chatter) {
	var got []string
	hidden := 0
	for _, a := range Achievements {
		switch {
		case c.Ach[a.ID]:
			got = append(got, a.Name)
		case a.Hidden:
			hidden++
		}
	}
	if len(got) == 0 {
		g.reply(c.Name, fmt.Sprintf("ачивок пока нет. Всего %d, из них %d скрытых", len(Achievements), hidden))
		return
	}
	g.reply(c.Name, fmt.Sprintf("ачивки (%d из %d): %s", len(got), len(Achievements), strings.Join(got, ", ")))
}

// Title is a rank earned by the souls gathered over all time (spending
// them does not lower it). The overlay paints it in the rarity colour.
type Title struct {
	Name  string
	Souls int // lifetime souls needed
}

// Ranks, from grey to gold. At some 40 souls a game: Ученик after a game or
// two, Скиталец after ~5, Первопроходец ~12, Странник ~30, Эталон ~75 and
// Полубог after ~175 games.
var Ranks = []Title{
	{"Неофит", 0},
	{"Ученик", 50},
	{"Скиталец", 200},
	{"Первопроходец", 500},
	{"Странник", 1200},
	{"Эталон", 3000},
	{"Полубог", 7000},
}

// earned is the lifetime souls of a chatter.
func (c *Chatter) earned() int { return c.Stats["souls"] }

// rank returns the index of the chatter's rank in Ranks.
func (c *Chatter) rank() int {
	r := 0
	for i, rk := range Ranks {
		if c.earned() >= rk.Souls {
			r = i
		}
	}
	return r
}

// giveSouls pays souls that count toward the rank and announces a new rank.
func (g *Game) giveSouls(key string, c *Chatter, n int) {
	if n <= 0 {
		return
	}
	was := c.rank()
	c.Souls += n
	c.count("souls", n)
	if now := c.rank(); now > was && key != StreamerName {
		g.emit(Event{Kind: "rank", User: c.Name, Text: Ranks[now].Name, Amount: now})
		g.reply(c.Name, fmt.Sprintf("новое звание: %s!", Ranks[now].Name))
	}
}

// rankLine is the chatter's rank and what is left to the next one.
func (c *Chatter) rankLine() string {
	r := c.rank()
	if r == len(Ranks)-1 {
		return Ranks[r].Name
	}
	return fmt.Sprintf("%s (до звания %s ещё %d душ)", Ranks[r].Name, Ranks[r+1].Name, Ranks[r+1].Souls-c.earned())
}
