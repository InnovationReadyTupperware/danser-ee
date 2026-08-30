package launcher

import (
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/innovationreadytupperware/danser-ee/app/database"
)

func TestBuildSongSelectCatalogViewSortsWithoutMutatingSnapshot(t *testing.T) {
	first := &database.BeatmapEntry{Dir: "set-b", File: "b.osu", Name: "Bravo"}
	second := &database.BeatmapEntry{Dir: "set-a", File: "a.osu", Name: "Alpha"}
	catalog := database.NewCatalogSnapshot([]*database.BeatmapEntry{first, second})

	view := buildSongSelectCatalogView(songSelectCatalogRequest{
		revision:  7,
		catalog:   catalog,
		sortBy:    Title,
		ascending: true,
	})

	if view.revision != 7 || view.catalog != catalog {
		t.Fatalf("view identity = revision %d catalog %p, want revision 7 catalog %p", view.revision, view.catalog, catalog)
	}
	if len(view.beatmaps) != 2 || view.beatmaps[0].entry != second || view.beatmaps[1].entry != first {
		t.Fatalf("sorted view = %#v, want Alpha then Bravo", view.beatmaps)
	}
	if catalog.Lookup(first.MapKey()) != first || catalog.Lookup(second.MapKey()) != second {
		t.Fatal("building the view changed catalog identity")
	}
}

func TestSongSelectCatalogWorkerPublishesLatestRequest(t *testing.T) {
	worker := newSongSelectCatalogWorker()
	defer worker.shutdown()

	for revision := uint64(1); revision <= 3; revision++ {
		catalog := database.NewCatalogSnapshot([]*database.BeatmapEntry{{
			Dir:  "set",
			File: string(rune('a'+revision-1)) + ".osu",
			Name: "Map",
		}})
		if !worker.request(songSelectCatalogRequest{
			revision:  revision,
			catalog:   catalog,
			sortBy:    Title,
			ascending: true,
		}) {
			t.Fatalf("request %d was rejected", revision)
		}
	}

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		if view, ok := worker.pollLatest(); ok && view.revision == 3 {
			return
		}

		select {
		case <-timer.C:
			t.Fatal("latest catalog view was not published")
		default:
			runtime.Gosched()
		}
	}
}

func TestSongSelectPopupRejectsStaleCatalogView(t *testing.T) {
	currentCatalog := database.NewCatalogSnapshot([]*database.BeatmapEntry{{Dir: "current", File: "map.osu"}})
	staleCatalog := database.NewCatalogSnapshot([]*database.BeatmapEntry{{Dir: "stale", File: "map.osu"}})
	popup := &songSelectPopup{
		catalog:         currentCatalog,
		catalogRevision: 2,
	}

	popup.applyCatalogView(buildSongSelectCatalogView(songSelectCatalogRequest{
		revision:  1,
		catalog:   staleCatalog,
		sortBy:    Title,
		ascending: true,
	}))

	if popup.catalogViewReady || len(popup.beatmaps) != 0 {
		t.Fatal("stale catalog view changed popup state")
	}
}

func BenchmarkBuildSongSelectCatalogView140K(b *testing.B) {
	entries := make([]*database.BeatmapEntry, 140_000)
	for i := range entries {
		entries[i] = &database.BeatmapEntry{
			Dir:  "set-" + strconv.Itoa(i/4),
			File: "map-" + strconv.Itoa(i%4) + ".osu",
			Name: "Map",
		}
	}
	catalog := database.NewCatalogSnapshot(entries)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		view := buildSongSelectCatalogView(songSelectCatalogRequest{
			catalog:   catalog,
			sortBy:    Title,
			ascending: true,
		})
		if len(view.beatmaps) != len(entries) {
			b.Fatalf("view contains %d entries, want %d", len(view.beatmaps), len(entries))
		}
	}
}
