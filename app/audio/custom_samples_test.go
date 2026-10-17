package audio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/framework/files"
)

func TestResolveCustomFilePathPreservesNestedAliasesAndFallbacks(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"Samples/Hit.wav",
		"Samples/Hit.ogg",
		"Samples/Hit.mp3",
		"Samples/Hit.ogg.wav",
		"Samples/Layer.wav",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("sample"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	fileMap, err := files.NewFileMap(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		want string
	}{
		{name: "samples/hit", want: "Samples/Hit.wav"},
		{name: "SAMPLES/HIT.MP3", want: "Samples/Hit.mp3"},
		{name: "samples/hit.ogg", want: "Samples/Hit.ogg.wav"},
		{name: "samples/layer.xyz", want: "Samples/Layer.wav"},
		{name: "samples/missing.wav", want: ""},
		{name: "../samples/hit.wav", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			name := filepath.ToSlash(filepath.Clean(test.name))
			got := resolveCustomFilePath(name, fileMap.GetFile)
			want := ""
			if test.want != "" {
				want = filepath.Join(root, filepath.FromSlash(test.want))
			}
			if got != want {
				t.Fatalf("resolveCustomFilePath(%q) = %q, want %q", test.name, got, want)
			}
		})
	}
}

func TestLoadBeatmapSamplesPreloadsExplicitNestedAudio(t *testing.T) {
	root := t.TempDir()
	rootSample := filepath.Join(root, "Normal-HitNormal.WAV")
	nestedSample := filepath.Join(root, "Custom", "Sample.WAV")
	for _, path := range []string{rootSample, nestedSample} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		// bass.NewSampleData recognizes 44-byte RIFF/WAVE placeholders as
		// silent samples without requiring an initialized BASS device.
		data := make([]byte, 44)
		copy(data, "RIFF")
		copy(data[8:], "WAVE")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fileMap, err := files.NewFileMap(root)
	if err != nil {
		t.Fatal(err)
	}

	LoadBeatmapSamples(map[string]string{"normal-hitnormal.wav": rootSample}, fileMap.GetFile)
	t.Cleanup(ClearBeatmapSamples)

	bank.mu.RLock()
	rootLoaded := bank.beatmap[0][0][1] != nil
	_, nestedIndexed := bank.customPaths["custom/sample.wav"]
	bank.mu.RUnlock()
	if !rootLoaded {
		t.Fatal("root hitnormal sample was not loaded")
	}
	if nestedIndexed {
		t.Fatal("nested file was eagerly indexed")
	}

	PreloadBeatmapFileSamples([]string{"CUSTOM/SAMPLE.WAV", "custom/sample.wav", "custom/missing.wav"})
	bank.mu.RLock()
	lookupFinalized := bank.resolveFile == nil
	missCached := len(bank.customMisses) == 1
	bank.mu.RUnlock()
	if !lookupFinalized {
		t.Fatal("nested resolver was left active during gameplay")
	}
	if !missCached {
		t.Fatal("missing authored sample was not cached")
	}

	loaded := resolveBeatmapFileSample("CUSTOM/SAMPLE.WAV")
	if loaded == nil || resolveBeatmapFileSample("custom/sample.wav") != loaded {
		t.Fatal("preloaded nested sample did not reuse its sample handle")
	}
	if err := os.Remove(nestedSample); err != nil {
		t.Fatal(err)
	}
	if resolveBeatmapFileSample("custom/sample.wav") != loaded {
		t.Fatal("playback tried to reload an already preloaded sample")
	}
}

func TestPreloadedSamplesDoNotResolveMissingFilesDuringGameplay(t *testing.T) {
	resolverCalls := 0
	LoadBeatmapSamples(nil, func(string) (string, error) {
		resolverCalls++
		return "", os.ErrNotExist
	})
	t.Cleanup(ClearBeatmapSamples)

	PreloadBeatmapFileSamples([]string{"sample/subfolder.wav", "SAMPLE/SUBFOLDER.WAV"})
	if resolverCalls == 0 {
		t.Fatal("preloading did not attempt to resolve the authored sample")
	}
	preloadCalls := resolverCalls
	if sample := resolveBeatmapFileSample("sample/subfolder.wav"); sample != nil {
		t.Fatal("missing sample unexpectedly loaded")
	}
	if sample := resolveBeatmapFileSample("other/missing.wav"); sample != nil {
		t.Fatal("unreferenced missing sample unexpectedly loaded")
	}
	if resolverCalls != preloadCalls {
		t.Fatalf("gameplay performed %d new filesystem lookups", resolverCalls-preloadCalls)
	}
}

func TestInvalidCustomSampleIsRejectedOnceAcrossAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.wav")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	LoadBeatmapSamples(map[string]string{"invalid.wav": path})
	t.Cleanup(ClearBeatmapSamples)

	PreloadBeatmapFileSamples([]string{"invalid", "invalid.wav"})
	bank.mu.RLock()
	invalidCount := len(bank.invalidPaths)
	missCount := len(bank.customMisses)
	bank.mu.RUnlock()
	if invalidCount != 1 || missCount != 2 {
		t.Fatalf("invalid custom file cache has %d paths and %d aliases, want 1 and 2", invalidCount, missCount)
	}
	if resolveBeatmapFileSample("invalid.wav") != nil {
		t.Fatal("invalid file unexpectedly reloaded during gameplay")
	}
}
