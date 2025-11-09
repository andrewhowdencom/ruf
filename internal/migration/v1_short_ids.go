package migration

import (
	"log/slog"

	"github.com/andrewhowdencom/ruf/internal/kv"
)

func init() {
	Register(&ShortIDMigration{})
}

// ShortIDMigration backfills the ShortID field for all sent messages.
type ShortIDMigration struct{}

// Version returns the migration version.
func (m *ShortIDMigration) Version() int {
	return 1
}

// Description returns the migration description.
func (m *ShortIDMigration) Description() string {
	return "Backfill ShortID for sent messages"
}

// Up runs the migration.
func (m *ShortIDMigration) Up(store kv.Storer) error {
	slog.Info("listing sent messages to backfill short IDs")
	messages, err := store.ListSentMessages()
	if err != nil {
		return err
	}

	for _, msg := range messages {
		if msg.ShortID == "" {
			msg.ShortID = kv.GenerateShortID(msg.ID)
			if err := store.UpdateSentMessage(msg); err != nil {
				slog.Error("failed to update message", "id", msg.ID, "error", err)
				continue
			}
		}
	}

	return nil
}
