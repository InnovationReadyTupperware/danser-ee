package bass

import "testing"

func BenchmarkPlaybackUpdateTransactions(b *testing.B) {
	startExecutor()
	b.Cleanup(stopExecutor)

	track := &TrackBass{speed: 1, pitch: 1, relativeFrequency: 1}

	b.Run("separate", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = track.GetState()
			_ = track.GetPosition()
			_ = track.GetSpeed()
			track.SetTempo(1)
			track.SetPitch(1)
			track.SetRelativeFrequency(1)
			_ = MixerPosition()
			_ = track.GetSpeed()
			track.Update()
			_ = track.GetBoost()
			_ = track.GetState()
			track.SetVolumeRelative(1)
		}
	})

	b.Run("batched", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = track.SnapshotPlayback()
			_ = track.ApplyPlaybackRate(1, 1, 1)
			_ = track.UpdateAnalysis()
			track.SetVolumeRelativeIfPlaying(1)
		}
	})
}
