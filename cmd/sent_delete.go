package cmd

import (
	"errors"
	"fmt"

	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var callID string

// sentDeleteCmd represents the sent delete command
var sentDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a sent call.",
	Long:  `Delete a sent call.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := datastore.NewStore(false)
		if err != nil {
			return fmt.Errorf("failed to create a new datastore: %w", err)
		}
		defer store.Close()

		sm, err := store.GetSentMessage(callID)
		if err != nil {
			if errors.Is(err, kv.ErrNotFound) {
				return fmt.Errorf("could not find a call with ID '%s'", callID)
			}
			return fmt.Errorf("failed to get sent message: %w", err)
		}

		if sm.Type == "slack" {
			client := slack.NewClient(viper.GetString("slack.app.token"))
			if err := client.DeleteMessage(sm.Destination, sm.Timestamp); err != nil {
				return fmt.Errorf("failed to delete message from slack: %w", err)
			}
		}

		if err := store.DeleteSentMessage(callID); err != nil {
			return fmt.Errorf("failed to delete sent message from datastore: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully deleted call with ID '%s' from Slack and marked as deleted in the database.\n", callID)

		return nil
	},
}

func init() {
	sentCmd.AddCommand(sentDeleteCmd)
	sentDeleteCmd.Flags().StringVar(&callID, "call-id", "", "The ID of the call to delete.")
	sentDeleteCmd.MarkFlagRequired("call-id")
}
