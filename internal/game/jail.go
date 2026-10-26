package game

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// The tavern's small pleasures and its lock-up: a line over the head (!s),
// a dance on the table (!dance), and the jail the players vote a nuisance
// into (!тюрьма). An inmate's buildings keep fighting, but his class skills
// stop, he cannot build or drink, and he hears only !s and !выкуп until he
// pays a ransom into the treasury or serves his term.

// sayMax is how many letters of a line show over the mercenary's head.
const sayMax = 40

// JailVote is the tavern deciding whether to lock a player up.
type JailVote struct {
	Target     string // login
	TargetName string
	ByName     string
	left       float64
	yes, no    map[string]bool
}

// jailFree are the commands an inmate still has.
var jailFree = map[string]bool{
	"!s": true, "!say": true, "!скажи": true,
	"!выкуп": true, "!ransom": true, "!bail": true,
}

// cmdSay shows a line over the mercenary's head, with its emotes as
// pictures. An emote counts as two letters toward the limit, so a line of
// emotes stays short too.
func (g *Game) cmdSay(c *Chatter, key string, args []string, emotes map[string]string) {
	if len(args) == 0 || c.out > 0 {
		return
	}
	if !c.Joined {
		if !c.arrived {
			if c.snored <= 0 {
				c.snored = 20
				g.reply(c.Name, "реплика появится над твоим наёмником: сначала зайди в таверну, !play")
			}
			return
		}
		// Played this game already (a restart forgets who sits in the
		// tavern): back to the seat, quietly, and the line goes up.
		g.join(key, c, "!s")
	}
	now := g.Now()
	if now.Sub(c.saidAt) < time.Duration(g.Cfg.SayCdSec*float64(time.Second)) {
		return
	}
	c.saidAt = now
	var words []string
	used := map[string]string{}
	room := sayMax
	for _, w := range args {
		cost := utf8.RuneCountInString(w) + 1
		if url, ok := emotes[w]; ok {
			cost = 3
			if room >= cost {
				used[w] = url
			}
		}
		if room < cost {
			if _, emote := emotes[w]; !emote && room > 2 {
				words = append(words, string([]rune(w)[:room-2])) // a long word is cut
			}
			words = append(words, "…")
			break
		}
		words = append(words, w)
		room -= cost
	}
	if len(used) == 0 {
		used = nil
	}
	g.emit(Event{Kind: "say", User: c.Name, Text: strings.Join(words, " "), Emotes: used})
}

func (g *Game) cmdDance(c *Chatter, key string) {
	if g.busyAtDice(c, key) {
		return
	}
	if g.Workers[key] != nil || g.panther(key) != nil || g.runOf(key) != nil {
		g.reply(c.Name, "твой наёмник сейчас в поле")
		return
	}
	now := g.Now()
	if wait := time.Duration(g.Cfg.DanceCdSec*float64(time.Second)) - now.Sub(c.dancedAt); wait > 0 {
		g.reply(c.Name, fmt.Sprintf("ноги ещё гудят, отдохни %d с", int(math.Ceil(wait.Seconds()))))
		return
	}
	c.dancedAt = now
	g.emit(Event{Kind: "dance", User: c.Name, Amount: int(g.Cfg.DanceSec)})
}

// jailed reports whether c sits behind bars.
func (c *Chatter) jailed() bool { return c.jailLeft > 0 }

// inJail tells an inmate (now and then) that only !s and !выкуп work.
func (g *Game) inJail(c *Chatter) {
	if c.snored > 0 {
		return
	}
	c.snored = 20
	g.reply(c.Name, fmt.Sprintf("ты за решёткой ещё %s. Выкуп в казну: !выкуп (%d з), сказать: !s текст", mmss(c.jailLeft), c.ransom))
}

func (g *Game) cmdJail(c *Chatter, key string, args []string) {
	if len(args) == 0 {
		g.reply(c.Name, "кого сажаем? !тюрьма ник — таверна проголосует !за или !против")
		return
	}
	if g.jailVote != nil {
		g.reply(c.Name, fmt.Sprintf("уже голосуем за %s: !за или !против", g.jailVote.TargetName))
		return
	}
	if g.mapVote != nil {
		g.reply(c.Name, "сначала дождись конца голосования за новую карту")
		return
	}
	who, t := g.findChatter(args[0])
	switch {
	case t == nil || !t.Joined || who == StreamerName:
		g.reply(c.Name, "такого наёмника в таверне нет")
	case who == key:
		g.reply(c.Name, "сам себя за решётку не сажают")
	case t.jailed():
		g.reply(c.Name, t.Name+" уже за решёткой")
	case t.jailedOnce:
		g.reply(c.Name, t.Name+" уже отсидел в этой игре, второй раз не сажают")
	case g.Now().Before(t.jailVoteAt):
		g.reply(c.Name, fmt.Sprintf("за %s недавно голосовали, подожди %d с", t.Name, int(math.Ceil(t.jailVoteAt.Sub(g.Now()).Seconds()))))
	default:
		g.jailVote = &JailVote{Target: who, TargetName: t.Name, ByName: c.Name, left: g.Cfg.JailVoteSec,
			yes: map[string]bool{key: true}, no: map[string]bool{}}
		g.emit(Event{Kind: "jailVote", User: t.Name, Text: c.Name, Amount: int(g.Cfg.JailVoteSec)})
		g.reply("", fmt.Sprintf("%s предлагает посадить %s за решётку! Голосуйте !за или !против, %d с", c.Name, t.Name, int(g.Cfg.JailVoteSec)))
	}
}

func (g *Game) cmdVote(c *Chatter, key string, yes bool) {
	if g.mapVote != nil {
		g.voteNewMap(c, key, yes)
		return
	}
	v := g.jailVote
	if v == nil || !c.Joined || key == v.Target {
		return
	}
	delete(v.yes, key)
	delete(v.no, key)
	if yes {
		v.yes[key] = true
	} else {
		v.no[key] = true
	}
	if len(v.yes)+len(v.no) >= len(g.jailVoters(v)) {
		g.endJailVote() // everyone has had a say
	}
}

// jailVoters are the players who may vote on v: everyone in the tavern but
// the accused.
func (g *Game) jailVoters(v *JailVote) []string {
	cut := g.Now().Add(-activeWindow(g.Cfg))
	var out []string
	for k, c := range g.Chatters {
		if k != v.Target && c.Joined && c.LastSeen.After(cut) && !c.jailed() {
			out = append(out, k)
		}
	}
	return out
}

func (g *Game) endJailVote() {
	v := g.jailVote
	g.jailVote = nil
	t := g.Chatters[v.Target]
	yes, no := len(v.yes), len(v.no)
	if t == nil || !t.Joined {
		return
	}
	if yes < g.Cfg.JailMinYes || yes <= no {
		t.jailVoteAt = g.Now().Add(time.Duration(g.Cfg.JailRetrySec * float64(time.Second)))
		g.emit(Event{Kind: "jailSpared", User: t.Name})
		g.reply("", fmt.Sprintf("%s остаётся на свободе: за %d, против %d", t.Name, yes, no))
		return
	}
	g.lockUp(v.Target, t)
	g.reply("", fmt.Sprintf("таверна решила: %s за решёткой на %s (за %d, против %d). Постройки стреляют, способности и стройка — нет. Выкуп в казну: !выкуп, %d з",
		t.Name, mmss(t.jailLeft), yes, no, t.ransom))
}

// lockUp puts a player behind bars: his skills and his music stop, a game
// of dice he sits at is called off with the stakes returned.
func (g *Game) lockUp(key string, c *Chatter) {
	c.jailLeft = g.Cfg.JailSec
	c.jailedOnce = true
	c.ransom = g.ransomOf(key, c)
	c.musicLeft = 0
	if d := g.atDice(key); d != nil {
		g.endDice(d)
		g.refundDice(d)
		g.emit(Event{Kind: "diceCancel", User: d.HostName})
	}
	g.emit(Event{Kind: "jailed", User: c.Name, Amount: c.ransom})
}

// ransomOf is what freedom costs, and it is dearer the less the player has
// given the game: whoever built his fair share (as the difficulty weighs it)
// pays JailRansomMin, whoever built nothing JailRansomPct of the others'
// median investment, at most half a wallet. Only the wallet pays, never
// the grant.
func (g *Game) ransomOf(key string, c *Chatter) int {
	inv := g.investedBy()
	freeloader := max(float64(g.Cfg.JailRansomMin), g.Cfg.JailRansomPct*g.othersMedian(inv, key))
	r := float64(g.Cfg.JailRansomMin) + (freeloader-float64(g.Cfg.JailRansomMin))*(1-g.contribution(inv, key))
	return min(int(math.Round(r)), max(g.Cfg.WalletCap/2, g.Cfg.JailRansomMin))
}

func (g *Game) cmdRansom(c *Chatter) {
	if !c.jailed() {
		g.reply(c.Name, "ты и так на свободе")
		return
	}
	if c.Gold < c.ransom {
		g.reply(c.Name, fmt.Sprintf("выкуп %d з, у тебя %d. Сидеть ещё %s", c.ransom, c.Gold, mmss(c.jailLeft)))
		return
	}
	c.Gold -= c.ransom
	g.toTreasury(c.ransom)
	g.free(c, true)
	g.reply(c.Name, fmt.Sprintf("выкупился за %d з, золото ушло в казну", c.ransom))
}

// free lets an inmate out, ransomed or with his term served.
func (g *Game) free(c *Chatter, ransomed bool) {
	c.jailLeft = 0
	amount := 0
	if ransomed {
		amount = c.ransom
	}
	g.emit(Event{Kind: "jailFree", User: c.Name, Amount: amount})
}

// tickJail runs the vote and the inmates' terms.
func (g *Game) tickJail(dt float64) {
	if v := g.jailVote; v != nil {
		if v.left -= dt; v.left <= 0 {
			g.endJailVote()
		}
	}
	for _, key := range slices.Sorted(maps.Keys(g.Chatters)) {
		c := g.Chatters[key]
		if c.jailed() {
			if c.jailLeft -= dt; c.jailLeft <= 0 {
				g.free(c, false)
				g.reply(c.Name, "срок вышел, ты на свободе")
			}
		}
	}
}

// cmdFree is the streamer's pardon: !tdfree nick.
func (g *Game) cmdFree(c *Chatter, args []string) {
	if len(args) == 0 {
		return
	}
	_, t := g.findChatter(args[0])
	if t == nil || !t.jailed() {
		g.reply(c.Name, "этот наёмник не за решёткой")
		return
	}
	g.free(t, false)
	g.reply("", t.Name+" помилован и выходит на свободу")
}

// mmss writes seconds as 2:05.
func mmss(sec float64) string {
	s := int(math.Ceil(sec))
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
