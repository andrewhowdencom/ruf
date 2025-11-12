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

	expandedCalls := s.Expand(sources, now, 1*time.Hour, 24*time.Hour)

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
	assert.Equal(t, "call-2:cron:0 14 * * *:2023-01-01T14:00:00Z:slack:#general", expandedCalls[2].ID)
	assert.Equal(t, time.Date(2023, 1, 1, 14, 0, 0, 0, time.UTC), expandedCalls[2].ScheduledAt)
	assert.Len(t, expandedCalls[2].Destinations, 1)
	assert.Equal(t, "slack", expandedCalls[2].Destinations[0].Type)
}

func TestSchedulerExpand_Hijri(t *testing.T) {
	dbPath := "test_hijri.db"
	defer os.Remove(dbPath)

	store, err := bbolt.NewTestStore(dbPath)
	assert.NoError(t, err)

	s := scheduler.New(store)

	now := time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC) // Aug 1, 2024

	sources := []*sourcer.Source{
		{
			Calls: []model.Call{
				{
					ID: "call-hijri-1",
					Triggers: []model.Trigger{
						{
							Hijri: "1 Muharram",
							Time:  "10:00",
						},
					},
					Destinations: []model.Destination{
						{Type: "email", To: []string{"test@example.com"}},
					},
				},
				{
					ID: "call-hijri-2",
					Triggers: []model.Trigger{
						{
							Hijri: "10 Dhu al-Hijjah",
							Time:  "08:00:00Z",
						},
					},
					Destinations: []model.Destination{
						{Type: "slack", To: []string{"#general"}},
					},
				},
				{
					ID: "call-hijri-3",
					Triggers: []model.Trigger{
						{
							Hijri: "27 Rajab",
							Time:  "20:00:00+03:00",
						},
					},
					Destinations: []model.Destination{
						{Type: "slack", To: []string{"#announcements"}},
					},
				},
			},
		},
	}

	expandedCalls := s.Expand(sources, now, 1*time.Hour, 365*24*time.Hour)

	// Sort calls by ID for deterministic testing, as expansion order is not guaranteed.
	sort.Slice(expandedCalls, func(i, j int) bool {
		return expandedCalls[i].ID < expandedCalls[j].ID
	})

	assert.Len(t, expandedCalls, 3, "should expand to 3 calls")

	// Test Hijri call 1 (1 Muharram 1447 -> June 27, 2025)
	assert.Contains(t, expandedCalls[0].ID, "call-hijri-1:hijri:1 Muharram")
	assert.Equal(t, 2025, expandedCalls[0].ScheduledAt.Year())
	assert.Equal(t, time.June, expandedCalls[0].ScheduledAt.Month())
	assert.Equal(t, 27, expandedCalls[0].ScheduledAt.Day())
	assert.Equal(t, 10, expandedCalls[0].ScheduledAt.Hour())
	assert.Equal(t, 0, expandedCalls[0].ScheduledAt.Minute())
	assert.Len(t, expandedCalls[0].Destinations, 1)
	assert.Equal(t, "email", expandedCalls[0].Destinations[0].Type)

	// Test Hijri call 2 (10 Dhu al-Hijjah 1446 -> June 7, 2025)
	assert.Contains(t, expandedCalls[1].ID, "call-hijri-2:hijri:10 Dhu al-Hijjah")
	assert.Equal(t, 2025, expandedCalls[1].ScheduledAt.Year())
	assert.Equal(t, time.June, expandedCalls[1].ScheduledAt.Month())
	assert.Equal(t, 7, expandedCalls[1].ScheduledAt.Day())
	assert.Equal(t, 8, expandedCalls[1].ScheduledAt.Hour())
	assert.Equal(t, 0, expandedCalls[1].ScheduledAt.Minute())
	assert.Len(t, expandedCalls[1].Destinations, 1)
	assert.Equal(t, "slack", expandedCalls[1].Destinations[0].Type)

	// Test Hijri call 3 (27 Rajab 1446 -> 27 January 2025)
	assert.Contains(t, expandedCalls[2].ID, "call-hijri-3:hijri:27 Rajab")
	assert.Equal(t, 2025, expandedCalls[2].ScheduledAt.Year())
	assert.Equal(t, time.January, expandedCalls[2].ScheduledAt.Month())
	assert.Equal(t, 27, expandedCalls[2].ScheduledAt.Day())
	assert.Equal(t, 17, expandedCalls[2].ScheduledAt.UTC().Hour()) // 20:00+03:00 -> 17:00 UTC
	assert.Equal(t, 0, expandedCalls[2].ScheduledAt.Minute())
	assert.Len(t, expandedCalls[2].Destinations, 1)
	assert.Equal(t, "slack", expandedCalls[2].Destinations[0].Type)
}
