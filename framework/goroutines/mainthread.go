package goroutines

import (
	"errors"
	"runtime"

	"github.com/innovationreadytupperware/danser-ee/framework/profiler"
)

// CallQueueCap is the capacity of the call queue. This means how many calls to CallNonBlock will not
// block until some call finishes.
var CallQueueCap = 100000

var (
	callQueue        chan func()
	mainLoopAdded    chan bool
	mainLoopCond     func() bool
	mainLoopFunc     func()
	mainLoopStart    func()
	mainLoopFinish   func()
	mainLoopFinished chan bool
)

func init() {
	runtime.LockOSThread()
}

func checkRun() {
	if callQueue == nil {
		panic(errors.New("did not call RunMain"))
	}
}

// RunMain enables processing tasks on main thread. To use it, put your main function
// code into the run function (the argument to RunMain) and simply call RunMain from the real main function.
//
// RunMain returns when run (argument) function finishes and runCond argument for RunMainLoop returns false.
func RunMain(run func()) {
	callQueue = make(chan func(), CallQueueCap)

	mainLoopAdded = make(chan bool)
	mainLoopFinished = make(chan bool)

	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		run()
		done <- struct{}{}
	}()

	for {
		select {
		case f := <-callQueue:
			f()
		case <-mainLoopAdded:
			goto mainLoop
		case <-done:
			return
		}
	}

mainLoop:
	executeMainLoop()

	mainLoopFinished <- true
}

func executeMainLoop() {
	if mainLoopStart != nil {
		mainLoopStart()
	}
	if mainLoopFinish != nil {
		defer mainLoopFinish()
	}

	for mainLoopCond() {
		profiler.Reset()

		profiler.StartGroup("goroutines.RunMain", profiler.PRoot)

		profiler.StartGroup("goroutines.RunMain", profiler.PSched)

		for sRun := len(callQueue) > 0; sRun; {
			select {
			case f := <-callQueue:
				f()
			default:
				sRun = false
			}
		}

		profiler.EndGroup()

		mainLoopFunc()

		profiler.EndGroup()
	}
}

// RunMainLoop wires runFunc to the main thread and runs it in a loop as long as runCond returns true
//
// RunMainLoop returns when runCond returns false
func RunMainLoop(runCond func() bool, runFunc func()) {
	RunMainLoopWithHooks(runCond, nil, runFunc, nil)
}

// RunMainLoopWithHooks is RunMainLoop with setup and cleanup hooks that execute
// on the main OS thread. It is intended for native thread-scoped resources;
// cleanup runs after the loop terminates and before this function returns.
func RunMainLoopWithHooks(runCond func() bool, setup func(), runFunc func(), cleanup func()) {
	checkRun()

	mainLoopCond = runCond
	mainLoopFunc = runFunc
	mainLoopStart = setup
	mainLoopFinish = cleanup
	mainLoopAdded <- true

	<-mainLoopFinished
}

// CallNonBlockMain queues function f on the main thread and returns immediately. Does not wait until f
// finishes.
func CallNonBlockMain(f func()) {
	checkRun()
	callQueue <- f
}

// CallMain queues function f on the main thread and blocks until the function f finishes.
func CallMain(f func()) {
	checkRun()

	done := make(chan uint8)

	callQueue <- func() {
		f()
		done <- 1
	}

	<-done
}
