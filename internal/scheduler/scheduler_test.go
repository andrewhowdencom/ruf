package scheduler_test

import (
	"sort"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/scheduler"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/stretchr/testify/assert"
	"github.com/andrewhowdencom/ruf/internal/kv/bbolt"
	"os"
)

func TestSchedulerExpand(t *testing.T) {
	dbPath := "test.db"
	defer os.Remove(dbPath)

	store, err := bbolt.NewTestStore(dbPath)
	assert.NoError(t, err)

	s := scheduler.New(store)

	viper.Set("slots.days", map[string][]string{})

	// Set 'now' to a time before the default RRule start time (09:00) to ensure the test is valid.
	now := time.Date(2023, 1, 1, 8, 0, 0, 0, time.UTC)

	sources := []*sourcer.Source{
		{
			Calls: []model.Call{
				{
					ID: "call-1",
					Triggers: []model.Trigger{
						{ScheduledAt: now.Add(1 * time.Hour)}, // 09:00
					},
				},
				{
					ID: "call-2",
					Triggers: []model.Trigger{
						{Cron: "0 14 * * *"}, // At 14:00
					},
				},
				{
					ID: "call-3",
					Triggers: []model.Trigger{
						// This will start at 09:00 on the same day due to default Dtstart logic
						{RRule: "FREQ=DAILY;COUNT=1"},
					},
				},
				{
					ID: "call-4",
					Triggers: []model.Trigger{
						{Sequence: "event-1", Delta: "5m"},
					},
				},
			},
			Events: []model.Event{
				{
					Sequence:  "event-1",
					StartTime: now.Add(30 * time.Minute), // 08:30
				},
			},
		},
	}

	expandedCalls := s.Expand(sources, now)

	assert.Len(t, expandedCalls, 4, "should expand to 4 calls")

	// Sort calls by ID for deterministic testing, as expansion order is not guaranteed.
	sort.Slice(expandedCalls, func(i, j int) bool {
		return expandedCalls[i].ID < expandedCalls[j].ID
	})

	// Test scheduled call (call-1)
	assert.Equal(t, "call-1:scheduled_at:2023-01-01T09:00:00Z", expandedCalls[0].ID)
	assert.Equal(t, now.Add(1*time.Hour), expandedCalls[0].ScheduledAt) // 09:00

	// Test cron call (call-2)
	assert.Equal(t, "call-2:cron:0 14 * * *", expandedCalls[1].ID)
	assert.Equal(t, time.Date(2023, 1, 1, 14, 0, 0, 0, time.UTC), expandedCalls[1].ScheduledAt)

	// Test RRule call (call-3)
	assert.Contains(t, expandedCalls[2].ID, "call-3:rrule:FREQ=DAILY;COUNT=1")
	assert.Equal(t, time.Date(2023, 1, 1, 9, 0, 0, 0, time.UTC), expandedCalls[2].ScheduledAt) // Should be 09:00

	// Test event-based call (call-4)
	assert.Equal(t, "call-4:sequence:event-1:2023-01-01T08:30:00Z", expandedCalls[3].ID)
	assert.Equal(t, now.Add(35*time.Minute), expandedCalls[3].ScheduledAt) // 08:35
}
