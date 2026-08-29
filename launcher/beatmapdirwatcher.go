package launcher

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// directoryWatcher owns the fsnotify channels and their goroutine. The
// callback is intentionally a notification boundary: filesystem events never
// mutate ImGui or launcher state from the watcher goroutine.
type directoryWatcher struct {
	watcher *fsnotify.Watcher
	done    chan struct{}
	onEvent func()
	once    sync.Once
}

func newDirectoryWatcher(root string, onEvent func()) (*directoryWatcher, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "watch", Path: abs, Err: os.ErrInvalid}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := watcher.Add(abs); err != nil {
		_ = watcher.Close()
		return nil, err
	}

	directoryWatcher := &directoryWatcher{
		watcher: watcher,
		done:    make(chan struct{}),
		onEvent: onEvent,
	}
	go directoryWatcher.run()

	return directoryWatcher, nil
}

func (directoryWatcher *directoryWatcher) run() {
	defer close(directoryWatcher.done)
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("DirWatcher: callback failed: %v", recovered)
		}
	}()

	for {
		select {
		case event, ok := <-directoryWatcher.watcher.Events:
			if !ok {
				return
			}

			log.Println("DirWatcher: New event:", event)
			if directoryWatcher.onEvent != nil {
				directoryWatcher.onEvent()
			}
		case err, ok := <-directoryWatcher.watcher.Errors:
			if !ok {
				return
			}

			log.Println("DirWatcher: Error:", err)
		}
	}
}

func (directoryWatcher *directoryWatcher) close() {
	if directoryWatcher == nil {
		return
	}

	directoryWatcher.once.Do(func() {
		if err := directoryWatcher.watcher.Close(); err != nil {
			log.Println("DirWatcher: Failed to close:", err)
		}
	})
	<-directoryWatcher.done
}
