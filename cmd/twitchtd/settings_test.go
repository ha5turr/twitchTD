package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"twitchtd/internal/store"
)

func TestSettingsOverridesAndValidation(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{"towerLimit": 4, "walletCap": 800}`), 0o644)
	st, err := store.Open(filepath.Join(dir, "td.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	s, err := newSettings(cfgPath, st)
	if err != nil {
		t.Fatal(err)
	}
	if c := s.Current(); c.TowerLimit != 4 || c.WalletCap != 800 || c.StartGold == 0 {
		t.Fatalf("file values not applied over defaults: %+v", c.Config)
	}
	var applied appConfig
	s.apply = func(c appConfig, _ map[string]any) { applied = c }

	if err := s.Set(json.RawMessage(`{"towerLimit": 7, "volume": 0.3}`)); err != nil {
		t.Fatal(err)
	}
	if applied.TowerLimit != 7 || applied.Volume != 0.3 || applied.WalletCap != 800 {
		t.Fatalf("panel change not applied: limit %d volume %v cap %d", applied.TowerLimit, applied.Volume, applied.WalletCap)
	}
	if err := s.Set(json.RawMessage(`{"db": "other.db"}`)); err == nil {
		t.Fatal("non-editable settings must be refused")
	}
	if err := s.Set(json.RawMessage(`{"towerLimit": 0}`)); err == nil || s.Current().TowerLimit != 7 {
		t.Fatal("invalid values must be refused and leave the old ones")
	}

	// A restart keeps the panel's changes on top of the file.
	again, err := newSettings(cfgPath, st)
	if err != nil {
		t.Fatal(err)
	}
	if again.Current().TowerLimit != 7 {
		t.Fatalf("overrides lost on restart: %d", again.Current().TowerLimit)
	}
	if v := again.View(); v["towerLimit"] != float64(7) || v["webcamFrom"] != "N1" {
		t.Fatalf("view %v", v)
	}
}
