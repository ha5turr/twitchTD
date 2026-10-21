package game

import (
	"strings"
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
	if DefaultConfig().RogueStepDmg != 1.0 {
		t.Fatal("the shadow-step stab was doubled, then raised again in 0.12.1")
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

func TestStreamerPlaysOnEqualTerms(t *testing.T) {
	g, _ := newTestGame(t)
	p := freeCell(t, g)
	g.HandleChat(ChatMessage{Login: "ha5turr", User: "Ha5turr", Mod: true, Broadcaster: true, Text: "!build " + p.Name()})
	if tw := g.Towers[p]; tw == nil || tw.Owner != StreamerName || !tw.Built {
		t.Fatal("by default the broadcaster builds for free and at once")
	}
	g.Cfg.StreamerPlays = true
	q := freeCell(t, g)
	g.HandleChat(ChatMessage{Login: "ha5turr", User: "Ha5turr", Mod: true, Broadcaster: true, Text: "!build " + q.Name()})
	c := g.Chatters["ha5turr"]
	if tw := g.Towers[q]; tw == nil || tw.Owner != "ha5turr" || tw.Built || c == nil || !c.Joined || c.Gold != g.Cfg.StartGold+g.Cfg.ChatGold-g.Cfg.Towers["arrow"].Cost {
		t.Fatal("with streamerPlays the broadcaster pays, waits for a mercenary and sits in the tavern")
	}
	g.HandleChat(ChatMessage{User: StreamerName, Mod: true, Broadcaster: true, Text: "!build " + freeCell(t, g).Name()})
	if g.towersOf(StreamerName) != 2 {
		t.Fatal("the panel still builds for free")
	}
	gold := c.Gold
	g.HandleChat(ChatMessage{Login: "ha5turr", User: "Ha5turr", Mod: true, Broadcaster: true, Text: "!tdgold ha5turr 50"})
	if c.Gold != gold+50 {
		t.Fatalf("admin commands still work from chat: gold %d", c.Gold)
	}
}

func TestLeaveSellsAndEmptiesTheWallet(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	c := g.Chatters["anna"]
	a := placeAs(g, "anna", "arrow", freeCell(t, g))
	a.Invested = 100
	placeAs(g, "anna", "magic", freeCell(t, g)).Invested = 200
	other := placeAs(g, "boris", "arrow", freeCell(t, g))
	c.Gold = 50
	treasury := g.Treasury
	active := g.ActiveChatters()
	g.HandleChat(ChatMessage{User: "anna", Text: "!leave"})
	want := 50 + (100+200)*g.Cfg.SellRefundPct/100
	if g.towersOf("anna") != 0 || g.Towers[other.Cell] == nil {
		t.Fatal("only the leaver's buildings are sold")
	}
	if c.Gold != 0 || g.Treasury != treasury+want {
		t.Fatalf("wallet and refunds go to the treasury: gold %d, treasury +%d, want +%d", c.Gold, g.Treasury-treasury, want)
	}
	if c.Joined || g.ActiveChatters() != active-1 {
		t.Fatal("the leaver is out of the tavern and the difficulty")
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!leave"})
	if r := lastReply(g); !strings.Contains(r, "не в игре") {
		t.Fatalf("a second !leave just says so: %q", r)
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	if !c.Joined {
		t.Fatal("!play brings them back")
	}
}
