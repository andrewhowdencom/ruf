package worker_test

import (
	"testing"
	"time"

	"github.com/andrewhowdencom/ruf/internal/clients/email"
	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/poller"
	"github.com/andrewhowdencom/ruf/internal/scheduler"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/andrewhowdencom/ruf/internal/worker"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// mockSourcer implements the sourcer.Sourcer interface for testing.
type mockSourcer struct {
	sourcesBySource map[string]*sourcer.Source
	err             error
}

func (m *mockSourcer) Source(url string) (*sourcer.Source, string, error) {
	if m.err != nil {
		return nil, "", m.err
	}
	return m.sourcesBySource[url], "state", nil
}

func TestWorker_RunTick(t *testing.T) {
	// Mock datastore
	store := datastore.NewMockStore()

	// Mock Slack client
	slackClient := slack.NewMockClient()

	// Mock Email client
	emailClient := email.NewMockClient()

	// Mock sourcer
	s := &mockSourcer{
		sourcesBySource: map[string]*sourcer.Source{
			"mock://url": {
				Calls: []model.Call{
					{
						ID:      "1",
						Author:  "test@author.com",
						Subject: "Test Subject",
						Content: "Hello, world!",
						Destinations: []model.Destination{
							{
								Type: "slack",
								To:   []string{"test-channel"},
							},
							{
								Type: "email",
								To:   []string{"test@example.com"},
							},
						},
						Triggers: []model.Trigger{
							{
								ScheduledAt: time.Now().Add(-1 * time.Minute),
							},
						},
						Campaign: model.Campaign{
							ID:   "mock-campaign",
							Name: "Mock Campaign",
						},
					},
				},
			},
		},
	}

	p := poller.New(s, 1*time.Minute)
	viper.Set("source.urls", []string{"mock://url"})
	viper.Set("worker.missed_lookback", "10m")
	viper.Set("worker.calculation.before", "24h")
	viper.Set("worker.calculation.after", "24h")

	sched := scheduler.New(store)
	w, err := worker.New(store, slackClient, emailClient, p, sched, 1*time.Minute, false)
	assert.NoError(t, err)

	err = w.RefreshSources()
	assert.NoError(t, err)

	err = w.ProcessMessages()
	assert.NoError(t, err)

	sentMessages, err := store.ListSentMessages()
	assert.NoError(t, err)
	assert.Len(t, sentMessages, 2)

	assert.Equal(t, 1, len(slackClient.PostMessageCalls()))
	assert.Equal(t, "test@author.com", slackClient.PostMessageCalls()[0].Author)
	assert.Equal(t, 1, len(emailClient.SendCalls()))
	assert.Equal(t, "test@author.com", emailClient.SendCalls()[0].Author)
}

func TestWorker_RunTickWithOldCall(t *testing.T) {
	// Mock datastore
	store := datastore.NewMockStore()

	// Mock Slack client
	slackClient := slack.NewMockClient()

	// Mock Email client
	emailClient := email.NewMockClient()

	// Mock sourcer
	s := &mockSourcer{
		sourcesBySource: map[string]*sourcer.Source{
			"mock://url": {
				Calls: []model.Call{
					{
						ID:      "1",
						Content: "Hello, world!",
						Destinations: []model.Destination{
							{
								Type: "slack",
								To:   []string{"test-channel"},
							},
						},
						Triggers: []model.Trigger{
							{
								ScheduledAt: time.Now().Add(-48 * time.Hour),
							},
						},
						Campaign: model.Campaign{
							ID:   "mock-campaign",
							Name: "Mock Campaign",
						},
					},
				},
			},
		},
	}

	p := poller.New(s, 1*time.Minute)

	viper.Set("source.urls", []string{"mock://url"})
	viper.Set("worker.calculation.before", "24h")
	viper.Set("worker.calculation.after", "24h")

	sched := scheduler.New(store)
	w, err := worker.New(store, slackClient, emailClient, p, sched, 1*time.Minute, false)
	assert.NoError(t, err)

	err = w.RefreshSources()
	assert.NoError(t, err)
	err = w.ProcessMessages()
	assert.NoError(t, err)

	sentMessages, err := store.ListSentMessages()
	assert.NoError(t, err)
	assert.Len(t, sentMessages, 1)
	assert.Equal(t, kv.StatusFailed, sentMessages[0].Status)
	assert.Equal(t, "Mock Campaign", sentMessages[0].CampaignName)
}

func TestWorker_RunTickWithDeletedCall(t *testing.T) {
	// Mock datastore
	store := datastore.NewMockStore()

	// Mock Slack client
	slackClient := slack.NewMockClient()

	// Mock Email client
	emailClient := email.NewMockClient()

	scheduledAt := time.Now().Add(-1 * time.Minute).UTC()

	// Add a deleted message to the store
	err := store.AddSentMessage("mock-campaign", "1:scheduled_at:"+scheduledAt.Format(time.RFC3339)+":slack:test-channel", &kv.SentMessage{
		SourceID:    "1",
		ScheduledAt: scheduledAt,
		Status:      kv.StatusDeleted,
		Type:        "slack",
		Destination: "test-channel",
	})
	assert.NoError(t, err)

	// Mock sourcer
	s := &mockSourcer{
		sourcesBySource: map[string]*sourcer.Source{
			"mock://url": {
				Calls: []model.Call{
					{
						ID:      "1",
						Subject: "Test Subject",
						Content: "Hello, world!",
						Destinations: []model.Destination{
							{
								Type: "slack",
								To:   []string{"test-channel"},
							},
						},
						Triggers: []model.Trigger{
							{
								ScheduledAt: scheduledAt,
							},
						},
						Campaign: model.Campaign{
							ID:   "mock-campaign",
							Name: "Mock Campaign",
						},
					},
				},
			},
		},
	}

	p := poller.New(s, 1*time.Minute)

	viper.Set("source.urls", []string{"mock://url"})

	sched := scheduler.New(store)
	w, err := worker.New(store, slackClient, emailClient, p, sched, 1*time.Minute, false)
	assert.NoError(t, err)

	err = w.RefreshSources()
	assert.NoError(t, err)
	err = w.ProcessMessages()
	assert.NoError(t, err)

	// Check that the slack client was not called
	assert.Equal(t, 0, len(slackClient.PostMessageCalls()))
}

func TestWorker_RunTickWithEvent(t *testing.T) {
	// Mock datastore
	store := datastore.NewMockStore()

	// Mock Slack client
	slackClient := slack.NewMockClient()

	// Mock Email client
	emailClient := email.NewMockClient()

	// Mock sourcer
	s := &mockSourcer{
		sourcesBySource: map[string]*sourcer.Source{
			"mock://url": {
				Calls: []model.Call{
					{
						ID:      "1",
						Subject: "Test Subject",
						Content: "Hello, world!",
						Triggers: []model.Trigger{
							{
								Sequence: "test-sequence",
								Delta:    "5m",
							},
						},
						Destinations: []model.Destination{
							{
								Type: "slack",
								To:   []string{"test-channel"},
							},
						},
						Campaign: model.Campaign{
							ID:   "mock-campaign",
							Name: "Mock Campaign",
						},
					},
				},
				Events: []model.Event{
					{
						Sequence:  "test-sequence",
						StartTime: time.Now().Add(-10 * time.Minute),
						Destinations: []model.Destination{
							{
								Type: "email",
								To:   []string{"test@example.com"},
							},
						},
					},
				},
			},
		},
	}

	p := poller.New(s, 1*time.Minute)
	viper.Set("source.urls", []string{"mock://url"})
	viper.Set("worker.missed_lookback", "1h")
	viper.Set("worker.calculation.before", "24h")
	viper.Set("worker.calculation.after", "24h")

	sched := scheduler.New(store)
	w, err := worker.New(store, slackClient, emailClient, p, sched, 1*time.Minute, false)
	assert.NoError(t, err)

	err = w.RefreshSources()
	assert.NoError(t, err)
	err = w.ProcessMessages()
	assert.NoError(t, err)

	sentMessages, err := store.ListSentMessages()
	assert.NoError(t, err)
	assert.Len(t, sentMessages, 1)
}
