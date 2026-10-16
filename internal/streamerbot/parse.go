package streamerbot

import "encoding/json"

// Streamer.bot changed the Twitch payloads between versions, so both shapes
// are accepted:
//
//	1.x:   {"user":{"login","name","role"},"text":"..."}
//	0.2.x: {"message":{"username","displayName","role","message":"...","internal"}}
type chatPayload struct {
	User *struct {
		Login string `json:"login"`
		Name  string `json:"name"`
		Role  int    `json:"role"`
	} `json:"user"`
	Text    string          `json:"text"`
	Message json.RawMessage `json:"message"`
}

type legacyMessage struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        int    `json:"role"`
	Message     string `json:"message"`
	Internal    bool   `json:"internal"`
}

func ParseChat(data []byte) (Chat, bool) {
	var p chatPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return Chat{}, false
	}
	if p.User != nil && p.User.Login != "" {
		text := p.Text
		if text == "" {
			_ = json.Unmarshal(p.Message, &text) // "message" as a plain string
		}
		return Chat{Login: p.User.Login, Name: p.User.Name, Text: text, Role: p.User.Role}, text != ""
	}
	var m legacyMessage
	if len(p.Message) > 0 && json.Unmarshal(p.Message, &m) == nil && m.Username != "" && !m.Internal {
		return Chat{Login: m.Username, Name: m.DisplayName, Text: m.Message, Role: m.Role}, m.Message != ""
	}
	return Chat{}, false
}

// ParsePresent extracts logins from a PresentViewers payload.
func ParsePresent(data []byte) []string {
	var p struct {
		Users []struct {
			Login    string `json:"login"`
			UserName string `json:"userName"`
			Name     string `json:"name"`
		} `json:"users"`
	}
	if json.Unmarshal(data, &p) != nil {
		return nil
	}
	var out []string
	for _, u := range p.Users {
		switch {
		case u.Login != "":
			out = append(out, u.Login)
		case u.UserName != "":
			out = append(out, u.UserName)
		case u.Name != "":
			out = append(out, u.Name)
		}
	}
	return out
}
