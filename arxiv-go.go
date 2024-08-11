package arxivgo

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type RequestMethod int

const (
	GET RequestMethod = iota
	POST
)

type SortBy string

const (
	SortByRelevance       SortBy = "relevance"
	SortByLastUpdatedDate SortBy = "lastUpdatedDate"
	SortBySubmittedDate   SortBy = "submittedDate"
)

type SortOrder string

const (
	SortOrderAscending  SortOrder = "ascending"
	SortOrderDescending SortOrder = "descending"
)

type Config struct {
	BaseURL          string
	RequestMethod    RequestMethod
	Timeout          time.Duration
	RateLimitSeconds int
}

var DefaultConfig = Config{
	BaseURL:          "http://export.arxiv.org/api/query",
	RequestMethod:    GET,
	Timeout:          10 * time.Second,
	RateLimitSeconds: 3,
}

type SearchParams struct {
	Query      string
	IdList     []string
	Start      int
	MaxResults int
	SortBy     SortBy
	SortOrder  SortOrder
}

type Author struct {
	Name        string
	Affiliation string
}

type Category struct {
	Term   string
	Scheme string
}

type Link struct {
	Href  string
	Rel   string
	Type  string
	Title string
}

type EntryMetadata struct {
	Title            string
	ID               string
	Published        time.Time
	Updated          time.Time
	Summary          string
	Authors          []Author
	Categories       []Category
	PrimaryCategory  Category
	Links            []Link
	Comment          string
	JournalReference string
	DOI              string
}

type Requester func(SearchParams) (*http.Response, error)

func MakeRequester(config Config) Requester {
	return func(params SearchParams) (*http.Response, error) {
		if config.RequestMethod == GET {
			return doGetRequest(config, params)
		} else {
			return doPostRequest(config, params)
		}
	}
}

func RawSearch(requester Requester, params SearchParams) (*http.Response, error) {
	return requester(params)
}

func Search(requester Requester, params SearchParams) ([]EntryMetadata, error) {
	response, err := RawSearch(requester, params)
	if err != nil {
		return nil, err
	}
	parseResponse(response)
	return nil, nil
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

func doGetRequest(config Config, params SearchParams) (*http.Response, error) {
	queryString := makeGetQuery(params)
	response, err := http.Get(config.BaseURL + "?" + queryString)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func doPostRequest(config Config, params SearchParams) (*http.Response, error) {
	return nil, nil
}

func parseResponse(response *http.Response) ([]EntryMetadata, error) {
	return nil, nil
}
