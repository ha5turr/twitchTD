package game

import (
	"testing"
	"time"
)

func TestShrineBuffsTheWholeMap(t *testing.T) {
	g, _ := newTestGame(t)
	place(g, "shrine", Pos{0, 0})
	far := place(g, "arrow", Pos{GridW - 1, GridH - 1})
	if dmg, _ := g.auras(far); dmg <= 1 {
		t.Fatal("a shrine's damage buff reaches every building on the map")
	}
}

func TestClericPillarOfLight(t *testing.T) {
	g, _ := newTestGame(t)
	skillReady(t, g, "kate", "cleric")
	crowd := []*Enemy{enemyAt(g, "skeleton", 5, 1e6), enemyAt(g, "skeleton", 5, 1e6)}
	lone := enemyAt(g, "skeleton", 12, 1e6)
	g.tickHeroes(0.05)
	for _, e := range crowd {
		if e.stunLeft <= 0 || e.HP >= 1e6 {
			t.Fatal("the pillar falls on the crowd: stunned and hurt")
		}
	}
	if lone.stunLeft > 0 {
		t.Fatal("others are spared")
	}
	d := crowd[0].Dist
	run(g, g.Cfg.PillarStunSec-0.2)
	if crowd[0].Dist != d {
		t.Fatal("a stunned enemy stands still")
	}
}

func TestPyroSetsThePathAlight(t *testing.T) {
	g, _ := newTestGame(t)
	skillReady(t, g, "pyra", "pyro")
	es := []*Enemy{enemyAt(g, "skeleton", 2, 1e6), enemyAt(g, "skeleton", 14, 1e6)}
	g.tickHeroes(0.05)
	for _, e := range es {
		if e.HP >= 1e6 || e.burnLeft <= 0 {
			t.Fatal("every enemy on the path is hit and set alight")
		}
	}
}

func TestRangerArrowRain(t *testing.T) {
	g, _ := newTestGame(t)
	skillReady(t, g, "robin", "ranger")
	near := []*Enemy{enemyAt(g, "skeleton", 6, 1e6), enemyAt(g, "skeleton", 7, 1e6)}
	g.tickHeroes(0.05)
	for _, e := range near {
		if e.HP >= 1e6 {
			t.Fatal("the volley hits everyone within two cells of the crowd")
		}
	}
}

func TestRogueHitsHarder(t *testing.T) {
	if DefaultConfig().RogueStepDmg != 0.8 {
		t.Fatal("the shadow-step stab was doubled")
	}
}

func TestAFKGoldGoesToTheTreasury(t *testing.T) {
	g, now := newTestGame(t)
	chatters(g, "anna")
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	g.Chatters["anna"].Gold = 300
	treasury := g.Treasury
	*now = now.Add(5 * time.Minute)
	run(g, 6)
	if g.Chatters["anna"].Gold != 300 {
		t.Fatal("five quiet minutes are not AFK yet")
	}
	*now = now.Add(time.Duration(g.Cfg.ActiveWindowMn) * time.Minute)
	run(g, 6)
	if g.Chatters["anna"].Gold != 0 || g.Treasury < treasury+300 {
		t.Fatalf("an AFK player's gold goes to the treasury: wallet %d", g.Chatters["anna"].Gold)
	}
	// Wallets loaded from the database (never active this session) stay.
	g.LoadChatters([]SavedChatter{{Login: "old", Name: "Old", Gold: 500}})
	run(g, 6)
	if g.Chatters["old"].Gold != 500 {
		t.Fatal("offline wallets are not swept")
	}
}

func TestOnlyPlayersJoinTheTavern(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "lurker", Text: "привет всем"})
	if g.ActiveChatters() != 0 {
		t.Fatal("a plain chat line does not count toward the difficulty")
	}
	for _, c := range g.StateMessage().Chatters {
		if c.Name == "lurker" {
			t.Fatal("nor puts a mercenary in the tavern")
		}
	}
	g.HandleChat(ChatMessage{User: "lurker", Text: "!помощь"})
	if g.Chatters["lurker"].Joined {
		t.Fatal("asking for help is not playing")
	}
	g.HandleChat(ChatMessage{User: "lurker", Text: "!drink"})
	if !g.Chatters["lurker"].Joined || g.ActiveChatters() != 1 {
		t.Fatal("any game command joins")
	}
	g.HandleChat(ChatMessage{User: "newbie", Text: "!play"})
	if r := lastReply(g); !g.Chatters["newbie"].Joined || r == "" {
		t.Fatal("!play joins with a welcome")
	}
}
