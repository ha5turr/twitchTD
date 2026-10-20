package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"twitchtd/internal/game"
	"twitchtd/internal/store"
)

// editable lists the settings the streamer panel may change at runtime. The
// rest (integrations, database) still need config.json and a restart.
var editable = []string{
	"towerLimit", "streamerPlays", "wallRepairCdSec", "walletCap", "keepGold", "startGold", "chatGold", "bitsGold", "subGold",
	"donationGoldPerRub", "buildPhaseSec", "bossEvery", "sdfChance", "sdfFromWave", "baseHP", "hpGrowth", "chatterHPCoef",
	"webcamFrom", "webcamTo", "volume", "muted", "ambient", "ambientVolume", "soundGroups",
}

// settings combines config.json with the panel's overrides (kept in the
// database), reloads when the file changes, and applies the result live.
type settings struct {
	mu        sync.Mutex
	path      string
	st        *store.Store
	overrides map[string]json.RawMessage
	modTime   time.Time
	current   appConfig
	apply     func(appConfig, map[string]any)
}

func newSettings(path string, st *store.Store) (*settings, error) {
	s := &settings{path: path, st: st, overrides: map[string]json.RawMessage{}}
	if st != nil {
		if raw, ok, err := st.GetKV("settings"); err != nil {
			return nil, err
		} else if ok {
			if err := json.Unmarshal([]byte(raw), &s.overrides); err != nil {
				log.Printf("settings: ignoring broken overrides: %v", err)
			}
		}
	}
	if fi, err := os.Stat(path); err == nil {
		s.modTime = fi.ModTime()
	}
	cfg, err := s.build()
	if err != nil {
		return nil, err
	}
	s.current = cfg
	return s, nil
}

// build reads the file and lays the overrides on top.
func (s *settings) build() (appConfig, error) {
	cfg, err := loadConfig(s.path)
	if err != nil {
		return cfg, err
	}
	if len(s.overrides) > 0 {
		data, _ := json.Marshal(s.overrides)
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("overrides: %w", err)
		}
	}
	return cfg, nil
}

func (s *settings) Current() appConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

// View returns the editable values, as the panel shows them.
func (s *settings) View() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return view(s.current)
}

func view(cfg appConfig) map[string]any {
	var all map[string]any
	data, _ := json.Marshal(cfg)
	_ = json.Unmarshal(data, &all)
	out := map[string]any{}
	for _, k := range editable {
		out[k] = all[k]
	}
	return out
}

// Set validates and applies changes from the panel and remembers them.
func (s *settings) Set(raw json.RawMessage) error {
	var changes map[string]json.RawMessage
	if err := json.Unmarshal(raw, &changes); err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, k := range editable {
		allowed[k] = true
	}
	s.mu.Lock()
	next := map[string]json.RawMessage{}
	for k, v := range s.overrides {
		next[k] = v
	}
	for k, v := range changes {
		if !allowed[k] {
			s.mu.Unlock()
			return fmt.Errorf("setting %q cannot be changed from the panel", k)
		}
		next[k] = v
	}
	prev := s.overrides
	s.overrides = next
	cfg, err := s.build()
	if err == nil {
		err = validate(cfg)
	}
	if err != nil {
		s.overrides = prev
		s.mu.Unlock()
		return err
	}
	s.current = cfg
	s.mu.Unlock()
	if s.st != nil {
		data, _ := json.Marshal(next)
		if err := s.st.SetKV("settings", string(data)); err != nil {
			log.Printf("settings: saving: %v", err)
		}
	}
	s.apply(cfg, view(cfg))
	return nil
}

func validate(cfg appConfig) error {
	switch {
	case cfg.TowerLimit < 1, cfg.WalletCap < 1, cfg.BuildPhaseSec < 5, cfg.BossEvery < 1:
		return fmt.Errorf("лимит построек, потолок кошелька, фаза стройки (от 5 с) и частота боссов должны быть положительными")
	case cfg.WallRepairCDSec < 0:
		return fmt.Errorf("кд починки баррикады не может быть отрицательным")
	case cfg.HPGrowth < 1 || cfg.HPGrowth > 2, cfg.BaseHP <= 0:
		return fmt.Errorf("рост ХП должен быть от 1 до 2, базовое ХП больше нуля")
	case cfg.Volume < 0 || cfg.Volume > 1, cfg.AmbientVolume < 0 || cfg.AmbientVolume > 1:
		return fmt.Errorf("громкость от 0 до 1")
	case cfg.SdfChance < 0 || cfg.SdfChance > 1:
		return fmt.Errorf("шанс ЭссДиЭфа от 0 до 1")
	case cfg.SdfFromWave < 1:
		return fmt.Errorf("волна первого ЭссДиЭфа должна быть от 1")
	}
	known := map[string]bool{}
	for _, g := range game.SoundGroups {
		known[g.Key] = true
	}
	for k, v := range cfg.SoundGroups {
		if !known[k] {
			return fmt.Errorf("нет такой группы звуков: %s", k)
		}
		if v < 0 || v > 1 {
			return fmt.Errorf("громкость группы %s от 0 до 1", k)
		}
	}
	return nil
}

// chatAdmin lets mod commands in chat change settings like the panel does.
type chatAdmin struct{ s *settings }

func (a chatAdmin) Values() map[string]any { return a.s.View() }

func (a chatAdmin) Change(changes map[string]any) error {
	raw, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	return a.s.Set(raw)
}

// watch reloads config.json when it changes on disk.
func (s *settings) watch(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		fi, err := os.Stat(s.path)
		if err != nil || !fi.ModTime().After(s.modTime) {
			continue
		}
		s.modTime = fi.ModTime()
		s.mu.Lock()
		cfg, err := s.build()
		if err == nil {
			err = validate(cfg)
		}
		if err != nil {
			s.mu.Unlock()
			log.Printf("config.json not applied: %v", err)
			continue
		}
		s.current = cfg
		s.mu.Unlock()
		log.Printf("config.json reloaded (integrations and the database still need a restart)")
		s.apply(cfg, view(cfg))
	}
}
