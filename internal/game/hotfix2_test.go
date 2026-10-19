package game

import (
	"math"
	"testing"
)

func TestEnemiesRegenerateAndTrapsCutHealing(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	e := enemyAt(g, "skeleton", 2, 1000)
	e.HP = 500
	g.tickEnemies(1)
	healed := e.HP - 500
	if healed < 1 {
		t.Fatalf("enemies slowly heal: +%.2f", healed)
	}
	e.HP, e.healCut = 500, 3
	g.tickEnemies(1)
	if cut := e.HP - 500; math.Abs(cut-healed*(1-g.Cfg.TrapHealCut)) > 0.01 {
		t.Fatalf("a fresh cut halves healing: +%.2f vs +%.2f", cut, healed)
	}
}

func TestTrapsBleedAndShareKills(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	chatters(g, "anna", "boris")
	trap := placeAs(g, "anna", "trap", g.Map.Path[5])
	if trap.Charges != 9 {
		t.Fatalf("traps have half as many charges again: %d", trap.Charges)
	}
	trap.Level = 3
	g.readyTower(trap)
	if trap.Charges != 15 {
		t.Fatalf("and a third more per level: %d", trap.Charges)
	}
	e := enemyAt(g, "skeleton", 5, 2000)
	g.spawnEnemy("troll", 1e9, 0)
	run(g, 0.6)
	if e.bleedLeft <= 0 || e.healCut <= 0 || !e.helpers["anna"] {
		t.Fatalf("the snap leaves a bleeding cut: bleed %.1f cut %.1f", e.bleedLeft, e.healCut)
	}
	hp := e.HP
	run(g, 1)
	if e.HP >= hp {
		t.Fatal("bleeding hurts over time")
	}
	g.damage(e, 1e9, "boris", true)
	share := int(math.Round(float64(g.Cfg.Enemies["skeleton"].Bounty) * g.Cfg.TrapAssistPct))
	if g.Chatters["anna"].Gold != share {
		t.Fatalf("the trap owner gets an assist of %d, got %d", share, g.Chatters["anna"].Gold)
	}
}

func TestMagicCallsLightning(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	a, _, i := pathNeighbours(t, g)
	m := place(g, "magic", a)
	m.cooldown = 1e9 // only the bolt
	es := []*Enemy{enemyAt(g, "skeleton", i, 1e6), enemyAt(g, "skeleton", i, 1e6)}
	g.tickTowers(0.05)
	for _, e := range es {
		if e.HP >= 1e6 {
			t.Fatal("the bolt hits everyone around its mark")
		}
	}
	every := g.Cfg.Towers["magic"].StrikeEvery
	if m.strikeCD != every {
		t.Fatalf("then recharges: %.1f", m.strikeCD)
	}
	m.Level, m.strikeCD = 5, 0
	g.tickTowers(0.05)
	if m.strikeCD >= every {
		t.Fatal("a better obelisk strikes more often")
	}
}

func TestRewardsGrowAfterBossWaves(t *testing.T) {
	g, _ := newTestGame(t)
	chatters(g, "anna")
	g.Wave = g.Cfg.BossEvery
	g.endWave()
	if math.Abs(g.rewardMult-(1+g.Cfg.RewardGrowth)) > 1e-9 {
		t.Fatalf("a boss wave held raises rewards: %.2f", g.rewardMult)
	}
	g.Phase = PhaseWave
	e := enemyAt(g, "troll", 3, 10)
	g.damage(e, 100, "anna", true)
	want := int(math.Round(float64(g.Cfg.Enemies["troll"].Bounty) * g.rewardMult))
	if g.Chatters["anna"].Gold != want {
		t.Fatalf("kills pay more: %d, want %d", g.Chatters["anna"].Gold, want)
	}
	g.NewGame("")
	if g.rewardMult != 1 {
		t.Fatal("a new game starts from plain rewards")
	}
}

func TestDrinksChangeDamage(t *testing.T) {
	g, _ := newTestGame(t)
	chatters(g, "anna")
	g.Chatters["anna"].Gold = 500
	a, b, _ := pathNeighbours(t, g)
	t1 := placeAs(g, "anna", "arrow", a)
	t2 := placeAs(g, "anna", "fire", b)
	g.HandleChat(ChatMessage{User: "anna", Text: "!drink эль"})
	if g.drinkMult(t1) != 1.5 {
		t.Fatalf("ale: +50%%: %.2f", g.drinkMult(t1))
	}
	g.tickDrinks(20.5)
	if g.drinkMult(t1) != 0.5 {
		t.Fatalf("then the hangover: %.2f", g.drinkMult(t1))
	}
	g.tickDrinks(20.5)
	if g.drinkMult(t1) != 1 {
		t.Fatal("and back to normal")
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!drink зелье"})
	g.tickDrinks(20.5)
	if g.drinkMult(t1) != 1 {
		t.Fatal("the potion leaves no hangover")
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!drink медовуха"})
	if m1, m2 := g.drinkMult(t1), g.drinkMult(t2); !(m1 == 2 && m2 == 0.5 || m1 == 0.5 && m2 == 2) {
		t.Fatalf("mead: one building twice as strong, another half: %.1f %.1f", m1, m2)
	}
}

func TestRoundPoursRandomDrinks(t *testing.T) {
	g, _ := newTestGame(t)
	chatters(g, "anna", "boris", "vera")
	g.Reward("someone", "Кто-то", "Выпивка за мой счёт")
	kinds := map[string]bool{}
	for _, e := range g.TakeEvents() {
		if e.Kind == "drink" {
			kinds[e.Text] = true
		}
	}
	if len(kinds) == 0 || g.Chatters["anna"].Drinks != 1 {
		t.Fatal("everyone in the tavern gets a drink")
	}
}

func TestRoguesPickPockets(t *testing.T) {
	g, _ := newTestGame(t)
	chatters(g, "anna", "boris")
	g.Chatters["anna"].Class = "rogue"
	g.Chatters["boris"].Class = "mage"
	g.Chatters["boris"].Gold = 200
	run(g, g.Cfg.RogueEverySec/2)
	before := g.Chatters["anna"].Gold
	g.tickRogues(g.Cfg.RogueEverySec)
	took := g.Chatters["anna"].Gold - before
	if took < 1 || took > g.Cfg.RogueStealMax || g.Chatters["boris"].Gold+took < 200 {
		t.Fatalf("a rogue lifts a little gold from another: %d", took)
	}
}

func TestDruidTurnsIntoAPanther(t *testing.T) {
	g, _ := newTestGame(t)
	chatters(g, "dina")
	if _, ok := classByKey("druid"); !ok {
		t.Fatal("the druid class exists")
	}
	g.Chatters["dina"].Class = "druid"
	if g.classBonus("dina", "arrow") != 1 {
		t.Fatal("druids have no building bonus")
	}
	g.Phase = PhaseWave
	e1 := enemyAt(g, "skeleton", len(g.Map.Path)-2, 1e6)
	e2 := enemyAt(g, "skeleton", len(g.Map.Path)-3, 1e6)
	g.tickDruids(0.05)
	if len(g.Panthers) != 1 {
		t.Fatal("in a wave the druid turns into a panther")
	}
	for i := 0; i < 200; i++ {
		g.tickDruids(0.05)
	}
	bite := g.waveHP * g.Cfg.DruidMaul * (1 - g.Cfg.Enemies["skeleton"].Armor)
	for _, e := range []*Enemy{e1, e2} {
		if lost := 1e6 - e.HP; math.Abs(lost-bite) > 1 {
			t.Fatalf("each enemy is mauled once: lost %.1f, bite %.1f", lost, bite)
		}
	}
	for i := 0; i < 400; i++ {
		g.tickDruids(0.05)
	}
	if len(g.Panthers) != 0 {
		t.Fatal("the hunt ends and the druid goes home")
	}
}
