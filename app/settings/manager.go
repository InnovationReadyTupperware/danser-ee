package settings

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/innovationreadytupperware/danser-ee/framework/env"
	"github.com/innovationreadytupperware/danser-ee/framework/files"
	"github.com/innovationreadytupperware/danser-ee/framework/goroutines"
	"github.com/innovationreadytupperware/danser-ee/framework/platform"
	"github.com/innovationreadytupperware/danser-ee/framework/platform/gcontext"
)

var currentConfig *Config

var filePath string
var watcher *fsnotify.Watcher

var reloadListeners []func()

const (
	settingsReloadDelay   = 200 * time.Millisecond
	settingsReloadRetries = 4
	settingsRetryDelay    = 100 * time.Millisecond
)

func initSettings() {
	if err := os.MkdirAll(env.ConfigDir(), 0755); err != nil {
		panic(err)
	}

	migrateSettings()
}

func LoadSettings(version string) bool {
	initSettings()

	fileName := "default"
	if version != "" {
		fileName = version
	}

	fileName += ".json"

	filePath = filepath.Join(env.ConfigDir(), fileName)

	file, err := os.Open(filePath)

	newFile := false

	if os.IsNotExist(err) {
		currentConfig = NewConfigFile()
		currentConfig.Save(filePath, true)

		newFile = true
	} else if err != nil {
		panic(err)
	} else {
		currentConfig, err = LoadConfig(file)
		if err != nil {
			panic(err)
		}

		file.Close()

		currentConfig.Save("", false) // this is done to save additions from the current format
	}

	LoadCredentials()

	currentConfig.attachToGlobals()

	if !RECORD {
		if err := setupWatcher(filePath); err != nil {
			log.Printf("SettingsManager: Warning: live reload unavailable for %q: %v", filePath, err)
			goroutines.CallNonBlockMain(func() {
				platform.ShowMessageBox(gcontext.SDLWindow(), platform.MessageBoxWarning,
					fmt.Sprintf("Live settings reload is unavailable for %s:\n\n%v\n\nRestart danser to apply settings changes.", filePath, err), false)
			})
		}
	}

	return newFile
}

func LoadPatch() {
	if err := applySettingsPatch(currentConfig, JsonPatch); err != nil {
		panic(err)
	}
}

func applySettingsPatch(config *Config, patch string) error {
	stripped := strings.TrimSpace(patch)

	if stripped != "" {
		if err := json.Unmarshal([]byte(stripped), config); err != nil {
			return fmt.Errorf("SettingsManager: Failed to parse the patch! Please re-check the sPatch argument for mistakes and ensure that quotation marks are escaped: %w", err)
		}
	}
	return nil
}

func setupWatcher(file string) error {
	abs, err := filepath.Abs(file)
	if err != nil {
		return fmt.Errorf("resolve settings path: %w", err)
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create settings watcher: %w", err)
	}
	// Watch the directory so atomic file replacements still trigger reloads.
	if err := w.Add(filepath.Dir(abs)); err != nil {
		_ = w.Close()
		return fmt.Errorf("watch settings directory: %w", err)
	}
	watcher = w
	goroutines.Run(func() { watchSettingsFile(w, abs) })
	return nil
}

func watchSettingsFile(w *fsnotify.Watcher, path string) {
	var timer *time.Timer
	var pending <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	warningShown := false
	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return
			}
			if !strings.EqualFold(filepath.Clean(event.Name), path) ||
				event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}
			if timer == nil {
				timer = time.NewTimer(settingsReloadDelay)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(settingsReloadDelay)
			}
			pending = timer.C
		case <-pending:
			pending = nil
			config, err := loadSettingsWithRetry(path)
			goroutines.CallNonBlockMain(func() {
				if watcher != w {
					return
				}
				handleSettingsReload(config, err, &warningShown, func(err error) {
					platform.ShowMessageBox(gcontext.SDLWindow(), platform.MessageBoxWarning,
						fmt.Sprintf("Could not reload settings from %s:\n\n%v\n\nThe previous settings remain active. Fix the file to retry.", path, err), false)
				})
			})
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			log.Printf("SettingsManager: Warning: watcher for %q: %v", path, err)
		}
	}
}

func loadSettingsWithRetry(path string) (*Config, error) {
	var lastErr error
	for attempt := range settingsReloadRetries {
		config, err := readSettingsCandidate(path)
		if err == nil {
			return config, nil
		}
		lastErr = err
		if attempt < settingsReloadRetries-1 {
			time.Sleep(settingsRetryDelay)
		}
	}
	return nil, fmt.Errorf("read settings %q: %w", path, lastErr)
}

func readSettingsCandidate(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	// This is a read-only file; a close error cannot lose buffered writes.
	//noinspection GoUnhandledErrorResult
	defer file.Close()
	return LoadConfig(file)
}

// applySettingsReload runs on the gameplay thread so configuration and listeners
// observe one complete update after parsing, patching, and saving succeed.
func applySettingsReload(config *Config, readErr error) error {
	if readErr != nil {
		return readErr
	}
	if err := config.SaveChecked("", false); err != nil {
		return fmt.Errorf("save reloaded settings: %w", err)
	}
	// CLI patches affect the running session, not the saved profile.
	if err := applySettingsPatch(config, JsonPatch); err != nil {
		return err
	}
	currentConfig = config
	config.attachToGlobals()
	for _, f := range reloadListeners {
		f()
	}
	return nil
}

func handleSettingsReload(config *Config, readErr error, warningShown *bool, warn func(error)) {
	err := applySettingsReload(config, readErr)
	if err == nil {
		*warningShown = false
		return
	}
	log.Printf("SettingsManager: Warning: could not reload settings; keeping previous settings: %v", err)
	if !*warningShown {
		*warningShown = true
		warn(err)
	}
}

func CloseWatcher() {
	if watcher != nil {
		err := watcher.Close()
		if err != nil {
			log.Println(err)
		}

		watcher = nil
	}
}

func AddReloadListener(f func()) {
	reloadListeners = append(reloadListeners, f)
}

func GetCompressedString() string {
	return currentConfig.GetCompressedString()
}

func Save() {
	currentConfig.Save(filePath, false)
}

func GetFormat() *Config {
	return currentConfig
}

func CreateDefault() {
	initSettings()

	currentConfig = NewConfigFile()
	currentConfig.attachToGlobals()
}

func migrateSettings() {
	currentSettings, _ := files.SearchFiles(env.DataDir(), "settings*.json", 0)

	for _, osPathname := range currentSettings {
		destName := filepath.Base(osPathname)

		if destName == "settings.json" {
			destName = "default.json"
		} else {
			destName = strings.TrimPrefix(destName, "settings-")
		}

		err := files.MoveFile(osPathname, filepath.Join(env.ConfigDir(), destName))
		if err != nil {
			panic(err)
		}
	}
}
