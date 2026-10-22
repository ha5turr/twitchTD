package game

import (
	"math"
	"testing"
)

// skillReady puts a chatter of the class in the tavern, in a wave, with
// its skill due on the next tick.
func skillReady(t *testing.T, g *Game, name, class string) *Chatter {
	t.Helper()
	chatters(g, name)
	c := g.Chatters[name]
	c.Class = class
	c.skillCD = 0.01
	g.Phase = PhaseWave
	g.waveHP = 100
	return c
}

func TestWarriorChargeShovesEnemiesBack(t *testing.T) {
	g, _ := newTestGame(t)
	skillReady(t, g, "vova", "warrior")
	wall := place(g, "wall", g.Map.Path[4])
	a := enemyAt(g, "skeleton", 8, 1e9)
	b := enemyAt(g, "skeleton", 3, 1e9)
	da, db := a.Dist, b.Dist
	g.tickHeroes(0.05)
	if g.runOf("vova") == nil {
		t.Fatal("the Воитель charges out")
	}
	for i := 0; i < 400 && g.runOf("vova") != nil; i++ {
		g.tickHeroes(0.05)
	}
	if math.Abs(a.Dist-(da-g.Cfg.WarriorPush)) > 0.01 || math.Abs(b.Dist-(db-g.Cfg.WarriorPush)) > 0.01 {
		t.Fatalf("each enemy is shoved two cells back once: %.2f→%.2f, %.2f→%.2f", da, a.Dist, db, b.Dist)
	}
	if g.Towers[wall.Cell] != wall {
		t.Fatal("he runs past barricades without touching them")
	}
}

func TestMageMeteorShower(t *testing.T) {
	g, _ := newTestGame(t)
	skillReady(t, g, "mira", "mage")
	e := enemyAt(g, "skeleton", 5, 1e6)
	g.tickHeroes(0.05)
	for i := 0; i < 100; i++ {
		g.tickHeroes(0.05)
	}
	if e.HP >= 1e6 || e.burnLeft <= 0 && e.burnDPS == 0 {
		t.Fatalf("meteors hit and set enemies alight: hp %.0f", e.HP)
	}
	e.burnLeft, e.healCut, e.HP = 2, 0, 500
	g.heal(e, 10)
	if e.HP != 500+10*(1-g.Cfg.TrapHealCut) {
		t.Fatal("a burning enemy heals half as much")
	}
}

func TestBardMusicSpreadsDrumsAndShootsNotes(t *testing.T) {
	g, _ := newTestGame(t)
	c := skillReady(t, g, "bard", "bard")
	a, _, i := pathNeighbours(t, g)
	drums := placeAs(g, "bard", "drums", a)
	far := place(g, "arrow", Pos{0, 0})
	if g.auras(far); g.auraRange(drums, g.Cfg.Towers["drums"]) != 3 {
		t.Fatal("drums reach three cells between songs")
	}
	e := enemyAt(g, "skeleton", i, 1e6)
	g.tickTowers(0.05)
	if len(g.Projectiles) != 0 && g.Projectiles[0].Kind == "note" {
		t.Fatal("no notes without music")
	}
	g.tickHeroes(0.05)
	if c.musicLeft <= 0 {
		t.Fatal("the bard starts to play")
	}
	if _, rate := g.auras(far); rate <= 1 {
		t.Fatal("while he plays his drums speed up the whole map")
	}
	g.tickTowers(0.05)
	notes := 0
	for _, p := range g.Projectiles {
		if p.Kind == "note" && p.Target == e.ID {
			notes++
		}
	}
	if notes != 1 {
		t.Fatalf("his drums shoot a note at one enemy: %d", notes)
	}
}

func TestRogueShadowStep(t *testing.T) {
	g, _ := newTestGame(t)
	skillReady(t, g, "rita", "rogue")
	boss := g.spawnBoss("lich", 1e6)
	boss.Dist = g.cellDist[3]
	boss.Pos = g.routePos(boss.Dist)
	far := enemyAt(g, "skeleton", 9, 1e6)
	g.tickHeroes(0.05)
	if boss.HP >= 1e6 || boss.bleedLeft <= 0 || boss.slowLeft <= 0 || far.HP < 1e6 {
		t.Fatal("the rogue goes for the boss first: a bleeding, slowing stab")
	}
	if g.runOf("rita") == nil {
		t.Fatal("and stands there in shadow for a moment")
	}
	run(g, 1)
	if g.runOf("rita") != nil {
		t.Fatal("then steps back to the tavern")
	}

	g2, _ := newTestGame(t)
	skillReady(t, g2, "rita", "rogue")
	crowd := []*Enemy{enemyAt(g2, "skeleton", 6, 1e6), enemyAt(g2, "skeleton", 6, 1e6), enemyAt(g2, "skeleton", 6, 1e6)}
	g2.tickHeroes(0.05)
	for _, e := range crowd {
		if e.HP >= 1e6 {
			t.Fatal("with no boss near and a crowd around, a fan of blades hits them all")
		}
	}
}

func TestHotfix4Numbers(t *testing.T) {
	c := DefaultConfig()
	if c.Towers["arrow"].NetEvery != 20 || c.Towers["trap"].Damage != 30 {
		t.Fatal("nets come less often and traps hit a little softer")
	}
}
