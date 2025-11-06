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
	var capturedSlackAuthor string
	slackClient.PostMessageFunc = func(channel, author, subject, text string, campaign model.Campaign) (string, string, error) {
		capturedSlackAuthor = author
		return "C1234567890", "1234567890.123456", nil
	}

	// Mock Email client
	emailClient := email.NewMockClient()
	var capturedEmailAuthor string
	emailClient.SendFunc = func(to []string, author, subject, body string, campaign model.Campaign) error {
		capturedEmailAuthor = author
		return nil
	}

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
	viper.Set("worker.lookback_period", "10m")

	w := worker.New(store, slackClient, emailClient, p, 1*time.Minute)

	err := w.RefreshSources()
	assert.NoError(t, err)

	err = w.ProcessMessages()
	assert.NoError(t, err)

	sentMessages, err := store.ListSentMessages()
	assert.NoError(t, err)
	assert.Len(t, sentMessages, 2)

	assert.Equal(t, "test@author.com", capturedSlackAuthor)
	assert.Equal(t, "test@author.com", capturedEmailAuthor)
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
	viper.Set("worker.lookback_period", "24h")

	w := worker.New(store, slackClient, emailClient, p, 1*time.Minute)

	err := w.RefreshSources()
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
	err := store.AddSentMessage("mock-campaign", "1:scheduled_at:"+scheduledAt.Format(time.RFC3339), &kv.SentMessage{
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

	w := worker.New(store, slackClient, emailClient, p, 1*time.Minute)

	err = w.RefreshSources()
	assert.NoError(t, err)
	err = w.ProcessMessages()
	assert.NoError(t, err)

	// Check that the slack client was not called
	assert.Equal(t, 0, slackClient.PostMessageCount)
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
	viper.Set("worker.lookback_period", "1h")

	w := worker.New(store, slackClient, emailClient, p, 1*time.Minute)

	err := w.RefreshSources()
	assert.NoError(t, err)
	err = w.ProcessMessages()
	assert.NoError(t, err)

	sentMessages, err := store.ListSentMessages()
	assert.NoError(t, err)
	assert.Len(t, sentMessages, 2)
}

func TestWorker_ExpandCallsWithRRule(t *testing.T) {
	w := worker.New(nil, nil, nil, nil, 0)

	sources := []*sourcer.Source{
		{
			Calls: []model.Call{
				{
					ID:      "1",
					Content: "Hello, world!",
					Triggers: []model.Trigger{
						{
							RRule: "FREQ=HOURLY;COUNT=5",
						},
					},
				},
			},
		},
	}

	// Set 'now' to a time before 09:00 to ensure the default start time is tested.
	now, _ := time.Parse(time.RFC3339, "2025-01-01T08:00:00Z")
	calls := w.ExpandCalls(sources, now)

	assert.Len(t, calls, 5)
}

func TestWorker_ExpandCallsWithRRuleAndDStart(t *testing.T) {
	w := worker.New(nil, nil, nil, nil, 0)

	// Set 'now' to a time before 09:00 to ensure the default start time is tested.
	now, _ := time.Parse(time.RFC3339, "2025-01-01T08:00:00Z")

	testCases := []struct {
		name          string
		trigger       model.Trigger
		expectedCount int
	}{
		{
			name: "valid rrule and dstart",
			trigger: model.Trigger{
				RRule:  "FREQ=DAILY;COUNT=3",
				DStart: "TZID=UTC:20250101T090000",
			},
			expectedCount: 1,
		},
		{
			name: "dstart without rrule",
			trigger: model.Trigger{
				DStart: "TZID=UTC:20250102T090000",
			},
			expectedCount: 0,
		},
		{
			name: "rrule without dstart",
			trigger: model.Trigger{
				RRule: "FREQ=HOURLY;COUNT=2",
			},
			expectedCount: 2,
		},
		{
			name: "invalid dstart format",
			trigger: model.Trigger{
				RRule:  "FREQ=DAILY;COUNT=3",
				DStart: "invalid-dstart",
			},
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sources := []*sourcer.Source{
				{
					Calls: []model.Call{
						{
							ID:       "1",
							Content:  "Hello, world!",
							Triggers: []model.Trigger{tc.trigger},
						},
					},
				},
			}

			calls := w.ExpandCalls(sources, now)
			assert.Len(t, calls, tc.expectedCount)
		})
	}
}
