package meta

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
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
	params := SearchParams{
		Query: "all:electron",
	}
	resp, err := DoGetRequest(*http.DefaultClient, DefaultConfig, params)
	if err != nil {
		t.Errorf("GetRequest(%v) = %v; want nil", params, err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GetRequest(%v) = %v; want 200", params, resp.StatusCode)
	}
}

func TestPostRequestGetsOKResponseWithDefaultConfig(t *testing.T) {
	params := SearchParams{
		Query: "all:electron",
	}
	config := DefaultConfig
	config.RequestMethod = POST
	resp, err := DoPostRequest(*http.DefaultClient, config, params)
	if err != nil {
		t.Errorf("PostRequest(%v) = %v; want nil", params, err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("PostRequest(%v) = %v; want 200", params, resp.StatusCode)
	}
}

func TestSearchWorksWithDefaultConfig(t *testing.T) {
	requester := MakeRequester(DefaultConfig)
	params := SearchParams{
		Query: "all:electron",
	}
	_, err := Search(requester, params)
	if err != nil {
		t.Errorf("Search(%v) = %v; want nil", params, err)
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
	config := DefaultConfig
	config.RateLimit = true
	config.RateLimitSeconds = 1

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	mockClient := mockServer.Client()

	requester := MakeRequesterWithClient(*mockClient, config)

	params := SearchParams{
		Query: "all:electron",
	}
	start := time.Now()
	iterations := 5
	for i := 0; i < iterations; i++ {
		_, err := Search(requester, params)
		if err != nil {
			t.Fatalf("Search() = %v; want nil", err)
		}
	}
	elapsed := time.Since(start)
	minSeconds := config.RateLimitSeconds * (iterations - 1)
	if elapsed < time.Duration(minSeconds)*time.Second {
		t.Errorf("Rate limit is not working: elapsed time = %v; want at least %v seconds", elapsed, minSeconds)
	}

	// Close the mock server
	mockServer.Close()
}
