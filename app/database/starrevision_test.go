package database

import (
	"database/sql"
	"testing"
)

func newStarRevisionDatabase(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Error(err)
		}
	})
	_, err = database.Exec(`CREATE TABLE beatmaps (mode INTEGER, stars REAL, starsVersion INTEGER);
		CREATE TABLE info (key TEXT UNIQUE, value TEXT);
		INSERT INTO beatmaps VALUES (0, 5.25, 20260706), (0, 4.5, 20251029), (3, 6, 20260706);`)
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func TestJulyStarRevisionPreservesStarsAndRefreshesOnlyOnce(t *testing.T) {
	database := newStarRevisionDatabase(t)
	if err := refreshJulyStarRatings(database); err != nil {
		t.Fatal(err)
	}
	rows, err := database.Query("SELECT stars, starsVersion FROM beatmaps ORDER BY rowid")
	if err != nil {
		t.Fatal(err)
	}
	wantVersions := []int{0, 20251029, 20260706}
	wantStars := []float64{5.25, 4.5, 6}
	for i := range wantVersions {
		if !rows.Next() {
			t.Fatalf("missing row %d: %v", i, rows.Err())
		}
		var stars float64
		var version int
		if err := rows.Scan(&stars, &version); err != nil {
			t.Fatal(err)
		}
		if stars != wantStars[i] || version != wantVersions[i] {
			t.Fatalf("row %d = stars %g version %d", i, stars, version)
		}
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("UPDATE beatmaps SET starsVersion = 20260706 WHERE mode = 0 AND starsVersion = 0"); err != nil {
		t.Fatal(err)
	}
	if err := refreshJulyStarRatings(database); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := database.QueryRow("SELECT starsVersion FROM beatmaps WHERE rowid = 1").Scan(&version); err != nil || version != 20260706 {
		t.Fatalf("second launch version = %d, error %v", version, err)
	}
}

func TestJulyStarRevisionRollsBackIfMarkerCannotBeSaved(t *testing.T) {
	database := newStarRevisionDatabase(t)
	if _, err := database.Exec(`CREATE TRIGGER reject_revision BEFORE INSERT ON info BEGIN SELECT RAISE(ABORT, 'interrupted'); END;`); err != nil {
		t.Fatal(err)
	}
	if err := refreshJulyStarRatings(database); err == nil {
		t.Fatal("refresh unexpectedly succeeded")
	}
	var version, markers int
	if err := database.QueryRow("SELECT starsVersion FROM beatmaps WHERE rowid = 1").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow("SELECT count(*) FROM info").Scan(&markers); err != nil {
		t.Fatal(err)
	}
	if version != 20260706 || markers != 0 {
		t.Fatalf("failed refresh retained version %d and %d markers", version, markers)
	}
	if _, err := database.Exec("DROP TRIGGER reject_revision"); err != nil {
		t.Fatal(err)
	}
	if err := refreshJulyStarRatings(database); err != nil {
		t.Fatalf("retry: %v", err)
	}
}
