package main

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"twitchtd/internal/game"
)

// The -king scenario plays only the final against the King in Yellow, the
// way the server's -kingdemo sets it up: one player of each class in -team,
// every one with as many artifact buildings as the limit allows (rogues'
// traps on the path, warriors' barricades across it), two of them wiping
// the Yellow Signs off; then it reports, by biome, how the fight went.
//
//	go run ./cmd/balance -king -games 12
//	go run ./cmd/balance -king -team rogue,ranger,mage,pyro,warrior -biome ice

// trapMode is what the rogue's traps are in the -king scenario.
var trapMode string

var allBiomes = []game.Biome{game.BiomeRuins, game.BiomeDungeon, game.BiomeCrypt, game.BiomeIce, game.BiomeSwamp, game.BiomeRiver}

// kingResult is one final.
type kingResult struct {
	won, killed bool
	kingLeft    float64            // share of his health left when the walls fell
	built       int                // buildings the ranger started with
	fight       float64            // seconds from the king's first step to his death or the walls'
	walls       float64            // share of the walls left
	share       map[string]float64 // share of his damage by class
	phase2      float64            // seconds into the fight each phase began, -1 never
	phase3      float64
	faceless    int // of the retinue summoned
	trapGuard   float64
	trapOpen    float64 // the rogue's damage to him under a whole and an open cloak
	heldGuard   int     // ticks he walked short under a whole cloak, with nothing in his way
	freeGuard   int     // ticks he walked under a whole cloak, with nothing in his way
	heldOpen    int
	freeOpen    int
	events      map[string]int
}

// kingEvents are the events the report counts.
var kingEvents = []string{"song", "fizzle", "sign", "signOff", "signFade", "drain", "petrify", "tear", "frostback", "doused", "linkBroken"}

func playKing(cfg game.Config, b game.Biome, team []string, seed uint64) kingResult {
	r := rand.New(rand.NewPCG(seed, 7))
	g := game.New(cfg, seed)
	clock := time.Unix(1_700_000_000, 0)
	g.Now = func() time.Time { return clock }
	g.KingDemoTeam(b, team)
	res := kingResult{phase2: -1, phase3: -1, events: map[string]int{}}
	// The rogue's traps as -traps asks: artifacts, plain level-5 ones, none.
	for p, tw := range g.Towers {
		if tw.Kind != "trap" {
			continue
		}
		switch trapMode {
		case "none":
			delete(g.Towers, p)
		case "plain":
			spec := cfg.Towers["trap"]
			tw.Level = spec.MaxLevel
			tw.Charges = spec.Charges + spec.Charges/3*(tw.Level-1)
		}
	}
	for _, tw := range g.Towers {
		if g.Chatters[tw.Owner] != nil && g.Chatters[tw.Owner].Class == "ranger" {
			res.built++
		}
	}
	class := map[string]string{} // player key -> class
	for key, c := range g.Chatters {
		if c.Class != "" {
			class[key] = c.Class
			if c.Class == "ranger" {
				g.SetAmmo(key, []string{"огонь", "яд", "взрыв"}[r.IntN(3)]) // as a player would pick
			}
		}
	}
	nextChat := map[string]float64{}
	const dt = 0.05
	t, start := 0.0, -1.0
	var last game.KingProbe
	for g.Phase != game.PhaseGameOver && t < 30*60 {
		for key := range class {
			if t >= nextChat[key] { // people chat now and then, staying in the tavern
				g.HandleChat(game.ChatMessage{User: g.Chatters[key].Name, Text: "gg"})
				nextChat[key] = t + 30 + r.Float64()*60
			}
		}
		before, there := g.KingNow()
		g.Tick(dt)
		for _, ev := range g.TakeEvents() {
			if ev.Kind == "kingIntro" {
				g.KingSceneDone() // no overlay: the scene is over at once
			}
			res.events[ev.Kind]++
		}
		t += dt
		clock = clock.Add(time.Duration(dt * float64(time.Second)))
		after, still := g.KingNow()
		if !there || !still {
			continue
		}
		if start < 0 {
			start = t
		}
		last = after
		if after.Phase >= 2 && res.phase2 < 0 {
			res.phase2 = t - start
		}
		if after.Phase >= 3 && res.phase3 < 0 {
			res.phase3 = t - start
		}
		res.faceless += max(0, after.Faceless-before.Faceless)
		for key, cl := range class {
			if cl == "rogue" {
				d := after.HurtBy[key] - before.HurtBy[key]
				if before.Open {
					res.trapOpen += d
				} else {
					res.trapGuard += d
				}
			}
		}
		// Did anything hold him? Only ticks he spent wholly in one state of
		// his cloak, with no barricade, sign or gate to stop him, count.
		if before.Open == after.Open && !before.Blocked && !after.Blocked && !before.Drawing && !after.Drawing && !after.AtGate {
			short := after.Dist-before.Dist < 0.9*before.Speed*dt
			if before.Open {
				res.freeOpen++
				if short {
					res.heldOpen++
				}
			} else {
				res.freeGuard++
				if short {
					res.heldGuard++
				}
			}
		}
	}
	res.won = g.Won()
	if k, alive := g.KingNow(); !alive && start >= 0 {
		res.killed = true
	} else if alive {
		res.kingLeft = k.HP / k.MaxHP
	}
	res.fight = t - max(start, 0)
	res.walls = g.Walls / cfg.WallsHP
	total := 0.0
	for _, d := range last.HurtBy {
		total += d
	}
	res.share = map[string]float64{}
	for key, d := range last.HurtBy {
		if total > 0 {
			res.share[class[key]] += d / total
		}
	}
	return res
}

func runKing(cfg game.Config, games int, team []string) {
	biomes := allBiomes
	if biome != "" {
		biomes = []game.Biome{biome}
	}
	fmt.Printf("Финал с Королём в Жёлтом: команда %s, капканы разбойника: %s, %d партий на биом\n\n", strings.Join(team, ", "), trapMode, games)
	header := "биом     | побед | ХП короля при поражении, мед. | бой, с p10/мед/p90 | стены | " + strings.Join(team, " | ") + " | ф2, с | ф3, с | безликих | песен/срывов | знаков/стёрто/слив ур. | держали под целым/раскрытым плащом | разбойник: урон под целым/раскрытым | сила биома"
	fmt.Println(header)
	var all []kingResult
	for _, b := range biomes {
		var rs []kingResult
		for i := 0; i < games; i++ {
			rs = append(rs, playKing(cfg, b, team, uint64(i*7919+int(b[0])*31+len(b))))
		}
		all = append(all, rs...)
		if b == biomes[0] {
			fmt.Printf("(построек у следопыта: %d)\n", rs[0].built)
		}
		printKingRow(string(b), rs, team, b)
	}
	if len(biomes) > 1 {
		printKingRow("все", all, team, "")
	}
}

// biomePower is the event that shows each biome's power at work.
var biomePower = map[game.Biome]string{game.BiomeRuins: "petrify", game.BiomeRiver: "tear", game.BiomeIce: "frostback", game.BiomeSwamp: "doused", game.BiomeCrypt: "linkBroken"}

func printKingRow(name string, rs []kingResult, team []string, b game.Biome) {
	n := float64(len(rs))
	var fights, p2, p3, left []float64
	wins := 0
	var walls, faceless, guard, open float64
	held := [4]int{}
	ev := map[string]float64{}
	share := map[string]float64{}
	for _, r := range rs {
		if r.won {
			wins++
		} else {
			left = append(left, r.kingLeft)
		}
		fights = append(fights, r.fight)
		if r.phase2 >= 0 {
			p2 = append(p2, r.phase2)
		}
		if r.phase3 >= 0 {
			p3 = append(p3, r.phase3)
		}
		walls += r.walls
		faceless += float64(r.faceless)
		guard += r.trapGuard
		open += r.trapOpen
		held[0] += r.heldGuard
		held[1] += r.freeGuard
		held[2] += r.heldOpen
		held[3] += r.freeOpen
		for k, v := range r.events {
			ev[k] += float64(v)
		}
		for k, v := range r.share {
			share[k] += v
		}
	}
	shares := make([]string, len(team))
	for i, cl := range team {
		shares[i] = fmt.Sprintf("%.0f%%", 100*share[cl]/n)
	}
	med := func(v []float64) string {
		if len(v) == 0 {
			return "—"
		}
		return fmt.Sprintf("%.0f (%d/%d)", pct(v, 50), len(v), len(rs))
	}
	power := "—"
	if k := biomePower[b]; k != "" {
		power = fmt.Sprintf("%s %.1f", k, ev[k]/n)
	} else if b == game.BiomeDungeon {
		power = "баррикады ×2"
	}
	rogueDmg := "—"
	if slices.Contains(team, "rogue") && guard+open > 0 {
		rogueDmg = fmt.Sprintf("%.0f%% / %.0f%%", 100*guard/(guard+open), 100*open/(guard+open))
	}
	kingLeft := "—"
	if len(left) > 0 {
		kingLeft = fmt.Sprintf("%.0f%%", 100*pct(left, 50))
	}
	fmt.Printf("%-8s | %3.0f%% | %s | %3.0f / %3.0f / %3.0f | %3.0f%% | %s | %s | %s | %.1f | %.1f/%.1f | %.1f/%.1f/%.1f | %s / %s | %s | %s\n",
		name, 100*float64(wins)/n, kingLeft, pct(fights, 10), pct(fights, 50), pct(fights, 90), 100*walls/n,
		strings.Join(shares, " | "), med(p2), med(p3), faceless/n,
		ev["song"]/n, ev["fizzle"]/n, ev["sign"]/n, ev["signOff"]/n, ev["drain"]/n,
		ratio(held[0], held[1]), ratio(held[2], held[3]), rogueDmg, power)
}

func ratio(a, b int) string {
	if b == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(a)/float64(b))
}
