package game

import (
	"fmt"
	"strconv"
	"strings"
)

// ChatMessage is one line from chat (or the streamer panel).
type ChatMessage struct {
	User string
	Text string
	Mod  bool // moderator or broadcaster: may run !td* admin commands
}

var towerAliases = map[string]string{
	"arrow": "arrow", "a": "arrow", "лук": "arrow", "лучник": "arrow", "стрелы": "arrow", "баллиста": "arrow",
	"magic": "magic", "m": "magic", "маг": "magic", "обелиск": "magic", "магия": "magic",
}

// HandleChat registers activity and executes a command if the line is one.
func (g *Game) HandleChat(m ChatMessage) {
	user := strings.TrimSpace(m.User)
	if user == "" {
		return
	}
	c := g.Touch(user)
	key := strings.ToLower(user)
	f := strings.Fields(m.Text)
	if len(f) == 0 || !strings.HasPrefix(f[0], "!") {
		return
	}
	args := f[1:]
	switch strings.ToLower(f[0]) {
	case "!build", "!b", "!строить", "!с":
		g.cmdBuild(c, key, args)
	case "!up", "!u", "!улучшить":
		g.cmdUpgrade(c, key, args)
	case "!sell", "!продать":
		g.cmdSell(c, key, args)
	case "!me", "!я":
		g.reply(c.Name, fmt.Sprintf("золото %d/%d, построек %d/%d, убийств %d",
			c.Gold, g.Cfg.WalletCap, g.towersOf(key), g.Cfg.TowerLimit, c.Kills))
		g.emit(Event{Kind: "card", User: c.Name})
	case "!towers", "!башни":
		var parts []string
		for _, k := range []string{"arrow", "magic"} {
			s := g.Cfg.Towers[k]
			parts = append(parts, fmt.Sprintf("%s (%s) — %d", s.Name, k, s.Cost))
		}
		g.reply(c.Name, strings.Join(parts, ", "))
	default:
		if m.Mod {
			g.cmdAdmin(c, strings.ToLower(f[0]), args)
		}
	}
}

func (g *Game) reply(user, text string) {
	g.emit(Event{Kind: "reply", User: user, Text: text})
}

func (g *Game) isStreamer(key string) bool { return key == StreamerName }

// cellArg joins "C 4" style arguments back together and returns the rest.
func cellArg(args []string) (Pos, []string, bool) {
	if len(args) == 0 {
		return Pos{}, nil, false
	}
	if p, ok := ParseCell(args[0]); ok {
		return p, args[1:], true
	}
	if len(args) >= 2 {
		if p, ok := ParseCell(args[0] + args[1]); ok {
			return p, args[2:], true
		}
	}
	return Pos{}, nil, false
}

func (g *Game) cmdBuild(c *Chatter, key string, args []string) {
	p, rest, ok := cellArg(args)
	if !ok {
		g.reply(c.Name, "формат: !build C4 arrow")
		return
	}
	kind := "arrow"
	if len(rest) > 0 {
		k, ok := towerAliases[strings.ToLower(rest[0])]
		if !ok {
			g.reply(c.Name, "неизвестная постройка, список: !towers")
			return
		}
		kind = k
	}
	if g.Phase == PhaseGameOver {
		return
	}
	switch {
	case g.Map.Grid.At(p) != CellBuild:
		g.reply(c.Name, p.Name()+": здесь строить нельзя")
		return
	case g.Towers[p] != nil:
		g.reply(c.Name, p.Name()+": клетка занята")
		return
	case !g.isStreamer(key) && g.towersOf(key) >= g.Cfg.TowerLimit:
		g.reply(c.Name, fmt.Sprintf("лимит построек %d", g.Cfg.TowerLimit))
		return
	}
	cost := g.Cfg.TowerCost(kind, 1)
	if !g.isStreamer(key) {
		if c.Gold < cost {
			g.reply(c.Name, fmt.Sprintf("нужно %d золота, у тебя %d", cost, c.Gold))
			return
		}
		c.Gold -= cost
	}
	t := &Tower{ID: g.id(), Kind: kind, Cell: p, Owner: key, Level: 1, Invested: cost}
	g.Towers[p] = t
	cp := center(p)
	g.emit(Event{Kind: "build", User: c.Name, X: cp.X, Y: cp.Y, Amount: -cost, Text: kind})
}

func (g *Game) ownTower(c *Chatter, key string, args []string) *Tower {
	p, _, ok := cellArg(args)
	if !ok {
		g.reply(c.Name, "укажи клетку, например C4")
		return nil
	}
	t := g.Towers[p]
	if t == nil || (t.Owner != key && !g.isStreamer(key)) {
		g.reply(c.Name, p.Name()+": здесь нет твоей постройки")
		return nil
	}
	return t
}

func (g *Game) cmdUpgrade(c *Chatter, key string, args []string) {
	t := g.ownTower(c, key, args)
	if t == nil {
		return
	}
	spec := g.Cfg.Towers[t.Kind]
	if t.Level >= spec.MaxLevel {
		g.reply(c.Name, "уже максимальный уровень")
		return
	}
	cost := g.Cfg.TowerCost(t.Kind, t.Level+1)
	if !g.isStreamer(key) {
		if c.Gold < cost {
			g.reply(c.Name, fmt.Sprintf("нужно %d золота, у тебя %d", cost, c.Gold))
			return
		}
		c.Gold -= cost
	}
	t.Level++
	t.Invested += cost
	cp := center(t.Cell)
	g.emit(Event{Kind: "upgrade", User: c.Name, X: cp.X, Y: cp.Y, Amount: -cost})
}

func (g *Game) cmdSell(c *Chatter, key string, args []string) {
	t := g.ownTower(c, key, args)
	if t == nil {
		return
	}
	delete(g.Towers, t.Cell)
	refund := t.Invested * g.Cfg.SellRefundPct / 100
	if owner := g.Chatters[t.Owner]; owner != nil && !g.isStreamer(t.Owner) {
		refund = g.addGold(owner, refund)
	}
	cp := center(t.Cell)
	g.emit(Event{Kind: "sell", User: c.Name, X: cp.X, Y: cp.Y, Amount: refund})
}

// cmdAdmin handles !td* commands for the streamer and moderators.
func (g *Game) cmdAdmin(c *Chatter, cmd string, args []string) {
	switch cmd {
	case "!tdwave":
		g.StartWave()
	case "!tdnew":
		biome := Biome("")
		if len(args) > 0 {
			biome = Biome(strings.ToLower(args[0]))
			if _, ok := profiles[biome]; !ok {
				biome = ""
			}
		}
		g.NewGame(biome)
	case "!tdpause":
		g.Paused = !g.Paused
	case "!tdremove":
		if p, _, ok := cellArg(args); ok {
			delete(g.Towers, p)
		}
	case "!tdgold":
		if len(args) == 2 {
			if n, err := strconv.Atoi(args[1]); err == nil {
				target := g.Touch(strings.TrimPrefix(args[0], "@"))
				g.addGold(target, n)
			}
		}
	}
}
