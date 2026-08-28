package database

import (
	"strconv"
	"testing"
)

func TestCatalogSnapshotApplyDeltaSharesUntouchedShards(t *testing.T) {
	first := &BeatmapEntry{Dir: "set-a", File: "a.osu", Name: "First"}
	second := &BeatmapEntry{Dir: "set-b", File: "b.osu", Name: "Second"}
	snapshot := NewCatalogSnapshot([]*BeatmapEntry{first, second})

	changedKey := first.MapKey()
	changedShard := catalogShardFor(changedKey)
	untouchedShard := (changedShard + 1) & (catalogShardCount - 1)
	for snapshot.shards[untouchedShard] == nil {
		untouchedShard = (untouchedShard + 1) & (catalogShardCount - 1)
		if untouchedShard == changedShard {
			t.Fatal("test entries did not occupy distinct shards")
		}
	}

	oldUntouched := snapshot.shards[untouchedShard]
	replacement := &BeatmapEntry{Dir: "set-a", File: "a.osu", Name: "Updated"}
	next := snapshot.ApplyDelta(CatalogDelta{Upserts: []*BeatmapEntry{replacement}})

	if got := next.Len(); got != 2 {
		t.Fatalf("updated snapshot length = %d, want 2", got)
	}
	if got := next.Lookup(changedKey); got != replacement {
		t.Fatalf("lookup returned %#v, want replacement entry", got)
	}
	if next.shards[untouchedShard] != oldUntouched {
		t.Fatal("publishing a delta copied an untouched shard")
	}
	if next.Generation() != snapshot.Generation()+1 {
		t.Fatalf("generation = %d, want %d", next.Generation(), snapshot.Generation()+1)
	}
}

func TestCatalogSnapshotApplyDeltaAddsAndRemovesEntries(t *testing.T) {
	first := &BeatmapEntry{Dir: "set", File: "first.osu", Name: "First"}
	second := &BeatmapEntry{Dir: "set", File: "second.osu", Name: "Second"}
	snapshot := NewCatalogSnapshot([]*BeatmapEntry{first, second})

	next := snapshot.ApplyDelta(CatalogDelta{
		Upserts:  []*BeatmapEntry{{Dir: "set", File: "third.osu", Name: "Third"}},
		Removals: []string{second.MapKey()},
	})

	if got := next.Len(); got != 2 {
		t.Fatalf("updated snapshot length = %d, want 2", got)
	}
	if next.Lookup(second.MapKey()) != nil {
		t.Fatal("removed entry is still present")
	}
	if next.Lookup(first.MapKey()) != first {
		t.Fatal("unchanged entry was lost")
	}
	if next.Lookup("set/third.osu") == nil {
		t.Fatal("added entry is missing")
	}
	if next.Lookup("SET\\THIRD.OSU") == nil {
		t.Fatal("lookup did not normalize Windows separators and casing")
	}
}

func TestNewCatalogSnapshotUsesLastDuplicateEntry(t *testing.T) {
	first := &BeatmapEntry{Dir: "set", File: "map.osu", Name: "First"}
	last := &BeatmapEntry{Dir: "SET", File: "MAP.OSU", Name: "Last"}

	snapshot := NewCatalogSnapshot([]*BeatmapEntry{first, last})
	if snapshot.Len() != 1 {
		t.Fatalf("snapshot length = %d, want 1", snapshot.Len())
	}
	if got := snapshot.Lookup("set/map.osu"); got != last {
		t.Fatalf("duplicate lookup returned %#v, want last entry", got)
	}
}

func TestBeatmapEntrySearchKeyPreservesSongSelectFields(t *testing.T) {
	entry := &BeatmapEntry{
		Artist:     "Artist",
		Name:       "Title",
		Difficulty: "Insane",
		Creator:    "Mapper",
		SetID:      123,
		ID:         456,
	}

	if got, want := entry.SearchKey(), "artist - title [insane] by mapper 123 456"; got != want {
		t.Fatalf("search key = %q, want %q", got, want)
	}
}

func BenchmarkCatalogSnapshotApplySmallDelta(b *testing.B) {
	entries := make([]*BeatmapEntry, 0, 140_000)
	for i := 0; i < 140_000; i++ {
		entries = append(entries, &BeatmapEntry{
			Dir:  "set-" + strconv.Itoa(i),
			File: "map.osu",
			Name: "Map " + strconv.Itoa(i),
		})
	}

	snapshot := NewCatalogSnapshot(entries)
	delta := CatalogDelta{Upserts: []*BeatmapEntry{{
		Dir:  "set-70000",
		File: "map.osu",
		Name: "Updated",
	}}}

	b.ResetTimer()
	for range b.N {
		next := snapshot.ApplyDelta(delta)
		if next.Len() != snapshot.Len() {
			b.Fatal("small delta changed catalog size")
		}
	}
}

func BenchmarkNewCatalogSnapshot140K(b *testing.B) {
	entries := make([]*BeatmapEntry, 0, 140_000)
	for i := 0; i < 140_000; i++ {
		entries = append(entries, &BeatmapEntry{
			Dir:  "set-" + strconv.Itoa(i),
			File: "map.osu",
			Name: "Map " + strconv.Itoa(i),
		})
	}

	b.ResetTimer()
	for range b.N {
		if NewCatalogSnapshot(entries).Len() != len(entries) {
			b.Fatal("snapshot lost entries")
		}
	}
}
