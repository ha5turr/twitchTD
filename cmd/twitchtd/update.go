package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"twitchtd/internal/game"
	"twitchtd/internal/server"
	"twitchtd/internal/updater"
)

// Releases on the streaming PC. !tdupdate downloads the latest GitHub release
// next to the running exe as twitchtd.next.exe, waits for the build phase,
// leaves a marker and exits with exitUpdate; run.ps1 swaps the files and
// starts the new build, which reads the marker and tells the chat how it
// went. !tdrollback does the same with the previous build run.ps1 kept, and
// !tdrestart with the same build, at once.

const (
	exitUpdate = 3 // run.ps1: swap in twitchtd.next.exe and start again
	nextExe    = "twitchtd.next.exe"
	prevExe    = "twitchtd.prev.exe"
	markerFile = "update.json"
)

type updateConfig struct {
	Repo  string `json:"repo"`  // owner/name on GitHub
	Token string `json:"token"` // only for a private repository
}

type marker struct {
	From    string `json:"from"`
	To      string `json:"to"`                // empty for a rollback
	Restart bool   `json:"restart,omitempty"` // !tdrestart: the same build again
}

// ops runs the release commands chat sends through the game.
type ops struct {
	cli  updater.Client
	dir  string // where the exe lives; run.ps1 keeps its siblings there
	srv  *server.Server
	stop context.CancelFunc
	busy atomic.Bool
	code atomic.Int32
}

func newOps(cfg updateConfig, srv *server.Server, stop context.CancelFunc) *ops {
	dir := "."
	if exe, err := os.Executable(); err == nil {
		dir = filepath.Dir(exe)
	}
	return &ops{cli: updater.Client{Repo: cfg.Repo, Token: cfg.Token}, dir: dir, srv: srv, stop: stop}
}

// exitCode is what main exits with once the game has saved.
func (o *ops) exitCode() int { return int(o.code.Load()) }

func (o *ops) say(text string) { o.srv.Do(func(g *game.Game) { g.Announce(text) }) }

// supervised reports whether run.ps1 started us: only then will someone
// swap the files and start the server again.
func supervised() bool { return os.Getenv("TWITCHTD_SUPERVISED") == "1" }

func (o *ops) begin() bool {
	if !supervised() {
		o.say("обновление работает, только когда сервер запущен через run.ps1")
		return false
	}
	if !o.busy.CompareAndSwap(false, true) {
		o.say("обновление уже идёт")
		return false
	}
	return true
}

func (o *ops) Update(by string) {
	if !o.begin() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		rel, err := o.cli.Latest(ctx)
		if err != nil {
			o.fail("не удалось найти релиз", err)
			return
		}
		if !updater.Newer(rel.Version, game.Version) {
			o.say(fmt.Sprintf("уже стоит последняя версия v%s", game.Version))
			o.busy.Store(false)
			return
		}
		o.say(fmt.Sprintf("%s запустил обновление: качаю v%s", by, rel.Version))
		if err := o.cli.Download(ctx, rel, filepath.Join(o.dir, nextExe)); err != nil {
			o.fail("не удалось скачать v"+rel.Version, err)
			return
		}
		o.restart(rel.Version)
	}()
}

func (o *ops) Rollback(by string) {
	if !o.begin() {
		return
	}
	go func() {
		if err := copyFile(filepath.Join(o.dir, prevExe), filepath.Join(o.dir, nextExe)); err != nil {
			o.fail("откатываться не на что", err)
			return
		}
		o.say(by + " запустил откат на прошлую версию")
		o.restart("")
	}()
}

// Restart starts the same build afresh, at once: config.json is read again,
// the game saves on its way out and resumes, an interrupted wave replayed.
func (o *ops) Restart(by string) {
	if !supervised() {
		o.say("перезапуск работает, только когда сервер запущен через run.cmd")
		return
	}
	if !o.busy.CompareAndSwap(false, true) {
		o.say("уже идёт обновление или перезапуск")
		return
	}
	go func() {
		data, _ := json.Marshal(marker{From: game.Version, To: game.Version, Restart: true})
		if err := os.WriteFile(filepath.Join(o.dir, markerFile), data, 0o644); err != nil {
			o.fail("не удалось записать отметку перезапуска", err)
			return
		}
		o.say(by + " перезапускает таверну, вернусь через пару секунд")
		time.Sleep(2 * time.Second) // let the line reach the chat
		o.exit(exitUpdate)          // run.ps1 starts us again; with no next exe nothing is swapped
	}()
}

func (o *ops) fail(what string, err error) {
	log.Printf("update: %s: %v", what, err)
	o.say(what + ", играем дальше на v" + game.Version)
	o.busy.Store(false)
}

// restart waits for the build phase, so no wave is cut short, then stops
// the server with exitUpdate; the game saves on its way out.
func (o *ops) restart(to string) {
	o.say("перезапущусь в фазе стройки")
	for {
		ch := make(chan bool, 1)
		o.srv.Do(func(g *game.Game) { ch <- g.Phase != game.PhaseWave })
		if <-ch {
			break
		}
		time.Sleep(time.Second)
	}
	data, _ := json.Marshal(marker{From: game.Version, To: to})
	if err := os.WriteFile(filepath.Join(o.dir, markerFile), data, 0o644); err != nil {
		o.fail("не удалось записать отметку обновления", err)
		return
	}
	o.say("перезапуск, вернусь через пару секунд")
	time.Sleep(2 * time.Second) // let the line reach the chat
	o.exit(exitUpdate)
}

// exitWait is how long a stop may take before the process ends anyway.
const exitWait = 20 * time.Second

// exit stops the server with code for run.ps1. The game saves on its way
// out; should anything hold the way out up, the process ends with the same
// code regardless, so the supervisor still starts the next build.
func (o *ops) exit(code int) {
	o.code.Store(int32(code))
	o.stop()
	go func() {
		time.Sleep(exitWait)
		os.Exit(code)
	}()
}

// reportUpdate tells the chat how the last update or rollback went, from the
// marker the previous run left.
func (o *ops) reportUpdate() {
	path := filepath.Join(o.dir, markerFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	os.Remove(path)
	var m marker
	if json.Unmarshal(data, &m) != nil {
		return
	}
	switch {
	case m.Restart:
		o.say(fmt.Sprintf("таверна перезапущена, v%s", game.Version))
	case m.To == "":
		o.say(fmt.Sprintf("откат выполнен: таверна снова на v%s", game.Version))
	case m.To == game.Version:
		o.say(fmt.Sprintf("таверна обновлена до v%s", game.Version))
	default:
		o.say(fmt.Sprintf("v%s не запустилась, вернулась v%s", m.To, game.Version))
	}
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
