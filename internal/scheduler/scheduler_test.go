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

	viper.Set("slots.timezone", "UTC")
	viper.Set("slots.default", map[string][]string{
		"sunday": {"10:00", "16:00"},
	})

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
					Destinations: []model.Destination{
						{Type: "slack", To: []string{"#general"}},
						{Type: "email", To: []string{"test@example.com"}},
					},
				},
				{
					ID: "call-2",
					Triggers: []model.Trigger{
						{Cron: "0 14 * * *"}, // At 14:00
					},
					Destinations: []model.Destination{
						{Type: "slack", To: []string{"#general"}},
					},
				},
			},
		},
	}

	expandedCalls := s.Expand(sources, now)

	assert.Len(t, expandedCalls, 3, "should expand to 3 calls")

	// Sort calls by ID for deterministic testing, as expansion order is not guaranteed.
	sort.Slice(expandedCalls, func(i, j int) bool {
		return expandedCalls[i].ID < expandedCalls[j].ID
	})

	// Test scheduled call (call-1) for email
	assert.Equal(t, "call-1:scheduled_at:2023-01-01T09:00:00Z:email:test@example.com", expandedCalls[0].ID)
	assert.Equal(t, now.Add(1*time.Hour), expandedCalls[0].ScheduledAt) // 09:00
	assert.Len(t, expandedCalls[0].Destinations, 1)
	assert.Equal(t, "email", expandedCalls[0].Destinations[0].Type)

	// Test scheduled call (call-1) for slack
	assert.Equal(t, "call-1:scheduled_at:2023-01-01T09:00:00Z:slack:#general", expandedCalls[1].ID)
	assert.Equal(t, now.Add(1*time.Hour), expandedCalls[1].ScheduledAt) // 09:00
	assert.Len(t, expandedCalls[1].Destinations, 1)
	assert.Equal(t, "slack", expandedCalls[1].Destinations[0].Type)

	// Test cron call (call-2)
	assert.Equal(t, "call-2:cron:0 14 * * *:slack:#general", expandedCalls[2].ID)
	assert.Equal(t, time.Date(2023, 1, 1, 14, 0, 0, 0, time.UTC), expandedCalls[2].ScheduledAt)
	assert.Len(t, expandedCalls[2].Destinations, 1)
	assert.Equal(t, "slack", expandedCalls[2].Destinations[0].Type)
}
