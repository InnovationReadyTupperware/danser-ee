package launcher

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/build"
	"github.com/innovationreadytupperware/danser-ee/framework/env"
	"github.com/innovationreadytupperware/danser-ee/framework/platform"
	"github.com/innovationreadytupperware/danser-ee/framework/platform/gcontext"
	"github.com/innovationreadytupperware/danser-ee/framework/util"
)

type managedProcess struct {
	ctx               context.Context
	cancel            context.CancelFunc
	done              chan struct{}
	mode              Mode
	outputMode        PMode
	launchRequestedAt time.Time
}

// Give Watch launches a short visual lead-in. The final 150ms of triangle
// deceleration overlaps child startup rather than delaying process creation.
const (
	triangleLaunchAnimationDuration = 600 * time.Millisecond
	triangleLaunchRampDuration      = 60 * time.Millisecond
	triangleLaunchPeakSpeed         = 25.0
	watchLaunchGracePeriod          = 450 * time.Millisecond
)

// pendingDanserStart is accessed only by the launcher frame/event thread.
// The timer goroutine posts an event carrying this exact request identity.
type pendingDanserStart struct {
	clickedAt   time.Time
	mode        Mode
	outputMode  PMode
	selectedMap *beatmap.BeatMap
	timer       *time.Timer
}

// processSupervisor owns the complete child-process lifecycle. In particular,
// no scanner or waiter mutates launcher/UI state directly; all observations
// cross the runtime event queue and are applied on the main thread.
type processSupervisor struct {
	owner *launcher

	mu     sync.Mutex
	active *managedProcess
}

type processProgress struct {
	status     string
	speed      string
	eta        string
	value      float32
	speedValue float64
}

type processResult struct {
	run *managedProcess

	mode        Mode
	outputMode  PMode
	showFile    bool
	started     bool
	startedAt   time.Time
	observedAt  time.Time
	encoding    bool
	resultFile  string
	diagnostics string
	duration    time.Duration
	canceled    bool
	err         error
}

func newProcessSupervisor(owner *launcher) *processSupervisor {
	return &processSupervisor{owner: owner}
}

func (s *processSupervisor) running() bool {
	if s == nil {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active != nil
}

func (s *processSupervisor) currentRun() *managedProcess {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

func (s *processSupervisor) stop() {
	if s == nil {
		return
	}

	s.mu.Lock()
	run := s.active
	s.mu.Unlock()

	if run != nil && run.cancel != nil {
		run.cancel()
	}
}

func (s *processSupervisor) close() {
	if s == nil {
		return
	}

	s.stop()

	s.mu.Lock()
	run := s.active
	s.mu.Unlock()
	if run != nil {
		<-run.done
	}
}

func (s *processSupervisor) start(executable string, args []string, mode Mode, outputMode PMode, showFile bool, requestedAt time.Time) error {
	if s == nil || s.owner == nil {
		return fmt.Errorf("launcher process supervisor is not initialized")
	}

	// The supervisor owns this operation context. Launcher shutdown cancels it
	// through stop before waiting for the child, so no long-lived context needs
	// to be stored on the supervisor or launcher structs.
	ctx, cancel := context.WithCancel(context.Background())
	run := &managedProcess{ctx: ctx, cancel: cancel, done: make(chan struct{}), mode: mode, outputMode: outputMode, launchRequestedAt: requestedAt}

	s.mu.Lock()
	if s.active != nil {
		s.mu.Unlock()
		cancel()
		return fmt.Errorf("danser is already running")
	}
	s.active = run
	s.mu.Unlock()

	go func() {
		result := processResult{
			run:        run,
			mode:       mode,
			outputMode: outputMode,
			showFile:   showFile,
		}

		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					result.err = fmt.Errorf("child process supervisor panic: %v", recovered)
				}
			}()
			result = s.execute(ctx, run, executable, args, result)
		}()

		close(run.done)
		s.owner.postEvent(launcherEvent{
			kind:    launcherProcessFinishedEvent,
			process: result,
		})
	}()

	return nil
}

func (s *processSupervisor) execute(ctx context.Context, run *managedProcess, executable string, args []string, result processResult) processResult {
	if runtime.GOOS != "windows" {
		if stat, err := os.Stat(executable); err == nil {
			if err := os.Chmod(executable, (stat.Mode()&os.ModePerm)|0111); err != nil {
				log.Println("Launcher: Could not mark danser executable:", err)
			}
		}
	}

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Env = env.LauncherChildEnvironment(true)
	cmd.Stdin = os.Stdin

	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		result.err = fmt.Errorf("create danser output pipe: %w", err)
		return result
	}

	defer readPipe.Close()
	defer writePipe.Close()

	cmd.Stdout = io.MultiWriter(os.Stdout, writePipe)
	cmd.Stderr = io.MultiWriter(os.Stderr, writePipe)

	if err := cmd.Start(); err != nil {
		result.err = fmt.Errorf("start danser: %w", err)
		result.canceled = ctx.Err() != nil
		return result
	}

	result.started = true
	result.startedAt = time.Now()
	s.owner.postEventContext(ctx, launcherEvent{
		kind: launcherProcessStartedEvent,
		process: processResult{
			run:        run,
			mode:       result.mode,
			outputMode: result.outputMode,
			showFile:   result.showFile,
			started:    true,
			startedAt:  result.startedAt,
		},
	})

	output := make(chan processOutput, 1)
	go scanProcessOutput(ctx, s.owner, run, result.outputMode, result.startedAt, readPipe, output)

	waitErr := cmd.Wait()
	// Wait does not own writePipe because it is wrapped by MultiWriter. Close
	// it explicitly so the scanner observes EOF before the result is emitted.
	_ = writePipe.Close()
	state := <-output

	result.encoding = state.encoding
	result.resultFile = state.resultFile
	result.diagnostics = state.diagnostics
	result.duration = time.Since(state.startedAt)
	result.canceled = ctx.Err() != nil
	result.err = waitErr
	return result
}

type processOutput struct {
	startedAt   time.Time
	encoding    bool
	resultFile  string
	diagnostics string
}

const (
	gameplayWindowCreatedLog = "Window created!"
	gameplaySplashReadyLog   = "Gameplay: loading screen presented"
	gameplayHandoffTimeout   = 5 * time.Second
)

func scanProcessOutput(ctx context.Context, owner *launcher, run *managedProcess, outputMode PMode, spawnedAt time.Time, reader io.Reader, output chan<- processOutput) {
	state := processOutput{startedAt: time.Now()}
	diagnostics := newDiagnosticTail()
	var handoffTimer *time.Timer
	splashSeen := false
	defer func() {
		if handoffTimer != nil {
			handoffTimer.Stop()
		}
		if recovered := recover(); recovered != nil {
			diagnostics.add(fmt.Sprintf("output scanner panic: %v", recovered))
		}

		state.diagnostics = diagnostics.string()
		select {
		case output <- state:
		case <-ctx.Done():
			// The supervisor still consumes the result after cancellation, so do
			// not leave the scanner goroutine blocked on a full result channel.
			output <- state
		}
	}()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 32*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		diagnostics.add(line)

		if outputMode == Watch {
			if strings.HasSuffix(line, gameplaySplashReadyLog) && !splashSeen {
				splashSeen = true
				if handoffTimer != nil {
					handoffTimer.Stop()
				}
				owner.postEventContext(ctx, launcherEvent{
					kind:    launcherProcessSplashReadyEvent,
					process: processResult{run: run, outputMode: outputMode, startedAt: spawnedAt, observedAt: time.Now()},
				})
			} else if strings.HasSuffix(line, gameplayWindowCreatedLog) && handoffTimer == nil && !splashSeen {
				// A legacy child may not emit the splash signal. Start the fallback
				// only after it has created its SDL window.
				handoffTimer = time.AfterFunc(gameplayHandoffTimeout, func() {
					owner.postEventContext(ctx, launcherEvent{
						kind:    launcherProcessHandoffTimeoutEvent,
						process: processResult{run: run, outputMode: outputMode, startedAt: spawnedAt, observedAt: time.Now()},
					})
				})
			}
		}

		if strings.Contains(line, "Launcher: Open settings") {
			owner.postEventContext(ctx, launcherEvent{kind: launcherProcessOpenSettingsEvent})
		}

		if strings.Contains(line, "Starting encoding!") || strings.Contains(line, "Recorder: Starting encoding session") {
			state.encoding = true
			owner.postEventContext(ctx, launcherEvent{kind: launcherProcessEncodingStartedEvent})
		}

		if strings.Contains(line, "Finishing rendering") {
			state.encoding = false
			owner.postEventContext(ctx, launcherEvent{kind: launcherProcessEncodingFinishedEvent})
		}

		if index := strings.Index(line, "Video is available at: "); index >= 0 {
			state.resultFile = strings.TrimSpace(line[index+len("Video is available at: "):])
		}

		if index := strings.Index(line, "Screenshot "); index >= 0 && strings.Contains(line, " saved!") {
			name := strings.TrimSuffix(strings.TrimPrefix(line[index:], "Screenshot "), " saved!")
			state.resultFile = filepath.Join(env.DataDir(), "screenshots", name)
		}

		if state.encoding {
			if progress, ok := parseProcessProgress(line); ok {
				owner.postEventContext(ctx, launcherEvent{
					kind:            launcherProcessProgressEvent,
					processProgress: progress,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		diagnostics.add("output scanner: " + err.Error())
	}

}

func parseProcessProgress(line string) (processProgress, bool) {
	index := strings.Index(line, "Progress")
	if index < 0 {
		return processProgress{}, false
	}

	parts := strings.SplitN(line[index:], ",", 3)
	if len(parts) < 3 {
		return processProgress{}, false
	}

	status := strings.TrimSpace(parts[0])
	colon := strings.IndexByte(status, ':')
	if colon < 0 {
		return processProgress{}, false
	}

	statusValue := strings.TrimSpace(status[colon+1:])
	if !strings.HasSuffix(statusValue, "%") {
		return processProgress{}, false
	}

	percentText := strings.TrimSpace(strings.TrimSuffix(statusValue, "%"))
	percent, err := strconv.ParseFloat(percentText, 32)
	if err != nil || math.IsNaN(percent) || math.IsInf(percent, 0) || percent < 0 || percent > 100 {
		return processProgress{}, false
	}

	speed := strings.TrimSpace(parts[1])
	eta := strings.TrimSpace(parts[2])
	speedValue := parseLabeledFloat(speed)

	return processProgress{
		status:     statusValue,
		speed:      speed,
		eta:        eta,
		value:      float32(percent / 100),
		speedValue: speedValue,
	}, true
}

func parseLabeledFloat(value string) float64 {
	if colon := strings.IndexByte(value, ':'); colon >= 0 {
		value = value[colon+1:]
	}
	value = strings.TrimSpace(strings.TrimRight(value, "xX"))

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0
	}

	return parsed
}

const diagnosticTailLimit = 16 * 1024

type diagnosticTail struct {
	lines []string
	bytes int
}

func newDiagnosticTail() *diagnosticTail {
	return &diagnosticTail{}
}

func (tail *diagnosticTail) add(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	tail.lines = append(tail.lines, line)
	tail.bytes += len(line) + 1
	for tail.bytes > diagnosticTailLimit && len(tail.lines) > 1 {
		tail.bytes -= len(tail.lines[0]) + 1
		tail.lines = tail.lines[1:]
	}
}

func (tail *diagnosticTail) string() string {
	return strings.Join(tail.lines, "\n")
}

func (l *launcher) scheduleDanserStart(clickedAt time.Time) {
	if l.danserRunning || l.pendingStart != nil {
		return
	}

	pending := &pendingDanserStart{
		clickedAt:   clickedAt,
		mode:        launcherConfig.CurrentMode,
		outputMode:  launcherConfig.CurrentPMode,
		selectedMap: l.bld.currentMap,
	}
	l.pendingStart = pending
	pending.timer = time.AfterFunc(watchLaunchGracePeriod, func() {
		// No launcher state is read on this goroutine. Even if Stop loses a race,
		// the event-thread identity check rejects canceled or superseded starts.
		l.postEvent(launcherEvent{
			kind: launcherStartupTaskEvent,
			task: func() {
				l.launchPendingDanserStart(pending)
			},
		})
	})
}

func (l *launcher) cancelPendingDanserStart() {
	if l.pendingStart == nil {
		return
	}
	if l.pendingStart.timer != nil {
		l.pendingStart.timer.Stop()
	}
	l.pendingStart = nil
}

func (l *launcher) launchPendingDanserStart(pending *pendingDanserStart) {
	if pending == nil || l.pendingStart != pending {
		return
	}
	l.cancelPendingDanserStart()
	if launcherConfig.CurrentMode != pending.mode || launcherConfig.CurrentPMode != pending.outputMode ||
		l.bld.currentMap != pending.selectedMap {
		log.Println("Launcher: Warning: Selected launch changed during triangle animation; skipping gameplay start")
		if l.triangleSpeed != nil {
			l.triangleSpeed.Reset()
			l.triangleSpeed.AddEvent(l.triangleSpeed.GetTime(), l.triangleSpeed.GetTime()+500, 1)
		}
		return
	}
	l.startDanser(pending.clickedAt)
}

func (l *launcher) startDanser(clickedAt time.Time) {
	if l.processSupervisor == nil || l.danserRunning {
		return
	}

	executable := os.Args[0]
	if build.IsRelease() {
		executable = filepath.Join(env.LibDir(), build.DanserExec)
	}
	if executable == "" {
		showMessage(mError, "danser failed to start: executable path is empty")
		return
	}

	args, err := l.bld.getArgumentsChecked()
	if err != nil {
		showMessage(mError, "danser failed to start: %s", err)
		return
	}
	log.Println("Launcher: Starting danser with arguments:", args)
	l.recordProgress = 0
	l.recordStatus = "Preparing..."
	l.recordStatusSpeed = ""
	l.recordStatusETA = ""
	l.encodeInProgress = false
	l.showProgressBar = launcherConfig.CurrentPMode == Record
	l.danserRunning = true
	l.gameplayHandedOff = false

	if err := l.processSupervisor.start(executable, args, launcherConfig.CurrentMode, launcherConfig.CurrentPMode, launcherConfig.ShowFileAfter, clickedAt); err != nil {
		l.danserRunning = false
		l.showProgressBar = false
		l.recordStatus = ""
		showMessage(mError, "danser failed to start: %s", err)
	}
}

func (l *launcher) applyProcessEvent(event launcherEvent) {
	switch event.kind {
	case launcherProcessStartedEvent:
		if !event.process.started {
			return
		}

		if event.process.run != nil && !event.process.run.launchRequestedAt.IsZero() {
			log.Printf("Launcher: gameplay child spawned after %s", event.process.startedAt.Sub(event.process.run.launchRequestedAt))
		}
	case launcherProcessSplashReadyEvent:
		l.handoffToGameplay(event.process, false)
	case launcherProcessHandoffTimeoutEvent:
		l.handoffToGameplay(event.process, true)
	case launcherProcessEncodingStartedEvent:
		l.encodeInProgress = true
		l.encodeStart = time.Now()
		gcontext.StartProgress()
	case launcherProcessEncodingFinishedEvent:
		l.encodeInProgress = false
		l.recordProgress = 1
		l.recordStatus = "Finalizing..."
		l.recordStatusSpeed = ""
		l.recordStatusETA = ""
		gcontext.SetProgress(1)
	case launcherProcessProgressEvent:
		progress := event.processProgress
		l.recordStatus = progress.status
		l.recordStatusSpeed = progress.speed
		l.recordStatusETA = progress.eta
		l.recordProgress = progress.value
		if l.triangleSpeed != nil {
			l.triangleSpeed.AddEvent(l.triangleSpeed.GetTime(), l.triangleSpeed.GetTime()+500, progress.speedValue)
		}
		gcontext.SetProgress(progress.value)
	case launcherProcessOpenSettingsEvent:
		if l.currentEditor == nil || !l.currentEditor.opened {
			l.openCurrentSettingsEditor()
		}
		gcontext.Restore()
		gcontext.Focus()
	case launcherProcessFinishedEvent:
		l.finishDanser(event.process)
	}
}

func (s *processSupervisor) activeRun(run *managedProcess) bool {
	if s == nil || run == nil {
		return false
	}
	s.mu.Lock()
	active := s.active == run
	s.mu.Unlock()
	if !active || run.ctx == nil || run.ctx.Err() != nil {
		return false
	}
	select {
	case <-run.done:
		return false
	default:
		return true
	}
}

func (l *launcher) handoffToGameplay(result processResult, timedOut bool) {
	if l.gameplayHandedOff || result.outputMode != Watch || !l.danserRunning ||
		l.processSupervisor == nil || !l.processSupervisor.activeRun(result.run) {
		return
	}
	l.gameplayHandedOff = true
	if timedOut {
		log.Printf("Launcher: gameplay loading-screen signal not received within %s after window creation, proceeding with handoff", gameplayHandoffTimeout)
	} else if !result.run.launchRequestedAt.IsZero() {
		log.Printf("Launcher: gameplay loading screen presented after %s (child startup %s)",
			result.observedAt.Sub(result.run.launchRequestedAt), result.observedAt.Sub(result.startedAt))
	}
	gcontext.Minimize()
}

func (l *launcher) finishDanser(result processResult) {
	l.dismissProcessCancelDialog(result.run)
	if l.processSupervisor != nil {
		l.processSupervisor.finish(result.run)
	}

	l.danserRunning = false
	l.gameplayHandedOff = false
	l.encodeInProgress = false
	l.recordStatusSpeed = ""
	l.recordStatusETA = ""
	if l.triangleSpeed != nil {
		l.triangleSpeed.AddEvent(l.triangleSpeed.GetTime(), l.triangleSpeed.GetTime()+500, 1)
	}

	if result.err != nil && !result.canceled {
		gcontext.ErrorProgress()
		gcontext.Restore()
		gcontext.Focus()
		message := result.diagnostics
		if message == "" {
			message = "No diagnostic output was captured."
		}
		showMessage(mError, "danser crashed: %s\n\n%s", result.err, message)
	} else {
		gcontext.StopProgress()
		gcontext.Restore()
		if result.canceled {
			l.recordStatus = ""
			l.showProgressBar = false
		} else {
			l.recordProgress = 1
			l.recordStatus = "Done in " + util.FormatSeconds(int(result.duration.Seconds()))
			if result.outputMode != Watch && result.mode != Play {
				if result.showFile && result.resultFile != "" {
					platform.ShowFileInManager(result.resultFile)
				}
				platform.Beep(platform.Ok)
			}
		}
	}

	if l.closeAfterDanser {
		l.closeAfterDanser = false
		gcontext.SetShouldClose(true)
	}
}

func (s *processSupervisor) finish(run *managedProcess) {
	if s == nil {
		return
	}

	s.mu.Lock()
	if s.active == run {
		s.active = nil
	}
	s.mu.Unlock()
}
