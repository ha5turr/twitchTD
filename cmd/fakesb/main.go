// Command fakesb imitates the Streamer.bot WebSocket server for local
// testing: it greets clients (optionally with an auth challenge), plays a
// short chat script with a few viewers and prints every reply the game sends.
// Lines typed on stdin as "login: text" are sent as chat too.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
)

type viewer struct{ login, name string }

var viewers = []viewer{{"vasya_tank", "Вася_Танк"}, {"grimreaper77", "GrimReaper77"}, {"ledi_moroz", "ЛедиМороз"}, {"pyro_petya", "pyro_Petya"}}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	password := flag.String("password", "", "require this password (empty = no auth)")
	flag.Parse()

	typed := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			typed <- sc.Text()
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		log.Printf("client connected")
		serve(r.Context(), conn, *password, typed)
		log.Printf("client gone")
	})
	log.Printf("fake Streamer.bot on ws://%s/", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func serve(ctx context.Context, conn *websocket.Conn, password string, typed <-chan string) {
	send := func(v any) {
		data, _ := json.Marshal(v)
		conn.Write(ctx, websocket.MessageText, data)
	}
	hello := map[string]any{"request": "Hello", "info": map[string]any{"name": "fakesb", "version": "1.0.0"}}
	if password != "" {
		hello["authentication"] = map[string]string{"salt": "c2FsdA==", "challenge": "Y2hhbA=="}
	}
	send(hello)

	subscribed := make(chan struct{})
	go func() {
		var once bool
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var m map[string]any
			json.Unmarshal(data, &m)
			switch m["request"] {
			case "SendMessage":
				fmt.Printf("BOT → %v\n", m["message"])
			case "DoAction":
				fmt.Printf("ACTION %v → %v\n", m["action"], m["args"])
			case "Subscribe":
				log.Printf("subscribed to %v", m["events"])
				if !once {
					once = true
					close(subscribed)
				}
			default:
				log.Printf("request %v", m["request"])
			}
			send(map[string]any{"status": "ok", "id": m["id"]})
		}
	}()
	select {
	case <-subscribed:
	case <-ctx.Done():
		return
	}

	chat := func(v viewer, role int, text string) {
		send(map[string]any{
			"timeStamp": time.Now().Format(time.RFC3339),
			"event":     map[string]string{"source": "Twitch", "type": "ChatMessage"},
			"data":      map[string]any{"user": map[string]any{"login": v.login, "name": v.name, "role": role}, "text": text},
		})
	}
	script := []string{"привет!", "!me", "!build C5 arrow", "!build Z9", "!b d4 magic", "!towers", "!up C5", "!sell D4"}
	tick := time.NewTicker(4 * time.Second)
	present := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	defer present.Stop()
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return
		case line := <-typed:
			login, text, ok := strings.Cut(line, ":")
			if !ok {
				login, text = "streamer_me", line
			}
			login = strings.TrimSpace(login)
			role := 1
			if login == "streamer_me" {
				role = 4
			}
			chat(viewer{login, login}, role, strings.TrimSpace(text))
		case <-present.C:
			var users []map[string]string
			for _, v := range viewers {
				users = append(users, map[string]string{"login": v.login, "display": v.name})
			}
			send(map[string]any{
				"event": map[string]string{"source": "Twitch", "type": "PresentViewers"},
				"data":  map[string]any{"users": users},
			})
		case <-tick.C:
			v := viewers[rand.IntN(len(viewers))]
			chat(v, 1, script[i%len(script)])
		}
	}
}
