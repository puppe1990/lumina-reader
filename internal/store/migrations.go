package store

import (
	"database/sql"
	"embed"

	"github.com/puppe1990/amarra-cais/pkg/cais/migrate"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func applyMigrations(db *sql.DB) error {
	return migrate.Apply(db, migrationFS, "migrations")
}
