// Package server runs the game loop and streams its state to the overlay and
// the streamer panel over WebSocket.
package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"twitchtd/internal/game"
)

type Options struct {
	// Say sends a line to Twitch chat. Nil logs replies instead.
	Say func(message string)
	// Persist saves the game; it is called from the game loop. Nil disables saving.
	Persist   func(game.Save) error
	SaveEvery time.Duration
	// OnSettings applies settings changed in the streamer panel.
	OnSettings func(json.RawMessage) error
}

type Server struct {
	game  *game.Game
	in    chan func(*game.Game)
	files fs.FS
	opt   Options
	reply *replier

	mu           sync.Mutex
	clients      map[*client]struct{}
	mapMsg       game.MapMsg
	mapJSON      []byte
	settingsJSON []byte
}

type client struct {
	send chan []byte
}

func New(g *game.Game, files fs.FS, opt Options) *Server {
	if opt.SaveEvery == 0 {
		opt.SaveEvery = 10 * time.Second
	}
	say := opt.Say
	if say == nil {
		say = func(m string) { log.Printf("chat reply: %s", m) }
	}
	return &Server{
		game:    g,
		in:      make(chan func(*game.Game), 512),
		files:   files,
		opt:     opt,
		reply:   newReplier(say),
		clients: map[*client]struct{}{},
	}
}

// Do runs fn on the game loop goroutine. It is the single entry point for
// Streamer.bot, the panel and simulation bots, so the game needs no locks.
func (s *Server) Do(fn func(*game.Game)) {
	select {
	case s.in <- fn:
	default:
		log.Printf("input queue full, dropping an event")
	}
}

// Chat feeds one chat line into the game.
func (s *Server) Chat(m game.ChatMessage) {
	s.Do(func(g *game.Game) { g.HandleChat(m) })
}

// CurrentMap returns the latest map message.
func (s *Server) CurrentMap() game.MapMsg {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mapMsg
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	files := http.FileServerFS(s.files)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// OBS keeps its browser cache between sessions; revalidate every file
		// so an updated game never runs with stale scripts.
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
	mux.HandleFunc("/ws", s.serveWS)
	return mux
}

// Run owns the game: every mutation happens on this goroutine.
func (s *Server) Run(ctx context.Context) {
	rate := s.game.Cfg.TickRate
	dt := 1 / float64(rate)
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	defer ticker.Stop()
	saveTicker := time.NewTicker(s.opt.SaveEvery)
	defer saveTicker.Stop()
	go s.reply.run(ctx)
	for {
		select {
		case <-ctx.Done():
			s.save()
			return
		case fn := <-s.in:
			fn(s.game)
		case <-saveTicker.C:
			s.save()
		case <-ticker.C:
			s.game.Tick(dt)
			if s.game.TakeMapChanged() {
				s.publishMap()
			}
			st := s.game.StateMessage()
			for _, e := range st.Events {
				if e.Kind == "reply" {
					s.reply.add(e.User, e.Text)
				}
			}
			s.broadcast(mustJSON(st))
		}
	}
}

func (s *Server) save() {
	if s.opt.Persist == nil {
		return
	}
	if err := s.opt.Persist(s.game.Export()); err != nil {
		log.Printf("save: %v", err)
	}
}

func (s *Server) publishMap() {
	m := s.game.MapMessage()
	data := mustJSON(m)
	s.mu.Lock()
	s.mapMsg, s.mapJSON = m, data
	s.mu.Unlock()
	log.Printf("new map: biome %s, seed %d", m.Biome, m.Seed)
	s.broadcast(data)
}

func (s *Server) broadcast(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		select {
		case c.send <- data:
		default:
			// A client that can't keep up loses frames; the next one replaces them.
		}
	}
}

// inbound is what clients may send: chat lines from the panel or dev console,
// and test support events so the streamer can rehearse without real money.
type inbound struct {
	T        string          `json:"t"`
	User     string          `json:"user"`
	Text     string          `json:"text"`
	Mod      bool            `json:"mod"`
	Amount   float64         `json:"amount"`
	Currency string          `json:"currency"`
	Values   json.RawMessage `json:"values"`
}

func (s *Server) handleInbound(m inbound) {
	switch m.T {
	case "chat":
		// The panel's "streamer" user acts as the broadcaster.
		s.Chat(game.ChatMessage{User: m.User, Text: m.Text, Mod: m.Mod, Broadcaster: m.User == game.StreamerName})
	case "donation":
		s.Do(func(g *game.Game) {
			g.Donate(game.Donation{Name: m.User, Message: m.Text, Amount: m.Amount, Currency: m.Currency})
		})
	case "reward":
		s.Do(func(g *game.Game) { g.Reward(m.User, m.User, m.Text) })
	case "bits":
		s.Do(func(g *game.Game) { g.Bits(m.User, m.User, int(m.Amount)) })
	case "soundTest": // the panel asks the overlay (OBS) to play a sample
		s.broadcast(mustJSON(map[string]string{"t": "soundTest", "group": m.Text}))
	case "settings":
		if s.opt.OnSettings == nil {
			return
		}
		if err := s.opt.OnSettings(m.Values); err != nil {
			s.broadcast(mustJSON(map[string]string{"t": "settingsError", "error": err.Error()}))
		}
	case "kingDone": // the overlay played the King in Yellow's scene to its end
		s.Do(func(g *game.Game) { g.KingSceneDone() })
	case "sub":
		s.Do(func(g *game.Game) { g.Sub(m.User, m.User) })
	}
}

func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	c := &client{send: make(chan []byte, 8)}
	s.mu.Lock()
	if s.settingsJSON != nil {
		c.send <- s.settingsJSON
	}
	if s.mapJSON != nil {
		c.send <- s.mapJSON
	}
	s.clients[c] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, c)
		s.mu.Unlock()
	}()

	go func() {
		defer cancel()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var m inbound
			if json.Unmarshal(data, &m) == nil {
				s.handleInbound(m)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case data := <-c.send:
			wctx, wcancel := context.WithTimeout(ctx, 2*time.Second)
			err := conn.Write(wctx, websocket.MessageText, data)
			wcancel()
			if err != nil {
				return
			}
		}
	}
}

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// SetSettings publishes the current editable settings to every client; the
// panel shows them and the overlay takes its sound volume from them.
func (s *Server) SetSettings(values map[string]any) {
	data := mustJSON(map[string]any{"t": "settings", "values": values})
	s.mu.Lock()
	s.settingsJSON = data
	s.mu.Unlock()
	s.broadcast(data)
}
