// Package arxiv provides a client interface to the arXiv metadata API.
//
// [ArXiv] provides a public API for accessing metadata of scientific papers.
// Documentation for the API can be found in the [ArXiv API User Manual].
//
// Basic usage:
//
//	package main
//
//	import (
//		"context"
//		"fmt"
//		"time"
//
//		"github.com/Epistemic-Technology/arxiv/arxiv"
//	)
//
//	func main() {
//		// Create a client with optional configuration
//		client := arxiv.NewClient(
//			arxiv.WithTimeout(30 * time.Second),
//			arxiv.WithRateLimit(3 * time.Second),
//			arxiv.WithRetry(arxiv.RetryConfig{
//				MaxAttempts:     3,
//				InitialInterval: 1 * time.Second,
//				MaxInterval:     10 * time.Second,
//				Multiplier:      2.0,
//			}),
//		)
//
//		// Search using the client
//		ctx := context.Background()
//		params := arxiv.SearchParams{
//			Query:      "all:electron",
//			MaxResults: 10,
//		}
//
//		response, err := client.Search(ctx, params)
//		if err != nil {
//			panic(err)
//		}
//
//		for _, entry := range response.Entries {
//			fmt.Printf("Title: %s\n", entry.Title)
//		}
//
//		// Get next page of results
//		nextPage, err := client.SearchNext(ctx, response)
//		if err != nil {
//			// Handle end of results or error
//		}
//		_ = nextPage
//	}
//
// [ArXiv]: https://arxiv.org/
// [ArXiv API User Manual]: https://info.arxiv.org/help/api/user-manual.html
package arxiv

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"iter"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// Client represents an arXiv API client.
type Client struct {
	BaseURL       string        // Base URL for the arXiv API
	RequestMethod RequestMethod // HTTP request method to use
	Timeout       time.Duration // Timeout for the HTTP request
	RateLimit     time.Duration // How long to wait between requests
	RetryConfig   *RetryConfig  // Configuration for retry
	interceptors  []Interceptor // Interceptors for modifying search behavior
	httpClient    *http.Client
	rateLimiter   *rate.Limiter
}

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxAttempts     int           // Maximum number of retry attempts (0 = no retries)
	InitialInterval time.Duration // Initial backoff interval
	MaxInterval     time.Duration // Maximum backoff interval
	Multiplier      float64       // Backoff multiplier (typically 2.0)
}

type ClientOption func(*Client)

// SearchFunc represents a function that performs a search operation.
// This is the core signature that interceptors wrap.
type SearchFunc func(ctx context.Context, params SearchParams) (SearchResults, error)

// Interceptor wraps a SearchFunc to add behavior before/after/instead of the search.
// Interceptors can:
// - Modify parameters before calling next
// - Short-circuit by not calling next (e.g., return cached results)
// - Handle errors from next
// - Add timing, logging, or other cross-cutting concerns
type Interceptor func(ctx context.Context, params SearchParams, next SearchFunc) (SearchResults, error)

// NewClient creates a new arXiv API client with the given options.
func NewClient(options ...ClientOption) *Client {
	client := &Client{
		BaseURL:       "http://export.arxiv.org/api/query",
		RequestMethod: RequestMethodGet,
		Timeout:       10 * time.Second,
		RateLimit:     3,
	}

	for _, option := range options {
		option(client)
	}

	if client.RateLimit > 0 {
		client.rateLimiter = rate.NewLimiter(rate.Every(client.RateLimit), 1)
	}

	client.httpClient = &http.Client{
		Timeout: client.Timeout,
	}

	return client
}

func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.BaseURL = baseURL
	}
}

func WithRequestMethod(method RequestMethod) ClientOption {
	return func(c *Client) {
		c.RequestMethod = method
	}
}

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.Timeout = timeout
	}
}

func WithRateLimit(rateLimit time.Duration) ClientOption {
	return func(c *Client) {
		c.RateLimit = rateLimit
	}
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func WithRateLimiter(rateLimiter *rate.Limiter) ClientOption {
	return func(c *Client) {
		c.rateLimiter = rateLimiter
	}
}

// WithRetry configures automatic retry with exponential backoff for transient failures.
// Common configurations:
//   - For basic retry: WithRetry(RetryConfig{MaxAttempts: 3})
//   - For custom backoff: WithRetry(RetryConfig{
//     MaxAttempts: 5,
//     InitialInterval: 1 * time.Second,
//     MaxInterval: 30 * time.Second,
//     Multiplier: 2.0,
//     })
func WithRetry(config RetryConfig) ClientOption {
	return func(c *Client) {
		// Set sensible defaults if not provided
		if config.InitialInterval == 0 {
			config.InitialInterval = 1 * time.Second
		}
		if config.MaxInterval == 0 {
			config.MaxInterval = 30 * time.Second
		}
		if config.Multiplier == 0 {
			config.Multiplier = 2.0
		}
		c.RetryConfig = &config
	}
}

// WithDefaultRetry configures automatic retry with exponential backoff for transient failures.
// Default settings are MaxAttempts: 3, InitialInterval: 1s, MaxInterval: 30s, Multiplier: 2.0.
func WithDefaultRetry() ClientOption {
	return func(c *Client) {
		c.RetryConfig = &RetryConfig{
			MaxAttempts:     3,
			InitialInterval: 1 * time.Second,
			MaxInterval:     30 * time.Second,
			Multiplier:      2.0,
		}
	}
}

// WithInterceptor adds one or more interceptors to the client.
// Interceptors are executed in the order they are added, with the first
// interceptor being the outermost (called first, returns last).
// Example usage:
//
//	client := arxiv.NewClient(
//		arxiv.WithInterceptor(
//			CachingInterceptor(cache, 5*time.Minute),
//			LoggingInterceptor(logger),
//		),
//	)
func WithInterceptor(interceptors ...Interceptor) ClientOption {
	return func(c *Client) {
		c.interceptors = append(c.interceptors, interceptors...)
	}
}

// RequestMethod specifies the HTTP method for API requests. ArXiv's API supports
// both GET and POST methods for search queries.
type RequestMethod int

const (
	RequestMethodGet RequestMethod = iota
	RequestMethodPost
)

// SearchParams contains parameters for making a search request to the arXiv API.
// See the [arXiv API documentation] for more information on the available
// parameters and constructing queries.
//
// Note that MaxResults should be limited to 2000 and Start should be limited to 30000.
//
// [arXiv API documentation]: https://info.arxiv.org/help/api/user-manual.html#_query_interface
type SearchParams struct {
	Query      string
	IdList     []string
	Start      int
	MaxResults int
	SortBy     SortBy
	SortOrder  SortOrder
}

func (p SearchParams) Validate() error {
	if p.MaxResults > 2000 {
		return fmt.Errorf("maxResults cannot exceed 2000")
	}
	if p.Start > 30000 {
		return fmt.Errorf("start cannot exceed 30000")
	}
	return nil
}

// SortBy specifies how to sort search results.
type SortBy string

// Query represents a search query string for the arXiv API.
type Query string

const (
	SortByRelevance       SortBy = "relevance"
	SortByLastUpdatedDate SortBy = "lastUpdatedDate"
	SortBySubmittedDate   SortBy = "submittedDate"
)

// SortOrder specifies the ordering direction for sorted search results.
type SortOrder string

const (
	SortOrderAscending  SortOrder = "ascending"
	SortOrderDescending SortOrder = "descending"
)

// SearchResults contains metadata for search results returned by the arXiv API.
// The Params field contains the parameters used to make the search request.
type SearchResults struct {
	Links        []Link          `xml:"link"`         // Links included in the response. Includes link for current search.
	Title        string          `xml:"title"`        // Title of the search response, includes search query.
	ID           string          `xml:"id"`           // ID of the search response, as a URL.
	Updated      string          `xml:"updated"`      // Time the search response was updated (generally the time it was made).
	TotalResults int             `xml:"totalResults"` // Total number of results available for the search query.
	StartIndex   int             `xml:"startIndex"`   // Index of the first result returned in the current response.
	ItemsPerPage int             `xml:"itemsPerPage"` // Number of results returned in the current response.
	Entries      []EntryMetadata `xml:"entry"`        // Metadata for each entry in the search response.
	Params       SearchParams    // Parameters used to make the search request.
}

// EntryMetadata contains metadata for a single entry in the search response.
type EntryMetadata struct {
	Title            string     `xml:"title"`            // Title of the entry.
	ID               string     `xml:"id"`               // ID of the entry, as a URL.
	Published        time.Time  `xml:"published"`        // Time the entry was published.
	Updated          time.Time  `xml:"updated"`          // Time the entry was last updated.
	Summary          string     `xml:"summary"`          // Summary (abstract) of the entry.
	Authors          []Author   `xml:"author"`           // Authors of the entry.
	Categories       []Category `xml:"category"`         // Subject categories of the entry.
	PrimaryCategory  Category   `xml:"primary_category"` // Primary subject category of the entry.
	Links            []Link     `xml:"link"`             // Links included in the entry. Includes link to the PDF.
	Comment          string     `xml:"comment"`          // Comment on the entry. Includes information such as where the paper was submitted or number of pages, figures, etc.
	JournalReference string     `xml:"journal_ref"`      // Journal reference for the entry.
	DOI              string     `xml:"doi"`              // Digital Object Identifier (DOI) for the entry.
}

// Author contains information about an author of a paper.
type Author struct {
	Name        string `xml:"name"`
	Affiliation string `xml:"affiliation"`
}

// Category contains information about a subject category of a paper.
type Category struct {
	Term string `xml:"term,attr"`
}

// Link contains information about a link associated with a paper.
type Link struct {
	Href  string `xml:"href,attr"`
	Rel   string `xml:"rel,attr"`
	Type  string `xml:"type,attr"`
	Title string `xml:"title,attr"`
}

// isRetryableError determines if an error is retryable.
// Retryable errors include temporary network failures and specific HTTP status codes.
func isRetryableError(err error, response *http.Response) bool {
	if err != nil {
		// Retry on timeout or temporary network errors
		if errors.Is(err, context.DeadlineExceeded) {
			return true
		}
		// Check for temporary network errors
		if urlErr, ok := err.(*url.Error); ok && urlErr.Temporary() {
			return true
		}
	}

	if response != nil {
		// Retry on specific HTTP status codes
		switch response.StatusCode {
		case http.StatusTooManyRequests, // 429
			http.StatusRequestTimeout,      // 408
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout:      // 504
			return true
		}
	}

	return false
}

// calculateBackoff calculates the next backoff interval with jitter.
func calculateBackoff(attempt int, config *RetryConfig) time.Duration {
	if config == nil || attempt <= 0 {
		return 0
	}

	// Calculate exponential backoff
	backoff := float64(config.InitialInterval) * math.Pow(config.Multiplier, float64(attempt-1))

	// Cap at max interval
	if backoff > float64(config.MaxInterval) {
		backoff = float64(config.MaxInterval)
	}

	// Add jitter (±10% randomization)
	jitter := 0.1 * backoff * (2*rand.Float64() - 1)
	backoff += jitter

	return time.Duration(backoff)
}

// RawSearch makes a search request to the arXiv API and returns the raw HTTP response.
// The caller is responsible for closing the response body.
// If RetryConfig is set, the method will automatically retry on transient failures
func (c *Client) RawSearch(ctx context.Context, params SearchParams) (*http.Response, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	// Determine max attempts
	maxAttempts := 1
	if c.RetryConfig != nil && c.RetryConfig.MaxAttempts > 0 {
		maxAttempts = c.RetryConfig.MaxAttempts
	}

	var lastErr error
	var lastResponse *http.Response

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Apply rate limiting
		if c.rateLimiter != nil {
			if err := c.rateLimiter.Wait(ctx); err != nil {
				return nil, err
			}
		}

		// Make the request
		var response *http.Response
		var err error
		if c.RequestMethod == RequestMethodGet {
			response, err = DoGetRequest(ctx, c, params)
		} else {
			response, err = DoPostRequest(ctx, c, params)
		}

		// If successful, return immediately
		if err == nil && response != nil && response.StatusCode == http.StatusOK {
			return response, nil
		}

		// Store the last error and response for potential return
		lastErr = err
		lastResponse = response

		// Check if we should retry
		if attempt < maxAttempts && isRetryableError(err, response) {
			// Close the response body if it exists before retrying
			if response != nil && response.Body != nil {
				response.Body.Close()
			}

			// Calculate backoff and wait
			backoff := calculateBackoff(attempt, c.RetryConfig)
			if backoff > 0 {
				select {
				case <-time.After(backoff):
					// Continue to next attempt
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		} else {
			// Not retryable or last attempt, return the result
			break
		}
	}

	return lastResponse, lastErr
}

// Search makes a search request to the arXiv API and returns the parsed response.
func (c *Client) Search(ctx context.Context, params SearchParams) (SearchResults, error) {
	// Build the interceptor chain
	searchFunc := c.doSearch

	// Apply interceptors in reverse order (first added = outermost)
	for i := len(c.interceptors) - 1; i >= 0; i-- {
		interceptor := c.interceptors[i]
		next := searchFunc
		searchFunc = func(ctx context.Context, p SearchParams) (SearchResults, error) {
			return interceptor(ctx, p, next)
		}
	}

	// Execute the interceptor chain
	return searchFunc(ctx, params)
}

// doSearch performs the actual search operation.
// This is the core implementation that interceptors wrap.
func (c *Client) doSearch(ctx context.Context, params SearchParams) (SearchResults, error) {
	response, err := c.RawSearch(ctx, params)
	if err != nil {
		return SearchResults{}, err
	}
	defer response.Body.Close()

	parsedResponse, err := ParseResponse(response.Body)
	if err != nil {
		return SearchResults{}, err
	}
	parsedResponse.Params = params

	return parsedResponse, nil
}

// SearchNext retrieves the next page of results based on the current SearchResponse.
func (c *Client) SearchNext(ctx context.Context, response SearchResults) (SearchResults, error) {
	if !SearchHasMoreResults(response) {
		return SearchResults{}, errors.New("no more results")
	}
	response.Params.Start = response.StartIndex + response.ItemsPerPage
	return c.Search(ctx, response.Params)
}

// SearchPrevious retrieves the previous page of results based on the current SearchResponse.
func (c *Client) SearchPrevious(ctx context.Context, response SearchResults) (SearchResults, error) {
	if !SearchHasPreviousResults(response) {
		return SearchResults{}, errors.New("no more results")
	}
	response.Params.Start = response.StartIndex - response.ItemsPerPage
	if response.Params.Start < 0 {
		response.Params.Start = 0
	}
	return c.Search(ctx, response.Params)
}

// SearchIter returns an iterator over search results, automatically handling pagination.
// The iterator will make multiple API requests as needed to retrieve all results.
func (c *Client) SearchIter(ctx context.Context, params SearchParams) iter.Seq[EntryMetadata] {
	return func(yield func(EntryMetadata) bool) {
		for {
			response, err := c.Search(ctx, params)
			if err != nil {
				return
			}
			for _, entry := range response.Entries {
				if !yield(entry) {
					return
				}
			}
			if !SearchHasMoreResults(response) {
				return
			}
			params.Start = response.StartIndex + response.ItemsPerPage
		}
	}
}

// SearchHasMoreResults returns true if there are more results available for the search query.
func SearchHasMoreResults(response SearchResults) bool {
	return response.TotalResults > 0 && response.StartIndex+response.ItemsPerPage < response.TotalResults
}

// SearchHasPreviousResults returns true if there are previous results available for the search query.
func SearchHasPreviousResults(response SearchResults) bool {
	return response.TotalResults > 0 && response.StartIndex > 0
}

// DoGetRequest performs a GET request to the arXiv API with the specified parameters.
func DoGetRequest(ctx context.Context, client *Client, params SearchParams) (*http.Response, error) {
	queryString := makeGetQuery(params)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.BaseURL+"?"+queryString, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// DoPostRequest performs a POST request to the arXiv API with the specified parameters.
func DoPostRequest(ctx context.Context, client *Client, params SearchParams) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.BaseURL, nil)
	if err != nil {
		return nil, err
	}
	if params.Query != "" {
		req.Header.Add("search_query", params.Query)
	}
	if len(params.IdList) > 0 {
		idListStr := strings.Join(params.IdList, ",")
		req.Header.Add("id_list", idListStr)
	}
	if params.Start > 0 {
		req.Header.Add("start", strconv.Itoa(params.Start))
	}
	if params.MaxResults > 0 {
		req.Header.Add("max_results", strconv.Itoa(params.MaxResults))
	}
	if params.SortBy != "" {
		req.Header.Add("sortBy", string(params.SortBy))
	}
	if params.SortOrder != "" {
		req.Header.Add("sortOrder", string(params.SortOrder))
	}
	response, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// ParseResponse parses a search response from the arXiv API.
func ParseResponse(responseData io.Reader) (SearchResults, error) {
	decoder := xml.NewDecoder(responseData)
	var searchResponse SearchResults
	err := decoder.Decode(&searchResponse)
	if err != nil {
		return SearchResults{}, err
	}
	return searchResponse, nil
}

// ParseSingleEntry parses a single entry from the arXiv API.
func ParseSingleEntry(entryData io.Reader) (EntryMetadata, error) {
	decoder := xml.NewDecoder(entryData)
	var entry EntryMetadata
	err := decoder.Decode(&entry)
	if err != nil {
		return EntryMetadata{}, err
	}
	return entry, nil
}

func makeGetQuery(params SearchParams) string {
	query := url.Values{}

	if params.Query != "" {
		query.Add("search_query", params.Query)
	}
	if len(params.IdList) > 0 {
		idListStr := strings.Join(params.IdList, ",")
		query.Add("id_list", idListStr)
	}
	if params.Start > 0 {
		query.Add("start", strconv.Itoa(params.Start))
	}
	if params.MaxResults > 0 {
		query.Add("max_results", strconv.Itoa(params.MaxResults))
	}
	if params.SortBy != "" {
		query.Add("sortBy", string(params.SortBy))
	}
	if params.SortOrder != "" {
		query.Add("sortOrder", string(params.SortOrder))
	}

	return query.Encode()
}
