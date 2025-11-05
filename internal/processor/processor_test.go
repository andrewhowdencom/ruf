package processor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplateProcessor(t *testing.T) {
	p := NewTemplateProcessor()
	content := "Hello, {{ .Name }}"
	data := map[string]interface{}{
		"Name": "World",
	}
	processedContent, err := p.Process(content, data)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, World", processedContent)
}

func TestMarkdownToHTMLProcessor(t *testing.T) {
	p := NewMarkdownToHTMLProcessor()
	markdown := "**Hello, World!**"
	expectedHTML := "<p><strong>Hello, World!</strong></p>\n"
	processedContent, err := p.Process(markdown, nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedHTML, processedContent)
}

func TestMarkdownToSlackProcessor(t *testing.T) {
	p := NewMarkdownToSlackProcessor()
	markdown := "**Hello, World!**"
	expectedSlack := "*Hello, World!*"
	processedContent, err := p.Process(markdown, nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedSlack, processedContent)
}

func TestProcessorStack(t *testing.T) {
	stack := ProcessorStack{
		NewTemplateProcessor(),
		NewMarkdownToHTMLProcessor(),
	}
	markdown := "**Hello, {{ .Name }}!**"
	data := map[string]interface{}{
		"Name": "World",
	}
	expectedHTML := "<p><strong>Hello, World!</strong></p>\n"
	processedContent, err := stack.Process(markdown, data)
	assert.NoError(t, err)
	assert.Equal(t, expectedHTML, processedContent)
}
