package game

import (
	"math"
	"strings"
	"testing"
)

// artGame gives anna a class and an artifact building of its signature kind.
func artGame(t *testing.T, class, kind string, p Pos) (*Game, *Tower) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	g.Chatters["anna"].Class = class
	g.Phase = PhaseWave
	tw := placeAs(g, "anna", kind, p)
	tw.Level = g.Cfg.Towers[kind].MaxLevel + 1
	g.readyTower(tw)
	return g, tw
}

func TestOnlyClassBuildingsReachArtifact(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	c := g.Chatters["anna"]
	c.Class, c.Gold = "ranger", 1000
	mine := placeAs(g, "anna", "arrow", freeCell(t, g))
	other := placeAs(g, "anna", "magic", freeCell(t, g))
	mine.Level, other.Level = 5, 5
	g.HandleChat(ChatMessage{User: "anna", Text: "!up " + other.Cell.Name()})
	if r := lastReply(g); other.PendingCost > 0 || !strings.Contains(r, "6-й") {
		t.Fatalf("a building of another class stops at 5: %q", r)
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!up " + mine.Cell.Name()})
	if mine.PendingCost != g.Cfg.TowerCost("arrow", 6) {
		t.Fatal("the class building goes on to 6 for 6x its price")
	}
	for i := 0; i < 300 && mine.PendingCost > 0; i++ {
		run(g, 0.1)
	}
	if mine.Level != 6 || !g.artifact(mine) || g.levelMult(mine) <= g.Cfg.Towers["arrow"].UpgMult*4 {
		t.Fatal("and grows stronger at the artifact level")
	}
}

func TestArtifactTrapPoisons(t *testing.T) {
	g, _ := newTestGame(t)
	g, tw := artGame(t, "rogue", "trap", g.Map.Path[5])
	charges := tw.Charges
	e := enemyAt(g, "skeleton", 5, 1000)
	run(g, 1)
	if e.poisonN != 1 || tw.Charges != charges {
		t.Fatalf("an artifact trap poisons and never spends a charge: poison %d charges %d", e.poisonN, tw.Charges)
	}
	for i := 0; i < 5; i++ {
		g.poison(e, "anna")
	}
	if e.poisonN != poisonMax {
		t.Fatal("poison stacks up to 3")
	}
	hp := e.HP
	e.HP = hp / 4
	before := e.HP
	g.tickAfflictions(e, g.Cfg.Enemies["skeleton"], 1)
	lowHit := before - e.HP
	e.HP = hp
	before = e.HP
	g.tickAfflictions(e, g.Cfg.Enemies["skeleton"], 1)
	if lowHit >= before-e.HP {
		t.Fatal("poison eats a share of the HP left: less from a weaker enemy")
	}
}

func TestArtifactArrows(t *testing.T) {
	g, _ := newTestGame(t)
	g, tw := artGame(t, "ranger", "arrow", freeCell(t, g))
	g.HandleChat(ChatMessage{User: "anna", Text: "!стрелы " + tw.Cell.Name() + " огонь"})
	if tw.Ammo != ammoFire {
		t.Fatalf("the ammo switches: %q", lastReply(g))
	}
	p := &Projectile{Damage: 100}
	g.arm(tw, p)
	if p.BurnDPS <= 0 {
		t.Fatal("fire arrows burn")
	}
	tw.Ammo = ammoBlast
	p = &Projectile{Damage: 100}
	g.arm(tw, p)
	if p.Splash <= 0 || p.SplashPct <= 0 {
		t.Fatal("blast arrows blow up around the target")
	}
	tw.Ammo = ammoPoison
	p = &Projectile{Damage: 100}
	g.arm(tw, p)
	if !p.Poison {
		t.Fatal("poison arrows poison")
	}
	plain := placeAs(g, "anna", "magic", freeCell(t, g))
	g.HandleChat(ChatMessage{User: "anna", Text: "!стрелы " + plain.Cell.Name() + " яд"})
	if plain.Ammo != "" {
		t.Fatal("only an artifact ballista takes ammo")
	}
}

func TestArtifactBrazierStacksBurns(t *testing.T) {
	g, _ := newTestGame(t)
	g, tw := artGame(t, "pyro", "fire", freeCell(t, g))
	e := enemyAt(g, "skeleton", 5, 1000)
	p := &Projectile{BurnDPS: 10, BurnSec: 3, Owner: "anna"}
	g.arm(tw, p)
	for i := 0; i < 5; i++ {
		g.burn(e, p)
	}
	if e.sburnN != burnStackMax || !e.burning() {
		t.Fatalf("an artifact brazier's burn stacks up to 3: %d", e.sburnN)
	}
}

func TestArtifactDrumsAndWall(t *testing.T) {
	g, _ := newTestGame(t)
	g, drums := artGame(t, "bard", "drums", freeCell(t, g))
	near := placeAs(g, "anna", "arrow", freeCell(t, g))
	_, rate := g.auras(near)
	if rate < 1+g.Cfg.Towers["drums"].AuraRate*auraMult(drums)+g.Cfg.DrumArtRate-1e-9 {
		t.Fatalf("artifact drums add fire rate on top: %.2f", rate)
	}
	drums.artCD = 0
	g.tickArtifact(drums, g.Cfg.Towers["drums"], 0.1)
	if near.warLeft <= 0 || g.drinkMult(near) < 1+g.Cfg.DrumWarBoost-1e-9 {
		t.Fatal("the war beat makes the buildings in reach hit harder")
	}

	g2, wall := artGame(t, "warrior", "wall", g.Map.Path[5])
	wall.HP, wall.MaxHP = 1e9, 1e9
	wall.artCD = 1e9 // spikes first, the shove below
	g2.Cfg.EnemyRegen = 0
	e := enemyAt(g2, "skeleton", 5, 1e6)
	run(g2, 1)
	if e.HP >= 1e6 {
		t.Fatal("the spikes hurt whoever hacks at it")
	}
	wall.artCD = 0
	at := e.Dist
	g2.tickArtifact(wall, g2.Cfg.Towers["wall"], 0.1)
	if e.Dist > at-0.99 || e.blocked != nil {
		t.Fatalf("and it shoves its attackers a cell back: %.2f -> %.2f", at, e.Dist)
	}
}

func TestArtifactTotemObeliskShrineAltar(t *testing.T) {
	g, _ := newTestGame(t)
	g, totem := artGame(t, "shaman", "totem", freeCell(t, g))
	totem.artCD = 0
	g.tickArtifact(totem, g.Cfg.Towers["totem"], 0.1)
	if len(g.storms) != 1 {
		t.Fatal("an artifact totem lays a charge of lightning on the path")
	}
	e := g.spawnEnemy("skeleton", 1000, 0)
	e.Pos = g.storms[0].Pos
	g.tickStorms(0.1)
	if e.slowMult >= 1 {
		t.Fatal("which slows whoever walks through it")
	}

	g2, obelisk := artGame(t, "mage", "magic", freeCell(t, g))
	hits := 0
	for i := 0; i < 1000; i++ {
		p := &Projectile{Damage: 100}
		g2.arm(obelisk, p)
		if p.SlowHit && p.Damage == 100*g2.Cfg.MageArtMult {
			hits++
		}
	}
	if hits < 120 || hits > 280 {
		t.Fatalf("about one shot in five is empowered: %d of 1000", hits)
	}

	g3, _ := artGame(t, "cleric", "shrine", freeCell(t, g))
	g3.Phase = PhaseBuild
	g3.tickWallShield()
	if g3.wallShield != g3.Cfg.WallsHP*g3.Cfg.ShrineShieldPct {
		t.Fatal("an artifact shrine shields 10% of the walls, refilled in the build phase")
	}
	g3.Phase = PhaseWave
	walls := g3.Walls
	g3.hurtWalls(50)
	if g3.Walls != walls || g3.wallShield != g3.Cfg.WallsHP*g3.Cfg.ShrineShieldPct-50 {
		t.Fatal("the shield takes the blows first")
	}
	g3.wallShield = 0
	g3.tickWallShield()
	if g3.wallShield != 0 {
		t.Fatal("and does not refill during a wave")
	}

	g4, a := artGame(t, "warlock", "curse", g.Map.Path[2])
	a.Cell = Pos{g4.Map.Path[2].X, 0}
	b := placeAs(g4, "anna", "curse", Pos{g4.Map.Path[2].X, 4})
	b.Level = a.Level
	if len(g4.soulLinks()) != 1 {
		t.Fatal("two artifact altars of one owner are linked")
	}
	f := g4.spawnEnemy("skeleton", 1000, 0)
	f.Pos = vec{float64(g4.Map.Path[2].X) + 0.5, 2.5}
	g4.tickSoulLinks()
	hp := f.HP
	g4.tickSoulLinks()
	if f.HP >= 1000 || f.HP != hp || f.slowMult >= 1 {
		t.Fatal("crossing the link hurts and slows, once")
	}
}

func TestArtifactAndMonogamistAchievements(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	g.HandleChat(ChatMessage{User: "boris", Text: "!play"})
	g.HandleChat(ChatMessage{User: "vera", Text: "!play"})
	a, b := g.Chatters["anna"], g.Chatters["boris"]
	a.Class, b.Class = "ranger", "cleric"
	g.Cfg.MaxOrders = 20
	a.Gold, b.Gold = 20000, 20000
	for i := 0; i < g.TowerLimit(); i++ {
		g.HandleChat(ChatMessage{User: "anna", Text: "!b " + freeCell(t, g).Name() + " arrow"})
		if i < g.Cfg.ShrineLimitCleric {
			g.HandleChat(ChatMessage{User: "boris", Text: "!b " + freeCell(t, g).Name() + " shrine"})
		}
	}
	run(g, 120)
	if !a.Ach["monogamist"] {
		t.Fatalf("every tower a ballista: «Однолюб» (%d of %d built)", g.kindOf("anna", "arrow"), g.TowerLimit())
	}
	if !b.Ach["monogamist"] {
		t.Fatal("a cleric gets it with all the shrines he may own")
	}
	var tw *Tower
	for _, x := range g.Towers {
		if x.Owner == "anna" {
			tw = x
			break
		}
	}
	tw.Level = 5
	g.HandleChat(ChatMessage{User: "anna", Text: "!up " + tw.Cell.Name()})
	run(g, 60)
	if !a.Ach["artifact"] {
		t.Fatal("an artifact building earns «Хранитель реликвии»")
	}
}

func TestArtifactTrapSnapsMilder(t *testing.T) {
	g, _ := newTestGame(t)
	g, tw := artGame(t, "rogue", "trap", g.Map.Path[5])
	g.Cfg.EnemyRegen = 0
	full := g.Cfg.Towers["trap"].Damage * g.levelMult(tw) * g.classBonus("anna", "trap")
	e := enemyAt(g, "skeleton", 5, 1e6)
	g.springTrap(e, tw, g.Cfg.Enemies["skeleton"])
	if hit := 1e6 - e.HP; math.Abs(hit-full*g.Cfg.TrapArtSnap) > 1e-6 {
		t.Fatalf("an artifact trap snaps for %.0f%% of its damage: %.1f of %.1f", g.Cfg.TrapArtSnap*100, hit, full)
	}
	if math.Abs(e.bleedDPS-full*g.Cfg.TrapBleed) > 1e-6 {
		t.Fatal("and bleeds in full")
	}
}

func TestSoulLinkRange(t *testing.T) {
	g, _ := newTestGame(t)
	g, a := artGame(t, "warlock", "curse", Pos{0, 0})
	far := placeAs(g, "anna", "curse", Pos{5, 0})
	far.Level = a.Level
	if len(g.soulLinks()) != 0 {
		t.Fatal("altars five cells apart are not linked")
	}
	near := placeAs(g, "anna", "curse", Pos{0, 4})
	near.Level = a.Level
	if len(g.soulLinks()) != 1 {
		t.Fatal("four cells apart they are")
	}
}
