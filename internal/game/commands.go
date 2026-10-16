package game

import (
	"fmt"
	"strconv"
	"strings"
)

// ChatMessage is one line from chat (or the streamer panel).
type ChatMessage struct {
	Login       string // stable Twitch login; falls back to User when empty
	User        string // display name, may contain any script
	Text        string
	Mod         bool // moderator or broadcaster: may run !td* admin commands
	Broadcaster bool // the streamer: builds instantly and without limits
}

var towerAliases = map[string]string{
	"arrow": "arrow", "a": "arrow", "лук": "arrow", "лучник": "arrow", "стрелы": "arrow", "баллиста": "arrow",
	"magic": "magic", "m": "magic", "маг": "magic", "обелиск": "magic", "магия": "magic",
	"fire": "fire", "f": "fire", "огонь": "fire", "жаровня": "fire", "пламя": "fire",
	"shrine": "shrine", "s": "shrine", "святилище": "shrine", "алтарь": "shrine",
	"trap": "trap", "t": "trap", "капкан": "trap", "капканы": "trap", "ловушка": "trap", "шипы": "trap",
	"drums": "drums", "d": "drums", "барабаны": "drums", "барабан": "drums",
	"wall": "wall", "w": "wall", "баррикада": "wall", "стена": "wall",
}

// HandleChat registers activity and executes a command if the line is one.
func (g *Game) HandleChat(m ChatMessage) {
	user := strings.TrimSpace(m.User)
	key := strings.ToLower(strings.TrimSpace(m.Login))
	if key == "" {
		key = strings.ToLower(user)
	}
	if user == "" {
		user = key
	}
	if key == "" || g.ignored(key) {
		return
	}
	if m.Broadcaster {
		key = StreamerName
	}
	c := g.Touch(key, user)
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
		cl, _ := classByKey(c.Class)
		g.reply(c.Name, fmt.Sprintf("%s, золото %d/%d, построек %d/%d, убийств %d",
			strings.ToLower(cl.Name), c.Gold, g.Cfg.WalletCap, g.towersOf(key), g.Cfg.TowerLimit, c.Kills))
		g.emit(Event{Kind: "card", User: c.Name, Amount: g.towersOf(key)})
	case "!link", "!привязать":
		g.cmdLink(c, key)
	case "!class", "!класс":
		g.cmdClass(c, args)
	case "!drink", "!d", "!выпить", "!пить":
		g.cmdDrink(c, key, args)
	case "!towers", "!башни":
		var parts []string
		for _, k := range TowerKinds {
			s := g.Cfg.Towers[k]
			where := ""
			if s.OnPath() {
				where = ", на дорогу"
			}
			parts = append(parts, fmt.Sprintf("%s (%s) %d%s", s.Name, k, s.Cost, where))
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
	if g.Banned[key] {
		g.reply(c.Name, "стример запретил тебе строить")
		return
	}
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
	spec := g.Cfg.Towers[kind]
	switch {
	case spec.OnPath() && g.Map.Grid.At(p) != CellPath:
		g.reply(c.Name, fmt.Sprintf("%s ставится только на дорогу", strings.ToLower(spec.Name)))
		return
	case !spec.OnPath() && g.Map.Grid.At(p) != CellBuild:
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
	streamer := g.isStreamer(key)
	if !streamer && !g.canQueue(key) {
		g.reply(c.Name, fmt.Sprintf("у наёмника уже %d приказа, подожди", g.Cfg.MaxOrders))
		return
	}
	if !streamer {
		if c.Gold < cost {
			g.reply(c.Name, fmt.Sprintf("нужно %d золота, у тебя %d", cost, c.Gold))
			return
		}
		c.Gold -= cost
	}
	// The streamer's towers appear at once; a chatter's mercenary walks out
	// and builds it, until then the tower is a construction site.
	t := &Tower{ID: g.id(), Kind: kind, Cell: p, Owner: key, Level: 1, Invested: cost, Built: streamer}
	g.Towers[p] = t
	if streamer {
		g.readyTower(t)
	} else {
		g.order(key, c.Name, p)
	}
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
	if g.Banned[key] {
		g.reply(c.Name, "стример запретил тебе строить")
		return
	}
	t := g.ownTower(c, key, args)
	if t == nil {
		return
	}
	spec := g.Cfg.Towers[t.Kind]
	switch {
	case t.Level >= spec.MaxLevel:
		g.reply(c.Name, "уже максимальный уровень")
		return
	case !t.Built || t.PendingCost > 0:
		g.reply(c.Name, t.Cell.Name()+": наёмник ещё работает над ней")
		return
	}
	streamer := g.isStreamer(key)
	if !streamer && !g.canQueue(key) {
		g.reply(c.Name, fmt.Sprintf("у наёмника уже %d приказа, подожди", g.Cfg.MaxOrders))
		return
	}
	cost := g.Cfg.TowerCost(t.Kind, t.Level+1)
	if !streamer {
		if c.Gold < cost {
			g.reply(c.Name, fmt.Sprintf("нужно %d золота, у тебя %d", cost, c.Gold))
			return
		}
		c.Gold -= cost
	}
	t.Invested += cost
	if streamer {
		t.Level++
		g.readyTower(t)
	} else {
		t.PendingCost = cost
		g.order(key, c.Name, t.Cell)
	}
	cp := center(t.Cell)
	g.emit(Event{Kind: "upgrade", User: c.Name, X: cp.X, Y: cp.Y, Amount: -cost})
}

func (g *Game) cmdSell(c *Chatter, key string, args []string) {
	t := g.ownTower(c, key, args)
	if t == nil {
		return
	}
	g.removeTower(t)
	// Unfinished work is refunded in full: nothing was built with that gold.
	refund := (t.Invested - t.PendingCost) * g.Cfg.SellRefundPct / 100
	if !t.Built {
		refund = t.Invested
	} else {
		refund += t.PendingCost
	}
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
			if t := g.Towers[p]; t != nil {
				g.removeTower(t)
			}
		}
	case "!tdspawn": // !tdspawn lich [n]: let the streamer summon enemies
		if len(args) == 0 {
			return
		}
		spec, ok := g.Cfg.Enemies[args[0]]
		if !ok || g.Phase == PhaseGameOver {
			return
		}
		n := 1
		if len(args) > 1 {
			n, _ = strconv.Atoi(args[1])
			n = min(max(n, 1), 20)
		}
		hp := g.waveHP
		if hp == 0 {
			hp = g.Cfg.BaseHP * g.Cfg.HPGrowth
		}
		for i := 0; i < n; i++ {
			if spec.Boss {
				g.spawnBoss(args[0], hp*g.Cfg.BossHPMult)
			} else {
				g.spawnEnemy(args[0], hp*spec.HPMult, 0.4*float64(i))
			}
		}
	case "!tdban", "!tdunban": // forbid or allow building for a chatter
		if len(args) == 1 {
			g.ban(args[0], cmd == "!tdban")
		}
	case "!tdtreasury": // !tdtreasury nick N: hand treasury gold to a chatter
		g.cmdTreasury(args)
	case "!tdwalls": // set the walls' hit points, e.g. to repair them by hand
		if len(args) == 1 {
			if n, err := strconv.Atoi(args[0]); err == nil {
				g.Walls = min(max(float64(n), 0), g.Cfg.WallsHP)
			}
		}
	case "!tdgold":
		if len(args) == 2 {
			if n, err := strconv.Atoi(args[1]); err == nil {
				name := strings.TrimPrefix(args[0], "@")
				g.addGold(g.chatter(strings.ToLower(name), name), n)
			}
		}
	}
}

func (g *Game) ignored(key string) bool {
	for _, u := range g.Cfg.IgnoreUsers {
		if strings.EqualFold(u, key) {
			return true
		}
	}
	return false
}
