package game

import (
	"strings"
	"testing"
)

// kingGame starts a test game with the King in Yellow on the field.
func kingGame(t *testing.T, biome Biome) (*Game, *Enemy) {
	t.Helper()
	g, _ := newTestGame(t)
	g.NewGame(biome)
	g.Phase = PhaseWave
	g.Cfg.EnemyRegen = 0
	k := g.spawnBoss(kingKind, 100000)
	return g, k
}

// nearKing moves the king onto the path cell next to p.
func nearKing(g *Game, k *Enemy, p Pos) {
	for i, c := range g.Map.Path {
		if center(c).sub(center(p)).len() < 1.5 {
			k.Dist = g.cellDist[i]
			k.Pos = g.routePos(k.Dist)
			return
		}
	}
}

func TestKingIntroHoldsTheFinal(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	g.finalDue = true
	g.StartWave()
	if g.queue[0].kind != kingKind || g.introLeft <= 0 {
		t.Fatal("the final is the king's, after his scene")
	}
	var intro Event
	for _, e := range g.TakeEvents() {
		if e.Kind == "kingIntro" {
			intro = e
		}
	}
	if !strings.Contains(intro.Text, `"players":1`) {
		t.Fatalf("the scene knows the game: %q", intro.Text)
	}
	run(g, 1)
	if len(g.Enemies) > 0 {
		t.Fatal("nobody comes during the scene")
	}
	g.HandleChat(ChatMessage{User: "streamer", Text: "!tdskip", Mod: true, Broadcaster: true})
	run(g, 0.2)
	if len(g.Enemies) == 0 || g.Enemies[0].Kind != kingKind {
		t.Fatal("!tdskip lets him in")
	}
}

func TestKingIsHeldByNothing(t *testing.T) {
	g, k := kingGame(t, BiomeRuins)
	k.netLeft, k.stunLeft, k.slowMult, k.slowLeft = 5, 5, 0.1, 5
	d := k.Dist
	run(g, 0.5)
	if k.netLeft > 0 || k.stunLeft > 0 || k.slowMult != 1 || k.Dist <= d {
		t.Fatal("nets, stuns and slows do not hold him")
	}
}

func TestKingTornCloak(t *testing.T) {
	g, k := kingGame(t, BiomeRuins)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	g.damage(k, 1000, "anna", true)
	first := k.MaxHP - k.HP
	for i := 0; i < 20; i++ {
		g.damage(k, 1000, "anna", true)
	}
	before := k.HP
	g.damage(k, 1000, "anna", true)
	if late := before - k.HP; late >= first*0.8 {
		t.Fatalf("one player's damage fades: first hit %.0f, later %.0f", first, late)
	}
	before = k.HP
	g.damage(k, 1000, "boris", true)
	if fresh := before - k.HP; fresh < first*0.99 {
		t.Fatalf("another player hits in full: %.0f", fresh)
	}
}

func TestYellowSign(t *testing.T) {
	g, k := kingGame(t, BiomeRuins)
	for _, u := range []string{"anna", "boris"} {
		g.HandleChat(ChatMessage{User: u, Text: "!play"})
	}
	gun := placeAs(g, "anna", "arrow", freeCell(t, g))
	gun.heat = 500
	aura := placeAs(g, "boris", "shrine", freeCell(t, g))
	aura.Level = 4
	nearKing(g, k, gun.Cell)
	k.Pos = center(gun.Cell)
	g.placeSign(k)
	if g.signOn(gun) == nil || !strings.Contains(lastReply(g), "Жёлтый знак на "+gun.Cell.Name()) {
		t.Fatal("the first sign falls on the deadliest shooter, and the chat is told")
	}
	k.HP = 50000
	e := g.spawnEnemy("skeleton", 1e6, 0)
	gun.cooldown = 0
	g.shoot(gun, g.Cfg.Towers["arrow"], 0.05)
	if k.HP <= 50000 {
		t.Fatal("a marked shooter heals the king")
	}
	_ = e
	k.Pos = center(aura.Cell)
	g.placeSign(k)
	if g.signOn(aura) == nil || len(g.signs) != 2 {
		t.Fatal("the next one falls on a support, and the first stays")
	}
	run(g, kingSignDrainSec+0.1)
	if aura.Level != 3 {
		t.Fatalf("a marked support loses a level: %d", aura.Level)
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!рз " + aura.Cell.Name()})
	if g.signOn(aura) == nil {
		t.Fatal("one player is not enough when two play")
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!рз " + aura.Cell.Name()})
	g.HandleChat(ChatMessage{User: "boris", Text: "!развеять " + aura.Cell.Name()})
	if g.signOn(aura) != nil {
		t.Fatal("two players wipe it off")
	}
	run(g, g.Cfg.KingSignSec)
	if g.signOn(gun) != nil {
		t.Fatal("a sign fades by itself")
	}
	if aura.Level != 3 {
		t.Fatal("the lost level is gone for good")
	}
}

func TestYellowSignSolo(t *testing.T) {
	g, k := kingGame(t, BiomeRuins)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	gun := placeAs(g, "anna", "arrow", freeCell(t, g))
	k.Pos = center(gun.Cell)
	g.placeSign(k)
	g.HandleChat(ChatMessage{User: "anna", Text: "!рз " + gun.Cell.Name()})
	if g.signOn(gun) != nil {
		t.Fatal("playing alone, one player wipes it off")
	}
}

func TestKingPhases(t *testing.T) {
	g, k := kingGame(t, BiomeRuins)
	run(g, 0.1)
	if k.phase != 1 {
		t.Fatal("he starts in the first phase")
	}
	k.HP = k.MaxHP * 0.6
	run(g, 0.1)
	if k.phase != 2 || !strings.Contains(k.Title, "II") {
		t.Fatalf("at 70%% the second: %d %q", k.phase, k.Title)
	}
	faceless := 0
	for _, e := range g.Enemies {
		if e.Kind == facelessKind {
			faceless++
		}
	}
	if faceless != retinueN {
		t.Fatalf("his retinue comes: %d", faceless)
	}
	before := k.HP
	g.damage(k, 1000, "", true)
	if got := before - k.HP; got > 1000*(1-retinueCut)+0.01 {
		t.Fatalf("he takes less while it lives: %.0f", got)
	}
	k.HP = k.MaxHP * 0.35
	run(g, 0.1)
	if k.phase != 2 || g.maskLeft > 0 {
		t.Fatal("the retinue's phase lasts down to 30%")
	}
	k.HP = k.MaxHP * 0.25
	run(g, 0.1)
	if k.phase != 3 || g.maskLeft <= 0 {
		t.Fatal("then the third, and the mask comes off")
	}
	shrine := place(g, "shrine", freeCell(t, g))
	gun := place(g, "arrow", freeCell(t, g))
	_ = shrine
	if dmg, _ := g.auras(gun); dmg >= 1 {
		t.Fatalf("with the mask off auras weaken: %.2f", dmg)
	}
	count := func() (n int) {
		for _, e := range g.Enemies {
			if e.Kind == facelessKind {
				n++
			}
		}
		return n
	}
	was := count()
	k.retinueCD = 0
	run(g, retinueEvery+1)
	if count() > was {
		t.Fatal("no more faceless once the mask is off")
	}
}

func TestCassildasSong(t *testing.T) {
	g, k := kingGame(t, BiomeRuins)
	k.HP = k.MaxHP * 0.6
	run(g, songEvery/2+0.2)
	if g.songLeft <= 0 {
		t.Fatal("the song plays in the second phase")
	}
	fails := 0
	for i := 0; i < 200; i++ {
		if g.songFails() {
			fails++
		}
	}
	if fails < 60 || fails > 140 {
		t.Fatalf("about half the skills break: %d of 200", fails)
	}
}

func TestDoNotSayHisName(t *testing.T) {
	g, k := kingGame(t, BiomeRuins)
	k.HP = k.MaxHP / 2
	g.HandleChat(ChatMessage{User: "anna", Text: "это же hasturyan TD"})
	if k.HP != k.MaxHP/2 {
		t.Fatal("the channel's name does not count")
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "Хастур, Хастур!"})
	if k.HP <= k.MaxHP/2 || k.nameHaste != kingNameHaste || !strings.Contains(lastReply(g), "Не произносите") {
		t.Fatal("his name heals him, speeds him up, and the bot warns")
	}
	hp := k.HP
	g.HandleChat(ChatMessage{User: "anna", Text: "хастура"})
	if k.HP != hp {
		t.Fatal("one player counts once in a while")
	}
	g.HandleChat(ChatMessage{User: "boris", Text: "хастуру"})
	if k.HP <= hp {
		t.Fatal("another player counts")
	}
}

func TestKingBiomePowers(t *testing.T) {
	t.Run("ruins", func(t *testing.T) {
		g, k := kingGame(t, BiomeRuins)
		a := place(g, "arrow", freeCell(t, g))
		a.Level = 3
		k.Pos = center(a.Cell)
		run(g, 0.1)
		if g.active(a) {
			t.Fatal("stone sleep takes his best buildings near him")
		}
	})
	t.Run("dungeon", func(t *testing.T) {
		g, k := kingGame(t, BiomeDungeon)
		if g.cramped(k) != 2 {
			t.Fatal("in the dungeon he breaks barricades twice as fast")
		}
	})
	t.Run("ice", func(t *testing.T) {
		g, k := kingGame(t, BiomeIce)
		trap := place(g, "trap", g.Map.Path[3])
		g.springTrap(k, trap, g.Cfg.Enemies[kingKind])
		if g.active(trap) {
			t.Fatal("a trap that tries to hold him freezes")
		}
	})
	t.Run("swamp", func(t *testing.T) {
		g, k := kingGame(t, BiomeSwamp)
		g.ignite(k, 50, 3, "")
		if k.burnLeft > 0 {
			t.Fatal("fire does not catch on him")
		}
		fire := place(g, "fire", freeCell(t, g))
		g.kingHit(k, &Projectile{From: fire.ID})
		if g.active(fire) {
			t.Fatal("a brazier that hits him goes out")
		}
	})
	t.Run("river", func(t *testing.T) {
		g, k := kingGame(t, BiomeRiver)
		boom := place(g, "wall", g.Map.Path[6])
		hp := boom.HP
		k.Pos = center(boom.Cell)
		g.kingBiome(k, 0.05)
		if boom.HP != hp/2 {
			t.Fatal("booms near him tear")
		}
		g.kingBiome(k, 0.05)
		if boom.HP != hp/2 {
			t.Fatal("once each")
		}
	})
	t.Run("crypt", func(t *testing.T) {
		g, k := kingGame(t, BiomeCrypt)
		if !g.tearsLink(k, 7) || !g.brokenLinks[7] {
			t.Fatal("he tears soul links instead of being hurt")
		}
	})
}

func TestKingSpotsTheBestPlayers(t *testing.T) {
	g, _ := newTestGame(t)
	for _, u := range []string{"anna", "boris"} {
		g.HandleChat(ChatMessage{User: u, Text: "!play"})
	}
	g.Chatters["anna"].GameDamage = 5000
	g.Chatters["boris"].GameDrinks = 9
	g.Chatters["boris"].GameDances = 2 // too few to mention
	g.Chatters["anna"].GameRepairs = 7
	got := map[string]string{}
	for _, s := range g.kingSpots() {
		got[s.Kind] = s.Name
	}
	if got["damage"] != "anna" || got["drinks"] != "boris" || got["repairs"] != "anna" || got["dances"] != "" {
		t.Fatalf("the king names the best at each, when it is worth it: %v", got)
	}
	g.HandleChat(ChatMessage{User: "anna", Text: "!dance"})
	if g.Chatters["anna"].GameDances != 1 {
		t.Fatal("dances are counted")
	}
}

func TestKingSceneDoneLetsHimIn(t *testing.T) {
	g, _ := newTestGame(t)
	g.finalDue = true
	g.StartWave()
	g.KingSceneDone()
	run(g, 0.2)
	if len(g.Enemies) == 0 || g.Enemies[0].Kind != kingKind {
		t.Fatal("the overlay's end of the scene lets the king in")
	}
}

func TestKingCloakBreathes(t *testing.T) {
	g, k := kingGame(t, BiomeRuins)
	run(g, 0.1)
	if k.open {
		t.Fatal("he comes in with his cloak whole")
	}
	run(g, g.Cfg.KingGuardSec)
	if !k.open {
		t.Fatal("then it opens")
	}
	k.netLeft, k.slowMult, k.slowLeft = 2, 0.3, 2
	d := k.Dist
	run(g, 0.5)
	if k.netLeft <= 0 || k.Dist != d {
		t.Fatal("while it is open a net holds him")
	}
	run(g, g.Cfg.KingOpenSec)
	if k.open || k.netLeft > 0 || k.slowMult != 1 {
		t.Fatal("and when it closes nothing holds him again")
	}
}

func TestKingDemo(t *testing.T) {
	g, _ := newTestGame(t)
	g.KingDemo(BiomeRuins, false)
	art := 0
	for _, tw := range g.Towers {
		if g.artifact(tw) {
			art++
		}
	}
	if art != 5*g.limitAfter(34) || !g.finalDue {
		t.Fatalf("five players, every building an artifact, the final next: %d", art)
	}
	run(g, 6)
	if !g.finalWave || g.introLeft <= 0 {
		t.Fatal("the king's scene starts")
	}
}

func TestKingDemoWarlockNet(t *testing.T) {
	g, _ := newTestGame(t)
	g.KingDemo(BiomeRuins, true)
	altars, walls := 0, 0
	for _, tw := range g.Towers {
		switch tw.Kind {
		case "curse":
			altars++
		case "wall":
			walls++
		}
	}
	if walls != 0 || altars < 6 || len(g.soulLinks()) < altars/2 {
		t.Fatalf("no barricades, a net of artifact altars: %d altars, %d links", altars, len(g.soulLinks()))
	}
}

func TestFinalScalesWithTheTeam(t *testing.T) {
	size := func(players []string) (int, float64) {
		g, _ := newTestGame(t)
		for _, u := range players {
			g.HandleChat(ChatMessage{User: u, Text: "!play"})
			placeAs(g, strings.ToLower(u), "arrow", freeCell(t, g))
		}
		g.finalDue = true
		g.StartWave()
		return len(g.queue), g.queue[0].hp
	}
	duoN, duoHP := size([]string{"anna", "boris"})
	fullN, fullHP := size([]string{"anna", "boris", "vera", "gleb", "dina"})
	if duoN >= fullN/3 || duoHP >= fullHP/2 {
		t.Fatalf("a duo meets a much smaller final: escort %d vs %d, king %.0f vs %.0f", duoN, fullN, duoHP, fullHP)
	}
}
