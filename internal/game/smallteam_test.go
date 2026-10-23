package game

import (
	"math"
	"testing"
)

func TestSmallTeamBoost(t *testing.T) {
	g, _ := newTestGame(t)
	for n, want := range map[float64]float64{1: 2, 2: 1, 3: 0, 5: 0, 0: 2} {
		if got := g.smallBoost(n, 3); math.Abs(got-want) > 1e-9 {
			t.Fatalf("%v players earn %v more on a x3 solo payout, not %v", n, want, got)
		}
	}
	if g.smallBoost(1.5, 1.5) != 0.375 {
		t.Fatal("an investment-weighted party sits in between")
	}
}

func TestSoloEarnsMore(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	a := g.Chatters["anna"]
	placeAs(g, "anna", "arrow", freeCell(t, g))
	g.StartWave()
	if g.smallKill != g.Cfg.SmallKillMult-1 {
		t.Fatalf("a solo wave pays x%.2f per kill", 1+g.smallKill)
	}
	a.Gold = 0
	g.damage(enemyAt(g, "skeleton", 5, 10), 1e6, "anna", true)
	if want := int(math.Round(float64(g.Cfg.Enemies["skeleton"].Bounty) * g.Cfg.SmallKillMult)); a.Gold != want {
		t.Fatalf("a solo kill pays %d, got %d", want, a.Gold)
	}
	a.Gold = 0
	g.Enemies, g.queue = nil, nil
	g.endWave()
	if want := int(math.Round(float64(10+2*g.Wave) * g.Cfg.SmallWaveMult)); a.Gold != want {
		t.Fatalf("a solo wave bonus is %d, got %d", want, a.Gold)
	}

	for _, n := range []string{"boris", "vera"} {
		g.HandleChat(ChatMessage{User: n, Text: "!play"})
	}
	g.StartWave()
	if g.smallKill != 0 {
		t.Fatal("three players are paid as usual")
	}
}
