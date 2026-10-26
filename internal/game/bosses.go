package game

import (
	"cmp"
	"slices"
)

// weighted is an enemy kind in a biome's pool, available from a given wave.
type weighted struct {
	kind   string
	from   int
	weight float64
}

var biomeEnemies = map[Biome][]weighted{
	BiomeRuins:   {{"skeleton", 1, 6}, {"ghoul", 2, 3}, {"cultist", 4, 2}, {"bomber", 5, 2}, {"shieldbearer", 6, 2}},
	BiomeDungeon: {{"spider", 1, 4}, {"rat", 1, 3}, {"troll", 4, 2}, {"bomber", 5, 3}},
	BiomeCrypt:   {{"skeleton", 1, 3}, {"mummy", 1, 3}, {"ghost", 2, 3}, {"necromancer", 4, 2}, {"shieldbearer", 6, 2}},
	BiomeSwamp:   {{"slime", 1, 4}, {"ghoul", 1, 2}, {"bat", 2, 4}, {"bomber", 4, 2}, {"goblin_shaman", 4, 3}},
	BiomeIce:     {{"skeleton", 1, 2}, {"iceskel", 1, 5}, {"frostspirit", 2, 3}, {"shieldbearer", 6, 1}, {"bomber", 8, 1}},
	BiomeRiver:   {{"pirate", 1, 5}, {"drowned", 1, 3}, {"deep_one", 2, 3}, {"bomber", 5, 2}},
}

// Each biome has two bosses that take turns every BossEvery waves.
var biomeBosses = map[Biome][]string{
	BiomeRuins:   {"bone_knight", "cult_leader"},
	BiomeDungeon: {"spider_queen", "cave_troll"},
	BiomeCrypt:   {"lich", "ghost_king"},
	BiomeIce:     {"ice_golem", "frost_demon"},
	BiomeSwamp:   {"swamp_witch", "hydra"},
	BiomeRiver:   {"octopus", "leviathan"},
}

func pickEnemy(roll float64, pool []weighted, wave int) string {
	total := 0.0
	for _, w := range pool {
		if wave >= w.from {
			total += w.weight
		}
	}
	roll *= total
	for _, w := range pool {
		if wave < w.from {
			continue
		}
		roll -= w.weight
		if roll <= 0 {
			return w.kind
		}
	}
	return pool[0].kind
}

// tickBoss runs the boss's ability on its cooldown.
func (g *Game) tickBoss(e *Enemy, dt float64) {
	e.ability -= dt
	if e.ability > 0 {
		return
	}
	ev := Event{Kind: "ability", Text: e.Kind, X: e.Pos.X, Y: e.Pos.Y}
	switch e.Kind {
	case "bone_knight": // raises the shield for 5 s, lowers it for 10 s
		e.shieldUp = !e.shieldUp
		e.ability = map[bool]float64{true: 5, false: 10}[e.shieldUp]
		if !e.shieldUp {
			return // lowering the shield is not announced
		}
	case "cult_leader": // the ritual raises fallen cultists
		e.ability = 14 // hotfix 4: fewer healers
		g.summon(e, "cultist", 2)
	case "spider_queen": // webs the nearest building and lays spiderlings
		e.ability = 7
		g.disableTowers(e, 3, 3, 1)
		g.summon(e, "spider", 1)
	case "cave_troll": // throws a rock that stuns a building
		e.ability = 6
		g.disableTowers(e, 3.5, 2, 1)
	case "lich": // raises the dead as skeletons
		e.ability = 6
		g.summon(e, "skeleton", 3)
	case "ghost_king": // steps through the veil, three cells ahead
		e.ability = 8
		e.Dist = min(e.Dist+3, g.total-0.5)
		e.blocked = nil
		e.Pos = g.routePos(e.Dist)
		ev.X, ev.Y = e.Pos.X, e.Pos.Y
	case sdfKind:
		ev = g.sdfTrick(e)
		e.ability = sdfEvery
	case "ice_golem": // freezes every building around it
		e.ability = 7
		g.disableTowers(e, 2, 3, 0)
	case "swamp_witch": // turns the best building near her into a frog
		e.ability = 8
		g.hex(e)
	case "hydra": // its heads grow with its wounds (tickHydra), nothing on a timer
		e.ability = 1e9
		return
	case "octopus": // sets an octopling on a building
		e.ability = clingEvery
		g.cling(e)
	case "leviathan": // raises the tide and swims deep
		e.ability = tideEvery
		g.tide(e)
	case "frost_demon": // ices the path: everyone runs faster for a while
		e.ability = 10
		g.rushLeft = 5
	default:
		e.ability = 1e9
		return
	}
	g.emit(ev)
}

// summon raises n enemies just behind the boss.
func (g *Game) summon(boss *Enemy, kind string, n int) {
	for i := 0; i < n; i++ {
		e := g.spawnEnemy(kind, g.waveHP*g.Cfg.Enemies[kind].HPMult*0.6, max(0, boss.Dist-0.3-0.25*float64(i)))
		e.summoned = true
	}
}

// disableTowers takes up to n buildings within radius out of action for secs
// (n = 0: all of them), nearest first.
func (g *Game) disableTowers(boss *Enemy, radius, secs float64, n int) {
	var hit []*Tower
	for _, t := range g.Towers {
		if t.Built && center(t.Cell).sub(boss.Pos).len() <= radius {
			hit = append(hit, t)
		}
	}
	slices.SortFunc(hit, func(a, b *Tower) int {
		return cmp.Compare(center(a.Cell).sub(boss.Pos).len(), center(b.Cell).sub(boss.Pos).len())
	})
	if n > 0 && len(hit) > n {
		hit = hit[:n]
	}
	for _, t := range hit {
		t.Disabled = max(t.Disabled, secs)
		cp := center(t.Cell)
		g.emit(Event{Kind: "disabled", Text: boss.Kind, X: cp.X, Y: cp.Y})
	}
}
