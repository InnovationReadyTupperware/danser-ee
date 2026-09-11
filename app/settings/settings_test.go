package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigIgnoresUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	data := []byte(`{"FutureTopLevelSetting":true,"Audio":{"FutureAudioSetting":123}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("os.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })

	config, err := LoadConfig(file)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if config.Audio == nil {
		t.Fatal("Audio settings are nil after loading unknown fields")
	}
}
