package launcher

import (
	"sync"

	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
)

type songSelectThumbnailRequest struct {
	revision uint64
	path     string
}

type songSelectThumbnailResult struct {
	revision uint64
	path     string
	pixmap   *texture.Pixmap
}

// songSelectThumbnailWorker keeps filesystem image decoding off the launcher
// thread. Texture creation still happens on the main thread because it owns
// the OpenGL context.
type songSelectThumbnailWorker struct {
	requests chan songSelectThumbnailRequest
	results  chan songSelectThumbnailResult
	stop     chan struct{}
	done     chan struct{}
	close    func()
}

func newSongSelectThumbnailWorker() *songSelectThumbnailWorker {
	worker := &songSelectThumbnailWorker{
		requests: make(chan songSelectThumbnailRequest, 1),
		results:  make(chan songSelectThumbnailResult, 1),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	worker.close = sync.OnceFunc(func() {
		close(worker.stop)
		<-worker.done
		for {
			select {
			case result := <-worker.results:
				if result.pixmap != nil {
					result.pixmap.Dispose()
				}
			default:
				return
			}
		}
	})

	go worker.run()
	return worker
}

func (w *songSelectThumbnailWorker) run() {
	defer close(w.done)

	for {
		select {
		case <-w.stop:
			return
		case request := <-w.requests:
			pixmap, err := texture.NewPixmapFileString(request.path)
			if err != nil {
				pixmap = nil
			}
			w.publishLatest(songSelectThumbnailResult{
				revision: request.revision,
				path:     request.path,
				pixmap:   pixmap,
			})
		}
	}
}

func (w *songSelectThumbnailWorker) request(request songSelectThumbnailRequest) bool {
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

func (w *songSelectThumbnailWorker) publishLatest(result songSelectThumbnailResult) {
	select {
	case <-w.stop:
		if result.pixmap != nil {
			result.pixmap.Dispose()
		}
		return
	case w.results <- result:
		return
	default:
	}

	select {
	case old := <-w.results:
		if old.pixmap != nil {
			old.pixmap.Dispose()
		}
	default:
	}

	select {
	case <-w.stop:
		if result.pixmap != nil {
			result.pixmap.Dispose()
		}
	case w.results <- result:
	}
}

func (w *songSelectThumbnailWorker) pollLatest() (songSelectThumbnailResult, bool) {
	if w == nil {
		return songSelectThumbnailResult{}, false
	}

	var latest songSelectThumbnailResult
	found := false
	for {
		select {
		case result := <-w.results:
			if found && latest.pixmap != nil {
				latest.pixmap.Dispose()
			}
			latest = result
			found = true
		default:
			return latest, found
		}
	}
}

func (w *songSelectThumbnailWorker) shutdown() {
	if w != nil && w.close != nil {
		w.close()
	}
}
