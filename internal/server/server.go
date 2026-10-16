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

type Server struct {
	game  *game.Game
	chat  chan game.ChatMessage
	files fs.FS

	mu      sync.Mutex
	clients map[*client]struct{}
	mapMsg  game.MapMsg
	mapJSON []byte
}

type client struct {
	send chan []byte
}

func New(g *game.Game, files fs.FS) *Server {
	return &Server{
		game:    g,
		chat:    make(chan game.ChatMessage, 256),
		files:   files,
		clients: map[*client]struct{}{},
	}
}

// Chat returns the channel that feeds chat lines into the game loop. It is the
// single entry point for Streamer.bot, the panel and simulation bots.
func (s *Server) Chat() chan<- game.ChatMessage { return s.chat }

// CurrentMap returns the latest map message.
func (s *Server) CurrentMap() game.MapMsg {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mapMsg
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServerFS(s.files))
	mux.HandleFunc("/ws", s.serveWS)
	return mux
}

// Run owns the game: every mutation happens on this goroutine.
func (s *Server) Run(ctx context.Context) {
	rate := s.game.Cfg.TickRate
	dt := 1 / float64(rate)
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-s.chat:
			s.game.HandleChat(m)
		case <-ticker.C:
			s.game.Tick(dt)
			if s.game.TakeMapChanged() {
				s.publishMap()
			}
			st := s.game.StateMessage()
			for _, e := range st.Events {
				if e.Kind == "reply" {
					// Until Streamer.bot is wired up, replies only go to the log.
					log.Printf("reply @%s: %s", e.User, e.Text)
				}
			}
			s.broadcast(mustJSON(st))
		}
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

// inbound is what clients may send: chat lines from the panel or dev console.
type inbound struct {
	T    string `json:"t"`
	User string `json:"user"`
	Text string `json:"text"`
	Mod  bool   `json:"mod"`
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
			if json.Unmarshal(data, &m) == nil && m.T == "chat" {
				s.chat <- game.ChatMessage{User: m.User, Text: m.Text, Mod: m.Mod}
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
