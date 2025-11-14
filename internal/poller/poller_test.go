package poller

import (
	"errors"
	"testing"
	"time"

	"github.com/andrewhowdencom/ruf/internal/sourcer"
)

// mockSourcer is a mock implementation of the sourcer.Sourcer interface for testing.
type mockSourcer struct {
	sources map[string]*sourcer.Source
	states  map[string]string
	err     error
}

func (m *mockSourcer) Source(url string) (*sourcer.Source, string, error) {
	if m.err != nil {
		return nil, "", m.err
	}
	source, ok := m.sources[url]
	if !ok {
		return nil, "", errors.New("source not found")
	}
	state, ok := m.states[url]
	if !ok {
		return nil, "", errors.New("state not found")
	}
	return source, state, nil
}

func TestPoller_Poll_AllSourcesFail(t *testing.T) {
	// Arrange
	mockSourcer := &mockSourcer{
		err: errors.New("failed to fetch source"),
	}
	poller := New(mockSourcer, 1*time.Minute)
	urls := []string{"http://example.com/source1.yaml", "http://example.com/source2.yaml"}

	// Act
	sources, err := poller.Poll(urls)

	// Assert
	if err == nil {
		t.Error("expected an error, but got nil")
	}
	if sources != nil {
		t.Errorf("expected nil sources, but got %v", sources)
	}
}
