package cmd

import (
	"fmt"
	"time"

	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/poller"
	"github.com/andrewhowdencom/ruf/internal/scheduler"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var scheduledRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh the schedule",
	Long: `Refreshes the schedule by recalculating all call instances and storing them in the datastore.

This command will:
- Fetch all source files.
- Clear the existing schedule from the datastore.
- Expand all call definitions into individual, scheduled instances.
- Persist the new schedule to the datastore.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := datastore.NewStore(false)
		if err != nil {
			return fmt.Errorf("failed to create datastore: %w", err)
		}
		defer store.Close()

		s := scheduler.New(store)

		sourcerImpl, err := buildSourcer()
		if err != nil {
			return fmt.Errorf("failed to build sourcer: %w", err)
		}

		p := poller.New(sourcerImpl, 0)

		sources, err := p.Poll(viper.GetStringSlice("source.urls"))
		if err != nil {
			return fmt.Errorf("failed to source calls: %w", err)
		}

		before, err := time.ParseDuration(viper.GetString("worker.calculation.before"))
		if err != nil {
			return fmt.Errorf("failed to parse worker.calculation.before: %w", err)
		}
		after, err := time.ParseDuration(viper.GetString("worker.calculation.after"))
		if err != nil {
			return fmt.Errorf("failed to parse worker.calculation.after: %w", err)
		}

		if err := s.RefreshSchedule(sources, time.Now(), before, after); err != nil {
			return fmt.Errorf("failed to refresh schedule: %w", err)
		}

		fmt.Println("Schedule refreshed successfully")

		return nil
	},
}

func init() {
	scheduledCmd.AddCommand(scheduledRefreshCmd)
}
