package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestScheduledSkipCmd(t *testing.T) {
	// Create a mock datastore for testing.
	mockStore := datastore.NewMockStore()
	datastoreNewStore = func(bool) (kv.Storer, error) {
		return mockStore, nil
	}

	// Add a scheduled call to the mock datastore.
	scheduledCall := &kv.ScheduledCall{
		Call: model.Call{
			ID:      "test-call",
			ShortID: "12345678",
			Destinations: []model.Destination{
				{
					Type: "slack",
					To:   []string{"#general"},
				},
			},
			Campaign: model.Campaign{
				ID: "test-campaign",
			},
		},
		ScheduledAt: time.Now(),
	}
	mockStore.AddScheduledCall(scheduledCall)

	// Execute the `scheduled skip` command, capturing stdout.
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"scheduled", "skip", "12345678"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	// Assert that stdout contains the correct confirmation message.
	assert.Equal(t, "call will be skipped\n", stdout.String())

	// Assert that the call was marked as skipped in the datastore.
	sent, err := mockStore.HasBeenSent("test-campaign", "test-call", "slack", "#general")
	assert.NoError(t, err)
	assert.True(t, sent)

	// Assert that the status is "skipped"
	messages, err := mockStore.ListSentMessages()
	assert.NoError(t, err)
	assert.Len(t, messages, 1)
	assert.Equal(t, kv.StatusSkipped, messages[0].Status)
}
