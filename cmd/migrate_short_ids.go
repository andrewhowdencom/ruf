package cmd

import (
	"fmt"
	"log/slog"

	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/spf13/cobra"
)

var migrateShortIDsCmd = &cobra.Command{
	Use:   "short-ids",
	Short: "Backfill short IDs for sent messages.",
	Long:  `Iterates through all sent messages in the datastore and generates a short ID for each one if it doesn't already exist.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := datastore.NewStore()
		if err != nil {
			return fmt.Errorf("failed to create datastore: %w", err)
		}
		defer store.Close()

		slog.Info("listing sent messages")
		messages, err := store.ListSentMessages()
		if err != nil {
			return fmt.Errorf("failed to list sent messages: %w", err)
		}

		slog.Info("found messages", "count", len(messages))

		var updated int
		for _, msg := range messages {
			if msg.ShortID == "" {
				msg.ShortID = kv.GenerateShortID(msg.ID)
				if err := store.UpdateSentMessage(msg); err != nil {
					slog.Error("failed to update message", "id", msg.ID, "error", err)
					continue
				}
				slog.Info("updated message", "id", msg.ID, "short_id", msg.ShortID)
				updated++
			}
		}

		slog.Info("migration complete", "updated", updated)

		return nil
	},
}

func init() {
	migrateCmd.AddCommand(migrateShortIDsCmd)
}
