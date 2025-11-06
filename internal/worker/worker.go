package worker

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/andrewhowdencom/ruf/internal/clients/email"
	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/poller"
	"github.com/andrewhowdencom/ruf/internal/processor"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"github.com/teambition/rrule-go"
)

// Worker is responsible for polling for calls and sending them.
type Worker struct {
	store           kv.Storer
	slackClient     slack.Client
	emailClient     email.Client
	poller          *poller.Poller
	refreshInterval time.Duration
	sources         []*sourcer.Source
	mu              sync.RWMutex
}

// New creates a new worker.
func New(store kv.Storer, slackClient slack.Client, emailClient email.Client, poller *poller.Poller, refreshInterval time.Duration) *Worker {
	return &Worker{
		store:           store,
		slackClient:     slackClient,
		emailClient:     emailClient,
		poller:          poller,
		refreshInterval: refreshInterval,
	}
}

// RunOnce performs a single poll for calls and sends them.
func (w *Worker) RunOnce() error {
	if err := w.RefreshSources(); err != nil {
		return fmt.Errorf("failed to refresh sources: %w", err)
	}
	if err := w.ProcessMessages(); err != nil {
		return fmt.Errorf("failed to process messages: %w", err)
	}
	return nil
}

// Run starts the worker.
func (w *Worker) Run() error {
	slog.Info("starting worker")

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP)

	refreshTicker := time.NewTicker(w.refreshInterval)
	defer refreshTicker.Stop()

	messageTicker := time.NewTicker(1 * time.Minute)
	defer messageTicker.Stop()

	// Run a poll on startup
	if err := w.RefreshSources(); err != nil {
		slog.Error("error running initial source refresh", "error", err)
	}
	if err := w.ProcessMessages(); err != nil {
		slog.Error("error running initial message processing", "error", err)
	}

	for {
		select {
		case <-refreshTicker.C:
			if err := w.RefreshSources(); err != nil {
				slog.Error("error running source refresh", "error", err)
			}
		case <-messageTicker.C:
			if err := w.ProcessMessages(); err != nil {
				slog.Error("error running message processing", "error", err)
			}
		case <-signals:
			slog.Info("SIGHUP received, running poller")
			refreshTicker.Reset(w.refreshInterval)
			if err := w.RefreshSources(); err != nil {
				slog.Error("error running source refresh", "error", err)
			}
		}
	}
}

// RefreshSources performs a poll for sources
func (w *Worker) RefreshSources() error {
	slog.Debug("refreshing sources")
	urls := viper.GetStringSlice("source.urls")
	slog.Debug("polling for calls", "urls", urls)
	sources, err := w.poller.Poll(urls)
	if err != nil {
		return err
	}

	w.mu.Lock()
	w.sources = sources
	w.mu.Unlock()

	return nil
}

// ProcessMessages performs a single poll for calls and sends them.
func (w *Worker) ProcessMessages() error {
	w.mu.RLock()
	sources := w.sources
	w.mu.RUnlock()

	calls := w.ExpandCalls(sources, time.Now())

	for _, call := range calls {
		if err := w.processCall(call); err != nil {
			slog.Error("error processing call", "call_id", call.ID, "error", err)
		}
	}

	return nil
}

// ExpandCalls takes a list of sources and expands the call definitions within them
// into a flat list of concrete, scheduled calls based on their triggers.
func (w *Worker) ExpandCalls(sources []*sourcer.Source, now time.Time) []*model.Call {
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
					newCall := w.createCallFromDefinition(callDef)
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
					effectiveScheduledAt := schedule.Next(now.Add(-2 * time.Minute)).Truncate(time.Minute)

					newCall := w.createCallFromDefinition(callDef)
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
					for _, occurrence := range rule.Between(now, now.Add(24*time.Hour), true) {
						newCall := w.createCallFromDefinition(callDef)
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

							newCall := w.createCallFromDefinition(callDef)
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
func (w *Worker) createCallFromDefinition(def model.Call) *model.Call {
	newCall := def // Start with a shallow copy

	// If the campaign name is empty, set a default.
	if newCall.Campaign.Name == "" {
		newCall.Campaign.Name = "announcements"
	}

	newCall.Destinations = make([]model.Destination, len(def.Destinations))
	copy(newCall.Destinations, def.Destinations)
	newCall.Triggers = nil // Triggers are not needed in the expanded call
	return &newCall
}

func (w *Worker) processCall(call *model.Call) error {
	slog.Debug("processing call", "call_id", call.ID)
	now := time.Now().UTC()
	effectiveScheduledAt := call.ScheduledAt

	// Don't process calls scheduled for the future.
	if now.Before(effectiveScheduledAt) {
		slog.Debug("skipping call scheduled for the future", "call_id", call.ID, "effective_scheduled_at", effectiveScheduledAt)
		return nil
	}

	lookbackPeriod := viper.GetDuration("worker.lookback_period")
	if effectiveScheduledAt.Before(now.Add(-lookbackPeriod)) {
		slog.Warn("skipping call outside lookback period", "call_id", call.ID, "scheduled_at", effectiveScheduledAt)
		for _, dest := range call.Destinations {
			for _, to := range dest.To {
				err := w.store.AddSentMessage(call.Campaign.ID, call.ID, &kv.SentMessage{
					SourceID:     call.ID,
					ScheduledAt:  effectiveScheduledAt,
					Status:       kv.StatusFailed,
					Type:         dest.Type,
					Destination:  to,
					CampaignName: call.Campaign.Name,
				})
				if err != nil {
					return fmt.Errorf("failed to add sent message: %w", err)
				}
			}
		}
		return nil
	}

	for _, dest := range call.Destinations {
		if len(dest.To) == 0 {
			slog.Warn("skipping call with no address in `to`", "call_id", call.ID)
			continue
		}

		for _, to := range dest.To {
			hasBeenSent, err := w.store.HasBeenSent(call.Campaign.ID, call.ID, dest.Type, to)
			if err != nil {
				return fmt.Errorf("failed to check if call has been sent: %w", err)
			}
			if hasBeenSent {
				slog.Debug("skipping call that has already been sent", "call_id", call.ID, "destination", to, "type", dest.Type)
				continue
			}

			// Define the processor stacks for each destination type
			var subjectProcessor, contentProcessor processor.ProcessorStack
			switch dest.Type {
			case "slack":
				subjectProcessor = processor.ProcessorStack{
					processor.NewTemplateProcessor(),
				}
				contentProcessor = processor.ProcessorStack{
					processor.NewTemplateProcessor(),
					processor.NewMarkdownToSlackProcessor(),
				}
			case "email":
				subjectProcessor = processor.ProcessorStack{
					processor.NewTemplateProcessor(),
				}
				contentProcessor = processor.ProcessorStack{
					processor.NewTemplateProcessor(),
					processor.NewMarkdownToHTMLProcessor(),
				}
			default:
				return fmt.Errorf("unsupported destination type: %s", dest.Type)
			}

			subject, err := subjectProcessor.Process(call.Subject, nil)
			if err != nil {
				slog.Error("failed to process subject", "error", err)
				w.store.AddSentMessage(call.Campaign.ID, call.ID, &kv.SentMessage{
					SourceID:     call.ID,
					ScheduledAt:  effectiveScheduledAt,
					Status:       kv.StatusFailed,
					Type:         dest.Type,
					Destination:  to,
					CampaignName: call.Campaign.Name,
				})
				continue
			}
			content, err := contentProcessor.Process(call.Content, nil)
			if err != nil {
				slog.Error("failed to process content", "error", err)
				w.store.AddSentMessage(call.Campaign.ID, call.ID, &kv.SentMessage{
					SourceID:     call.ID,
					ScheduledAt:  effectiveScheduledAt,
					Status:       kv.StatusFailed,
					Type:         dest.Type,
					Destination:  to,
					CampaignName: call.Campaign.Name,
				})
				continue
			}

			switch dest.Type {
			case "slack":
				slog.Info("sending slack message", "call_id", call.ID, "destination", to, "scheduled_at", effectiveScheduledAt)
				channelID, timestamp, err := w.slackClient.PostMessage(to, call.Author, subject, content, call.Campaign)
				sentMessage := &kv.SentMessage{
					SourceID:     call.ID,
					ScheduledAt:  effectiveScheduledAt,
					Timestamp:    timestamp,
					Destination:  to,
					Type:         dest.Type,
					CampaignName: call.Campaign.Name,
				}

				if err != nil {
					sentMessage.Status = kv.StatusFailed
					slog.Error("failed to send slack message", "error", err)
				} else {
					sentMessage.Status = kv.StatusSent
					slog.Info("sent slack message", "call_id", call.ID, "destination", to, "scheduled_at", effectiveScheduledAt)

					if call.Author != "" {
						err := w.slackClient.NotifyAuthor(call.Author, channelID, timestamp, to)
						if err != nil {
							slog.Error("failed to send author notification", "error", err)
						}
					}
				}

				if err := w.store.AddSentMessage(call.Campaign.ID, call.ID, sentMessage); err != nil {
					return err
				}
			case "email":
				slog.Info("sending email", "call_id", call.ID, "recipient", to, "scheduled_at", effectiveScheduledAt)
				err := w.emailClient.Send([]string{to}, call.Author, subject, content, call.Campaign)
				sentMessage := &kv.SentMessage{
					SourceID:     call.ID,
					ScheduledAt:  effectiveScheduledAt,
					Destination:  to,
					Type:         dest.Type,
					CampaignName: call.Campaign.Name,
				}

				if err != nil {
					sentMessage.Status = kv.StatusFailed
					slog.Error("failed to send email", "error", err)
				} else {
					sentMessage.Status = kv.StatusSent
					slog.Info("sent email", "call_id", call.ID, "recipient", to, "scheduled_at", effectiveScheduledAt)
				}

				if err := w.store.AddSentMessage(call.Campaign.ID, call.ID, sentMessage); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported destination type: %s", dest.Type)
			}
		}
	}

	return nil
}
