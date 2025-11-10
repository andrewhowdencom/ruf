package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// scheduledListCmd represents the list command
var scheduledListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scheduled calls",
	Long:  `List all upcoming scheduled calls, showing the next time they are due to run.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		destType, _ := cmd.Flags().GetString("type")
		destination, _ := cmd.Flags().GetString("destination")

		store, err := datastore.NewStore(true)
		if err != nil {
			return fmt.Errorf("failed to create store: %w", err)
		}
		defer store.Close()

		return doScheduledList(store, cmd.OutOrStdout(), destType, destination)
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

func doScheduledList(store kv.Storer, w io.Writer, destType, destination string) error {
	var allScheduledCalls []scheduledCall
	now := time.Now().UTC()

	expandedCalls, err := store.ListScheduledCalls()
	if err != nil {
		return fmt.Errorf("failed to list scheduled calls: %w", err)
	}

	for _, pCall := range expandedCalls {
		call := pCall.Call
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

		if pCall.ScheduledAt.Before(now) {
			continue
		}

		allScheduledCalls = append(allScheduledCalls, scheduledCall{
			NextRun:      pCall.ScheduledAt,
			ScheduleDef:  call.ID, // Using the expanded call ID as the schedule definition
			Campaign:     call.Campaign.Name,
			Subject:      call.Subject,
			Content:      truncateContent(call.Content),
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

func truncateContent(content string) string {
	if len(content) <= 52 {
		return content
	}

	// Find the last space, period, or newline within the first 52 characters
	lastBreak := -1
	for i := 51; i >= 0; i-- {
		if content[i] == ' ' || content[i] == '.' || content[i] == '\n' {
			lastBreak = i
			break
		}
	}

	if lastBreak != -1 {
		return content[:lastBreak] + "..."
	}

	return content[:52] + "..."
}
