package game

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// MemeRequest is a chatter's order of memecoins paid in souls. MemeAlerts
// has no official API for crediting a viewer, so the streamer credits them
// by hand from the panel and marks the request done, or rejects it and the
// souls come back.
type MemeRequest struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Coins int    `json:"coins"`
	Souls int    `json:"souls"`
}

func (g *Game) cmdMemes(c *Chatter, key string, args []string) {
	rate := g.Cfg.SoulsPerMemecoin
	if len(args) == 0 {
		g.reply(c.Name, fmt.Sprintf("!мемы N — заявка на N мемкоинов (до %d), %d душ за мемкоин. У тебя %d душ", g.Cfg.MemeMax, rate, c.Souls))
		return
	}
	n, err := strconv.Atoi(args[0])
	switch {
	case key == StreamerName:
		return
	case err != nil || n < 1 || n > g.Cfg.MemeMax:
		g.reply(c.Name, fmt.Sprintf("укажи число мемкоинов от 1 до %d", g.Cfg.MemeMax))
		return
	case slices.ContainsFunc(g.Memes, func(r *MemeRequest) bool { return r.Login == key }):
		g.reply(c.Name, "твоя прошлая заявка ещё ждёт стримера")
		return
	case c.Souls < n*rate:
		g.reply(c.Name, fmt.Sprintf("нужно %d душ, у тебя %d", n*rate, c.Souls))
		return
	}
	c.Souls -= n * rate
	g.nextMeme++
	r := &MemeRequest{ID: g.nextMeme, Login: key, Name: c.Name, Coins: n, Souls: n * rate}
	g.Memes = append(g.Memes, r)
	g.emit(Event{Kind: "memeRequest", User: c.Name, Amount: n})
	g.reply(c.Name, fmt.Sprintf("заявка #%d: %d мемкоинов за %d душ, стример начислит вручную", r.ID, n, r.Souls))
}

// cmdMemeAdmin handles !tdmeme [done|reject ID] for the streamer and mods.
func (g *Game) cmdMemeAdmin(c *Chatter, args []string) {
	if len(args) < 2 {
		if len(g.Memes) == 0 {
			g.reply(c.Name, "заявок на мемкоины нет")
			return
		}
		var parts []string
		for _, r := range g.Memes {
			parts = append(parts, fmt.Sprintf("#%d %s: %d", r.ID, r.Name, r.Coins))
		}
		g.reply(c.Name, "заявки: "+strings.Join(parts, ", ")+". Закрыть: !tdmeme done|reject номер")
		return
	}
	id, _ := strconv.Atoi(strings.TrimPrefix(args[1], "#"))
	i := slices.IndexFunc(g.Memes, func(r *MemeRequest) bool { return r.ID == id })
	if i < 0 {
		g.reply(c.Name, fmt.Sprintf("заявки #%d нет", id))
		return
	}
	r := g.Memes[i]
	switch strings.ToLower(args[0]) {
	case "done", "выдано", "ок":
		g.reply(r.Name, fmt.Sprintf("мемкоины по заявке #%d начислены: %d", r.ID, r.Coins))
	case "reject", "отказ", "нет":
		if p := g.Chatters[r.Login]; p != nil {
			p.Souls += r.Souls
		}
		g.reply(r.Name, fmt.Sprintf("заявка #%d отклонена, %d душ вернулись", r.ID, r.Souls))
	default:
		g.reply(c.Name, "!tdmeme done|reject номер")
		return
	}
	g.Memes = slices.Delete(g.Memes, i, i+1)
}

// LoadMemes restores open requests after a restart.
func (g *Game) loadMemes(rs []*MemeRequest) {
	g.Memes = rs
	for _, r := range rs {
		g.nextMeme = max(g.nextMeme, r.ID)
	}
}
