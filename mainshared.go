package main

import "C"

import (
	"os"

	"github.com/innovationreadytupperware/danser-ee/app"
	"github.com/innovationreadytupperware/danser-ee/framework/env"
	"github.com/innovationreadytupperware/danser-ee/launcher"
)

//export danserMain
func danserMain(isLauncher bool, args []string) {
	os.Args = args

	env.Init("danser")
	if isLauncher {
		launcher.StartLauncher()
	} else {
		app.Run()
	}
}
