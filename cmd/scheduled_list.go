package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/scheduler"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// scheduledListCmd represents the list command
var scheduledListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scheduled calls",
	Long:  `List all upcoming scheduled calls, showing the next time they are due to run.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := buildSourcer()
		if err != nil {
			return fmt.Errorf("failed to build sourcer: %w", err)
		}

		destType, _ := cmd.Flags().GetString("type")
		destination, _ := cmd.Flags().GetString("destination")

		store, err := datastore.NewStore()
		if err != nil {
			return fmt.Errorf("failed to create store: %w", err)
		}
		defer store.Close()

		sched := scheduler.New(store)
		return doScheduledList(s, sched, cmd.OutOrStdout(), destType, destination)
	},
}

// scheduledCall is an internal struct to hold information about a call for sorting and display.
type scheduledCall struct {
	NextRun       time.Time // The next calculated run time. Zero for event-based calls.
	ScheduleDef   string    // The original definition (cron string, rrule, delta, etc.).
	Campaign      string
	Subject       string
	Content       string
	IsEvent       bool
	EventSequence string // Only for event-based calls.
	Destinations  []model.Destination
}

func doScheduledList(s sourcer.Sourcer, sched *scheduler.Scheduler, w io.Writer, destType, destination string) error {
	urls := viper.GetStringSlice("source.urls")
	if len(urls) == 0 {
		fmt.Fprintln(w, "No source URLs configured.")
		return nil
	}

	var allScheduledCalls []scheduledCall
	now := time.Now().UTC()

	var sources []*sourcer.Source
	for _, url := range urls {
		source, _, err := s.Source(url)
		if err != nil {
			return fmt.Errorf("failed to source from %s: %w", url, err)
		}
		sources = append(sources, source)
	}

	// Read the calculation window from viper config
	before, err := time.ParseDuration(viper.GetString("worker.calculation.before"))
	if err != nil {
		return fmt.Errorf("failed to parse worker.calculation.before: %w", err)
	}
	after, err := time.ParseDuration(viper.GetString("worker.calculation.after"))
	if err != nil {
		return fmt.Errorf("failed to parse worker.calculation.after: %w", err)
	}

	expandedCalls := sched.Expand(sources, now, before, after)

	for _, call := range expandedCalls {
		// If filters are provided, check if the call has a matching destination.
		if destType != "" || destination != "" {
			matchFound := false
			for _, d := range call.Destinations {
				typeMatch := destType == "" || d.Type == destType
				destMatch := destination == ""
				if !destMatch {
					for _, to := range d.To {
						if to == destination {
							destMatch = true
							break
						}
					}
				}
				if typeMatch && destMatch {
					matchFound = true
					break
				}
			}
			if !matchFound {
				continue // Skip this call if it doesn't match the filters.
			}
		}

		if call.ScheduledAt.Before(now) {
			continue
		}

		firstLine := strings.Split(call.Content, "\n")[0]

		allScheduledCalls = append(allScheduledCalls, scheduledCall{
			NextRun:      call.ScheduledAt,
			ScheduleDef:  call.ID, // Using the expanded call ID as the schedule definition
			Campaign:     call.Campaign.Name,
			Subject:      call.Subject,
			Content:      firstLine,
			IsEvent:      false, // Expanded calls are always time-based
			Destinations: call.Destinations,
		})
	}

	sortAndDisplay(allScheduledCalls, w)
	return nil
}

func sortAndDisplay(calls []scheduledCall, w io.Writer) {
	if len(calls) == 0 {
		fmt.Fprintln(w, "No scheduled calls found matching the criteria.")
		return
	}

	sort.Slice(calls, func(i, j int) bool {
		return calls[i].NextRun.Before(calls[j].NextRun)
	})

	table := tablewriter.NewWriter(w)
	table.Header("Next Run", "Schedule", "Campaign", "Subject", "Content", "Destinations")

	for _, c := range calls {
		nextRunDisplay := c.NextRun.Format(time.RFC1123)
		if c.IsEvent {
			nextRunDisplay = fmt.Sprintf("On Event '%s'", c.EventSequence)
		}

		var destStrings []string
		for _, d := range c.Destinations {
			destStrings = append(destStrings, fmt.Sprintf("%s: %s", d.Type, strings.Join(d.To, ", ")))
		}

		table.Append([]string{nextRunDisplay, c.ScheduleDef, c.Campaign, c.Subject, c.Content, strings.Join(destStrings, "\n")})
	}

	table.Render()
}

func init() {
	scheduledCmd.AddCommand(scheduledListCmd)
	scheduledListCmd.Flags().String("type", "", "Filter by destination type (e.g., 'slack', 'email')")
	scheduledListCmd.Flags().String("destination", "", "Filter by a specific destination (e.g., '#channel', 'user@example.com')")
}
