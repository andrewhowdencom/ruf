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
	"github.com/andrewhowdencom/ruf/internal/poller"
	"github.com/andrewhowdencom/ruf/internal/scheduler"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/spf13/viper"
)

// Worker is responsible for polling for calls and sending them.
type Worker struct {
	store             kv.Storer
	slackClient       slack.Client
	emailClient       email.Client
	poller            *poller.Poller
	scheduler         *scheduler.Scheduler
	refreshInterval   time.Duration
	sources           []*sourcer.Source
	mu                sync.RWMutex
	calculationBefore time.Duration
	calculationAfter  time.Duration
	dryRun            bool
}

// New creates a new worker.
func New(store kv.Storer, slackClient slack.Client, emailClient email.Client, poller *poller.Poller, scheduler *scheduler.Scheduler, refreshInterval time.Duration, dryRun bool) (*Worker, error) {
	before, err := time.ParseDuration(viper.GetString("worker.calculation.before"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse worker.calculation.before: %w", err)
	}
	after, err := time.ParseDuration(viper.GetString("worker.calculation.after"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse worker.calculation.after: %w", err)
	}

	return &Worker{
		store:             store,
		slackClient:       slackClient,
		emailClient:       emailClient,
		poller:            poller,
		scheduler:         scheduler,
		refreshInterval:   refreshInterval,
		calculationBefore: before,
		calculationAfter:  after,
		dryRun:            dryRun,
	}, nil
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

	calls := w.scheduler.Expand(sources, time.Now(), w.calculationBefore, w.calculationAfter)

	for _, call := range calls {
		now := time.Now().UTC()
		effectiveScheduledAt := call.ScheduledAt

		// Don't process calls scheduled for the future.
		if now.Before(effectiveScheduledAt) {
			slog.Debug("skipping call scheduled for the future", "call_id", call.ID, "effective_scheduled_at", effectiveScheduledAt)
			continue
		}

		missedLookback := viper.GetDuration("worker.missed_lookback")
		if effectiveScheduledAt.Before(now.Add(-missedLookback)) {
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
				slog.Error("failed to add sent message for missed call", "call_id", call.ID, "error", err)
			}
			continue
		}

		if err := ProcessCall(call, w.store, w.slackClient, w.emailClient, w.dryRun); err != nil {
			slog.Error("error processing call", "call_id", call.ID, "error", err)
		}
	}

	return nil
}
