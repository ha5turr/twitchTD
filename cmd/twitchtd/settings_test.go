package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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

// Every field the panel's settings form shows must be editable here, or the
// panel's save is refused as a whole.
func TestPanelFieldsEditable(t *testing.T) {
	js, err := os.ReadFile(filepath.Join("..", "..", "web", "static", "js", "panel.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(js)
	start := strings.Index(src, "const SETTING_GROUPS = [")
	if start < 0 {
		t.Fatal("SETTING_GROUPS not found in panel.js")
	}
	end := strings.Index(src[start:], "\n];")
	if end < 0 {
		t.Fatal("SETTING_GROUPS is not closed in panel.js")
	}
	fields := regexp.MustCompile(`\['(\w+)', '`).FindAllStringSubmatch(src[start:start+end], -1)
	if len(fields) == 0 {
		t.Fatal("no fields in SETTING_GROUPS")
	}
	for _, f := range fields {
		if !slices.Contains(editable, f[1]) {
			t.Errorf("panel field %q is not editable", f[1])
		}
	}
}

func TestKingSettings(t *testing.T) {
	s, err := newSettings(filepath.Join(t.TempDir(), "none.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	s.apply = func(appConfig, map[string]any) {}
	if err := s.Set(json.RawMessage(`{"kingHp": 9, "kingMaskAt": 0.25, "kingIntroSec": 45}`)); err != nil {
		t.Fatal(err)
	}
	if c := s.Current(); c.KingHP != 9 || c.KingMaskAt != 0.25 || c.KingIntroSec != 45 {
		t.Fatalf("king settings %v %v %v", c.KingHP, c.KingMaskAt, c.KingIntroSec)
	}
	for _, bad := range []string{`{"kingHp": 0}`, `{"kingMaskAt": 1.5}`, `{"kingIntroSec": -1}`} {
		if s.Set(json.RawMessage(bad)) == nil {
			t.Errorf("%s must be refused", bad)
		}
	}
}

func TestSoundGroupSettings(t *testing.T) {
	s, err := newSettings(filepath.Join(t.TempDir(), "none.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	s.apply = func(appConfig, map[string]any) {}
	if err := s.Set(json.RawMessage(`{"soundGroups": {"combat": 0.3, "alerts": 1, "building": 1, "support": 1, "tavern": 1, "walls": 1}, "ambientVolume": 0.2}`)); err != nil {
		t.Fatal(err)
	}
	if c := s.Current(); c.SoundGroups["combat"] != 0.3 || c.AmbientVolume != 0.2 {
		t.Fatalf("sound settings %+v %v", c.SoundGroups, c.AmbientVolume)
	}
	if s.Set(json.RawMessage(`{"soundGroups": {"music": 1}}`)) == nil {
		t.Fatal("unknown group must be refused")
	}
	if s.Set(json.RawMessage(`{"soundGroups": {"combat": 2}}`)) == nil {
		t.Fatal("group level above 1 must be refused")
	}
}
