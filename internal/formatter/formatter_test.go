package formatter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToHTML(t *testing.T) {
	tests := []struct {
		name     string
		markdown []byte
		expected []byte
	}{
		{
			name:     "headings",
			markdown: []byte("# Hello"),
			expected: []byte("<h1 id=\"hello\">Hello</h1>\n"),
		},
		{
			name:     "link",
			markdown: []byte("[link](https://example.com)"),
			expected: []byte("<p><a href=\"https://example.com\" target=\"_blank\">link</a></p>\n"),
		},
		{
			name:     "list",
			markdown: []byte("- one\n- two"),
			expected: []byte("<ul>\n<li>one</li>\n<li>two</li>\n</ul>\n"),
		},
		{
			name:     "paragraph",
			markdown: []byte("some text"),
			expected: []byte("<p>some text</p>\n"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, string(tt.expected), string(ToHTML(tt.markdown)))
		})
	}
}

func TestToSlack(t *testing.T) {
	tests := []struct {
		name     string
		markdown []byte
		expected string
		err      bool
	}{
		{
			name:     "headings",
			markdown: []byte("# Hello"),
			expected: "*Hello*",
		},
		{
			name:     "link",
			markdown: []byte("[link](https://example.com)"),
			expected: "<https://example.com|link>",
		},
		{
			name:     "list",
			markdown: []byte("- one\n- two\n- three"),
			expected: "• one\n• two\n• three",
		},
		{
			name:     "paragraph",
			markdown: []byte("some text"),
			expected: "some text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ToSlack(tt.markdown)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, actual)
			}
		})
	}
}
