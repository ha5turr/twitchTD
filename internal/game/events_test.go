package game

import "testing"

func TestGuestsJoinOrdinaryWaves(t *testing.T) {
	g, _ := newTestGame(t)
	g.Cfg.EventChance = 1
	g.Wave = g.Cfg.EventFromWave - 1
	g.StartWave()
	found := ""
	for _, s := range g.queue {
		if isGuest(s.kind) {
			found = s.kind
		}
	}
	if found == "" {
		t.Fatal("chance 1 brings a guest on an ordinary wave")
	}
	g2, _ := newTestGame(t)
	g2.Cfg.EventChance = 1
	g2.Wave = g2.Cfg.BossEvery - 1
	g2.StartWave()
	for _, s := range g2.queue {
		if isGuest(s.kind) {
			t.Fatal("never on a boss wave")
		}
	}
	g3, _ := newTestGame(t)
	g3.Cfg.EventChance = 1
	g3.Wave = 0
	g3.StartWave()
	for _, s := range g3.queue {
		if isGuest(s.kind) {
			t.Fatal("never before eventFromWave")
		}
	}
}

func TestTaxCollector(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	g.Treasury = 1000
	walls := g.Walls
	e := g.spawnBoss(taxKind, 1e9)
	e.Dist = g.total - 0.1
	run(g, 1)
	if !e.dead || g.Treasury != 500 || g.Walls != walls {
		t.Fatalf("at the gate he takes half the treasury and leaves the walls alone: dead %v treasury %d", e.dead, g.Treasury)
	}
	e2 := g.spawnBoss(taxKind, 100)
	g.damage(e2, 1e9, "", true)
	if g.Treasury <= 500 {
		t.Fatal("killed, he adds a bonus to the treasury")
	}
}

func TestGoldenGolemRain(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	for _, u := range []string{"anna", "boris", "vera"} {
		g.HandleChat(ChatMessage{User: u, Text: "!play"})
		g.Chatters[u].Gold = 0
	}
	e := g.spawnBoss(golemKind, 1000)
	g.damage(e, 300, "anna", true)
	g.damage(e, 100, "boris", true)
	g.damage(e, 1000, "anna", true)
	a, b, v := g.Chatters["anna"].Gold, g.Chatters["boris"].Gold, g.Chatters["vera"].Gold
	if b <= 0 || a <= b || v != 0 {
		t.Fatalf("gold goes to those who hit it, by their share: anna %d boris %d vera %d", a, b, v)
	}
	run2 := g.spawnBoss(golemKind, 1e9)
	run2.Dist = g.total - 0.1
	walls := g.Walls
	run(g, 1)
	if !run2.dead || g.Walls != walls {
		t.Fatal("an escaped golem just leaves")
	}
}
