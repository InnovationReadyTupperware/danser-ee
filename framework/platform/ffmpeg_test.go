package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// A danser-bundled executable resolves its libraries from the lib
// directory next to its bin directory.
func TestBundledLibDir(t *testing.T) {
	root := t.TempDir()
	tool := filepath.Join(root, "ffmpeg", "bin", "ffmpeg")
	writeEmpty(t, tool)
	lib := filepath.Join(root, "ffmpeg", "lib")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatalf("create lib directory: %v", err)
	}

	if got := bundledLibDir(tool); got != lib {
		t.Errorf("bundledLibDir = %q, want %q", got, lib)
	}
}

// System executables and incomplete layouts resolve their own libraries.
func TestBundledLibDirMisses(t *testing.T) {
	root := t.TempDir()
	binOnly := filepath.Join(root, "ffmpeg", "bin", "ffmpeg")
	writeEmpty(t, binOnly)
	loose := filepath.Join(root, "tools", "ffmpeg")
	writeEmpty(t, loose)

	for _, execPath := range []string{
		filepath.Join(string(filepath.Separator), "usr", "bin", "ffmpeg"),
		binOnly,
		loose,
	} {
		if got := bundledLibDir(execPath); got != "" {
			t.Errorf("bundledLibDir(%q) = %q, want empty", execPath, got)
		}
	}
}

// The child environment carries the bundled library directory first,
// keeping any inherited search path behind it. exec deduplicates to
// the last entry, so the assertion targets the tail.
func TestWithBundledLibPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("library search path is a non-Windows mechanism")
	}

	root := t.TempDir()
	tool := filepath.Join(root, "ffmpeg", "bin", "ffmpeg")
	writeEmpty(t, tool)
	if err := os.MkdirAll(filepath.Join(root, "ffmpeg", "lib"), 0o755); err != nil {
		t.Fatalf("create lib directory: %v", err)
	}
	t.Setenv("LD_LIBRARY_PATH", filepath.Join(root, "system-libs"))

	cmd := &exec.Cmd{}
	withBundledLibPath(cmd, tool)

	want := "LD_LIBRARY_PATH=" + filepath.Join(root, "ffmpeg", "lib") +
		string(os.PathListSeparator) + filepath.Join(root, "system-libs")
	if len(cmd.Env) == 0 || cmd.Env[len(cmd.Env)-1] != want {
		t.Errorf("child environment tail = %q, want %q", tail(cmd.Env), want)
	}
}

// System executables keep the inherited environment untouched.
func TestWithBundledLibPathLeavesSystemAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("library search path is a non-Windows mechanism")
	}

	cmd := &exec.Cmd{}
	withBundledLibPath(cmd, filepath.Join(string(filepath.Separator), "usr", "bin", "ffmpeg"))
	if cmd.Env != nil {
		t.Errorf("child environment = %q, want untouched", cmd.Env)
	}
}

func tail(env []string) string {
	if len(env) == 0 {
		return ""
	}
	return env[len(env)-1]
}

func writeEmpty(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}
