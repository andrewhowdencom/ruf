package cmd

import (
	"fmt"

	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/migration"
	"github.com/spf13/cobra"
)

var migrateDbCmd = &cobra.Command{
	Use:   "db",
	Short: "Apply all pending database migrations.",
	Long:  `Apply all pending database migrations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := datastore.NewStore(false)
		if err != nil {
			return fmt.Errorf("failed to create datastore: %w", err)
		}
		defer store.Close()

		return migration.Apply(store)
	},
}

func init() {
	migrateCmd.AddCommand(migrateDbCmd)
}
