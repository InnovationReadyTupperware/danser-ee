package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsReloadKeepsLastGoodProfileAndWarnsOncePerFailure(t *testing.T) {
	previousConfig, previousPatch, previousListeners := currentConfig, JsonPatch, reloadListeners
	previousGlobals := &Config{
		General: General, Graphics: Graphics, Audio: Audio, Input: Input,
		Gameplay: Gameplay, Skin: Skin, Cursor: Cursor, Objects: Objects,
		Playfield: Playfield, CursorDance: CursorDance, Knockout: Knockout,
		Recording: Recording, Debug: Debug,
	}
	t.Cleanup(func() {
		currentConfig, JsonPatch, reloadListeners = previousConfig, previousPatch, previousListeners
		previousGlobals.attachToGlobals()
	})

	path := filepath.Join(t.TempDir(), "profile.json")
	initial := NewConfigFile()
	initial.Audio.GeneralVolume = 0.35
	if err := initial.SaveChecked(path, true); err != nil {
		t.Fatal(err)
	}
	currentConfig = initial
	initial.attachToGlobals()

	var warnings, updates int
	warned := false
	warn := func(error) { warnings++ }
	reloadListeners = []func(){func() { updates++ }}
	JsonPatch = `{"Audio":{"MusicVolume":0.25}}`

	if err := os.WriteFile(path, []byte(`{"Audio":`), 0o600); err != nil {
		t.Fatal(err)
	}
	badConfig, readErr := loadSettingsWithRetry(path)
	if readErr == nil {
		t.Fatal("corrupt profile unexpectedly loaded")
	}
	handleSettingsReload(badConfig, readErr, &warned, warn)
	handleSettingsReload(badConfig, readErr, &warned, warn)
	if currentConfig != initial || Audio.GeneralVolume != 0.35 || updates != 0 || warnings != 1 {
		t.Fatalf("failed reload changed live settings or duplicated warning: volume=%v updates=%d warnings=%d", Audio.GeneralVolume, updates, warnings)
	}
	if err := os.WriteFile(path, []byte(`{"Gameplay":{"HitCounter":null}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	badConfig, readErr = loadSettingsWithRetry(path)
	if readErr == nil {
		t.Fatal("invalid nested settings unexpectedly loaded")
	}
	if !strings.Contains(readErr.Error(), "Gameplay.HitCounter must not be null") {
		t.Fatalf("unexpected nested settings error: %v", readErr)
	}
	handleSettingsReload(badConfig, readErr, &warned, warn)
	if currentConfig != initial || warnings != 1 {
		t.Fatalf("invalid nested settings changed the active profile or repeated the warning: warnings=%d", warnings)
	}
	if err := os.WriteFile(path, []byte(`{"Gameplay":{"HitCounter":{"Color":[]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	badConfig, readErr = loadSettingsWithRetry(path)
	if readErr == nil || !strings.Contains(readErr.Error(), "Gameplay.HitCounter.Color must not be empty") {
		t.Fatalf("empty legacy color list should return a validation error, got %v", readErr)
	}
	handleSettingsReload(badConfig, readErr, &warned, warn)
	if currentConfig != initial || warnings != 1 {
		t.Fatalf("invalid legacy colors changed live settings or repeated the warning: warnings=%d", warnings)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	missingConfig, missingErr := loadSettingsWithRetry(path)
	if missingErr == nil {
		t.Fatal("missing settings file unexpectedly loaded")
	}
	handleSettingsReload(missingConfig, missingErr, &warned, warn)
	if currentConfig != initial || warnings != 1 {
		t.Fatalf("missing profile changed live settings or repeated the warning: warnings=%d", warnings)
	}

	updated := NewConfigFile()
	updated.Audio.GeneralVolume = 0.8
	if err := updated.SaveChecked(path, true); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadSettingsWithRetry(path)
	if err != nil {
		t.Fatal(err)
	}
	handleSettingsReload(loaded, nil, &warned, warn)
	if currentConfig == initial || Audio.GeneralVolume != 0.8 || Audio.MusicVolume != 0.25 || updates != 1 || warned {
		t.Fatalf("valid reload was not fully applied: volume=%v music=%v updates=%d warned=%v", Audio.GeneralVolume, Audio.MusicVolume, updates, warned)
	}
	saved, err := loadSettingsWithRetry(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Audio.MusicVolume != 0.5 {
		t.Fatalf("session-only patch leaked into saved profile: music=%v", saved.Audio.MusicVolume)
	}

	handleSettingsReload(nil, readErr, &warned, warn)
	if warnings != 2 || updates != 1 {
		t.Fatalf("new failure episode should warn exactly once: warnings=%d updates=%d", warnings, updates)
	}
}

func TestProfileSaveAtomicallyReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	config := NewConfigFile()
	if err := config.SaveChecked(path, true); err != nil {
		t.Fatal(err)
	}
	config.Audio.GeneralVolume = 0.7
	if err := config.SaveChecked("", false); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadSettingsWithRetry(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Audio.GeneralVolume != 0.7 {
		t.Fatalf("saved volume = %v, want 0.7", loaded.Audio.GeneralVolume)
	}
	if leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".profile.json-*.tmp")); err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary save files remain: %v (error %v)", leftovers, err)
	}
}
