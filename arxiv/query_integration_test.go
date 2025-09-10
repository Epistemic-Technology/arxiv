package arxiv

import (
	"context"
	"testing"
	"time"
)

func TestSearchQueryIntegration_BasicFields(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewClient()
	ctx := context.Background()

	tests := []struct {
		name         string
		queryBuilder func() *SearchQuery
		minResults   int
		description  string
	}{
		{
			name: "title search",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().Title("quantum computing")
			},
			minResults:  1,
			description: "Should find papers with 'quantum computing' in title",
		},
		{
			name: "author search",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().Author("Hinton")
			},
			minResults:  1,
			description: "Should find papers by author Hinton",
		},
		{
			name: "category search",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().Category("cs.LG")
			},
			minResults:  10,
			description: "Should find papers in cs.LG category",
		},
		{
			name: "all fields search",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().All("neural network")
			},
			minResults:  10,
			description: "Should find papers with 'neural network' in any field",
		},
		{
			name: "abstract search",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().Abstract("machine learning")
			},
			minResults:  10,
			description: "Should find papers with 'machine learning' in abstract",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := tt.queryBuilder()
			params := SearchParams{
				Query:      query.String(),
				MaxResults: 20,
			}

			response, err := client.Search(ctx, params)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			if response.TotalResults < tt.minResults {
				t.Errorf("%s: expected at least %d results, got %d",
					tt.description, tt.minResults, response.TotalResults)
			}

			if len(response.Entries) == 0 && response.TotalResults > 0 {
				t.Errorf("%s: TotalResults=%d but no entries returned",
					tt.description, response.TotalResults)
			}

			// Verify the query string was properly formed
			t.Logf("Query string: %s", query.String())
			t.Logf("Total results: %d", response.TotalResults)
		})
	}
}

func TestSearchQueryIntegration_Operators(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewClient()
	ctx := context.Background()

	tests := []struct {
		name         string
		queryBuilder func() *SearchQuery
		description  string
	}{
		{
			name: "AND operator",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().
					Category("cs.LG").
					And().
					Title("neural")
			},
			description: "Papers in cs.LG with 'neural' in title",
		},
		{
			name: "OR operator",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().
					Category("cs.LG").
					Or().
					Category("cs.AI")
			},
			description: "Papers in either cs.LG or cs.AI",
		},
		{
			name: "ANDNOT operator",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().
					Category("cs.LG").
					AndNot().
					Title("survey")
			},
			description: "Papers in cs.LG but not surveys",
		},
		{
			name: "multiple operators",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().
					Title("deep").
					And().
					Title("learning").
					AndNot().
					Title("survey")
			},
			description: "Papers with 'deep' and 'learning' in title but not 'survey'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := tt.queryBuilder()
			params := SearchParams{
				Query:      query.String(),
				MaxResults: 10,
			}

			response, err := client.Search(ctx, params)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			t.Logf("%s", tt.description)
			t.Logf("Query string: %s", query.String())
			t.Logf("Total results: %d", response.TotalResults)

			if response.TotalResults == 0 {
				t.Logf("Warning: No results found for query: %s", query.String())
			}
		})
	}
}

func TestSearchQueryIntegration_Groups(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewClient()
	ctx := context.Background()

	tests := []struct {
		name         string
		queryBuilder func() *SearchQuery
		description  string
	}{
		{
			name: "simple group",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().
					Group(func(g *SearchQuery) {
						g.Title("quantum").Or().Title("classical")
					}).
					And().
					Category("physics.quant-ph")
			},
			description: "Papers with (quantum OR classical) in title AND in physics.quant-ph",
		},
		{
			name: "complex group",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().
					Group(func(g *SearchQuery) {
						g.Title("neural network").Or().Abstract("neural network")
					}).
					And().
					Category("cs.LG")
			},
			description: "Papers with 'neural network' in title or abstract AND in cs.LG",
		},
		{
			name: "multiple groups",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().
					Group(func(g *SearchQuery) {
						g.Category("cs.LG").Or().Category("cs.AI")
					}).
					And().
					Group(func(g *SearchQuery) {
						g.Title("transformer").Or().Title("attention")
					})
			},
			description: "Papers in (cs.LG OR cs.AI) AND (transformer OR attention in title)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := tt.queryBuilder()
			params := SearchParams{
				Query:      query.String(),
				MaxResults: 10,
			}

			response, err := client.Search(ctx, params)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			t.Logf("%s", tt.description)
			t.Logf("Query string: %s", query.String())
			t.Logf("Total results: %d", response.TotalResults)

			if response.TotalResults == 0 {
				t.Logf("Warning: No results found for query: %s", query.String())
			}
		})
	}
}

func TestSearchQueryIntegration_DateRanges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewClient()
	ctx := context.Background()

	// Use a date range that's likely to have results
	start := time.Now().AddDate(-1, 0, 0) // 1 year ago
	end := time.Now()

	tests := []struct {
		name         string
		queryBuilder func() *SearchQuery
		description  string
	}{
		{
			name: "submitted date range",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().
					Category("cs.LG").
					SubmittedBetween(start, end)
			},
			description: "Papers in cs.LG submitted in the last year",
		},
		{
			name: "last updated date range",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().
					Category("cs.AI").
					LastUpdatedBetween(start, end)
			},
			description: "Papers in cs.AI updated in the last year",
		},
		{
			name: "complex query with date range",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().
					Group(func(g *SearchQuery) {
						g.Title("transformer").Or().Title("bert")
					}).
					And().
					Category("cs.CL").
					SubmittedBetween(start, end)
			},
			description: "Papers about transformer/bert in cs.CL from last year",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := tt.queryBuilder()
			params := SearchParams{
				Query:      query.String(),
				MaxResults: 10,
			}

			response, err := client.Search(ctx, params)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			t.Logf("%s", tt.description)
			t.Logf("Query string: %s", query.String())
			t.Logf("Total results: %d", response.TotalResults)

			if response.TotalResults == 0 {
				t.Logf("Warning: No results found for query: %s", query.String())
			}
		})
	}
}

func TestSearchQueryIntegration_ComplexRealWorld(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewClient()
	ctx := context.Background()

	// Test the exact example from the requirements
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 23, 59, 0, 0, time.UTC)

	query := NewSearchQuery().
		Group(func(g *SearchQuery) {
			g.Title("graph neural networks").Or().Abstract("graph neural networks")
		}).
		And().
		Category("cs.LG").
		AndNot().
		Author("Doe, John").
		SubmittedBetween(start, end)

	params := SearchParams{
		Query:      query.String(),
		MaxResults: 10,
	}

	response, err := client.Search(ctx, params)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	t.Logf("Complex query test:")
	t.Logf("Query string: %s", query.String())
	t.Logf("Total results: %d", response.TotalResults)

	// Verify we got some results (should be papers about GNNs in cs.LG, not by John Doe)
	if response.TotalResults > 0 {
		// Check that none of the results have "Doe, John" as author
		for _, entry := range response.Entries {
			for _, author := range entry.Authors {
				if author.Name == "Doe, John" || author.Name == "John Doe" {
					t.Errorf("Found paper by excluded author 'Doe, John': %s", entry.Title)
				}
			}
		}
		t.Logf("Verified: No papers by 'Doe, John' in results")
	}
}

func TestSearchQueryIntegration_WithSearchIterator(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewClient()
	ctx := context.Background()

	// Build a query using SearchQuery
	query := NewSearchQuery().
		Category("cs.LG").
		And().
		Title("deep learning")

	params := SearchParams{
		Query:      query.String(),
		MaxResults: 5,
	}

	count := 0
	maxIterations := 15

	for entry := range client.SearchIter(ctx, params) {
		count++

		// Verify entry has expected fields
		if entry.ID == "" {
			t.Errorf("Entry %d has empty ID", count)
		}
		if entry.Title == "" {
			t.Errorf("Entry %d has empty Title", count)
		}

		if count >= maxIterations {
			break
		}
	}

	if count < maxIterations {
		t.Logf("Iterator returned %d results (less than max %d)", count, maxIterations)
	} else {
		t.Logf("Successfully iterated through %d results", count)
	}
}

func TestSearchQueryIntegration_SpecialCharacters(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewClient()
	ctx := context.Background()

	tests := []struct {
		name         string
		queryBuilder func() *SearchQuery
		description  string
	}{
		{
			name: "author with comma",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().Author("LeCun, Yann")
			},
			description: "Author name with comma",
		},
		{
			name: "title with quotes",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().Title(`"deep learning"`)
			},
			description: "Title with quoted phrase",
		},
		{
			name: "category with dot",
			queryBuilder: func() *SearchQuery {
				return NewSearchQuery().Category("stat.ML")
			},
			description: "Category with dot notation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := tt.queryBuilder()
			params := SearchParams{
				Query:      query.String(),
				MaxResults: 5,
			}

			response, err := client.Search(ctx, params)
			if err != nil {
				t.Fatalf("Search failed for %s: %v", tt.description, err)
			}

			t.Logf("%s", tt.description)
			t.Logf("Query string: %s", query.String())
			t.Logf("Total results: %d", response.TotalResults)
		})
	}
}

func TestSearchQueryIntegration_Sorting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewClient()
	ctx := context.Background()

	query := NewSearchQuery().Category("cs.LG")

	sortOptions := []struct {
		sortBy    SortBy
		sortOrder SortOrder
		desc      string
	}{
		{SortByRelevance, SortOrderDescending, "relevance descending"},
		{SortByLastUpdatedDate, SortOrderDescending, "last updated descending"},
		{SortBySubmittedDate, SortOrderAscending, "submitted date ascending"},
	}

	for _, opt := range sortOptions {
		t.Run(opt.desc, func(t *testing.T) {
			params := SearchParams{
				Query:      query.String(),
				MaxResults: 5,
				SortBy:     opt.sortBy,
				SortOrder:  opt.sortOrder,
			}

			response, err := client.Search(ctx, params)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			t.Logf("Sort: %s", opt.desc)
			t.Logf("Results: %d", response.TotalResults)

			// Log first few titles to verify sorting
			for i, entry := range response.Entries {
				if i >= 3 {
					break
				}
				t.Logf("  %d. %s (updated: %s)", i+1,
					entry.Title[:min(50, len(entry.Title))],
					entry.Updated.Format("2006-01-02"))
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
