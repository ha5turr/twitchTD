package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"twitchtd/internal/game"
)

// The -killbox scenario plays whole games with a team of five killbox
// players (see game.KillboxGame): from the first wave they wall in one turn,
// raise their class buildings to artifacts, drink and redeem the rain of
// arrows on boss waves, and in the final play as the server's -killbox demo.
// Teams follow game.KillboxTeams and every team meets the biomes in turn.
// -killboxx plays each team that many times over (two fives by default: five
// players alone seldom live to the final, see the plain run with 5 chatters).
// -killboxn takes that many players of each team instead.
//
//	go run ./cmd/balance -killbox -games 30
//	go run ./cmd/balance -killbox -killboxx 1   (one five)
//	go run ./cmd/balance -killbox -killboxshrines   (everyone builds shrines too)
//	go run ./cmd/balance -killbox -games 12 -biome ice -killboxpoints=false
//	go run ./cmd/balance -killbox -killboxn 5 -killboxvote 1   (one act above the advice)

// killboxN is how many players each team has, 0 for -killboxx fives.
var killboxN int

// killboxVote is what the team votes for over the advised acts.
var killboxVote int

func runKillbox(cfg game.Config, games, copies int, points, shrines bool) {
	fmt.Printf("Целые партии с командой килбокса ×%d, %d партий, награды за баллы: %v\n\n", copies, games, points)
	fmt.Println("# | команда | биом | волн | мин | финал | итог | ХП Короля | бой, с | акты пали, с | ф2/ф3, с | стены | построек/артефактов/ср.ур. | у поворота | выпито на боссах/в финале | дождей до/в финале | знаков/стёрто | переносов | акты (совет)")
	var rs []game.KillboxResult
	teams := map[string][2]int{}
	for i := 0; i < games; i++ {
		team := game.KillboxTeams[i%len(game.KillboxTeams)]
		b := biome
		if b == "" {
			b = allBiomes[(i+i/len(game.KillboxTeams))%len(allBiomes)]
		}
		players := slices.Repeat(team, copies)
		if killboxN > 0 { // the first killboxN of the team, over again as needed
			players = slices.Repeat(team, (killboxN+len(team)-1)/len(team))[:killboxN]
			team = players
		}
		r := playKillbox(cfg, b, players, points, shrines, uint64(i*7919+13))
		rs = append(rs, r)
		name := strings.Join(team, ",")
		t := teams[name]
		t[0]++
		res := "не дошли"
		switch {
		case r.Won:
			res = "победа"
			t[1]++
		case r.Final:
			res = "поражение"
		}
		teams[name] = t
		falls := "—"
		if len(r.Falls) > 0 {
			s := make([]string, len(r.Falls))
			for j, f := range r.Falls {
				s[j] = fmt.Sprintf("%.0f", f)
			}
			falls = strings.Join(s, ",")
		}
		left := "—"
		if r.Final && !r.Won {
			left = fmt.Sprintf("%.0f%%", 100*r.KingLeft)
		}
		fmt.Printf("%d | %s | %s | %d | %.0f | %v | %s | %s | %.0f | %s | %s/%s | %.0f%% | %d/%d/%.1f | %.0f%% | %d/%d | %d/%d | %d/%d | %d | %d (%d)\n",
			i+1, name, b, r.Waves, r.Minutes, r.Final, res, left, r.Fight, falls, sec(r.Phase2), sec(r.Phase3), 100*r.Walls,
			r.Towers, r.Arts, r.Level, 100*r.InBox, r.BossDrinks, r.Drinks, r.BossStrikes, r.Strikes, r.Signs, r.Wiped, r.Carries, r.Acts, r.Advice)
	}
	finals, wins := 0, 0
	var fights, waves, left []float64
	for _, r := range rs {
		waves = append(waves, float64(r.Waves))
		if r.Final {
			finals++
			if r.Won {
				wins++
				fights = append(fights, r.Fight)
			} else {
				left = append(left, r.KingLeft)
			}
		}
	}
	fmt.Printf("\nдошли до финала %d/%d, победы %d/%d (%.0f%% финалов); волн p10/мед/p90 %.0f/%.0f/%.0f", finals, len(rs), wins, len(rs), 100*float64(wins)/float64(max(finals, 1)), pct(waves, 10), pct(waves, 50), pct(waves, 90))
	if len(fights) > 0 {
		fmt.Printf("; победный бой, мед. %.0f с", pct(fights, 50))
	}
	if len(left) > 0 {
		fmt.Printf("; ХП Короля при поражении, мед. %.0f%%", 100*pct(left, 50))
	}
	fmt.Println()
	for _, team := range game.KillboxTeams {
		name := strings.Join(team, ",")
		if t, ok := teams[name]; ok {
			fmt.Printf("  %s: %d/%d\n", name, t[1], t[0])
		}
	}
}

func sec(s float64) string {
	if s < 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f", s)
}

func playKillbox(cfg game.Config, b game.Biome, team []string, points, shrines bool, seed uint64) game.KillboxResult {
	g := game.New(cfg, seed)
	clock := time.Unix(1_700_000_000, 0)
	g.Now = func() time.Time { return clock }
	g.NewGame(b)
	g.KillboxGame(team, points, shrines)
	if killboxVote != 0 {
		g.KillboxVote(killboxVote)
	}
	const dt = 0.05
	t, nextPresent := 0.0, 300.0
	for g.Phase != game.PhaseGameOver && t < 4*3600 {
		g.KillboxPlay(dt)
		if t >= nextPresent {
			var names []string
			for key := range g.Chatters {
				names = append(names, key)
			}
			g.Present(names)
			nextPresent += 300
		}
		g.Tick(dt)
		for _, ev := range g.TakeEvents() {
			if ev.Kind == "kingIntro" {
				g.KingSceneDone() // no overlay: the scene is over at once
			}
		}
		t += dt
		clock = clock.Add(time.Duration(dt * float64(time.Second)))
	}
	return g.KillboxReport()
}
