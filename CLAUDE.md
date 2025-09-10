# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go library that provides a client interface to the arXiv.org metadata API. ArXiv is a free distribution service for scholarly articles, and this library enables programmatic access to search and retrieve metadata about scientific papers.

**Module Path**: `github.com/Epistemic-Technology/arxiv`

## Key Architecture Components

### Core Library (`arxiv/` package)

#### Main Client (`arxiv.go`)
- **Client**: Main struct with configurable BaseURL, RequestMethod, Timeout, and RateLimit
- **ClientOption**: Functional options pattern for client configuration
- **Features**:
  - Client-based architecture using `Client` struct with configurable options
  - Rate limiting (respects arXiv's 3-second delay requirement) using `golang.org/x/time/rate`
  - Support for GET and POST request methods
  - XML response parsing
  - Iterator pattern for large result sets (using Go 1.23's `iter.Seq`)
  - Context support for cancellation and timeouts

#### Query Builder (`query.go`)
- **SearchQuery**: Fluent query builder for constructing complex arXiv search queries
- **Query Operations**:
  - Field-specific searches: Title, Abstract, Author, Category, Comment, Journal, All
  - Boolean operators: And(), Or(), AndNot()
  - Grouping: Group() for complex boolean expressions
  - Date ranges: SubmittedBetween(), LastUpdatedBetween()
- **Query Components**:
  - `queryNode` interface for building query trees
  - `fieldQuery`, `groupQuery`, `operatorNode`, `dateRangeQuery` implementations
  - `ParseSearchQuery()` for parsing query strings back into SearchQuery objects
- **Usage Example**:
  ```go
  query := NewSearchQuery().
      Group(func(g *SearchQuery) {
          g.Title("graph neural networks").Or().Abstract("graph neural networks")
      }).
      And().
      Category("cs.LG").
      AndNot().
      Author("Doe, John")
  ```

#### Data Types
- **SearchParams**: Configures search requests (Query, IdList, Start, MaxResults, SortBy, SortOrder)
- **SearchResponse**: Contains parsed results with metadata
- **EntryMetadata**: Individual paper data (Title, Authors, Abstract, Categories, Links, DOI, etc.)
- **RequestMethod**: Typed constant for GET/POST
- **SortBy**: Typed constants (SortByRelevance, SortByLastUpdatedDate, SortBySubmittedDate)
- **SortOrder**: Typed constants (SortOrderAscending, SortOrderDescending)

### CLI Tool (`cmd/arxiv/`)
- Command-line interface for searching arXiv
- Outputs results in tabular format using `text/tabwriter`
- Supports all search parameters via flags
- Uses context for proper request handling

### Testing
- **Unit Tests** (`arxiv_test.go`, `query_test.go`):
  - Tests with mock data in `arxiv/test_data/`
  - Query builder tests for all operations
  - Parser tests for query strings
- **Integration Tests** (`query_integration_test.go`):
  - Real API calls to arXiv (skipped in short mode)
  - Tests for all query types and operators
  - Verification of sorting and pagination
  - Iterator pattern testing

## Common Development Commands

### Testing
```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./arxiv

# Run specific test
go test -run TestParseResponse ./arxiv

# Run with coverage
go test -cover ./arxiv

# Test with race detection
go test -race ./...

# Run integration tests (hits real API)
go test -v ./arxiv -run Integration

# Skip integration tests (unit tests only)
go test -short ./...
```

### Building
```bash
# Build CLI tool
go build -o arxiv-cli ./cmd/arxiv

# Install to $GOPATH/bin
go install ./cmd/arxiv
```

### Running the CLI
```bash
# Search by query
./arxiv-cli -query "all:electron" -max-results 10

# Search by IDs
./arxiv-cli -id-list "2408.03982,2408.03988"

# With sorting
./arxiv-cli -query "all:quantum" -sort-by lastUpdatedDate -sort-order descending
```

### Code Quality
```bash
# Format code
go fmt ./...

# Static analysis
go vet ./...

# Tidy dependencies
go mod tidy
```

## Important Implementation Notes

1. **Client-based Architecture**: The library uses a `Client` struct with functional options pattern for configuration. Create a client with `NewClient()` and pass options like `WithTimeout()`, `WithRateLimit()`, etc.

2. **Query Builder Pattern**: The `SearchQuery` type provides a fluent interface for building complex queries programmatically. This makes it easier to construct dynamic queries compared to manual string concatenation.

3. **Rate Limiting**: The library enforces a minimum 3-second delay between API requests to comply with arXiv's usage policy using `golang.org/x/time/rate` limiter. Configurable via `WithRateLimit()` option.

4. **Context Support**: All API methods accept a `context.Context` parameter for proper cancellation and timeout handling.

5. **Testing Strategy**: 
   - Unit tests use mock XML data to test parsing logic
   - Integration tests verify real API behavior but respect rate limits
   - Use `-short` flag to skip integration tests during rapid development

6. **Error Handling**: The library returns errors from all operations. Always check errors, especially for network operations. Parameter validation is performed before making requests.

7. **Iterator Pattern**: The `SearchIter` method uses Go 1.23's iterator pattern (`iter.Seq`) for efficient processing of large result sets without loading all results into memory at once.

8. **XML Parsing**: Response parsing handles the Atom feed format returned by arXiv's API. Test data files in `arxiv/test_data/` show expected XML structure.

9. **Type Safety**: The library uses typed constants for `RequestMethod`, `SortBy`, and `SortOrder` to provide compile-time safety and better documentation.

## API Usage Examples

### Basic Search
```go
// Create a client with custom configuration
client := arxiv.NewClient(
    arxiv.WithTimeout(30 * time.Second),
    arxiv.WithRateLimit(5 * time.Second),
)

// Simple search with parameters
ctx := context.Background()
params := arxiv.SearchParams{
    Query:      "all:electron",
    MaxResults: 10,
    SortBy:     arxiv.SortByRelevance,
}
response, err := client.Search(ctx, params)
```

### Using the Query Builder
```go
// Build a complex query
query := arxiv.NewSearchQuery().
    Group(func(g *arxiv.SearchQuery) {
        g.Title("quantum computing").Or().Abstract("quantum computing")
    }).
    And().
    Category("quant-ph").
    SubmittedBetween(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), time.Now())

// Use the query
params := arxiv.SearchParams{
    Query:      query.String(),
    MaxResults: 20,
    SortBy:     arxiv.SortBySubmittedDate,
    SortOrder:  arxiv.SortOrderDescending,
}
response, err := client.Search(ctx, params)
```

### Iterator for Large Result Sets
```go
// Process results in batches
params := arxiv.SearchParams{
    Query:      arxiv.NewSearchQuery().Category("cs.LG").String(),
    MaxResults: 100,
}

for entry := range client.SearchIter(ctx, params) {
    // Process each entry
    fmt.Printf("Title: %s\n", entry.Title)
    fmt.Printf("Authors: %v\n", entry.Authors)
    // Iterator handles pagination automatically
}
```

### Search by arXiv IDs
```go
params := arxiv.SearchParams{
    IdList: []string{"2408.03982", "2408.03988", "2408.04000"},
}
response, err := client.Search(ctx, params)
```