package game

import (
	"testing"
	"time"
)

func newTestGame(t *testing.T) (*Game, *time.Time) {
	t.Helper()
	g := New(DefaultConfig(), 7)
	now := time.Unix(1_700_000_000, 0)
	g.Now = func() time.Time { return now }
	return g, &now
}

// freeCell returns a buildable cell next to the path.
func freeCell(t *testing.T, g *Game) Pos {
	t.Helper()
	for y := 0; y < GridH; y++ {
		for x := 0; x < GridW; x++ {
			p := Pos{x, y}
			if g.Map.Grid.At(p) == CellBuild && nextToPath(g, p) && g.Towers[p] == nil {
				return p
			}
		}
	}
	t.Fatal("no free cell")
	return Pos{}
}

func run(g *Game, secs float64) {
	for i := 0; i < int(secs*20); i++ {
		g.Tick(0.05)
	}
}

func TestMercenaryBuildsTower(t *testing.T) {
	g, _ := newTestGame(t)
	p := freeCell(t, g)
	g.HandleChat(ChatMessage{Login: "alice", User: "Алиса", Text: "!build " + p.Name()})
	tw := g.Towers[p]
	if tw == nil || tw.Built {
		t.Fatalf("expected a construction site at %s, got %+v", p.Name(), tw)
	}
	if g.Workers["alice"] == nil {
		t.Fatal("a mercenary should walk out")
	}
	run(g, 20) // walk across the field and build, still in the build phase
	if !tw.Built {
		t.Fatalf("tower not built, worker %+v", g.Workers["alice"])
	}
	if g.Workers["alice"] != nil {
		t.Fatal("the mercenary should be back in the tavern")
	}
	if g.Chatters["alice"].Name != "Алиса" {
		t.Fatalf("display name lost: %q", g.Chatters["alice"].Name)
	}

	g.Chatters["alice"].Gold = 500
	g.HandleChat(ChatMessage{Login: "alice", User: "Алиса", Text: "!up " + p.Name()})
	if tw.Level != 1 || tw.PendingCost == 0 {
		t.Fatalf("upgrade must wait for the mercenary: %+v", tw)
	}
	run(g, 20)
	if tw.Level != 2 || tw.PendingCost != 0 {
		t.Fatalf("upgrade not applied: %+v", tw)
	}
}

func TestStreamerBuildsInstantly(t *testing.T) {
	g, _ := newTestGame(t)
	p := freeCell(t, g)
	g.HandleChat(ChatMessage{Login: "the_streamer", User: "TheStreamer", Text: "!b " + p.Name() + " magic", Broadcaster: true})
	tw := g.Towers[p]
	if tw == nil || !tw.Built || tw.Kind != "magic" || len(g.Workers) != 0 {
		t.Fatalf("streamer tower should appear at once: %+v workers %d", tw, len(g.Workers))
	}
}

func TestOrderQueueLimit(t *testing.T) {
	g, _ := newTestGame(t)
	g.chatter("bob", "bob").Gold = 900
	for i := 0; i < g.Cfg.MaxOrders+2; i++ {
		g.HandleChat(ChatMessage{User: "bob", Text: "!build " + freeCell(t, g).Name()})
	}
	if n := len(g.Workers["bob"].Orders); n != g.Cfg.MaxOrders {
		t.Fatalf("orders %d, want %d", n, g.Cfg.MaxOrders)
	}
}

func TestSellConstructionRefundsInFull(t *testing.T) {
	g, _ := newTestGame(t)
	p := freeCell(t, g)
	g.HandleChat(ChatMessage{User: "carol", Text: "!build " + p.Name()})
	before := g.Chatters["carol"].Gold
	g.HandleChat(ChatMessage{User: "carol", Text: "!sell " + p.Name()})
	if got := g.Chatters["carol"].Gold - before; got != g.Cfg.TowerCost("arrow", 1) {
		t.Fatalf("refund %d, want full price", got)
	}
	if len(g.Workers["carol"].Orders) != 0 {
		t.Fatal("the order should be cancelled")
	}
}

func TestEnemyStunsMercenary(t *testing.T) {
	g, _ := newTestGame(t)
	p := freeCell(t, g)
	g.HandleChat(ChatMessage{User: "dave", Text: "!build " + p.Name()})
	g.Chatters["dave"].Class = "mage"
	w := g.Workers["dave"]
	w.Pos = center(p)
	g.Enemies = append(g.Enemies, &Enemy{ID: 999, Kind: "skeleton", HP: 1e9, MaxHP: 1e9, Pos: center(p)})
	g.tickWorkers(0.05)
	if w.State != WorkerFlee {
		t.Fatalf("state %s, want flee", w.State)
	}
	g.Chatters["dave"].Class = "warrior"
	w.State = WorkerBuild
	g.tickWorkers(0.05)
	if w.State == WorkerFlee {
		t.Fatal("the Воитель is never stunned")
	}
}

func TestIgnoredUsers(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{Login: "Nightbot", User: "Nightbot", Text: "!build A1"})
	if g.Chatters["nightbot"] != nil {
		t.Fatal("bots must be ignored")
	}
}

func TestSaveRestore(t *testing.T) {
	g, _ := newTestGame(t)
	p := freeCell(t, g)
	g.HandleChat(ChatMessage{User: "erin", Text: "!build " + p.Name()})
	run(g, 20)
	q := freeCell(t, g)
	g.HandleChat(ChatMessage{User: "erin", Text: "!build " + q.Name()}) // left unfinished
	g.StartWave()
	g.Wave = 4
	save := g.Export()
	goldAtSave := g.Chatters["erin"].Gold

	h, _ := newTestGame(t)
	h.LoadChatters(save.Chatters)
	h.Restore(save)
	if h.Map.Biome != g.Map.Biome || h.Map.Grid != g.Map.Grid {
		t.Fatal("map not reproduced")
	}
	if h.Wave != 3 || h.Phase != PhaseBuild {
		t.Fatalf("interrupted wave should be replayed: wave %d phase %s", h.Wave, h.Phase)
	}
	if h.Towers[p] == nil || !h.Towers[p].Built || h.Towers[q] != nil {
		t.Fatal("built tower kept, construction site dropped")
	}
	if got := h.Chatters["erin"].Gold; got != goldAtSave+h.Cfg.TowerCost("arrow", 1) {
		t.Fatalf("gold %d, want %d (unfinished site refunded)", got, goldAtSave+h.Cfg.TowerCost("arrow", 1))
	}
}
