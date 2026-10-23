package store

import (
	"path/filepath"
	"testing"

	"twitchtd/internal/game"
)

func TestSaveLoad(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "td.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, ok, err := s.Load(); ok || err != nil {
		t.Fatalf("empty db: ok=%v err=%v", ok, err)
	}
	in := game.Save{
		Seed: 42, Biome: game.BiomeCrypt, Wave: 7, Walls: 640,
		Towers: []game.SavedTower{{Kind: "arrow", X: 3, Y: 4, Owner: "vasya", Level: 2, Invested: 150, Built: true}},
		Chatters: []game.SavedChatter{{Login: "vasya", Name: "Вася", Gold: 320, Kills: 12,
			Progress: game.Progress{Souls: 55, Ach: []string{"first_blood"}, Stats: map[string]int{"games": 2}}}},
		Memes: []*game.MemeRequest{{ID: 3, Login: "vasya", Name: "Вася", Coins: 2, Souls: 20}},
	}
	for i := 0; i < 2; i++ { // the second save must update, not duplicate
		if err := s.Save(in); err != nil {
			t.Fatal(err)
		}
	}
	out, ok, err := s.Load()
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if out.Seed != 42 || out.Wave != 7 || len(out.Towers) != 1 || out.Towers[0].Level != 2 {
		t.Fatalf("game %+v", out)
	}
	if len(out.Chatters) != 1 || out.Chatters[0].Name != "Вася" || out.Chatters[0].Gold != 320 {
		t.Fatalf("chatters %+v", out.Chatters)
	}
	if p := out.Chatters[0].Progress; p.Souls != 55 || len(p.Ach) != 1 || p.Stats["games"] != 2 || len(out.Memes) != 1 {
		t.Fatalf("progress %+v memes %+v", p, out.Memes)
	}
}

// A restart mid-game (an update, say) goes through the database: whoever
// already played this game is no newcomer after it and gets no grant.
func TestRestartKeepsSession(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "td.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	g := game.New(game.DefaultConfig(), 7)
	for _, n := range []string{"anna", "boris"} {
		g.HandleChat(game.ChatMessage{User: n, Text: "!play"})
	}
	g.Wave = 8
	g.HandleChat(game.ChatMessage{User: "anna", Text: "!class маг"})
	placed := 0
	for y := 0; y < game.GridH && placed < 2; y++ {
		for x := 0; x < game.GridW && placed < 2; x++ {
			if p := (game.Pos{X: x, Y: y}); g.Map.Grid.At(p) == game.CellBuild {
				g.Towers[p] = &game.Tower{Kind: "arrow", Cell: p, Owner: []string{"anna", "boris"}[placed], Level: 3, Invested: 500, Built: true}
				placed++
			}
		}
	}
	if err := s.Save(g.Export()); err != nil {
		t.Fatal(err)
	}

	save, ok, err := s.Load()
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	g2 := game.New(game.DefaultConfig(), 7)
	g2.LoadChatters(save.Chatters)
	g2.Restore(save)
	g2.HandleChat(game.ChatMessage{User: "anna", Text: "!play"})
	g2.HandleChat(game.ChatMessage{User: "boris", Text: "!b C4 arrow"})
	if a, b := g2.Chatters["anna"], g2.Chatters["boris"]; a.Grant != 0 || b.Grant != 0 {
		t.Fatalf("players back after a restart are no newcomers: grants %d, %d", a.Grant, b.Grant)
	}
	g2.HandleChat(game.ChatMessage{User: "anna", Text: "!class жрец"})
	if c := g2.Chatters["anna"].Class; c != "mage" {
		t.Fatalf("nor may they change class again: %s", c)
	}
	g2.HandleChat(game.ChatMessage{User: "vera", Text: "!play"})
	if g2.Chatters["vera"].Grant <= 0 {
		t.Fatal("a real newcomer still gets the grant")
	}
}
