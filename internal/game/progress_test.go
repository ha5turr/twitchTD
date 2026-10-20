package game

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func replies(g *Game) []string {
	var out []string
	for _, e := range g.TakeEvents() {
		if e.Kind == "reply" {
			out = append(out, e.Text)
		}
	}
	return out
}

func TestHelpCooldown(t *testing.T) {
	g, now := newTestGame(t)
	say := func(text string) { g.HandleChat(ChatMessage{User: "anna", Text: text}) }
	say("!помощь")
	if r := replies(g); len(r) != 1 || !strings.Contains(r[0], "!b C4") {
		t.Fatalf("help answers with the commands: %q", r)
	}
	say("!help")
	if r := replies(g); len(r) != 0 {
		t.Fatal("a minute of silence after an answer")
	}
	say("!помощь таверна")
	if r := replies(g); len(r) != 1 || !strings.Contains(r[0], "!кости") {
		t.Fatalf("each topic has its own cooldown: %q", r)
	}
	say("!помощь клоун")
	if r := replies(g); len(r) != 0 {
		t.Fatal("the clown topic falls back to the general one while he is away, which is still cooling down")
	}
	*now = now.Add(61 * time.Second)
	say("!помощь")
	if r := replies(g); len(r) != 1 || strings.Contains(strings.ToLower(r[0]), "клоун") {
		t.Fatalf("after a minute help answers again, keeping the secret: %q", r)
	}
}

func TestSoulsAtGameOver(t *testing.T) {
	g, _ := newTestGame(t)
	for _, u := range []string{"anna", "boris", "vera"} {
		g.HandleChat(ChatMessage{User: u, Text: "!play"})
	}
	g.Chatters["anna"].Gold, g.Chatters["anna"].GameDamage, g.Chatters["anna"].GameKills = 400, 2000, 30
	g.Chatters["boris"].Gold = 1000 // chatted a lot, never fought
	g.Chatters["vera"].GameDamage = 1e7
	g.Walls = 0
	g.Tick(0.05)
	anna := g.Chatters["anna"].Souls
	want := int(400*g.Cfg.SoulsGoldPct + 2000*g.Cfg.SoulsPerDamage + 30*g.Cfg.SoulsPerKill)
	if anna != want {
		t.Fatalf("anna earns gold share plus her fight: %d, want %d", anna, want)
	}
	if g.Chatters["boris"].Souls != 0 {
		t.Fatal("chatting alone earns no souls")
	}
	if g.Chatters["vera"].Souls != g.Cfg.SoulsGameCap {
		t.Fatalf("one game gives at most the cap: %d", g.Chatters["vera"].Souls)
	}
}

func TestAchievementsPayOnce(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	e := enemyAt(g, "skeleton", 3, 10)
	g.damage(e, 100, "anna", true)
	c := g.Chatters["anna"]
	fb, _ := achievement("first_blood")
	if !c.Ach["first_blood"] || c.Souls != fb.Souls {
		t.Fatalf("first kill gives its achievement: %+v souls %d", c.Ach, c.Souls)
	}
	g.damage(enemyAt(g, "skeleton", 3, 10), 100, "anna", true)
	if c.Souls != fb.Souls {
		t.Fatal("an achievement pays only once")
	}
	for _, a := range Achievements {
		if a.Souls < 1 || a.Souls > 10 {
			t.Fatalf("achievements give a few souls: %s %d", a.ID, a.Souls)
		}
	}
}

func TestMemeRequests(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	c := g.Chatters["anna"]
	c.Souls = 100
	g.HandleChat(ChatMessage{User: "anna", Text: "!мемы 20"})
	if len(g.Memes) != 0 {
		t.Fatal("not enough souls")
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!мемы 5"})
	if len(g.Memes) != 1 || c.Souls != 100-5*g.Cfg.SoulsPerMemecoin {
		t.Fatalf("a request takes the souls: %d", c.Souls)
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!мемы 1"})
	if len(g.Memes) != 1 {
		t.Fatal("one open request per chatter")
	}
	id := g.Memes[0].ID
	g.HandleChat(ChatMessage{User: "streamer", Mod: true, Broadcaster: true, Text: "!tdmeme reject " + strconv.Itoa(id)})
	if len(g.Memes) != 0 || c.Souls != 100 {
		t.Fatal("a rejected request gives the souls back")
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!мемы 2"})
	g.HandleChat(ChatMessage{User: "mod", Mod: true, Text: "!tdmeme done " + strconv.Itoa(g.Memes[0].ID)})
	if len(g.Memes) != 0 || c.Souls != 100-2*g.Cfg.SoulsPerMemecoin {
		t.Fatal("a done request keeps the souls spent")
	}
}

func TestProgressSurvivesSave(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	c := g.Chatters["anna"]
	c.Souls = 42
	g.unlock("anna", "drunkard")
	c.count("games", 3)
	c.Souls = 42
	g.HandleChat(ChatMessage{User: "anna", Text: "!мемы 1"})
	s := g.Export()
	g2, _ := newTestGame(t)
	g2.LoadChatters(s.Chatters)
	g2.LoadEconomy(s)
	c2 := g2.Chatters["anna"]
	if c2.Souls != 32 || !c2.Ach["drunkard"] || c2.Stats["games"] != 3 || len(g2.Memes) != 1 {
		t.Fatalf("souls, achievements, counters and requests are kept: %+v memes %d", c2, len(g2.Memes))
	}
}

func TestRanksFollowLifetimeSouls(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	c := g.Chatters["anna"]
	if c.rank() != 0 || Ranks[0].Name != "Неофит" {
		t.Fatal("everyone starts as Неофит")
	}
	g.TakeEvents()
	g.giveSouls("anna", c, Ranks[2].Souls)
	ranked := false
	for _, e := range g.TakeEvents() {
		ranked = ranked || e.Kind == "rank" && e.Text == Ranks[2].Name
	}
	if c.rank() != 2 || !ranked {
		t.Fatalf("souls raise the rank and announce it: rank %d", c.rank())
	}
	c.Souls = 0 // spent on memecoins
	if c.rank() != 2 {
		t.Fatal("spending souls never lowers the rank")
	}
	g.HandleChat(ChatMessage{User: "mod", Mod: true, Text: "!tdsouls anna 10000"})
	if c.rank() != len(Ranks)-1 {
		t.Fatal("souls granted by hand count too")
	}
	var old Chatter
	old.setProgress(Progress{Souls: 600}) // saved before ranks existed
	if old.rank() != 3 {
		t.Fatalf("old balances count as earned: rank %d", old.rank())
	}
}
