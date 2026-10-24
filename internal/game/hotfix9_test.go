package game

import (
	"strings"
	"testing"
)

func TestTowerLimitGrowsWithBosses(t *testing.T) {
	g, _ := newTestGame(t)
	for _, u := range []string{"anna", "boris", "vera"} {
		g.HandleChat(ChatMessage{User: u, Text: "!play"})
	}
	g.Cfg.BossEvery = 5
	if got := g.TowerLimit(); got != g.Cfg.TowerLimit {
		t.Fatalf("the game starts at the base limit: %d", got)
	}
	g.Wave, g.Phase = 5, PhaseWave
	if got := g.TowerLimit(); got != g.Cfg.TowerLimit {
		t.Fatalf("a boss wave in progress is not held yet: %d", got)
	}
	g.endWave()
	if got := g.TowerLimit(); got != g.Cfg.TowerLimit+1 {
		t.Fatalf("a boss held adds one: %d", got)
	}
	g.Wave = 100
	if got := g.TowerLimit(); got != g.Cfg.TowerLimitMax {
		t.Fatalf("the limit stops at the cap: %d", got)
	}
	g2, _ := newTestGame(t)
	g2.HandleChat(ChatMessage{User: "solo", Text: "!play"})
	if got := g2.TowerLimit(); got != g2.Cfg.TowerLimit+g2.Cfg.SmallTeamBonus {
		t.Fatalf("a party of one or two builds more: %d", got)
	}
}

func TestSiegeRage(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	g.Cfg.EnemyRegen = 0.05
	wall := placeAs(g, "anna", "wall", g.Map.Path[5])
	wall.HP, wall.MaxHP = 1e9, 1e9
	e := enemyAt(g, "skeleton", 5, 1000)
	run(g, 1)
	if e.blocked != wall {
		t.Fatal("the skeleton is held by the barricade")
	}
	e.HP = 500
	run(g, 1)
	if e.HP <= 500 || e.rage {
		t.Fatalf("before the rage it heals: hp %.0f", e.HP)
	}
	run(g, g.Cfg.SiegeRageSec)
	if !e.rage {
		t.Fatal("held long enough, it rages")
	}
	e.HP = 500
	before := wall.HP
	run(g, 1)
	if e.HP != 500 {
		t.Fatalf("a raging enemy does not heal: hp %.0f", e.HP)
	}
	calm := g.Cfg.Enemies["skeleton"].WallDPS
	if hit := before - wall.HP; hit <= calm*1.05 {
		t.Fatalf("and hits the barricade harder: %.1f vs %.1f", hit, calm)
	}
	g.removeTower(wall)
	run(g, 0.5)
	if e.rage {
		t.Fatal("walking on cools it down")
	}
}

func TestBarricadeHoldGoldCap(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	c := g.Chatters["anna"]
	c.Gold = 0
	wall := placeAs(g, "anna", "wall", g.Map.Path[3])
	for i := 0; i < 200; i++ {
		g.payHold(wall, 1)
	}
	if c.Gold != int(g.Cfg.WallHoldMax) {
		t.Fatalf("a barricade pays at most wallHoldMax a wave: %d", c.Gold)
	}
	g.endWave()
	gold := c.Gold
	g.StartWave()
	g.payHold(wall, 4)
	if c.Gold <= gold {
		t.Fatal("a new wave pays again")
	}
}

func TestBossesShakeOffNets(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	spec := g.Cfg.Towers["arrow"]
	boss := g.spawnBoss("lich", 1e9)
	boss.Dist = g.cellDist[6]
	boss.Pos = g.routePos(boss.Dist)
	boss.ability = 1e9
	sk := enemyAt(g, "skeleton", 6, 1e9)
	sk.Pos = boss.Pos
	tw := &Tower{Cell: g.Map.Path[6], Level: 1, Kind: "arrow"}
	g.tickNet(tw, spec, 1)
	if boss.netLeft <= 0 || sk.netLeft <= 0 {
		t.Fatal("the first net catches the boss and those around")
	}
	boss.netLeft, sk.netLeft = 0, 0
	tw.netCD = 0
	g.tickNet(tw, spec, 1)
	if boss.netLeft > 0 || sk.netLeft <= 0 {
		t.Fatal("right after, the boss shakes nets off; others do not")
	}
	boss.netImmune = 0
	tw.netCD = 0
	g.tickNet(tw, spec, 1)
	if boss.netLeft <= 0 {
		t.Fatal("once the immunity is over a net holds him again")
	}
}

// veterans sets up three players with 2000, 2500 and 3000 gold in buildings.
func veterans(t *testing.T) *Game {
	g, _ := newTestGame(t)
	for i, u := range []string{"anna", "boris", "vera"} {
		g.HandleChat(ChatMessage{User: u, Text: "!play"})
		placeAs(g, u, "arrow", freeCell(t, g)).Invested = 2000 + 500*i
	}
	return g
}

func TestLateNewcomerGetsAGrant(t *testing.T) {
	g := veterans(t)
	g.Wave = g.Cfg.GrantFromWave
	g.HandleChat(ChatMessage{User: "newbie", Text: "!play"})
	c := g.Chatters["newbie"]
	if c.Grant != 1250 {
		t.Fatalf("half the median of 2500: %d", c.Grant)
	}
	gold := c.Gold
	p := freeCell(t, g)
	g.HandleChat(ChatMessage{User: "newbie", Text: "!b " + p.Name() + " fire"})
	tw := g.Towers[p]
	if tw == nil || c.Gold != gold || c.Grant != 1250-g.Cfg.Towers["fire"].Cost || tw.Granted != tw.Invested {
		t.Fatal("the grant pays first and the wallet stays whole")
	}
	tw.Built = true
	treasury := g.Treasury
	g.HandleChat(ChatMessage{User: "newbie", Text: "!sell " + p.Name()})
	if c.Gold != gold || g.Treasury <= treasury {
		t.Fatal("what a grant bought sells back into the treasury, not the wallet")
	}
	g.HandleChat(ChatMessage{User: "newbie", Text: "!leave"})
	g.HandleChat(ChatMessage{User: "newbie", Text: "!play"})
	if c.Grant != 0 {
		t.Fatalf("one grant per game, and leaving burns it: %d", c.Grant)
	}
	g.HandleChat(ChatMessage{User: "early", Text: "!play"})
	g2 := veterans(t)
	g2.HandleChat(ChatMessage{User: "early", Text: "!play"})
	if g2.Chatters["early"].Grant != 0 {
		t.Fatal("no grant before grantFromWave")
	}
}

func TestDifficultyFollowsInvestment(t *testing.T) {
	g := veterans(t)
	if got := g.difficultyChatters(); got != 3 {
		t.Fatalf("three builders count in full: %.2f", got)
	}
	g.HandleChat(ChatMessage{User: "newbie", Text: "!play"})
	if got := g.difficultyChatters(); got != 3 {
		t.Fatalf("a newcomer with nothing built adds nothing: %.2f", got)
	}
	placeAs(g, "newbie", "arrow", freeCell(t, g)).Invested = 500
	if got := g.difficultyChatters(); got < 3.39 || got > 3.41 {
		t.Fatalf("500 of the 1250 needed counts as 0.4 of a player: %.2f", got)
	}
	g.Towers[freeCell(t, g)] = &Tower{Kind: "magic", Owner: "newbie", Invested: 2000, Built: true}
	if got := g.difficultyChatters(); got != 4 {
		t.Fatalf("enough built, a full player: %.2f", got)
	}
}

func TestPassingOutDrunk(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	c := g.Chatters["anna"]
	c.Gold = 1000
	for c.out <= 0 && c.Drinks < 20 {
		g.HandleChat(ChatMessage{User: "anna", Text: "!drink зелье"})
	}
	if c.out <= 0 || c.Drunk < g.Cfg.PassOutMugs || !c.Ach["zabuldyga"] {
		t.Fatalf("enough drink knocks him out and earns the achievement: drunk %.1f", c.Drunk)
	}
	gold := c.Gold
	g.TakeEvents()
	g.HandleChat(ChatMessage{User: "anna", Text: "!b " + freeCell(t, g).Name()})
	if c.Gold != gold || g.towersOf("anna") != 0 || !strings.Contains(lastReply(g), "храпит") {
		t.Fatal("a passed out mercenary ignores commands")
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!me"})
	if r := lastReply(g); r != "" {
		t.Fatalf("and does not flood the chat with snoring: %q", r)
	}
	run(g, g.Cfg.PassOutSec+1)
	g.HandleChat(ChatMessage{User: "anna", Text: "!b " + freeCell(t, g).Name()})
	if g.towersOf("anna") != 1 {
		t.Fatalf("after sleeping it off he works again: %q", lastReply(g))
	}
}

func TestShrinesShareInKills(t *testing.T) {
	g, _ := newTestGame(t)
	for _, u := range []string{"anna", "boris", "vera"} {
		g.HandleChat(ChatMessage{User: u, Text: "!play"})
		g.Chatters[u].Class = "ranger" // no class bonus on shrines
		g.Chatters[u].Gold = 0
	}
	placeAs(g, "boris", "shrine", freeCell(t, g))
	placeAs(g, "vera", "shrine", freeCell(t, g))
	placeAs(g, "anna", "shrine", freeCell(t, g))
	for i := 0; i < 100; i++ {
		g.damage(enemyAt(g, "skeleton", 5, 1), 10, "anna", true)
	}
	bounty := float64(g.Cfg.Enemies["skeleton"].Bounty) * 100
	// Three +15% shrines: each made 15/145 of the damage, half of it is paid.
	want := bounty * g.Cfg.ShrineAssistPct * 0.15 / 1.45
	for _, u := range []string{"boris", "vera"} {
		if got := float64(g.Chatters[u].Gold); got < want-1 || got > want+1 {
			t.Fatalf("%s's shrine earns its share of anna's kills: %v, want %.1f", u, got, want)
		}
	}
	if g.Chatters["anna"].Gold != int(bounty) {
		t.Fatalf("no share from one's own kills, the killer keeps the full bounty: %d", g.Chatters["anna"].Gold)
	}
	g.endWave()
	found := false
	for _, e := range g.TakeEvents() {
		found = found || e.Kind == "shrineGold" && e.User == g.Chatters["boris"].Name
	}
	if !found {
		t.Fatal("the wave end tells what the shrines earned")
	}
}

func TestRageAtTheTavernDoor(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	g.Cfg.GateDPS = 0 // keep the guards from killing it
	g.Cfg.EnemyRegen = 0.05
	e := g.spawnEnemy("skeleton", 1000, g.total)
	e.AtGate = true
	walls := g.Walls
	run(g, 1)
	calm := walls - g.Walls
	e.HP = 500
	run(g, 1)
	if e.rage || e.HP <= 500 {
		t.Fatal("before the rage it heals at the door")
	}
	run(g, g.Cfg.SiegeRageSec)
	if !e.rage {
		t.Fatal("held at the door long enough, it rages")
	}
	e.HP = 500
	before := g.Walls
	run(g, 1)
	if e.HP != 500 || before-g.Walls <= calm*1.05 {
		t.Fatalf("a raging enemy at the door heals no more and hits harder: hp %.0f, hit %.1f vs %.1f", e.HP, before-g.Walls, calm)
	}
}

func TestUpgradeToMax(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	c := g.Chatters["anna"]
	tw := placeAs(g, "anna", "arrow", freeCell(t, g))
	// Levels 2, 3 and 4 cost 120 + 180 + 240 = 540; level 5 would need 300 more.
	c.Gold = 590
	g.HandleChat(ChatMessage{User: "anna", Text: "!up " + tw.Cell.Name() + " max"})
	if c.Gold != 50 || tw.PendingUp != 3 || tw.PendingCost != 540 {
		t.Fatalf("max pays for as many levels as the wallet allows: gold %d, levels %d, cost %d", c.Gold, tw.PendingUp, tw.PendingCost)
	}
	for i := 0; i < 600 && tw.PendingCost > 0; i++ {
		run(g, 0.1)
	}
	if tw.Level != 4 {
		t.Fatalf("the mercenary raises it three levels in one go: level %d", tw.Level)
	}
	c.Gold = 100
	g.HandleChat(ChatMessage{User: "anna", Text: "!up " + tw.Cell.Name() + " макс"})
	if r := lastReply(g); tw.PendingUp != 0 || c.Gold != 100 || !strings.Contains(r, "нужно 300") {
		t.Fatalf("not enough for one level: nothing is spent and the price is told: %q", r)
	}
}

func TestShrineLimit(t *testing.T) {
	g, _ := newTestGame(t)
	g.Cfg.TowerLimit, g.Cfg.TowerLimitMax, g.Cfg.MaxOrders = 10, 10, 10
	for _, u := range []string{"anna", "boris"} {
		g.HandleChat(ChatMessage{User: u, Text: "!play"})
		g.Chatters[u].Gold = 1000
	}
	g.Chatters["anna"].Class = "ranger"
	g.Chatters["boris"].Class = "cleric"
	build := func(u string) bool {
		p := freeCell(t, g)
		g.HandleChat(ChatMessage{User: u, Text: "!b " + p.Name() + " shrine"})
		return g.Towers[p] != nil
	}
	for i := 0; i < g.Cfg.ShrineLimit; i++ {
		if !build("anna") {
			t.Fatalf("shrine %d is within the limit", i+1)
		}
	}
	if build("anna") {
		t.Fatal("a third shrine is refused")
	}
	for i := 0; i < g.Cfg.ShrineLimitCleric; i++ {
		if !build("boris") {
			t.Fatalf("a cleric builds shrine %d", i+1)
		}
	}
	if build("boris") {
		t.Fatal("even a cleric stops at his limit")
	}
}

func TestClassChangeSurvivesRestart(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	g.Chatters["anna"].Class = "ranger"
	g.HandleChat(ChatMessage{User: "anna", Text: "!class маг"})
	save := g.Export()

	g2, _ := newTestGame(t)
	g2.LoadChatters(save.Chatters)
	g2.Restore(save)
	g2.HandleChat(ChatMessage{User: "anna", Text: "!класс жрец"})
	if c := g2.Chatters["anna"]; c.Class != "mage" {
		t.Fatalf("a restart mid-game does not allow a second class change: %s, %q", c.Class, lastReply(g2))
	}
	g2.NewGame("")
	g2.HandleChat(ChatMessage{User: "anna", Text: "!класс жрец"})
	if g2.Chatters["anna"].Class != "cleric" {
		t.Fatal("a new game allows a change again")
	}
}
