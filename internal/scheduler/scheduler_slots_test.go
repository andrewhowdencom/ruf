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
	viper.Set("slots.default", map[string][]string{
		"sunday": {"10:00", "16:00"},
		"monday": {"09:00"},
	})
	viper.Set("slots.slack.default", map[string][]string{
		"sunday": {"11:00", "17:00"},
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
					Destinations: []model.Destination{
						{Type: "email", To: []string{"test@example.com"}},
						{Type: "slack", To: []string{"#general"}},
					},
				},
			},
		},
	}

	expandedCalls := s.Expand(sources, now)
	assert.Len(t, expandedCalls, 2, "should expand to 2 calls")

	sort.Slice(expandedCalls, func(i, j int) bool {
		return expandedCalls[i].ID < expandedCalls[j].ID
	})

	// Test email destination (should use default slots)
	assert.Equal(t, "call-1:scheduled_at:2023-01-01T00:00:00Z:email:test@example.com", expandedCalls[0].ID)
	assert.Equal(t, time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC), expandedCalls[0].ScheduledAt)

	// Test slack destination (should use slack default slots)
	assert.Equal(t, "call-1:scheduled_at:2023-01-01T00:00:00Z:slack:#general", expandedCalls[1].ID)
	assert.Equal(t, time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC), expandedCalls[1].ScheduledAt)
}
