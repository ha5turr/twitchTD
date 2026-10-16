package main

import (
	"context"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"twitchtd/internal/game"
	"twitchtd/internal/server"
	"twitchtd/internal/sim"
	"twitchtd/internal/store"
	"twitchtd/internal/streamerbot"
	"twitchtd/web"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "listen address")
	cfgPath := flag.String("config", "config.json", "settings file (optional)")
	bots := flag.Int("sim", 0, "number of fake chatters to simulate")
	seed := flag.Uint64("seed", 0, "random seed (0 = time based)")
	webDir := flag.String("web", "", "serve web files from this directory instead of the embedded copy (for development)")
	fresh := flag.Bool("fresh", false, "ignore the saved game and start a new one (wallets are kept)")
	flag.Parse()

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if *seed == 0 {
		*seed = uint64(time.Now().UnixNano())
	}

	var files fs.FS = web.Files()
	if *webDir != "" {
		files = os.DirFS(*webDir)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	g := game.New(cfg.Config, *seed)
	opt := server.Options{}

	if cfg.DB != "" {
		st, err := store.Open(cfg.DB)
		if err != nil {
			log.Fatalf("db: %v", err)
		}
		defer st.Close()
		save, ok, err := st.Load()
		if err != nil {
			log.Fatalf("db: %v", err)
		}
		g.LoadChatters(save.Chatters)
		if ok && !*fresh {
			g.Restore(save)
			log.Printf("resumed: %s, wave %d", save.Biome, save.Wave)
		}
		opt.Persist = st.Save
	}

	var sb *streamerbot.Client
	var srv *server.Server
	if cfg.StreamerBot.Enabled {
		sb = streamerbot.New(cfg.StreamerBot, streamerbot.Handlers{
			Chat: func(m streamerbot.Chat) {
				srv.Chat(game.ChatMessage{
					Login: m.Login, User: m.Name, Text: m.Text,
					Mod:         m.Role >= streamerbot.RoleModerator,
					Broadcaster: m.Role == streamerbot.RoleBroadcaster,
				})
			},
			Present: func(logins []string) {
				srv.Do(func(g *game.Game) { g.Present(logins) })
			},
		})
		opt.Say = sb.Send
	}

	srv = server.New(g, files, opt)
	runDone := make(chan struct{})
	go func() {
		srv.Run(ctx)
		close(runDone)
	}()
	if sb != nil {
		go sb.Run(ctx)
	}
	if *bots > 0 {
		sim.Run(ctx, *bots, srv.Chat, srv.CurrentMap)
		log.Printf("simulating %d chatters", *bots)
	}

	hs := &http.Server{Addr: *addr, Handler: srv.Handler()}
	go func() {
		<-ctx.Done()
		hs.Close()
	}()
	log.Printf("overlay: http://%s/   dev console: http://%s/dev.html", *addr, *addr)
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	<-runDone // the loop saves the game on its way out
}
