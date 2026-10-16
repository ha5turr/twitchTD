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
		Towers:   []game.SavedTower{{Kind: "arrow", X: 3, Y: 4, Owner: "vasya", Level: 2, Invested: 150, Built: true}},
		Chatters: []game.SavedChatter{{Login: "vasya", Name: "Вася", Gold: 320, Kills: 12}},
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
}
