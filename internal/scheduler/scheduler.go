package scheduler

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"github.com/teambition/rrule-go"
)

// Scheduler is responsible for expanding call definitions into a flat list of concrete, scheduled calls.
type Scheduler struct {
	storer kv.Storer
}

// New creates a new scheduler.
func New(storer kv.Storer) *Scheduler {
	return &Scheduler{
		storer: storer,
	}
}

// Expand takes a list of sources and expands the call definitions within them
// into a flat list of concrete, scheduled calls based on their triggers.
func (s *Scheduler) Expand(sources []*sourcer.Source, now time.Time, before, after time.Duration) []*model.Call {
	if err := s.storer.ClearAllSlots(); err != nil {
		slog.Error("failed to clear all slots", "error", err)
		return nil
	}

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
				for _, destination := range callDef.Destinations {
					// Handle direct schedule triggers
					if !trigger.ScheduledAt.IsZero() {
						newCall := createCallFromDefinition(callDef)
						newCall.ScheduledAt = trigger.ScheduledAt
						newCall.ID = fmt.Sprintf("%s:scheduled_at:%s:%s:%s", callDef.ID, trigger.ScheduledAt.Format(time.RFC3339), destination.Type, destination.To[0])
						if newCall.ScheduledAt.Hour() == 0 && newCall.ScheduledAt.Minute() == 0 && newCall.ScheduledAt.Second() == 0 {
							slot, err := s.findNextAvailableSlot(newCall, destination, newCall.ScheduledAt, now)
							if err != nil {
								slog.Error("failed to find next available slot", "error", err, "call_id", newCall.ID)
								continue
							}
							newCall.ScheduledAt = slot
						}
						newCall.Destinations = []model.Destination{destination}
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

						// Calculate occurrences within the window [now - before, now + after]
						startTime := now.Add(-before)
						endTime := now.Add(after)

						// Start checking from the beginning of the window.
						// We subtract a second to make sure that if the startTime itself is a valid
						// cron time, it is included.
						for t := schedule.Next(startTime.Add(-1 * time.Second)); !t.After(endTime); t = schedule.Next(t) {
							effectiveScheduledAt := t.Truncate(time.Minute)

							newCall := createCallFromDefinition(callDef)
							newCall.ScheduledAt = effectiveScheduledAt
							if newCall.ScheduledAt.Hour() == 0 && newCall.ScheduledAt.Minute() == 0 && newCall.ScheduledAt.Second() == 0 {
								slot, err := s.findNextAvailableSlot(newCall, destination, newCall.ScheduledAt, now)
								if err != nil {
									slog.Error("failed to find next available slot", "error", err, "call_id", newCall.ID)
									continue
								}
								newCall.ScheduledAt = slot
							}
							newCall.ID = fmt.Sprintf("%s:cron:%s:%s:%s", callDef.ID, trigger.Cron, destination.Type, destination.To[0])
							newCall.Destinations = []model.Destination{destination}
							expandedCalls = append(expandedCalls, newCall)
						}
					}

					// Handle RRule triggers
					if trigger.RRule != "" {
						rOption, err := rrule.StrToROption(trigger.RRule)
						if err != nil {
							slog.Error("failed to parse rrule", "error", err, "rrule", trigger.RRule)
							continue
						}

						if trigger.DStart != "" {
							loc := time.UTC // Default to UTC
							dateTimePart := trigger.DStart

							// Check if a timezone is specified
							if strings.Contains(trigger.DStart, ":") {
								parts := strings.SplitN(trigger.DStart, ":", 2)
								if strings.HasPrefix(parts[0], "TZID=") {
									tzid := strings.TrimPrefix(parts[0], "TZID=")
									// Attempt to load the location, but fall back to UTC on error
									if loadedLoc, err := time.LoadLocation(tzid); err == nil {
										loc = loadedLoc
									}
									dateTimePart = parts[1]
								}
							}

							// Try to parse as a full datetime first
							dtstart, err := time.ParseInLocation("20060102T150405", dateTimePart, loc)
							if err != nil {
								// If that fails, try to parse as a date-only string.
								// This will result in a time of 00:00:00 in the specified location.
								dtstart, err = time.ParseInLocation("20060102", dateTimePart, loc)
								if err != nil {
									slog.Error("failed to parse dstart as datetime or date", "error", err, "dstart", trigger.DStart)
									continue
								}
							}
							rOption.Dtstart = dtstart.UTC()
						} else {
							// If the RRule itself contains a time, use 'now' as the DTStart to ensure
							// the next occurrence is calculated correctly relative to the current time.
							if strings.Contains(trigger.RRule, "BYHOUR") || strings.Contains(trigger.RRule, "BYMINUTE") || strings.Contains(trigger.RRule, "BYSECOND") {
								rOption.Dtstart = now
							} else {
								// If no DStart and no time in the RRule, default to midnight UTC of the current day.
								year, month, day := now.Date()
								rOption.Dtstart = time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
							}
						}

						rule, err := rrule.NewRRule(*rOption)
						if err != nil {
							slog.Error("failed to create rrule", "error", err, "rrule", trigger.RRule)
							continue
						}

						// Use UTC for the 'between' calculation to ensure occurrences are consistent.
						startTime := now.Add(-before)
						endTime := now.Add(after)
						for _, occurrence := range rule.Between(startTime, endTime, true) {
							newCall := createCallFromDefinition(callDef)
							newCall.ScheduledAt = occurrence
							if newCall.ScheduledAt.Hour() == 0 && newCall.ScheduledAt.Minute() == 0 && newCall.ScheduledAt.Second() == 0 {
								slot, err := s.findNextAvailableSlot(newCall, destination, newCall.ScheduledAt, now)
								if err != nil {
									slog.Error("failed to find next available slot", "error", err, "call_id", newCall.ID)
									continue
								}
								newCall.ScheduledAt = slot
							}
							newCall.ID = fmt.Sprintf("%s:rrule:%s:%s:%s:%s", callDef.ID, trigger.RRule, occurrence.Format(time.RFC3339), destination.Type, destination.To[0])
							newCall.Destinations = []model.Destination{destination}
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
								newCall.ID = fmt.Sprintf("%s:sequence:%s:%s:%s:%s", callDef.ID, trigger.Sequence, event.StartTime.Format(time.RFC3339), destination.Type, destination.To[0])
								newCall.Destinations = []model.Destination{destination}
								expandedCalls = append(expandedCalls, newCall)
							}
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
func (s *Scheduler) findNextAvailableSlot(call *model.Call, destination model.Destination, scheduledAt time.Time, now time.Time) (time.Time, error) {
	loc, err := time.LoadLocation(viper.GetString("slots.timezone"))
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to load timezone: %w", err)
	}

	// Try to get the slots for the specific destination, then the type, then the default.
	// The destination `to` field can contain special characters that viper doesn't like, so we need to escape them.
	// We'll replace them with underscores.
	safeTo := strings.ReplaceAll(destination.To[0], ".", "_")
	safeTo = strings.ReplaceAll(safeTo, "#", "_")
	keys := []string{
		fmt.Sprintf("slots.%s.%s", destination.Type, safeTo),
		fmt.Sprintf("slots.%s.default", destination.Type),
		"slots.default",
	}
	var slotsByDay map[string][]string
	for _, key := range keys {
		if viper.IsSet(key) {
			slotsByDay = viper.GetStringMapStringSlice(key)
			break
		}
	}

	// If there are no slots defined, we can just return the scheduled time.
	if len(slotsByDay) == 0 {
		return scheduledAt, nil
	}

	// Start searching from the scheduled day
	for i := 0; i < 365; i++ { // Limit to 1 year of searching
		currentDay := scheduledAt.AddDate(0, 0, i)
		dayOfWeek := strings.ToLower(currentDay.Weekday().String())

		if slots, ok := slotsByDay[dayOfWeek]; ok {
			for _, slot := range slots {
				parts := strings.Split(slot, ":")
				if len(parts) != 2 {
					slog.Warn("invalid slot format", "slot", slot)
					continue
				}
				hour, _ := time.ParseDuration(parts[0] + "h")
				minute, _ := time.ParseDuration(parts[1] + "m")
				slotTime := time.Date(currentDay.Year(), currentDay.Month(), currentDay.Day(), 0, 0, 0, 0, loc).Add(hour).Add(minute)

				// Only consider slots in the future
				if slotTime.Before(now) {
					continue
				}

				// The key for the reservation should be unique for the destination.
				key := fmt.Sprintf("%s:%s", destination.Type, destination.To[0])
				reserved, err := s.storer.ReserveSlot(slotTime, key)
				if err != nil {
					return time.Time{}, fmt.Errorf("failed to reserve slot: %w", err)
				}
				if reserved {
					return slotTime, nil
				}
			}
		}
	}

	return time.Time{}, fmt.Errorf("no available slots found for call %s, destination %s", call.ID, destination.To[0])
}

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
