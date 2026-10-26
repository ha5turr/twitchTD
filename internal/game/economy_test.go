package game

import (
	"strings"
	"testing"
)

func lastReply(g *Game) string {
	var r string
	for _, e := range g.TakeEvents() {
		if e.Kind == "reply" {
			r = e.Text
		}
	}
	return r
}

func TestDonationLinking(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{Login: "vasya_tank", User: "Вася_Танк", Text: "!link"})
	reply := lastReply(g)
	m := linkCode.FindStringSubmatch(reply)
	if m == nil {
		t.Fatalf("no code in reply %q", reply)
	}
	c := g.Chatters["vasya_tank"]
	c.Gold = 0

	// First donation under a free-text name, with the code in the message.
	g.Donate(Donation{Name: "Василий", Message: "за таверну! TD-" + m[1], Amount: 50, Currency: "RUB"})
	if c.Gold != 500 {
		t.Fatalf("gold %d, want 500 (50 RUB × 10)", c.Gold)
	}
	if c.Name != "Вася_Танк" {
		t.Fatalf("display name lost: %q", c.Name)
	}
	// Later donations under the same name need no code.
	g.Donate(Donation{Name: "василий", Message: "ещё", Amount: 10, Currency: "RUB"})
	if c.Gold != 600 {
		t.Fatalf("linked name should pay the wallet: gold %d", c.Gold)
	}
	// The cap holds; the rest goes to the treasury.
	g.Donate(Donation{Name: "Василий", Amount: 100, Currency: "RUB"})
	if c.Gold != g.Cfg.WalletCap || g.Treasury != 600 {
		t.Fatalf("gold %d treasury %d", c.Gold, g.Treasury)
	}
}

func TestDonationUnmatchedGoesToTreasury(t *testing.T) {
	g, _ := newTestGame(t)
	g.Donate(Donation{Name: "Аноним", Amount: 2, Currency: "USD"})
	if g.Treasury != 1800 {
		t.Fatalf("treasury %d, want 1800 (2 USD × 90 × 10)", g.Treasury)
	}
	g.HandleChat(ChatMessage{User: "streamer", Text: "!tdtreasury Петя 300", Mod: true, Broadcaster: true})
	if g.Chatters["петя"] == nil || g.Treasury != 1500 {
		t.Fatalf("the streamer hands out treasury gold: treasury %d", g.Treasury)
	}
}

func TestDonationMatchesNick(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{Login: "grim77", User: "Grim77", Text: "hi"})
	before := g.Chatters["grim77"].Gold
	g.Donate(Donation{Name: "GRIM77", Amount: 20, Currency: "RUB"})
	if g.Chatters["grim77"].Gold != before+200 {
		t.Fatal("a donation under the chatter's nick reaches the wallet")
	}
}

func TestRewards(t *testing.T) {
	g, _ := newTestGame(t)
	g.Reward("lurker", "Lurker", "htd: мешок золота")
	if c := g.Chatters["lurker"]; c == nil || c.Gold != g.Cfg.StartGold+300 {
		t.Fatal("gold reward creates the wallet and pays it")
	}
	full := g.Chatters["lurker"]
	full.Gold = g.Cfg.WalletCap
	treasury := g.Treasury
	g.Reward("lurker", "Lurker", "HTD: Мешок золота")
	if full.Gold != g.Cfg.WalletCap+300 || g.Treasury != treasury {
		t.Fatalf("a bag of gold reaches the buyer past the wallet cap: %d, treasury +%d", full.Gold, g.Treasury-treasury)
	}
	g.Walls = 100
	g.Reward("lurker", "Lurker", "Починить стены")
	if g.Walls != 300 {
		t.Fatalf("walls %.0f", g.Walls)
	}
	e := g.spawnEnemy("skeleton", 100, 1)
	boss := g.spawnEnemy("lich", 1000, 2)
	g.Reward("lurker", "Lurker", "Дождь стрел")
	if e.HP != 50 || boss.HP != 900 {
		t.Fatalf("strike: skeleton %.0f (want 50), boss %.0f (want 900)", e.HP, boss.HP)
	}
	g.Reward("lurker", "Lurker", "Чужая награда")
	for _, ev := range g.TakeEvents() {
		if ev.Kind == "reward" && strings.Contains(ev.Text, "Чужая") {
			t.Fatal("unknown rewards are ignored")
		}
	}
}

func TestStreamerRewardsReachTheirWallet(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{Login: "ha5turr", User: "ha5turr", Text: "привет", Broadcaster: true})
	s := g.Chatters[StreamerName]
	s.Gold = 152
	g.Reward("ha5turr", "ha5turr", "HTD: Мешок золота")
	g.Bits("ha5turr", "ha5turr", 10)
	if g.Chatters["ha5turr"] != nil || s.Gold != 152+300+80 {
		t.Fatalf("the streamer's own redemption and cheer pay the wallet they play with: %d", s.Gold)
	}
	g.Restore(g.Export())
	g.Reward("ha5turr", "ha5turr", "HTD: Мешок золота")
	if g.Chatters["ha5turr"] != nil || g.Chatters[StreamerName].Gold != 152+600+80 {
		t.Fatal("the streamer's login survives a restart")
	}
}

func TestBitsAndSubs(t *testing.T) {
	g, _ := newTestGame(t)
	g.Bits("a", "A", 50)
	g.GiftSub("b", "B", "c", "C")
	if g.Chatters["a"].Gold != g.Cfg.StartGold+400 {
		t.Fatalf("bits gold %d", g.Chatters["a"].Gold)
	}
	if g.Chatters["b"].Gold != g.Cfg.StartGold+g.Cfg.SubGold || g.Chatters["c"].Gold != g.Cfg.StartGold+g.Cfg.GiftRecipientGold {
		t.Fatal("gift sub pays both sides")
	}
}

func TestRewardPerUserCap(t *testing.T) {
	g, _ := newTestGame(t)
	g.Cfg.Rewards = []RewardSpec{{Title: "HTD: Мешок золота", Action: "gold", Amount: 300, PerUser: 2}}
	for i := 0; i < 2; i++ {
		if g.Reward("vasya", "Vasya", "HTD: Мешок золота") {
			t.Fatalf("redemption %d is within the cap", i+1)
		}
	}
	gold := g.Chatters["vasya"].Gold
	if !g.Reward("vasya", "Vasya", "HTD: Мешок золота") || g.Chatters["vasya"].Gold != gold {
		t.Fatal("the third one in a game is refused and pays nothing")
	}
	if r := lastReply(g); !strings.Contains(r, "не больше 2") {
		t.Fatalf("the viewer is told why: %q", r)
	}
	if g.Reward("petya", "Petya", "HTD: Мешок золота") {
		t.Fatal("the cap is per viewer")
	}
	g.Restore(g.Export())
	if !g.Reward("vasya", "Vasya", "HTD: Мешок золота") {
		t.Fatal("the count survives a restart")
	}
	g.NewGame("")
	if g.Reward("vasya", "Vasya", "HTD: Мешок золота") {
		t.Fatal("a new game resets the cap")
	}
}

func TestKeepTreasury(t *testing.T) {
	g, _ := newTestGame(t)
	g.Treasury = 500
	g.NewGame("")
	if g.Treasury != 500 {
		t.Fatal("by default the treasury outlives the game")
	}
	g.Cfg.KeepTreasury = false
	g.NewGame("")
	if g.Treasury != 0 {
		t.Fatal("with keepTreasury off a new game empties it")
	}
}
