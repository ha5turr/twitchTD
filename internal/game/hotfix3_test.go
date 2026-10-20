package game

import "testing"

func TestBallistaNetsHoldEnemies(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	a, _, i := pathNeighbours(t, g)
	b := place(g, "arrow", a)
	b.cooldown = 1e9 // only the net
	e := enemyAt(g, "skeleton", i, 1000)
	g.tickTowers(0.05)
	spec := g.Cfg.Towers["arrow"]
	if e.netLeft != spec.NetSec || e.HP != 1000 {
		t.Fatalf("a net holds the enemy without hurting it: net %.1f hp %.0f", e.netLeft, e.HP)
	}
	d := e.Dist
	run(g, spec.NetSec-0.2)
	if e.Dist != d {
		t.Fatal("a netted enemy does not move")
	}
	run(g, 1)
	if e.Dist == d {
		t.Fatal("then walks on")
	}
	if b.netCD <= 0 || b.netCD > spec.NetEvery {
		t.Fatalf("the ballista reloads its net: %.1f", b.netCD)
	}
	b.Level, b.netCD = 5, 0
	e.netLeft = 0
	g.tickTowers(0.05)
	if b.netCD >= spec.NetEvery {
		t.Fatal("a better ballista nets more often")
	}
}

func TestBotIsIgnored(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "Hasturyan", Text: "!build C4"})
	if g.Chatters["hasturyan"] != nil {
		t.Fatal("the channel's own bot never joins the game")
	}
}
