package game

import "testing"

func swampGame(t *testing.T) *Game {
	g, _ := newTestGame(t)
	g.NewGame(BiomeSwamp)
	g.Phase = PhaseWave
	return g
}

func TestSlimeSplits(t *testing.T) {
	g := swampGame(t)
	e := enemyAt(g, "slime", 5, 100)
	g.damage(e, 1000, "", true)
	young := 0
	for _, o := range g.Enemies {
		if o.Kind == "slimelet" && !o.dead {
			young++
			if o.MaxHP != 100*g.Cfg.Enemies["slimelet"].HPMult {
				t.Fatalf("the young are a share of the slime: %.0f", o.MaxHP)
			}
		}
	}
	if young != 2 {
		t.Fatalf("a slime dies into two: %d", young)
	}
	for _, o := range append([]*Enemy(nil), g.Enemies...) {
		if o.Kind == "slimelet" {
			g.damage(o, 1000, "", true)
		}
	}
	for _, o := range g.Enemies {
		if !o.dead {
			t.Fatal("the young do not split again")
		}
	}
}

func TestBatsFlyOverBarricades(t *testing.T) {
	g := swampGame(t)
	for _, p := range g.Map.Path {
		place(g, "wall", p)
	}
	bat := g.spawnEnemy("bat", 1e9, 0)
	walker := g.spawnEnemy("slime", 1e9, 0)
	run(g, 60)
	if !bat.AtGate {
		t.Fatalf("a bat flies straight to the gate over every barricade: dist %.1f of %.1f", bat.Dist, g.total)
	}
	if walker.AtGate {
		t.Fatal("while walkers are held")
	}
	line := g.gate().sub(g.route[0]).len()
	if line >= g.total {
		t.Skip("a straight route: nothing to cut")
	}
}

func TestGoblinShamanDispels(t *testing.T) {
	g := swampGame(t)
	sh := enemyAt(g, "goblin_shaman", 5, 1000)
	sk := enemyAt(g, "slime", 5, 1000)
	sk.netLeft, sk.burnLeft, sk.curseLeft, sk.slowLeft, sk.slowMult = 5, 5, 5, 5, 0.5
	sh.ability = 0
	g.tickDispel(sh, g.Cfg.Enemies["goblin_shaman"], 0.1)
	if sk.netLeft > 0 || sk.burnLeft > 0 || sk.curseLeft > 0 || sk.slowMult != 1 {
		t.Fatal("the shaman washes nets, burns, curses and slows off his neighbours")
	}
}

func TestWitchHexesTheBestBuilding(t *testing.T) {
	g := swampGame(t)
	w := g.spawnBoss("swamp_witch", 1e6)
	w.Dist = g.cellDist[6]
	w.Pos = g.routePos(w.Dist)
	var low, high *Tower
	for _, p := range g.Map.Path {
		for _, d := range dirs {
			q := Pos{p.X + d.X, p.Y + d.Y}
			if q.In() && g.Map.Grid.At(q) == CellBuild && g.Towers[q] == nil && center(q).sub(w.Pos).len() <= hexRadius {
				if low == nil {
					low = place(g, "arrow", q)
				} else if high == nil {
					high = place(g, "arrow", q)
					high.Level = 4
				}
			}
		}
	}
	if high == nil {
		t.Skip("no room around the witch")
	}
	g.hex(w)
	if high.hexLeft <= 0 || high.Disabled <= 0 || low.hexLeft > 0 {
		t.Fatal("the witch turns the best building near her into a frog")
	}
}

func TestHydraGrowsHeadsUnlessBurning(t *testing.T) {
	g := swampGame(t)
	h := g.spawnBoss("hydra", 1000)
	g.tickHydra(h)
	if h.heads != hydraHeads {
		t.Fatalf("born with %d heads: %d", hydraHeads, h.heads)
	}
	h.HP = 700
	g.tickHydra(h)
	if h.heads != hydraHeads+1 {
		t.Fatal("a quarter lost grows a head")
	}
	speed, blows := h.fury()
	if speed <= 1 || blows <= 1 {
		t.Fatal("more heads, faster and harder")
	}
	h.HP, h.burnLeft = 450, 2
	g.tickHydra(h)
	if h.heads != hydraHeads+1 {
		t.Fatal("a burning stump grows nothing")
	}
	h.HP, h.burnLeft = 100, 0
	g.tickHydra(h)
	if h.heads != hydraHeads+2 {
		t.Fatalf("the last quarter grows one more: %d", h.heads)
	}
}

func TestSwampIsABiome(t *testing.T) {
	g, _ := newTestGame(t)
	g.NewGame(BiomeSwamp)
	if g.Map.Biome != BiomeSwamp || len(biomeBosses[BiomeSwamp]) != 2 {
		t.Fatal("the swamp has its map and two bosses")
	}
	g.Wave = 4
	g.StartWave()
	if last := g.queue[len(g.queue)-1]; last.kind != "swamp_witch" && last.kind != "hydra" && last.kind != sdfKind {
		t.Fatalf("a swamp boss comes on the boss wave: %s", last.kind)
	}
}
