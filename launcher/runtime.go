package launcher

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/innovationreadytupperware/danser-ee/app/database"
	"github.com/innovationreadytupperware/danser-ee/framework/bass"
	"github.com/innovationreadytupperware/danser-ee/framework/platform/gcontext"
	"github.com/innovationreadytupperware/danser-ee/framework/qpc"
)

// launcherEventKind identifies work that crossed from a worker into the
// launcher's main thread. SDL, ImGui, and launcher-owned mutable state are
// touched only while these events are drained by the frame loop.
type launcherEventKind uint8

const (
	launcherCatalogSnapshotEvent launcherEventKind = iota
	launcherCatalogDeltaEvent
	launcherCatalogErrorEvent
	launcherBeatmapChangedEvent
	launcherProcessStartedEvent
	launcherProcessEncodingStartedEvent
	launcherProcessEncodingFinishedEvent
	launcherProcessProgressEvent
	launcherProcessOpenSettingsEvent
	launcherProcessFinishedEvent
	launcherStartupTaskEvent
)

// launcherEvent is deliberately a value type. Worker results are immutable by
// the time they enter the queue, which avoids sharing partially updated state
// with the renderer or depending on a callback that might outlive shutdown.
type launcherEvent struct {
	kind       launcherEventKind
	generation uint64

	catalog *database.CatalogSnapshot
	delta   database.CatalogDelta

	processProgress processProgress
	err             error

	callbacks []func()
	process   processResult
	task      func()
}

// The queue covers several frames of encoder progress while bounding memory
// use. Catalog work emits only a few events, while process output can emit one
// progress event per line; producers stop waiting when the runtime context is
// cancelled instead of growing an unbounded backlog.
const launcherEventQueueCapacity = 256

func (l *launcher) shutdown() {
	if l == nil {
		return
	}

	l.shutdownOnce.Do(func() {
		if l.startTimer != nil {
			l.startTimer.Stop()
			l.startTimer = nil
		}
		if l.runtimeCancel != nil {
			l.runtimeCancel()
		}
		if l.processSupervisor != nil {
			l.processSupervisor.close()
		}
		if l.catalogCoordinator != nil {
			l.catalogCoordinator.close()
		}
		if l.songSelectCatalogWorker != nil {
			l.songSelectCatalogWorker.shutdown()
		}
		if l.selectWindow != nil {
			l.selectWindow.shutdown()
		}
		if l.audioReady {
			bass.Shutdown()
			l.audioReady = false
		}
		l.backgroundWG.Wait()

		l.closeWatcher()
		saveLauncherConfig()
		if l.currentConfig != nil {
			if err := l.currentConfig.SaveChecked("", false); err != nil {
				log.Printf("Launcher: Failed to save current profile: %v", err)
			}
		}
	})
}

func (l *launcher) postEvent(event launcherEvent) bool {
	if l == nil || l.runtimeEvents == nil || l.runtimeDone == nil {
		return false
	}

	select {
	case l.runtimeEvents <- event:
		return true
	case <-l.runtimeDone:
		return false
	}
}

func (l *launcher) postEventContext(ctx context.Context, event launcherEvent) bool {
	if l == nil || l.runtimeEvents == nil || ctx == nil {
		return false
	}

	select {
	case l.runtimeEvents <- event:
		return true
	case <-ctx.Done():
		return false
	}
}

// postBeatmapChangedEvent coalesces filesystem bursts before they enter the
// bounded launcher queue. A single import prompt is sufficient for a set
// extraction or editor save that emits many fsnotify events.
func (l *launcher) postBeatmapChangedEvent() {
	if l == nil || !l.beatmapEventPending.CompareAndSwap(false, true) {
		return
	}

	if !l.postEvent(launcherEvent{kind: launcherBeatmapChangedEvent}) {
		l.beatmapEventPending.Store(false)
	}
}

// drainEvents is called once per launcher frame. A bounded batch prevents a
// burst of process or catalog notifications from starving input and drawing.
func (l *launcher) drainEvents() {
	const maxEventsPerFrame = 128

	for i := 0; i < maxEventsPerFrame; i++ {
		select {
		case event := <-l.runtimeEvents:
			l.applyEvent(event)
		default:
			return
		}
	}
}

func (l *launcher) applyEvent(event launcherEvent) {
	switch event.kind {
	case launcherCatalogSnapshotEvent:
		if event.generation < l.catalogGeneration.Load() {
			return
		}

		if event.catalog == nil {
			event.catalog = database.NewCatalogSnapshot(nil)
		}

		l.catalog = event.catalog
		if event.generation != 0 {
			l.catalogSnapshotGeneration.Store(event.generation)
		}
		l.catalogSnapshotReady.Store(true)
		l.updateSongSelectCatalog(event.catalog)
		l.runCatalogCallbacks(event.callbacks)
	case launcherCatalogDeltaEvent:
		if event.generation < l.catalogGeneration.Load() {
			return
		}

		if len(event.delta.Upserts) > 0 || len(event.delta.Removals) > 0 {
			if l.catalog == nil {
				l.catalog = database.NewCatalogSnapshot(nil)
			}

			l.catalog = l.catalog.ApplyDelta(event.delta)
			l.updateSongSelectCatalog(l.catalog)
		}

		l.runCatalogCallbacks(event.callbacks)
	case launcherCatalogErrorEvent:
		if event.generation != 0 && event.generation != l.catalogGeneration.Load() {
			return
		}
		if event.err == nil || errors.Is(event.err, context.Canceled) || errors.Is(event.err, context.DeadlineExceeded) {
			return
		}

		showMessage(mError, "Catalog refresh failed: %s", event.err)
	case launcherBeatmapChangedEvent:
		l.beatmapEventPending.Store(false)
		delay := 3000.0
		if launcherConfig.AutoRefreshDB {
			delay = 6000
		}
		l.showBeatmapAlert = qpc.GetMilliTimeF() + delay
		l.beatmapDirUpdated = true
	case launcherProcessStartedEvent,
		launcherProcessEncodingStartedEvent,
		launcherProcessEncodingFinishedEvent,
		launcherProcessProgressEvent,
		launcherProcessOpenSettingsEvent,
		launcherProcessFinishedEvent:
		l.applyProcessEvent(event)
	case launcherStartupTaskEvent:
		if event.task != nil {
			event.task()
		}
	}
}

func (l *launcher) runCatalogCallbacks(callbacks []func()) {
	for _, callback := range callbacks {
		if callback != nil {
			callback()
		}
	}
}

func (l *launcher) processStartupSelection() {
	if len(os.Args) > 2 {
		l.trySelectReplaysFromPaths(os.Args[1:])
	} else if len(os.Args) > 1 {
		l.trySelectReplayFromPath(os.Args[1])
	} else if launcherConfig.LoadLatestReplay {
		l.loadLatestReplay()
	}
}

func (l *launcher) handleDrop(event gcontext.DropEvent) {
	paths := make([]string, 0, len(event.Names))
	for _, name := range event.Names {
		if strings.TrimSpace(name) != "" {
			paths = append(paths, name)
		}
	}
	if len(paths) == 0 || l.danserRunning {
		return
	}

	hasArchive := false
	for _, name := range paths {
		if strings.EqualFold(filepath.Ext(name), ".osz") {
			hasArchive = true
			break
		}
	}

	if hasArchive {
		l.loadOSZs(paths)
	} else if len(paths) > 1 {
		l.trySelectReplaysFromPaths(paths)
	} else {
		l.trySelectReplayFromPath(paths[0])
	}
}

func (l *launcher) handleCloseRequest() {
	if l.danserStartPending {
		if l.startTimer != nil {
			l.startTimer.Stop()
			l.startTimer = nil
		}
		l.danserStartPending = false
		gcontext.SetShouldClose(true)
		return
	}

	if l.processSupervisor == nil || !l.processSupervisor.running() {
		gcontext.SetShouldClose(true)
		return
	}

	gcontext.SetShouldClose(false)
	if !showMessage(mQuestion, "Recording is in progress, do you want to exit?") {
		return
	}

	l.closeAfterDanser = true
	l.processSupervisor.stop()
}
