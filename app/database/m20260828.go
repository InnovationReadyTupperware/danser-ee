package database

import "github.com/innovationreadytupperware/danser-ee/app/beatmap"

type M20260828 struct{}

func (m *M20260828) RequiredSections() []string {
	return nil
}

func (m *M20260828) FieldsToMigrate() []string {
	return nil
}

func (m *M20260828) GetValues(_ *beatmap.BeatMap) []any {
	return nil
}

func (m *M20260828) Date() int {
	return 20260828
}

func (m *M20260828) GetMigrationStmts() string {
	// The columns are added by the manager after inspecting the existing
	// schema. SQLite has no portable "ADD COLUMN IF NOT EXISTS" form, so
	// doing this migration one column at a time makes a retry safe if the
	// process stops between the two ALTER TABLE statements.
	return ""
}
