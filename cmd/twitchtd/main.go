package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"twitchtd/internal/donationalerts"
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

	var st *store.Store
	if cfg.DB != "" {
		st, err = store.Open(cfg.DB)
		if err != nil {
			log.Fatalf("db: %v", err)
		}
		defer st.Close()
	}
	// The panel's overrides live in the database, so settings come after it.
	set, err := newSettings(*cfgPath, st)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	cfg = set.Current()

	g := game.New(cfg.Config, *seed)
	g.Admin = chatAdmin{set}
	opt := server.Options{OnSettings: set.Set}

	if st != nil {
		save, ok, err := st.Load()
		if err != nil {
			log.Fatalf("db: %v", err)
		}
		g.LoadChatters(save.Chatters)
		g.LoadEconomy(save)
		if ok && !*fresh {
			g.Restore(save)
			log.Printf("resumed: %s, wave %d", save.Biome, save.Wave)
		}
		opt.Persist = st.Save
	}

	var srv *server.Server
	var sb *streamerbot.Client
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
			Reward: func(u streamerbot.User, r streamerbot.Redemption) {
				srv.Do(func(g *game.Game) {
					if g.Reward(u.Login, u.Name, r.Title) {
						sb.Refund(r) // over the per-game cap
					}
				})
			},
			Bits: func(u streamerbot.User, bits int) {
				srv.Do(func(g *game.Game) { g.Bits(u.Login, u.Name, bits) })
			},
			Sub: func(u streamerbot.User) {
				srv.Do(func(g *game.Game) { g.Sub(u.Login, u.Name) })
			},
			GiftSub: func(from, to streamerbot.User) {
				srv.Do(func(g *game.Game) { g.GiftSub(from.Login, from.Name, to.Login, to.Name) })
			},
		})
		opt.Say = sb.Send
	}

	var da *donationalerts.Client
	if cfg.DonationAlerts.Enabled {
		var tokens donationalerts.TokenStore = &memTokens{}
		if st != nil {
			tokens = kvTokens{st}
		} else {
			log.Printf("donationalerts: no database, the login will not survive a restart")
		}
		da = donationalerts.New(cfg.DonationAlerts, donationalerts.Production, "http://"+*addr+"/da/callback", tokens,
			func(d donationalerts.Donation) {
				srv.Do(func(g *game.Game) {
					g.Donate(game.Donation{Name: d.Name, Message: d.Message, Amount: d.Amount, Currency: d.Currency})
				})
			})
	}

	srv = server.New(g, files, opt)
	up := newOps(cfg.Update, srv, stop)
	g.Ops = up
	set.apply = func(c appConfig, values map[string]any) {
		srv.Do(func(g *game.Game) { g.SetConfig(c.Config) })
		srv.SetSettings(values)
	}
	srv.SetSettings(set.View())
	go set.watch(ctx)
	runDone := make(chan struct{})
	go func() {
		srv.Run(ctx)
		close(runDone)
	}()
	up.reportUpdate()
	if sb != nil {
		go sb.Run(ctx)
	}
	if da != nil {
		go da.Run(ctx)
	}
	if *bots > 0 {
		sim.Run(ctx, *bots, srv.Chat, srv.CurrentMap)
		log.Printf("simulating %d chatters", *bots)
	}

	mux := http.NewServeMux()
	mux.Handle("/", srv.Handler())
	if da != nil {
		mux.Handle("/da/", da.Handler())
	} else {
		mux.HandleFunc("/da/status", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"выключено в config.json"}`))
		})
	}
	hs := &http.Server{Addr: *addr, Handler: mux}
	go func() {
		<-ctx.Done()
		hs.Close()
	}()
	log.Printf("overlay: http://%s/   dev console: http://%s/dev.html", *addr, *addr)
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	<-runDone // the loop saves the game on its way out
	if code := up.exitCode(); code != 0 {
		// An update or a rollback: run.ps1 swaps the build and starts it.
		if st != nil {
			st.Close()
		}
		os.Exit(code)
	}
}

// kvTokens keeps the DonationAlerts token in the game database.
type kvTokens struct{ st *store.Store }

func (k kvTokens) LoadToken() (donationalerts.Token, bool, error) {
	raw, ok, err := k.st.GetKV("donationalerts_token")
	if err != nil || !ok {
		return donationalerts.Token{}, false, err
	}
	var t donationalerts.Token
	return t, true, json.Unmarshal([]byte(raw), &t)
}

func (k kvTokens) SaveToken(t donationalerts.Token) error {
	data, _ := json.Marshal(t)
	return k.st.SetKV("donationalerts_token", string(data))
}

type memTokens struct{ t donationalerts.Token }

func (m *memTokens) LoadToken() (donationalerts.Token, bool, error) {
	return m.t, m.t.Access != "", nil
}
func (m *memTokens) SaveToken(t donationalerts.Token) error { m.t = t; return nil }
