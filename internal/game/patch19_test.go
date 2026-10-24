package game

import (
	"math"
	"testing"
)

func TestMageObeliskSlows(t *testing.T) {
	slowed := func(class string, level int) int {
		g, _ := newTestGame(t)
		g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
		g.Chatters["anna"].Class = class
		g.Phase = PhaseWave
		a, _, i := pathNeighbours(t, g)
		tw := placeAs(g, "anna", "magic", a)
		tw.Level = level
		enemyAt(g, "skeleton", i, 1e9)
		n := 0
		for k := 0; k < 2000; k++ {
			tw.cooldown = 0
			g.Projectiles = nil
			g.shoot(tw, g.Cfg.Towers["magic"], 0.01)
			if len(g.Projectiles) == 1 && g.Projectiles[0].SlowHit {
				n++
			}
		}
		return n
	}
	if n := slowed("ranger", 1); n != 0 {
		t.Fatalf("only a mage's obelisk slows: %d", n)
	}
	if n := slowed("mage", 1); n < 140 || n > 260 {
		t.Fatalf("10%% at level 1: %d of 2000", n)
	}
	if n := slowed("mage", 5); n < 500 || n > 700 {
		t.Fatalf("30%% at level 5: %d of 2000", n)
	}
}

func TestArtifactDrumsHasteSkills(t *testing.T) {
	g, _ := newTestGame(t)
	if g.skillHaste() != 1 {
		t.Fatal("no artifact drums: skills as they are")
	}
	g, d := artGame(t, "bard", "drums", freeCell(t, g))
	if want := 1 / (1 - g.Cfg.DrumArtHaste); math.Abs(g.skillHaste()-want) > 1e-9 {
		t.Fatalf("one artifact drum cuts cooldowns by 5%%: x%.3f", g.skillHaste())
	}
	for i := 0; i < 20; i++ {
		placeAs(g, "anna", "drums", freeCell(t, g)).Level = d.Level
	}
	if want := 1 / (1 - g.Cfg.DrumArtHasteMax); math.Abs(g.skillHaste()-want) > 1e-9 {
		t.Fatal("up to the cap")
	}
}

func TestSkillsWaitForEnemies(t *testing.T) {
	g, c := classGame(t, "mage")
	g.queue = []spawn{{"skeleton", 100}, {"skeleton", 100}, {"skeleton", 100}}
	g.Enemies = nil
	enemyAt(g, "skeleton", 1, 1e6)
	c.skillCD = 0.01
	g.tickHeroes(0.1)
	if c.skillCD > 0 || len(g.meteors) > 0 {
		t.Fatal("a ready skill waits while the first enemy trickles in")
	}
	enemyAt(g, "skeleton", 2, 1e6)
	enemyAt(g, "skeleton", 3, 1e6)
	g.tickHeroes(0.1)
	if c.skillCD <= 0 {
		t.Fatal("and goes off once a few are on the field")
	}

	g2, c2 := classGame(t, "mage")
	g2.queue, g2.Enemies = nil, nil
	enemyAt(g2, "skeleton", 1, 1e6)
	c2.skillCD = 0.01
	g2.tickHeroes(0.1)
	if c2.skillCD <= 0 {
		t.Fatal("or on the wave's last enemy")
	}
}

func TestDiceShortForms(t *testing.T) {
	g, _ := newTestGame(t)
	for _, n := range []string{"anna", "boris"} {
		g.HandleChat(ChatMessage{User: n, Text: "!play"})
		g.Chatters[n].Gold = 500
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!к 100"})
	if len(g.Dice) != 1 {
		t.Fatal("!к 100 sits down with a stake")
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!к о"})
	if len(g.Dice) != 0 {
		t.Fatal("!к о stands up")
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!к 100"})
	g.HandleChat(ChatMessage{User: "boris", Text: "!к п"})
	if d := g.Dice[0]; d.Guest != "boris" {
		t.Fatal("!к п accepts")
	}
	g.Dice = nil
	g.HandleChat(ChatMessage{User: "anna", Text: "!к 100"})
	g.HandleChat(ChatMessage{User: "boris", Text: "!к пб 150"})
	if d := g.Dice[0]; d.Guest != "boris" || d.GuestStake != 150 {
		t.Fatal("!к пб 150 raises")
	}
}

func TestBossTweaks(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	k := g.spawnBoss("bone_knight", 1000)
	k.shieldUp, k.ability = true, 0
	g.tickBoss(k, 0.1)
	if k.shieldUp || k.ability != 6 {
		t.Fatalf("the bone knight's shield takes 6 s to come back: %.1f", k.ability)
	}

	g.Wave = 10
	if math.Abs(g.sdfHaste()-1.2) > 1e-9 {
		t.Fatalf("the guest walks 10%% faster for every 5th wave: x%.2f", g.sdfHaste())
	}
	g.Wave = 200
	if g.sdfHaste() != 1+g.Cfg.SdfHasteMax {
		t.Fatal("up to twice as fast")
	}
}
