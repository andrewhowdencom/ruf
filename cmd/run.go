package cmd

import (
	"fmt"
	"log/slog"

	"github.com/andrewhowdencom/ruf/internal/clients/email"
	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/poller"
	"github.com/andrewhowdencom/ruf/internal/worker"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Perform a single run of the dispatcher",
	Long:  `Perform a single run of the dispatcher.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doRun()
	},
}

func doRun() error {
	slog.Debug("performing a single run")

	store, err := datastore.NewStore()
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	defer store.Close()

	slackToken := viper.GetString("slack.app.token")
	slackClient := slack.NewClient(slackToken)

	emailClient := email.NewClient(
		viper.GetString("email.host"),
		viper.GetInt("email.port"),
		viper.GetString("email.username"),
		viper.GetString("email.password"),
		viper.GetString("email.from"),
	)

	s, err := buildSourcer()
	if err != nil {
		return fmt.Errorf("failed to build sourcer: %w", err)
	}

	// For a single run, the refresh interval isn't used by the poller,
	// but we pass a zero value to the worker constructor.
	p := poller.New(s, 0)

	w := worker.New(store, slackClient, emailClient, p, 0)
	return w.RunOnce()
}

func init() {
	dispatcherCmd.AddCommand(runCmd)
}
