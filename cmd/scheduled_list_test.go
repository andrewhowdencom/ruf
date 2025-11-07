package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/scheduler"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/andrewhowdencom/ruf/internal/datastore"
)

// mockSourcer is a mock implementation of the sourcer.Sourcer interface for testing.
type mockSourcer struct {
	source *sourcer.Source
	err    error
}

func (m *mockSourcer) Source(url string) (*sourcer.Source, string, error) {
	return m.source, "mock_state", m.err
}

func TestDoScheduledList(t *testing.T) {
	// Set up viper for the test
	viper.Set("source.urls", []string{"mock://url"})
	defer viper.Reset()

	now := time.Now().UTC()
	futureTime := now.Add(1 * time.Hour)
	farFutureTime := now.Add(2 * time.Hour)
	pastTime := now.Add(-1 * time.Hour)

	mockSource := &sourcer.Source{
		Campaign: model.Campaign{
			Name: "Test Campaign",
		},
		Calls: []model.Call{
			{
				ID: "past-call", Subject: "Past Call", Content: "This should be ignored.",
				Triggers:     []model.Trigger{{ScheduledAt: pastTime}},
				Destinations: []model.Destination{{Type: "test", To: []string{"#test"}}},
			},
			{
				ID: "far-future-call", Subject: "Far Future Call", Content: "Second in time-based list.",
				Triggers:     []model.Trigger{{ScheduledAt: farFutureTime}},
				Destinations: []model.Destination{{Type: "test", To: []string{"#test"}}},
			},
			{
				ID: "future-call", Subject: "Future Call", Content: "First in time-based list.",
				Triggers:     []model.Trigger{{ScheduledAt: futureTime}},
				Destinations: []model.Destination{{Type: "test", To: []string{"#test"}}},
			},
		},
	}

	s := &mockSourcer{source: mockSource}
	store := datastore.NewMockStore()
	sched := scheduler.New(store)
	var buf bytes.Buffer

	err := doScheduledList(s, sched, &buf, "", "")
	assert.NoError(t, err)

	output := buf.String()

	// 1. Check that the past call is excluded
	assert.NotContains(t, output, "Past Call")

	// 2. Check that all other calls are included
	assert.Contains(t, output, "Future Call")
	assert.Contains(t, output, "Far Future Call")

	// 3. Check the sorting order
	futureCallIndex := strings.Index(output, "Future Call")
	farFutureCallIndex := strings.Index(output, "Far Future Call")

	// Time-based calls should be sorted by their schedule
	assert.True(t, futureCallIndex < farFutureCallIndex, "Future Call should appear before Far Future Call")
}
