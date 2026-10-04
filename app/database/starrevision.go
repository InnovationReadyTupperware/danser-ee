package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
)

// refreshJulyStarRatings invalidates the original July-model results once.
// Preserve stars for cache-first browsing while the normal workers replace them
func refreshJulyStarRatings(database *sql.DB) error {
	const marker = "sr260706_parity_revision"
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin July star-rating refresh: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("DatabaseManager: Failed to roll back July star-rating refresh: %v", err)
		}
	}()

	var revision string
	err = tx.QueryRow("SELECT value FROM info WHERE key = ?", marker).Scan(&revision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read July star-rating revision: %w", err)
	}
	if err == nil && revision == "1" {
		return nil
	}

	result, err := tx.Exec("UPDATE beatmaps SET starsVersion = 0 WHERE mode = 0 AND starsVersion = 20260706")
	if err != nil {
		return fmt.Errorf("mark July star ratings for recalculation: %w", err)
	}
	if _, err = tx.Exec("REPLACE INTO info (key, value) VALUES (?, '1')", marker); err != nil {
		return fmt.Errorf("save July star-rating revision: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit July star-rating refresh: %w", err)
	}
	if count, err := result.RowsAffected(); err == nil && count > 0 {
		log.Printf("DatabaseManager: Marked %d July star ratings for recalculation; cached stars remain available.", count)
	}
	return nil
}
