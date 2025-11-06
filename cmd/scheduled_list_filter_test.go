package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/scheduler"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/andrewhowdencom/ruf/internal/datastore"
)

func TestDoScheduledListWithFilter(t *testing.T) {
	viper.Set("source.urls", []string{"mock://url"})
	defer viper.Reset()

	futureTime := time.Now().Add(1 * time.Hour).UTC()

	mockSources := []*sourcer.Source{
		{
			Campaign: model.Campaign{
				Name: "Test Campaign",
			},
			Calls: []model.Call{
				{
					ID:      "slack-call",
					Subject: "Slack Call", Content: "This should be in the output.",
					Destinations: []model.Destination{
						{Type: "slack", To: []string{"#general"}},
					},
					Triggers: []model.Trigger{{ScheduledAt: futureTime}},
				},
				{
					ID:      "email-call",
					Subject: "Email Call", Content: "This should be filtered out.",
					Destinations: []model.Destination{
						{Type: "email", To: []string{"test@example.com"}},
					},
					Triggers: []model.Trigger{{ScheduledAt: futureTime}},
				},
			},
		},
	}

	s := &mockSourcer{source: mockSources[0]}
	store := datastore.NewMockStore()
	sched := scheduler.New(store)
	var buf bytes.Buffer

	err := doScheduledList(s, sched, &buf, "slack", "")
	assert.NoError(t, err)

	output := buf.String()

	assert.Contains(t, output, "Slack Call")
	assert.NotContains(t, output, "Email Call")
}
