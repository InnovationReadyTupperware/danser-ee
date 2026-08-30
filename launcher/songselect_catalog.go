package launcher

import (
	"sync"

	"github.com/innovationreadytupperware/danser-ee/app/database"
)

type songSelectCatalogRequest struct {
	revision  uint64
	catalog   *database.CatalogSnapshot
	sortBy    SortBy
	ascending bool
}

// songSelectCatalogView is immutable after the worker publishes it. Ownership
// of the search entries transfers to the launcher thread with the value.
type songSelectCatalogView struct {
	revision         uint64
	catalog          *database.CatalogSnapshot
	beatmaps         searchEntries
	groupByDirectory map[string]int
	groupCount       int
}

func buildSongSelectCatalogView(request songSelectCatalogRequest) songSelectCatalogView {
	beatmaps := make(searchEntries, 0)
	if request.catalog != nil {
		beatmaps = make(searchEntries, 0, request.catalog.Len())
		request.catalog.ForEach(func(entry *database.BeatmapEntry) bool {
			if searchEntry := newSearchEntry(entry); searchEntry != nil {
				beatmaps = append(beatmaps, searchEntry)
			}
			return true
		})
	}

	groups := sortMaps(beatmaps, request.sortBy, request.ascending)
	return songSelectCatalogView{
		revision:         request.revision,
		catalog:          request.catalog,
		beatmaps:         beatmaps,
		groupByDirectory: groups,
		groupCount:       len(groups),
	}
}

// songSelectCatalogWorker serializes full catalog preparation and keeps only
// the newest pending request and result. A large import can therefore publish
// many durable batches without creating one goroutine or queued view per batch.
type songSelectCatalogWorker struct {
	requests chan songSelectCatalogRequest
	results  chan songSelectCatalogView
	stop     chan struct{}
	done     chan struct{}
	close    func()
}

func newSongSelectCatalogWorker() *songSelectCatalogWorker {
	worker := &songSelectCatalogWorker{
		requests: make(chan songSelectCatalogRequest, 1),
		results:  make(chan songSelectCatalogView, 1),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	worker.close = sync.OnceFunc(func() {
		close(worker.stop)
		<-worker.done
	})

	go worker.run()
	return worker
}

func (w *songSelectCatalogWorker) run() {
	defer close(w.done)

	for {
		select {
		case <-w.stop:
			return
		case request := <-w.requests:
			view := buildSongSelectCatalogView(request)
			w.publishLatest(view)
		}
	}
}

func (w *songSelectCatalogWorker) request(request songSelectCatalogRequest) bool {
	if w == nil {
		return false
	}

	for {
		select {
		case <-w.stop:
			return false
		case w.requests <- request:
			return true
		default:
		}

		select {
		case <-w.requests:
		default:
		}
	}
}

func (w *songSelectCatalogWorker) publishLatest(view songSelectCatalogView) {
	select {
	case <-w.stop:
		return
	case w.results <- view:
		return
	default:
	}

	select {
	case <-w.results:
	default:
	}

	select {
	case <-w.stop:
	case w.results <- view:
	}
}

func (w *songSelectCatalogWorker) pollLatest() (songSelectCatalogView, bool) {
	if w == nil {
		return songSelectCatalogView{}, false
	}

	var latest songSelectCatalogView
	found := false
	for {
		select {
		case latest = <-w.results:
			found = true
		default:
			return latest, found
		}
	}
}

func (w *songSelectCatalogWorker) shutdown() {
	if w != nil && w.close != nil {
		w.close()
	}
}
