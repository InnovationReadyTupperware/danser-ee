package bass

import (
	"runtime"
	"sync"
)

// audioExecutor serializes calls into BASS and owns the operating-system
// thread used for those calls. BASS has its own mixer thread, but keeping all
// application-side handle mutations on one thread prevents a gameplay update,
// storyboard worker, and offline mixer pull from racing over the same handle.
//
// The executor is deliberately synchronous at the API boundary. Audio
// submissions are tiny control operations, so waiting for their completion is
// preferable to exposing a second queue whose ordering callers would have to
// reason about. Offline rendering also relies on the same guarantee before it
// asks the mixer for the next output block.
type audioExecutor struct {
	commands chan func()
	stop     chan struct{}
	done     chan struct{}
	ready    chan struct{}

	lifecycleMu sync.Mutex
	stopping    bool
	submissions sync.WaitGroup
}

var (
	executorMu sync.RWMutex
	executor   *audioExecutor
)

func startExecutor() {
	executorMu.Lock()
	defer executorMu.Unlock()

	if executor != nil {
		return
	}

	e := &audioExecutor{
		commands: make(chan func()),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		ready:    make(chan struct{}),
	}

	executor = e

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(e.done)

		close(e.ready)

		for {
			select {
			case command := <-e.commands:
				command()
			case <-e.stop:
				return
			}
		}
	}()

	<-e.ready
}

func stopExecutor() {
	executorMu.RLock()
	e := executor
	executorMu.RUnlock()
	if e == nil {
		return
	}

	e.lifecycleMu.Lock()
	if e.stopping {
		e.lifecycleMu.Unlock()
		<-e.done
		return
	}
	e.stopping = true
	e.lifecycleMu.Unlock()

	// Every submission is synchronously waiting for its command to finish.
	// Drain those submissions before stopping the command loop; otherwise a
	// select could choose the stop signal while a queued command is still
	// waiting to close its completion channel.
	e.submissions.Wait()
	close(e.stop)
	<-e.done

	executorMu.Lock()
	if executor == e {
		executor = nil
	}
	executorMu.Unlock()
	return
}

func runOnAudioThread(fn func()) {
	if fn == nil {
		return
	}

	executorMu.RLock()
	e := executor
	executorMu.RUnlock()
	if e == nil {
		return
	}

	e.lifecycleMu.Lock()
	if e.stopping {
		e.lifecycleMu.Unlock()
		return
	}
	e.submissions.Add(1)
	e.lifecycleMu.Unlock()
	defer e.submissions.Done()

	done := make(chan struct{})
	e.commands <- func() {
		defer close(done)
		fn()
	}
	<-done
}

func runOnAudioThreadResult[T any](fn func() T) T {
	var result T
	runOnAudioThread(func() {
		result = fn()
	})

	return result
}
