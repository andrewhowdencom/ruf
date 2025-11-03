package sourcer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompositeFetcher(t *testing.T) {
	// Test HTTP
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", "test-etag")
		fmt.Fprintln(w, "Hello, client")
	}))
	defer server.Close()

	fetcher := NewCompositeFetcher()
	fetcher.AddFetcher("http", NewHTTPFetcher())

	data, state, err := fetcher.Fetch(server.URL)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, client\n", string(data))
	assert.Equal(t, "test-etag", state)

	// Test File
	tmpfile, err := os.CreateTemp("", "example")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString("Hello, file")
	assert.NoError(t, err)
	tmpfile.Close()

	fetcher.AddFetcher("file", NewFileFetcher())
	fileURL := "file://" + tmpfile.Name()
	data, _, err = fetcher.Fetch(fileURL)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, file", string(data))

	// Test Unsupported Scheme
	_, _, err = fetcher.Fetch("ftp://example.com")
	assert.Error(t, err)
}

func TestYAMLParser(t *testing.T) {
	schema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"title": "Ruf Call Configuration",
		"type": "object",
		"properties": {
			"campaign": {
				"$ref": "#/definitions/Campaign"
			},
			"calls": {
				"type": "array",
				"items": {
					"$ref": "#/definitions/Call"
				}
			}
		},
		"definitions": {
			"Campaign": {
				"type": "object",
				"properties": {
					"id": { "type": "string" },
					"name": { "type": "string" }
				}
			},
			"Call": {
				"type": "object",
				"properties": {
					"id": { "type": "string" },
					"subject": { "type": "string" },
					"content": { "type": "string" },
					"destinations": { "type": "array" },
					"triggers": { "type": "array" }
				},
				"required": ["id", "content", "destinations", "triggers"]
			}
		}
	}`

	tmpDir, err := os.MkdirTemp("", "test-schema")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	schemaPath := filepath.Join(tmpDir, "schema.json")
	err = os.WriteFile(schemaPath, []byte(schema), 0644)
	assert.NoError(t, err)

	parser, err := NewYAMLParser(schemaPath)
	assert.NoError(t, err)

	// Test with campaign
	yamlWithCampaign := `
campaign:
  id: "test-campaign"
  name: "Test Campaign"
calls:
  - id: "test-call"
    subject: "Test Subject"
    content: "Test Content"
    destinations: []
    triggers: []
`
	source, err := parser.Parse("file:///test.yaml", []byte(yamlWithCampaign))
	assert.NoError(t, err)
	assert.NotNil(t, source)
	assert.Len(t, source.Calls, 1)
	assert.Equal(t, "test-campaign", source.Calls[0].Campaign.ID)
	assert.Equal(t, "Test Campaign", source.Calls[0].Campaign.Name)

	// Test without campaign
	yamlWithoutCampaign := `
calls:
  - id: "test-call"
    subject: "Test Subject"
    content: "Test Content"
    destinations: []
    triggers: []
`
	source, err = parser.Parse("file:///test.yaml", []byte(yamlWithoutCampaign))
	assert.NoError(t, err)
	assert.NotNil(t, source)
	assert.Len(t, source.Calls, 1)
	assert.Equal(t, "test", source.Calls[0].Campaign.ID)
	assert.Equal(t, "/test.yaml", source.Calls[0].Campaign.Name)

	// Test with an invalid file (missing required 'content' field)
	invalidYAML := `
calls:
  - id: "test-call"
    destinations: []
    triggers: []
`
	source, err = parser.Parse("file:///invalid.yaml", []byte(invalidYAML))
	assert.NoError(t, err)
	assert.Nil(t, source)
}
