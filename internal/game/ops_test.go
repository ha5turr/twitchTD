package game

import (
	"strings"
	"testing"
)

type fakeOps struct{ updates, rollbacks []string }

func (f *fakeOps) Update(by string)   { f.updates = append(f.updates, by) }
func (f *fakeOps) Rollback(by string) { f.rollbacks = append(f.rollbacks, by) }

func TestReleaseCommands(t *testing.T) {
	g, _ := newTestGame(t)
	g.HandleChat(ChatMessage{User: "modder", Mod: true, Text: "!tdupdate"})
	if r := lastReply(g); !strings.Contains(r, "не настроены") {
		t.Fatalf("without an operator the server says so: %q", r)
	}
	ops := &fakeOps{}
	g.Ops = ops
	g.HandleChat(ChatMessage{User: "viewer", Text: "!tdupdate"})
	if len(ops.updates) != 0 {
		t.Fatal("viewers cannot update the server")
	}
	g.HandleChat(ChatMessage{User: "modder", Mod: true, Text: "!tdupdate"})
	g.HandleChat(ChatMessage{User: "modder", Mod: true, Text: "!tdrollback"})
	if len(ops.updates) != 1 || len(ops.rollbacks) != 1 || ops.updates[0] != "modder" {
		t.Fatalf("mods update and roll back: %+v", ops)
	}
	g.HandleChat(ChatMessage{User: "modder", Mod: true, Text: "!tdversion"})
	if r := lastReply(g); !strings.Contains(r, "v"+Version) {
		t.Fatalf("!tdversion tells the version: %q", r)
	}
}
