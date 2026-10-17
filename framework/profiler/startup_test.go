package profiler

import (
	"testing"
	"time"
)

func TestStartupProbeCheckpointDurations(t *testing.T) {
	base := time.Now()
	now := base
	probe := &StartupProbe{name: "test", started: base, last: base, now: func() time.Time { return now }}
	now = now.Add(12 * time.Millisecond)
	probe.Mark("settings")
	now = now.Add(8 * time.Millisecond)
	probe.Mark("first-frame")

	if len(probe.stages) != 2 {
		t.Fatalf("got %d checkpoints, want 2", len(probe.stages))
	}
	if got := probe.stages[0]; got.elapsed != 12*time.Millisecond || got.delta != 12*time.Millisecond {
		t.Fatalf("first checkpoint = %+v", got)
	}
	if got := probe.stages[1]; got.elapsed != 20*time.Millisecond || got.delta != 8*time.Millisecond {
		t.Fatalf("second checkpoint = %+v", got)
	}
	probe.Dump()
	probe.Dump()
	probe.Mark("ignored")
	if len(probe.stages) != 2 {
		t.Fatalf("checkpoint added after dump")
	}
}

func TestStartupProbeEnvironmentControlsAllocation(t *testing.T) {
	t.Setenv("DANSER_STARTUP_PROBE", "")
	probe := NewStartupProbe("disabled")
	if probe != nil {
		t.Fatal("disabled startup probe allocated a checkpoint buffer")
	}
	// Production checkpoints call Mark and Dump without checking for nil.
	probe.Mark("ignored")
	probe.Dump()

	t.Setenv("DANSER_STARTUP_PROBE", "1")
	probe = NewStartupProbe("enabled")
	if probe == nil {
		t.Fatal("startup probe was not created when enabled")
	}
}
