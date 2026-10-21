package game

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// SoundGroup is a family of overlay sounds the streamer can turn down
// together. The overlay maps each sound to its group (web/static/js/audio.js).
type SoundGroup struct {
	Key     string
	Name    string
	Aliases []string
}

var SoundGroups = []SoundGroup{
	{"combat", "бой", []string{"бой", "combat", "выстрелы"}},
	{"building", "стройка", []string{"стройка", "building", "build", "золото"}},
	{"alerts", "волны и боссы", []string{"волны", "alerts", "боссы", "босс"}},
	{"support", "донаты и награды", []string{"донаты", "support", "награды", "донат"}},
	{"tavern", "таверна", []string{"таверна", "tavern", "выпивка"}},
	{"walls", "стены и наёмники", []string{"стены", "walls", "наёмники"}},
}

func soundGroup(s string) (SoundGroup, bool) {
	s = strings.ToLower(s)
	for _, g := range SoundGroups {
		for _, a := range g.Aliases {
			if a == s {
				return g, true
			}
		}
	}
	return SoundGroup{}, false
}

// Operator runs the release commands, which reach outside the game: the
// server downloads a build and restarts on it, or goes back to the last one.
type Operator interface {
	Update(by string)
	Rollback(by string)
}

// Announce says something to the whole chat.
func (g *Game) Announce(text string) { g.reply("", text) }

// SettingsAdmin lets chat commands read and change the overlay settings,
// which live outside the game (config.json plus the panel's overrides).
type SettingsAdmin interface {
	Values() map[string]any
	Change(changes map[string]any) error
}

// percent reads "0".."100", "on" (100) or "off" (0) as a share 0..1.
func percent(s string) (float64, bool) {
	switch strings.ToLower(strings.TrimSuffix(s, "%")) {
	case "on", "вкл":
		return 1, true
	case "off", "выкл":
		return 0, true
	}
	n, err := strconv.Atoi(strings.TrimSuffix(s, "%"))
	if err != nil || n < 0 || n > 100 {
		return 0, false
	}
	return float64(n) / 100, true
}

func pct(v any) int {
	f, _ := v.(float64)
	return int(math.Round(f * 100))
}

// cmdSound handles !tdvolume, !tdmute, !tdambient and !tdsound for the
// streamer and moderators.
func (g *Game) cmdSound(c *Chatter, cmd string, args []string) {
	if g.Admin == nil {
		return
	}
	cur := g.Admin.Values()
	change := map[string]any{}
	usage := ""
	switch cmd {
	case "!tdvolume":
		if v, ok := percentArg(args); ok {
			change["volume"] = v
			change["muted"] = false
		} else {
			usage = fmt.Sprintf("громкость %d%%. Формат: !tdvolume 0-100", pct(cur["volume"]))
		}
	case "!tdmute":
		muted, _ := cur["muted"].(bool)
		change["muted"] = !muted
	case "!tdambient":
		switch v, ok := percentArg(args); {
		case !ok:
			usage = "формат: !tdambient on, off или 0-100"
		case v == 0:
			change["ambient"] = false
		default:
			change["ambient"] = true
			if a := strings.ToLower(args[0]); a != "on" && a != "вкл" {
				change["ambientVolume"] = v // a number sets the level too
			}
		}
	case "!tdsound":
		if len(args) < 2 {
			usage = g.soundStatus(cur)
			break
		}
		grp, ok := soundGroup(args[0])
		v, okv := percent(args[1])
		if !ok || !okv {
			usage = "формат: !tdsound <бой|стройка|волны|донаты|таверна|стены> 0-100, on или off"
			break
		}
		// Send the whole map: overrides replace a setting as a whole.
		groups := map[string]any{}
		if m, ok := cur["soundGroups"].(map[string]any); ok {
			for k, v := range m {
				groups[k] = v
			}
		}
		groups[grp.Key] = v
		change["soundGroups"] = groups
	}
	if usage != "" {
		g.reply(c.Name, usage)
		return
	}
	if err := g.Admin.Change(change); err != nil {
		g.reply(c.Name, "не получилось: "+err.Error())
		return
	}
	g.reply(c.Name, g.soundStatus(g.Admin.Values()))
}

func percentArg(args []string) (float64, bool) {
	if len(args) == 0 {
		return 0, false
	}
	return percent(args[0])
}

func (g *Game) soundStatus(v map[string]any) string {
	if muted, _ := v["muted"].(bool); muted {
		return "звук выключен (!tdmute — включить)"
	}
	parts := []string{fmt.Sprintf("громкость %d%%", pct(v["volume"]))}
	if amb, _ := v["ambient"].(bool); amb {
		parts = append(parts, fmt.Sprintf("эмбиент %d%%", pct(v["ambientVolume"])))
	} else {
		parts = append(parts, "эмбиент выкл")
	}
	groups, _ := v["soundGroups"].(map[string]any)
	for _, sg := range SoundGroups {
		val, ok := groups[sg.Key]
		if !ok {
			val = 1.0
		}
		parts = append(parts, fmt.Sprintf("%s %d%%", sg.Name, pct(val)))
	}
	return strings.Join(parts, ", ")
}
