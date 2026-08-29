package launcher

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wieku/danser-go/framework/env"
	"github.com/wieku/danser-go/framework/files"
	"github.com/wieku/danser-go/framework/math/mutils"
)

var launcherConfig = &launcherConf{
	Profile:          nil,
	CheckForUpdates:  true,
	ShowFileAfter:    true,
	PreviewSelected:  true,
	PreviewVolume:    0.25,
	SortMapsBy:       Title,
	SortAscending:    true,
	LoadLatestReplay: false,
	SkipMapUpdate:    false,
	AutoRefreshDB:    false,
	ShowJSONPaths:    false,
	CurrentMode:      CursorDance,
	CurrentPMode:     Watch,
}

type launcherConf struct {
	Profile          *string
	CheckForUpdates  bool
	ShowFileAfter    bool
	PreviewSelected  bool
	PreviewVolume    float64
	SortMapsBy       SortBy
	SortAscending    bool
	LoadLatestReplay bool
	SkipMapUpdate    bool
	AutoRefreshDB    bool
	ShowJSONPaths    bool
	LastKnockoutPath string
	CurrentMode      Mode
	CurrentPMode     PMode
}

func defaultLauncherConfig() launcherConf {
	return launcherConf{
		CheckForUpdates: true,
		ShowFileAfter:   true,
		PreviewSelected: true,
		PreviewVolume:   0.25,
		SortMapsBy:      Title,
		SortAscending:   true,
		CurrentMode:     CursorDance,
		CurrentPMode:    Watch,
	}
}

func loadLauncherConfig() {
	cPath := filepath.Join(env.ConfigDir(), "launcher.json")

	if file, err := os.Open(cPath); err == nil {
		data, err := io.ReadAll(files.NewUnicodeReader(file))
		closeErr := file.Close()
		if err != nil {
			log.Println("Launcher: Failed to read launcher configuration:", err)
		} else if closeErr != nil {
			log.Println("Launcher: Failed to close launcher configuration:", closeErr)
		} else if err = json.Unmarshal(data, launcherConfig); err != nil {
			log.Println("Launcher: Failed to parse launcher configuration:", err)
			*launcherConfig = defaultLauncherConfig()
		}
	} else if !os.IsNotExist(err) {
		log.Println("Launcher: Failed to open launcher configuration:", err)
	}

	if launcherConfig.Profile == nil {
		lastModified := time.UnixMilli(0)

		filepath.WalkDir(env.ConfigDir(), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				log.Println("Launcher: Failed to inspect config path:", err)
				return nil
			}
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
				return nil
			}

			stPath := profilePath(env.ConfigDir(), path)
			if stPath == "" || isReservedProfileName(stPath) {
				return nil
			}

			info, infoErr := entry.Info()
			if infoErr != nil {
				log.Println("Launcher: Failed to inspect config file:", infoErr)
				return nil
			}
			if info.ModTime().After(lastModified) {
				lastModified = info.ModTime()
				profile := stPath
				launcherConfig.Profile = &profile
			}

			return nil
		})
	}

	if launcherConfig.Profile == nil {
		def := "default"
		launcherConfig.Profile = &def
	}

	launcherConfig.CurrentMode = mutils.Clamp(launcherConfig.CurrentMode, CursorDance, SoloKnockout)
	launcherConfig.CurrentPMode = mutils.Clamp(launcherConfig.CurrentPMode, Watch, Screenshot)

	saveLauncherConfig()
}

func saveLauncherConfig() {
	if err := saveLauncherConfigChecked(); err != nil {
		log.Println("Launcher: Failed to save launcher configuration:", err)
	}
}

func saveLauncherConfigChecked() error {
	data, err := json.MarshalIndent(launcherConfig, "", "\t")
	if err != nil {
		return fmt.Errorf("encode launcher configuration: %w", err)
	}

	configDir := env.ConfigDir()
	if err = os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create launcher configuration directory: %w", err)
	}

	if err = os.WriteFile(filepath.Join(configDir, "launcher.json"), data, 0644); err != nil {
		return fmt.Errorf("write launcher configuration: %w", err)
	}

	return nil
}

func profilePath(configDir, path string) string {
	relative, err := filepath.Rel(configDir, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return ""
	}

	return strings.TrimSuffix(filepath.ToSlash(relative), filepath.Ext(relative))
}

func isReservedProfileName(name string) bool {
	switch strings.ToLower(name) {
	case "credentials", "default", "launcher":
		return true
	default:
		return false
	}
}

// profileFilePath resolves a user-selected profile name without allowing it
// to escape the configuration directory. The launcher permits subdirectories
// in profile names, but names from the editable UI must remain relative to the
// profile root on every supported filesystem.
func profileFilePath(name string) (string, error) {
	return profileFilePathIn(env.ConfigDir(), name)
}

func profileFilePathIn(configDir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("profile name is empty")
	}

	relative := filepath.Clean(filepath.FromSlash(name))
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) || filepath.VolumeName(relative) != "" {
		return "", fmt.Errorf("profile name must stay inside the configuration directory")
	}

	configDir, err := filepath.Abs(configDir)
	if err != nil {
		return "", fmt.Errorf("resolve configuration directory: %w", err)
	}

	path := filepath.Join(configDir, relative+".json")
	resolvedRelative, err := filepath.Rel(configDir, path)
	if err != nil || resolvedRelative == ".." || strings.HasPrefix(resolvedRelative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("profile name must stay inside the configuration directory")
	}

	return path, nil
}
