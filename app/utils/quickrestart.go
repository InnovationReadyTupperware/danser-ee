package utils

import (
	"os"
	"os/exec"

	"github.com/wieku/danser-go/framework/env"
	"github.com/wieku/danser-go/framework/goroutines"
	"github.com/wieku/danser-go/framework/platform/gcontext"
)

func QuickRestart() {
	danserPath := os.Args[0]

	arguments := make([]string, 0)

	noDbCheck := false
	quickStart := false

	for _, arg := range os.Args[1:] {
		if arg == "-nodbcheck" {
			noDbCheck = true
		}

		if arg == "-quickstart" {
			quickStart = true
		}

		arguments = append(arguments, arg)
	}

	if !noDbCheck {
		arguments = append(arguments, "-nodbcheck")
	}

	if !quickStart {
		arguments = append(arguments, "-quickstart")
	}

	cmd := exec.Command(danserPath, arguments...)
	// The restarted process is detached from the launcher's child-process
	// monitor when this app exits, so it must own its own fatal dialog rather
	// than inheriting the stale marker from the original launcher child.
	cmd.Env = env.LauncherChildEnvironment(false)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Start()

	goroutines.CallNonBlockMain(func() {
		gcontext.SetShouldClose(true)
	})
}
