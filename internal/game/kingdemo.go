package game

import "slices"

const demoDispelSec = 6.0

// tickDemo lets the demo's players wipe the Yellow Signs as a chat would.
func (g *Game) tickDemo() {
	for _, s := range slices.Clone(g.signs) {
		if g.Cfg.KingSignSec-s.Left < demoDispelSec {
			continue
		}
		for _, name := range g.demoDispel {
			g.HandleChat(ChatMessage{User: name, Text: "!рз " + s.Tower.Cell.Name()})
		}
	}
}

// KingDemo sets up the final for a look at the King in Yellow (the server's
// -kingdemo flag, for development): five players of different classes, each
// with as many artifact buildings as the limit after wave 34 allows, on the
// cells that see the most of the path, the warrior's barricades across it;
// then the build phase runs out in a few seconds and the king's scene starts.
// Two of them wipe each Yellow Sign off demoDispelSec after it falls.
// With warlock a warlock takes the warrior's place: no barricades, his
// altars in pairs across the path so their soul links net it.
func (g *Game) KingDemo(biome Biome, warlock bool) {
	classes := []string{"ranger", "mage", "pyro", "shaman", "warrior"}
	if warlock {
		classes[4] = "warlock"
	}
	g.KingDemoTeam(biome, classes)
}

// demoNames are the demo players' names by class.
var demoNames = map[string]string{
	"ranger": "Следопыт", "mage": "Маг", "pyro": "Пиромант", "shaman": "Шаман", "warrior": "Воитель",
	"warlock": "Чернокнижник", "cleric": "Жрец", "rogue": "Разбойник", "bard": "Бард", "druid": "Друид",
}

// KingDemoTeam is KingDemo with a team of these classes, one player each,
// every one building up to the limit a team of that size has (a small
// team's bonus too).
func (g *Game) KingDemoTeam(biome Biome, classes []string) {
	g.NewGame(biome)
	type member struct{ name, class string }
	var party []member
	for _, cl := range classes {
		party = append(party, member{demoNames[cl], cl})
	}
	for _, p := range party {
		g.HandleChat(ChatMessage{User: p.name, Text: "!play"})
		if _, c := g.findChatter(p.name); c != nil {
			c.Class = p.class
		}
	}
	g.Wave = 34
	limit := g.TowerLimit() // after wave 34, with a small team's bonus
	type cell struct {
		p Pos
		n int
	}
	var cells []cell
	for y := 0; y < GridH; y++ {
		for x := 0; x < GridW; x++ {
			p := Pos{x, y}
			if g.Map.Grid.At(p) != CellBuild {
				continue
			}
			n := 0
			for _, q := range g.Map.Path {
				if center(q).sub(center(p)).len() <= 2.4 {
					n++
				}
			}
			if n > 0 {
				cells = append(cells, cell{p, n})
			}
		}
	}
	slices.SortFunc(cells, func(a, b cell) int { return b.n - a.n })
	path := g.Map.Path
	var altars []Pos
	if slices.Contains(classes, "warlock") {
		altars = g.altarNet(limit) // only a warlock's altars take those cells
	}
	taken := map[Pos]bool{}
	for _, p := range altars {
		taken[p] = true
	}
	k := 0
	for i := 0; i < limit; i++ {
		for _, p := range party {
			key, _ := g.findChatter(p.name)
			spec, _ := classByKey(p.class)
			at := Pos{-1, -1}
			switch {
			case g.Cfg.Towers[spec.Tower].Role == RoleWall:
				at = path[len(path)/3+i*(len(path)*2/3-2)/limit] // across the path's second half
			case spec.Tower == "curse" && i < len(altars):
				at = altars[i]
			default:
				for k < len(cells) && taken[cells[k].p] {
					k++
				}
				if k < len(cells) {
					at = cells[k].p
					k++
				}
			}
			if at.X < 0 || g.Towers[at] != nil {
				continue
			}
			t := &Tower{ID: g.id(), Kind: spec.Tower, Cell: at, Owner: key, Level: g.Cfg.Towers[spec.Tower].MaxLevel + 1, Built: true}
			g.Towers[at] = t
			g.readyTower(t)
		}
	}
	for _, p := range party[:min(2, len(party))] {
		g.demoDispel = append(g.demoDispel, p.name) // they wipe the Yellow Signs, as a chat would
	}
	g.finalDue = true
	g.matchElapsed = g.Cfg.MatchMinutes * 60
	g.Phase, g.Timer = PhaseBuild, 5
}

// altarNet picks up to n cells for a warlock's altars: pairs facing each
// other across the path, a pair every couple of cells along it, so every
// pair's link crosses the path and neighbouring pairs link too.
func (g *Game) altarNet(n int) []Pos {
	free := func(p Pos) bool {
		return p.X >= 0 && p.Y >= 0 && p.X < GridW && p.Y < GridH && g.Map.Grid.At(p) == CellBuild
	}
	var out []Pos
	var last vec
	path := g.Map.Path
	for i := len(path) / 5; i < len(path)-2 && len(out)+2 <= n; i++ {
		p := path[i]
		for _, d := range []Pos{{1, 0}, {0, 1}} {
			a, b := Pos{p.X - d.X, p.Y - d.Y}, Pos{p.X + d.X, p.Y + d.Y}
			if !free(a) || !free(b) || slices.Contains(out, a) || slices.Contains(out, b) {
				continue
			}
			if len(out) > 0 && center(p).sub(last).len() < 2 {
				continue // spread the pairs along the path
			}
			out = append(out, a, b)
			last = center(p)
			break
		}
	}
	return out
}
