package game

import (
	"strings"
	"testing"
)

// sdfGame starts a wave with ЭссДиЭф on the route and three chatters in the tavern.
func sdfGame(t *testing.T) (*Game, *Enemy) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	for _, u := range []string{"anna", "boris", "vera"} {
		g.HandleChat(ChatMessage{User: u, Text: "привет"})
		g.Chatters[u].Gold = 200
	}
	e := g.spawnBoss(sdfKind, 10000)
	e.Dist = g.cellDist[6]
	e.Pos = g.routePos(e.Dist)
	e.ability = 1e9 // tricks only when a test plays them
	return g, e
}

func TestClownBuildRules(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "привет"})
	g.Chatters["anna"].Gold = 1000
	p := freeCell(t, g)
	g.HandleChat(ChatMessage{User: "anna", Text: "!build " + p.Name() + " клоун"})
	if g.Towers[p] != nil || !strings.Contains(lastReply(g), "ЭссДиЭф") {
		t.Fatal("no clowns while ЭссДиЭф is away")
	}
	g.Cfg.TowerLimit = 1
	g.HandleChat(ChatMessage{User: "anna", Text: "!build " + p.Name()})
	g.spawnBoss(sdfKind, 1000)
	q := freeCell(t, g)
	g.HandleChat(ChatMessage{User: "anna", Text: "!build " + q.Name() + " clown"})
	if g.Towers[q] == nil || g.Towers[q].Kind != clownKind {
		t.Fatalf("a clown is allowed while he is out, past the tower limit: %q", lastReply(g))
	}
	r := freeCell(t, g)
	g.HandleChat(ChatMessage{User: "anna", Text: "!build " + r.Name() + " clown"})
	if g.Towers[r] != nil {
		t.Fatal("one clown per chatter")
	}
	gold := g.Chatters["anna"].Gold
	g.endWave()
	if g.Towers[q] != nil || g.Chatters["anna"].Gold < gold+g.Cfg.Towers[clownKind].Cost {
		t.Fatal("clowns are packed up after the wave with a full refund")
	}
}

func TestClownHitsOnlySdf(t *testing.T) {
	g, e := sdfGame(t)
	place(g, clownKind, freeCell(t, g))
	sk := enemyAt(g, "skeleton", 3, 1000)
	run(g, 3)
	if sk.HP != 1000 {
		t.Fatal("clowns ignore everyone else")
	}
	if e.HP >= 10000*0.99 {
		t.Fatalf("clowns hit ЭссДиЭф anywhere on the map: hp %.0f", e.HP)
	}
	arrow := g.Cfg.Towers["arrow"]
	if g.Cfg.Towers[clownKind].BossPct*e.MaxHP < 5*arrow.Damage {
		t.Fatal("a clown shot should be far stronger than an arrow against him")
	}
}

func TestSdfStopsForTricks(t *testing.T) {
	g, e := sdfGame(t)
	from := e.Dist
	ev := g.sdfPlay(e, "offroad")
	if ev.Text != sdfKind || ev.Amount != 1 || e.act != 1 {
		t.Fatalf("the first trick is a burp: %+v", ev)
	}
	run(g, sdfPause-0.2)
	if e.Dist != from {
		t.Fatal("he stands still while burping")
	}
	run(g, 3)
	if e.Dist <= from || e.act != 0 {
		t.Fatal("then walks on")
	}
	if e.Pos.sub(g.routePos(e.Dist)).len() < 1 {
		t.Fatal("offroad takes him off the path")
	}
	if g.sdfPlay(e, "disable").Amount != 2 {
		t.Fatal("the second trick is a fart")
	}
	run(g, sdfOffSec+3)
	if e.Pos.sub(g.routePos(e.Dist)).len() > 0.01 {
		t.Fatal("and he comes back to the path")
	}
}

func TestSdfDisablesAndFlattens(t *testing.T) {
	g, e := sdfGame(t)
	var near, clown *Tower
	for _, p := range g.Map.Path {
		for _, d := range dirs {
			q := Pos{p.X + d.X, p.Y + d.Y}
			if q.In() && g.Map.Grid.At(q) == CellBuild && g.Towers[q] == nil && center(q).sub(e.Pos).len() <= sdfFlatten {
				if near == nil {
					near = place(g, "arrow", q)
				} else if clown == nil {
					clown = place(g, clownKind, q)
				}
			}
		}
	}
	if near == nil || clown == nil {
		t.Skip("map has no room around the boss")
	}
	g.sdfPlay(e, "disable")
	if near.Disabled <= 0 || clown.Disabled > 0 {
		t.Fatal("buildings around him switch off, clowns do not")
	}
	g.sdfPlay(e, "destroy")
	if g.Towers[near.Cell] != nil || g.Towers[clown.Cell] == nil {
		t.Fatal("buildings around him are flattened, clowns are not")
	}
}

func TestSdfRobsTheTavern(t *testing.T) {
	g, e := sdfGame(t)
	g.Chatters["vera"].Gold = 20
	g.sdfPlay(e, "tavern")
	if g.Chatters["anna"].Gold != 150 || g.Chatters["vera"].Gold != 0 {
		t.Fatalf("robbed 50 each (or what there was): anna %d vera %d", g.Chatters["anna"].Gold, g.Chatters["vera"].Gold)
	}
	run(g, 1)
	if e.Pos != sdfTavern {
		t.Fatal("he stands in the tavern while robbing")
	}
	run(g, sdfTavernSec)
	if e.Pos.sub(g.routePos(e.Dist)).len() > 0.01 {
		t.Fatal("then returns to the path")
	}
	g.damage(e, 1e9, "boris", true)
	if g.Chatters["anna"].Gold != 200 || g.Chatters["vera"].Gold != 20 {
		t.Fatal("his death returns the loot")
	}
}

func TestSdfChance(t *testing.T) {
	g, _ := newTestGame(t)
	g.Cfg.SdfChance = 1
	g.Wave = g.Cfg.BossEvery - 1
	g.StartWave()
	if last := g.queue[len(g.queue)-1]; last.kind != sdfKind || last.hp != g.waveHP*g.Cfg.BossHPMult*g.Cfg.Enemies[sdfKind].HPMult {
		t.Fatalf("chance 1 always brings him, with his extra hit points: %+v", last)
	}
	g2, _ := newTestGame(t)
	g2.Cfg.SdfChance = 0
	g2.Wave = g2.Cfg.BossEvery - 1
	g2.StartWave()
	if g2.queue[len(g2.queue)-1].kind == sdfKind {
		t.Fatal("chance 0 never does")
	}
}
