package osuapi

import (
	"net/url"
	"slices"
	"testing"
)

func TestTeamScoreRequestPreservesEndpointAndFilters(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		request, err := url.Parse(scoreRequestPath(123, legacy, TeamMode, 51, "HD", "DT"))
		if err != nil {
			t.Fatal(err)
		}
		wantPath := "beatmaps/123/solo-scores"
		wantLegacy := ""
		if legacy {
			wantPath = "beatmaps/123/scores"
			wantLegacy = "1"
		}
		query := request.Query()
		if request.Path != wantPath || query.Get("type") != "team" || query.Get("legacy_only") != wantLegacy || query.Get("limit") != "51" || !slices.Equal(query["mods[]"], []string{"HD", "DT"}) {
			t.Fatalf("legacy %t: unexpected Team request %s", legacy, request)
		}
	}
}
