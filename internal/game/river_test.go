package game

import "testing"

// riverGame starts a test game on the river camp, mid-wave.
func riverGame(t *testing.T) *Game {
	t.Helper()
	g, _ := newTestGame(t)
	g.NewGame(BiomeRiver)
	g.Phase = PhaseWave
	g.Cfg.EnemyRegen = 0
	return g
}

func TestRiverNetsAndBooms(t *testing.T) {
	g := riverGame(t)
	if !g.wet(g.Map.Path[5]) || g.wet(g.Map.Path[len(g.Map.Path)-1]) {
		t.Fatal("the river runs to the pier, and the pier is dry")
	}
	net := place(g, "trap", g.Map.Path[5])
	e := enemyAt(g, "drowned", 5, 1000)
	run(g, 1)
	ts := g.Cfg.Towers["trap"]
	if e.bleedLeft > 0 || e.slowMult >= ts.Slow || 1000-e.HP > ts.Damage*netDmgMult*1.01 {
		t.Fatalf("an anchor net cuts less, slows harder and nothing bleeds: hp %.0f slow %.2f bleed %.1f", e.HP, e.slowMult, e.bleedLeft)
	}
	if net.Charges != ts.Charges-1 || g.towerName("trap") != "Якорная сеть" {
		t.Fatal("it is still a trap with charges, under its river name")
	}
	boom := place(g, "wall", g.Map.Path[8])
	if boom.MaxHP != g.Cfg.Towers["wall"].HP*boomHPMult {
		t.Fatalf("a boom holds less: %.0f", boom.MaxHP)
	}
	pier := place(g, "wall", g.Map.Path[len(g.Map.Path)-1])
	if pier.MaxHP != g.Cfg.Towers["wall"].HP {
		t.Fatal("a barricade on the pier is a barricade")
	}
}

func TestRiverFireAndBoats(t *testing.T) {
	g := riverGame(t)
	swimmer := enemyAt(g, "drowned", 5, 1000)
	g.ignite(swimmer, 10, 3, "")
	if swimmer.burnLeft > 0 || !g.swimming(swimmer) {
		t.Fatal("fire goes out on a swimmer")
	}
	pirate := enemyAt(g, "pirate", 5, 1000)
	if pirate.boatHP != 600 || g.swimming(pirate) {
		t.Fatalf("a pirate sails a boat of 60%% of his health: %.0f", pirate.boatHP)
	}
	g.damage(pirate, 100, "", false)
	if pirate.HP != 1000 || pirate.boatHP != 600-100*(1-boatArmor) {
		t.Fatalf("the boat takes the blow, armoured: pirate %.0f boat %.0f", pirate.HP, pirate.boatHP)
	}
	g.ignite(pirate, 100, 1, "")
	before := pirate.boatHP
	run(g, 1)
	if burnt := before - pirate.boatHP; burnt < 100*boatBurn*0.95 {
		t.Fatalf("boats burn half as hard again: %.0f", burnt)
	}
	g.damage(pirate, 1e6, "", true)
	if pirate.boatHP != 0 || pirate.HP != 1000 || !g.swimming(pirate) {
		t.Fatalf("the boat sinks and the pirate swims on: hp %.0f", pirate.HP)
	}
	g.damage(pirate, 100, "", false)
	if pirate.HP != 900 {
		t.Fatalf("in the water he has no armour: %.0f", pirate.HP)
	}
}

func TestDeepOnesDiveUnderBooms(t *testing.T) {
	g := riverGame(t)
	place(g, "wall", g.Map.Path[6])
	e := enemyAt(g, "deep_one", 5, 1000)
	e.diveCD = 0.01
	run(g, 0.1)
	if !e.submerged() {
		t.Fatal("the deep one dives")
	}
	g.damage(e, 500, "", true)
	if e.HP != 1000 {
		t.Fatal("nothing reaches it under water")
	}
	run(g, diveSec-0.2)
	if e.Dist <= g.cellDist[6] || e.blocked != nil {
		t.Fatalf("it swims under the boom: dist %.2f, boom at %.2f", e.Dist, g.cellDist[6])
	}
	run(g, 0.5)
	if e.submerged() {
		t.Fatal("and comes up again")
	}
}

func TestDrownedRise(t *testing.T) {
	g := riverGame(t)
	g.queue = []spawn{{kind: "drowned", hp: 100}}
	g.spawnTimer = 0
	g.tickSpawns(0.05)
	e := g.Enemies[len(g.Enemies)-1]
	if e.Dist < g.total*riseFrom || e.Dist > g.total*riseTo {
		t.Fatalf("the drowned rises halfway down: %.1f of %.1f", e.Dist, g.total)
	}
}

func TestOctopusClings(t *testing.T) {
	g := riverGame(t)
	boss := enemyAt(g, "octopus", 6, 10000)
	near := place(g, "arrow", freeCell(t, g))
	boss.Dist = 0
	for i := range g.cellDist {
		if center(g.Map.Path[i]).sub(center(near.Cell)).len() < 1.5 {
			boss.Dist = g.cellDist[i]
			break
		}
	}
	boss.Pos = g.routePos(boss.Dist)
	g.cling(boss)
	var o *Enemy
	for _, e := range g.Enemies {
		if e.cling == near {
			o = e
		}
	}
	if o == nil || g.active(near) {
		t.Fatal("an octopling sits on the building and silences it")
	}
	g.damage(o, o.HP+1, StreamerName, true)
	if !g.active(near) || o.cling != nil {
		t.Fatal("shot off, it frees the building")
	}
	g.cling(boss)
	run(g, clingSec+0.5)
	for _, e := range g.Enemies {
		if e.Kind == "octopling" {
			t.Fatal("an octopling lets go by itself, so the wave can end")
		}
	}
	if !g.active(near) {
		t.Fatal("and the building works again")
	}
}

func TestLeviathanTide(t *testing.T) {
	g := riverGame(t)
	bank := place(g, "arrow", freeCell(t, g))
	boss := g.spawnEnemy("leviathan", 10000, 0)
	for i := range g.cellDist {
		if center(g.Map.Path[i]).sub(center(bank.Cell)).len() < 1.5 {
			boss.Dist = g.cellDist[i]
		}
	}
	boss.Pos = g.routePos(boss.Dist)
	g.tide(boss)
	if g.active(bank) || bank.floodLeft <= 0 || !boss.submerged() {
		t.Fatal("the tide drowns the bank and the leviathan swims deep")
	}
	g.damage(boss, 1000, "", true)
	if boss.HP != 10000 {
		t.Fatal("nothing reaches it in the tide")
	}
	run(g, tideSec+0.2)
	if !g.active(bank) || boss.submerged() {
		t.Fatal("the tide falls")
	}
}
