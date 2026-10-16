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

// User is a Twitch account as it appears in support events.
type User struct{ Login, Name string }

// eventPayload covers the support events in both payload generations:
// 1.x puts accounts in "user"/"recipient" objects, 0.2.x flattens them.
type eventPayload struct {
	User      *userObj `json:"user"`
	Recipient *userObj `json:"recipient"`
	Reward    *struct {
		Title string `json:"title"`
	} `json:"reward"`
	Bits    int             `json:"bits"`
	Message json.RawMessage `json:"message"`

	UserName             string `json:"userName"`
	DisplayName          string `json:"displayName"`
	Title                string `json:"title"`
	RewardTitle          string `json:"rewardTitle"`
	RecipientUserName    string `json:"recipientUserName"`
	RecipientDisplayName string `json:"recipientDisplayName"`
}

type userObj struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

func parseEvent(data []byte) (eventPayload, bool) {
	var p eventPayload
	return p, json.Unmarshal(data, &p) == nil
}

func (p eventPayload) user() User {
	if p.User != nil && p.User.Login != "" {
		return User{p.User.Login, p.User.Name}
	}
	if p.UserName != "" {
		return User{p.UserName, p.DisplayName}
	}
	var m legacyMessage // 0.2.x cheers carry the chatter inside "message"
	if json.Unmarshal(p.Message, &m) == nil && m.Username != "" {
		return User{m.Username, m.DisplayName}
	}
	return User{}
}

func (p eventPayload) recipient() User {
	if p.Recipient != nil && p.Recipient.Login != "" {
		return User{p.Recipient.Login, p.Recipient.Name}
	}
	return User{p.RecipientUserName, p.RecipientDisplayName}
}

func (p eventPayload) rewardTitle() string {
	switch {
	case p.Reward != nil && p.Reward.Title != "":
		return p.Reward.Title
	case p.RewardTitle != "":
		return p.RewardTitle
	}
	return p.Title
}

func (p eventPayload) bits() int {
	if p.Bits > 0 {
		return p.Bits
	}
	var m struct {
		Bits int `json:"bits"`
	}
	_ = json.Unmarshal(p.Message, &m)
	return m.Bits
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
