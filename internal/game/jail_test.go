package game

import (
	"strings"
	"testing"
	"time"
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
	if b.ransom != g.Cfg.JailRansomMin { // he built, the others nothing
		t.Fatalf("whoever built his share pays the least: %d", b.ransom)
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
	if b.jailed() || b.Gold != gold-b.ransom || g.Treasury != treasury+b.ransom {
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

// A save written before the session was kept: nobody known gets a grant.
func TestLegacySaveGivesNoGrants(t *testing.T) {
	g := jailParty(t)
	g.Wave = 8
	placeAs(g, "boris", "arrow", freeCell(t, g)).Built = true
	save := g.Export()
	save.Session = nil
	g2, _ := newTestGame(t)
	g2.LoadChatters(save.Chatters)
	g2.Restore(save)
	g2.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	if g2.Chatters["anna"].Grant != 0 {
		t.Fatal("an old save hands out no grants to players already known")
	}
}

func TestRansomByContribution(t *testing.T) {
	g := jailParty(t)
	for i, n := range []string{"anna", "vera"} {
		placeAs(g, n, "arrow", freeCell(t, g)).Invested = 800 + 200*i
	}
	g.HandleChat(ChatMessage{User: "gleb", Text: "!play"})
	placeAs(g, "boris", "arrow", freeCell(t, g)).Invested = 250 // half of a fair share (450)
	idle := g.ransomOf("gleb", g.Chatters["gleb"])
	half := g.ransomOf("boris", g.Chatters["boris"])
	full := g.ransomOf("anna", g.Chatters["anna"])
	if !(idle > half && half > full) || full != g.Cfg.JailRansomMin {
		t.Fatalf("the less one gave, the dearer: idle %d, half %d, full %d", idle, half, full)
	}
	if idle > g.Cfg.WalletCap/2 {
		t.Fatal("at most half a wallet")
	}
	b := g.Chatters["boris"]
	g.lockUp("boris", b)
	b.Gold, b.Grant = b.ransom-1, 1000
	g.HandleChat(ChatMessage{User: "boris", Text: "!выкуп"})
	if !b.jailed() || b.Grant != 1000 {
		t.Fatal("the grant does not pay a ransom")
	}
}

func TestKick(t *testing.T) {
	g, now := newTestGame(t)
	for _, n := range []string{"anna", "boris", "vera"} {
		g.HandleChat(ChatMessage{User: n, Text: "!play"})
	}
	b := g.Chatters["boris"]
	b.Gold = 300
	placeAs(g, "boris", "arrow", freeCell(t, g))
	g.lockUp("boris", b)
	g.HandleChat(ChatMessage{User: "anna", Text: "!tdkick boris"})
	if !b.Joined {
		t.Fatal("only a moderator kicks")
	}
	treasury := g.Treasury
	g.HandleChat(ChatMessage{User: "anna", Text: "!tdkick @boris", Mod: true})
	if b.Joined || b.jailed() || b.Gold != 0 || g.towersOf("boris") != 0 || g.Treasury <= treasury {
		t.Fatal("a kicked player leaves as with !leave, out of jail too")
	}
	g.HandleChat(ChatMessage{User: "boris", Text: "!play"})
	if b.Joined {
		t.Fatal("and may not come back for a while")
	}
	*now = now.Add(11 * time.Minute)
	g.HandleChat(ChatMessage{User: "boris", Text: "!play"})
	if !b.Joined {
		t.Fatal("until the time is up")
	}
}

func TestSayEmotes(t *testing.T) {
	g := jailParty(t)
	url := "https://cdn.7tv.app/emote/x/2x.webp"
	g.HandleChat(ChatMessage{User: "anna", Text: "!s привет catJAM", Emotes: map[string]string{"catJAM": url, "Kappa": url}})
	ev := lastEvent(g, "say")
	if ev == nil || ev.Text != "привет catJAM" || ev.Emotes["catJAM"] != url || len(ev.Emotes) != 1 {
		t.Fatalf("a line carries the emotes it uses: %+v", ev)
	}
}
