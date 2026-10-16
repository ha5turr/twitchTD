// Package streamerbot connects to the Streamer.bot WebSocket server, turns
// Twitch events into game input and sends chat replies back.
package streamerbot

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

type Config struct {
	Enabled  bool   `json:"enabled"`
	URL      string `json:"url"`      // Streamer.bot WebSocket server, default ws://127.0.0.1:8080/
	Password string `json:"password"` // only when authentication is enabled in Streamer.bot
	// SendAsBot sends replies from the bot account connected in Streamer.bot
	// instead of the broadcaster account.
	SendAsBot bool `json:"sendAsBot"`
	// ReplyAction, when set, sends replies by running this Streamer.bot action
	// with a "message" argument instead of the built-in SendMessage request.
	ReplyAction string `json:"replyAction"`
}

func DefaultConfig() Config {
	return Config{URL: "ws://127.0.0.1:8080/", SendAsBot: true}
}

// Role values Streamer.bot uses for Twitch users.
const (
	RoleViewer      = 1
	RoleVIP         = 2
	RoleModerator   = 3
	RoleBroadcaster = 4
)

type Chat struct {
	Login, Name, Text string
	Role              int
}

type Handlers struct {
	Chat    func(Chat)
	Present func(logins []string)
}

type Client struct {
	cfg  Config
	h    Handlers
	out  chan []byte
	seq  atomic.Int64
	live atomic.Bool
}

func New(cfg Config, h Handlers) *Client {
	return &Client{cfg: cfg, h: h, out: make(chan []byte, 64)}
}

// Connected reports whether the client is currently authenticated and subscribed.
func (c *Client) Connected() bool { return c.live.Load() }

// Run keeps a connection open until ctx is cancelled, reconnecting with backoff.
func (c *Client) Run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		start := time.Now()
		err := c.session(ctx)
		c.live.Store(false)
		if ctx.Err() != nil {
			return
		}
		if time.Since(start) > 30*time.Second {
			backoff = time.Second
		}
		log.Printf("streamer.bot: %v; reconnecting in %s", err, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

// Send queues a chat message. It is dropped while disconnected.
func (c *Client) Send(message string) {
	var req map[string]any
	if c.cfg.ReplyAction != "" {
		req = map[string]any{
			"request": "DoAction", "id": c.nextID(),
			"action": map[string]any{"name": c.cfg.ReplyAction},
			"args":   map[string]any{"message": message},
		}
	} else {
		req = map[string]any{
			"request": "SendMessage", "id": c.nextID(),
			"platform": "twitch", "bot": c.cfg.SendAsBot, "internal": false, "message": message,
		}
	}
	data, _ := json.Marshal(req)
	select {
	case c.out <- data:
	default:
		log.Printf("streamer.bot: outgoing queue full, dropped: %s", message)
	}
}

func (c *Client) nextID() string { return "td-" + strconv.FormatInt(c.seq.Add(1), 10) }

type hello struct {
	Request        string `json:"request"`
	Authentication *struct {
		Salt      string `json:"salt"`
		Challenge string `json:"challenge"`
	} `json:"authentication"`
}

func (c *Client) session(ctx context.Context) error {
	dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	conn, _, err := websocket.Dial(dctx, c.cfg.URL, nil)
	cancel()
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	conn.SetReadLimit(4 << 20)

	// The server greets every client with Hello; it carries the auth
	// challenge when authentication is enabled.
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	_, data, err := conn.Read(rctx)
	cancel()
	if err != nil {
		return fmt.Errorf("waiting for Hello: %w", err)
	}
	var h hello
	if err := json.Unmarshal(data, &h); err != nil || h.Request != "Hello" {
		return fmt.Errorf("unexpected first message: %.200s", data)
	}
	if h.Authentication != nil {
		if c.cfg.Password == "" {
			return fmt.Errorf("Streamer.bot requires authentication, set streamerbot.password")
		}
		auth := AuthString(c.cfg.Password, h.Authentication.Salt, h.Authentication.Challenge)
		if err := c.write(ctx, conn, map[string]any{"request": "Authenticate", "id": c.nextID(), "authentication": auth}); err != nil {
			return err
		}
	}
	sub := map[string]any{
		"request": "Subscribe", "id": c.nextID(),
		"events": map[string][]string{"Twitch": {"ChatMessage", "PresentViewers"}},
	}
	if err := c.write(ctx, conn, sub); err != nil {
		return err
	}
	c.live.Store(true)
	log.Printf("streamer.bot: connected to %s", c.cfg.URL)

	sctx, stop := context.WithCancel(ctx)
	defer stop()
	go func() {
		defer stop()
		for {
			select {
			case <-sctx.Done():
				return
			case msg := <-c.out:
				wctx, cancel := context.WithTimeout(sctx, 5*time.Second)
				err := conn.Write(wctx, websocket.MessageText, msg)
				cancel()
				if err != nil {
					return
				}
			}
		}
	}()
	for {
		_, data, err := conn.Read(sctx)
		if err != nil {
			return err
		}
		c.dispatch(data)
	}
}

func (c *Client) write(ctx context.Context, conn *websocket.Conn, v any) error {
	data, _ := json.Marshal(v)
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return conn.Write(wctx, websocket.MessageText, data)
}

// AuthString implements Streamer.bot's challenge: base64(sha256(base64(sha256(password+salt)) + challenge)).
func AuthString(password, salt, challenge string) string {
	secret := sha256.Sum256([]byte(password + salt))
	auth := sha256.Sum256([]byte(base64.StdEncoding.EncodeToString(secret[:]) + challenge))
	return base64.StdEncoding.EncodeToString(auth[:])
}

type envelope struct {
	Event *struct {
		Source string `json:"source"`
		Type   string `json:"type"`
	} `json:"event"`
	Data   json.RawMessage `json:"data"`
	Status string          `json:"status"`
	Error  string          `json:"error"`
	ID     string          `json:"id"`
}

func (c *Client) dispatch(data []byte) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return
	}
	if env.Status == "error" {
		log.Printf("streamer.bot: request %s failed: %s", env.ID, env.Error)
		return
	}
	if env.Event == nil || env.Event.Source != "Twitch" {
		return
	}
	switch env.Event.Type {
	case "ChatMessage":
		if m, ok := ParseChat(env.Data); ok && c.h.Chat != nil {
			c.h.Chat(m)
		}
	case "PresentViewers":
		if logins := ParsePresent(env.Data); len(logins) > 0 && c.h.Present != nil {
			c.h.Present(logins)
		}
	}
}
