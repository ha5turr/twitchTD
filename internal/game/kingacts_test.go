package game

import (
	"strings"
	"testing"
)

// actsGame is kingGame with the King in Yellow's two acts.
func actsGame(t *testing.T) (*Game, *Enemy) {
	t.Helper()
	g, k := kingGame(t, BiomeRuins)
	g.Cfg.KingActs = 2
	return g, k
}

func TestKingFallsAndRises(t *testing.T) {
	g, k := actsGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	gun := placeAs(g, "anna", "arrow", freeCell(t, g))
	nearKing(g, k, gun.Cell)
	gun.kingDmg = 1
	dist := k.Dist
	g.damage(k, k.HP+1, "anna", true)
	g.sweepDead()
	if g.king() != nil || g.kingRest != k || k.HP != k.MaxHP {
		t.Fatal("he falls off the field with a full bar for the next act")
	}
	if !g.burned[gun.Cell] {
		t.Fatal("the ground where he was hurt most burns")
	}
	run(g, 1)
	if g.Phase != PhaseWave {
		t.Fatal("the final waits for his next act")
	}
	if gun.Disabled <= 0 {
		t.Fatal("a building on burnt ground is silent")
	}
	run(g, g.Cfg.KingAntractSec)
	if g.king() != k || k.kingAct != 1 || k.Dist < dist || !strings.Contains(k.Title, "акт 2") {
		t.Fatalf("he rises further along for his second act: %v %d %.1f>%.1f %q", g.king() != nil, k.kingAct, k.Dist, dist, k.Title)
	}
	g.damage(k, k.HP*10, "", true) // past his torn cloak
	if !k.dead || g.kingRest != nil {
		t.Fatal("in his last act he dies")
	}
}

func TestKingPhasesSpanHisActs(t *testing.T) {
	g, k := actsGame(t)
	k.HP = k.MaxHP * 0.5
	run(g, 0.1)
	if k.phase != 1 {
		t.Fatalf("half his first bar is three quarters of him: phase %d", k.phase)
	}
	k.HP = k.MaxHP * 0.35
	run(g, 0.1)
	if k.phase != 2 {
		t.Fatalf("below 70%% of all his health the second phase: %d", k.phase)
	}
	k.kingAct = 1
	k.HP = k.MaxHP * 0.5
	run(g, 0.1)
	if k.phase != 3 || g.maskLeft <= 0 {
		t.Fatal("half his last bar is a quarter of him: the mask comes off")
	}
}

func TestCarryABuilding(t *testing.T) {
	g, k := actsGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	from := freeCell(t, g)
	gun := placeAs(g, "anna", "arrow", from)
	to := freeCell(t, g)
	g.Treasury = 500
	g.HandleChat(ChatMessage{User: "anna", Text: "!перенести " + from.Name() + " " + to.Name()})
	if gun.moveTo == nil || g.Treasury != 500-g.Cfg.KingMoveCost {
		t.Fatalf("the treasury pays for the carry: %d", g.Treasury)
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!b " + to.Name() + " arrow"})
	if g.Towers[to] != nil {
		t.Fatal("the cell it goes to is taken")
	}
	run(g, 0.1)
	if gun.Disabled <= 0 {
		t.Fatal("it is silent from the order on")
	}
	k.Dist = 0 // far from the mercenary's way
	for i := 0; i < 1200 && (gun.packed || gun.moveTo != nil); i++ {
		k.HP = k.MaxHP
		run(g, 0.05)
	}
	if g.Towers[to] != gun || g.Towers[from] != nil || gun.packed || gun.moveTo != nil || gun.Level != 1 {
		t.Fatal("it stands on its new cell, whole")
	}
	run(g, 0.1)
	if gun.Disabled > 0 {
		t.Fatal("and works again")
	}
}

func TestCarryOnlyInTheKingsFight(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	from := freeCell(t, g)
	gun := placeAs(g, "anna", "arrow", from)
	g.HandleChat(ChatMessage{User: "anna", Text: "!перенести " + from.Name() + " " + freeCell(t, g).Name()})
	if gun.moveTo != nil {
		t.Fatal("no carrying before the King")
	}
}

func TestCarryCancelled(t *testing.T) {
	g, _ := actsGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	from := freeCell(t, g)
	gun := placeAs(g, "anna", "arrow", from)
	g.Treasury = 500
	g.HandleChat(ChatMessage{User: "anna", Text: "!перенести " + from.Name() + " " + freeCell(t, g).Name()})
	g.HandleChat(ChatMessage{User: "anna", Text: "!отмена"})
	if gun.moveTo != nil || g.Towers[from] != gun {
		t.Fatal("a carry not begun is called off, the building stays")
	}
}

func TestRainOnceAWave(t *testing.T) {
	g, _ := newTestGame(t)
	g.Cfg.Rewards = []RewardSpec{{Title: "Дождь", Action: "strike", Amount: 50}}
	g.Phase, g.Wave = PhaseWave, 3
	if g.Reward("anna", "anna", "Дождь") {
		t.Fatal("the first rain falls")
	}
	if !g.Reward("bob", "bob", "Дождь") {
		t.Fatal("the second in the same wave is refused, points back")
	}
	g.Wave = 4
	if g.Reward("bob", "bob", "Дождь") {
		t.Fatal("the next wave has its own")
	}
}

func TestRainOnceAnAct(t *testing.T) {
	g, k := actsGame(t)
	g.Cfg.Rewards = []RewardSpec{{Title: "Дождь", Action: "strike", Amount: 50}}
	if g.Reward("anna", "anna", "Дождь") || !g.Reward("bob", "bob", "Дождь") {
		t.Fatal("one rain in his first act")
	}
	g.damage(k, k.HP+1, "", true)
	g.sweepDead()
	if g.Reward("bob", "bob", "Дождь") {
		t.Fatal("his next act has its own")
	}
}

func TestKingThroughAnyBarricade(t *testing.T) {
	g, k := actsGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	c := g.Chatters["anna"]
	c.Class, c.Gold = "warrior", 5000
	var cell Pos
	for i, p := range g.Map.Path {
		if i > len(g.Map.Path)/2 {
			cell = p
			break
		}
	}
	wall := placeAs(g, "anna", "wall", cell)
	wall.Level = g.Cfg.Towers["wall"].MaxLevel + 1 // an artifact barricade
	g.readyTower(wall)
	k.blocked = wall
	wall.HP = wall.MaxHP * 0.9
	g.HandleChat(ChatMessage{User: "anna", Text: "!чинить " + cell.Name()})
	if wall.repairing {
		t.Fatal("a barricade he hacks at cannot be mended")
	}
	spec := g.Cfg.Enemies[kingKind]
	for i := 0; i < int(g.Cfg.KingWallSec*20)+2 && g.Towers[cell] == wall; i++ {
		g.hackBarricade(k, spec, 1, 0.05)
	}
	if g.Towers[cell] == wall {
		t.Fatalf("he is through it in %.0f s", g.Cfg.KingWallSec)
	}
}
