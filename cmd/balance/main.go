// Command balance plays many games headless with simulated chatters and
// reports how far the tavern holds and how long a game lasts, per chat size.
// With the match clock a game ends at the final (won or lost) or when the
// walls fall before it; the table tells how often the final came and was held.
//
//	go run ./cmd/balance -games 60
//	go run ./cmd/balance -config configs/candidate.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"time"

	"twitchtd/internal/game"
)

func main() {
	games := flag.Int("games", 40, "games per chat size")
	sizes := flag.String("chatters", "1,5,10,20", "comma-separated chat sizes")
	cfgPath := flag.String("config", "", "balance overrides (same format as config.json)")
	biomeFlag := flag.String("biome", "", "play every game in this biome (ruins, dungeon, crypt, ice, swamp, river)")
	readyFlag := flag.Bool("ready", false, "bots vote !готов once they have nothing to spend, cutting build phases short")
	flag.Parse()
	biome = game.Biome(*biomeFlag)
	hurry = *readyFlag

	cfg := game.DefaultConfig()
	if *cfgPath != "" {
		data, err := os.ReadFile(*cfgPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	fmt.Println("чатерсов | волн пережито: p10 / медиана / p90 | длительность, мин: медиана | построек в конце, ср. уровень | дошли до финала | победы")
	for _, s := range strings.Split(*sizes, ",") {
		var n int
		fmt.Sscan(s, &n)
		var waves, mins []float64
		var towers, level float64
		finals, wins := 0, 0
		for i := 0; i < *games; i++ {
			r := play(cfg, n, uint64(i*7919+n))
			waves = append(waves, float64(r.waves))
			mins = append(mins, r.minutes)
			towers += r.towers
			level += r.level
			if r.final {
				finals++
			}
			if r.won {
				wins++
			}
		}
		fmt.Printf("%8d | %5.0f / %5.0f / %5.0f | %6.1f | %5.1f, ур. %.1f | %3.0f%% | %3.0f%%\n",
			n, pct(waves, 10), pct(waves, 50), pct(waves, 90), pct(mins, 50), towers/float64(*games), level/float64(*games),
			100*float64(finals)/float64(*games), 100*float64(wins)/float64(*games))
	}
}

type result struct {
	waves   int
	minutes float64
	towers  float64 // standing when the walls fell
	level   float64 // their average level
	final   bool    // the final wave came
	won     bool    // and was held
}

func pct(v []float64, p float64) float64 {
	s := slices.Clone(v)
	slices.Sort(s)
	return s[min(len(s)-1, int(float64(len(s)-1)*p/100+0.5))]
}

// kinds is how often simulated chatters pick each building, roughly what a
// chat that does not min-max would do.
var kinds = []struct {
	kind   string
	weight float64
}{{"arrow", 27}, {"magic", 16}, {"fire", 14}, {"trap", 10}, {"wall", 9}, {"shrine", 9}, {"drums", 9}, {"curse", 8}, {"totem", 10}}

func pickKind(r *rand.Rand) string {
	total := 0.0
	for _, k := range kinds {
		total += k.weight
	}
	x := r.Float64() * total
	for _, k := range kinds {
		x -= k.weight
		if x <= 0 {
			return k.kind
		}
	}
	return "arrow"
}

type bot struct {
	name     string
	nextChat float64
	nextAct  float64
	cells    []game.Pos
}

// biome, when set, is the one every simulated game is played in.
var biome game.Biome

// hurry makes the bots vote !готов once they have nothing left to spend.
var hurry bool

// play runs one game to the end and reports the waves survived.
func play(cfg game.Config, chatters int, seed uint64) result {
	r := rand.New(rand.NewPCG(seed, 99))
	g := game.New(cfg, seed)
	clock := time.Unix(1_700_000_000, 0)
	g.Now = func() time.Time { return clock }
	if biome != "" {
		g.NewGame(biome)
	}

	bots := make([]*bot, chatters)
	for i := range bots {
		bots[i] = &bot{name: fmt.Sprintf("bot%02d", i), nextChat: r.Float64() * 30, nextAct: 5 + r.Float64()*20}
		g.HandleChat(game.ChatMessage{User: bots[i].name, Text: "!play"})
	}
	const dt = 0.05
	t := 0.0
	nextPresent := 300.0
	for g.Phase != game.PhaseGameOver && t < 4*3600 {
		for _, b := range bots {
			if t >= b.nextChat { // people chat about once a minute
				g.HandleChat(game.ChatMessage{User: b.name, Text: "gg"})
				b.nextChat = t + 30 + r.Float64()*60
			}
			if t >= b.nextAct {
				b.act(g, r)
				b.nextAct = t + 8 + r.Float64()*15
			}
		}
		if t >= nextPresent {
			names := make([]string, len(bots))
			for i, b := range bots {
				names[i] = b.name
			}
			g.Present(names)
			nextPresent += 300
		}
		g.Tick(dt)
		g.TakeEvents()
		t += dt
		clock = clock.Add(time.Duration(dt * float64(time.Second)))
	}
	levels := 0
	for _, tw := range g.Towers {
		levels += tw.Level
	}
	avg := 0.0
	if len(g.Towers) > 0 {
		avg = float64(levels) / float64(len(g.Towers))
	}
	return result{waves: max(g.Wave-1, 0), minutes: t / 60, towers: float64(len(g.Towers)), level: avg, final: g.FinalReached(), won: g.Won()}
}

// act spends gold the way a chatter would: build up to the limit, then upgrade.
func (b *bot) act(g *game.Game, r *rand.Rand) {
	c := g.Chatters[b.name]
	if c == nil {
		return
	}
	for _, p := range g.SignCells() {
		g.HandleChat(game.ChatMessage{User: b.name, Text: "!рз " + p.Name()}) // the chat wipes the Yellow Sign
	}
	mine := 0
	for _, t := range g.Towers {
		if t.Owner == b.name && t.Kind == "arrow" && g.IsArtifact(t) && t.Ammo == "" {
			// An artifact ballista gets arrows, as a player would pick them.
			ammo := []string{"огонь", "яд", "взрыв"}[r.IntN(3)]
			g.HandleChat(game.ChatMessage{User: b.name, Text: "!стрелы " + t.Cell.Name() + " " + ammo})
		}
		if t.Owner == b.name {
			mine++
			// A barricade under half its strength is mended first, as a
			// careful player would; the game refuses while it cools down.
			if t.Built && g.Cfg.Towers[t.Kind].Role == game.RoleWall && t.HP < t.MaxHP/2 && c.Gold >= 30 {
				g.HandleChat(game.ChatMessage{User: b.name, Text: "!чинить " + t.Cell.Name()})
			}
		}
	}
	if mine < g.TowerLimit() {
		kind := pickKind(r)
		if c.Gold+c.Grant < g.Cfg.TowerCost(kind, 1) {
			return
		}
		if p, ok := spot(g, r, g.Cfg.Towers[kind].OnPath()); ok {
			g.HandleChat(game.ChatMessage{User: b.name, Text: "!build " + p.Name() + " " + kind})
		}
		return
	}
	// Upgrade the cheapest of our buildings that still can grow.
	var best *game.Tower
	for _, t := range g.Towers {
		if t.Owner == b.name && t.Built && t.PendingCost == 0 && t.Level < g.TowerMaxLevel(t) {
			if best == nil || g.Cfg.TowerCost(t.Kind, t.Level+1) < g.Cfg.TowerCost(best.Kind, best.Level+1) {
				best = t
			}
		}
	}
	if best != nil && c.Gold+c.Grant >= g.Cfg.TowerCost(best.Kind, best.Level+1) {
		g.HandleChat(game.ChatMessage{User: b.name, Text: "!up " + best.Cell.Name()})
		return
	}
	if hurry && g.Phase == game.PhaseBuild {
		g.HandleChat(game.ChatMessage{User: b.name, Text: "!готов"}) // nothing to spend: go on
	}
}

// spot finds a free cell: on the path for traps and barricades, otherwise
// next to it, preferring cells that see much of the path.
func spot(g *game.Game, r *rand.Rand, onPath bool) (game.Pos, bool) {
	path := g.Map.Path
	if onPath {
		for tries := 0; tries < 20; tries++ {
			p := path[3+r.IntN(len(path)-4)]
			if g.Towers[p] == nil {
				return p, true
			}
		}
		return game.Pos{}, false
	}
	var best game.Pos
	bestScore := -1.0
	for y := 0; y < game.GridH; y++ {
		for x := 0; x < game.GridW; x++ {
			p := game.Pos{X: x, Y: y}
			if g.Map.Grid.At(p) != game.CellBuild || g.Towers[p] != nil {
				continue
			}
			score := 0.0
			for _, q := range path {
				dx, dy := float64(q.X-x), float64(q.Y-y)
				if dx*dx+dy*dy <= 6 {
					score++
				}
			}
			score += r.Float64() * 3 // chatters are not perfect
			if score > bestScore {
				best, bestScore = p, score
			}
		}
	}
	return best, bestScore > 0
}
