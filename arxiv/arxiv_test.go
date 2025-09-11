package arxiv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestMakeGetQuery(t *testing.T) {
	tests := []struct {
		params SearchParams
		want   string
	}{
		{
			SearchParams{
				Query: "all:electron",
			},
			"search_query=all%3Aelectron",
		},
		{
			SearchParams{
				Query:  "all:galaxy",
				IdList: []string{"2408.03982", "2408.03988"},
			},
			"id_list=2408.03982%2C2408.03988&search_query=all%3Agalaxy",
		},
		{
			SearchParams{
				Query:      "all:galaxy",
				IdList:     []string{"2408.03982", "2408.03988"},
				Start:      10,
				MaxResults: 5,
			},
			"id_list=2408.03982%2C2408.03988&max_results=5&search_query=all%3Agalaxy&start=10",
		},
		{
			SearchParams{
				Query: "au:del_maestro ANDNOT (ti:checkerboard OR ti:Pyrochlore)",
			},
			"search_query=au%3Adel_maestro+ANDNOT+%28ti%3Acheckerboard+OR+ti%3APyrochlore%29",
		},
		{
			SearchParams{
				Query:     "all:electron",
				SortBy:    SortByRelevance,
				SortOrder: SortOrderAscending,
			},
			"search_query=all%3Aelectron&sortBy=relevance&sortOrder=ascending",
		},
		{
			SearchParams{
				Query:     "all:electron",
				SortBy:    SortByLastUpdatedDate,
				SortOrder: SortOrderDescending,
			},
			"search_query=all%3Aelectron&sortBy=lastUpdatedDate&sortOrder=descending",
		},
		{
			SearchParams{
				Query:     "all:electron",
				SortBy:    SortBySubmittedDate,
				SortOrder: SortOrderAscending,
			},
			"search_query=all%3Aelectron&sortBy=submittedDate&sortOrder=ascending",
		},
		{
			SearchParams{
				Query:      "all:electron",
				Start:      10,
				MaxResults: 50,
			},
			"max_results=50&search_query=all%3Aelectron&start=10",
		},
	}
	for _, test := range tests {
		got := makeGetQuery(test.params)
		if got != test.want {
			t.Errorf("makeGetQuery(%v) = %v; want %v", test.params, got, test.want)
		}
	}
}

func TestGetRequestGetsOKResponseWithDefaultConfig(t *testing.T) {
	client := NewClient()
	ctx := context.Background()
	params := SearchParams{
		Query: "all:electron",
	}
	resp, err := DoGetRequest(ctx, client, params)
	if err != nil {
		t.Errorf("DoGetRequest(%v) = %v; want nil", params, err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("DoGetRequest(%v) = %v; want 200", params, resp.StatusCode)
	}
}

func TestPostRequestGetsOKResponseWithDefaultConfig(t *testing.T) {
	client := NewClient(WithRequestMethod(RequestMethodPost))
	ctx := context.Background()
	params := SearchParams{
		Query: "all:electron",
	}
	resp, err := DoPostRequest(ctx, client, params)
	if err != nil {
		t.Errorf("DoPostRequest(%v) = %v; want nil", params, err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("DoPostRequest(%v) = %v; want 200", params, resp.StatusCode)
	}
}

func TestSearchWorksWithDefaultConfig(t *testing.T) {
	client := NewClient()
	ctx := context.Background()
	params := SearchParams{
		Query: "all:electron",
	}
	_, err := client.Search(ctx, params)
	if err != nil {
		t.Errorf("Search(%v) = %v; want nil", params, err)
	}
}

func TestSearchIteratesOverMultiplePages(t *testing.T) {
	client := NewClient()
	ctx := context.Background()
	params := SearchParams{
		Query:      "all:electron",
		MaxResults: 10,
	}
	count := 0
	for result := range client.SearchIter(ctx, params) {
		if result.ID == "" {
			t.Errorf("SearchIter() = %v; want non-empty string", result.ID)
		}
		count++
		if count > 30 {
			break
		}
	}
	if count < 30 {
		t.Errorf("SearchIter() = %v; want at least 30 results", count)
	}
}

func TestParseResponse(t *testing.T) {
	file, err := os.Open("test_data/full-results.xml")
	if err != nil {
		t.Fatalf("Failed to open test_data/full-results.xml: %v", err)
	}
	defer file.Close()

	parsedResponse, err := ParseResponse(file)
	if err != nil {
		t.Fatalf("ParseResponse() = %v; want nil", err)
	}
	if parsedResponse.TotalResults != 12345 {
		t.Errorf("ParseResponse().TotalResults = %v; want 12345", parsedResponse.TotalResults)
	}
	if parsedResponse.StartIndex != 99 {
		t.Errorf("ParseResponse().StartIndex = %v; want 99", parsedResponse.StartIndex)
	}
	if parsedResponse.ItemsPerPage != 10 {
		t.Errorf("ParseResponse().ItemsPerPage = %v; want 10", parsedResponse.ItemsPerPage)
	}
	if len(parsedResponse.Entries) != 10 {
		t.Errorf("ParseResponse().Entries = %v; want 10", len(parsedResponse.Entries))
	}

}

func TestParseSingleEntry(t *testing.T) {
	file, err := os.Open("test_data/single-entry.xml")
	if err != nil {
		t.Fatalf("Failed to open test_data/single-entry.xml: %v", err)
	}
	defer file.Close()

	parsedEntry, err := ParseSingleEntry(file)
	if err != nil {
		t.Fatalf("ParseSingleEntry() = %v; want nil", err)
	}
	if len(parsedEntry.Links) != 3 {
		t.Errorf("ParseSingleEntry().Links = %v; want 3", len(parsedEntry.Links))
	}
	for i, link := range parsedEntry.Links {
		if link.Href == "" {
			t.Errorf("ParseSingleEntry().Links[%v].Href = %v; want non-empty string", i, link.Href)
		}
	}
	if parsedEntry.Title != "The Title" {
		t.Errorf("ParseSingleEntry().Title = %v; want 'The Title'", parsedEntry.Title)
	}
	if parsedEntry.ID != "http://arxiv.org/abs/cond-mat/12345v1" {
		t.Errorf("ParseSingleEntry().ID = %v; want 'http://arxiv.org/abs/cond-mat/12345v1'", parsedEntry.ID)
	}
	published, _ := time.Parse("2006-01-02T15:04:05Z", "2001-02-28T20:12:09Z")
	if !parsedEntry.Published.Equal(published) {
		t.Errorf("ParseSingleEntry().Published = %v; want '2001-02-28T20:12:09Z'", parsedEntry.Published)
	}
	updated, _ := time.Parse("2006-01-02T15:04:05Z", "2001-02-28T20:12:09Z")
	if !parsedEntry.Updated.Equal(updated) {
		t.Errorf("ParseSingleEntry().Updated = %v; want '2001-02-28T20:12:09Z'", parsedEntry.Updated)
	}
	if parsedEntry.Summary != "The Summary" {
		t.Errorf("ParseSingleEntry().Summary = %v; want 'The Summary'", parsedEntry.Summary)
	}
	if len(parsedEntry.Authors) != 2 {
		t.Errorf("ParseSingleEntry().Authors = %v; want 2", len(parsedEntry.Authors))
	}
	if parsedEntry.Authors[0].Name != "First Author" {
		t.Errorf("ParseSingleEntry().Authors[0].Name = %v; want 'First Author'", parsedEntry.Authors[0].Name)
	}
	if parsedEntry.Authors[0].Affiliation != "First Affiliation" {
		t.Errorf("ParseSingleEntry().Authors[0].Affiliation = %v; want 'First Affiliation'", parsedEntry.Authors[0].Affiliation)
	}
	if parsedEntry.Authors[1].Name != "Second Author" {
		t.Errorf("ParseSingleEntry().Authors[1].Name = %v; want 'Second Author'", parsedEntry.Authors[1].Name)
	}
	if parsedEntry.Authors[1].Affiliation != "Second Affiliation" {
		t.Errorf("ParseSingleEntry().Authors[1].Affiliation = %v; want 'Second Affiliation'", parsedEntry.Authors[1].Affiliation)
	}
	if parsedEntry.DOI != "10.9090/1.12345" {
		t.Errorf("ParseSingleEntry().DOI = %v; want '10.9090/1.12345'", parsedEntry.DOI)
	}
	if parsedEntry.JournalReference != "Journal Reference" {
		t.Errorf("ParseSingleEntry().JournalReference = %v; want 'Journal Reference'", parsedEntry.JournalReference)
	}
	if parsedEntry.Comment != "The Comment" {
		t.Errorf("ParseSingleEntry().Comment = %v; want 'The Comment'", parsedEntry.Comment)
	}
	if parsedEntry.PrimaryCategory.Term != "primary-category" {
		t.Errorf("ParseSingleEntry().primary-category = %v; want 'primary-category'", parsedEntry.PrimaryCategory.Term)
	}
	if len(parsedEntry.Categories) != 2 {
		t.Errorf("ParseSingleEntry().Categories = %v; want 2", len(parsedEntry.Categories))
	}
	if parsedEntry.Categories[0].Term != "first-category" {
		t.Errorf("ParseSingleEntry().Categories[0].Term = %v; want 'first-category'", parsedEntry.Categories[0].Term)
	}
	if parsedEntry.Categories[1].Term != "second-category" {
		t.Errorf("ParseSingleEntry().Categories[1].Term = %v; want 'second-category'", parsedEntry.Categories[1].Term)
	}
}

func TestRateLimit(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/">
  <opensearch:totalResults>0</opensearch:totalResults>
  <opensearch:startIndex>0</opensearch:startIndex>
  <opensearch:itemsPerPage>0</opensearch:itemsPerPage>
</feed>`))
	}))
	defer mockServer.Close()

	client := NewClient(
		WithBaseURL(mockServer.URL),
		WithRateLimit(1*time.Second),
		WithHTTPClient(mockServer.Client()),
	)

	ctx := context.Background()
	params := SearchParams{
		Query: "all:electron",
	}

	start := time.Now()
	iterations := 5
	for i := 0; i < iterations; i++ {
		_, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search() = %v; want nil", err)
		}
	}
	elapsed := time.Since(start)
	minSeconds := 1 * (iterations - 1)
	if elapsed < time.Duration(minSeconds)*time.Second {
		t.Errorf("Rate limit is not working: elapsed time = %v; want at least %v seconds", elapsed, minSeconds)
	}
}

func TestSearchNext(t *testing.T) {
	client := NewClient()
	ctx := context.Background()
	params := SearchParams{
		Query:      "all:electron",
		MaxResults: 10,
	}

	response, err := client.Search(ctx, params)
	if err != nil {
		t.Fatalf("Search() = %v; want nil", err)
	}

	if SearchHasMoreResults(response) {
		nextResponse, err := client.SearchNext(ctx, response)
		if err != nil {
			t.Errorf("SearchNext() = %v; want nil", err)
		}
		if nextResponse.StartIndex != response.StartIndex+response.ItemsPerPage {
			t.Errorf("SearchNext().StartIndex = %v; want %v", nextResponse.StartIndex, response.StartIndex+response.ItemsPerPage)
		}
	}
}

func TestSearchPrevious(t *testing.T) {
	client := NewClient()
	ctx := context.Background()
	params := SearchParams{
		Query:      "all:electron",
		Start:      20,
		MaxResults: 10,
	}

	response, err := client.Search(ctx, params)
	if err != nil {
		t.Fatalf("Search() = %v; want nil", err)
	}

	if SearchHasPreviousResults(response) {
		prevResponse, err := client.SearchPrevious(ctx, response)
		if err != nil {
			t.Errorf("SearchPrevious() = %v; want nil", err)
		}
		expectedStart := response.StartIndex - response.ItemsPerPage
		if expectedStart < 0 {
			expectedStart = 0
		}
		if prevResponse.StartIndex != expectedStart {
			t.Errorf("SearchPrevious().StartIndex = %v; want %v", prevResponse.StartIndex, expectedStart)
		}
	}
}

func TestClientOptions(t *testing.T) {
	customTimeout := 30 * time.Second
	customRateLimit := 5 * time.Second
	customBaseURL := "http://custom.url/api"

	client := NewClient(
		WithBaseURL(customBaseURL),
		WithRequestMethod(RequestMethodPost),
		WithTimeout(customTimeout),
		WithRateLimit(customRateLimit),
	)

	if client.BaseURL != customBaseURL {
		t.Errorf("Client.BaseURL = %v; want %v", client.BaseURL, customBaseURL)
	}
	if client.RequestMethod != RequestMethodPost {
		t.Errorf("Client.RequestMethod = %v; want %v", client.RequestMethod, RequestMethodPost)
	}
	if client.Timeout != customTimeout {
		t.Errorf("Client.Timeout = %v; want %v", client.Timeout, customTimeout)
	}
	if client.RateLimit != customRateLimit {
		t.Errorf("Client.RateLimit = %v; want %v", client.RateLimit, customRateLimit)
	}
}

func TestWithDefaultRetry(t *testing.T) {
	client := NewClient(WithDefaultRetry())

	if client.RetryConfig == nil {
		t.Fatal("RetryConfig should not be nil when WithDefaultRetry is used")
	}

	// Check default values
	if client.RetryConfig.MaxAttempts != 3 {
		t.Errorf("Default MaxAttempts = %d; want 3", client.RetryConfig.MaxAttempts)
	}
	if client.RetryConfig.InitialInterval != 1*time.Second {
		t.Errorf("Default InitialInterval = %v; want 1s", client.RetryConfig.InitialInterval)
	}
	if client.RetryConfig.MaxInterval != 30*time.Second {
		t.Errorf("Default MaxInterval = %v; want 30s", client.RetryConfig.MaxInterval)
	}
	if client.RetryConfig.Multiplier != 2.0 {
		t.Errorf("Default Multiplier = %v; want 2.0", client.RetryConfig.Multiplier)
	}
}

func TestWithRateLimiter(t *testing.T) {
	// Test that WithRateLimiter option is accepted without error
	// We can't directly access the private rateLimiter field, but we can
	// verify the option works by using the client
	customLimiter := rate.NewLimiter(rate.Every(1*time.Second), 1)

	// This should not panic
	client := NewClient(WithRateLimiter(customLimiter))

	// Verify client was created successfully
	if client == nil {
		t.Error("Client should not be nil")
	}

	// Test that multiple rate options can be provided (last one wins)
	client2 := NewClient(
		WithRateLimit(5*time.Second),
		WithRateLimiter(customLimiter),
	)

	if client2 == nil {
		t.Error("Client with multiple rate options should not be nil")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		params  SearchParams
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid query",
			params: SearchParams{
				Query: "test",
			},
			wantErr: false,
		},
		{
			name: "valid id list",
			params: SearchParams{
				IdList: []string{"1234.5678"},
			},
			wantErr: false,
		},
		{
			name: "both query and id list",
			params: SearchParams{
				Query:  "test",
				IdList: []string{"1234.5678"},
			},
			wantErr: false,
		},
		{
			name: "maxResults exceeds 2000",
			params: SearchParams{
				Query:      "test",
				MaxResults: 2001,
			},
			wantErr: true,
			errMsg:  "maxResults cannot exceed 2000",
		},
		{
			name: "start exceeds 30000",
			params: SearchParams{
				Query: "test",
				Start: 30001,
			},
			wantErr: true,
			errMsg:  "start cannot exceed 30000",
		},
		{
			name:    "empty params are valid",
			params:  SearchParams{},
			wantErr: false, // The current implementation doesn't validate empty params
		},
		{
			name: "negative values are allowed",
			params: SearchParams{
				Query:      "test",
				Start:      -1,
				MaxResults: -1,
			},
			wantErr: false, // The current implementation doesn't check for negative values
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestSearchWithValidation(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not reach here if validation fails
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/">
  <opensearch:totalResults>0</opensearch:totalResults>
</feed>`))
	}))
	defer mockServer.Close()

	client := NewClient(
		WithBaseURL(mockServer.URL),
		WithHTTPClient(mockServer.Client()),
	)
	ctx := context.Background()

	// Test with params that exceed limits
	t.Run("maxResults exceeds limit", func(t *testing.T) {
		params := SearchParams{
			Query:      "test",
			MaxResults: 2001,
		}

		_, err := client.Search(ctx, params)
		if err == nil {
			t.Error("Expected validation error for maxResults > 2000")
		}
	})

	t.Run("start exceeds limit", func(t *testing.T) {
		params := SearchParams{
			Query: "test",
			Start: 30001,
		}

		_, err := client.Search(ctx, params)
		if err == nil {
			t.Error("Expected validation error for start > 30000")
		}
	})
}
