package arxivgo

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
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
	SB_Relevance       SortBy = "relevance"
	SB_LastUpdatedDate SortBy = "lastUpdatedDate"
	SB_SubmittedDate   SortBy = "submittedDate"
)

type SortOrder string

const (
	SO_Ascending  SortOrder = "ascending"
	SO_Descending SortOrder = "descending"
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
	Query      string    `query:"search_query"`
	IdList     []string  `query:"id_list"`
	Start      int       `query:"start"`
	MaxResults int       `query:"max_results"`
	SortBy     SortBy    `query:"sortBy"`
	SortOrder  SortOrder `query:"sortOrder"`
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
	// Doing a bunch of reflect nonsense so that we can build query strings
	// from the struct tags.
	v := reflect.ValueOf(params)
	q := url.Values{}

	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		tag := field.Tag.Get("query")
		var value string
		if v.Field(i).Kind() == reflect.Slice {
			s := reflect.ValueOf(v.Field(i).Interface())
			sliceStrings := make([]string, s.Len())
			for i := 0; i < s.Len(); i++ {
				sliceStrings[i] = fmt.Sprint(s.Index(i))
			}
			value = strings.Join(sliceStrings, ",")
		} else {
			value = fmt.Sprintf("%v", v.Field(i))
		}
		if value != "" && value != "0" {
			q.Add(tag, value)
		}
	}

	return q.Encode()
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
