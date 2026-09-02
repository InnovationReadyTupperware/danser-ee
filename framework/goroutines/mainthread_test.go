package goroutines

import (
	"slices"
	"testing"
)

func TestExecuteMainLoop_RunsHooksInOrder(t *testing.T) {
	previousQueue := callQueue
	previousCond := mainLoopCond
	previousFunc := mainLoopFunc
	previousStart := mainLoopStart
	previousFinish := mainLoopFinish
	defer func() {
		callQueue = previousQueue
		mainLoopCond = previousCond
		mainLoopFunc = previousFunc
		mainLoopStart = previousStart
		mainLoopFinish = previousFinish
	}()

	events := make([]string, 0, 4)
	iterations := 0
	callQueue = make(chan func())
	mainLoopStart = func() { events = append(events, "setup") }
	mainLoopCond = func() bool { return iterations < 2 }
	mainLoopFunc = func() {
		events = append(events, "loop")
		iterations++
	}
	mainLoopFinish = func() { events = append(events, "cleanup") }

	executeMainLoop()

	want := []string{"setup", "loop", "loop", "cleanup"}
	if !slices.Equal(events, want) {
		t.Fatalf("hook order = %v, want %v", events, want)
	}
}

func TestExecuteMainLoop_CleansUpAfterPanic(t *testing.T) {
	previousQueue := callQueue
	previousCond := mainLoopCond
	previousFunc := mainLoopFunc
	previousStart := mainLoopStart
	previousFinish := mainLoopFinish
	defer func() {
		callQueue = previousQueue
		mainLoopCond = previousCond
		mainLoopFunc = previousFunc
		mainLoopStart = previousStart
		mainLoopFinish = previousFinish
	}()

	cleanedUp := false
	callQueue = make(chan func())
	mainLoopCond = func() bool { return true }
	mainLoopFunc = func() { panic("render failure") }
	mainLoopFinish = func() { cleanedUp = true }

	defer func() {
		if recover() == nil {
			t.Fatal("main loop did not panic")
		}
		if !cleanedUp {
			t.Fatal("cleanup did not run after panic")
		}
	}()
	executeMainLoop()
}
