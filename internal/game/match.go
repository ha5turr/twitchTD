package game

import (
	"fmt"
	"math"
)

// A game is a match against the clock: MatchMinutes of play, counted in
// both phases and stopped by the streamer's pause, saved with the board so
// a restart or an update does not reset it. When it runs out the wave in
// progress is played to its end, and after the build phase that follows
// comes the final wave. Hold it and the tavern wins; lose the walls on the
// way and it falls. Either way the game then waits for !нк.
//
// The final's strength is fixed to the clock, not to the wave reached: it
// is the wave FinalWave would be. A team that hurried through its build
// phases meets the same final with more gold and levels behind it.

// warnAt are the minutes left the chat is told about.
var warnAt = []float64{30, 10, 5, 1}

// matchLeft is how many seconds of the match are left; +Inf with no limit.
func (g *Game) matchLeft() float64 {
	if g.Cfg.MatchMinutes <= 0 {
		return math.Inf(1)
	}
	return max(0, g.Cfg.MatchMinutes*60-g.matchElapsed)
}

// tickMatch runs the match clock: the warnings, and the final once the time
// is up.
func (g *Game) tickMatch(dt float64) {
	if g.Cfg.MatchMinutes <= 0 || g.Phase == PhaseGameOver {
		return
	}
	before := g.matchLeft()
	g.matchElapsed += dt
	left := g.matchLeft()
	for _, m := range warnAt {
		if s := m * 60; before > s && left <= s && s < g.Cfg.MatchMinutes*60 {
			g.emit(Event{Kind: "matchWarn", Amount: int(m)})
			g.reply("", fmt.Sprintf("до финала %s", minutesLeft(int(m))))
		}
	}
	if left > 0 || g.finalDue {
		return
	}
	g.finalDue = true
	g.emit(Event{Kind: "timeUp"})
	if g.Phase == PhaseWave {
		g.reply("", "время вышло! Доиграйте эту волну: после стройки придёт финал")
	} else {
		g.reply("", "время вышло! Финал — сразу после этой стройки")
	}
}

func minutesLeft(m int) string {
	switch {
	case m == 1:
		return "1 минута"
	case m < 5:
		return fmt.Sprintf("%d минуты", m)
	}
	return fmt.Sprintf("%d минут", m)
}

// finalQueue fills the wave with the final: a wave as strong as wave
// FinalWave, both of the biome's bosses at its head.
func (g *Game) finalQueue(sq float64) {
	w := g.Cfg.FinalWave
	hp := g.Cfg.BaseHP * math.Pow(g.Cfg.HPGrowth, float64(w)) * (1 + g.Cfg.ChatterHPCoef*sq)
	g.waveHP = hp
	pool := biomeEnemies[g.Map.Biome]
	g.queue = g.queue[:0]
	g.queue = append(g.queue, spawn{kingKind, g.kingHP(hp) * g.kingTeamMult()}) // the King in Yellow, always
	count := g.Cfg.BaseCount + g.Cfg.CountPerWave*w + int(sq)
	// A small team meets a smaller escort, by the square of its share: its
	// buildings are fewer and each one has to hold more of the path.
	share := g.teamShare()
	count = max(1, int(math.Round(float64(count)*max(share*share, g.Cfg.FinalEscortMin))))
	for i := 0; i < count; i++ {
		kind := pickEnemy(g.rng.Float64(), pool, w)
		g.queue = append(g.queue, spawn{kind, hp * g.Cfg.Enemies[kind].HPMult})
	}
	g.finalWave = true
	g.startKingIntro()
	g.reply("", "ФИНАЛ! Король в Жёлтом идёт на таверну. Выстоим — победа")
}

// teamShare is how much of a full team (FinalTeamSize) plays the final, at
// most 1: bigger chats meet the final as designed.
func (g *Game) teamShare() float64 {
	if g.Cfg.FinalTeamSize <= 0 {
		return 1
	}
	n := g.finalTeam
	if n <= 0 {
		n = g.Cfg.FinalTeamSize
	}
	return min(1, n/g.Cfg.FinalTeamSize)
}

// kingTeamMult scales the king's health to the team: its share to the
// power KingTeamPow, a full team's at most.
func (g *Game) kingTeamMult() float64 {
	return math.Pow(g.teamShare(), g.Cfg.KingTeamPow)
}

// cmdReady is a vote to start the wave early: once more than half the
// players in the tavern are ready, it starts after a short countdown, and
// the seconds saved pay everyone with buildings (payTempo).
func (g *Game) cmdReady(c *Chatter, key string) {
	if g.Phase != PhaseBuild || !c.Joined || g.readyGo {
		return
	}
	if g.ready == nil {
		g.ready = map[string]bool{}
	}
	g.ready[key] = true
	need := len(g.mapVoters())/2 + 1
	g.emit(Event{Kind: "ready", User: c.Name, Amount: len(g.ready)})
	if len(g.ready) < need {
		return
	}
	g.readyGo = true
	if cd := g.Cfg.ReadyCountdownSec; g.Timer > cd {
		g.tempoSaved += g.Timer - cd
		g.Timer = cd
	}
	g.emit(Event{Kind: "readyGo", Amount: int(math.Ceil(g.Timer))})
	g.reply("", fmt.Sprintf("таверна готова! Волна через %d с", int(math.Ceil(g.Timer))))
}

// readyNeed is how many ready players start the wave now.
func (g *Game) readyNeed() int { return len(g.mapVoters())/2 + 1 }

// payTempo pays the build phase cut short: TempoGoldPerSec for every second
// saved, up to TempoGoldMax a wave, both growing with the rewards, to
// everyone with buildings.
func (g *Game) payTempo(saved float64) {
	if saved < 1 || g.Cfg.TempoGoldPerSec <= 0 {
		return
	}
	gold := int(math.Round(min(saved*g.Cfg.TempoGoldPerSec, g.Cfg.TempoGoldMax) * g.rewardMult))
	if gold <= 0 {
		return
	}
	paid := map[string]bool{}
	for _, t := range g.Towers {
		if c := g.Chatters[t.Owner]; c != nil && !paid[t.Owner] && t.Owner != StreamerName {
			paid[t.Owner] = true
			g.addGold(c, gold)
		}
	}
	if len(paid) > 0 {
		g.emit(Event{Kind: "tempo", Amount: gold})
	}
}

// endMatch ends the game: won when the final wave was held, lost when the
// walls fell. The summary goes up, souls are paid (more for a win) and the
// game waits for !нк, or GameOverSec when that is set.
func (g *Game) endMatch(won bool) {
	g.Phase = PhaseGameOver
	g.clearKing()
	if g.muted {
		g.unmuteLeft = kingUnmuteSec // once his theme has faded out
	}
	g.Timer = g.Cfg.GameOverSec
	g.Workers = map[string]*Worker{}
	g.summary = g.makeSummary()
	g.summary.Won = won
	g.summary.Minutes = int(g.matchElapsed / 60)
	if won {
		g.summary.Wave = g.Wave // the final counts as held
	}
	g.awardSouls(g.summary.Wave, won)
	if won {
		for key, c := range g.Chatters {
			if key != StreamerName && c.GameDamage > 0 {
				g.unlock(key, "victory")
			}
		}
		g.emit(Event{Kind: "victory", Amount: g.Wave})
	} else {
		g.emit(Event{Kind: "gameOver", Amount: g.Wave})
	}
	if g.Cfg.GameOverSec <= 0 {
		g.reply("", "новая партия — !нк [биом]")
	}
}

// Won reports whether the game ended with the final held (the balance
// simulator asks).
func (g *Game) Won() bool { return g.summary != nil && g.summary.Won }

// FinalReached reports whether the final wave came (the balance simulator
// asks).
func (g *Game) FinalReached() bool { return g.finalWave || g.Won() }
