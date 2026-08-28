package launcher

import (
	"testing"

	"github.com/wieku/danser-go/app/database"
)

func TestCatalogProgressMessage(t *testing.T) {
	tests := []struct {
		name     string
		progress catalogProgressState
		want     string
	}{
		{
			name: "idle",
			want: "",
		},
		{
			name: "discovery",
			progress: catalogProgressState{
				stage:     database.Discovery,
				processed: 128,
				active:    true,
			},
			want: "Scanning beatmaps: 128 directories",
		},
		{
			name: "comparison",
			progress: catalogProgressState{
				stage:  database.Comparison,
				active: true,
			},
			want: "Comparing beatmaps...",
		},
		{
			name: "import",
			progress: catalogProgressState{
				stage:     database.Import,
				processed: 12481,
				target:    48932,
				active:    true,
			},
			want: "Indexing beatmaps: 12481 / 48932",
		},
		{
			name: "cleanup",
			progress: catalogProgressState{
				stage:     database.Cleanup,
				processed: 8,
				target:    16,
				active:    true,
			},
			want: "Removing beatmaps: 8 / 16",
		},
		{
			name: "star rating",
			progress: catalogProgressState{
				stage:     database.StarRating,
				processed: 3,
				target:    12,
				active:    true,
			},
			want: "Updating star ratings: 3 / 12",
		},
		{
			name: "finished",
			progress: catalogProgressState{
				stage:  database.Finished,
				active: true,
			},
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := catalogProgressMessage(test.progress); got != test.want {
				t.Fatalf("catalogProgressMessage() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCatalogImportListenerThresholdAndStages(t *testing.T) {
	var small launcher
	smallListener := small.catalogImportListener()
	smallListener(database.Discovery, catalogProgressVisibilityThreshold-1, 0)
	smallListener(database.Import, 1, catalogProgressVisibilityThreshold-1)
	if progress := loadCatalogProgress(&small); progress.active {
		t.Fatalf("small refresh unexpectedly became visible: %#v", progress)
	}

	var large launcher
	largeListener := large.catalogImportListener()
	largeListener(database.Discovery, catalogProgressVisibilityThreshold, 0)
	if progress := loadCatalogProgress(&large); progress.stage != database.Discovery || !progress.active {
		t.Fatalf("large discovery progress = %#v, want active discovery", loadCatalogProgress(&large))
	}

	largeListener(database.Comparison, 0, 0)
	if progress := loadCatalogProgress(&large); progress.stage != database.Comparison || !progress.active {
		t.Fatalf("comparison progress = %#v, want active comparison", loadCatalogProgress(&large))
	}

	largeListener(database.Import, 12, catalogProgressVisibilityThreshold)
	progress := loadCatalogProgress(&large)
	if progress.stage != database.Import || progress.processed != 12 || progress.target != catalogProgressVisibilityThreshold || !progress.active {
		t.Fatalf("import progress = %#v, want active import at 12/%d", progress, catalogProgressVisibilityThreshold)
	}
}

func TestClearCatalogProgressPublishesIdleState(t *testing.T) {
	var l launcher
	l.catalogProgress.Store(catalogProgressState{
		stage:     database.Import,
		processed: 4,
		target:    8,
		active:    true,
	})

	l.clearCatalogProgress()
	if progress := loadCatalogProgress(&l); progress.active {
		t.Fatalf("cleared progress = %#v, want idle state", progress)
	}
}

func loadCatalogProgress(l *launcher) catalogProgressState {
	value := l.catalogProgress.Load()
	if value == nil {
		return catalogProgressState{}
	}

	return value.(catalogProgressState)
}
