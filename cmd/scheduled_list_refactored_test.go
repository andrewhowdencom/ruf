package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestDoScheduledListRefactored(t *testing.T) {
	now := time.Now().UTC()
	futureTime := now.Add(1 * time.Hour)
	farFutureTime := now.Add(2 * time.Hour)
	pastTime := now.Add(-1 * time.Hour)

	// Create a mock store and pre-populate it with scheduled calls
	store := datastore.NewMockStore()
	store.AddScheduledCall(&model.Call{
		ID:          "past-call",
		Subject:     "Past Call",
		ScheduledAt: pastTime,
		Destinations: []model.Destination{{Type: "test", To: []string{"#past"}}},
	})
	store.AddScheduledCall(&model.Call{
		ID:          "far-future-call",
		Subject:     "Far Future Call",
		ScheduledAt: farFutureTime,
		Destinations: []model.Destination{{Type: "test", To: []string{"#far-future"}}},
	})
	store.AddScheduledCall(&model.Call{
		ID:          "future-call",
		Subject:     "Future Call",
		ScheduledAt: futureTime,
		Destinations: []model.Destination{{Type: "test", To: []string{"#future"}}},
	})
	store.AddScheduledCall(&model.Call{
		ID:          "filtered-call",
		Subject:     "Filtered Call",
		ScheduledAt: futureTime,
		Destinations: []model.Destination{{Type: "email", To: []string{"test@example.com"}}},
	})

	t.Run("lists all future calls with no filter", func(t *testing.T) {
		var buf bytes.Buffer
		err := doScheduledList(store, &buf, "", "")
		assert.NoError(t, err)

		output := buf.String()

		// 1. Check that the past call is excluded
		assert.NotContains(t, output, "Past Call")

		// 2. Check that all future calls are included
		assert.Contains(t, output, "Future Call")
		assert.Contains(t, output, "Far Future Call")
		assert.Contains(t, output, "Filtered Call")

		// 3. Check the sorting order
		futureCallIndex := strings.Index(output, "Future Call")
		farFutureCallIndex := strings.Index(output, "Far Future Call")
		assert.True(t, futureCallIndex < farFutureCallIndex, "Future Call should appear before Far Future Call")
	})

	t.Run("filters by destination type", func(t *testing.T) {
		var buf bytes.Buffer
		err := doScheduledList(store, &buf, "email", "")
		assert.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Filtered Call")
		assert.NotContains(t, output, "Future Call")
		assert.NotContains(t, output, "Far Future Call")
	})

	t.Run("filters by specific destination", func(t *testing.T) {
		var buf bytes.Buffer
		err := doScheduledList(store, &buf, "", "#future")
		assert.NoError(t, err)

		output := buf.String()
		assert.NotContains(t, output, "Filtered Call")
		assert.Contains(t, output, "Future Call")
		assert.NotContains(t, output, "Far Future Call")
	})
}
