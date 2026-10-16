package game

import (
	"fmt"
	"testing"
	"time"
)

func TestParseCell(t *testing.T) {
	cases := map[string]string{"c4": "C4", "C 4": "C4", "4c": "C4", "p10": "P10", "a1": "A1", "С4": "C4"}
	for in, want := range cases {
		p, ok := ParseCell(in)
		if !ok || p.Name() != want {
			t.Errorf("ParseCell(%q) = %v %v, want %s", in, p.Name(), ok, want)
		}
	}
	for _, bad := range []string{"", "q1", "a0", "a11", "zz", "a123"} {
		if _, ok := ParseCell(bad); ok {
			t.Errorf("ParseCell(%q) should fail", bad)
		}
	}
}

func TestGenerateMap(t *testing.T) {
	cam, _ := DefaultConfig().WebcamRect()
	for _, b := range AllBiomes {
		for seed := uint64(0); seed < 200; seed++ {
			m := GenerateMap(seed, b, cam, true)
			name := fmt.Sprintf("%s/%d", b, seed)
			p := m.Path
			if p[0].X != GridW-1 || p[len(p)-1].X != 0 {
				t.Fatalf("%s: path must run from column P to A, got %v..%v", name, p[0], p[len(p)-1])
			}
			for i, c := range p {
				if cam.Has(c) {
					t.Fatalf("%s: path crosses the webcam zone at %s", name, c.Name())
				}
				if i > 0 {
					d := abs(c.X-p[i-1].X) + abs(c.Y-p[i-1].Y)
					if d != 1 {
						t.Fatalf("%s: path breaks between %s and %s", name, p[i-1].Name(), c.Name())
					}
				}
			}
			again := GenerateMap(seed, b, cam, true)
			if again.Grid != m.Grid {
				t.Fatalf("%s: generation is not deterministic", name)
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// TestFullGame plays waves with a few towers until the walls fall and a new
// game starts, checking nothing panics and the flow reaches every phase.
func TestFullGame(t *testing.T) {
	g := New(DefaultConfig(), 42)
	now := time.Unix(1_700_000_000, 0)
	g.Now = func() time.Time { return now }

	built := 0
	for y := 0; y < GridH && built < 3; y++ {
		for x := 0; x < GridW && built < 3; x++ {
			p := Pos{x, y}
			if g.Map.Grid.At(p) == CellBuild && nextToPath(g, p) {
				g.HandleChat(ChatMessage{User: fmt.Sprintf("viewer%d", built), Text: "!build " + p.Name() + " arrow"})
				built++
			}
		}
	}
	if len(g.Towers) != 3 {
		t.Fatalf("expected 3 towers, got %d; events %+v", len(g.Towers), g.TakeEvents())
	}

	sawWave, sawOver := false, false
	for i := 0; i < 20*60*60 && !(sawOver && g.Phase == PhaseBuild); i++ {
		g.Tick(0.05)
		now = now.Add(50 * time.Millisecond)
		switch g.Phase {
		case PhaseWave:
			sawWave = true
		case PhaseGameOver:
			sawOver = true
		}
		g.StateMessage() // exercise snapshot building and drain events
	}
	if !sawWave || !sawOver {
		t.Fatalf("flow incomplete: wave=%v gameOver=%v (wave %d, walls %.0f)", sawWave, sawOver, g.Wave, g.Walls)
	}
	if len(g.Towers) != 0 || g.Wave != 0 {
		t.Fatalf("new game should start clean, got %d towers, wave %d", len(g.Towers), g.Wave)
	}
}

func nextToPath(g *Game, p Pos) bool {
	for _, d := range dirs {
		q := Pos{p.X + d.X, p.Y + d.Y}
		if q.In() && g.Map.Grid.At(q) == CellPath {
			return true
		}
	}
	return false
}

func TestWalletCapOverflowRepairsWalls(t *testing.T) {
	g := New(DefaultConfig(), 1)
	c := g.Touch("donor")
	g.Walls = 100
	added := g.addGold(c, g.Cfg.WalletCap*2)
	if c.Gold != g.Cfg.WalletCap {
		t.Fatalf("gold %d, want cap %d", c.Gold, g.Cfg.WalletCap)
	}
	if added >= g.Cfg.WalletCap*2 || g.Walls <= 100 {
		t.Fatalf("overflow should repair walls: added %d, walls %.0f", added, g.Walls)
	}
}
