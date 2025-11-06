package scheduler_test

import (
	"os"
	"sort"
	"testing"
	"time"

	"github.com/andrewhowdencom/ruf/internal/kv/bbolt"
	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/scheduler"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestSchedulerExpandWithSlots(t *testing.T) {
	dbPath := "test.db"
	defer os.Remove(dbPath)

	store, err := bbolt.NewTestStore(dbPath)
	assert.NoError(t, err)

	s := scheduler.New(store)

	viper.Set("slots.timezone", "UTC")
	viper.Set("slots.days", map[string][]string{
		"sunday": {"10:00", "16:00"},
		"monday": {"09:00"},
	})

	now := time.Date(2023, 1, 1, 8, 0, 0, 0, time.UTC) // A Sunday

	sources := []*sourcer.Source{
		{
			Calls: []model.Call{
				{
					ID: "call-1",
					Triggers: []model.Trigger{
						{ScheduledAt: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)}, // Will be moved to the first slot
					},
				},
				{
					ID: "call-2",
					Triggers: []model.Trigger{
						{Cron: "* * * * *"}, // Will be moved to the second slot
					},
				},
				{
					ID: "call-3",
					Triggers: []model.Trigger{
						{RRule: "FREQ=DAILY;COUNT=1"}, // Will be moved to the next available slot on the next day
					},
				},
			},
		},
	}

	expandedCalls := s.Expand(sources, now)
	assert.Len(t, expandedCalls, 3, "should expand to 3 calls")

	sort.Slice(expandedCalls, func(i, j int) bool {
		return expandedCalls[i].ID < expandedCalls[j].ID
	})

	assert.Equal(t, "call-1:scheduled_at:2023-01-01T00:00:00Z", expandedCalls[0].ID)
	assert.Equal(t, time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC), expandedCalls[0].ScheduledAt)

	assert.Equal(t, "call-2:cron:* * * * *", expandedCalls[1].ID)
	assert.Equal(t, time.Date(2023, 1, 1, 16, 0, 0, 0, time.UTC), expandedCalls[1].ScheduledAt)

	assert.Contains(t, expandedCalls[2].ID, "call-3:rrule:FREQ=DAILY;COUNT=1")
	assert.Equal(t, time.Date(2023, 1, 2, 9, 0, 0, 0, time.UTC), expandedCalls[2].ScheduledAt)
}
