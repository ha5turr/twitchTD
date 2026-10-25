package streamerbot

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestParseChatNewFormat(t *testing.T) {
	data := []byte(`{"user":{"id":"1","login":"grim_77","name":"Grim_77","role":3},"messageId":"x","text":"!build C4 arrow","isTest":false}`)
	m, ok := ParseChat(data)
	if !ok || m.Login != "grim_77" || m.Name != "Grim_77" || m.Text != "!build C4 arrow" || m.Role != RoleModerator {
		t.Fatalf("got %+v %v", m, ok)
	}
}

func TestParseChatLegacyFormat(t *testing.T) {
	data := []byte(`{"message":{"msgId":"x","userId":1,"username":"vasya","displayName":"Вася","role":1,"message":"!me","internal":false}}`)
	m, ok := ParseChat(data)
	if !ok || m.Login != "vasya" || m.Name != "Вася" || m.Text != "!me" {
		t.Fatalf("got %+v %v", m, ok)
	}
	internal := []byte(`{"message":{"username":"bot","message":"hi","internal":true}}`)
	if _, ok := ParseChat(internal); ok {
		t.Fatal("internal messages must be skipped")
	}
}

func TestParsePresent(t *testing.T) {
	got := ParsePresent([]byte(`{"users":[{"login":"a"},{"userName":"b"},{"display":"C","name":"c"}]}`))
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("got %v", got)
	}
}

func TestAuthString(t *testing.T) {
	// Recompute by the documented steps to pin the concatenation order.
	secret := sha256.Sum256([]byte("pass" + "salt"))
	want := sha256.Sum256([]byte(base64.StdEncoding.EncodeToString(secret[:]) + "challenge"))
	if got := AuthString("pass", "salt", "challenge"); got != base64.StdEncoding.EncodeToString(want[:]) {
		t.Fatalf("auth string %s", got)
	}
}

func TestParseSupportEvents(t *testing.T) {
	reward, _ := parseEvent([]byte(`{"user":{"login":"vasya","name":"Вася"},"reward":{"title":"Мешок золота","cost":500},"userInput":""}`))
	if u := reward.user(); u.Login != "vasya" || reward.rewardTitle() != "Мешок золота" {
		t.Fatalf("reward %+v %q", u, reward.rewardTitle())
	}
	ids, _ := parseEvent([]byte(`{"id":"red-1","user":{"login":"vasya"},"reward":{"id":"rw-9","title":"Мешок золота"}}`))
	if r := ids.redemption(); r != (Redemption{"Мешок золота", "rw-9", "red-1"}) {
		t.Fatalf("redemption ids %+v", r)
	}
	legacyReward, _ := parseEvent([]byte(`{"userName":"petya","displayName":"Петя","title":"Дождь стрел","cost":100}`))
	if u := legacyReward.user(); u.Login != "petya" || legacyReward.rewardTitle() != "Дождь стрел" {
		t.Fatalf("legacy reward %+v %q", u, legacyReward.rewardTitle())
	}
	cheer, _ := parseEvent([]byte(`{"user":{"login":"a","name":"A"},"bits":250,"text":"cheer250"}`))
	if cheer.bits() != 250 || cheer.user().Login != "a" {
		t.Fatal("cheer")
	}
	legacyCheer, _ := parseEvent([]byte(`{"message":{"username":"b","displayName":"B","bits":100,"message":"cheer100"}}`))
	if legacyCheer.bits() != 100 || legacyCheer.user().Login != "b" {
		t.Fatal("legacy cheer")
	}
	gift, _ := parseEvent([]byte(`{"user":{"login":"g","name":"G"},"recipient":{"login":"r","name":"R"},"subTier":"1000"}`))
	if gift.user().Login != "g" || gift.recipient().Login != "r" {
		t.Fatal("gift sub")
	}
}

func TestParseChatEmotes(t *testing.T) {
	data := []byte(`{"user":{"login":"vasya","name":"Вася"},"text":"!s KEKW hi catJAM",
		"emotes":[{"type":"Twitch","name":"KEKW","startIndex":3,"endIndex":6,"imageUrl":"https://static-cdn.jtvnw.net/emoticons/v2/1/default/dark/2.0"}],
		"parts":[{"type":"text","text":"!s "},{"type":"emote","text":"catJAM","source":"SevenTV","imageUrl":"https://cdn.7tv.app/emote/x/2x.webp"},
			{"type":"emote","text":"evil","imageUrl":"javascript:alert(1)"}]}`)
	c, ok := ParseChat(data)
	if !ok || c.Emotes["KEKW"] == "" || c.Emotes["catJAM"] != "https://cdn.7tv.app/emote/x/2x.webp" {
		t.Fatalf("emotes come from both the list and the parts: %+v", c.Emotes)
	}
	if _, bad := c.Emotes["evil"]; bad {
		t.Fatal("only https images pass")
	}
	legacy := []byte(`{"message":{"username":"vasya","displayName":"Вася","message":"!s Kappa","emotes":[{"name":"Kappa","imageUrl":"https://static-cdn.jtvnw.net/emoticons/v2/25/default/dark/2.0"}]}}`)
	if c, _ := ParseChat(legacy); c.Emotes["Kappa"] == "" {
		t.Fatal("and from the older payload")
	}
}
