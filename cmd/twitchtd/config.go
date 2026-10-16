package main

import (
	"encoding/json"
	"os"

	"twitchtd/internal/game"
	"twitchtd/internal/streamerbot"
)

// appConfig is config.json: balance fields at the top level (as in
// game.Config), integrations in their own sections.
type appConfig struct {
	game.Config
	StreamerBot streamerbot.Config `json:"streamerbot"`
	DB          string             `json:"db"` // SQLite file; empty disables saving
}

func loadConfig(path string) (appConfig, error) {
	cfg := appConfig{
		Config:      game.DefaultConfig(),
		StreamerBot: streamerbot.DefaultConfig(),
		DB:          "twitchtd.db",
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
