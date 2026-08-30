package ffmpeg

import "testing"

func TestRecordingTimelineCountsAndEndpoints(t *testing.T) {
	tests := []struct {
		name       string
		fps        int
		oversample int
		blend      int
		durationMS float64
		wantFrames int64
	}{
		{name: "24 fps exact second", fps: 24, oversample: 1, blend: 1, durationMS: 1000, wantFrames: 24},
		{name: "30 fps exact second", fps: 30, oversample: 1, blend: 1, durationMS: 1000, wantFrames: 30},
		{name: "60 fps partial frame", fps: 60, oversample: 1, blend: 1, durationMS: 1001, wantFrames: 61},
		{name: "144 fps", fps: 144, oversample: 1, blend: 1, durationMS: 1000, wantFrames: 144},
		{name: "centered blur", fps: 60, oversample: 16, blend: 24, durationMS: 1000, wantFrames: 60},
		{name: "one-frame blur", fps: 60, oversample: 16, blend: 1, durationMS: 1000, wantFrames: 60},
		{name: "zero duration", fps: 60, oversample: 1, blend: 1, durationMS: 0, wantFrames: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &RecordingSessionConfig{
				outputFPS: tt.fps, sourceFPS: tt.fps * tt.oversample,
				oversample: tt.oversample, blendFrames: tt.blend,
				audioSampleRate: 48_000,
			}
			timeline, err := NewRecordingTimeline(tt.durationMS, config)
			if err != nil {
				t.Fatal(err)
			}
			if timeline.OutputFrames() != tt.wantFrames {
				t.Fatalf("OutputFrames() = %d, want %d", timeline.OutputFrames(), tt.wantFrames)
			}

			var emitted int64
			var lastSimulation, lastAudio int64
			for event, ok := timeline.Next(); ok; event, ok = timeline.Next() {
				if event.SimulationSample < lastSimulation || event.AudioSample < lastAudio {
					t.Fatal("timeline positions are not monotonic")
				}
				lastSimulation, lastAudio = event.SimulationSample, event.AudioSample
				if event.EmitOutput {
					if event.OutputIndex != emitted {
						t.Fatalf("output index = %d, want %d", event.OutputIndex, emitted)
					}
					emitted++
				}
			}
			if emitted != tt.wantFrames {
				t.Fatalf("emitted frames = %d, want %d", emitted, tt.wantFrames)
			}
		})
	}
}

func TestRecordingTimelineRejectsInvalidDuration(t *testing.T) {
	config := &RecordingSessionConfig{outputFPS: 60, sourceFPS: 60, oversample: 1, blendFrames: 1, audioSampleRate: 48_000}
	if _, err := NewRecordingTimeline(-1, config); err == nil {
		t.Fatal("NewRecordingTimeline accepted a negative duration")
	}
}
