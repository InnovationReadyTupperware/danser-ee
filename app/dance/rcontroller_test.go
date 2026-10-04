package dance

import (
	"bytes"
	"encoding/binary"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/itchio/lzma"
	"github.com/wieku/rplpa"
)

func TestReplayCandidatesValidateStructuredModsAndProvenance(t *testing.T) {
	oldPaths := settings.KNOCKOUTREPLAYS
	t.Cleanup(func() { settings.KNOCKOUTREPLAYS = oldPaths })
	for _, test := range []struct {
		name    string
		version int32
		legacy  difficulty.Modifier
		mods    []*rplpa.ModInfo
		want    bool
	}{
		{name: "lazer perfect", version: difficulty.LazerReplayVersion, legacy: difficulty.Perfect | difficulty.SuddenDeath, want: true},
		{name: "stable perfect", version: 20250101, legacy: difficulty.Perfect | difficulty.SuddenDeath, want: true},
		{name: "stable scorev2", version: 20250101, legacy: difficulty.ScoreV2, want: true},
		{name: "stable relax nofail", version: 20250101, legacy: difficulty.Relax | difficulty.NoFail},
		{name: "lazer structured sd", version: difficulty.LazerReplayVersion, legacy: difficulty.NoFail | difficulty.SuddenDeath, mods: []*rplpa.ModInfo{{Acronym: "SD", Settings: map[string]any{"fail_on_slider_tail": true}}}, want: true},
		{name: "lazer structured conflicts", version: difficulty.LazerReplayVersion, mods: []*rplpa.ModInfo{{Acronym: "HD"}, {Acronym: "TC"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			replay := &rplpa.Replay{OsuVersion: test.version, Mods: uint32(test.legacy), BeatmapMD5: "test-map", Username: test.name,
				ReplayData: []*rplpa.ReplayData{{Time: 1, KeyPressed: new(rplpa.KeyPressed{})}, {Time: 10, KeyPressed: new(rplpa.KeyPressed{})}}}
			data, err := rplpa.WriteReplay(replay)
			if err != nil {
				t.Fatal(err)
			}
			if test.mods != nil {
				scoreJSON, err := json.Marshal(rplpa.ScoreInfo{Mods: test.mods})
				if err != nil {
					t.Fatal(err)
				}
				var compressed bytes.Buffer
				writer := lzma.NewWriter(&compressed)
				if _, err := writer.Write(scoreJSON); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				data = binary.LittleEndian.AppendUint32(data, uint32(compressed.Len()))
				data = append(data, compressed.Bytes()...)
			}
			path := filepath.Join(t.TempDir(), "replay.osr")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			settings.KNOCKOUTREPLAYS = []string{path}
			controller := &ReplayController{bMap: &beatmap.BeatMap{MD5: "test-map"}}
			candidates := controller.getCandidates()
			if got := len(candidates) == 1; got != test.want {
				t.Fatalf("replay accepted = %t, want %t", got, test.want)
			}
		})
	}
}
