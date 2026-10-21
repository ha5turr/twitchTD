package game

import (
	"strings"
	"testing"
)

func diceGame(t *testing.T) (*Game, func(user, text string)) {
	g, _ := newTestGame(t)
	say := func(user, text string) { g.HandleChat(ChatMessage{User: user, Text: text}) }
	for _, u := range []string{"anna", "boris", "vera"} {
		say(u, "привет")
		g.Chatters[u].Gold = 500
		g.Chatters[u].Class = "bard" // no skill that kills and pays while the test waits
	}
	return g, say
}

func TestDiceAcceptAndRoll(t *testing.T) {
	g, say := diceGame(t)
	say("anna", "!кости 100")
	if g.Chatters["anna"].Gold != 400 || len(g.Dice) != 1 {
		t.Fatalf("stake should leave the wallet: gold %d, tables %d", g.Chatters["anna"].Gold, len(g.Dice))
	}
	say("anna", "!build "+freeCell(t, g).Name())
	if len(g.Towers) != 0 {
		t.Fatal("no building while at the dice table")
	}
	say("boris", "!кости принять @anna")
	if g.Chatters["boris"].Gold != 400 || g.Dice[0].Guest != "boris" {
		t.Fatal("the guest's stake is taken")
	}
	run(g, g.Cfg.DiceRollSec+0.5)
	if len(g.Dice) != 0 {
		t.Fatal("the dice should have been thrown")
	}
	a, b := g.Chatters["anna"].Gold, g.Chatters["boris"].Gold
	if !(a == 600 && b == 400 || a == 400 && b == 600) {
		t.Fatalf("winner takes the pot of 200: anna %d boris %d", a, b)
	}
	if r := lastReply(g); !strings.Contains(r, "забирает 200") {
		t.Fatalf("result reply %q", r)
	}
}

func TestDiceRaise(t *testing.T) {
	g, say := diceGame(t)
	say("anna", "!кости 100")
	say("boris", "!кости перебить 80")
	if g.Dice[0].Guest != "" {
		t.Fatal("a raise must be higher than the stake")
	}
	say("boris", "!кости перебить 250")
	if g.Dice[0].GuestStake != 250 || g.Chatters["boris"].Gold != 250 {
		t.Fatalf("raise: %+v gold %d", g.Dice[0], g.Chatters["boris"].Gold)
	}
	run(g, g.Cfg.DiceRollSec+0.5)
	if a, b := g.Chatters["anna"].Gold, g.Chatters["boris"].Gold; a+b != 1000 || (a != 750 && b != 750) {
		t.Fatalf("pot of 350 goes to one player: anna %d boris %d", a, b)
	}
}

func TestDiceCancelAndTimeout(t *testing.T) {
	g, say := diceGame(t)
	say("anna", "!кости 100")
	say("boris", "!кости отмена")
	if len(g.Dice) != 1 {
		t.Fatal("only the host can leave the table")
	}
	say("anna", "!кости отмена")
	if len(g.Dice) != 0 || g.Chatters["anna"].Gold != 500 {
		t.Fatal("cancel refunds the stake")
	}
	say("anna", "!кости 50")
	run(g, g.Cfg.DiceWaitSec+1)
	if len(g.Dice) != 0 || g.Chatters["anna"].Gold != 500 {
		t.Fatal("an unanswered table closes and refunds")
	}
	say("anna", "!кости 50")
	say("boris", "!кости принять")
	say("anna", "!кости отмена")
	if len(g.Dice) != 1 {
		t.Fatal("once someone sat down there is no way back")
	}
}

func TestDiceRules(t *testing.T) {
	g, say := diceGame(t)
	say("anna", "!кости 5")
	say("anna", "!кости 900")
	if len(g.Dice) != 0 {
		t.Fatal("stakes below the minimum or above the wallet are refused")
	}
	say("anna", "!кости 100")
	say("anna", "!кости принять")
	if g.Dice[0].Guest != "" {
		t.Fatal("nobody plays against themselves")
	}
	say("boris", "!кости 100")
	say("vera", "!кости 100")
	if len(g.Dice) != g.Cfg.DiceTables {
		t.Fatalf("tables %d, limit %d", len(g.Dice), g.Cfg.DiceTables)
	}
	// A save counts the stakes back, and a new game returns them.
	for _, sc := range g.Export().Chatters {
		if sc.Login == "anna" && sc.Gold != 500 {
			t.Fatalf("saved gold %d should include the stake", sc.Gold)
		}
	}
	g.Cfg.KeepGold = true
	g.NewGame("")
	if len(g.Dice) != 0 || g.Chatters["anna"].Gold != 500 || g.Chatters["boris"].Gold != 500 {
		t.Fatal("a new game ends dice games with refunds")
	}
}
