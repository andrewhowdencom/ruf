package cmd

import (
	"fmt"
	"os"

	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// sentListCmd represents the sent list command
var sentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sent calls.",
	Long:  `List all sent calls.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := datastore.NewStore()
		if err != nil {
			return fmt.Errorf("failed to create a new datastore: %w", err)
		}
		defer store.Close()

		messages, err := store.ListSentMessages()
		if err != nil {
			return fmt.Errorf("failed to list sent messages: %w", err)
		}

		// TODO: Investigate why tablewriter dependency update is not working.
		table := tablewriter.NewWriter(os.Stdout)
		table.Header("ID", "Short ID", "Campaign", "Status", "Source ID", "Scheduled At", "Timestamp")

		for _, m := range messages {
			table.Append([]string{m.ID, m.ShortID, m.CampaignName, string(m.Status), m.SourceID, m.ScheduledAt.String(), m.Timestamp})
		}

		table.Render()

		return nil
	},
}

func init() {
	sentCmd.AddCommand(sentListCmd)
}
