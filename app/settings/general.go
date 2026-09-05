package settings

import (
	"path/filepath"

	"github.com/innovationreadytupperware/danser-ee/framework/env"
)

var General = initGeneral()

func initGeneral() *general {
	osuBaseDir := getOsuInstallation()

	return &general{
		OsuSongsDir:       filepath.Join(osuBaseDir, "Songs"),
		OsuSkinsDir:       filepath.Join(osuBaseDir, "Skins"),
		OsuReplaysDir:     filepath.Join(osuBaseDir, "Replays"),
		DiscordPresenceOn: false,
		UnpackOszFiles:    true,
		VerboseImportLogs: false,
	}
}

type general struct {
	// Directory that contains osu! songs
	OsuSongsDir string `long:"true" label:"osu! Songs directory" path:"Select osu! Songs directory"`

	// Directory that contains osu! skins
	OsuSkinsDir string `long:"true" label:"osu! Skins directory" path:"Select osu! Skins directory"`

	// Directory that contains osu! replays
	OsuReplaysDir string `long:"true" label:"osu! Replays directory" path:"Select osu! Replays directory" tooltip:"Don't use replays directory inside danser's directory!"`

	// Whether discord should show that danser is on
	DiscordPresenceOn bool `label:"Discord Rich Presence"`

	// Whether danser should unpack .osz files in Songs folder, osu! may complain about it
	UnpackOszFiles bool

	// Whether import details should be shown. If false, only failures will be logged.
	VerboseImportLogs bool
}

func (g *general) GetSongsDir() string {
	if filepath.IsAbs(g.OsuSongsDir) {
		return g.OsuSongsDir
	}

	return filepath.Join(env.DataDir(), g.OsuSongsDir)
}

func (g *general) GetSkinsDir() string {
	if filepath.IsAbs(g.OsuSkinsDir) {
		return g.OsuSkinsDir
	}

	return filepath.Join(env.DataDir(), g.OsuSkinsDir)
}

func (g *general) GetReplaysDir() string {
	if filepath.IsAbs(g.OsuReplaysDir) {
		return g.OsuReplaysDir
	}

	return filepath.Join(env.DataDir(), g.OsuReplaysDir)
}
