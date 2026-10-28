package server

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"
)

// replier batches bot replies so a busy chat never trips Twitch rate limits:
// at most one message per interval, several replies joined into it.
type replier struct {
	say      func(string)
	interval time.Duration
	maxLen   int

	mu    sync.Mutex
	queue []string
	alone map[string]bool // lines that go out as a message of their own: commands for other bots
}

const maxQueued = 40

func newReplier(say func(string)) *replier {
	return &replier{say: say, interval: 1500 * time.Millisecond, maxLen: 450}
}

// add queues a line; an empty user makes it an announcement to the whole chat.
func (r *replier) add(user, text string) {
	line := text
	if user != "" {
		line = "@" + user + " " + text
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, q := range r.queue {
		if q == line {
			return // the same answer is already on its way
		}
	}
	if len(r.queue) < maxQueued {
		r.queue = append(r.queue, line)
	}
}

// command queues a chat command for another bot (say !mute): it is sent
// on its own, never joined to other lines, or the bot would not see it.
func (r *replier) command(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.alone == nil {
		r.alone = map[string]bool{}
	}
	if slices.Contains(r.queue, text) {
		return // already on its way
	}
	r.alone[text] = true
	if len(r.queue) < maxQueued {
		r.queue = append(r.queue, text)
	}
}

// next takes as many queued lines as fit into one message.
func (r *replier) next() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var b strings.Builder
	n := 0
	for _, line := range r.queue {
		if r.alone[line] {
			if n == 0 {
				b.WriteString(line)
				n++
			}
			break // a command goes alone, and nothing joins it
		}
		sep := 0
		if n > 0 {
			sep = len(" • ")
		}
		if n > 0 && b.Len()+sep+len(line) > r.maxLen {
			break
		}
		if n > 0 {
			b.WriteString(" • ")
		}
		b.WriteString(line)
		n++
	}
	r.queue = r.queue[n:]
	delete(r.alone, b.String())
	return b.String()
}

func (r *replier) run(ctx context.Context) {
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if msg := r.next(); msg != "" {
				r.say(msg)
			}
		}
	}
}
