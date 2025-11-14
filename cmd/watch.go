package cmd

import (
	"fmt"
	"log/slog"

	"github.com/andrewhowdencom/ruf/internal/clients/email"
	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/http"
	"github.com/andrewhowdencom/ruf/internal/poller"
	"github.com/andrewhowdencom/ruf/internal/scheduler"
	"github.com/andrewhowdencom/ruf/internal/worker"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// watchCmd represents the watch command
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Run the watcher to send calls",
	Long:  `Run the watcher to send calls.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWatch()
	},
}

func runWatch() error {
	slog.Debug("running watch")

	go http.Start(viper.GetInt("watch.port"))

	store, err := datastoreNewStore(false)
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

	refreshInterval := viper.GetDuration("watch.refresh_interval")
	p := poller.New(s, refreshInterval)

	sched := scheduler.New(store)
	w, err := worker.New(store, slackClient, emailClient, p, sched, refreshInterval, viper.GetBool("dispatcher.dry_run"))
	if err != nil {
		return fmt.Errorf("failed to create worker: %w", err)
	}
	return w.Run()
}

func init() {
	dispatcherCmd.AddCommand(watchCmd)
	viper.SetDefault("watch.refresh_interval", "1h")
	viper.SetDefault("watch.port", 8080)
}
