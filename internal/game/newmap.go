package game

import (
	"fmt"
	"strings"
	"time"
)

// A new map by the players' will: !новаякарта [биом]. A moderator's, or a
// lone player's, starts it at once; anyone else's puts it to the tavern.
// Silence counts as yes: the map changes unless at least half of the
// players in the tavern say !против.

// MapVote is the tavern deciding whether to start a new map.
type MapVote struct {
	ByName  string
	Biome   Biome // "" for a random one
	left    float64
	yes, no map[string]bool
}

func (g *Game) cmdNewMap(c *Chatter, key string, mod bool, args []string) {
	biome := Biome("")
	if len(args) > 0 {
		b, ok := parseBiome(args[0])
		if !ok {
			g.reply(c.Name, "нет такого биома: руины, подземелье, крипта, лёд, болото")
			return
		}
		biome = b
	}
	voters := g.mapVoters()
	switch {
	case mod || g.Phase == PhaseGameOver || len(voters) <= 1:
		g.reply("", c.Name+" начинает новую карту")
		g.NewGame(biome)
	case g.mapVote != nil:
		g.reply(c.Name, "уже голосуем за новую карту: !за или !против")
	case g.jailVote != nil:
		g.reply(c.Name, "сначала дождись конца суда таверны")
	case g.Now().Before(g.mapVoteAt):
		g.reply(c.Name, fmt.Sprintf("за новую карту недавно голосовали, подожди %d с", int(g.mapVoteAt.Sub(g.Now()).Seconds())+1))
	default:
		g.mapVote = &MapVote{ByName: c.Name, Biome: biome, left: g.Cfg.NewMapVoteSec, yes: map[string]bool{key: true}, no: map[string]bool{}}
		where := ""
		if biome != "" {
			where = " (" + biomeName(biome) + ")"
		}
		g.emit(Event{Kind: "mapVote", User: c.Name, Text: string(biome), Amount: int(g.Cfg.NewMapVoteSec)})
		g.reply("", fmt.Sprintf("%s предлагает начать новую карту%s! Против — пишите !против, %d с. Кто промолчит, тот согласен", c.Name, where, int(g.Cfg.NewMapVoteSec)))
	}
}

// parseBiome reads a biome by its key or Russian name.
func parseBiome(s string) (Biome, bool) {
	b := Biome(strings.ToLower(s))
	if a, ok := biomeAliases[string(b)]; ok {
		b = a
	}
	_, ok := profiles[b]
	return b, ok
}

// biomeName is a biome's name for the chat.
func biomeName(b Biome) string {
	return map[Biome]string{BiomeRuins: "руины", BiomeDungeon: "подземелье", BiomeCrypt: "крипта", BiomeIce: "ледяные пещеры", BiomeSwamp: "болото"}[b]
}

// mapVoters are the players in the tavern: all of them have a say.
func (g *Game) mapVoters() []string {
	cut := g.Now().Add(-activeWindow(g.Cfg))
	var out []string
	for k, c := range g.Chatters {
		if k != StreamerName && c.Joined && c.LastSeen.After(cut) {
			out = append(out, k)
		}
	}
	return out
}

func (g *Game) voteNewMap(c *Chatter, key string, yes bool) {
	v := g.mapVote
	if !c.Joined {
		return
	}
	delete(v.yes, key)
	delete(v.no, key)
	if yes {
		v.yes[key] = true
	} else {
		v.no[key] = true
	}
	voters := len(g.mapVoters())
	if len(v.yes)+len(v.no) >= voters || 2*len(v.no) >= voters {
		g.endMapVote() // everyone has had a say, or the "no" has won already
	}
}

func (g *Game) endMapVote() {
	v := g.mapVote
	g.mapVote = nil
	voters := len(g.mapVoters())
	no := len(v.no)
	yes := max(voters-no, len(v.yes)) // silence is consent
	if 2*no >= voters {
		g.mapVoteAt = g.Now().Add(time.Duration(g.Cfg.NewMapRetrySec * float64(time.Second)))
		g.reply("", fmt.Sprintf("играем дальше: против %d из %d", no, voters))
		return
	}
	g.reply("", fmt.Sprintf("таверна решила: новая карта (за %d, против %d)", yes, no))
	g.NewGame(v.Biome)
}

func (g *Game) tickMapVote(dt float64) {
	if v := g.mapVote; v != nil {
		if v.left -= dt; v.left <= 0 {
			g.endMapVote()
		}
	}
}
