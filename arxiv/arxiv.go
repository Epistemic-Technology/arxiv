// Package arxiv provides a simple interface to the arXiv metadata API.
//
// [ArXiv] provides a public API for accessing metadata of scientific papers.
// Documentation for the API can be found in the [ArXiv API User Manual].
//
// Basic usage:
//
//	   package main
//	   import (
//	         "github.com/mikethicke/arxiv-go"
//	   )
//	   func main() {
//	         params := meta.SearchParams{
//	                 Query: "all:electron",
//	         }
//	         requester := meta.MakeRequester(arxivgo.DefaultConfig)
//	         response, err := meta.Search(requester, params)
//	         if err != nil {
//	                 panic(err)
//	         }
//	         for _, entry := range response.Entries {
//					  // Do something
//	         }
//	         nextPage, err := meta.SearchNext(requester, response)
//	         // Do something
//	    }
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
	httpClient    *http.Client
	rateLimiter   *rate.Limiter
}

type ClientOption func(*Client)

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

// Request method for making API search requests. ArXiv's API supports GET or POST
// requests for search queries.
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

// Possible ways to sort search results.
type SortBy string

// The search query.
type Query string

const (
	SortByRelevance       SortBy = "relevance"
	SortByLastUpdatedDate SortBy = "lastUpdatedDate"
	SortBySubmittedDate   SortBy = "submittedDate"
)

// Possible sort orders for search results.
type SortOrder string

const (
	SortOrderAscending  SortOrder = "ascending"
	SortOrderDescending SortOrder = "descending"
)

// SearchResponse contains metadata for search results returned by the arXiv API.
// The Params field contains the parameters used to make the search request.
type SearchResponse struct {
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

// RawSearch makes a search request to the arXiv API using the provided Requester.
func (c *Client) RawSearch(ctx context.Context, params SearchParams) (*http.Response, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, err
		}
	}
	if c.RequestMethod == RequestMethodGet {
		return DoGetRequest(ctx, c, params)
	}
	return DoPostRequest(ctx, c, params)
}

// Search makes a search request to the arXiv API using the provided Requester and parses the response.
func (c *Client) Search(ctx context.Context, params SearchParams) (SearchResponse, error) {
	response, err := c.RawSearch(ctx, params)
	if err != nil {
		return SearchResponse{}, err
	}
	defer response.Body.Close()
	parsedResponse, err := ParseResponse(response.Body)
	if err != nil {
		return SearchResponse{}, err
	}
	parsedResponse.Params = params
	return parsedResponse, nil
}

// SearchNext searches for the next page of results using the provided Requester and SearchResponse.
func (c *Client) SearchNext(ctx context.Context, response SearchResponse) (SearchResponse, error) {
	if !SearchHasMoreResults(response) {
		return SearchResponse{}, errors.New("no more results")
	}
	response.Params.Start = response.StartIndex + response.ItemsPerPage
	return c.Search(ctx, response.Params)
}

// SearchPrevious searches for the previous page of results using the provided Requester and SearchResponse.
func (c *Client) SearchPrevious(ctx context.Context, response SearchResponse) (SearchResponse, error) {
	if !SearchHasPreviousResults(response) {
		return SearchResponse{}, errors.New("no more results")
	}
	response.Params.Start = response.StartIndex - response.ItemsPerPage
	if response.Params.Start < 0 {
		response.Params.Start = 0
	}
	return c.Search(ctx, response.Params)
}

// Returns an iterator over the search results. The iterator will make multiple requests to the API as needed.
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
func SearchHasMoreResults(response SearchResponse) bool {
	return response.TotalResults > 0 && response.StartIndex+response.ItemsPerPage < response.TotalResults
}

// SearchHasPreviousResults returns true if there are previous results available for the search query.
func SearchHasPreviousResults(response SearchResponse) bool {
	return response.TotalResults > 0 && response.StartIndex > 0
}

// DoGetRequest makes a GET request to the arXiv API. It is normally called by the Requester.
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

// DoPostRequest makes a POST request to the arXiv API. It is normally called by the Requester.
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
func ParseResponse(responseData io.Reader) (SearchResponse, error) {
	decoder := xml.NewDecoder(responseData)
	var searchResponse SearchResponse
	err := decoder.Decode(&searchResponse)
	if err != nil {
		return SearchResponse{}, err
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
