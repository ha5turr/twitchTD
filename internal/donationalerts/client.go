// Package donationalerts receives donations from DonationAlerts in real time:
// OAuth authorization through the game's own web server, then the
// Centrifugo WebSocket channel of the streamer's donations.
package donationalerts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

type Config struct {
	Enabled      bool   `json:"enabled"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// Endpoints are the DonationAlerts addresses; tests point them elsewhere.
type Endpoints struct {
	Site      string // OAuth and REST API
	Websocket string // Centrifugo
}

var Production = Endpoints{
	Site:      "https://www.donationalerts.com",
	Websocket: "wss://centrifugo.donationalerts.com/connection/websocket",
}

const scopes = "oauth-user-show oauth-donation-subscribe"

type Token struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh"`
}

// TokenStore keeps the OAuth token between restarts.
type TokenStore interface {
	LoadToken() (Token, bool, error)
	SaveToken(Token) error
}

type Donation struct {
	Name     string
	Message  string
	Amount   float64
	Currency string
}

type Client struct {
	cfg      Config
	ep       Endpoints
	redirect string
	store    TokenStore
	onDonate func(Donation)
	http     *http.Client

	mu     sync.Mutex
	token  Token
	wake   chan struct{}
	status atomic.Value // string, for the dev console
	seq    atomic.Int64
}

// New creates a client. redirect is the full URL of the OAuth callback on
// the game server, as registered in the DonationAlerts application.
func New(cfg Config, ep Endpoints, redirect string, store TokenStore, onDonate func(Donation)) *Client {
	c := &Client{
		cfg: cfg, ep: ep, redirect: redirect, store: store, onDonate: onDonate,
		http: &http.Client{Timeout: 15 * time.Second},
		wake: make(chan struct{}, 1),
	}
	c.status.Store("не подключено")
	return c
}

func (c *Client) Status() string { return c.status.Load().(string) }

func (c *Client) setStatus(s string) {
	if c.Status() != s {
		log.Printf("donationalerts: %s", s)
	}
	c.status.Store(s)
}

// Handler serves /da/login, /da/callback and /da/status.
func (c *Client) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/da/login", func(w http.ResponseWriter, r *http.Request) {
		q := url.Values{
			"client_id": {c.cfg.ClientID}, "redirect_uri": {c.redirect},
			"response_type": {"code"}, "scope": {scopes},
		}
		http.Redirect(w, r, c.ep.Site+"/oauth/authorize?"+q.Encode(), http.StatusFound)
	})
	mux.HandleFunc("/da/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "DonationAlerts не прислал код авторизации: "+r.URL.Query().Get("error"), http.StatusBadRequest)
			return
		}
		tok, err := c.tokenRequest(r.Context(), url.Values{
			"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {c.redirect},
		})
		if err != nil {
			http.Error(w, "Не удалось получить токен DonationAlerts: "+err.Error(), http.StatusBadGateway)
			return
		}
		c.useToken(tok)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><title>DonationAlerts</title>`+
			`<body style="font:18px system-ui;background:#111;color:#eee;padding:40px">`+
			`DonationAlerts подключён, это окно можно закрыть.</body>`)
	})
	mux.HandleFunc("/da/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": c.Status()})
	})
	return mux
}

func (c *Client) useToken(t Token) {
	c.mu.Lock()
	c.token = t
	c.mu.Unlock()
	if err := c.store.SaveToken(t); err != nil {
		log.Printf("donationalerts: saving token: %v", err)
	}
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *Client) currentToken() Token {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

func (c *Client) tokenRequest(ctx context.Context, form url.Values) (Token, error) {
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.ep.Site+"/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return Token{}, err
	}
	defer resp.Body.Close()
	var out struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Token{}, fmt.Errorf("token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK || out.Access == "" {
		return Token{}, fmt.Errorf("token request failed: %d %s %s", resp.StatusCode, out.Error, out.Message)
	}
	return Token{Access: out.Access, Refresh: out.Refresh}, nil
}

var errUnauthorized = errors.New("unauthorized")

func (c *Client) api(ctx context.Context, method, path string, body, out any) error {
	var rd *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		rd = bytes.NewReader(data)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequestWithContext(ctx, method, c.ep.Site+path, rd)
	req.Header.Set("Authorization", "Bearer "+c.currentToken().Access)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: status %d", method, path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Run keeps the donation channel open until ctx ends. Without a token it
// waits for the streamer to authorize through /da/login.
func (c *Client) Run(ctx context.Context) {
	if t, ok, err := c.store.LoadToken(); err != nil {
		log.Printf("donationalerts: loading token: %v", err)
	} else if ok {
		c.mu.Lock()
		c.token = t
		c.mu.Unlock()
	}
	backoff := 2 * time.Second
	for ctx.Err() == nil {
		if c.currentToken().Access == "" {
			c.setStatus("нужна авторизация: откройте /da/login")
			select {
			case <-ctx.Done():
				return
			case <-c.wake:
			}
			continue
		}
		start := time.Now()
		err := c.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errUnauthorized) {
			if rerr := c.refresh(ctx); rerr != nil {
				c.setStatus("токен устарел, авторизуйтесь заново: /da/login")
				c.mu.Lock()
				c.token = Token{}
				c.mu.Unlock()
			}
			continue
		}
		if time.Since(start) > time.Minute {
			backoff = 2 * time.Second
		}
		c.setStatus(fmt.Sprintf("переподключение: %v", err))
		select {
		case <-ctx.Done():
			return
		case <-c.wake:
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, time.Minute)
	}
}

func (c *Client) refresh(ctx context.Context) error {
	t := c.currentToken()
	if t.Refresh == "" {
		return errUnauthorized
	}
	nt, err := c.tokenRequest(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {t.Refresh}, "scope": {scopes}})
	if err != nil {
		return err
	}
	if nt.Refresh == "" {
		nt.Refresh = t.Refresh
	}
	c.useToken(nt)
	return nil
}

// frame is a Centrifugo v2 JSON protocol message (reply or push).
type frame struct {
	ID     int64 `json:"id"`
	Result *struct {
		Client  string          `json:"client"`
		Channel string          `json:"channel"`
		Data    json.RawMessage `json:"data"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) session(ctx context.Context) error {
	var user struct {
		Data struct {
			ID    int64  `json:"id"`
			Token string `json:"socket_connection_token"`
		} `json:"data"`
	}
	if err := c.api(ctx, http.MethodGet, "/api/v1/user/oauth", nil, &user); err != nil {
		return err
	}
	dctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	conn, _, err := websocket.Dial(dctx, c.ep.Websocket, nil)
	cancel()
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)

	send := func(v any) error {
		data, _ := json.Marshal(v)
		wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return conn.Write(wctx, websocket.MessageText, data)
	}
	channel := fmt.Sprintf("$alerts:donation_%d", user.Data.ID)
	// handle delivers donations; Centrifugo may batch them with replies.
	handle := func(f frame) {
		if f.ID == 0 && f.Result != nil && f.Result.Channel == channel {
			if d, ok := parseDonation(f.Result.Data); ok {
				c.onDonate(d)
			}
		}
	}
	read := func(id int64) (frame, error) {
		for {
			rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			_, data, err := conn.Read(rctx)
			cancel()
			if err != nil {
				return frame{}, err
			}
			var reply *frame
			for _, f := range splitFrames(data) {
				if f.ID == id && reply == nil {
					reply = &f
				} else {
					handle(f)
				}
			}
			if reply != nil {
				if reply.Error != nil {
					return *reply, fmt.Errorf("centrifugo error %d: %s", reply.Error.Code, reply.Error.Message)
				}
				return *reply, nil
			}
		}
	}

	if err := send(map[string]any{"params": map[string]string{"token": user.Data.Token}, "id": 1}); err != nil {
		return err
	}
	hello, err := read(1)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	if hello.Result == nil || hello.Result.Client == "" {
		return errors.New("connect: no client id")
	}

	var sub struct {
		Channels []struct {
			Channel string `json:"channel"`
			Token   string `json:"token"`
		} `json:"channels"`
	}
	body := map[string]any{"channels": []string{channel}, "client": hello.Result.Client}
	if err := c.api(ctx, http.MethodPost, "/api/v1/centrifuge/subscribe", body, &sub); err != nil {
		return err
	}
	if len(sub.Channels) == 0 {
		return errors.New("subscribe: no channel token")
	}
	if err := send(map[string]any{"params": map[string]string{"channel": channel, "token": sub.Channels[0].Token}, "method": 1, "id": 2}); err != nil {
		return err
	}
	if _, err := read(2); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	c.setStatus("подключено")

	sctx, stop := context.WithCancel(ctx)
	defer stop()
	go func() { // keep the connection alive through proxies
		t := time.NewTicker(25 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-sctx.Done():
				return
			case <-t.C:
				if send(map[string]any{"method": 7, "id": 100 + c.seq.Add(1)}) != nil {
					stop()
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
		for _, f := range splitFrames(data) {
			handle(f)
		}
	}
}

// splitFrames decodes a WebSocket message, which Centrifugo may fill with
// several newline-separated JSON replies.
func splitFrames(data []byte) []frame {
	var out []frame
	for _, line := range bytes.Split(data, []byte("\n")) {
		var f frame
		if len(bytes.TrimSpace(line)) > 0 && json.Unmarshal(line, &f) == nil {
			out = append(out, f)
		}
	}
	return out
}

// parseDonation reads a publication: {"data": {donation}} (Centrifugo wraps
// the published payload once more in "data").
func parseDonation(raw json.RawMessage) (Donation, bool) {
	var pub struct {
		Data struct {
			Name     string  `json:"name"`
			Username string  `json:"username"`
			Message  string  `json:"message"`
			Amount   float64 `json:"amount"`
			Currency string  `json:"currency"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &pub) != nil || pub.Data.Amount <= 0 {
		return Donation{}, false
	}
	d := pub.Data
	return Donation{Name: d.Username, Message: d.Message, Amount: d.Amount, Currency: d.Currency}, true
}
