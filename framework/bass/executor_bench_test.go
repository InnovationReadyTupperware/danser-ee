package bass

import "testing"

func BenchmarkRunOnAudioThread(b *testing.B) {
	startExecutor()
	b.Cleanup(stopExecutor)
	b.ReportAllocs()

	for b.Loop() {
		runOnAudioThread(func() {})
	}
}

func BenchmarkRunOnAudioThreadResult(b *testing.B) {
	startExecutor()
	b.Cleanup(stopExecutor)
	b.ReportAllocs()

	for b.Loop() {
		_ = runOnAudioThreadResult(func() int { return 1 })
	}
}
