package bass

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestRunOnAudioThreadReusesCommandsConcurrently(t *testing.T) {
	startExecutor()
	t.Cleanup(stopExecutor)

	const (
		workers = 16
		calls   = 100
	)

	var executed atomic.Int64
	var workersDone sync.WaitGroup
	for range workers {
		workersDone.Go(func() {
			for range calls {
				runOnAudioThread(func() {
					executed.Add(1)
				})
			}
		})
	}
	workersDone.Wait()

	if got, want := executed.Load(), int64(workers*calls); got != want {
		t.Fatalf("executed commands = %d, want %d", got, want)
	}
}

func TestRunOnAudioThreadResult(t *testing.T) {
	startExecutor()
	t.Cleanup(stopExecutor)

	if got, want := runOnAudioThreadResult(func() int { return 42 }), 42; got != want {
		t.Fatalf("result = %d, want %d", got, want)
	}
}
