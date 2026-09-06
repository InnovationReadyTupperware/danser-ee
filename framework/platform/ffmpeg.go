package platform

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/innovationreadytupperware/danser-ee/framework/files"
)

const errMsg = "ffmpeg not found! Please make sure it's installed in danser directory or in PATH. Follow download instructions at https://github.com/Wieku/danser-go/wiki/FFmpeg"

var ffmpegInit bool
var ffPath string

func PrepareFFMpeg(cmdName string, args ...string) (*exec.Cmd, error) {
	if !ffmpegInit {
		ffmpegInit = true

		ffmpegExec, err := files.GetCommandExec("ffmpeg", "ffmpeg")
		if err != nil {
			return nil, fmt.Errorf(errMsg)
		}

		ffPath = filepath.Dir(ffmpegExec)
		log.Println("FFmpeg exec location:", ffmpegExec)
	} else if ffPath == "" {
		return nil, fmt.Errorf(errMsg)
	}

	execPath := filepath.Join(ffPath, cmdName)

	if runtime.GOOS != "windows" {
		if stat, err := os.Stat(execPath); err == nil {
			os.Chmod(execPath, (stat.Mode()&os.ModePerm)|0111) // Just try
		}
	}

	cmd := exec.Command(execPath, args...)
	cmd.Dir = ffPath

	withBundledLibPath(cmd, execPath)

	return cmd, nil
}

// withBundledLibPath points a bundled ffmpeg child at its libraries.
// System executables resolve their own and are left alone.
func withBundledLibPath(cmd *exec.Cmd, execPath string) {
	if runtime.GOOS == "windows" {
		return
	}
	libDir := bundledLibDir(execPath)
	if libDir == "" {
		return
	}
	if parent := os.Getenv("LD_LIBRARY_PATH"); parent != "" {
		libDir += string(os.PathListSeparator) + parent
	}
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+libDir)
}

// bundledLibDir returns the library directory next to a danser-bundled
// ffmpeg executable, or "" for system executables and missing directories.
func bundledLibDir(execPath string) string {
	binDir := filepath.Dir(execPath)
	if filepath.Base(binDir) != "bin" || filepath.Base(filepath.Dir(binDir)) != "ffmpeg" {
		return ""
	}
	libDir := filepath.Join(filepath.Dir(binDir), "lib")
	if info, err := os.Stat(libDir); err != nil || !info.IsDir() {
		return ""
	}
	return libDir
}
