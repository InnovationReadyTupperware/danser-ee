package launcher

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/wieku/danser-go/app/database"
)

// Catalog updates larger than this are published as a worker-built snapshot.
// Applying a large delta on the UI thread would make a full import look like
// a launcher freeze, while small deltas still avoid rebuilding every cached
// row.
const catalogSnapshotDeltaThreshold = 4096

type catalogRequest struct {
	generation    uint64
	skipMapUpdate bool
	callbacks     []func()
}

// catalogCoordinator owns the database manager's process-wide connection and
// serializes refreshes. The launcher can request another refresh at any time;
// the active scan is cancelled at its next safe boundary and requests are
// coalesced to one latest generation.
type catalogCoordinator struct {
	owner *launcher

	cancel context.CancelFunc
	wake   chan struct{}
	done   chan struct{}

	mu           sync.Mutex
	pending      *catalogRequest
	activeCancel context.CancelFunc
	closed       bool
	started      bool
}

func newCatalogCoordinator(owner *launcher) *catalogCoordinator {
	return &catalogCoordinator{
		owner: owner,
		wake:  make(chan struct{}, 1),
		done:  make(chan struct{}),
	}
}

func (c *catalogCoordinator) start(parent context.Context) {
	if c == nil || c.owner == nil || parent == nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)

	c.mu.Lock()
	if c.started || c.closed {
		c.mu.Unlock()
		cancel()
		return
	}
	c.started = true
	c.cancel = cancel
	c.mu.Unlock()

	go c.run(ctx)
}

// request never blocks the UI thread. Repeated watcher events collapse into a
// single request while retaining all continuations that belong to the latest
// refresh trigger.
func (c *catalogCoordinator) request(after func()) {
	if c == nil || c.owner == nil {
		return
	}

	request := catalogRequest{
		generation:    c.owner.catalogGeneration.Add(1),
		skipMapUpdate: launcherConfig.SkipMapUpdate,
	}
	if after != nil {
		request.callbacks = []func(){after}
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}

	if c.pending == nil {
		c.pending = &request
	} else {
		c.pending.generation = request.generation
		c.pending.skipMapUpdate = request.skipMapUpdate
		c.pending.callbacks = append(c.pending.callbacks, request.callbacks...)
	}

	if c.activeCancel != nil {
		c.activeCancel()
	}
	c.mu.Unlock()

	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *catalogCoordinator) close() {
	if c == nil {
		return
	}

	c.mu.Lock()
	if c.closed {
		started := c.started
		c.mu.Unlock()
		if started {
			<-c.done
		}
		return
	}
	c.closed = true
	if c.activeCancel != nil {
		c.activeCancel()
	}
	if c.cancel != nil {
		c.cancel()
	}
	started := c.started
	c.mu.Unlock()

	if started {
		<-c.done
	}
}

func (c *catalogCoordinator) run(ctx context.Context) {
	defer close(c.done)
	defer database.Close()
	defer func() {
		if recovered := recover(); recovered != nil {
			err := fmt.Errorf("catalog coordinator panic: %v", recovered)
			log.Println("Launcher:", err)
			c.setActiveCancel(nil)
			if c.owner != nil {
				c.owner.clearCatalogProgress()
				c.owner.postEventContext(ctx, launcherEvent{
					kind: launcherCatalogErrorEvent,
					err:  err,
				})
			}
		}
	}()

	databaseReady := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.wake:
		}

		for {
			request, ok := c.takePending()
			if !ok {
				break
			}

			operationCtx, cancel := context.WithCancel(ctx)
			c.setActiveCancel(cancel)
			// A newer generation owns the status line. Clear the previous
			// generation immediately so a cancelled scan cannot leave stale text
			// visible while the replacement is still below its visibility threshold.
			c.owner.clearCatalogProgress()

			if !databaseReady {
				if err := database.Init(); err != nil {
					c.owner.postEventContext(ctx, launcherEvent{
						kind:       launcherCatalogErrorEvent,
						generation: request.generation,
						err:        err,
					})
					cancel()
					c.setActiveCancel(nil)
					continue
				}

				databaseReady = true
			}

			// If a request was superseded before its cache snapshot reached the main
			// thread, publish the cache again for the latest generation. Tracking the
			// snapshot generation also covers an already-applied empty snapshot whose
			// optional Stable seed event was later discarded as stale.
			if !c.owner.catalogSnapshotReady.Load() || c.owner.catalogSnapshotGeneration.Load() < request.generation {
				c.publishCachedCatalog(ctx, request.generation)
			}

			// Stable metadata is an optional accelerator, but it can make a cold
			// catalog searchable while the authoritative filesystem pass is still
			// walking Songs. ReconcileCatalogContext checks the same condition again
			// and therefore does not parse the accelerator twice.
			stableDelta, stableErr := database.SeedCatalogFromStableDatabaseContext(operationCtx)
			if stableErr != nil {
				if !errors.Is(stableErr, context.Canceled) && !errors.Is(stableErr, context.DeadlineExceeded) {
					c.owner.postEventContext(ctx, launcherEvent{
						kind:       launcherCatalogErrorEvent,
						generation: request.generation,
						err:        stableErr,
					})
				}
				cancel()
				c.owner.clearCatalogProgressFor(request.generation)
				c.setActiveCancel(nil)
				continue
			}
			if len(stableDelta.Upserts) > 0 || len(stableDelta.Removals) > 0 {
				c.publishCatalogUpdate(operationCtx, catalogRequest{
					generation: request.generation,
				}, stableDelta)
			}

			delta, err := database.ReconcileCatalogContext(operationCtx, request.skipMapUpdate, c.owner.catalogImportListenerFor(request.generation))
			if err == nil {
				c.publishCatalogUpdate(operationCtx, request, delta)

				starDelta, starErr := database.UpdateCatalogStarRatingContext(operationCtx, c.owner.catalogStarRatingListenerFor(request.generation))
				if starErr != nil && !errors.Is(starErr, context.Canceled) && !errors.Is(starErr, context.DeadlineExceeded) {
					log.Println("DatabaseManager: Star-rating refresh failed:", starErr)
				} else if starErr == nil {
					c.publishCatalogUpdate(operationCtx, catalogRequest{
						generation: request.generation,
					}, starDelta)
				}
			} else if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				c.owner.postEventContext(ctx, launcherEvent{
					kind:       launcherCatalogErrorEvent,
					generation: request.generation,
					err:        err,
				})
			}

			cancel()
			c.owner.clearCatalogProgressFor(request.generation)
			c.setActiveCancel(nil)
		}
	}
}

func (c *catalogCoordinator) publishCatalogUpdate(ctx context.Context, request catalogRequest, delta database.CatalogDelta) {
	event := launcherEvent{
		kind:       launcherCatalogDeltaEvent,
		generation: request.generation,
		delta:      delta,
		callbacks:  request.callbacks,
	}

	if len(delta.Upserts)+len(delta.Removals) >= catalogSnapshotDeltaThreshold {
		event.kind = launcherCatalogSnapshotEvent
		catalog, err := database.LoadCachedCatalogContext(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				log.Println("DatabaseManager: Failed to publish refreshed catalog:", err)
			}
			return
		}
		event.catalog = catalog
		event.delta = database.CatalogDelta{}
	}

	c.owner.postEventContext(ctx, event)
}

func (c *catalogCoordinator) publishCachedCatalog(ctx context.Context, generation uint64) {
	catalog, err := database.LoadCachedCatalogContext(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			c.owner.postEventContext(ctx, launcherEvent{
				kind:       launcherCatalogErrorEvent,
				generation: generation,
				err:        fmt.Errorf("load cached catalog: %w", err),
			})
		}
		return
	}

	c.owner.postEventContext(ctx, launcherEvent{
		kind:       launcherCatalogSnapshotEvent,
		generation: generation,
		catalog:    catalog,
	})
}

func (c *catalogCoordinator) takePending() (catalogRequest, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pending == nil {
		return catalogRequest{}, false
	}

	request := *c.pending
	c.pending = nil
	return request, true
}

func (c *catalogCoordinator) setActiveCancel(cancel context.CancelFunc) {
	c.mu.Lock()
	c.activeCancel = cancel
	c.mu.Unlock()
}
