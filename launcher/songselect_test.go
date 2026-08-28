package launcher

import (
	"strconv"
	"testing"

	"github.com/wieku/danser-go/app/database"
)

func TestSearchMapSetsFiltersAndPreservesSortedGroups(t *testing.T) {
	first := &database.BeatmapEntry{Dir: "set-a", File: "easy.osu", Artist: "Artist", Name: "Target", Difficulty: "Easy"}
	second := &database.BeatmapEntry{Dir: "set-a", File: "hard.osu", Artist: "Artist", Name: "Target", Difficulty: "Hard"}
	other := &database.BeatmapEntry{Dir: "set-b", File: "map.osu", Artist: "Other", Name: "Different", Difficulty: "Normal"}
	beatmaps := maps{newMapWithName(first), newMapWithName(second), newMapWithName(other)}

	results := searchMapSets(beatmaps, "target")
	if len(results) != 1 {
		t.Fatalf("search returned %d groups, want 1", len(results))
	}
	if len(results[0].entries) != 2 || results[0].entries[0] != first || results[0].entries[1] != second {
		t.Fatalf("search group = %#v, want both target entries in input order", results[0].entries)
	}
}

func BenchmarkSearchMapSets140K(b *testing.B) {
	beatmaps := make(maps, 0, 140_000)
	for i := 0; i < 140_000; i++ {
		entry := &database.BeatmapEntry{
			Dir:  "set-" + strconv.Itoa(i/4),
			File: "map-" + strconv.Itoa(i%4) + ".osu",
			Name: "Map " + strconv.Itoa(i),
		}
		beatmaps = append(beatmaps, newMapWithName(entry))
	}

	b.ResetTimer()
	for range b.N {
		if len(searchMapSets(beatmaps, "map")) != 35_000 {
			b.Fatal("search did not return all fixture groups")
		}
	}
}
