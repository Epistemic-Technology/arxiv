package arxiv_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Epistemic-Technology/arxiv/arxiv"
)

func ExampleNewClient() {
	// Create a client with default configuration
	client := arxiv.NewClient()

	// Search for papers about quantum computing
	ctx := context.Background()
	params := arxiv.SearchParams{
		Query:      "quantum computing",
		MaxResults: 5,
	}

	response, err := client.Search(ctx, params)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d papers about quantum computing\n", response.TotalResults)
}

func ExampleNewClient_withOptions() {
	// Create a client with custom configuration
	client := arxiv.NewClient(
		arxiv.WithTimeout(30*time.Second),
		arxiv.WithRateLimit(5*time.Second),
		arxiv.WithDefaultRetry(),
	)

	ctx := context.Background()
	params := arxiv.SearchParams{
		Query:      "machine learning",
		MaxResults: 10,
		SortBy:     arxiv.SortBySubmittedDate,
		SortOrder:  arxiv.SortOrderDescending,
	}

	response, err := client.Search(ctx, params)
	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range response.Entries {
		fmt.Printf("Title: %s\n", entry.Title)
		fmt.Printf("Submitted: %s\n\n", entry.Published.Format("2006-01-02"))
	}
}

func ExampleClient_Search() {
	client := arxiv.NewClient()
	ctx := context.Background()

	// Search by query
	params := arxiv.SearchParams{
		Query:      "all:electron",
		MaxResults: 10,
		SortBy:     arxiv.SortByRelevance,
	}

	response, err := client.Search(ctx, params)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Total results: %d\n", response.TotalResults)
	fmt.Printf("Showing results %d-%d\n", response.StartIndex+1, response.StartIndex+len(response.Entries))

	for _, entry := range response.Entries {
		fmt.Printf("- %s\n", entry.Title)
	}
}

func ExampleClient_Search_byID() {
	client := arxiv.NewClient()
	ctx := context.Background()

	// Search by specific arXiv IDs
	params := arxiv.SearchParams{
		IdList: []string{"2312.02121", "2312.02120"},
	}

	response, err := client.Search(ctx, params)
	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range response.Entries {
		fmt.Printf("ID: %s\n", entry.ID)
		fmt.Printf("Title: %s\n", entry.Title)
		fmt.Printf("Authors: ")
		for i, author := range entry.Authors {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(author.Name)
		}
		fmt.Println()
	}
}

func ExampleClient_SearchIter() {
	client := arxiv.NewClient()
	ctx := context.Background()

	params := arxiv.SearchParams{
		Query:      "cat:cs.LG",
		MaxResults: 100, // Will fetch in batches
	}

	count := 0
	maxResults := 250 // Process up to 250 results

	// Iterate through results efficiently
	for entry := range client.SearchIter(ctx, params) {
		count++
		fmt.Printf("%d. %s\n", count, entry.Title)

		if count >= maxResults {
			break
		}
	}

	fmt.Printf("Processed %d papers\n", count)
}

func ExampleSearchQuery() {
	// Build a simple query
	query := arxiv.NewSearchQuery().
		Title("neural networks").
		And().
		Category("cs.LG")

	fmt.Println(query.String())
	// Output: ti:neural networks AND cat:cs.LG
}

func ExampleSearchQuery_complex() {
	// Build a complex query with groups and date ranges
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)

	query := arxiv.NewSearchQuery().
		Group(func(g *arxiv.SearchQuery) {
			g.Title("transformer").Or().Title("attention mechanism")
		}).
		And().
		Category("cs.CL").
		AndNot().
		Title("survey").
		SubmittedBetween(start, end)

	client := arxiv.NewClient()
	ctx := context.Background()

	params := arxiv.SearchParams{
		Query:      query.String(),
		MaxResults: 10,
	}

	response, err := client.Search(ctx, params)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d papers matching complex query\n", response.TotalResults)
}

func ExampleSearchQuery_allFields() {
	// Search across all fields
	query := arxiv.NewSearchQuery().All("quantum entanglement")

	fmt.Println(query.String())
	// Output: all:quantum entanglement
}

func ExampleWithInterceptor() {
	// Create a logging interceptor
	loggingInterceptor := func(ctx context.Context, params arxiv.SearchParams, next arxiv.SearchFunc) (arxiv.SearchResults, error) {
		start := time.Now()
		fmt.Printf("Searching for: %s\n", params.Query)

		result, err := next(ctx, params)

		duration := time.Since(start)
		if err != nil {
			fmt.Printf("Search failed after %v: %v\n", duration, err)
		} else {
			fmt.Printf("Found %d results in %v\n", result.TotalResults, duration)
		}

		return result, err
	}

	// Create client with interceptor
	client := arxiv.NewClient(
		arxiv.WithInterceptor(loggingInterceptor),
	)

	ctx := context.Background()
	params := arxiv.SearchParams{
		Query:      "machine learning",
		MaxResults: 5,
	}

	_, err := client.Search(ctx, params)
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleWithRetry() {
	// Configure automatic retry for transient failures
	client := arxiv.NewClient(
		arxiv.WithRetry(arxiv.RetryConfig{
			MaxAttempts:     3,
			InitialInterval: 1 * time.Second,
			MaxInterval:     10 * time.Second,
			Multiplier:      2.0,
		}),
	)

	ctx := context.Background()
	params := arxiv.SearchParams{
		Query:      "distributed systems",
		MaxResults: 10,
	}

	// The client will automatically retry on temporary failures
	response, err := client.Search(ctx, params)
	if err != nil {
		log.Printf("Search failed after retries: %v", err)
		return
	}

	fmt.Printf("Successfully retrieved %d results\n", len(response.Entries))
}

func ExampleClient_SearchNext() {
	client := arxiv.NewClient()
	ctx := context.Background()

	// Get first page of results
	params := arxiv.SearchParams{
		Query:      "deep learning",
		MaxResults: 10,
	}

	firstPage, err := client.Search(ctx, params)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("First page: %d-%d of %d total results\n",
		firstPage.StartIndex+1,
		firstPage.StartIndex+len(firstPage.Entries),
		firstPage.TotalResults)

	// Get next page if available
	if arxiv.SearchHasMoreResults(firstPage) {
		secondPage, err := client.SearchNext(ctx, firstPage)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Second page: %d-%d of %d total results\n",
			secondPage.StartIndex+1,
			secondPage.StartIndex+len(secondPage.Entries),
			secondPage.TotalResults)
	}
}

func ExampleParseSearchQuery() {
	// Parse a query string back into a SearchQuery object
	queryString := "ti:quantum computing AND cat:quant-ph"

	query, err := arxiv.ParseSearchQuery(queryString)
	if err != nil {
		log.Fatal(err)
	}

	// You can now modify the parsed query
	modifiedQuery := query.AndNot().Author("Anonymous")

	fmt.Println(modifiedQuery.String())
	// Output: ti:quantum computing AND cat:quant-ph ANDNOT au:Anonymous
}
