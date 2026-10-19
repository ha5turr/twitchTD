package game

import "strings"

// ClassSpec is a mercenary class. Its signature building gets ClassBonus
// extra damage; any class may build anything.
type ClassSpec struct {
	Key     string
	Name    string
	Tower   string // signature building
	Aliases []string
}

var Classes = []ClassSpec{
	{"ranger", "Следопыт", "arrow", []string{"следопыт", "ranger", "лучник"}},
	{"mage", "Маг", "magic", []string{"маг", "mage", "волшебник"}},
	{"pyro", "Пиромант", "fire", []string{"пиромант", "pyro", "pyromancer"}},
	{"cleric", "Жрец", "shrine", []string{"жрец", "cleric", "priest"}},
	{"rogue", "Разбойник", "trap", []string{"разбойник", "rogue", "вор"}},
	{"bard", "Бард", "drums", []string{"бард", "bard"}},
	{"warrior", "Воитель", "wall", []string{"воитель", "warrior", "воин", "fighter"}},
	// The druid has no signature building: it turns into a panther (rogues.go).
	{"druid", "Друид", "", []string{"друид", "druid"}},
}

func classByKey(key string) (ClassSpec, bool) {
	for _, c := range Classes {
		if c.Key == key {
			return c, true
		}
	}
	return ClassSpec{}, false
}

func classByAlias(s string) (ClassSpec, bool) {
	s = strings.ToLower(s)
	for _, c := range Classes {
		for _, a := range c.Aliases {
			if a == s {
				return c, true
			}
		}
	}
	return ClassSpec{}, false
}

// classBonus is the damage multiplier the owner's class gives to a tower.
func (g *Game) classBonus(owner, tower string) float64 {
	c := g.Chatters[owner]
	if c == nil {
		return 1
	}
	if spec, ok := classByKey(c.Class); ok && spec.Tower == tower {
		return 1 + g.Cfg.ClassBonus
	}
	return 1
}
