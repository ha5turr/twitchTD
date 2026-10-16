package donationalerts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type memStore struct {
	mu  sync.Mutex
	tok Token
	ok  bool
}

func (m *memStore) LoadToken() (Token, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tok, m.ok, nil
}
func (m *memStore) SaveToken(t Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tok, m.ok = t, true
	return nil
}

// fakeDA serves the OAuth, REST and Centrifugo endpoints the client uses.
// The stored token starts expired so the refresh path runs first.
func fakeDA(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "R1" || r.Form.Get("client_secret") != "secret" {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": "fresh", "refresh_token": "R2"})
	})
	mux.HandleFunc("/api/v1/user/oauth", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fresh" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"data":{"id":42,"code":"streamer","socket_connection_token":"SCT"}}`))
	})
	mux.HandleFunc("/api/v1/centrifuge/subscribe", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Channels []string `json:"channels"`
			Client   string   `json:"client"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.Client != "client-uuid" || len(body.Channels) != 1 || body.Channels[0] != "$alerts:donation_42" {
			http.Error(w, "bad subscribe", http.StatusBadRequest)
			return
		}
		w.Write([]byte(`{"channels":[{"channel":"$alerts:donation_42","token":"CHT"}]}`))
	})
	mux.HandleFunc("/connection/websocket", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx := r.Context()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var m struct {
				ID     int               `json:"id"`
				Method int               `json:"method"`
				Params map[string]string `json:"params"`
			}
			json.Unmarshal(data, &m)
			switch {
			case m.ID == 1 && m.Params["token"] == "SCT":
				conn.Write(ctx, websocket.MessageText, []byte(`{"id":1,"result":{"client":"client-uuid","version":"2.2.1"}}`))
			case m.Method == 1 && m.Params["token"] == "CHT":
				// Reply and the first donation in one batched message.
				conn.Write(ctx, websocket.MessageText, []byte(`{"id":2,"result":{}}`+"\n"+
					`{"result":{"channel":"$alerts:donation_42","data":{"seq":1,"data":{"id":7,"name":"donation","username":"Василий","message":"TD-1234 удачи","amount":300,"currency":"RUB"}}}}`))
			}
		}
	})
	return httptest.NewServer(mux)
}

func TestClientReceivesDonation(t *testing.T) {
	srv := fakeDA(t)
	defer srv.Close()
	store := &memStore{tok: Token{Access: "expired", Refresh: "R1"}, ok: true}
	got := make(chan Donation, 1)
	ep := Endpoints{Site: srv.URL, Websocket: "ws" + strings.TrimPrefix(srv.URL, "http") + "/connection/websocket"}
	c := New(Config{Enabled: true, ClientID: "id", ClientSecret: "secret"}, ep, "http://127.0.0.1:8787/da/callback", store, func(d Donation) { got <- d })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	select {
	case d := <-got:
		if d.Name != "Василий" || d.Amount != 300 || d.Currency != "RUB" || !strings.Contains(d.Message, "TD-1234") {
			t.Fatalf("donation %+v", d)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("no donation; status %q", c.Status())
	}
	if tok, _, _ := store.LoadToken(); tok.Access != "fresh" || tok.Refresh != "R2" {
		t.Fatalf("refreshed token not saved: %+v", tok)
	}
	// The donation is batched with the subscribe reply, so it can arrive
	// a moment before the status flips.
	for i := 0; i < 50 && c.Status() != "подключено"; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if c.Status() != "подключено" {
		t.Fatalf("status %q", c.Status())
	}
}

func TestLoginRedirect(t *testing.T) {
	c := New(Config{ClientID: "abc"}, Production, "http://127.0.0.1:8787/da/callback", &memStore{}, nil)
	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/da/login", nil))
	loc := rec.Header().Get("Location")
	for _, want := range []string{"donationalerts.com/oauth/authorize", "client_id=abc", "response_type=code", "oauth-donation-subscribe", "redirect_uri=http%3A%2F%2F127.0.0.1%3A8787%2Fda%2Fcallback"} {
		if !strings.Contains(loc, want) {
			t.Fatalf("redirect %q lacks %q", loc, want)
		}
	}
}
