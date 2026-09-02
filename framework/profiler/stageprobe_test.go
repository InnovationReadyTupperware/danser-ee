package profiler

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

func TestNewStageProbe_Disabled(t *testing.T) {
	t.Setenv("DANSER_FRAME_PROBE", "")

	if probe := NewStageProbe("test", "work"); probe != nil {
		t.Fatal("disabled probe was created")
	}
}

func TestNewStageProbe_RejectsTooManyStages(t *testing.T) {
	t.Setenv("DANSER_FRAME_PROBE", "")
	stages := make([]string, stageProbeMaxStages+1)

	defer func() {
		if recover() == nil {
			t.Fatal("too many stages did not panic")
		}
	}()
	NewStageProbe("test", stages...)
}

func TestStageProbe_RecordsStagesAndTotal(t *testing.T) {
	base := time.Unix(0, 0)
	times := []time.Time{
		base,
		base.Add(time.Millisecond),
		base.Add(3 * time.Millisecond),
		base.Add(6 * time.Millisecond),
		base.Add(10 * time.Millisecond),
	}
	next := 0
	probe := newStageProbe("test", []string{"first", "second"}, func() time.Time {
		now := times[next]
		next++
		return now
	})

	sample := probe.Begin()
	sample.Mark()
	sample.Mark()
	probe.Commit(sample)

	if got, want := len(probe.samples), 1; got != want {
		t.Fatalf("sample count = %d, want %d", got, want)
	}
	recorded := probe.samples[0]
	if got, want := recorded.stages[0], 2*time.Millisecond; got != want {
		t.Fatalf("first stage = %v, want %v", got, want)
	}
	if got, want := recorded.stages[1], 3*time.Millisecond; got != want {
		t.Fatalf("second stage = %v, want %v", got, want)
	}
	if got, want := recorded.total, 9*time.Millisecond; got != want {
		t.Fatalf("total = %v, want %v", got, want)
	}
}

func TestStageProbe_RetainsOnlySlowestSamples(t *testing.T) {
	base := time.Unix(0, 0)
	probe := newStageProbe("test", nil, func() time.Time {
		return base.Add(time.Second)
	})

	for duration := time.Millisecond; duration <= (stageProbeCapacity+1)*time.Millisecond; duration += time.Millisecond {
		probe.Commit(StageSample{
			started: base.Add(time.Second).Add(-duration),
			active:  true,
		})
	}

	if got, want := len(probe.samples), stageProbeCapacity; got != want {
		t.Fatalf("sample count = %d, want %d", got, want)
	}
	for _, sample := range probe.samples {
		if sample.total == time.Millisecond {
			t.Fatal("fastest sample was retained")
		}
	}
}

func TestStageProbe_DumpIsOrderedAndBounded(t *testing.T) {
	base := time.Unix(0, 0)
	probe := newStageProbe("render", []string{"draw"}, func() time.Time { return base })
	probe.samples = append(probe.samples,
		StageSample{elapsed: 2 * time.Second, total: 2 * time.Millisecond, stages: [stageProbeMaxStages]time.Duration{time.Millisecond}},
		StageSample{elapsed: time.Second, total: 4 * time.Millisecond, stages: [stageProbeMaxStages]time.Duration{3 * time.Millisecond}},
	)

	var output bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&output)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	}()

	probe.Dump()
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if got, want := len(lines), 2; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}
	if !strings.Contains(lines[0], "name=render at=1s total=4ms draw=3ms") {
		t.Fatalf("first line = %q, want slowest sample", lines[0])
	}
	if !strings.Contains(lines[1], "name=render at=2s total=2ms draw=1ms") {
		t.Fatalf("second line = %q, want next slowest sample", lines[1])
	}
}

func BenchmarkStageProbeDisabled(b *testing.B) {
	b.Setenv("DANSER_FRAME_PROBE", "")
	probe := NewStageProbe("render", "draw")
	b.ReportAllocs()

	for b.Loop() {
		sample := probe.Begin()
		sample.Mark()
		probe.Commit(sample)
	}
}
