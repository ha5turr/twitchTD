package main

import (
	"encoding/json"
	"os"

	"twitchtd/internal/donationalerts"
	"twitchtd/internal/game"
	"twitchtd/internal/streamerbot"
)

// appConfig is config.json: balance fields at the top level (as in
// game.Config), integrations in their own sections.
type appConfig struct {
	game.Config
	StreamerBot    streamerbot.Config    `json:"streamerbot"`
	DonationAlerts donationalerts.Config `json:"donationalerts"`
	DB             string                `json:"db"` // SQLite file; empty disables saving

	// Overlay sound. OBS can also mix the browser source like any other.
	Volume        float64            `json:"volume"`
	Muted         bool               `json:"muted"`
	Ambient       bool               `json:"ambient"`
	AmbientVolume float64            `json:"ambientVolume"`
	SoundGroups   map[string]float64 `json:"soundGroups"` // level 0..1 per game.SoundGroups key
}

func defaultSoundGroups() map[string]float64 {
	m := map[string]float64{}
	for _, g := range game.SoundGroups {
		m[g.Key] = 1
	}
	return m
}

func loadConfig(path string) (appConfig, error) {
	cfg := appConfig{
		Config:        game.DefaultConfig(),
		StreamerBot:   streamerbot.DefaultConfig(),
		DB:            "twitchtd.db",
		Volume:        0.2,
		Ambient:       false,
		AmbientVolume: 0.7,
		SoundGroups:   defaultSoundGroups(),
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	return cfg, json.Unmarshal(data, &cfg)
}
