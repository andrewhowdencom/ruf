package cmd

import (
	"errors"
	"fmt"

	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/spf13/cobra"
)

// scheduledSkipCmd represents the scheduled skip command
var scheduledSkipCmd = &cobra.Command{
	Use:   "skip [call-id]",
	Short: "Skip a scheduled call.",
	Long:  `Skip a scheduled call.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := datastoreNewStore(false)
		if err != nil {
			return fmt.Errorf("failed to create a new datastore: %w", err)
		}
		defer store.Close()

		callID := args[0]
		call, err := store.GetScheduledCall(callID)
		if err != nil {
			if errors.Is(err, kv.ErrNotFound) {
				return fmt.Errorf("could not find a call with ID '%s'", callID)
			}
			return fmt.Errorf("failed to get scheduled call: %w", err)
		}

		for _, dest := range call.Destinations {
			for _, to := range dest.To {
				hasBeenSent, err := store.HasBeenSent(call.Campaign.ID, call.ID, dest.Type, to)
				if err != nil {
					return fmt.Errorf("failed to check if call has been sent: %w", err)
				}
				if hasBeenSent {
					return fmt.Errorf("call with ID '%s' has already been sent to '%s'", callID, to)
				}

				sm := &kv.SentMessage{
					ScheduledAt: call.ScheduledAt,
					Destination: to,
					Type:        dest.Type,
					Status:      kv.StatusSkipped,
				}
				if err := store.AddSentMessage(call.Campaign.ID, call.ID, sm); err != nil {
					return fmt.Errorf("failed to add skipped message to datastore: %w", err)
				}
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "call will be skipped\n")

		return nil
	},
}

func init() {
	scheduledCmd.AddCommand(scheduledSkipCmd)
}
