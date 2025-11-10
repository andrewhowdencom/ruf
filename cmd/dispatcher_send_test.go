package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/andrewhowdencom/ruf/internal/clients/email"
	"github.com/andrewhowdencom/ruf/internal/clients/slack"
	"github.com/andrewhowdencom/ruf/internal/datastore"
	"github.com/andrewhowdencom/ruf/internal/kv"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

type sendCmdTest struct {
	mockSlackClient *slack.MockClient
	mockEmailClient *email.MockClient
	mockStore       kv.Storer
}

func (s *sendCmdTest) setup(t *testing.T) {
	viper.Reset()

	// Create a temporary calls.yaml file
	tmpfile, err := os.CreateTemp("", "calls.yaml")
	assert.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpfile.Name()) })

	content := `
calls:
  - id: test-call
    subject: "Test Subject"
    content: "This is a **test** message."
    destinations:
      - type: slack
        to:
          - "#dummy"
    triggers:
      - scheduled_at: "2024-01-01T00:00:00Z"
`
	_, err = tmpfile.Write([]byte(content))
	assert.NoError(t, err)
	tmpfile.Close()

	// Set up viper to use the temporary file
	viper.Set("source.urls", []string{"file://" + tmpfile.Name()})
	viper.Set("datastore.type", "bbolt") // a real type to avoid errors
	viper.Set("datastore.path", "/tmp/ruf-test.db")

	// Create mock clients and datastore
	s.mockSlackClient = slack.NewMockClient()
	s.mockEmailClient = email.NewMockClient()
	s.mockStore = datastore.NewMockStore()
}

func TestSendCmdSlack(t *testing.T) {
	test := &sendCmdTest{}
	test.setup(t)

	// Inject the mock clients and datastore
	datastoreNewStore = func(readOnly bool) (kv.Storer, error) {
		return test.mockStore, nil
	}
	slackNewClient = func(token string) slack.Client {
		return test.mockSlackClient
	}

	// Redirect stdout
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	// Execute the command for Slack
	rootCmd.SetArgs([]string{"dispatcher", "send", "--id", "test-call", "--destination", "#general", "--type", "slack"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	// Assert the output
	assert.Contains(t, buf.String(), "Message sent successfully to #general")

	// Assert that the Slack client was called with the correct parameters
	assert.Equal(t, 1, len(test.mockSlackClient.PostMessageCalls()))
	assert.Equal(t, "#general", test.mockSlackClient.PostMessageCalls()[0].Destination)
	assert.Equal(t, "Test Subject", test.mockSlackClient.PostMessageCalls()[0].Subject)
	assert.Equal(t, "This is a *test* message.", test.mockSlackClient.PostMessageCalls()[0].Text)

	// Assert that the datastore was updated
	sentMessages, err := test.mockStore.ListSentMessages()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(sentMessages))
	assert.Equal(t, "test-call", sentMessages[0].SourceID)
	assert.Equal(t, kv.StatusSent, sentMessages[0].Status)
	assert.Equal(t, "slack", sentMessages[0].Type)
	assert.Equal(t, "#general", sentMessages[0].Destination)
}

func TestSendCmdEmail(t *testing.T) {
	test := &sendCmdTest{}
	test.setup(t)

	// Inject the mock clients and datastore
	datastoreNewStore = func(readOnly bool) (kv.Storer, error) {
		return test.mockStore, nil
	}
	emailNewClient = func(host string, port int, username, password, from string) email.Client {
		return test.mockEmailClient
	}

	// Redirect stdout
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	// Execute the command for Email
	rootCmd.SetArgs([]string{"dispatcher", "send", "--id", "test-call", "--destination", "test@example.com", "--type", "email"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	// Assert the output
	assert.Contains(t, buf.String(), "Message sent successfully to test@example.com")

	// Assert that the Email client was called with the correct parameters
	assert.Equal(t, 1, len(test.mockEmailClient.SendCalls()))
	assert.Equal(t, []string{"test@example.com"}, test.mockEmailClient.SendCalls()[0].To)
	assert.Equal(t, "Test Subject", test.mockEmailClient.SendCalls()[0].Subject)
	assert.Equal(t, "<p>This is a <strong>test</strong> message.</p>\n", test.mockEmailClient.SendCalls()[0].Body)

	// Assert that the datastore was updated
	sentMessages, err := test.mockStore.ListSentMessages()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(sentMessages))
	assert.Equal(t, "test-call", sentMessages[0].SourceID)
	assert.Equal(t, kv.StatusSent, sentMessages[0].Status)
	assert.Equal(t, "email", sentMessages[0].Type)
	assert.Equal(t, "test@example.com", sentMessages[0].Destination)
}
