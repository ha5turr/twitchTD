package game

import (
	"strings"
	"testing"
)

// matchGame seats anna and boris, each with a building, at a build phase.
func matchGame(t *testing.T) *Game {
	g, _ := newTestGame(t)
	for _, n := range []string{"anna", "boris"} {
		g.HandleChat(ChatMessage{User: n, Text: "!play"})
		placeAs(g, n, "arrow", freeCell(t, g))
	}
	g.Phase, g.Timer = PhaseBuild, 30
	return g
}

func TestMatchClockWarnsAndCallsTheFinal(t *testing.T) {
	g := matchGame(t)
	g.matchElapsed = g.Cfg.MatchMinutes*60 - 10*60 - 0.5
	g.tickMatch(1)
	if !strings.Contains(lastReply(g), "10 минут") {
		t.Fatalf("10 minutes left are announced: %q", lastReply(g))
	}
	g.Paused = true
	before := g.matchElapsed
	g.Tick(5)
	if g.matchElapsed != before {
		t.Fatal("the streamer's pause stops the clock")
	}
	g.Paused = false

	g.Phase = PhaseWave
	g.queue = []spawn{{"skeleton", 10}}
	g.matchElapsed = g.Cfg.MatchMinutes * 60
	g.tickMatch(0.1)
	if !g.finalDue || g.finalWave || !strings.Contains(lastReply(g), "Доиграйте") {
		t.Fatal("time up mid-wave: the wave is played to its end, the final comes after the build phase")
	}

	g.Phase, g.Timer = PhaseBuild, 0
	g.Wave = 7
	g.StartWave()
	if !g.finalWave {
		t.Fatal("after the build phase comes the final")
	}
	bosses := 0
	for _, s := range g.queue {
		if g.Cfg.Enemies[s.kind].Boss {
			bosses++
		}
	}
	if bosses != 1 || g.queue[0].kind != kingKind {
		t.Fatalf("the King in Yellow alone leads it: %d bosses", bosses)
	}
	// The final is as strong as wave FinalWave, whatever wave was reached.
	if g.waveHP < g.Cfg.BaseHP*pow(g.Cfg.HPGrowth, g.Cfg.FinalWave) {
		t.Fatalf("the final is pinned to the clock: hp %.0f", g.waveHP)
	}

	g.queue, g.Enemies = nil, nil
	g.Tick(0.05)
	if g.Phase != PhaseGameOver || g.summary == nil || !g.summary.Won {
		t.Fatal("the final held: the tavern wins")
	}
	if !g.Chatters["anna"].Ach["victory"] && g.Chatters["anna"].GameDamage > 0 {
		t.Fatal("and the fighters get «Час выстоян»")
	}
	g.Tick(60)
	if g.Phase != PhaseGameOver {
		t.Fatal("then waits for !нк")
	}
}

func pow(x float64, n int) float64 {
	r := 1.0
	for i := 0; i < n; i++ {
		r *= x
	}
	return r
}

func TestReadyVoteAndTempo(t *testing.T) {
	g := matchGame(t)
	g.HandleChat(ChatMessage{User: "vera", Text: "!play"})
	anna, vera := g.Chatters["anna"], g.Chatters["vera"]
	anna.Gold, vera.Gold = 0, 0
	g.HandleChat(ChatMessage{User: "anna", Text: "!готов"})
	if g.readyGo {
		t.Fatal("one of three is not enough")
	}
	g.HandleChat(ChatMessage{User: "boris", Text: "!go"})
	if !g.readyGo || g.Timer != g.Cfg.ReadyCountdownSec {
		t.Fatalf("two of three: the wave comes after the countdown (%.1f s)", g.Timer)
	}
	g.Tick(g.Cfg.ReadyCountdownSec + 0.1)
	if g.Phase != PhaseWave {
		t.Fatal("and starts")
	}
	if want := int(min(30-g.Cfg.ReadyCountdownSec, g.Cfg.TempoGoldMax)); anna.Gold != want || vera.Gold != 0 {
		t.Fatalf("25 s saved pay 25 gold to those with buildings, up to the cap: anna %d, vera %d", anna.Gold, vera.Gold)
	}
	if len(g.ready) != 0 {
		t.Fatal("the vote starts afresh every build phase")
	}
}

func TestMatchClockSurvivesRestart(t *testing.T) {
	g := matchGame(t)
	g.matchElapsed, g.finalDue = 1234, true
	g2, _ := newTestGame(t)
	g2.LoadChatters(g.Export().Chatters)
	g2.Restore(g.Export())
	if g2.matchElapsed != 1234 || !g2.finalDue {
		t.Fatal("a restart keeps the match clock and the final due")
	}
	g2.NewGame("")
	if g2.matchElapsed != 0 || g2.finalDue {
		t.Fatal("a new game starts the clock afresh")
	}
}

func TestDefeatWaitsAndAutoRestartOption(t *testing.T) {
	g := matchGame(t)
	g.Phase = PhaseWave
	g.Walls = 0
	g.Tick(0.05)
	if g.Phase != PhaseGameOver || g.summary.Won {
		t.Fatal("the walls fell: lost")
	}
	g.Tick(100)
	if g.Phase != PhaseGameOver {
		t.Fatal("no new game by itself")
	}
	g.Cfg.GameOverSec = 5
	g.Timer = 5
	g.Tick(6)
	if g.Phase != PhaseBuild {
		t.Fatal("unless gameOverSec is set")
	}
}
