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
func (g *Game) KingDemo(biome Biome) {
	g.NewGame(biome)
	party := []struct{ name, class string }{
		{"Следопыт", "ranger"}, {"Маг", "mage"}, {"Пиромант", "pyro"}, {"Шаман", "shaman"}, {"Воитель", "warrior"},
	}
	for _, p := range party {
		g.HandleChat(ChatMessage{User: p.name, Text: "!play"})
		if _, c := g.findChatter(p.name); c != nil {
			c.Class = p.class
		}
	}
	g.Wave = 34
	limit := g.limitAfter(g.Wave)
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
	k := 0
	for i := 0; i < limit; i++ {
		for _, p := range party {
			key, _ := g.findChatter(p.name)
			spec, _ := classByKey(p.class)
			at := Pos{-1, -1}
			if g.Cfg.Towers[spec.Tower].Role == RoleWall {
				at = path[len(path)/3+i*(len(path)*2/3-2)/limit] // across the path's second half
			} else if k < len(cells) {
				at = cells[k].p
				k++
			}
			if at.X < 0 || g.Towers[at] != nil {
				continue
			}
			t := &Tower{ID: g.id(), Kind: spec.Tower, Cell: at, Owner: key, Level: g.Cfg.Towers[spec.Tower].MaxLevel + 1, Built: true}
			g.Towers[at] = t
			g.readyTower(t)
		}
	}
	g.demoDispel = []string{party[0].name, party[1].name} // they wipe the Yellow Signs, as a chat would
	g.finalDue = true
	g.matchElapsed = g.Cfg.MatchMinutes * 60
	g.Phase, g.Timer = PhaseBuild, 5
}
