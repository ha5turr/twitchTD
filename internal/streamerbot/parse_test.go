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
