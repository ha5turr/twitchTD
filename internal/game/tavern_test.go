package game

import "testing"

func TestClassChangeOncePerGame(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "zoe", Text: "привет"})
	c := g.Chatters["zoe"]
	if _, ok := classByKey(c.Class); !ok {
		t.Fatalf("new chatter should get a class, got %q", c.Class)
	}
	target := "mage"
	if c.Class == "mage" {
		target = "ranger"
	}
	spec, _ := classByKey(target)
	g.HandleChat(ChatMessage{User: "zoe", Text: "!class " + spec.Aliases[0]})
	if c.Class != target {
		t.Fatalf("class %s, want %s", c.Class, target)
	}
	g.HandleChat(ChatMessage{User: "zoe", Text: "!class воитель"})
	if c.Class != target {
		t.Fatal("second change in one game must be refused")
	}
	g.NewGame("")
	g.HandleChat(ChatMessage{User: "zoe", Text: "!class воитель"})
	if c.Class != "warrior" {
		t.Fatal("a new game allows a new change")
	}
}

func TestClassBonusOnSignatureTower(t *testing.T) {
	g, _ := newTestGame(t)
	c := g.chatter("ann", "ann")
	c.Class = "ranger"
	if b := g.classBonus("ann", "arrow"); b != 1+g.Cfg.ClassBonus {
		t.Fatalf("ranger arrow bonus %v", b)
	}
	if b := g.classBonus("ann", "magic"); b != 1 {
		t.Fatalf("ranger magic bonus %v", b)
	}
}

func TestDrinking(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "max", Text: "!drink медовуха"})
	c := g.Chatters["max"]
	want := g.Cfg.StartGold + g.Cfg.ChatGold - g.Cfg.Drinks["mead"].Price
	if c.Gold != want || c.Drinks != 1 || c.Drunk != 2 {
		t.Fatalf("gold %d (want %d), drinks %d, drunk %v", c.Gold, want, c.Drinks, c.Drunk)
	}
	run(g, g.Cfg.DrunkDecaySec*3)
	if c.Drunk != 0 {
		t.Fatalf("should sober up, drunk %v", c.Drunk)
	}
	c.Gold = 0
	g.HandleChat(ChatMessage{User: "max", Text: "!drink зелье"})
	if c.Drinks != 1 {
		t.Fatal("no gold, no drink")
	}
}

func TestGameOverSummary(t *testing.T) {
	g, _ := newTestGame(t)
	g.chatter("a", "Альфа").GameDamage = 500
	g.chatter("b", "Бета").GameDamage = 900
	g.chatter("b", "Бета").GameDrinks = 3
	g.Walls = 0
	g.Tick(0.05)
	st := g.StateMessage()
	if st.Summary == nil || len(st.Summary.Damage) != 2 || st.Summary.Damage[0].Name != "Бета" {
		t.Fatalf("summary %+v", st.Summary)
	}
	if len(st.Summary.Drinks) != 1 || len(st.Summary.Kills) != 0 {
		t.Fatalf("only non-zero entries: %+v", st.Summary)
	}
}
