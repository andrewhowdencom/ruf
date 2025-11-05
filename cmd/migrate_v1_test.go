package cmd

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrateV1Cmd(t *testing.T) {
	// Create a temporary directory for test files.
	tmpDir, err := ioutil.TempDir("", "ruf-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a legacy YAML file.
	legacyYAML := `
campaign:
  id: "my-campaign"
  name: "My Campaign"
calls:
  - id: "unique-id-1"
    author: "author@example.com"
    subject: "Hello!"
    content: "Hello, world!"
    destinations:
      - type: "slack"
        to:
          - "C1234567890"
    scheduled_at: "2025-01-01T12:00:00Z"
  - id: "unique-id-2"
    subject: "Recurring hello!"
    content: "Hello, recurring world!"
    destinations:
      - type: "slack"
        to:
          - "C1234567890"
    cron: "0 * * * *"
    recurring: true
  - id: "unique-id-3"
    subject: "Event-based hello!"
    content: "Hello, event-based world!"
    destinations:
      - type: "slack"
        to:
          - "C1234567890"
    sequence: "my-sequence"
    delta: "5m"
events:
  - sequence: "my-sequence"
    start_time: "2025-01-01T12:00:00Z"
`
	legacyFile := filepath.Join(tmpDir, "legacy.yaml")
	err = ioutil.WriteFile(legacyFile, []byte(legacyYAML), 0644)
	assert.NoError(t, err)

	// Execute the `migrate v1` command, capturing stdout and stderr.
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"migrate", "v1", legacyFile})
	err = rootCmd.Execute()
	assert.NoError(t, err)

	// Assert that stdout contains the correct migrated YAML.
	expectedYAML := `
campaign:
    id: my-campaign
    name: My Campaign
calls:
    - id: unique-id-1
      author: author@example.com
      subject: Hello!
      content: Hello, world!
      destinations:
        - type: slack
          to:
            - C1234567890
      triggers:
        - scheduled_at: 2025-01-01T12:00:00Z
    - id: unique-id-2
      subject: Recurring hello!
      content: Hello, recurring world!
      destinations:
        - type: slack
          to:
            - C1234567890
      triggers:
        - cron: 0 * * * *
    - id: unique-id-3
      subject: Event-based hello!
      content: Hello, event-based world!
      destinations:
        - type: slack
          to:
            - C1234567890
      triggers:
        - delta: 5m
          sequence: my-sequence
events:
    - sequence: my-sequence
      start_time: 2025-01-01T12:00:00Z
`
	assert.Equal(t, strings.TrimSpace(expectedYAML), strings.TrimSpace(stdout.String()))
}
