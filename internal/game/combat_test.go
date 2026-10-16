package game

import "testing"

// place puts a finished building owned by the streamer at cell.
func place(g *Game, kind string, p Pos) *Tower {
	t := &Tower{ID: g.id(), Kind: kind, Cell: p, Owner: StreamerName, Level: 1, Built: true}
	g.Towers[p] = t
	g.readyTower(t)
	return t
}

// enemyAt puts an enemy on the route just before path cell i.
func enemyAt(g *Game, kind string, i int, hp float64) *Enemy {
	return g.spawnEnemy(kind, hp, g.cellDist[i]-0.6)
}

func TestTrapHurtsSlowsAndRunsOutOfCharges(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	trap := place(g, "trap", g.Map.Path[5])
	e := enemyAt(g, "skeleton", 5, 1000)
	run(g, 1)
	if e.HP >= 1000 || e.slowMult != g.Cfg.Towers["trap"].Slow && e.slowLeft <= 0 {
		t.Fatalf("trap did nothing: hp %.0f slow %.2f", e.HP, e.slowMult)
	}
	if trap.Charges != g.Cfg.Towers["trap"].Charges-1 {
		t.Fatalf("charges %d", trap.Charges)
	}
	trap.Charges = 0
	e2 := enemyAt(g, "skeleton", 5, 1000)
	run(g, 1)
	if e2.HP != 1000 {
		t.Fatal("an empty trap must not fire")
	}
	g.endWave()
	if trap.Charges == 0 {
		t.Fatal("traps re-arm between waves")
	}
}

func TestBarricadeBlocksUntilBroken(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	wall := place(g, "wall", g.Map.Path[6])
	e := enemyAt(g, "skeleton", 6, 1e9)
	run(g, 3)
	if e.blocked != wall || e.Dist >= g.cellDist[6]-0.5 {
		t.Fatalf("enemy should stop at the barricade: dist %.2f edge %.2f", e.Dist, g.cellDist[6]-0.5)
	}
	if wall.HP >= wall.MaxHP {
		t.Fatal("the barricade should take damage")
	}
	ghost := enemyAt(g, "ghost", 6, 1e9)
	run(g, 2)
	if ghost.Dist <= g.cellDist[6] {
		t.Fatal("ghosts float through barricades")
	}
	wall.HP = 0.01
	run(g, 0.5)
	if g.Towers[wall.Cell] != nil || e.blocked != nil {
		t.Fatal("a broken barricade is removed and frees the enemy")
	}
}

func TestFireSplashAndBurn(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	a := enemyAt(g, "skeleton", 4, 1000)
	b := enemyAt(g, "skeleton", 4, 1000)
	b.Dist += 0.3
	g.Projectiles = append(g.Projectiles, &Projectile{
		ID: g.id(), Kind: "fire", Pos: a.Pos, Target: a.ID, Aim: a.Pos, Damage: 10, Speed: 6,
		Owner: StreamerName, Splash: 0.9, BurnDPS: 50, BurnSec: 2,
	})
	g.tickProjectiles(0.05)
	if a.HP >= 1000 || b.HP >= 1000 {
		t.Fatalf("splash should hit both: %.0f %.0f", a.HP, b.HP)
	}
	before := a.HP
	g.tickEnemies(1)
	if a.HP >= before-40 {
		t.Fatalf("burn should keep hurting: %.0f -> %.0f", before, a.HP)
	}
}

func TestAuras(t *testing.T) {
	g, _ := newTestGame(t)
	p := freeCell(t, g)
	arrow := place(g, "arrow", p)
	dmg, rate := g.auras(arrow)
	if dmg != 1 || rate != 1 {
		t.Fatal("no auras yet")
	}
	for _, d := range dirs {
		q := Pos{p.X + d.X, p.Y + d.Y}
		if q.In() && g.Map.Grid.At(q) == CellBuild && g.Towers[q] == nil {
			if len(g.Towers) == 1 {
				place(g, "shrine", q)
			} else {
				place(g, "drums", q)
				break
			}
		}
	}
	if len(g.Towers) < 3 {
		t.Skip("no room for both auras next to this cell")
	}
	dmg, rate = g.auras(arrow)
	if dmg <= 1 || rate <= 1 {
		t.Fatalf("auras missing: dmg %.2f rate %.2f", dmg, rate)
	}
	g.Walls = 500
	g.tickTowers(10)
	if g.Walls <= 500 {
		t.Fatal("the shrine repairs the walls")
	}
}

func TestBossesSummonAndDisable(t *testing.T) {
	g, _ := newTestGame(t)
	g.Phase = PhaseWave
	g.waveHP = 100
	lich := enemyAt(g, "lich", 8, 1e9)
	g.tickBoss(lich, 0.05)
	if len(g.Enemies) != 4 {
		t.Fatalf("the lich raises three skeletons, enemies %d", len(g.Enemies))
	}
	tw := place(g, "arrow", freeCell(t, g))
	golem := g.spawnEnemy("ice_golem", 1e9, 0)
	golem.Pos = center(tw.Cell)
	g.tickBoss(golem, 0.05)
	if tw.Disabled <= 0 {
		t.Fatal("the ice golem freezes buildings around it")
	}
}

func TestBiomePools(t *testing.T) {
	for b, pool := range biomeEnemies {
		for roll := 0.0; roll < 1; roll += 0.05 {
			k := pickEnemy(roll, pool, 1)
			if _, ok := DefaultConfig().Enemies[k]; !ok {
				t.Fatalf("%s: unknown enemy %q", b, k)
			}
		}
		for _, boss := range biomeBosses[b] {
			if !DefaultConfig().Enemies[boss].Boss {
				t.Fatalf("%s: %q is not a boss", b, boss)
			}
		}
	}
}

func TestPathBuildingsOnlyOnPath(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "ivan", Text: "!build " + freeCell(t, g).Name() + " trap"})
	g.HandleChat(ChatMessage{User: "ivan", Text: "!build " + g.Map.Path[4].Name() + " arrow"})
	if len(g.Towers) != 0 {
		t.Fatal("traps go on the path, towers off it")
	}
	g.HandleChat(ChatMessage{User: "ivan", Text: "!build " + g.Map.Path[4].Name() + " капкан"})
	if g.Towers[g.Map.Path[4]] == nil {
		t.Fatal("a trap on the path should be accepted")
	}
}
