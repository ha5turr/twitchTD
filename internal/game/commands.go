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
	Broadcaster bool // the streamer: builds instantly and without limits, unless StreamerPlays
}

var towerAliases = map[string]string{
	"arrow": "arrow", "a": "arrow", "лук": "arrow", "лучник": "arrow", "стрелы": "arrow", "баллиста": "arrow",
	"magic": "magic", "m": "magic", "маг": "magic", "обелиск": "magic", "магия": "magic",
	"fire": "fire", "f": "fire", "огонь": "fire", "жаровня": "fire", "пламя": "fire",
	"shrine": "shrine", "s": "shrine", "святилище": "shrine", "алтарь": "shrine",
	"trap": "trap", "t": "trap", "капкан": "trap", "капканы": "trap", "ловушка": "trap", "шипы": "trap",
	"drums": "drums", "d": "drums", "барабаны": "drums", "барабан": "drums",
	"wall": "wall", "w": "wall", "баррикада": "wall", "стена": "wall",
	"curse": "curse", "c": "curse", "проклятье": "curse", "проклятие": "curse", "проклятия": "curse",
	"totem": "totem", "тотем": "totem", "гроза": "totem",
	"clown": "clown", "клоун": "clown", "🤡": "clown",
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
	if m.Broadcaster && !g.Cfg.StreamerPlays {
		key = StreamerName
	}
	c := g.Touch(key, user)
	f := strings.Fields(m.Text)
	if len(f) == 0 || !strings.HasPrefix(f[0], "!") {
		return
	}
	args := f[1:]
	cmd := strings.ToLower(f[0])
	if (gameCommands[cmd] || cmd == "!leave" || cmd == "!выйти" || cmd == "!уйти") && g.asleep(c) {
		return // passed out under the table
	}
	if c.jailed() && !jailFree[cmd] && !(m.Mod && strings.HasPrefix(cmd, "!td")) {
		g.inJail(c) // behind bars: only a line over the head and the ransom
		return
	}
	if gameCommands[cmd] && key != StreamerName {
		g.join(key, c, cmd)
	}
	switch cmd {
	case "!play", "!играть", "!join":
		// join already welcomed them; a second !play just says so.
	case "!leave", "!выйти", "!уйти":
		g.cmdLeave(c, key)
	case "!build", "!b", "!строить", "!с":
		g.cmdBuild(c, key, args)
	case "!up", "!u", "!улучшить":
		g.cmdUpgrade(c, key, args)
	case "!repair", "!чинить", "!починить":
		g.cmdRepair(c, key, args)
	case "!sell", "!продать":
		g.cmdSell(c, key, args)
	case "!me", "!я":
		cl, _ := classByKey(c.Class)
		grant := ""
		if c.Grant > 0 {
			grant = fmt.Sprintf(" (+%d подъёмных)", c.Grant)
		}
		g.reply(c.Name, fmt.Sprintf("%s, %s, золото %d/%d%s, построек %d/%d, убийств %d, душ %d",
			Ranks[c.rank()].Name, strings.ToLower(cl.Name), c.Gold, g.Cfg.WalletCap, grant, g.towersOf(key), g.TowerLimit(), c.Kills, c.Souls))
		g.emit(Event{Kind: "card", User: c.Name, Amount: g.towersOf(key)})
	case "!link", "!привязать":
		g.cmdLink(c, key)
	case "!dice", "!кости", "!к":
		g.cmdDice(c, key, args)
	case "!class", "!класс":
		g.cmdClass(c, args)
	case "!drink", "!d", "!выпить", "!пить":
		g.cmdDrink(c, key, args)
	case "!помощь", "!help", "!команды", "!h":
		g.cmdHelp(c, args)
	case "!души", "!souls":
		g.cmdSouls(c)
	case "!ачивки", "!achievements", "!ach":
		g.cmdAchievements(c)
	case "!мемы", "!memes":
		g.cmdMemes(c, key, args)
	case "!стрелы", "!arrows", "!ammo":
		g.cmdAmmo(c, key, args)
	case "!s", "!say", "!скажи":
		g.cmdSay(c, args)
	case "!dance", "!танец", "!пляс", "!танцевать":
		g.cmdDance(c, key)
	case "!тюрьма", "!jail", "!посадить":
		g.cmdJail(c, key, args)
	case "!новаякарта", "!нк", "!newmap":
		g.cmdNewMap(c, key, m.Mod || m.Broadcaster, args)
	case "!за", "!yes":
		g.cmdVote(c, key, true)
	case "!против", "!no":
		g.cmdVote(c, key, false)
	case "!выкуп", "!ransom", "!bail":
		g.cmdRansom(c)
	case "!towers", "!башни":
		var parts []string
		for _, k := range TowerKinds {
			s := g.Cfg.Towers[k]
			if s.Only != "" && !g.sdfOut() {
				continue // the clown is a secret until his boss shows up
			}
			where := ""
			if s.OnPath() {
				where = ", на дорогу"
			}
			if s.Only != "" {
				where = ", только сейчас"
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
	if g.busyAtDice(c, key) {
		return
	}
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
		if !ok || k == clownKind && !g.sdfOut() { // no hint that clowns exist
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
	case kind == clownKind && g.clownsOf(key) > 0:
		g.reply(c.Name, "у тебя уже есть клоун")
		return
	case kind == "shrine" && !g.isStreamer(key) && g.shrineLimit(c) > 0 && g.kindOf(key, "shrine") >= g.shrineLimit(c):
		g.reply(c.Name, fmt.Sprintf("святилищ у одного игрока не больше %d (у жреца %d)", g.Cfg.ShrineLimit, g.Cfg.ShrineLimitCleric))
		return
	case kind != clownKind && !g.isStreamer(key) && g.towersOf(key)-g.clownsOf(key) >= g.TowerLimit():
		g.reply(c.Name, fmt.Sprintf("лимит построек %d", g.TowerLimit()))
		return
	}
	cost := g.Cfg.TowerCost(kind, 1)
	streamer := g.isStreamer(key)
	if !streamer && !g.canQueue(key) {
		g.reply(c.Name, fmt.Sprintf("у наёмника уже %d приказа, подожди", g.Cfg.MaxOrders))
		return
	}
	granted := 0
	if !streamer {
		var ok bool
		if granted, ok = g.spend(c, cost); !ok {
			g.reply(c.Name, fmt.Sprintf("нужно %d золота, у тебя %d", cost, c.Gold+c.Grant))
			return
		}
	}
	// The streamer's towers appear at once; a chatter's mercenary walks out
	// and builds it, until then the tower is a construction site.
	t := &Tower{ID: g.id(), Kind: kind, Cell: p, Owner: key, Level: 1, Invested: cost, Granted: granted, Built: streamer}
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
	if g.busyAtDice(c, key) {
		return
	}
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
	case t.Level >= g.maxLevel(t):
		if g.maxLevel(t) == spec.MaxLevel && t.Kind != clownKind {
			g.reply(c.Name, "уже максимальный уровень; 6-й, артефактный, — только для постройки своего класса")
		} else {
			g.reply(c.Name, "уже максимальный уровень")
		}
		return
	case !t.Built || t.PendingCost > 0 || t.repairing:
		g.reply(c.Name, t.Cell.Name()+": наёмник ещё работает над ней")
		return
	}
	streamer := g.isStreamer(key)
	if !streamer && !g.canQueue(key) {
		g.reply(c.Name, fmt.Sprintf("у наёмника уже %d приказа, подожди", g.Cfg.MaxOrders))
		return
	}
	cost, levels := g.Cfg.TowerCost(t.Kind, t.Level+1), 1
	if len(args) > 1 && upToMax[strings.ToLower(args[1])] {
		// As many levels as the wallet (and a grant) pays for, at once.
		budget := c.Gold + c.Grant
		cost, levels = 0, 0
		for lvl := t.Level + 1; lvl <= g.maxLevel(t); lvl++ {
			next := g.Cfg.TowerCost(t.Kind, lvl)
			if !streamer && cost+next > budget {
				break
			}
			cost += next
			levels++
		}
		if levels == 0 {
			g.reply(c.Name, fmt.Sprintf("на следующий уровень нужно %d золота, у тебя %d", g.Cfg.TowerCost(t.Kind, t.Level+1), budget))
			return
		}
		g.reply(c.Name, fmt.Sprintf("%s: улучшаю до %d ур. за %d золота", t.Cell.Name(), t.Level+levels, cost))
	}
	if !streamer {
		granted, ok := g.spend(c, cost)
		if !ok {
			g.reply(c.Name, fmt.Sprintf("нужно %d золота, у тебя %d", cost, c.Gold+c.Grant))
			return
		}
		t.Granted += granted
	}
	t.Invested += cost
	if streamer {
		t.Level += levels
		g.readyTower(t)
		g.announceArtifact(t, c.Name)
	} else {
		t.PendingCost, t.PendingUp = cost, levels
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
	g.sellTower(t, c.Name)
}

// sellTower takes a building down and pays its owner the refund, which it
// returns.
func (g *Game) sellTower(t *Tower, by string) int {
	g.removeTower(t)
	// Unfinished work is refunded in full: nothing was built with that gold.
	refund := (t.Invested - t.PendingCost) * g.Cfg.SellRefundPct / 100
	if !t.Built {
		refund = t.Invested
	} else {
		refund += t.PendingCost
	}
	refund = g.refund(t, refund)
	cp := center(t.Cell)
	g.emit(Event{Kind: "sell", User: by, X: cp.X, Y: cp.Y, Amount: refund})
	return refund
}

// cmdLeave takes a player out of the game: every building is sold and the
// whole wallet goes to the treasury. !play brings them back.
func (g *Game) cmdLeave(c *Chatter, key string) {
	switch {
	case g.isStreamer(key):
		return
	case !c.Joined:
		g.reply(c.Name, "ты и так не в игре; войти: !play")
		return
	case g.busyAtDice(c, key):
		return
	}
	towers := 0
	for _, t := range g.Towers {
		if t.Owner == key {
			g.sellTower(t, c.Name)
			towers++
		}
	}
	gold := c.Gold
	c.Gold, c.Grant = 0, 0
	g.toTreasury(gold)
	c.Joined, c.wasActive = false, false
	g.emit(Event{Kind: "leave", User: c.Name, Amount: gold})
	g.reply(c.Name, fmt.Sprintf("ты покинул таверну: продано построек %d, %d золота ушло в казну. Вернуться: !play", towers, gold))
}

// cmdAdmin handles !td* commands for the streamer and moderators.
func (g *Game) cmdAdmin(c *Chatter, cmd string, args []string) {
	switch cmd {
	case "!tdmeme", "!tdмемы": // memecoin requests: list, done or reject
		g.cmdMemeAdmin(c, args)
	case "!tdversion", "!tdверсия":
		g.reply(c.Name, "версия v"+Version)
	case "!tdupdate", "!tdобновить", "!tdrollback", "!tdоткат":
		if g.Ops == nil {
			g.reply(c.Name, "обновления не настроены")
		} else if cmd == "!tdupdate" || cmd == "!tdобновить" {
			g.Ops.Update(c.Name)
		} else {
			g.Ops.Rollback(c.Name)
		}
	case "!tdwave":
		g.StartWave()
	case "!tdnew":
		biome := Biome("")
		if len(args) > 0 {
			biome, _ = parseBiome(args[0]) // an unknown one: random
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
				g.spawnBoss(args[0], hp*g.Cfg.BossHPMult*spec.HPMult)
			} else {
				g.spawnEnemy(args[0], hp*spec.HPMult, 0.4*float64(i))
			}
		}
	case "!tdfree", "!tdосвободить": // let an inmate out of the jail
		g.cmdFree(c, args)
	case "!tdban", "!tdunban": // forbid or allow building for a chatter
		if len(args) == 1 {
			g.ban(args[0], cmd == "!tdban")
		}
	case "!tdtreasury": // !tdtreasury nick N: hand treasury gold to a chatter
		g.cmdTreasury(args)
	case "!tdvolume", "!tdmute", "!tdambient", "!tdsound":
		g.cmdSound(c, cmd, args)
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
	case "!tdsouls": // !tdsouls nick N: grant (or with a minus take) souls by hand
		if len(args) == 2 {
			if n, err := strconv.Atoi(args[1]); err == nil {
				name := strings.TrimPrefix(args[0], "@")
				key := strings.ToLower(name)
				p := g.chatter(key, name)
				if n > 0 {
					g.giveSouls(key, p, n) // a gift counts toward the rank
				} else {
					p.Souls = max(0, p.Souls+n)
				}
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

// gameCommands are the chat commands that bring a chatter into the game.
// Until then a chatter only earns gold for talking: no mercenary in the
// tavern and no weight in the difficulty, so people who just say hi and
// leave do not make the waves harder. !помощь only explains, so it does not
// count.
// upToMax are the words that make !up raise a building as far as the gold goes.
var upToMax = map[string]bool{"max": true, "макс": true, "максимум": true, "all": true, "все": true, "всё": true}

var gameCommands = map[string]bool{
	"!play": true, "!играть": true, "!join": true,
	"!build": true, "!b": true, "!строить": true, "!с": true,
	"!up": true, "!u": true, "!улучшить": true,
	"!sell": true, "!продать": true,
	"!repair": true, "!чинить": true, "!починить": true,
	"!me": true, "!я": true,
	"!стрелы": true, "!arrows": true, "!ammo": true,
	"!dice": true, "!кости": true, "!к": true,
	"!class": true, "!класс": true,
	"!drink": true, "!d": true, "!выпить": true, "!пить": true,
	"!towers": true, "!башни": true,
	"!души": true, "!souls": true, "!ачивки": true, "!achievements": true, "!ach": true,
	"!мемы": true, "!memes": true, "!link": true, "!привязать": true,
	"!dance": true, "!танец": true, "!пляс": true, "!танцевать": true,
	"!тюрьма": true, "!jail": true, "!посадить": true,
	"!новаякарта": true, "!нк": true, "!newmap": true,
}

// join seats a chatter in the tavern for this session.
func (g *Game) join(key string, c *Chatter, cmd string) {
	if c.Joined {
		if cmd == "!play" || cmd == "!играть" || cmd == "!join" {
			g.reply(c.Name, "ты уже в таверне")
		}
		return
	}
	c.Joined = true
	g.emit(Event{Kind: "join", User: c.Name})
	if cmd == "!play" || cmd == "!играть" || cmd == "!join" {
		g.reply(c.Name, "добро пожаловать в таверну! Строй: !b C4 arrow, все команды: !помощь")
	}
	g.arrive(key, c)
}

// shrineLimit is how many shrines c may own: more for a cleric.
func (g *Game) shrineLimit(c *Chatter) int {
	if c.Class == "cleric" {
		return g.Cfg.ShrineLimitCleric
	}
	return g.Cfg.ShrineLimit
}

// kindOf counts the buildings of one kind a player owns.
func (g *Game) kindOf(owner, kind string) int {
	n := 0
	for _, t := range g.Towers {
		if t.Owner == owner && t.Kind == kind {
			n++
		}
	}
	return n
}
