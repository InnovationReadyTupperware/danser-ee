package database

import (
	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
)

type Migration interface {
	// Returns statements to update schema
	GetMigrationStmts() string

	// Returns required sections from .osu file
	RequiredSections() []string

	// Returns database fields to update
	FieldsToMigrate() []string

	// Returns beatmap values to update
	GetValues(beatMap *beatmap.BeatMap) []any

	// Returns database version on which changes were made
	Date() int
}
