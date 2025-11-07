package worker_test

import (
	"testing"
	"time"

	"github.com/andrewhowdencom/ruf/internal/clients/email"
	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/poller"
	"github.com/andrewhowdencom/ruf/internal/scheduler"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/andrewhowdencom/ruf/internal/worker"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestWorker_RunTick_MarkdownFormatting(t *testing.T) {
	store := datastore.NewMockStore()
	slackClient := slack.NewMockClient()
	emailClient := email.NewMockClient()


	markdownContent := `# Title

**bold**

_italic_

[link](http://example.com)

- one
- two
- three
`
	s := &mockSourcer{
		sourcesBySource: map[string]*sourcer.Source{
			"mock://url": {
				Calls: []model.Call{
					{
						ID:      "markdown-test",
						Author:  "test@author.com",
						Subject: "Markdown Test",
						Content: markdownContent,
						Destinations: []model.Destination{
							{
								Type: "slack",
								To:   []string{"test-channel"},
							},
							{
								Type: "email",
								To:   []string{"test@example.com"},
							},
						},
						Triggers: []model.Trigger{
							{
								ScheduledAt: time.Now().Add(-1 * time.Minute),
							},
						},
					},
				},
			},
		},
	}

	p := poller.New(s, 1*time.Minute)
	viper.Set("source.urls", []string{"mock://url"})
	viper.Set("worker.missed_lookback", "10m")
	viper.Set("worker.calculation.before", "24h")
	viper.Set("worker.calculation.after", "24h")

	sched := scheduler.New(store)
	w, err := worker.New(store, slackClient, emailClient, p, sched, 1*time.Minute, false)
	assert.NoError(t, err)

	err = w.RefreshSources()
	assert.NoError(t, err)
	err = w.ProcessMessages()
	assert.NoError(t, err)

	// Assertions for Slack mrkdwn
	assert.Equal(t, 1, len(slackClient.PostMessageCalls()))
	expectedSlackMrkdwn := "*Title*\n\n\n*bold*\n\n\n_italic_\n\n\n<http://example.com|link>\n\n\n• one\n• two\n• three"
	assert.Equal(t, expectedSlackMrkdwn, slackClient.PostMessageCalls()[0].Text)

	// Assertions for Email HTML
	assert.Equal(t, 1, len(emailClient.SendCalls()))
	expectedEmailHTML := `<h1 id="title">Title</h1>

<p><strong>bold</strong></p>

<p><em>italic</em></p>

<p><a href="http://example.com" target="_blank">link</a></p>

<ul>
<li>one</li>
<li>two</li>
<li>three</li>
</ul>
`
	assert.Equal(t, expectedEmailHTML, emailClient.SendCalls()[0].Body)
}
