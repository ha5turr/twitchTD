package server

import (
	"strings"
	"testing"
)

func TestReplierBatchesAndDedupes(t *testing.T) {
	r := newReplier(nil)
	r.add("a", "нужно 50 золота")
	r.add("a", "нужно 50 золота")
	r.add("b", "клетка занята")
	if msg := r.next(); msg != "@a нужно 50 золота • @b клетка занята" {
		t.Fatalf("got %q", msg)
	}
	if r.next() != "" {
		t.Fatal("queue should be empty")
	}

	for i := 0; i < 30; i++ {
		r.add("user"+strings.Repeat("x", i), "очень длинный ответ про золото и постройки")
	}
	first := r.next()
	if len(first) > r.maxLen || r.next() == "" {
		t.Fatalf("long batches must split: first %d bytes", len(first))
	}
}

func TestReplierSendsCommandsAlone(t *testing.T) {
	r := newReplier(nil)
	r.add("a", "нужно 50 золота")
	r.command("!mute")
	r.add("b", "клетка занята")
	for _, want := range []string{"@a нужно 50 золота", "!mute", "@b клетка занята", ""} {
		if got := r.next(); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}
