package game

import (
	"math"
	"testing"
)

// classGame starts a wave with one player of the given class in the tavern.
func classGame(t *testing.T, class string) (*Game, *Chatter) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	c := g.Chatters["anna"]
	c.Class = class
	g.Phase = PhaseWave
	return g, c
}

func TestPillarGrowsWithShrines(t *testing.T) {
	g, c := classGame(t, "cleric")
	g.waveHP = 100
	e := enemyAt(g, "skeleton", 5, 1e6)
	g.pillarOfLight("anna", c)
	plain, stun := 1e6-e.HP, e.stunLeft
	for range 2 {
		placeAs(g, "anna", "shrine", freeCell(t, g))
	}
	e.HP, e.stunLeft = 1e6, 0
	g.pillarOfLight("anna", c)
	if got := 1e6 - e.HP; got < plain*(1+2*g.Cfg.ClericShrineDmg)-0.01 {
		t.Fatalf("two shrines make the pillar hit harder: %.1f vs %.1f", got, plain)
	}
	if e.stunLeft != stun+2*g.Cfg.ClericShrineStun {
		t.Fatalf("and stun longer: %.1f vs %.1f", e.stunLeft, stun)
	}
}

func TestRangerBallistas(t *testing.T) {
	g, _ := classGame(t, "ranger")
	g.HandleChat(ChatMessage{User: "boris", Text: "!play"})
	g.Chatters["boris"].Class = "mage"
	mine := placeAs(g, "anna", "arrow", freeCell(t, g))
	theirs := placeAs(g, "boris", "arrow", freeCell(t, g))
	spec := g.Cfg.Towers["arrow"]
	if g.towerRange(mine, spec) != spec.Range+g.Cfg.RangerRangeBonus || g.towerRange(theirs, spec) != spec.Range {
		t.Fatal("a ranger's ballista reaches a cell further, others do not")
	}
	if g.critChance(theirs) != 0 || g.critChance(mine) != g.Cfg.CritBase {
		t.Fatal("only a ranger's ballistas crit, 10% at first")
	}
	g.Wave, g.Phase = 2*g.Cfg.BossEvery, PhaseBuild
	if got := g.critChance(mine); got < g.Cfg.CritBase+2*g.Cfg.CritPerBoss-1e-9 {
		t.Fatalf("each boss held adds 5%%: %.2f", got)
	}
	g.Wave = 100 * g.Cfg.BossEvery
	if g.critChance(mine) != g.Cfg.CritMax {
		t.Fatal("up to 70%")
	}
}

func TestPyromancerMansTheGate(t *testing.T) {
	g, c := classGame(t, "pyro")
	far := enemyAt(g, "skeleton", 2, 1e6)
	if g.manGate("anna", c) || len(g.gateFires) != 0 {
		t.Fatal("no enemy near the door: he waits")
	}
	near := g.spawnEnemy("skeleton", 1e6, g.total-1)
	g.Wave = g.Cfg.BossEvery + 1 // one boss held
	if !g.manGate("anna", c) || len(g.gateFires) != 1 || g.gateFires[0].t.Level != 2 {
		t.Fatal("enemies at the door: a brazier on the wall, a level per boss held")
	}
	run(g, 3)
	if near.HP >= 1e6 {
		t.Fatal("it burns the enemies at the door")
	}
	if far.HP < 1e6 && far.Pos.sub(center(g.gateCell())).len() > g.Cfg.Towers["fire"].Range {
		t.Fatal("but only within a brazier's reach")
	}
	run(g, g.Cfg.PyroGateSec)
	if len(g.gateFires) != 0 {
		t.Fatal("and goes out after its time")
	}
}

func TestDruidBearHoldsTheLine(t *testing.T) {
	g, c := classGame(t, "druid")
	g.waveHP = 100
	c.passiveCD = 1e9 // no panther in this test
	e := enemyAt(g, "skeleton", 4, 1e6)
	if !g.bearForm("anna", c) {
		t.Fatal("the bear comes out while enemies walk")
	}
	for i := 0; i < 100 && !g.runOf("anna").hold; i++ {
		run(g, 0.1)
	}
	bear := g.runOf("anna")
	if bear == nil || !bear.hold {
		t.Fatal("it runs into the first enemy and holds the line")
	}
	at := e.Dist
	run(g, 2)
	if e.Dist > at+0.5 || e.HP >= 1e6 {
		t.Fatalf("the enemy is held and mauled: dist %.2f -> %.2f, hp %.0f", at, e.Dist, e.HP)
	}
	run(g, g.Cfg.BearFightSec)
	if g.runOf("anna") != nil {
		t.Fatal("after the fight the druid goes back")
	}
	if g.tavernMates()[0] != "anna" {
		t.Fatal("and sits in the tavern again")
	}
}

func TestBearShovesBackFromTheDoor(t *testing.T) {
	g, c := classGame(t, "druid")
	boss := g.spawnBoss("lich", 1e6)
	boss.ability = 1e9
	boss.Dist, boss.AtGate = g.total, true
	g.bearForm("anna", c)
	if boss.AtGate || boss.Dist > g.total-1.1 {
		t.Fatalf("at the door the bear shoves even a boss back a cell: dist %.2f of %.2f", boss.Dist, g.total)
	}
	if b := g.runOf("anna"); b == nil || !b.hold {
		t.Fatal("and fights there")
	}
}

func TestMageFeast(t *testing.T) {
	g, c := classGame(t, "mage")
	g.HandleChat(ChatMessage{User: "boris", Text: "!play"})
	b := g.Chatters["boris"]
	if !g.feast(c) {
		t.Fatal("the feast is served")
	}
	potion := g.Cfg.Drinks["potion"]
	if b.buff != potion.Boost || b.buffLeft != potion.Sec || c.buff != potion.Boost || b.Drunk != 0 {
		t.Fatalf("everyone gets the black potion's boost without the drink: buff %.2f left %.0f drunk %.1f", b.buff, b.buffLeft, b.Drunk)
	}
}

func TestShamanWolves(t *testing.T) {
	g, c := classGame(t, "shaman")
	g.waveHP = 100
	g.Cfg.EnemyRegen, g.Cfg.GateDPS = 0, 0 // count the bites alone
	var es []*Enemy
	for i := 2; i < 8; i += 2 {
		es = append(es, enemyAt(g, "skeleton", i, 1e6))
	}
	if !g.spiritWolves("anna", c) {
		t.Fatal("the wolves come out")
	}
	run(g, 10)
	want := 2 * g.skillHP() * g.Cfg.WolfDmg
	for _, e := range es {
		if got := 1e6 - e.HP; got < want*0.99 || got > want*1.01 {
			t.Fatalf("each wolf bites every enemy once: %.1f, want %.1f", got, want)
		}
	}
	for _, r := range g.Runs {
		if r.Kind == "wolf" {
			t.Fatal("and the wolves are gone at the end of the path")
		}
	}
}

func TestTotemGrowsWithBosses(t *testing.T) {
	g, _ := newTestGame(t)
	if g.stormGrowth() != 1 {
		t.Fatal("no boss held: lightning as is")
	}
	g.Wave = 6 * g.Cfg.BossEvery
	if want := 1 + 6*g.Cfg.TotemPerBoss; math.Abs(g.stormGrowth()-want) > 1e-9 {
		t.Fatalf("six bosses held: x%.2f, want x%.2f", g.stormGrowth(), want)
	}
	g.Wave = 100
	if g.stormGrowth() != 1+g.Cfg.TotemBossMax {
		t.Fatal("and no more than the cap")
	}
}

func TestRogueVeil(t *testing.T) {
	g, c := classGame(t, "rogue")
	g.Wave = 1 // waves count from 1
	cell := freeCell(t, g)
	placeAs(g, "anna", "arrow", cell).Built = false
	g.order("anna", c.Name, cell)
	w := g.Workers["anna"]
	e := enemyAt(g, "skeleton", 3, 1e6)
	w.Pos = e.Pos
	g.tickWorkers(0.05)
	if !w.Veiled || w.State == WorkerFlee || c.veilWave != g.Wave {
		t.Fatalf("a rogue out in a wave walks veiled: enemies do not stun him (%s)", w.State)
	}
	delete(g.Workers, "anna")
	g.order("anna", c.Name, cell)
	w = g.Workers["anna"]
	w.Pos = e.Pos
	g.tickWorkers(0.05)
	if w.Veiled || w.State != WorkerFlee {
		t.Fatal("once a wave only")
	}
	g.Wave++
	delete(g.Workers, "anna")
	g.order("anna", c.Name, cell)
	g.tickWorkers(0.05)
	if !g.Workers["anna"].Veiled {
		t.Fatal("and again the next wave")
	}

	g2, c2 := classGame(t, "rogue")
	enemyAt(g2, "skeleton", 3, 1e6)
	g2.shadowStep("anna", c2)
	if c2.veilWave != 0 {
		t.Fatal("a shadow-step does not spend the veil")
	}

	g3, _ := classGame(t, "mage")
	g3.Wave = 1
	g3.order("anna", "anna", freeCell(t, g3))
	g3.tickWorkers(0.05)
	if g3.Workers["anna"].Veiled {
		t.Fatal("only rogues veil")
	}
}
