package processor

import (
	"bytes"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// TemplateProcessor renders a Go template string.
type TemplateProcessor struct{}

// NewTemplateProcessor creates a new TemplateProcessor.
func NewTemplateProcessor() *TemplateProcessor {
	return &TemplateProcessor{}
}

// Process renders a template string.
func (p *TemplateProcessor) Process(content string, data map[string]interface{}) (string, error) {
	t, err := template.New("").Funcs(sprig.TxtFuncMap()).Parse(content)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
