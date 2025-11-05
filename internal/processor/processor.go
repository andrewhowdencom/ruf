package processor

// Processor is the interface that all processors must implement.
type Processor interface {
	Process(content string, data map[string]interface{}) (string, error)
}

// ProcessorStack is a slice of processors that are applied in sequence.
type ProcessorStack []Processor

// Process applies all the processors in the stack to the content.
func (s ProcessorStack) Process(content string, data map[string]interface{}) (string, error) {
	var err error
	for _, p := range s {
		content, err = p.Process(content, data)
		if err != nil {
			return "", err
		}
	}
	return content, nil
}
