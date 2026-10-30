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

func TestKingPhasesInHisFirstAct(t *testing.T) {
	g, k := actsGame(t)
	k.HP = k.MaxHP * 0.8
	run(g, 0.1)
	if k.phase != 1 {
		t.Fatalf("80%% of his first bar: phase %d", k.phase)
	}
	k.HP = k.MaxHP * 0.5
	run(g, 0.1)
	if k.phase != 2 {
		t.Fatalf("below 70%% of his first bar the second phase: %d", k.phase)
	}
	k.HP = k.MaxHP * 0.25
	run(g, 0.1)
	if k.phase != 3 || g.maskLeft <= 0 {
		t.Fatal("below 30%% of his first bar the mask comes off")
	}
}

func TestLaterActsHaveAllHisSkills(t *testing.T) {
	g, k := actsGame(t)
	g.Cfg.KingActs = 3
	k.kingAct = 1
	run(g, 0.1)
	if k.phase != 3 || !k.masked {
		t.Fatalf("his second act opens with all his phases: %d %v", k.phase, k.masked)
	}
	faceless := func() int {
		n := 0
		for _, e := range g.Enemies {
			if e.Kind == facelessKind && !e.dead {
				n++
			}
		}
		return n
	}
	if faceless() == 0 {
		t.Fatal("the retinue comes in later acts too, under the mask")
	}
	if cd := g.actCD(k); cd >= 1 || k.retinueCD > retinueEvery*cd {
		t.Fatalf("and sooner: %.2f %.1f", cd, k.retinueCD)
	}
	k.kingAct = 2
	if g.actCD(k) >= g.Cfg.KingActCD {
		t.Fatal("sooner still in the third")
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

func TestMoreSignsEveryAct(t *testing.T) {
	g, k := actsGame(t)
	nearKing(g, k, freeCell(t, g))
	for y := 0; y < GridH; y++ {
		for x := 0; x < GridW; x++ {
			if p := (Pos{x, y}); g.Map.Grid.At(p) == CellBuild && center(p).sub(k.Pos).len() < 3 {
				placeAs(g, StreamerName, "arrow", p) // shooters all round him
			}
		}
	}
	k.kingAct = 1
	k.phase = 1
	k.signCD = 0.01
	run(g, 0.05)
	if len(g.signs) != 1+g.Cfg.KingSignPerAct {
		t.Fatalf("in his second act %d signs at once, got %d", 1+g.Cfg.KingSignPerAct, len(g.signs))
	}
}

func TestSongHoldsArtifacts(t *testing.T) {
	g, _ := actsGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	g.Chatters["anna"].Class = "ranger"
	gun := placeAs(g, "anna", "arrow", freeCell(t, g))
	gun.Level = g.Cfg.Towers["arrow"].MaxLevel + 1
	full := g.levelMult(gun)
	if !g.artifact(gun) {
		t.Fatal("an artifact")
	}
	g.songLeft = 3
	if g.artifact(gun) || g.levelMult(gun) >= full {
		t.Fatal("under the song it plays as a level-5 building")
	}
	g.songLeft = 0
	if !g.artifact(gun) {
		t.Fatal("and is an artifact again after it")
	}
}

func TestRetinueHasHisHealth(t *testing.T) {
	g, k := actsGame(t)
	g.summonRetinue(k, 2)
	n := 0
	for _, e := range g.Enemies {
		if e.Kind == facelessKind {
			n++
			if e.MaxHP < k.MaxHP*g.Cfg.KingRetinueHP*0.99 {
				t.Fatalf("a faceless one has a share of his health: %.0f of %.0f", e.MaxHP, k.MaxHP)
			}
		}
	}
	if n != 2 {
		t.Fatalf("two faceless: %d", n)
	}
}

func TestMaskAgainInLaterActs(t *testing.T) {
	g, k := actsGame(t)
	k.kingAct = 1 // the mask comes off as he rises
	run(g, 0.1)
	if !k.masked {
		t.Fatal("the mask is off")
	}
	run(g, maskSec+1)
	if g.maskLeft > 0 {
		t.Fatal("and back on")
	}
	run(g, g.Cfg.KingMaskEvery*g.actCD(k)-maskSec-1+1) // just past its cooldown since it first came off
	if g.maskLeft <= 0 {
		t.Fatal("after his first act it comes off again")
	}
}

func TestBearShovesTheKingFromTheDoor(t *testing.T) {
	g, k := actsGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	c := g.Chatters["anna"]
	c.Class = "druid"
	k.Dist, k.AtGate = g.total, true
	g.bearForm("anna", c)
	if k.AtGate || k.Dist > g.total-1.1 {
		t.Fatalf("the bear shoves even the King back from the door: dist %.2f of %.2f", k.Dist, g.total)
	}
}
