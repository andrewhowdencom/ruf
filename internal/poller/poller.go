package poller

import (
	"fmt"
	"time"

	"github.com/andrewhowdencom/ruf/internal/sourcer"
)

// Poller periodically checks for updates in a list of sources.
type Poller struct {
	sourcer    sourcer.Sourcer
	interval   time.Duration
	knownState map[string]string
}

// New creates a new Poller.
func New(sourcer sourcer.Sourcer, interval time.Duration) *Poller {
	return &Poller{
		sourcer:    sourcer,
		interval:   interval,
		knownState: make(map[string]string),
	}
}

// Poll checks for updates in the sources and returns the calls from the changed URLs.
func (p *Poller) Poll(urls []string) ([]*sourcer.Source, error) {
	var allSources []*sourcer.Source
	var lastErr error
	for _, url := range urls {
		source, err := p.pollURL(url)
		if err != nil {
			// If a source can't be found, we log the error and continue.
			fmt.Printf("Error checking source %s: %v\n", url, err)
			lastErr = err
			continue
		}
		if source != nil {
			allSources = append(allSources, source)
		}
	}

	// If we failed to poll all sources, and we have no sources to return,
	// then we should return the last error we saw.
	if len(allSources) == 0 && lastErr != nil {
		return nil, fmt.Errorf("failed to poll any sources: %w", lastErr)
	}
	return allSources, nil
}

func (p *Poller) pollURL(url string) (*sourcer.Source, error) {
	source, state, err := p.sourcer.Source(url)
	if err != nil {
		return nil, err
	}

	if p.knownState[url] == state {
		return nil, nil // No change
	}

	p.knownState[url] = state
	return source, nil
}
