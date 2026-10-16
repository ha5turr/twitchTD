package server

import (
	"context"
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
}

const maxQueued = 40

func newReplier(say func(string)) *replier {
	return &replier{say: say, interval: 1500 * time.Millisecond, maxLen: 450}
}

func (r *replier) add(user, text string) {
	line := "@" + user + " " + text
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

// next takes as many queued lines as fit into one message.
func (r *replier) next() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var b strings.Builder
	n := 0
	for _, line := range r.queue {
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
