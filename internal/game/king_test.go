package game

import (
	"slices"
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
	g.Cfg.KingActs = 1 // one bar: the acts have their own tests (kingacts_test.go)
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
	if kingIndex(g) < 0 || g.introLeft <= 0 {
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
	if len(g.Enemies) == 0 {
		t.Fatal("!tdskip lets the final in")
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
	if len(g.Enemies) == 0 {
		t.Fatal("the overlay's end of the scene lets the final in")
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

func TestKingCloakTurnsSpikes(t *testing.T) {
	g, k := kingGame(t, BiomeRuins)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	if _, c := g.findChatter("anna"); c != nil {
		c.Class = "warrior"
	}
	wall := placeAs(g, "anna", "wall", g.Map.Path[len(g.Map.Path)/2])
	wall.Level = g.Cfg.Towers["wall"].MaxLevel + 1
	if !g.artifact(wall) {
		t.Fatal("an artifact barricade")
	}
	spikes := func() float64 {
		k.blocked = wall
		hp := k.HP
		g.hackBarricade(k, g.Cfg.Enemies[kingKind], 1, 0.5)
		return hp - k.HP
	}
	k.open = false
	if got := spikes(); got != 0 {
		t.Fatalf("with his cloak whole its spikes do not hurt him: %.1f", got)
	}
	k.open = true
	if got := spikes(); got <= 0 {
		t.Fatal("with it open they do")
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
		return len(g.queue), g.queue[kingIndex(g)].hp
	}
	duoN, duoHP := size([]string{"anna", "boris"})
	fullN, fullHP := size([]string{"anna", "boris", "vera", "gleb", "dina"})
	if duoN >= fullN/3 || duoHP >= fullHP/2 {
		t.Fatalf("a duo meets a much smaller final: escort %d vs %d, king %.0f vs %.0f", duoN, fullN, duoHP, fullHP)
	}
}

func TestYellowSignLooksAhead(t *testing.T) {
	g, k := kingGame(t, BiomeRuins)
	path := g.Map.Path
	beside := func(j int) Pos { // a free building cell next to path cell j
		for _, d := range []Pos{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			p := Pos{path[j].X + d.X, path[j].Y + d.Y}
			if p.X >= 0 && p.Y >= 0 && p.X < GridW && p.Y < GridH && g.Map.Grid.At(p) == CellBuild && g.Towers[p] == nil {
				return p
			}
		}
		return Pos{-1, -1}
	}
	i := -1
	for j := 4; j < len(path)-4 && i < 0; j++ {
		if a, b, c := beside(j-2), beside(j+2), beside(j+3); a.X >= 0 && b.X >= 0 && c.X >= 0 && b != c {
			i = j
		}
	}
	if i < 0 {
		t.Skip("no room on this map")
	}
	k.Dist = g.cellDist[i]
	k.Pos = g.routePos(k.Dist)
	behind := place(g, "arrow", beside(i-2))
	behind.heat = 1000
	ahead := place(g, "arrow", beside(i+2))
	ahead.heat = 10
	g.placeSign(k)
	if g.signOn(ahead) == nil {
		t.Fatal("he marks a building ahead of him before one he has passed")
	}
	g.signs, g.signSupport = nil, false
	ahead.heat = 1000
	fire := place(g, "fire", beside(i+3))
	fire.heat = 10
	k.hurtByKind = map[string]float64{"fire": 5000, "arrow": 100}
	g.placeSign(k)
	if g.signOn(fire) == nil {
		t.Fatal("ahead of him, the kind that hurt him most comes first")
	}
}

func TestYellowSignHitsTheBarricadeAhead(t *testing.T) {
	g, k := kingGame(t, BiomeRuins)
	i := len(g.Map.Path) / 2
	k.Dist = g.cellDist[i]
	k.Pos = g.routePos(k.Dist)
	wall := place(g, "wall", g.Map.Path[i+1])
	near := Pos{-1, -1} // a hot shooter within his reach
	for y := 0; y < GridH && near.X < 0; y++ {
		for x := 0; x < GridW; x++ {
			if p := (Pos{x, y}); g.Map.Grid.At(p) == CellBuild && center(p).sub(k.Pos).len() <= kingSignRadius {
				near = p
				break
			}
		}
	}
	if near.X < 0 {
		t.Skip("no room by the path")
	}
	gun := place(g, "arrow", near)
	gun.heat = 1e6
	g.signSupport = false // a shooter's turn
	g.placeSign(k)
	if g.signOn(wall) == nil {
		t.Fatal("a barricade one cell from him takes the sign first")
	}
	g.signs = nil
	g.removeTower(wall)
	g.placeSign(k)
	if g.signOn(gun) == nil {
		t.Fatal("without it the shooter's turn goes on as before")
	}
}

func TestKingMutesTheStream(t *testing.T) {
	g, _ := kingGame(t, BiomeRuins)
	sent := func() (out []string) {
		for _, e := range g.TakeEvents() {
			if e.Kind == "chatCommand" {
				out = append(out, e.Text)
			}
		}
		return out
	}
	sent()
	g.startKingIntro()
	if got := sent(); len(got) != 1 || got[0] != "!mute" {
		t.Fatalf("the scene starts with !mute: %v", got)
	}
	g.startKingIntro()
	if got := sent(); len(got) != 0 {
		t.Fatalf("only once: %v", got)
	}
	g.endMatch(true)
	run(g, kingUnmuteSec-1)
	if got := sent(); len(got) != 0 {
		t.Fatalf("not while his theme fades: %v", got)
	}
	run(g, 1.5)
	if got := sent(); len(got) != 1 || got[0] != "!unmute" {
		t.Fatalf("then !unmute: %v", got)
	}
	g.startKingIntro()
	sent()
	g.NewGame(BiomeRuins)
	if got := sent(); len(got) != 1 || got[0] != "!unmute" {
		t.Fatalf("a new map unmutes at once: %v", got)
	}
}

// kingNet sets the king on the path at cell 6 with a skeleton just ahead of
// him under one net, and a ballista over them.
func kingNet(t *testing.T, biome Biome) (*Game, *Enemy, *Enemy, *Tower, TowerSpec) {
	t.Helper()
	g, k := kingGame(t, biome)
	k.Dist = g.cellDist[6]
	k.Pos = g.routePos(k.Dist)
	sk := g.spawnEnemy("skeleton", 1e9, k.Dist+0.1)
	sk.Pos = vec{k.Pos.X + 0.2, k.Pos.Y}
	return g, k, sk, &Tower{Cell: g.Map.Path[6], Level: 1, Kind: "arrow", Owner: "anna"}, g.Cfg.Towers["arrow"]
}

func TestNetSlidesOffTheWholeCloak(t *testing.T) {
	g, k, sk, tw, spec := kingNet(t, BiomeRuins)
	g.tickNet(tw, spec, 1)
	if sk.netLeft <= 0 {
		t.Fatal("the net catches the skeleton it was thrown at")
	}
	if k.netLeft > 0 || k.netImmune > 0 {
		t.Fatalf("under the whole cloak it leaves nothing on the king: net %.1f, wariness %.1f", k.netLeft, k.netImmune)
	}
	// Nor at him: the net is spent on those around, he walks on.
	sk.Dist = k.Dist - 1
	tw.netCD = 0
	g.tickNet(tw, spec, 1)
	if k.netLeft > 0 || k.netImmune > 0 {
		t.Fatal("a net thrown at him does not hold him either")
	}
	d := k.Dist
	run(g, 0.5)
	if k.Dist <= d {
		t.Fatal("he walks on")
	}
}

func TestNetFromTheWholeCloakDoesNotCarryOver(t *testing.T) {
	g, k := kingGame(t, BiomeRuins)
	run(g, 0.1)
	k.cloakCD = 0.01 // it opens on the next tick
	k.netLeft, k.stunLeft, k.slowMult, k.slowLeft = 2, 2, 0.3, 2
	g.Tick(0.05)
	if !k.open {
		t.Fatal("the cloak opens")
	}
	if k.netLeft > 0 || k.stunLeft > 0 || k.slowMult != 1 {
		t.Fatal("what got on him while it was whole does not hold him in the open spell")
	}
}

func TestNetHoldsTheOpenCloak(t *testing.T) {
	g, k, sk, tw, spec := kingNet(t, BiomeRuins)
	k.open, k.cloakCD = true, 100
	g.tickNet(tw, spec, 1)
	if k.netLeft <= 0 || sk.netLeft <= 0 {
		t.Fatal("while the cloak is open a net on his neighbour holds him too")
	}
	if k.netImmune <= 0 {
		t.Fatal("and he tears free like any boss")
	}
}

func TestIceFreezesOnlyTheBallistaAimedAtHim(t *testing.T) {
	g, k, sk, tw, spec := kingNet(t, BiomeIce)
	g.tickNet(tw, spec, 1)
	if tw.Disabled > 0 {
		t.Fatal("a net thrown at his neighbour does not freeze the ballista")
	}
	sk.Dist = k.Dist - 1
	tw.netCD = 0
	g.tickNet(tw, spec, 1)
	if tw.Disabled <= 0 {
		t.Fatal("one thrown at him does")
	}
}

func TestKingWalksBehindHalfHisEscort(t *testing.T) {
	g, _ := newTestGame(t)
	for _, u := range []string{"anna", "boris", "vera", "gleb", "dina"} {
		g.HandleChat(ChatMessage{User: u, Text: "!play"})
	}
	g.finalDue = true
	g.StartWave()
	at := kingIndex(g)
	escort := len(g.queue) - 1
	if at < 1 || at != kingAhead(escort) || at >= len(g.queue)-1 {
		t.Fatalf("half the escort goes ahead of him, the rest after: king %d of %d", at, len(g.queue))
	}
	g.skipKingIntro()
	for range at {
		g.spawnTimer = 0
		g.tickSpawns(0.05)
	}
	for _, e := range g.Enemies {
		if e.Kind == kingKind {
			t.Fatal("he does not lead")
		}
	}
	g.spawnTimer = 0
	g.tickSpawns(0.05)
	if last := g.Enemies[len(g.Enemies)-1]; last.Kind != kingKind {
		t.Fatalf("then he comes: %s", last.Kind)
	}
}

func TestSmallEscortStillGoesAhead(t *testing.T) {
	if kingAhead(1) != 1 || kingAhead(0) != 0 || kingAhead(7) != 3 {
		t.Fatal("at least one goes ahead, never more than there are")
	}
}

// kingIndex finds the king in the wave's queue.
func kingIndex(g *Game) int {
	return slices.IndexFunc(g.queue, func(s spawn) bool { return s.kind == kingKind })
}
