package game

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// jailParty seats anna, boris and vera, and gives boris a building.
func jailParty(t *testing.T) *Game {
	g, _ := newTestGame(t)
	for _, n := range []string{"anna", "boris", "vera"} {
		g.HandleChat(ChatMessage{User: n, Text: "!play"})
	}
	return g
}

func lastEvent(g *Game, kind string) *Event {
	for i := len(g.events) - 1; i >= 0; i-- {
		if g.events[i].Kind == kind {
			return &g.events[i]
		}
	}
	return nil
}

func TestSayIsShort(t *testing.T) {
	g := jailParty(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!s " + strings.Repeat("а", 80)})
	ev := lastEvent(g, "say")
	if ev == nil || utf8.RuneCountInString(ev.Text) != sayMax || !strings.HasSuffix(ev.Text, "…") {
		t.Fatalf("a long line is cut to %d letters: %+v", sayMax, ev)
	}
	g.events = nil
	g.HandleChat(ChatMessage{User: "anna", Text: "!s ещё"})
	if lastEvent(g, "say") != nil {
		t.Fatal("and lines come at most every few seconds")
	}
}

func TestDanceCooldown(t *testing.T) {
	g := jailParty(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!dance"})
	if lastEvent(g, "dance") == nil {
		t.Fatal("!dance sends the mercenary up on the table")
	}
	g.events = nil
	g.HandleChat(ChatMessage{User: "anna", Text: "!танец"})
	if lastEvent(g, "dance") != nil || !strings.Contains(lastReply(g), "отдохни") {
		t.Fatal("and not again right away")
	}
}

func TestJailVote(t *testing.T) {
	g := jailParty(t)
	b := g.Chatters["boris"]
	b.Gold = 400
	placeAs(g, "boris", "arrow", freeCell(t, g)).Invested = 600
	g.HandleChat(ChatMessage{User: "anna", Text: "!тюрьма boris"})
	if g.jailVote == nil {
		t.Fatal("a vote starts")
	}
	g.HandleChat(ChatMessage{User: "boris", Text: "!против"})
	if len(g.jailVote.no) != 0 {
		t.Fatal("the accused has no vote")
	}
	g.HandleChat(ChatMessage{User: "vera", Text: "!за"})
	if g.jailVote != nil || !b.jailed() {
		t.Fatal("everyone voted for: boris is locked up at once")
	}
	if want := 150; b.ransom != want { // 15% of 400 gold and 600 in buildings
		t.Fatalf("the ransom is a share of what he owns: %d, want %d", b.ransom, want)
	}

	// Behind bars only !s and !выкуп work.
	gold := b.Gold
	g.HandleChat(ChatMessage{User: "boris", Text: "!b " + freeCell(t, g).Name() + " arrow"})
	if b.Gold != gold || !strings.Contains(lastReply(g), "за решёткой") {
		t.Fatal("an inmate cannot build")
	}
	g.HandleChat(ChatMessage{User: "boris", Text: "!s выпустите"})
	if lastEvent(g, "say") == nil {
		t.Fatal("but may speak")
	}
	if g.Workers["boris"] != nil {
		t.Fatal("no mercenary goes out")
	}
	for _, k := range g.tavernMates() {
		if k == "boris" {
			t.Fatal("an inmate's class skills stop")
		}
	}

	treasury := g.Treasury
	g.HandleChat(ChatMessage{User: "boris", Text: "!выкуп"})
	if b.jailed() || b.Gold != gold-150 || g.Treasury != treasury+150 {
		t.Fatal("the ransom frees him and goes to the treasury")
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!тюрьма boris"})
	if g.jailVote != nil {
		t.Fatal("the same player goes to jail at most once a game")
	}
}

func TestJailVoteFails(t *testing.T) {
	g := jailParty(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!jail boris"})
	g.HandleChat(ChatMessage{User: "vera", Text: "!против"})
	if g.jailVote != nil || g.Chatters["boris"].jailed() {
		t.Fatal("one for, one against: boris stays free")
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!jail boris"})
	if g.jailVote != nil {
		t.Fatal("and is safe from the next vote for a while")
	}

	g2 := jailParty(t)
	g2.HandleChat(ChatMessage{User: "anna", Text: "!jail boris"})
	run(g2, g2.Cfg.JailVoteSec+1)
	if g2.Chatters["boris"].jailed() {
		t.Fatal("the accuser alone is not enough")
	}
}

func TestJailTermEnds(t *testing.T) {
	g := jailParty(t)
	b := g.Chatters["boris"]
	g.lockUp("boris", b)
	run(g, g.Cfg.JailSec+1)
	if b.jailed() {
		t.Fatal("the term runs out")
	}
	g.lockUp("boris", b)
	g.HandleChat(ChatMessage{User: "streamer", Text: "!tdfree boris", Mod: true})
	if b.jailed() {
		t.Fatal("the streamer may pardon")
	}
}

func TestJailSurvivesRestart(t *testing.T) {
	g := jailParty(t)
	g.lockUp("boris", g.Chatters["boris"])
	g2, _ := newTestGame(t)
	g2.LoadChatters(g.Export().Chatters)
	g2.Restore(g.Export())
	if b := g2.Chatters["boris"]; b == nil || !b.jailed() || !b.jailedOnce {
		t.Fatal("a restart does not open the jail")
	}
}

func TestHelpFitsTwitch(t *testing.T) {
	g, _ := newTestGame(t)
	for _, topic := range []string{"", "build", "tavern", "souls", "clown"} {
		if n := utf8.RuneCountInString(g.helpText(topic)); n > 480 {
			t.Fatalf("help %q is %d letters, Twitch cuts at 500", topic, n)
		}
	}
}
