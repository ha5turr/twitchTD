package game

import (
	"strings"
	"testing"
)

// fakeAdmin stands in for the settings manager: values as JSON decodes them.
type fakeAdmin struct{ v map[string]any }

func (f *fakeAdmin) Values() map[string]any { return f.v }
func (f *fakeAdmin) Change(c map[string]any) error {
	for k, v := range c {
		f.v[k] = v
	}
	return nil
}

func newSoundGame(t *testing.T) (*Game, *fakeAdmin) {
	g, _ := newTestGame(t)
	groups := map[string]any{}
	for _, sg := range SoundGroups {
		groups[sg.Key] = 1.0
	}
	a := &fakeAdmin{v: map[string]any{"volume": 0.6, "muted": false, "ambient": true, "ambientVolume": 0.7, "soundGroups": groups}}
	g.Admin = a
	return g, a
}

func TestSoundCommands(t *testing.T) {
	g, a := newSoundGame(t)
	mod := func(text string) { g.HandleChat(ChatMessage{User: "moder", Text: text, Mod: true}) }

	mod("!tdvolume 35")
	if a.v["volume"] != 0.35 {
		t.Fatalf("volume %v", a.v["volume"])
	}
	mod("!tdmute")
	if a.v["muted"] != true {
		t.Fatal("mute toggles on")
	}
	if r := lastReply(g); !strings.Contains(r, "выключен") {
		t.Fatalf("reply %q", r)
	}
	mod("!tdvolume 50")
	if a.v["muted"] != false || a.v["volume"] != 0.5 {
		t.Fatal("setting a volume unmutes")
	}
	mod("!tdambient off")
	if a.v["ambient"] != false {
		t.Fatal("ambient off")
	}
	mod("!tdambient 40")
	if a.v["ambient"] != true || a.v["ambientVolume"] != 0.4 {
		t.Fatalf("ambient 40: %v %v", a.v["ambient"], a.v["ambientVolume"])
	}
	mod("!tdsound бой 20")
	groups := a.v["soundGroups"].(map[string]any)
	if groups["combat"] != 0.2 || groups["alerts"] != 1.0 {
		t.Fatalf("groups %v", groups)
	}
	mod("!tdsound донаты off")
	if a.v["soundGroups"].(map[string]any)["support"] != 0.0 {
		t.Fatal("group off")
	}
	mod("!tdsound")
	if r := lastReply(g); !strings.Contains(r, "бой 20%") || !strings.Contains(r, "донаты и награды 0%") {
		t.Fatalf("status %q", r)
	}
	mod("!tdvolume 150")
	if a.v["volume"] != 0.5 || !strings.Contains(lastReply(g), "Формат") {
		t.Fatal("out of range is refused with usage")
	}
}

func TestSoundCommandsNeedMod(t *testing.T) {
	g, a := newSoundGame(t)
	g.HandleChat(ChatMessage{User: "viewer", Text: "!tdmute"})
	if a.v["muted"] != false {
		t.Fatal("viewers cannot mute the stream")
	}
}
