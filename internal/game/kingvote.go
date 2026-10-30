package game

import (
	"fmt"
	"math"
	"slices"
	"strconv"
)

// Patch 0.27: the chat picks how many acts the King in Yellow plays. One
// act per KingActsPer players is advised, and KingActsSpread more or fewer
// may be picked: 1–2 players pick 1–2 acts, 3–4 pick 1–3, 5–6 pick 2–4 and
// so on. The vote (!акты 3) opens KingVoteMin minutes before the final and
// closes when the final wave starts, before his scene; the median of the
// votes wins, and with no votes the advice does. A win pays KingActSouls
// more souls for each act above the advice, and as much less below it.
// A fixed KingActs (the demos, a config) turns the vote off.

// actsVoting reports whether the chat picks the acts at all.
func (g *Game) actsVoting() bool { return g.Cfg.KingActs <= 0 && g.Cfg.KingActsPer > 0 }

// actsRange is the advised acts for n players and the fewest and most they
// may pick.
func (g *Game) actsRange(n int) (advice, lo, hi int) {
	top := g.Cfg.KingActsMax
	if top <= 0 {
		top = math.MaxInt32
	}
	advice = min(top, max(1, int(math.Ceil(float64(max(n, 1))/g.Cfg.KingActsPer))))
	spread := max(0, g.Cfg.KingActsSpread)
	return advice, max(1, advice-spread), min(top, advice+spread)
}

// actsNow is the range for the players in the tavern now.
func (g *Game) actsNow() (advice, lo, hi int) { return g.actsRange(len(g.mapVoters())) }

// actVoteOpen reports whether !акты counts now: the last minutes before the
// final, until it starts.
func (g *Game) actVoteOpen() bool {
	if !g.actsVoting() || g.finalWave || g.Phase == PhaseGameOver {
		return false
	}
	return g.finalDue || g.matchLeft() <= g.Cfg.KingVoteMin*60
}

// actsPicked is the lower median of the votes within lo..hi, 0 with none.
func (g *Game) actsPicked(lo, hi int) int {
	var v []int
	for _, n := range g.actVotes {
		v = append(v, min(hi, max(lo, n)))
	}
	if len(v) == 0 {
		return 0
	}
	slices.Sort(v)
	return v[(len(v)-1)/2]
}

// cmdActs is a vote for the King's acts (!акты 3).
func (g *Game) cmdActs(c *Chatter, key string, args []string) {
	if !g.actsVoting() || !c.Joined {
		return
	}
	advice, lo, hi := g.actsNow()
	if !g.actVoteOpen() {
		if g.finalWave {
			g.reply(c.Name, "Король уже здесь: актов будет столько, сколько выбрали")
		} else {
			g.reply(c.Name, fmt.Sprintf("голосование за акты Короля откроется за %d мин до финала", int(g.Cfg.KingVoteMin)))
		}
		return
	}
	n := 0
	if len(args) > 0 {
		n, _ = strconv.Atoi(args[0])
	}
	if n < lo || n > hi {
		g.reply(c.Name, fmt.Sprintf("сколько актов выдержите? !акты %s (советуем %d)", rangeText(lo, hi), advice))
		return
	}
	if g.actVotes == nil {
		g.actVotes = map[string]int{}
	}
	g.actVotes[key] = n
	now := g.actsPicked(lo, hi)
	g.emit(Event{Kind: "actVote", User: c.Name, Amount: n})
	g.reply(c.Name, fmt.Sprintf("голос за %s принят. Сейчас побеждает %s (%s)", actsWord(n), actsWord(now), votesWord(len(g.actVotes))))
}

// announceActs calls the chat to the vote: at every build phase while it is open.
func (g *Game) announceActs() {
	if !g.actVoteOpen() || g.Phase != PhaseBuild {
		return
	}
	advice, lo, hi := g.actsNow()
	g.emit(Event{Kind: "actVoteOpen", Amount: advice})
	g.reply("", fmt.Sprintf("Король в Жёлтом близко. Сколько актов выдержите? Голосуйте до финала: !акты %s (советуем %d)", rangeText(lo, hi), advice))
}

// chooseActs settles the acts as the final starts: the votes' median, or
// the advice. The votes stay till a new game: an interrupted final is
// played again as picked.
func (g *Game) chooseActs() {
	g.acts, g.actsAdvice = 0, 0
	if !g.actsVoting() {
		return
	}
	advice, lo, hi := g.actsNow()
	g.acts, g.actsAdvice = advice, advice
	how := "никто не голосовал: играем по совету"
	if n := g.actsPicked(lo, hi); n > 0 {
		g.acts = n
		how = "так решила таверна (" + votesWord(len(g.actVotes)) + ")"
	}
	g.reply("", fmt.Sprintf("Король сыграет %s: %s", actsWord(g.acts), how))
}

// actsStake is what the acts picked make of a win's souls.
func (g *Game) actsStake() float64 {
	if g.actsAdvice <= 0 {
		return 1
	}
	return max(0.5, 1+g.Cfg.KingActSouls*float64(g.acts-g.actsAdvice))
}

// actCD is how much sooner his powers come back in his act: 1 in the first.
func (g *Game) actCD(e *Enemy) float64 {
	if e.kingAct <= 0 || g.Cfg.KingActCD <= 0 {
		return 1
	}
	return max(g.Cfg.KingActCDMin, math.Pow(g.Cfg.KingActCD, float64(e.kingAct)))
}

// ActVoteView is the call to the vote for the overlay.
type ActVoteView struct {
	Advice int `json:"advice"`
	Lo     int `json:"lo"`
	Hi     int `json:"hi"`
	Now    int `json:"now"` // what wins now: the median, or the advice with no votes
	Votes  int `json:"votes"`
}

func (g *Game) actVoteView() *ActVoteView {
	if !g.actVoteOpen() {
		return nil
	}
	advice, lo, hi := g.actsNow()
	now := g.actsPicked(lo, hi)
	if now == 0 {
		now = advice
	}
	return &ActVoteView{advice, lo, hi, now, len(g.actVotes)}
}

func rangeText(lo, hi int) string {
	if lo == hi {
		return strconv.Itoa(lo)
	}
	return fmt.Sprintf("%d–%d", lo, hi)
}

func actsWord(n int) string {
	return fmt.Sprintf("%d %s", n, ruPlural(n, "акт", "акта", "актов"))
}

func votesWord(n int) string {
	return fmt.Sprintf("%d %s", n, ruPlural(n, "голос", "голоса", "голосов"))
}

// ruPlural picks the Russian form for n: one, few or many.
func ruPlural(n int, one, few, many string) string {
	m10, m100 := n%10, n%100
	switch {
	case m10 == 1 && m100 != 11:
		return one
	case m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14):
		return few
	}
	return many
}

// actsHP is the share of all his health one bar holds: all of it is split
// between his acts, and acts other than the advice make all of it
// (acts/advice) to the power KingActHPPow.
func (g *Game) actsHP() float64 {
	acts := float64(g.kingActs())
	advice := acts
	if g.actsAdvice > 0 {
		advice = float64(g.actsAdvice)
	}
	return math.Pow(acts/advice, g.Cfg.KingActHPPow) / acts
}
