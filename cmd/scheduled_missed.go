package cmd

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/andrewhowdencom/ruf/internal/scheduler"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// scheduledMissedCmd represents the missed command
var scheduledMissedCmd = &cobra.Command{
	Use:   "missed",
	Short: "List missed scheduled calls",
	Long: `List all scheduled calls that were missed (or failed) in the past N days.

A call is considered "missed" if it was scheduled to occur in the past and
either has a "failed" status in the datastore or has no status record at all.

Example:
  # List all missed calls from the last 14 days
  ruf scheduled missed --days 14`,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := buildSourcer()
		if err != nil {
			return fmt.Errorf("failed to build sourcer: %w", err)
		}

		days, _ := cmd.Flags().GetInt("days")

		store, err := datastore.NewStore(true)
		if err != nil {
			return fmt.Errorf("failed to create store: %w", err)
		}
		defer store.Close()

		sched := scheduler.New(store)
		return doScheduledMissed(s, store, sched, cmd.OutOrStdout(), days)
	},
}

func doScheduledMissed(s sourcer.Sourcer, store kv.Storer, sched *scheduler.Scheduler, w io.Writer, days int) error {
	urls := viper.GetStringSlice("source.urls")
	if len(urls) == 0 {
		fmt.Fprintln(w, "No source URLs configured.")
		return nil
	}

	var missedCalls []scheduledCall
	now := time.Now().UTC()
	lookbackTime := now.AddDate(0, 0, -days)

	var sources []*sourcer.Source
	for _, url := range urls {
		source, _, err := s.Source(url)
		if err != nil {
			// Log the error but continue processing other sources
			fmt.Fprintf(w, "Warning: failed to source from %s: %v\n", url, err)
			continue
		}
		sources = append(sources, source)
	}

	// We pass 'now' to Expand, and a lookback duration matching the 'days' flag.
	// The `after` duration is 0 because we only care about past/missed calls.
	lookbackDuration := time.Duration(days) * 24 * time.Hour
	expandedCalls := sched.Expand(sources, now, lookbackDuration, 0)

	for _, call := range expandedCalls {
		// Filter 1: Is the call within our lookback window?
		if call.ScheduledAt.Before(lookbackTime) || call.ScheduledAt.After(now) {
			continue
		}

		// Filter 2: Check the status in the datastore.
		sentMessage, err := store.GetSentMessage(call.ID)
		if err != nil {
			// If the error is ErrNotFound, it means we have no record, so it's missed.
			if errors.Is(err, kv.ErrNotFound) {
				missedCalls = append(missedCalls, scheduledCall{
					NextRun:      call.ScheduledAt,
					Campaign:     call.Campaign.Name,
					Subject:      call.Subject,
					Destinations: call.Destinations,
					// Store the specific call ID for potential debugging
					ScheduleDef: call.ID,
				})
			} else {
				// For other errors, log it and continue.
				fmt.Fprintf(w, "Warning: failed to get status for call %s: %v\n", call.ID, err)
			}
			continue
		}

		// If we found a record, check if the status is 'failed'.
		if sentMessage.Status == "failed" {
			missedCalls = append(missedCalls, scheduledCall{
				NextRun:      call.ScheduledAt,
				Campaign:     call.Campaign.Name,
				Subject:      call.Subject,
				Destinations: call.Destinations,
				ScheduleDef:  call.ID,
			})
		}
	}

	sortAndDisplayMissed(missedCalls, w)
	return nil
}

func sortAndDisplayMissed(calls []scheduledCall, w io.Writer) {
	if len(calls) == 0 {
		fmt.Fprintln(w, "No missed scheduled calls found matching the criteria.")
		return
	}

	// Sort by most recent first
	sort.Slice(calls, func(i, j int) bool {
		return calls[i].NextRun.After(calls[j].NextRun)
	})

	table := tablewriter.NewWriter(w)
	table.Header("Scheduled At", "Campaign", "Call ID", "Destinations")

	for _, c := range calls {
		var destStrings []string
		for _, d := range c.Destinations {
			destStrings = append(destStrings, fmt.Sprintf("%s: %s", d.Type, strings.Join(d.To, ", ")))
		}

		table.Append([]string{
			c.NextRun.Format(time.RFC1123),
			c.Campaign,
			c.ScheduleDef, // Using ScheduleDef to show the unique call ID
			strings.Join(destStrings, "\n"),
		})
	}

	table.Render()
}

func init() {
	scheduledCmd.AddCommand(scheduledMissedCmd)
	scheduledMissedCmd.Flags().Int("days", 7, "The number of days to look back for missed calls")
}
