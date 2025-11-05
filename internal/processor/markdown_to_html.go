package processor

import (
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// MarkdownToHTMLProcessor converts a Markdown string to an HTML string.
type MarkdownToHTMLProcessor struct{}

// NewMarkdownToHTMLProcessor creates a new MarkdownToHTMLProcessor.
func NewMarkdownToHTMLProcessor() *MarkdownToHTMLProcessor {
	return &MarkdownToHTMLProcessor{}
}

// Process converts a Markdown string to an HTML string.
func (p *MarkdownToHTMLProcessor) Process(content string, _ map[string]interface{}) (string, error) {
	// create markdown parser with extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	ps := parser.NewWithExtensions(extensions)
	doc := ps.Parse([]byte(content))

	// create HTML renderer with extensions
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return string(markdown.Render(doc, renderer)), nil
}
