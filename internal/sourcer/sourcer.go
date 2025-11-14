package sourcer

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/ghodss/yaml"
	"github.com/teambition/rrule-go"
	"github.com/xeipuuv/gojsonschema"
)

// Source represents a source file.
type Source struct {
	Campaign model.Campaign `json:"campaign" yaml:"campaign"`
	Calls    []model.Call   `json:"calls" yaml:"calls"`
	Events   []model.Event  `json:"events" yaml:"events"`
}

// Fetcher defines the interface for fetching content from a URL.
type Fetcher interface {
	Fetch(url string) ([]byte, string, error)
}

// CompositeFetcher is a fetcher that can handle multiple schemes.
type CompositeFetcher struct {
	fetchers map[string]Fetcher
}

// NewCompositeFetcher creates a new CompositeFetcher.
func NewCompositeFetcher() *CompositeFetcher {
	return &CompositeFetcher{
		fetchers: make(map[string]Fetcher),
	}
}

// AddFetcher adds a new fetcher for a given scheme.
func (f *CompositeFetcher) AddFetcher(scheme string, fetcher Fetcher) {
	f.fetchers[scheme] = fetcher
}

// Fetch fetches the content of a URL and returns it as a byte slice.
func (f *CompositeFetcher) Fetch(rawURL string) ([]byte, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse url %s: %w", rawURL, err)
	}

	fetcher, ok := f.fetchers[u.Scheme]
	if !ok {
		return nil, "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	return fetcher.Fetch(rawURL)
}

// HTTPFetcher is an implementation of Fetcher that fetches content over HTTP.
type HTTPFetcher struct {
	client *http.Client
}

// NewHTTPFetcher creates a new HTTPFetcher.
func NewHTTPFetcher(client *http.Client) *HTTPFetcher {
	return &HTTPFetcher{
		client: client,
	}
}

// Fetch fetches the content of a URL and returns it as a byte slice.
func (f *HTTPFetcher) Fetch(url string) ([]byte, string, error) {
	resp, err := f.client.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch url %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to fetch url %s: status code %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	// Prefer ETag, but fall back to Last-Modified.
	var state string
	if etag := resp.Header.Get("ETag"); etag != "" {
		state = etag
	} else if lastModified := resp.Header.Get("Last-Modified"); lastModified != "" {
		state = lastModified
	} else {
		state = fmt.Sprintf("%x", sha256.Sum256(body))
	}

	return body, state, nil
}

// FileFetcher is an implementation of Fetcher that fetches content from a local file.
type FileFetcher struct{}

// NewFileFetcher creates a new FileFetcher.
func NewFileFetcher() *FileFetcher {
	return &FileFetcher{}
}

// Fetch fetches the content of a URL and returns it as a byte slice.
func (f *FileFetcher) Fetch(rawURL string) ([]byte, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse url %s: %w", rawURL, err)
	}

	data, err := os.ReadFile(u.Path)
	if err != nil {
		return nil, "", err
	}

	return data, fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

// Parser defines the interface for parsing content into a list of calls.
type Parser interface {
	Parse(url string, data []byte) (*Source, error)
}

// YAMLParser is an implementation of Parser that parses YAML content.
type YAMLParser struct {
	schemaLoader gojsonschema.JSONLoader
}

// NewYAMLParser creates a new YAMLParser.
func NewYAMLParser(schemaPath string) (*YAMLParser, error) {
	schemaLoader := gojsonschema.NewReferenceLoader(fmt.Sprintf("file://%s", schemaPath))
	_, err := schemaLoader.LoadJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}

	return &YAMLParser{
		schemaLoader: schemaLoader,
	}, nil
}

// Parse parses a YAML byte slice and returns a list of calls.
func (p *YAMLParser) Parse(rawURL string, data []byte) (*Source, error) {
	// Convert YAML to JSON, as gojsonschema only works with JSON
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert yaml to json: %w", err)
	}

	documentLoader := gojsonschema.NewBytesLoader(jsonData)

	result, err := gojsonschema.Validate(p.schemaLoader, documentLoader)
	if err != nil {
		return nil, fmt.Errorf("failed to validate document: %w", err)
	}

	if !result.Valid() {
		log.Printf("document '%s' is not valid:", rawURL)
		for _, desc := range result.Errors() {
			log.Printf("- %s", desc)
		}
		return nil, nil // Returning nil, nil to skip the file
	}

	var s Source
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml: %w", err)
	}

	p.fillCampaign(rawURL, &s)

	// Add the campaign to each call.
	for i := range s.Calls {
		s.Calls[i].Campaign = s.Campaign
	}

	// Validate RRules
	for _, call := range s.Calls {
		for _, trigger := range call.Triggers {
			if trigger.RRule != "" {
				if _, err := rrule.StrToRRule(trigger.RRule); err != nil {
					log.Printf("document '%s' is not valid: invalid rrule: %s", rawURL, err)
					return nil, nil // Returning nil, nil to skip the file
				}
			}
		}
	}

	return &s, nil
}

func (p *YAMLParser) fillCampaign(rawURL string, s *Source) error {
	// If the campaign isn't specified, we'll derive it from the filename.
	if s.Campaign.ID == "" {
		u, err := url.Parse(rawURL)
		if err != nil {
			return fmt.Errorf("failed to parse url %s: %w", rawURL, err)
		}

		// my-campaign.yaml -> my-campaign-yaml
		base := u.Path[strings.LastIndex(u.Path, "/")+1:]
		s.Campaign.ID = strings.ReplaceAll(
			strings.TrimSuffix(base, ".yaml"),
			".", "-",
		)
	}
	if s.Campaign.Name == "" {
		u, err := url.Parse(rawURL)
		if err != nil {
			return fmt.Errorf("failed to parse url %s: %w", rawURL, err)
		}
		s.Campaign.Name = u.Path
	}
	return nil
}

// Sourcer is an interface that defines the methods for sourcing calls.
type Sourcer interface {
	Source(url string) (*Source, string, error)
}

// sourcer is the concrete implementation of the Sourcer interface.
type sourcer struct {
	fetcher Fetcher
	parser  Parser
}

// NewSourcer creates a new Sourcer.
func NewSourcer(fetcher Fetcher, parser Parser) Sourcer {
	return &sourcer{
		fetcher: fetcher,
		parser:  parser,
	}
}

// Source fetches and parses calls from a URL.
func (s *sourcer) Source(url string) (*Source, string, error) {
	data, state, err := s.fetcher.Fetch(url)
	if err != nil {
		return nil, "", err
	}

	source, err := s.parser.Parse(url, data)
	if err != nil {
		return nil, "", err
	}

	// If the source is nil, it means the document was invalid and should be skipped.
	if source == nil {
		return nil, "", nil
	}

	return source, state, nil
}
