package game

import "testing"

// placeAs puts a finished building owned by a chatter at cell.
func placeAs(g *Game, owner, kind string, p Pos) *Tower {
	t := place(g, kind, p)
	t.Owner = owner
	g.readyTower(t)
	return t
}

// pathNeighbours returns two free buildable cells next to each other and
// next to the path, and the path index the first one touches.
func pathNeighbours(t *testing.T, g *Game) (Pos, Pos, int) {
	t.Helper()
	for i, p := range g.Map.Path {
		for _, d := range dirs {
			a := Pos{p.X + d.X, p.Y + d.Y}
			if !a.In() || g.Map.Grid.At(a) != CellBuild {
				continue
			}
			for _, d2 := range dirs {
				b := Pos{a.X + d2.X, a.Y + d2.Y}
				if b.In() && g.Map.Grid.At(b) == CellBuild && b != a {
					return a, b, i
				}
			}
		}
	}
	t.Fatal("no room by the path")
	return Pos{}, Pos{}, 0
}

func chatters(g *Game, names ...string) {
	for _, n := range names {
		g.HandleChat(ChatMessage{User: n, Text: "привет"})
		g.Chatters[n].Gold = 0
	}
}

func TestDrumsShareKills(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	chatters(g, "anna", "boris")
	a, b, i := pathNeighbours(t, g)
	placeAs(g, "anna", "arrow", a)
	placeAs(g, "boris", "drums", b)
	enemyAt(g, "skeleton", i, 1)
	g.spawnEnemy("troll", 1e9, 0) // keeps the wave going, so no wave bonus mixes in
	run(g, 2)
	bounty := g.Cfg.Enemies["skeleton"].Bounty
	if g.Chatters["anna"].Gold < bounty {
		t.Fatalf("the killer is paid: %d", g.Chatters["anna"].Gold)
	}
	if want := int(float64(bounty)*g.Cfg.DrumAssistPct + 0.5); g.Chatters["boris"].Gold != want {
		t.Fatalf("the drummer gets an assist of %d, got %d", want, g.Chatters["boris"].Gold)
	}
	if g.Cfg.Towers["drums"].AuraRange != 3 {
		t.Fatal("drums reach three cells")
	}
}

func TestShrinesMendBarricadesAfterAWave(t *testing.T) {
	g, _ := newTestGame(t)
	wall := placeAs(g, "anna", "wall", g.Map.Path[4])
	a, _, _ := pathNeighbours(t, g)
	placeAs(g, "boris", "shrine", a)
	wall.HP = wall.MaxHP / 2
	g.endWave()
	if want := wall.MaxHP * (0.5 + g.Cfg.ShrineWallRepair); wall.HP < want-0.01 || wall.HP > want+0.01 {
		t.Fatalf("a shrine mends every barricade a little: hp %.1f want %.1f", wall.HP, want)
	}
}

func TestBarricadeHoldsForGoldAndRefundsWhenBroken(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	chatters(g, "anna", "vova")
	g.Chatters["anna"].Class = "mage"
	g.Chatters["vova"].Class = "warrior"
	wall := placeAs(g, "anna", "wall", g.Map.Path[6])
	wall.Invested = 100
	enemyAt(g, "skeleton", 6, 1e9)
	run(g, 4)
	if g.Chatters["anna"].Gold < 1 {
		t.Fatal("holding enemies pays a little")
	}
	gold := g.Chatters["anna"].Gold
	wall.HP = 0.01
	run(g, 0.2)
	if got := g.Chatters["anna"].Gold - gold; got < g.Cfg.WallRefundPct || got > g.Cfg.WallRefundPct+1 {
		t.Fatalf("a broken barricade gives half its price back: %d", got)
	}
	wall2 := placeAs(g, "vova", "wall", g.Map.Path[8])
	wall2.Invested = 100
	if got := g.wallRefund(wall2); got != g.Cfg.WallRefundClass {
		t.Fatalf("the Воитель gets 75%%: %d", got)
	}
}

func TestRepairBarricade(t *testing.T) {
	g, _ := newTestGame(t)
	chatters(g, "anna")
	g.Chatters["anna"].Gold = 500
	wall := placeAs(g, "anna", "wall", g.Map.Path[3])
	wall.HP = wall.MaxHP / 2
	cost := g.repairCost(wall)
	if full := g.Cfg.TowerCost("wall", 2); cost >= full/2 {
		t.Fatalf("mending is much cheaper than an upgrade: %d vs %d", cost, full)
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!чинить " + wall.Cell.Name()})
	if g.Chatters["anna"].Gold != 500-cost || g.Workers["anna"] == nil {
		t.Fatal("the mercenary goes out to mend it")
	}
	run(g, 30)
	if wall.HP != wall.MaxHP || wall.repairing {
		t.Fatalf("the barricade is whole again: %.0f/%.0f", wall.HP, wall.MaxHP)
	}
}

func TestTrapsSlowEveryone(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	place(g, "trap", g.Map.Path[5])
	e := enemyAt(g, "iceskel", 5, 1000)
	run(g, 1)
	if e.slowMult != g.Cfg.Towers["trap"].Slow || e.HP > 1000-g.Cfg.Towers["trap"].Damage*0.5 {
		t.Fatalf("traps slow even the frost-proof and hit hard: slow %.2f hp %.0f", e.slowMult, e.HP)
	}
}
