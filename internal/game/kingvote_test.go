package game

import (
	"fmt"
	"strings"
	"testing"
)

// voteGame is a build phase a minute before the final with n players.
func voteGame(t *testing.T, n int) *Game {
	t.Helper()
	g, _ := newTestGame(t)
	for i := 0; i < n; i++ {
		g.HandleChat(ChatMessage{User: fmt.Sprintf("p%d", i), Text: "!play"})
	}
	g.matchElapsed = g.Cfg.MatchMinutes*60 - 60
	return g
}

func TestActsRangeFollowsPlayers(t *testing.T) {
	g, _ := newTestGame(t)
	for _, c := range []struct{ n, advice, lo, hi int }{
		{1, 1, 1, 2}, {2, 1, 1, 2}, {3, 2, 1, 3}, {4, 2, 1, 3}, {5, 3, 2, 4}, {6, 3, 2, 4}, {10, 5, 4, 6},
	} {
		if a, lo, hi := g.actsRange(c.n); a != c.advice || lo != c.lo || hi != c.hi {
			t.Errorf("%d players: %d %d–%d, want %d %d–%d", c.n, a, lo, hi, c.advice, c.lo, c.hi)
		}
	}
}

func TestActVoteOpensBeforeTheFinal(t *testing.T) {
	g := voteGame(t, 5)
	g.matchElapsed = 0
	g.HandleChat(ChatMessage{User: "p0", Text: "!акты 4"})
	if len(g.actVotes) != 0 || !strings.Contains(lastReply(g), "откроется") {
		t.Fatal("no vote an hour before the final")
	}
	g.matchElapsed = g.Cfg.MatchMinutes*60 - 60
	g.HandleChat(ChatMessage{User: "p0", Text: "!акты 7"})
	if len(g.actVotes) != 0 || !strings.Contains(lastReply(g), "!акты 2–4") {
		t.Fatalf("five players pick 2–4: %q", lastReply(g))
	}
	g.HandleChat(ChatMessage{User: "p0", Text: "!акты 4"})
	if g.actVotes["p0"] != 4 {
		t.Fatal("a vote in range counts")
	}
	if v := g.StateMessage().ActVote; v == nil || v.Now != 4 || v.Advice != 3 || v.Votes != 1 {
		t.Fatalf("the overlay shows the vote: %+v", v)
	}
}

func TestMedianOfVotesPicksTheActs(t *testing.T) {
	g := voteGame(t, 5)
	for i, n := range []int{2, 4, 4} {
		g.HandleChat(ChatMessage{User: fmt.Sprintf("p%d", i), Text: fmt.Sprintf("!акты %d", n)})
	}
	g.finalDue = true
	g.StartWave()
	if g.acts != 4 || g.kingActs() != 4 || g.actsAdvice != 3 {
		t.Fatalf("the median wins: %d of advice %d", g.acts, g.actsAdvice)
	}
	if g.HandleChat(ChatMessage{User: "p3", Text: "!акты 2"}); g.acts != 4 {
		t.Fatal("the vote is closed once the final starts")
	}
	if s := g.actsStake(); s != 1+g.Cfg.KingActSouls {
		t.Fatalf("an act above the advice pays more: %.2f", s)
	}
}

func TestNoVotesPlaysTheAdvice(t *testing.T) {
	g := voteGame(t, 3)
	g.finalDue = true
	g.StartWave()
	if g.acts != 2 || g.actsStake() != 1 {
		t.Fatalf("three players without a vote: %d acts", g.acts)
	}
}

func TestFixedActsTurnTheVoteOff(t *testing.T) {
	g := voteGame(t, 5)
	g.Cfg.KingActs = 2
	g.HandleChat(ChatMessage{User: "p0", Text: "!акты 4"})
	g.finalDue = true
	g.StartWave()
	if g.kingActs() != 2 || len(g.actVotes) != 0 {
		t.Fatal("kingActs fixes the acts")
	}
}

func TestKingHealthGrowsAboveAFullTeam(t *testing.T) {
	g, _ := newTestGame(t)
	g.finalTeam = 5
	five := g.kingTeamMult()
	g.finalTeam = 10
	ten := g.kingTeamMult()
	if five != 1 || ten < 1.6 || ten > 2 {
		t.Fatalf("ten players meet a king close to twice as strong: %.2f", ten)
	}
}
