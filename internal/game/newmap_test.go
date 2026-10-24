package game

import "testing"

// mapParty seats n players and starts a few waves into a game.
func mapParty(t *testing.T, names ...string) *Game {
	g, _ := newTestGame(t)
	for _, n := range names {
		g.HandleChat(ChatMessage{User: n, Text: "!play"})
	}
	g.Wave = 7
	return g
}

func TestNewMapAtOnce(t *testing.T) {
	g := mapParty(t, "anna")
	g.HandleChat(ChatMessage{User: "anna", Text: "!нк болото"})
	if g.Wave != 0 || g.Map.Biome != BiomeSwamp || g.mapVote != nil {
		t.Fatal("a lone player starts a new map at once")
	}

	g2 := mapParty(t, "anna", "boris", "vera")
	g2.HandleChat(ChatMessage{User: "boris", Text: "!новаякарта", Mod: true})
	if g2.Wave != 0 || g2.mapVote != nil {
		t.Fatal("so does a moderator")
	}
}

func TestNewMapVote(t *testing.T) {
	g := mapParty(t, "anna", "boris", "vera", "gleb")
	g.HandleChat(ChatMessage{User: "anna", Text: "!нк крипта"})
	if g.mapVote == nil || g.Wave != 7 {
		t.Fatal("anyone else puts it to a vote")
	}
	g.HandleChat(ChatMessage{User: "boris", Text: "!против"})
	run(g, g.Cfg.NewMapVoteSec+1)
	if g.Wave != 0 || g.Map.Biome != BiomeCrypt {
		t.Fatal("one against of four, the silent agree: a new map")
	}
}

func TestNewMapVoteFails(t *testing.T) {
	g := mapParty(t, "anna", "boris", "vera", "gleb")
	g.HandleChat(ChatMessage{User: "anna", Text: "!newmap"})
	g.HandleChat(ChatMessage{User: "boris", Text: "!против"})
	g.HandleChat(ChatMessage{User: "vera", Text: "!против"})
	if g.mapVote != nil || g.Wave != 7 {
		t.Fatal("half against: the game goes on, at once")
	}
	g.HandleChat(ChatMessage{User: "gleb", Text: "!нк"})
	if g.mapVote != nil {
		t.Fatal("and no new vote for a while")
	}
	g.HandleChat(ChatMessage{User: "gleb", Text: "!тюрьма anna"})
	g.HandleChat(ChatMessage{User: "vera", Text: "!нк"})
	if g.mapVote != nil {
		t.Fatal("nor during a jail vote")
	}
}

func TestTwoPlayersNewMap(t *testing.T) {
	g := mapParty(t, "anna", "boris")
	g.HandleChat(ChatMessage{User: "anna", Text: "!нк"})
	g.HandleChat(ChatMessage{User: "boris", Text: "!против"})
	if g.Wave != 7 {
		t.Fatal("of two, one against is enough to stay")
	}
	g2 := mapParty(t, "anna", "boris")
	g2.HandleChat(ChatMessage{User: "anna", Text: "!нк"})
	g2.HandleChat(ChatMessage{User: "boris", Text: "!за"})
	if g2.Wave != 0 {
		t.Fatal("both for: a new map at once")
	}
}
