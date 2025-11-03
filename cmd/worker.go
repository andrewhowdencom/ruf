package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/andrewhowdencom/ruf/internal/clients/email"
	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/poller"
	"github.com/andrewhowdencom/ruf/internal/worker"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// workerCmd represents the worker command
var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Run the worker to send calls",
	Long:  `Run the worker to send calls.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWorker()
	},
}

func runWorker() error {
	slog.Debug("running worker")
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

	s := buildSourcer()
	pollInterval := viper.GetDuration("worker.interval")
	if pollInterval == 0 {
		pollInterval = 1 * time.Minute
	}
	p := poller.New(s, pollInterval)

	w := worker.New(store, slackClient, emailClient, p, pollInterval)
	return w.Run()
}

func init() {
	rootCmd.AddCommand(workerCmd)
	viper.SetDefault("worker.interval", "1m")
	viper.SetDefault("worker.lookback_period", "24h")
}
