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
	beatmaps := searchEntries{newSearchEntry(first), newSearchEntry(second), newSearchEntry(other)}

	results := searchMapSets(beatmaps, "target")
	if len(results.sets) != 1 {
		t.Fatalf("search returned %d groups, want 1", len(results.sets))
	}
	entries := results.entriesForSet(0)
	if len(entries) != 2 || entries[0].entry != first || entries[1].entry != second {
		t.Fatalf("search group = %#v, want both target entries in input order", entries)
	}
}

func TestSearchMapSetsGroupsDirectoryRegardlessOfSortOrder(t *testing.T) {
	first := &database.BeatmapEntry{Dir: "set-a", File: "easy.osu", Artist: "Artist", Creator: "Mapper", Name: "Target", Difficulty: "Easy"}
	other := &database.BeatmapEntry{Dir: "set-b", File: "normal.osu", Name: "Target", Difficulty: "Normal"}
	last := &database.BeatmapEntry{Dir: "SET-A", File: "hard.osu", Name: "Target", Difficulty: "Hard"}
	beatmaps := searchEntries{newSearchEntry(first), newSearchEntry(other), newSearchEntry(last)}

	results := searchMapSets(beatmaps, "target")
	if len(results.sets) != 2 {
		t.Fatalf("search returned %d groups, want 2", len(results.sets))
	}

	firstSet := results.entriesForSet(0)
	if len(firstSet) != 2 || firstSet[0].entry != first || firstSet[1].entry != last {
		t.Fatalf("first set entries = %#v, want both case-insensitive set-a entries", firstSet)
	}
	if results.sets[0].artistCreator != "Artist // Mapper" || results.sets[0].title != "Target" {
		t.Fatalf("first set display metadata = %#v, want metadata from its first match", results.sets[0])
	}
}

func TestNewSearchEntryPrecomputesNormalizedDisplayValues(t *testing.T) {
	entry := &database.BeatmapEntry{
		Dir:        `Set-A\Subdir`,
		File:       "hard.osu",
		Name:       "A Song",
		Artist:     "An Artist",
		Creator:    "A Mapper",
		Difficulty: "Hard",
		MD5:        "ABCDEF",
	}

	got := newSearchEntry(entry)
	if got.directoryKey != "set-a/subdir" {
		t.Fatalf("directory key = %q, want normalized slashes and casing", got.directoryKey)
	}
	if got.difficultyLabel != ">   Hard" {
		t.Fatalf("difficulty label = %q, want precomputed label", got.difficultyLabel)
	}
	if got.md5 != "abcdef" {
		t.Fatalf("md5 = %q, want lowercase identity", got.md5)
	}
}

func TestSearchEntryMatchesMapIdentityByMD5OrPath(t *testing.T) {
	entry := newSearchEntry(&database.BeatmapEntry{
		Dir:  "set-a",
		File: "hard.osu",
		MD5:  "ABCDEF",
	})

	if entry.matches(mapIdentity{}) {
		t.Fatal("nil map identity unexpectedly matched a search entry")
	}
	if !entry.matches(mapIdentity{md5: "abcdef"}) {
		t.Fatal("matching md5 did not select the search entry")
	}
	if !entry.matches(mapIdentity{mapKey: "set-a/hard.osu"}) {
		t.Fatal("matching path did not select the search entry")
	}
}

func BenchmarkSearchMapSets140K(b *testing.B) {
	beatmaps := make(searchEntries, 0, 140_000)
	for i := 0; i < 140_000; i++ {
		entry := &database.BeatmapEntry{
			Dir:  "set-" + strconv.Itoa(i/4),
			File: "map-" + strconv.Itoa(i%4) + ".osu",
			Name: "Map " + strconv.Itoa(i),
		}
		beatmaps = append(beatmaps, newSearchEntry(entry))
	}

	groupCount := len(assignSearchGroupIndices(beatmaps))
	var searchScratch []searchMatch
	var groupScratch []int
	_, searchScratch, groupScratch = searchMapSetsWithScratch(beatmaps, "map", searchScratch, groupCount, groupScratch)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var results searchResults
		results, searchScratch, groupScratch = searchMapSetsWithScratch(beatmaps, "map", searchScratch, groupCount, groupScratch)
		if len(results.sets) != 35_000 {
			b.Fatal("search did not return all fixture groups")
		}
	}
}
