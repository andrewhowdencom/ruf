package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
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

	futureTime := time.Now().Add(1 * time.Hour).UTC()
	farFutureTime := time.Now().Add(2 * time.Hour).UTC()
	pastTime := time.Now().Add(-1 * time.Hour).UTC()

	mockSource := &sourcer.Source{
		Campaign: model.Campaign{
			Name: "Test Campaign",
		},
		Calls: []model.Call{
			{
				Subject: "Past Call", Content: "This should be ignored.",
				Triggers: []model.Trigger{{ScheduledAt: pastTime}},
			},
			{
				Subject: "Far Future Call", Content: "Second in time-based list.",
				Triggers: []model.Trigger{{ScheduledAt: farFutureTime}},
			},
			{
				Subject: "Event Call", Content: "First in the overall list.",
				Triggers: []model.Trigger{{Sequence: "start", Delta: "5m"}},
			},
			{
				Subject: "Future Call", Content: "First in time-based list.",
				Triggers: []model.Trigger{{ScheduledAt: futureTime}},
			},
		},
		Events: []model.Event{
			{Sequence: "start"},
		},
	}

	sourcer := &mockSourcer{source: mockSource}
	var buf bytes.Buffer

	err := doScheduledList(sourcer, &buf, "", "")
	assert.NoError(t, err)

	output := buf.String()

	// 1. Check that the past call is excluded
	assert.NotContains(t, output, "Past Call")

	// 2. Check that all other calls are included
	assert.Contains(t, output, "Event Call")
	assert.Contains(t, output, "Future Call")
	assert.Contains(t, output, "Far Future Call")
	assert.Contains(t, output, "On Event 'start'")

	// 3. Check the sorting order
	eventCallIndex := strings.Index(output, "Event Call")
	futureCallIndex := strings.Index(output, "Future Call")
	farFutureCallIndex := strings.Index(output, "Far Future Call")

	// Event call should be first
	assert.True(t, eventCallIndex < futureCallIndex, "Event Call should appear before Future Call")
	assert.True(t, eventCallIndex < farFutureCallIndex, "Event Call should appear before Far Future Call")

	// Then the time-based calls, sorted by their schedule
	assert.True(t, futureCallIndex < farFutureCallIndex, "Future Call should appear before Far Future Call")
}
