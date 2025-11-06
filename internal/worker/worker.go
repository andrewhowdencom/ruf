package worker

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/andrewhowdencom/ruf/internal/clients/email"
	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/poller"
	"github.com/andrewhowdencom/ruf/internal/processor"
	"github.com/andrewhowdencom/ruf/internal/scheduler"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/spf13/viper"
)

// Worker is responsible for polling for calls and sending them.
type Worker struct {
	store           kv.Storer
	slackClient     slack.Client
	emailClient     email.Client
	poller          *poller.Poller
	scheduler       *scheduler.Scheduler
	refreshInterval time.Duration
	sources         []*sourcer.Source
	mu              sync.RWMutex
}

// New creates a new worker.
func New(store kv.Storer, slackClient slack.Client, emailClient email.Client, poller *poller.Poller, scheduler *scheduler.Scheduler, refreshInterval time.Duration) *Worker {
	return &Worker{
		store:           store,
		slackClient:     slackClient,
		emailClient:     emailClient,
		poller:          poller,
		scheduler:       scheduler,
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

	calls := w.scheduler.Expand(sources, time.Now())

	for _, call := range calls {
		if err := w.processCall(call); err != nil {
			slog.Error("error processing call", "call_id", call.ID, "error", err)
		}
	}

	return nil
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
		dest := call.Destinations[0]
		to := dest.To[0]
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
		return nil
	}

	dest := call.Destinations[0]
	if len(dest.To) == 0 {
		slog.Warn("skipping call with no address in `to`", "call_id", call.ID)
		return nil
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

	return nil
}
