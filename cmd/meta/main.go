package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/Epistemic-Technology/arxiv/meta"
)

func main() {
	query := flag.String("query", "", "Search query")
	idList := flag.String("id-list", "", "Comma-separated list of IDs")
	start := flag.Int("start", 0, "Start index for results")
	maxResults := flag.Int("max-results", 0, "Maximum number of results")
	sortBy := flag.String("sort-by", "", "Field to sort results by (relevance, lastUpdatedDate, submittedDate)")
	sortOrder := flag.String("sort-order", "", "Sort order (ascending, descending)")

	flag.Parse()

	params := meta.SearchParams{
		Query:      *query,
		IdList:     strings.Split(*idList, ","),
		Start:      *start,
		MaxResults: *maxResults,
		SortBy:     meta.SortBy(*sortBy),
		SortOrder:  meta.SortOrder(*sortOrder),
	}

	requester := meta.MakeRequester(meta.DefaultConfig)
	response, err := meta.Search(requester, params)
	if err != nil {
		fmt.Println("Error searching arXiv:", err)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "Title\tAuthor\tYear\tDOI")
	fmt.Fprintln(w, "-----\t----\t----\t---")
	for _, result := range response.Entries {
		fmt.Fprintf(
			w,
			"%.50s\t%.20s\t%d\t%.20s\n",
			result.Title,
			result.Authors[0].Name,
			result.Published.Year(),
			result.DOI,
		)
	}
	w.Flush()
}
