package osuapi

import (
	"errors"
	"io"
	"net/url"
	"slices"
	"strings"
	"testing"
)

type scoreResponseBody struct {
	io.Reader
	closed bool
}

func (body *scoreResponseBody) Close() error {
	body.closed = true
	return nil
}

type failedScoreReader struct{ err error }

func (reader failedScoreReader) Read([]byte) (int, error) { return 0, reader.err }

func TestReadScoresPropagatesReadErrorsAndClosesBody(t *testing.T) {
	wantErr := errors.New("truncated score response")
	body := &scoreResponseBody{Reader: failedScoreReader{err: wantErr}}
	scores, err := readScores(body)
	if !errors.Is(err, wantErr) || scores != nil || !body.closed {
		t.Fatalf("scores=%v err=%v closed=%t; want read error and closed body", scores, err, body.closed)
	}
}

func TestReadScoresClosesBodyForValidAndInvalidJSON(t *testing.T) {
	for _, valid := range []bool{false, true} {
		payload := "{"
		if valid {
			payload = `{"scores":[{"id":123}]}`
		}
		body := &scoreResponseBody{Reader: strings.NewReader(payload)}
		scores, err := readScores(body)
		if (err == nil) != valid || !body.closed {
			t.Fatalf("valid=%t err=%v closed=%t", valid, err, body.closed)
		}
		if valid && (len(scores) != 1 || scores[0].ID != 123) {
			t.Fatalf("decoded scores = %#v", scores)
		}
	}
}

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
