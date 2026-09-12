package launcher

import (
	"testing"
	"time"

	appUpdate "github.com/innovationreadytupperware/danser-ee/app/update"
)

func TestAboutUpdatePresentation(t *testing.T) {
	now := time.Date(2026, time.September, 12, 8, 30, 0, 0, time.UTC)
	lastAttempt := now.Add(-18 * time.Minute)
	lastSuccess := now.Add(-25 * time.Hour)

	tests := []struct {
		name          string
		snapshot      appUpdate.Snapshot
		wantTitle     string
		wantSecondary string
		wantLoading   bool
		wantAction    aboutUpdateAction
		wantText      string
	}{
		{
			name:          "unknown",
			snapshot:      appUpdate.Snapshot{Status: appUpdate.StatusUnknown},
			wantTitle:     "Not checked yet",
			wantSecondary: "No previous checks",
			wantAction:    aboutUpdateActionCheck,
			wantText:      "Check now",
		},
		{
			name: "checking",
			snapshot: appUpdate.Snapshot{
				Status:      appUpdate.StatusUpToDate,
				Checking:    true,
				LastAttempt: lastAttempt,
			},
			wantTitle:     "Checking for updates…",
			wantSecondary: "Last checked 18 minutes ago",
			wantLoading:   true,
		},
		{
			name: "up to date",
			snapshot: appUpdate.Snapshot{
				Status:      appUpdate.StatusUpToDate,
				LastAttempt: lastAttempt,
			},
			wantTitle:     "Up to date",
			wantSecondary: "Last checked 18 minutes ago",
			wantAction:    aboutUpdateActionCheck,
			wantText:      "Check now",
		},
		{
			name: "available",
			snapshot: appUpdate.Snapshot{
				Status:        appUpdate.StatusUpdateAvailable,
				LatestVersion: "2.4.0",
				ReleaseURL:    "https://example.invalid/release",
				LastAttempt:   lastAttempt,
			},
			wantTitle:     "2.4.0 is available",
			wantSecondary: "Last checked 18 minutes ago",
			wantAction:    aboutUpdateActionRelease,
			wantText:      "View release",
		},
		{
			name: "no releases",
			snapshot: appUpdate.Snapshot{
				Status:      appUpdate.StatusNoReleases,
				LastAttempt: lastAttempt,
			},
			wantTitle:     "No published releases",
			wantSecondary: "Last checked 18 minutes ago",
			wantAction:    aboutUpdateActionCheck,
			wantText:      "Try again",
		},
		{
			name: "failed with previous success",
			snapshot: appUpdate.Snapshot{
				Status:              appUpdate.StatusFailed,
				LastAttempt:         lastAttempt,
				LastSuccessfulCheck: lastSuccess,
			},
			wantTitle:     "Couldn't check for updates",
			wantSecondary: "Last successful check yesterday",
			wantAction:    aboutUpdateActionCheck,
			wantText:      "Try again",
		},
		{
			name: "development build",
			snapshot: appUpdate.Snapshot{
				Status: appUpdate.StatusDisabled,
			},
			wantTitle:     "Development build",
			wantSecondary: "Automatic update checks are disabled",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := aboutUpdatePresentationFor(test.snapshot, now, true)
			if got.title != test.wantTitle {
				t.Fatalf("title = %q, want %q", got.title, test.wantTitle)
			}
			if got.secondary != test.wantSecondary {
				t.Fatalf("secondary = %q, want %q", got.secondary, test.wantSecondary)
			}
			if got.loading != test.wantLoading {
				t.Fatalf("loading = %t, want %t", got.loading, test.wantLoading)
			}
			if got.action != test.wantAction {
				t.Fatalf("action = %v, want %v", got.action, test.wantAction)
			}
			if got.actionText != test.wantText {
				t.Fatalf("actionText = %q, want %q", got.actionText, test.wantText)
			}
		})
	}
}

func TestAboutUpdatePresentationWhenAutomaticChecksAreDisabled(t *testing.T) {
	now := time.Date(2026, time.September, 12, 8, 30, 0, 0, time.UTC)

	tests := []struct {
		name       string
		snapshot   appUpdate.Snapshot
		wantTitle  string
		wantAction aboutUpdateAction
		wantText   string
	}{
		{
			name:       "not checked yet",
			snapshot:   appUpdate.Snapshot{Status: appUpdate.StatusUnknown},
			wantTitle:  "Not checked yet",
			wantAction: aboutUpdateActionCheck,
			wantText:   "Check now",
		},
		{
			name: "cached update remains visible",
			snapshot: appUpdate.Snapshot{
				Status:        appUpdate.StatusUpdateAvailable,
				LatestVersion: "2.4.0",
				ReleaseURL:    "https://example.invalid/release",
			},
			wantTitle:  "2.4.0 is available",
			wantAction: aboutUpdateActionRelease,
			wantText:   "View release",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := aboutUpdatePresentationFor(test.snapshot, now, false)
			if got.title != test.wantTitle {
				t.Fatalf("title = %q, want %q", got.title, test.wantTitle)
			}
			if got.secondary != "Automatic update checks are disabled" {
				t.Fatalf("secondary = %q, want automatic checks disabled", got.secondary)
			}
			if got.action != test.wantAction || got.actionText != test.wantText {
				t.Fatalf("action = %v %q, want %v %q", got.action, got.actionText, test.wantAction, test.wantText)
			}
		})
	}
}

func TestAboutRelativeTime(t *testing.T) {
	now := time.Date(2026, time.September, 12, 8, 30, 0, 0, time.UTC)
	tests := []struct {
		name string
		then time.Time
		want string
	}{
		{name: "future", then: now.Add(time.Minute), want: "just now"},
		{name: "seconds", then: now.Add(-30 * time.Second), want: "just now"},
		{name: "one minute", then: now.Add(-time.Minute), want: "1 minute ago"},
		{name: "minutes", then: now.Add(-18 * time.Minute), want: "18 minutes ago"},
		{name: "one hour", then: now.Add(-time.Hour), want: "1 hour ago"},
		{name: "hours", then: now.Add(-8 * time.Hour), want: "8 hours ago"},
		{name: "yesterday", then: now.Add(-25 * time.Hour), want: "yesterday"},
		{name: "days", then: now.Add(-72 * time.Hour), want: "3 days ago"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := aboutRelativeTime(now, test.then); got != test.want {
				t.Fatalf("aboutRelativeTime() = %q, want %q", got, test.want)
			}
		})
	}
}
