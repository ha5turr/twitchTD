package streamerbot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// fakeBot mimics the Streamer.bot WebSocket server closely enough to check
// the handshake, the subscription and both directions of chat.
func fakeBot(t *testing.T, password string, got chan<- map[string]any, events <-chan string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx := r.Context()
		hello := `{"request":"Hello","info":{"version":"1.0.0"}}`
		if password != "" {
			hello = `{"request":"Hello","authentication":{"salt":"c2FsdA==","challenge":"Y2hhbA=="}}`
		}
		conn.Write(ctx, websocket.MessageText, []byte(hello))
		go func() {
			for ev := range events {
				conn.Write(ctx, websocket.MessageText, []byte(ev))
			}
		}()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var m map[string]any
			json.Unmarshal(data, &m)
			if m["request"] == "Authenticate" && m["authentication"] != AuthString(password, "c2FsdA==", "Y2hhbA==") {
				conn.Close(websocket.StatusPolicyViolation, "bad auth")
				return
			}
			got <- m
		}
	}))
}

func TestClientHandshakeAndChat(t *testing.T) {
	got := make(chan map[string]any, 10)
	events := make(chan string, 10)
	srv := fakeBot(t, "secret", got, events)
	defer srv.Close()

	chats := make(chan Chat, 1)
	c := New(Config{Enabled: true, URL: "ws" + strings.TrimPrefix(srv.URL, "http"), Password: "secret", SendAsBot: true, ReplyPrefix: "/me "},
		Handlers{Chat: func(m Chat) { chats <- m }})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	want := []string{"Authenticate", "Subscribe"}
	for _, req := range want {
		select {
		case m := <-got:
			if m["request"] != req {
				t.Fatalf("got request %v, want %s", m["request"], req)
			}
			if req == "Subscribe" {
				ev := m["events"].(map[string]any)["Twitch"].([]any)
				if len(ev) == 0 || ev[0] != "ChatMessage" {
					t.Fatalf("subscription %v", ev)
				}
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for %s", req)
		}
	}

	events <- `{"timeStamp":"2026-10-16T12:00:00Z","event":{"source":"Twitch","type":"ChatMessage"},"data":{"user":{"login":"vasya","name":"Вася","role":1},"text":"!me"}}`
	select {
	case m := <-chats:
		if m.Login != "vasya" || m.Text != "!me" {
			t.Fatalf("chat %+v", m)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("chat event not delivered")
	}

	c.Send("@Вася золото 120/1000")
	select {
	case m := <-got:
		if m["request"] != "SendMessage" || m["message"] != "/me @Вася золото 120/1000" || m["bot"] != true {
			t.Fatalf("send %v", m)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reply not sent")
	}
}
