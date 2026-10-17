package profiler

import (
	"log"
	"os"
	"time"
)

// StartupProbe records monotonic startup checkpoints without logging on the
// measured thread. The probe is owned by one startup sequence and is not
// safe for concurrent calls.
type StartupProbe struct {
	name    string
	started time.Time
	last    time.Time
	now     func() time.Time
	stages  []startupStage
	dumped  bool
}

type startupStage struct {
	name    string
	elapsed time.Duration
	delta   time.Duration
}

// NewStartupProbe enables startup tracing with DANSER_STARTUP_PROBE.
// Checkpoint names must be static strings so logs remain comparable.
func NewStartupProbe(name string) *StartupProbe {
	if os.Getenv("DANSER_STARTUP_PROBE") == "" {
		return nil
	}
	now := time.Now()
	return &StartupProbe{
		name:    name,
		started: now,
		last:    now,
		now:     time.Now,
		stages:  make([]startupStage, 0, 16),
	}
}

// Mark records the elapsed time since the preceding checkpoint and startup.
// It is safe to call on a disabled probe.
func (probe *StartupProbe) Mark(name string) {
	if probe == nil || probe.dumped {
		return
	}
	now := probe.now()
	probe.stages = append(probe.stages, startupStage{
		name:    name,
		elapsed: now.Sub(probe.started),
		delta:   now.Sub(probe.last),
	})
	probe.last = now
}

// Dump logs each checkpoint once, preserving the observed stage order.
func (probe *StartupProbe) Dump() {
	if probe == nil || probe.dumped {
		return
	}
	probe.dumped = true
	for _, stage := range probe.stages {
		log.Printf("StartupProbe: name=%s stage=%s elapsed=%s delta=%s", probe.name, stage.name, stage.elapsed, stage.delta)
	}
}
