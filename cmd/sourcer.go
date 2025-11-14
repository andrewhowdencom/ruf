package cmd

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/andrewhowdencom/ruf/internal/http"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
)

// buildSourcer creates a new sourcer with the default fetchers.
func buildSourcer() (sourcer.Sourcer, error) {
	httpClient := http.NewClient()

	fetcher := sourcer.NewCompositeFetcher()
	fetcher.AddFetcher("http", sourcer.NewHTTPFetcher(httpClient))
	fetcher.AddFetcher("https", sourcer.NewHTTPFetcher(httpClient))
	fetcher.AddFetcher("file", sourcer.NewFileFetcher())
	fetcher.AddFetcher("git", sourcer.NewGitFetcher())

	// Get the path to the current source file, and then find the schema file relative to that.
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(b)
	schemaPath := filepath.Join(basepath, "..", "schema", "calls.json")

	parser, err := sourcer.NewYAMLParser(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	return sourcer.NewSourcer(fetcher, parser), nil
}
