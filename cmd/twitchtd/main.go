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
	"twitchtd/web"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "listen address")
	cfgPath := flag.String("config", "config.json", "balance overrides (optional)")
	bots := flag.Int("sim", 0, "number of fake chatters to simulate")
	seed := flag.Uint64("seed", 0, "random seed (0 = time based)")
	webDir := flag.String("web", "", "serve web files from this directory instead of the embedded copy (for development)")
	flag.Parse()

	cfg, err := game.LoadConfig(*cfgPath)
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

	srv := server.New(game.New(cfg, *seed), files)
	go srv.Run(ctx)
	if *bots > 0 {
		sim.Run(ctx, *bots, srv.Chat(), srv.CurrentMap)
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
}
