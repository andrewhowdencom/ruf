package cmd

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/gorhill/cronexpr"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/teambition/rrule-go"
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

		return doScheduledList(s, cmd.OutOrStdout(), destType, destination)
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

func doScheduledList(s sourcer.Sourcer, w io.Writer, destType, destination string) error {
	urls := viper.GetStringSlice("source.urls")
	if len(urls) == 0 {
		fmt.Fprintln(w, "No source URLs configured.")
		return nil
	}

	var allScheduledCalls []scheduledCall
	now := time.Now().UTC()

	for _, url := range urls {
		source, _, err := s.Source(url) // Correctly handle the 3 return values
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error sourcing from %s: %v\n", url, err)
			continue
		}
		if source == nil { // Skip invalid or empty sources
			continue
		}

		campaignName := source.Campaign.Name
		if campaignName == "" {
			campaignName = "announcements"
		}

		for _, call := range source.Calls {
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

			for _, trigger := range call.Triggers {
				var next time.Time
				var scheduleDef, eventSequence string
				isEvent := false

				switch {
				case !trigger.ScheduledAt.IsZero():
					next = trigger.ScheduledAt.UTC()
					scheduleDef = trigger.ScheduledAt.Format(time.RFC3339)

				case trigger.Cron != "":
					expr, err := cronexpr.Parse(trigger.Cron)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error parsing Cron for call '%s': %v\n", call.Subject, err)
						continue
					}
					next = expr.Next(now)
					scheduleDef = trigger.Cron

				case trigger.RRule != "":
					r, err := rrule.StrToRRule(trigger.RRule)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error parsing RRule for call '%s': %v\n", call.Subject, err)
						continue
					}
					next = r.After(now, true)
					scheduleDef = trigger.RRule

				case trigger.Delta != "" && trigger.Sequence != "":
					isEvent = true
					scheduleDef = trigger.Delta
					eventSequence = trigger.Sequence

				default:
					continue
				}

				if !isEvent && next.Before(now) {
					continue
				}

				firstLine := strings.Split(call.Content, "\n")[0]

				allScheduledCalls = append(allScheduledCalls, scheduledCall{
					NextRun:       next,
					ScheduleDef:   scheduleDef,
					Campaign:      campaignName,
					Subject:       call.Subject,
					Content:       firstLine,
					IsEvent:       isEvent,
					EventSequence: eventSequence,
					Destinations:  call.Destinations,
				})
			}
		}
	}

	sortAndDisplay(allScheduledCalls, w)
	return nil
}

func sortAndDisplay(calls []scheduledCall, w io.Writer) {
	if len(calls) == 0 {
		fmt.Fprintln(w, "No scheduled calls found matching the criteria.")
		return
	}

	var eventCalls, timeBasedCalls []scheduledCall
	for _, c := range calls {
		if c.IsEvent {
			eventCalls = append(eventCalls, c)
		} else {
			timeBasedCalls = append(timeBasedCalls, c)
		}
	}

	sort.Slice(eventCalls, func(i, j int) bool {
		return eventCalls[i].Campaign < eventCalls[j].Campaign
	})
	sort.Slice(timeBasedCalls, func(i, j int) bool {
		return timeBasedCalls[i].NextRun.Before(timeBasedCalls[j].NextRun)
	})

	sortedCalls := append(eventCalls, timeBasedCalls...)

	table := tablewriter.NewWriter(w)
	table.Headed([]string{"Next Run", "Schedule", "Campaign", "Subject", "Content", "Destinations"})

	for _, c := range sortedCalls {
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
