package cmd

import (
	"fmt"
	"log/slog"

	"github.com/andrewhowdencom/ruf/internal/clients/email"
	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/http"
	"github.com/andrewhowdencom/ruf/internal/poller"
	"github.com/andrewhowdencom/ruf/internal/worker"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// dispatcherCmd represents the dispatcher command
var dispatcherCmd = &cobra.Command{
	Use:   "dispatcher",
	Short: "Run the dispatcher to send calls",
	Long:  `Run the dispatcher to send calls.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDispatcher()
	},
}

func runDispatcher() error {
	slog.Debug("running dispatcher")

	go http.Start(viper.GetInt("dispatcher.port"))

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

	refreshInterval := viper.GetDuration("dispatcher.refresh_interval")
	p := poller.New(s, refreshInterval)

	w := worker.New(store, slackClient, emailClient, p, refreshInterval)
	return w.Run()
}

func init() {
	rootCmd.AddCommand(dispatcherCmd)
	viper.SetDefault("dispatcher.refresh_interval", "1h")
	viper.SetDefault("dispatcher.lookback_period", "24h")
	viper.SetDefault("dispatcher.port", 8080)
}
