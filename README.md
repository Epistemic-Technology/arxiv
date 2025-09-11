# ArXiv Go Client

A Go library for interfacing with the [arXiv.org](https://arxiv.org/) metadata API.

[ArXiv](https://arxiv.org/) provides a public API for accessing metadata of scientific papers.
Documentation for the API can be found in the [ArXiv API User Manual](https://info.arxiv.org/help/api/user-manual.html).

## Installation

```bash
go get github.com/Epistemic-Technology/arxiv
```

## Usage Examples

### Basic Search

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/Epistemic-Technology/arxiv/arxiv"
)

func main() {
    // Create a client with default configuration
    client := arxiv.NewClient()
    
    // Search for papers
    ctx := context.Background()
    params := arxiv.SearchParams{
        Query:      "all:electron",
        MaxResults: 10,
    }
    
    response, err := client.Search(ctx, params)
    if err != nil {
        log.Fatal(err)
    }
    
    for _, entry := range response.Entries {
        fmt.Printf("Title: %s\n", entry.Title)
        fmt.Printf("Authors: %v\n", entry.Authors)
        fmt.Printf("Abstract: %s\n\n", entry.Abstract)
    }
}
```

### Using the Query Builder

```go
// Build complex queries with the fluent interface
query := arxiv.NewSearchQuery().
    Group(func(g *arxiv.SearchQuery) {
        g.Title("quantum computing").Or().Abstract("quantum computing")
    }).
    And().
    Category("quant-ph").
    AndNot().
    Author("Smith")

params := arxiv.SearchParams{
    Query:      query.String(),
    MaxResults: 20,
    SortBy:     arxiv.SortBySubmittedDate,
    SortOrder:  arxiv.SortOrderDescending,
}

response, err := client.Search(ctx, params)
```

### Search with Date Ranges

```go
// Find recent papers in machine learning
query := arxiv.NewSearchQuery().
    Category("cs.LG").
    And().
    SubmittedBetween(
        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
        time.Now(),
    )

params := arxiv.SearchParams{
    Query:      query.String(),
    MaxResults: 50,
}

response, err := client.Search(ctx, params)
```

### Using the Iterator for Large Result Sets

```go
// Process results in batches without loading all into memory
params := arxiv.SearchParams{
    Query:      arxiv.NewSearchQuery().Category("cs.AI").String(),
    MaxResults: 100, // Per batch
}

count := 0
for entry := range client.SearchIter(ctx, params) {
    fmt.Printf("[%d] %s\n", count+1, entry.Title)
    count++
    
    // Stop after processing 500 papers
    if count >= 500 {
        break
    }
}
```

### Search by arXiv IDs

```go
// Retrieve specific papers by their arXiv IDs
params := arxiv.SearchParams{
    IdList: []string{"2408.03982", "2408.03988", "2408.04000"},
}

response, err := client.Search(ctx, params)
if err != nil {
    log.Fatal(err)
}

for _, entry := range response.Entries {
    fmt.Printf("ID: %s\n", entry.ID)
    fmt.Printf("Title: %s\n", entry.Title)
    fmt.Printf("PDF: %s\n\n", entry.PdfURL)
}
```

### Custom Client Configuration

```go
import "time"

// Create a client with custom settings
client := arxiv.NewClient(
    arxiv.WithTimeout(30 * time.Second),
    arxiv.WithRateLimit(5 * time.Second), // Respect API rate limits
    arxiv.WithRequestMethod(arxiv.RequestMethodPOST),
    arxiv.WithRetry(arxiv.RetryConfig{
        MaxAttempts:     3,               // Retry up to 3 times
        InitialInterval: 1 * time.Second, // Start with 1 second delay
        MaxInterval:     10 * time.Second, // Cap backoff at 10 seconds
        Multiplier:      2.0,              // Double the delay each time
    }),
)
```

### Automatic Retry with Exponential Backoff

The client supports automatic retry for transient failures:

```go
// Simple retry configuration with defaults
client := arxiv.NewClient(
    arxiv.WithRetry(arxiv.RetryConfig{
        MaxAttempts: 3,
    }),
)

// Or with custom backoff settings
client := arxiv.NewClient(
    arxiv.WithRetry(arxiv.RetryConfig{
        MaxAttempts:     5,
        InitialInterval: 500 * time.Millisecond,
        MaxInterval:     30 * time.Second,
        Multiplier:      1.5,
    }),
)
```

The retry mechanism:
- Automatically retries on temporary network errors
- Retries on HTTP status codes: 408, 429, 500, 502, 503, 504
- Uses exponential backoff with jitter to prevent thundering herd
- Respects context cancellation during backoff periods

### Pagination

```go
// Manual pagination through results
params := arxiv.SearchParams{
    Query:      "all:neural networks",
    Start:      0,
    MaxResults: 20,
}

// First page
response, err := client.Search(ctx, params)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Showing results %d-%d of %d\n", 
    response.StartIndex+1, 
    response.StartIndex+len(response.Entries),
    response.TotalResults)

// Next page
params.Start = response.StartIndex + len(response.Entries)
response, err = client.Search(ctx, params)
```

## CLI Tool

The library includes a command-line tool for searching arXiv:

```bash
# Install the CLI
go install github.com/Epistemic-Technology/arxiv/cmd/arxiv@latest

# Search by query
arxiv -query "all:electron" -max-results 10

# Search by IDs
arxiv -id-list "2408.03982,2408.03988"

# Advanced search with sorting
arxiv -query "cat:cs.LG" -sort-by lastUpdatedDate -sort-order descending -max-results 20
```

## Query Builder Reference

The query builder supports:

- **Field searches**: `Title()`, `Abstract()`, `Author()`, `Category()`, `Comment()`, `Journal()`, `All()`
- **Boolean operators**: `And()`, `Or()`, `AndNot()`
- **Grouping**: `Group()` for complex boolean expressions
- **Date ranges**: `SubmittedBetween()`, `LastUpdatedBetween()`

Example of a complex query:

```go
query := arxiv.NewSearchQuery().
    Group(func(g *arxiv.SearchQuery) {
        g.Title("graph neural networks").
            Or().Abstract("GNN")
    }).
    And().
    Group(func(g *arxiv.SearchQuery) {
        g.Category("cs.LG").Or().Category("cs.AI")
    }).
    AndNot().
    Author("Anonymous")
```

## Features

- **Client-based architecture** with configurable options
- **Rate limiting** to respect arXiv's API guidelines
- **Automatic retry** with exponential backoff for transient failures
- **Query builder** for constructing complex searches programmatically
- **Iterator pattern** for efficient processing of large result sets
- **Context support** for cancellation and timeouts
- **Type-safe constants** for sort options and request methods
- **Comprehensive error handling**

## Requirements

- Go 1.23 or later (uses `iter.Seq` for iterators)

## License

MIT License - see [LICENSE.md](LICENSE.md) for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.