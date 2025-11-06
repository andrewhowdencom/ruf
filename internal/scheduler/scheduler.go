package scheduler

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/robfig/cron/v3"
	"github.com/teambition/rrule-go"
)

// Scheduler is responsible for expanding call definitions into a flat list of concrete, scheduled calls.
type Scheduler struct{}

// New creates a new scheduler.
func New() *Scheduler {
	return &Scheduler{}
}

// Expand takes a list of sources and expands the call definitions within them
// into a flat list of concrete, scheduled calls based on their triggers.
func (s *Scheduler) Expand(sources []*sourcer.Source, now time.Time) []*model.Call {
	now = now.UTC() // Ensure 'now' is in UTC for consistent calculations.
	var expandedCalls []*model.Call

	for _, source := range sources {
		// Build an event map for the current source to allow for efficient lookups.
		eventsBySequence := make(map[string][]model.Event)
		for _, event := range source.Events {
			eventsBySequence[event.Sequence] = append(eventsBySequence[event.Sequence], event)
		}

		for _, callDef := range source.Calls {
			for _, trigger := range callDef.Triggers {
				// Handle direct schedule triggers
				if !trigger.ScheduledAt.IsZero() {
					newCall := createCallFromDefinition(callDef)
					newCall.ScheduledAt = trigger.ScheduledAt
					newCall.ID = fmt.Sprintf("%s:scheduled_at:%s", callDef.ID, trigger.ScheduledAt.Format(time.RFC3339))
					expandedCalls = append(expandedCalls, newCall)
				}

				// Handle cron triggers
				if trigger.Cron != "" {
					parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
					schedule, err := parser.Parse(trigger.Cron)
					if err != nil {
						slog.Error("failed to parse cron", "error", err, "cron", trigger.Cron)
						continue
					}
					// Check for the next run time within a recent window to catch jobs that should have just run.
					effectiveScheduledAt := schedule.Next(now.Add(-2 * time.Minute)).Truncate(time.Minute)

					newCall := createCallFromDefinition(callDef)
					newCall.ScheduledAt = effectiveScheduledAt
					newCall.ID = fmt.Sprintf("%s:cron:%s", callDef.ID, trigger.Cron)
					expandedCalls = append(expandedCalls, newCall)
				}

				// Handle RRule triggers
				if trigger.RRule != "" {
					rOption, err := rrule.StrToROption(trigger.RRule)
					if err != nil {
						slog.Error("failed to parse rrule", "error", err, "rrule", trigger.RRule)
						continue
					}

					if trigger.DStart != "" {
						parts := strings.SplitN(trigger.DStart, ":", 2)
						if len(parts) != 2 {
							slog.Error("invalid dstart format", "dstart", trigger.DStart)
							continue
						}
						tzid := strings.TrimPrefix(parts[0], "TZID=")
						loc, err := time.LoadLocation(tzid)
						if err != nil {
							slog.Error("failed to load location", "error", err, "tzid", tzid)
							continue
						}
						dtstart, err := time.ParseInLocation("20060102T150405", parts[1], loc)
						if err != nil {
							slog.Error("failed to parse dstart time", "error", err, "dstart", trigger.DStart)
							continue
						}
						rOption.Dtstart = dtstart.UTC()
					} else if !strings.Contains(trigger.RRule, "BYHOUR") {
						// If no DStart and no BYHOUR, default the time to 09:00 UTC of the current day.
						year, month, day := now.Date()
						rOption.Dtstart = time.Date(year, month, day, 9, 0, 0, 0, time.UTC)
					} else {
						// If no DStart but BYHOUR is present, or for any other case, use 'now'.
						rOption.Dtstart = now
					}

					rule, err := rrule.NewRRule(*rOption)
					if err != nil {
						slog.Error("failed to create rrule", "error", err, "rrule", trigger.RRule)
						continue
					}

					// Use UTC for the 'between' calculation to ensure occurrences are consistent.
					// Look for occurrences in the next 24 hours, with a 2-minute lookback to catch recent events.
					for _, occurrence := range rule.Between(now.Add(-2*time.Minute), now.Add(24*time.Hour), true) {
						newCall := createCallFromDefinition(callDef)
						newCall.ScheduledAt = occurrence.UTC() // Ensure scheduled time is stored as UTC.
						newCall.ID = fmt.Sprintf("%s:rrule:%s:%s", callDef.ID, trigger.RRule, occurrence.Format(time.RFC3339))
						expandedCalls = append(expandedCalls, newCall)
					}
				} else if trigger.DStart != "" {
					slog.Error("dstart specified without rrule", "dstart", trigger.DStart)
					continue
				}

				// Handle event sequence triggers
				if trigger.Sequence != "" && trigger.Delta != "" {
					if matchingEvents, ok := eventsBySequence[trigger.Sequence]; ok {
						for _, event := range matchingEvents {
							delta, err := time.ParseDuration(trigger.Delta)
							if err != nil {
								slog.Error("failed to parse delta", "error", err, "delta", trigger.Delta)
								continue
							}

							newCall := createCallFromDefinition(callDef)
							newCall.ScheduledAt = event.StartTime.Add(delta)
							newCall.Destinations = append(newCall.Destinations, event.Destinations...)
							newCall.ID = fmt.Sprintf("%s:sequence:%s:%s", callDef.ID, trigger.Sequence, event.StartTime.Format(time.RFC3339))
							expandedCalls = append(expandedCalls, newCall)
						}
					}
				}
			}
		}
	}
	return expandedCalls
}

// createCallFromDefinition creates a new call instance from a call definition,
// ensuring that mutable fields like Destinations are deep-copied.
func createCallFromDefinition(def model.Call) *model.Call {
	newCall := def // Start with a shallow copy

	// If the campaign name is empty, set a default.
	if newCall.Campaign.Name == "" {
		newCall.Campaign.Name = "announcements"
	}

	// Deep-copy the destinations slice to prevent modification of the original definition
	newCall.Destinations = make([]model.Destination, len(def.Destinations))
	copy(newCall.Destinations, def.Destinations)

	// Triggers are not needed in the expanded call, so we clear them.
	newCall.Triggers = nil

	return &newCall
}
