package cmd

import "github.com/andrewhowdencom/ruf/internal/sourcer"

// buildSourcer creates a new sourcer with the default fetchers.
func buildSourcer() sourcer.Sourcer {
	fetcher := sourcer.NewCompositeFetcher()
	fetcher.AddFetcher("http", sourcer.NewHTTPFetcher())
	fetcher.AddFetcher("https", sourcer.NewHTTPFetcher())
	fetcher.AddFetcher("file", sourcer.NewFileFetcher())
	fetcher.AddFetcher("git", sourcer.NewGitFetcher())
	parser := sourcer.NewYAMLParser()
	return sourcer.NewSourcer(fetcher, parser)
}
