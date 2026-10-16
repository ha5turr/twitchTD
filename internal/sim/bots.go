// Package sim feeds fake chatters into the game so balance and visuals can be
// checked without a live stream.
package sim

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"twitchtd/internal/game"
)

var names = []string{
	"GrimReaper_77", "ale_enjoyer", "Некромантик", "dark_knightess", "SkellyBones", "trollface_ru",
	"torchbearer", "лучник_Вася", "CryptKeeper", "frostbite99", "HolyMoly", "bard_of_doom",
	"rogue_one", "pyro_Petya", "MummyDearest", "IceQueen", "Gravedigger", "spider_lord", "lich_please", "tavern_rat",
}

var chatter = []string{"gg", "lol", "давай давай", "босс скоро?", "PogChamp", "где строить", "эль за мой счёт", "KEKW"}

// Run starts n bots. currentMap tells them where building makes sense.
func Run(ctx context.Context, n int, chat chan<- game.ChatMessage, currentMap func() game.MapMsg) {
	for i := 0; i < n && i < len(names); i++ {
		go bot(ctx, names[i], uint64(i), chat, currentMap)
	}
}

func bot(ctx context.Context, name string, seed uint64, chat chan<- game.ChatMessage, currentMap func() game.MapMsg) {
	r := rand.New(rand.NewPCG(seed, uint64(time.Now().UnixNano())))
	var built []string
	send := func(text string) {
		select {
		case chat <- game.ChatMessage{User: name, Text: text}:
		case <-ctx.Done():
		}
	}
	send("привет")
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(3000+r.IntN(9000)) * time.Millisecond):
		}
		switch roll := r.Float64(); {
		case roll < 0.35:
			send(chatter[r.IntN(len(chatter))])
		case roll < 0.75:
			if cell, ok := pickCell(r, currentMap()); ok {
				kind := []string{"arrow", "magic"}[r.IntN(2)]
				send(fmt.Sprintf("!build %s %s", cell, kind))
				built = append(built, cell)
			}
		case roll < 0.95 && len(built) > 0:
			send("!up " + built[r.IntN(len(built))])
		default:
			send("!me")
		}
	}
}

// pickCell chooses a free cell next to the path, where towers are useful.
func pickCell(r *rand.Rand, m game.MapMsg) (string, bool) {
	var cands []game.Pos
	for y, row := range m.Cells {
		for x := range row {
			if row[x] != '.' {
				continue
			}
			for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}, {1, -1}, {-1, 1}} {
				nx, ny := x+d[0], y+d[1]
				if ny >= 0 && ny < len(m.Cells) && nx >= 0 && nx < len(row) && m.Cells[ny][nx] == '#' {
					cands = append(cands, game.Pos{X: x, Y: y})
					break
				}
			}
		}
	}
	if len(cands) == 0 {
		return "", false
	}
	return cands[r.IntN(len(cands))].Name(), true
}
