package game

import (
	"fmt"
	"strconv"
	"strings"
)

// DiceGame is a game of dice at one of the tavern's dice tables. The host
// puts a stake on the table and waits; a guest accepts it or raises; both
// roll a D20 and the higher roll takes the whole pot. Stakes leave the
// wallets at once, and players at the table cannot build until it is over.
type DiceGame struct {
	ID         int
	Table      int
	Host       string // login
	HostName   string
	Stake      int
	Guest      string
	GuestName  string
	GuestStake int
	wait       float64 // seconds left for someone to sit down
	roll       float64 // seconds of suspense before the dice are thrown
}

func (d *DiceGame) pot() int { return d.Stake + d.GuestStake }

// atDice reports whether the chatter sits at a dice table.
func (g *Game) atDice(key string) *DiceGame {
	for _, d := range g.Dice {
		if d.Host == key || d.Guest == key {
			return d
		}
	}
	return nil
}

// busyAtDice tells a chatter at a dice table that this has to wait.
func (g *Game) busyAtDice(c *Chatter, key string) bool {
	d := g.atDice(key)
	if d == nil {
		return false
	}
	if d.Guest == "" {
		g.reply(c.Name, "ты за столом для костей; встать: !кости отмена")
	} else {
		g.reply(c.Name, "сначала доиграй в кости")
	}
	return true
}

func (g *Game) freeDiceTable() int {
	used := map[int]bool{}
	for _, d := range g.Dice {
		used[d.Table] = true
	}
	for t := 0; t < g.Cfg.DiceTables; t++ {
		if !used[t] {
			return t
		}
	}
	return -1
}

var diceHelp = "кости: !кости 100 — сесть со ставкой, !кости принять [@ник], !кости перебить 150 [@ник], !кости отмена"

func (g *Game) cmdDice(c *Chatter, key string, args []string) {
	if len(args) == 0 {
		g.reply(c.Name, diceHelp+g.openDiceList())
		return
	}
	switch strings.ToLower(args[0]) {
	case "отмена", "cancel", "отменить":
		g.diceCancel(c, key)
	case "принять", "accept", "да":
		g.diceJoin(c, key, args[1:], 0)
	case "перебить", "raise":
		if len(args) < 2 {
			g.reply(c.Name, "формат: !кости перебить 150 [@ник]")
			return
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			g.reply(c.Name, "ставка — число золота")
			return
		}
		g.diceJoin(c, key, args[2:], n)
	default:
		n, err := strconv.Atoi(args[0])
		if err != nil {
			g.reply(c.Name, diceHelp)
			return
		}
		g.diceOpen(c, key, n)
	}
}

// diceReady checks what both sides need before sitting down with a stake.
func (g *Game) diceReady(c *Chatter, key string, stake int) bool {
	switch {
	case g.atDice(key) != nil:
		g.reply(c.Name, "ты уже сидишь за столом")
	case g.Workers[key] != nil:
		g.reply(c.Name, "твой наёмник сейчас в поле")
	case stake < g.Cfg.DiceMinStake:
		g.reply(c.Name, fmt.Sprintf("ставка от %d золота", g.Cfg.DiceMinStake))
	case c.Gold < stake:
		g.reply(c.Name, fmt.Sprintf("на ставку не хватает: у тебя %d", c.Gold))
	default:
		return true
	}
	return false
}

func (g *Game) diceOpen(c *Chatter, key string, stake int) {
	if !g.diceReady(c, key, stake) {
		return
	}
	table := g.freeDiceTable()
	if table < 0 {
		g.reply(c.Name, "все столы для костей заняты, подожди")
		return
	}
	c.Gold -= stake
	d := &DiceGame{ID: g.id(), Table: table, Host: key, HostName: c.Name, Stake: stake, wait: g.Cfg.DiceWaitSec}
	g.Dice = append(g.Dice, d)
	g.emit(Event{Kind: "diceOpen", User: c.Name, Amount: stake})
	g.reply(c.Name, fmt.Sprintf("сел за кости со ставкой %d. Сыграть: !кости принять @%s или !кости перебить N", stake, c.Name))
}

// diceJoin seats a guest. raise = 0 accepts the host's stake; a raise must
// be higher, and the pot simply grows: the winner still takes it all.
func (g *Game) diceJoin(c *Chatter, key string, args []string, raise int) {
	var d *DiceGame
	nick := ""
	if len(args) > 0 {
		nick = strings.ToLower(strings.TrimPrefix(args[0], "@"))
	}
	for _, o := range g.Dice {
		if o.Guest != "" || o.Host == key {
			continue
		}
		if nick == "" || o.Host == nick || strings.EqualFold(o.HostName, nick) {
			d = o
			break // the oldest open table
		}
	}
	if d == nil {
		g.reply(c.Name, "нет открытого стола"+g.openDiceList())
		return
	}
	stake := d.Stake
	if raise > 0 {
		if raise <= d.Stake {
			g.reply(c.Name, fmt.Sprintf("перебить — больше чем %d", d.Stake))
			return
		}
		stake = raise
	}
	if !g.diceReady(c, key, stake) {
		return
	}
	c.Gold -= stake
	d.Guest, d.GuestName, d.GuestStake = key, c.Name, stake
	d.roll = g.Cfg.DiceRollSec
	g.emit(Event{Kind: "diceJoin", User: c.Name, Text: d.HostName, Amount: stake})
}

func (g *Game) diceCancel(c *Chatter, key string) {
	d := g.atDice(key)
	switch {
	case d == nil:
		g.reply(c.Name, "ты не сидишь за столом")
	case d.Host != key || d.Guest != "":
		g.reply(c.Name, "кости уже брошены на стол, отменить нельзя")
	default:
		g.endDice(d)
		g.refundDice(d)
		g.emit(Event{Kind: "diceCancel", User: c.Name})
		g.reply(c.Name, fmt.Sprintf("встал из-за стола, ставка %d вернулась", d.Stake))
	}
}

func (g *Game) openDiceList() string {
	var open []string
	for _, d := range g.Dice {
		if d.Guest == "" {
			open = append(open, fmt.Sprintf("@%s %d", d.HostName, d.Stake))
		}
	}
	if len(open) == 0 {
		return ""
	}
	return ". Ждут соперника: " + strings.Join(open, ", ")
}

func (g *Game) endDice(d *DiceGame) {
	for i, o := range g.Dice {
		if o == d {
			g.Dice = append(g.Dice[:i], g.Dice[i+1:]...)
			return
		}
	}
}

// refundDice gives the stakes back, e.g. when nobody sat down in time.
func (g *Game) refundDice(d *DiceGame) {
	if c := g.Chatters[d.Host]; c != nil {
		c.Gold += d.Stake // the stake came from this wallet, so it fits
	}
	if c := g.Chatters[d.Guest]; c != nil {
		c.Gold += d.GuestStake
	}
}

func (g *Game) tickDice(dt float64) {
	for _, d := range append([]*DiceGame(nil), g.Dice...) {
		if d.Guest == "" {
			d.wait -= dt
			if d.wait <= 0 {
				g.endDice(d)
				g.refundDice(d)
				g.emit(Event{Kind: "diceCancel", User: d.HostName})
				g.reply(d.HostName, "никто не сел играть, ставка вернулась")
			}
			continue
		}
		d.roll -= dt
		if d.roll > 0 {
			continue
		}
		a, b := 1+g.rng.IntN(20), 1+g.rng.IntN(20)
		for a == b {
			a, b = 1+g.rng.IntN(20), 1+g.rng.IntN(20) // a draw is thrown again
		}
		winKey, winName := d.Host, d.HostName
		if b > a {
			winKey, winName = d.Guest, d.GuestName
		}
		got := 0
		if c := g.Chatters[winKey]; c != nil {
			got = g.addGold(c, d.pot()) // above the wallet cap it goes to the treasury
		}
		g.endDice(d)
		g.emit(Event{Kind: "diceRoll", User: winName, Amount: d.pot(),
			Text: fmt.Sprintf("%s:%d:%s:%d", d.HostName, a, d.GuestName, b)})
		msg := fmt.Sprintf("кости: %s — %d, %s — %d. %s забирает %d золота", d.HostName, a, d.GuestName, b, winName, d.pot())
		if got < d.pot() {
			msg += fmt.Sprintf(" (%d не влезло в кошелёк и ушло в казну)", d.pot()-got)
		}
		g.reply(winName, msg)
		g.diceAchievements(d, winKey, a, b)
	}
}

// diceEscrow is the gold lying on dice tables per chatter, so a save never
// loses a stake: it is counted back into the wallets.
func (g *Game) diceEscrow() map[string]int {
	m := map[string]int{}
	for _, d := range g.Dice {
		m[d.Host] += d.Stake
		if d.Guest != "" {
			m[d.Guest] += d.GuestStake
		}
	}
	return m
}

// diceAchievements tracks win streaks and the big and humiliating throws.
func (g *Game) diceAchievements(d *DiceGame, winKey string, a, b int) {
	loseKey, lost := d.Guest, b
	if winKey == d.Guest {
		loseKey, lost = d.Host, a
	}
	if c := g.Chatters[winKey]; c != nil {
		c.diceStreak++
		if c.diceStreak >= streakNeeded {
			g.unlock(winKey, "lucky")
		}
	}
	if c := g.Chatters[loseKey]; c != nil {
		c.diceStreak = 0
	}
	if d.pot() >= bigPot {
		g.unlock(winKey, "high_roller")
	}
	if lost == 1 && max(a, b) == 20 {
		g.unlock(loseKey, "nat1")
	}
}
