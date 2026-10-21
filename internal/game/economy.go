package game

import (
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
)

// RewardSpec maps a channel point reward, by its title, to a game effect.
type RewardSpec struct {
	Title  string `json:"title"`
	Action string `json:"action"` // gold | repair | strike | round
	Amount int    `json:"amount"` // gold, wall HP, or % of max HP for a strike
	// PerUser caps how many times one viewer can take the reward in a game;
	// 0 means no cap.
	PerUser int `json:"perUser,omitempty"`
}

// Donation is one donation from DonationAlerts.
type Donation struct {
	Name     string // free text the donor typed, not a Twitch login
	Message  string
	Amount   float64
	Currency string
}

// payTo credits gold to a chatter, creating one for a supporter who never
// chatted, and returns what reached the wallet.
func (g *Game) payTo(login, display string, gold int) (*Chatter, int) {
	key := strings.ToLower(strings.TrimSpace(login))
	if key == "" || g.ignored(key) {
		g.toTreasury(gold)
		return nil, 0
	}
	if display == "" {
		// Keep the display name we know; a login is only a fallback.
		display = login
		if known := g.Chatters[key]; known != nil {
			display = known.Name
		}
	}
	c := g.chatter(key, display)
	return c, g.addGold(c, gold)
}

// toTreasury puts gold into the common treasury, which repairs the walls.
func (g *Game) toTreasury(gold int) {
	if gold > 0 {
		g.Treasury += gold
	}
}

// tickTreasury spends the treasury on wall repairs while the walls are hurt.
func (g *Game) tickTreasury(dt float64) {
	missing := g.Cfg.WallsHP - g.Walls
	if g.Treasury <= 0 || missing <= 0 || g.Cfg.OverflowWallsPerGold <= 0 {
		return
	}
	g.treasuryAcc += g.Cfg.TreasuryGoldPerSec * dt
	spend := min(int(g.treasuryAcc), g.Treasury, int(math.Ceil(missing/g.Cfg.OverflowWallsPerGold)))
	if spend <= 0 {
		return
	}
	g.treasuryAcc -= float64(spend)
	g.Treasury -= spend
	g.Walls = min(g.Cfg.WallsHP, g.Walls+float64(spend)*g.Cfg.OverflowWallsPerGold)
}

// Reward handles a channel point redemption. It reports refused when the
// viewer has used up the reward's PerUser cap for this game, so the caller
// can give the points back.
func (g *Game) Reward(login, display, title string) (refused bool) {
	var r *RewardSpec
	for i := range g.Cfg.Rewards {
		if strings.EqualFold(strings.TrimSpace(g.Cfg.Rewards[i].Title), strings.TrimSpace(title)) {
			r = &g.Cfg.Rewards[i]
		}
	}
	if r == nil {
		return false // someone else's reward, not ours
	}
	if r.PerUser > 0 {
		key := strings.ToLower(r.Title) + "|" + strings.ToLower(login)
		if g.RewardUses[key] >= r.PerUser {
			g.reply(display, fmt.Sprintf("«%s» — не больше %d за игру, баллы вернутся.", r.Title, r.PerUser))
			return true
		}
		if g.RewardUses == nil {
			g.RewardUses = map[string]int{}
		}
		g.RewardUses[key]++
	}
	ev := Event{Kind: "reward", User: display, Text: r.Title}
	switch r.Action {
	case "gold":
		_, ev.Amount = g.payTo(login, display, r.Amount)
	case "repair":
		g.Walls = min(g.Cfg.WallsHP, g.Walls+float64(r.Amount))
		ev.Amount = r.Amount
	case "strike":
		// A volley from the sky: a share of every enemy's max HP, a fifth
		// of that for bosses so a reward never skips a boss fight.
		share := float64(r.Amount) / 100
		owner := strings.ToLower(login)
		for _, e := range g.Enemies {
			s := share
			if g.Cfg.Enemies[e.Kind].Boss {
				s /= 5
			}
			g.damage(e, e.MaxHP*s, owner, true)
		}
		g.sweepDead()
		g.emit(Event{Kind: "strike"})
	case "round":
		// The redeemer buys a round: everyone in the tavern gets a random
		// drink, effect and all.
		cutoff := g.Now().Add(-activeWindow(g.Cfg))
		keys := slices.Sorted(maps.Keys(g.Chatters))
		for _, key := range keys {
			c := g.Chatters[key]
			if key != StreamerName && c.Joined && c.LastSeen.After(cutoff) && g.Workers[key] == nil {
				g.serve(key, c, drinkKinds[g.rng.IntN(len(drinkKinds))])
			}
		}
		g.reply("", fmt.Sprintf("%s угощает таверну: каждому случайный напиток!", display))
	default:
		return false
	}
	g.emit(ev)
	return false
}

// Bits pays for a cheer.
func (g *Game) Bits(login, display string, bits int) {
	if bits <= 0 {
		return
	}
	_, got := g.payTo(login, display, int(float64(bits)*g.Cfg.BitsGold))
	g.emit(Event{Kind: "support", User: display, Text: fmt.Sprintf("%d битс", bits), Amount: got})
}

// Sub pays for a subscription or a resubscription.
func (g *Game) Sub(login, display string) {
	_, got := g.payTo(login, display, g.Cfg.SubGold)
	g.emit(Event{Kind: "support", User: display, Text: "подписка", Amount: got})
}

// GiftSub pays both the gifter and the lucky recipient.
func (g *Game) GiftSub(login, display, toLogin, toDisplay string) {
	_, got := g.payTo(login, display, g.Cfg.SubGold)
	g.emit(Event{Kind: "support", User: display, Text: "подарил подписку " + toDisplay, Amount: got})
	if toLogin != "" {
		g.payTo(toLogin, toDisplay, g.Cfg.GiftRecipientGold)
	}
}

var linkCode = regexp.MustCompile(`(?i)\bTD-?(\d{4})\b`)

// cmdLink gives a chatter a code to put into a donation message, so that
// donations under any name reach the right wallet from then on.
func (g *Game) cmdLink(c *Chatter, key string) {
	for code, login := range g.LinkCodes {
		if login == key {
			delete(g.LinkCodes, code)
		}
	}
	code := ""
	for code == "" || g.LinkCodes[code] != "" {
		code = fmt.Sprintf("%04d", g.rng.IntN(10000))
	}
	g.LinkCodes[code] = key
	g.reply(c.Name, fmt.Sprintf("добавь в сообщение доната код TD-%s, и золото будет приходить тебе", code))
}

// resolveDonor finds the chatter behind a donation: a link code in the
// message first, then a name linked before, then a nick equal to the name.
func (g *Game) resolveDonor(d Donation) string {
	name := strings.ToLower(strings.TrimSpace(d.Name))
	if m := linkCode.FindStringSubmatch(d.Message); m != nil {
		if login := g.LinkCodes[m[1]]; login != "" {
			delete(g.LinkCodes, m[1])
			if name != "" {
				g.Links[name] = login
			}
			return login
		}
	}
	if login := g.Links[name]; login != "" {
		return login
	}
	if _, ok := g.Chatters[name]; ok && name != StreamerName {
		return name
	}
	for key, c := range g.Chatters {
		if key != StreamerName && strings.EqualFold(c.Name, name) {
			return key
		}
	}
	return ""
}

// Donate turns a donation into gold for the donor's wallet, or into the
// common treasury when the donor cannot be matched to a chatter.
func (g *Game) Donate(d Donation) {
	rate, ok := g.Cfg.CurrencyRub[strings.ToUpper(d.Currency)]
	if !ok {
		rate = 1 // unknown currency: count it as roubles rather than drop it
	}
	gold := int(math.Round(d.Amount * rate * g.Cfg.DonationGoldPerRub))
	ev := Event{Kind: "donation", User: d.Name, Text: fmt.Sprintf("%g %s", d.Amount, strings.ToUpper(d.Currency))}
	if login := g.resolveDonor(d); login != "" {
		c, got := g.payTo(login, "", gold)
		ev.User, ev.Amount = c.Name, got
		g.reply(c.Name, fmt.Sprintf("спасибо за донат! +%d золота", got))
	} else {
		g.toTreasury(gold)
		ev.Text += " · в общую казну"
		ev.Amount = gold
	}
	g.emit(ev)
}

// cmdTreasury lets the streamer hand treasury gold to a chatter.
func (g *Game) cmdTreasury(args []string) {
	if len(args) != 2 {
		return
	}
	var n int
	if _, err := fmt.Sscan(args[1], &n); err != nil || n <= 0 {
		return
	}
	n = min(n, g.Treasury)
	g.Treasury -= n
	name := strings.TrimPrefix(args[0], "@")
	g.payTo(name, name, n) // what does not fit the wallet flows back to the treasury
}

// sweepAFK sends the gold of mercenaries who went quiet into the treasury:
// a chatter who talked this session and then fell silent for the active
// window (10 minutes) has left the game, and the gold keeps helping the
// walls. Mercenaries out on a job or sitting at the dice wait until done.
func (g *Game) sweepAFK(dt float64) {
	g.afkCheck -= dt
	if g.afkCheck > 0 {
		return
	}
	g.afkCheck = 5
	cut := g.Now().Add(-activeWindow(g.Cfg))
	for _, key := range slices.Sorted(maps.Keys(g.Chatters)) {
		c := g.Chatters[key]
		if !c.wasActive || !c.Joined || key == StreamerName || c.LastSeen.After(cut) || g.Workers[key] != nil || g.atDice(key) != nil {
			continue
		}
		c.wasActive, c.Joined, c.Grant = false, false, 0 // gone: back in with !play
		if c.Gold <= 0 {
			continue
		}
		gold := c.Gold
		c.Gold = 0
		g.toTreasury(gold)
		g.emit(Event{Kind: "afk", User: c.Name, Amount: gold})
	}
}
