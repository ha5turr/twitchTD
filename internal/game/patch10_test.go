package game

import "testing"

func TestCurseAmplifiesAndStripsArmor(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	for _, u := range []string{"anna", "boris"} {
		g.HandleChat(ChatMessage{User: u, Text: "!play"})
		g.Chatters[u].Class = "ranger"
	}
	alt := placeAs(g, "anna", "curse", g.Map.Path[5])
	weak := enemyAt(g, "mummy", 5, 100)
	tough := enemyAt(g, "mummy", 5, 1000)
	g.tickCurse(alt, g.Cfg.Towers["curse"], 1)
	if tough.curseLeft <= 0 || weak.curseLeft > 0 {
		t.Fatal("the altar curses the toughest enemy in range")
	}
	hp := tough.HP
	g.damage(tough, 100, "boris", false)
	want := 100 * (1 + g.Cfg.Towers["curse"].CurseAmp)
	if got := hp - tough.HP; got < want-0.01 || got > want+0.01 {
		t.Fatalf("a cursed mummy takes +25%% and its armor is gone: %.1f, want %.1f", got, want)
	}
	g.Chatters["anna"].Gold = 0
	g.damage(tough, 1e9, "boris", false)
	if g.Chatters["anna"].Gold <= 0 {
		t.Fatal("a cursed kill pays the altar's owner a soul shard")
	}
}

func TestTotemChainsLightning(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	tw := placeAs(g, "anna", "totem", g.Map.Path[5])
	var es []*Enemy
	for i := 0; i < 6; i++ {
		e := enemyAt(g, "skeleton", 5, 1000)
		e.Pos = vec{e.Pos.X + float64(i)*0.5, e.Pos.Y}
		es = append(es, e)
	}
	spec := g.Cfg.Towers["totem"]
	g.chainZap(tw, spec, es[0], 100)
	hit := 0
	for _, e := range es {
		if e.HP < 1000 {
			hit++
		}
	}
	if hit != 1+spec.Chain {
		t.Fatalf("the bolt hits its target and jumps %d more times: %d hit", spec.Chain, hit)
	}
	if es[1].HP <= es[0].HP {
		t.Fatal("every jump is weaker")
	}
}

func TestBomberBlowsUpBarricades(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	wall := placeAs(g, "anna", "wall", g.Map.Path[5])
	e := enemyAt(g, "bomber", 5, 100)
	run(g, 1)
	if !e.dead || wall.HP > wall.MaxHP*(1-g.Cfg.Enemies["bomber"].Bomb)+0.01 {
		t.Fatalf("the goblin blows up at the barricade: dead %v, hp %.0f/%.0f", e.dead, wall.HP, wall.MaxHP)
	}
	if g.Chatters["anna"].Kills != 0 {
		t.Fatal("nobody gets the kill of a bomber that blew himself up")
	}
	g.Phase = PhaseWave
	enemyAt(g, "bomber", 5, 100)
	run(g, 1)
	if g.Towers[wall.Cell] != nil {
		t.Fatal("a second one finishes it")
	}
}

func TestShieldbearerCoversNeighbours(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	sk := enemyAt(g, "skeleton", 5, 1000)
	g.damage(sk, 100, "", false)
	open := 1000 - sk.HP
	sb := enemyAt(g, "shieldbearer", 5, 1000)
	sb.Pos = sk.Pos
	hp := sk.HP
	g.damage(sk, 100, "", false)
	if got := hp - sk.HP; got > open*0.51 {
		t.Fatalf("physical damage is halved under the shield: %.1f of %.1f", got, open)
	}
	hp = sk.HP
	g.damage(sk, 100, "", true)
	if hp-sk.HP != 100 {
		t.Fatal("magic goes through")
	}
}

func TestWarlockAndShamanSkills(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	c := g.Chatters["anna"]
	boss := g.spawnBoss("lich", 10000)
	boss.ability = 1e9
	sk := enemyAt(g, "skeleton", 5, 1000)
	ghost := enemyAt(g, "ghost", 5, 1000)
	g.massCurse(c)
	if sk.curseLeft <= 0 || sk.curseAmp != g.Cfg.WarlockCurseAmp || boss.curseAmp != g.Cfg.WarlockCurseAmp/2 || sk.curseOwner != "" {
		t.Fatal("the mass curse curses everyone, bosses at half, and pays no shards")
	}
	sk.curseLeft = 0
	g.Walls = g.Cfg.WallsHP
	if !g.bloodPact(c) || g.Walls != g.Cfg.WallsHP*(1-g.Cfg.PactWallsPct) || g.pactLeft <= 0 {
		t.Fatal("the blood pact costs the walls and powers every building")
	}
	tw := placeAs(g, "anna", "arrow", freeCell(t, g))
	if got := g.drinkMult(tw); got != 1+g.Cfg.PactBoost {
		t.Fatalf("buildings hit harder under the pact: %.2f", got)
	}
	g.Walls = g.Cfg.WallsHP * g.Cfg.PactMinWalls * 0.9
	if g.bloodPact(c) {
		t.Fatal("no pact while the walls are low")
	}
	g.earthquake(c)
	if sk.stunLeft != g.Cfg.QuakeStunSec || boss.stunLeft != g.Cfg.QuakeStunSec/2 || ghost.stunLeft > 0 {
		t.Fatal("the quake stuns the ground, bosses for half as long, ghosts not at all")
	}
}

func TestNewClassesExist(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "anna", Text: "!play"})
	g.HandleChat(ChatMessage{User: "anna", Text: "!class чернокнижник"})
	if g.Chatters["anna"].Class != "warlock" || g.classBonus("anna", "curse") <= 1 {
		t.Fatal("the warlock's signature building is the altar")
	}
	g.HandleChat(ChatMessage{User: "boris", Text: "!play"})
	g.Chatters["boris"].Gold = 500
	p := freeCell(t, g)
	g.HandleChat(ChatMessage{User: "boris", Text: "!b " + p.Name() + " тотем"})
	if tw := g.Towers[p]; tw == nil || tw.Kind != "totem" {
		t.Fatalf("a totem can be built from chat: %q", lastReply(g))
	}
}
