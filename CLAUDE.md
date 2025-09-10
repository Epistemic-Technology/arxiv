# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go library that provides a client interface to the arXiv.org metadata API. ArXiv is a free distribution service for scholarly articles, and this library enables programmatic access to search and retrieve metadata about scientific papers.

**Module Path**: `github.com/Epistemic-Technology/arxiv`

## Key Architecture Components

### Core Library (`arxiv/` package)
- **arxiv.go**: Main API client implementation with:
  - Client-based architecture using `Client` struct with configurable options
  - Rate limiting (respects arXiv's 3-second delay requirement) using `golang.org/x/time/rate`
  - Support for GET and POST request methods
  - XML response parsing
  - Iterator pattern for large result sets (using Go 1.23's `iter.Seq`)
  - Context support for cancellation and timeouts
- **Client**: Main struct with configurable BaseURL, RequestMethod, Timeout, and RateLimit
- **ClientOption**: Functional options pattern for client configuration
- **SearchParams**: Configures search requests (Query, IdList, Start, MaxResults, SortBy, SortOrder)
- **SearchResponse**: Contains parsed results with metadata
- **EntryMetadata**: Individual paper data (Title, Authors, Abstract, Categories, Links, DOI, etc.)

### CLI Tool (`cmd/arxiv/`)
- Command-line interface for searching arXiv
- Outputs results in tabular format using `text/tabwriter`
- Supports all search parameters via flags
- Uses context for proper request handling

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

1. **Client-based Architecture**: The library now uses a `Client` struct with functional options pattern for configuration. Create a client with `NewClient()` and pass options like `WithTimeout()`, `WithRateLimit()`, etc.

2. **Rate Limiting**: The library enforces a minimum 3-second delay between API requests to comply with arXiv's usage policy using `golang.org/x/time/rate` limiter. Configurable via `WithRateLimit()` option.

3. **Context Support**: All API methods now accept a `context.Context` parameter for proper cancellation and timeout handling.

4. **Testing**: Tests include both unit tests with mock data (in `arxiv/test_data/`) and integration tests that hit the real arXiv API. Be mindful of rate limits when running tests.

5. **Error Handling**: The library returns errors from all operations. Always check errors, especially for network operations. Parameter validation is performed before making requests.

6. **Iterator Pattern**: The `SearchIter` method uses Go 1.23's iterator pattern (`iter.Seq`) for efficient processing of large result sets without loading all results into memory at once.

7. **XML Parsing**: Response parsing handles the Atom feed format returned by arXiv's API. Test data files in `arxiv/test_data/` show expected XML structure.

8. **Type Safety**: The library uses typed constants for `RequestMethod`, `SortBy`, and `SortOrder` to provide compile-time safety and better documentation.

## API Usage Examples

```go
// Create a client with custom configuration
client := arxiv.NewClient(
    arxiv.WithTimeout(30 * time.Second),
    arxiv.WithRateLimit(5 * time.Second),
)

// Search with parameters
ctx := context.Background()
params := arxiv.SearchParams{
    Query:      "all:electron",
    MaxResults: 10,
    SortBy:     arxiv.SortByRelevance,
}
response, err := client.Search(ctx, params)

// Use iterator for large result sets
for entry := range client.SearchIter(ctx, params) {
    // Process each entry
}
```