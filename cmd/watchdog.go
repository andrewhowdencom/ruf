package cmd

import (
	"fmt"
	"log/slog"

	"github.com/andrewhowdencom/ruf/internal/clients/email"
	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/http"
	"github.com/andrewhowdencom/ruf/internal/poller"
	"github.com/andrewhowdencom/ruf/internal/scheduler"
	"github.com/andrewhowdencom/ruf/internal/worker"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// watchdogCmd represents the watchdog command
var watchdogCmd = &cobra.Command{
	Use:   "watchdog",
	Short: "Run the watchdog to send calls",
	Long:  `Run the watchdog to send calls.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWatchdog()
	},
}

func runWatchdog() error {
	slog.Debug("running watchdog")

	go http.Start(viper.GetInt("watchdog.port"))

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

	refreshInterval := viper.GetDuration("watchdog.refresh_interval")
	p := poller.New(s, refreshInterval)

	sched := scheduler.New(store)
	w := worker.New(store, slackClient, emailClient, p, sched, refreshInterval)
	return w.Run()
}

func init() {
	dispatcherCmd.AddCommand(watchdogCmd)
	viper.SetDefault("watchdog.refresh_interval", "1h")
	viper.SetDefault("watchdog.lookback_period", "24h")
	viper.SetDefault("watchdog.port", 8080)
}
