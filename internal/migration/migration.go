package migration

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/andrewhowdencom/ruf/internal/kv"
)

// Migration defines the interface for a database migration.
type Migration interface {
	Version() int
	Description() string
	Up(store kv.Storer) error
}

var migrations []Migration

// Register adds a new migration to the list of available migrations.
func Register(m Migration) {
	migrations = append(migrations, m)
}

// Apply runs all pending migrations against the datastore.
func Apply(store kv.Storer) error {
	slog.Info("applying database migrations")

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version() < migrations[j].Version()
	})

	currentVersion, err := store.GetSchemaVersion()
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	slog.Info("current database version", "version", currentVersion)

	for _, m := range migrations {
		if m.Version() > currentVersion {
			slog.Info("running migration", "version", m.Version(), "description", m.Description())
			if err := m.Up(store); err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}
			if err := store.SetSchemaVersion(m.Version()); err != nil {
				return fmt.Errorf("failed to set schema version: %w", err)
			}
			slog.Info("migration successful", "version", m.Version())
		}
	}

	slog.Info("migrations are up to date")
	return nil
}
