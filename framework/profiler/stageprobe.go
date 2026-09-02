package profiler

import (
	"cmp"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"time"
)

const (
	stageProbeCapacity  = 128
	stageProbeMaxStages = 12
)

// StageProbe retains the slowest samples in memory and writes them only when
// Dump is called. It is intended for opt-in realtime diagnosis without doing
// file or console I/O on the measured thread. A probe and its samples belong
// to one goroutine; concurrent use is not supported.
type StageProbe struct {
	name    string
	started time.Time
	stages  []string
	samples []StageSample
	now     func() time.Time
}

// StageSample records one operation split into ordered stages.
type StageSample struct {
	started time.Time
	last    time.Time
	elapsed time.Duration
	total   time.Duration
	stages  [stageProbeMaxStages]time.Duration
	next    int
	active  bool
	now     func() time.Time
}

// NewStageProbe creates an enabled probe when DANSER_FRAME_PROBE is set.
// It returns nil otherwise, allowing call sites to remain cheap when disabled.
// Names must be static diagnostic labels, not user-provided values.
func NewStageProbe(name string, stages ...string) *StageProbe {
	if len(stages) > stageProbeMaxStages {
		panic(fmt.Sprintf("stage probe %q has %d stages, maximum is %d", name, len(stages), stageProbeMaxStages))
	}
	if os.Getenv("DANSER_FRAME_PROBE") == "" {
		return nil
	}

	return newStageProbe(name, stages, time.Now)
}

func newStageProbe(name string, stages []string, now func() time.Time) *StageProbe {
	return &StageProbe{
		name:    name,
		started: now(),
		stages:  slices.Clone(stages),
		samples: make([]StageSample, 0, stageProbeCapacity),
		now:     now,
	}
}

// Begin starts one sample. Calling it on a disabled probe is safe.
func (probe *StageProbe) Begin() StageSample {
	if probe == nil {
		return StageSample{}
	}

	now := probe.now()
	return StageSample{started: now, last: now, active: true, now: probe.now}
}

// Mark completes the next stage in the sample.
func (sample *StageSample) Mark() {
	if !sample.active || sample.next >= len(sample.stages) {
		return
	}

	now := sample.now()
	sample.stages[sample.next] = now.Sub(sample.last)
	sample.last = now
	sample.next++
}

// Commit retains sample if it belongs in the probe's bounded slowest set.
func (probe *StageProbe) Commit(sample StageSample) {
	if probe == nil || !sample.active {
		return
	}

	now := probe.now()
	sample.total = now.Sub(sample.started)
	sample.elapsed = now.Sub(probe.started)

	if len(probe.samples) < cap(probe.samples) {
		probe.samples = append(probe.samples, sample)
		return
	}

	fastest := 0
	for i := 1; i < len(probe.samples); i++ {
		if probe.samples[i].total < probe.samples[fastest].total {
			fastest = i
		}
	}

	if sample.total > probe.samples[fastest].total {
		probe.samples[fastest] = sample
	}
}

// Dump writes the retained samples from slowest to fastest.
func (probe *StageProbe) Dump() {
	if probe == nil {
		return
	}

	slices.SortFunc(probe.samples, func(a, b StageSample) int {
		return cmp.Compare(b.total, a.total)
	})

	var line strings.Builder
	for _, sample := range probe.samples {
		line.Reset()
		_, _ = fmt.Fprintf(&line, "FrameProbe: name=%s at=%s total=%s", probe.name, sample.elapsed, sample.total)
		for i, stage := range probe.stages {
			_, _ = fmt.Fprintf(&line, " %s=%s", stage, sample.stages[i])
		}
		log.Print(line.String())
	}
}
