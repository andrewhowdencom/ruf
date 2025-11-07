package cmd

import (
	"fmt"
	"time"

	"github.com/andrewhowdencom/ruf/internal/clients/email"
	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/worker"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	datastoreNewStore = datastore.NewStore
	slackNewClient    = slack.NewClient
	emailNewClient    = email.NewClient
)

// sendCmd represents the send command
var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a message from a call to a specific destination.",
	Long:  `Send a message from a call to a specific destination.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get flags
		id, _ := cmd.Flags().GetString("id")
		dest, _ := cmd.Flags().GetString("destination")
		destType, _ := cmd.Flags().GetString("type")

		s, err := buildSourcer()
		if err != nil {
			return fmt.Errorf("failed to build sourcer: %w", err)
		}
		urls := viper.GetStringSlice("source.urls")
		var selectedCall *model.Call

		for _, url := range urls {
			source, _, err := s.Source(url)
			if err != nil {
				return fmt.Errorf("could not source calls from %s: %w", url, err)
			}

			if source == nil {
				continue
			}

			for i := range source.Calls {
				if source.Calls[i].ID == id {
					selectedCall = &source.Calls[i]
					break
				}
			}
			if selectedCall != nil {
				break
			}
		}

		if selectedCall == nil {
			return fmt.Errorf("call with id '%s' not found", id)
		}

		// Replace the call's destinations with the one specified on the command line
		selectedCall.Destinations = []model.Destination{
			{
				Type: destType,
				To:   []string{dest},
			},
		}
		selectedCall.ScheduledAt = time.Now()

		store, err := datastoreNewStore()
		if err != nil {
			return fmt.Errorf("failed to create a new datastore: %w", err)
		}

		slackToken := viper.GetString("slack.app.token")
		slackClient := slackNewClient(slackToken)
		emailClient := emailNewClient(
			viper.GetString("email.host"),
			viper.GetInt("email.port"),
			viper.GetString("email.username"),
			viper.GetString("email.password"),
			viper.GetString("email.from"),
		)

		if err := worker.ProcessCall(selectedCall, store, slackClient, emailClient, viper.GetBool("dispatcher.dry_run")); err != nil {
			return fmt.Errorf("failed to process call: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Message sent successfully to %s\n", dest)
		return nil
	},
}

func init() {
	dispatcherCmd.AddCommand(sendCmd)
	sendCmd.Flags().String("id", "", "ID of the call to send")
	sendCmd.Flags().String("destination", "", "Destination to send the message to")
	sendCmd.Flags().String("type", "", "Type of the destination (e.g., slack, email)")

	sendCmd.MarkFlagRequired("id")
	sendCmd.MarkFlagRequired("destination")
	sendCmd.MarkFlagRequired("type")
}
