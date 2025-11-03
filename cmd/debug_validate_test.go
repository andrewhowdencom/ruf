package cmd

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugValidateCmd(t *testing.T) {
	// Create a temporary directory for test files
	tmpdir, err := ioutil.TempDir("", "ruf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	// Test case 1: Valid file
	validYAML := `
calls:
  - id: "test-call"
    subject: "Test Subject"
    content: "Test Content"
    destinations:
      - type: "slack"
        to: ["#general"]
    triggers:
      - scheduled_at: "2025-01-01T12:00:00Z"
`
	validFile := filepath.Join(tmpdir, "valid.yaml")
	if err := ioutil.WriteFile(validFile, []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Test case 2: Invalid YAML
	invalidYAML := `
calls:
  - id: "test-call"
    subject: "Test Subject"
    content: "Test Content"
    destinations:
      - type: "slack"
        to: ["#general"]
  foo: bar
`
	invalidYAMLFile := filepath.Join(tmpdir, "invalid.yaml")
	if err := ioutil.WriteFile(invalidYAMLFile, []byte(invalidYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Test case 3: Missing required fields
	missingFieldsYAML := `
calls:
  - id: "test-call"
    content: "Test Content"
    destinations:
      - type: "slack"
        to: ["#general"]
`
	missingFieldsFile := filepath.Join(tmpdir, "missing_fields.yaml")
	if err := ioutil.WriteFile(missingFieldsFile, []byte(missingFieldsYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Test case 4: Invalid cron expression
	invalidCronYAML := `
calls:
  - id: "test-call"
    subject: "Test Subject"
    content: "Test Content"
    destinations:
      - type: "slack"
        to: ["#general"]
    triggers:
      - cron: "invalid cron"
`
	invalidCronFile := filepath.Join(tmpdir, "invalid_cron.yaml")
	if err := ioutil.WriteFile(invalidCronFile, []byte(invalidCronYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Test case 5: Invalid destination type
	invalidDestinationYAML := `
calls:
  - id: "test-call"
    subject: "Test Subject"
    content: "Test Content"
    destinations:
      - type: "invalid"
        to: ["#general"]
    triggers:
      - scheduled_at: "2025-01-01T12:00:00Z"
`
	invalidDestinationFile := filepath.Join(tmpdir, "invalid_destination.yaml")
	if err := ioutil.WriteFile(invalidDestinationFile, []byte(invalidDestinationYAML), 0644); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name          string
		args          []string
		expectedOutput string
		expectError   bool
	}{
		{
			name:          "valid file",
			args:          []string{"validate", "file://" + validFile},
			expectedOutput: "OK\n",
			expectError:   false,
		},
		{
			name:          "invalid yaml",
			args:          []string{"validate", "file://" + invalidYAMLFile},
			expectedOutput: "",
			expectError:   true,
		},
		{
			name:          "missing required fields",
			args:          []string{"validate", "file://" + missingFieldsFile},
			expectedOutput: "",
			expectError:   false, // The sourcer will return nil, nil, so the command should not error
		},
		{
			name:          "invalid cron expression",
			args:          []string{"validate", "file://" + invalidCronFile},
			expectedOutput: "",
			expectError:   true,
		},
		{
			name:          "invalid destination type",
			args:          []string{"validate", "file://" + invalidDestinationFile},
			expectedOutput: "",
			expectError:   true,
		},
		{
			name:          "file not found",
			args:          []string{"validate", "file:///nonexistent.yaml"},
			expectedOutput: "",
			expectError:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd.SetArgs(append([]string{"debug"}, tc.args...))
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)

			err := rootCmd.Execute()
			if tc.expectError {
				if err == nil {
					t.Errorf("expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if tc.expectedOutput != "" && !strings.Contains(out.String(), tc.expectedOutput) {
				t.Errorf("expected output to contain %q, but got %q", tc.expectedOutput, out.String())
			}
		})
	}
}
