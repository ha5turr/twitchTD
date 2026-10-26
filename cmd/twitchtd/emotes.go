package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

// emoteChannels lists the channels whose 7TV, BTTV and FFZ emotes the
// overlay loads for !s lines, by numeric Twitch ID, in config order: the
// old twitchChannelId first, then emoteChannels. An entry of digits is an
// ID already; a nick is looked up on api.ivr.fi (only the nick goes there).
// A nick that cannot be resolved is skipped with a log line.
func emoteChannels(first string, rest []string) []string {
	var ids []string
	seen := map[string]bool{}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, entry := range append([]string{first}, rest...) {
		entry = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(entry)), "@")
		if entry == "" {
			continue
		}
		id := entry
		if !allDigits(entry) {
			var err error
			if id, err = twitchID(client, entry); err != nil {
				log.Printf("emotes: channel %q: %v", entry, err)
				continue
			}
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func allDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// twitchID looks a channel's numeric ID up by its nick.
func twitchID(client *http.Client, login string) (string, error) {
	resp, err := client.Get("https://api.ivr.fi/v2/twitch/user?login=" + url.QueryEscape(login))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api.ivr.fi: %s", resp.Status)
	}
	var users []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return "", err
	}
	if len(users) == 0 || !allDigits(users[0].ID) {
		return "", fmt.Errorf("no such channel")
	}
	return users[0].ID, nil
}
