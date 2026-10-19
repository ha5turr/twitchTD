// Package store keeps chatters' wallets and the current game in SQLite, so a
// game paused at the end of a stream break continues in the next one.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"twitchtd/internal/game"
)

type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS chatters (
	login   TEXT PRIMARY KEY,
	name    TEXT NOT NULL,
	gold    INTEGER NOT NULL,
	kills   INTEGER NOT NULL,
	updated INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS kv (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Load returns the saved game; ok is false when nothing was saved yet.
func (s *Store) Load() (save game.Save, ok bool, err error) {
	var raw string
	err = s.db.QueryRow(`SELECT value FROM kv WHERE key = 'game'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	} else if err == nil {
		err = json.Unmarshal([]byte(raw), &save)
		ok = err == nil
	}
	if err != nil {
		return save, false, err
	}
	rows, err := s.db.Query(`SELECT login, name, gold, kills, drinks, class, progress FROM chatters`)
	if err != nil {
		return save, ok, err
	}
	defer rows.Close()
	for rows.Next() {
		var c game.SavedChatter
		var progress string
		if err := rows.Scan(&c.Login, &c.Name, &c.Gold, &c.Kills, &c.Drinks, &c.Class, &progress); err != nil {
			return save, ok, err
		}
		if err := json.Unmarshal([]byte(progress), &c.Progress); err != nil {
			return save, ok, err
		}
		save.Chatters = append(save.Chatters, c)
	}
	return save, ok, rows.Err()
}

// Save writes the whole game in one transaction.
func (s *Store) Save(save game.Save) error {
	chatters := save.Chatters
	save.Chatters = nil
	raw, err := json.Marshal(save)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO kv(key, value) VALUES('game', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, string(raw)); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO chatters(login, name, gold, kills, drinks, class, progress, updated) VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(login) DO UPDATE SET name = excluded.name, gold = excluded.gold, kills = excluded.kills,
			drinks = excluded.drinks, class = excluded.class, progress = excluded.progress, updated = excluded.updated`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now().Unix()
	for _, c := range chatters {
		progress, err := json.Marshal(c.Progress)
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(c.Login, c.Name, c.Gold, c.Kills, c.Drinks, c.Class, string(progress), now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// migrate creates the tables and adds columns that later versions introduced.
func migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	for _, col := range []string{
		`ALTER TABLE chatters ADD COLUMN drinks INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE chatters ADD COLUMN class TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE chatters ADD COLUMN progress TEXT NOT NULL DEFAULT '{}'`, // souls, achievements
	} {
		if _, err := db.Exec(col); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}
	return nil
}

// GetKV reads a value saved with SetKV.
func (s *Store) GetKV(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM kv WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

// SetKV saves a small value, such as an OAuth token.
func (s *Store) SetKV(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO kv(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
