# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go library that provides a client interface to the arXiv.org metadata API. ArXiv is a free distribution service for scholarly articles, and this library enables programmatic access to search and retrieve metadata about scientific papers.

**Module Path**: `github.com/Epistemic-Technology/arxiv`

## Key Architecture Components

### Core Library (`arxiv/` package)
- **arxiv.go**: Main API client implementation with:
  - Rate limiting (respects arXiv's 3-second delay requirement)
  - Support for GET and POST methods
  - XML response parsing
  - Iterator pattern for large result sets (using Go 1.23's `iter.Seq`)
- **SearchParams**: Configures search requests (query, IDs, pagination, sorting)
- **SearchResponse**: Contains parsed results with metadata
- **EntryMetadata**: Individual paper data (title, authors, abstract, categories, links)

### CLI Tool (`cmd/arxiv/`)
- Command-line interface for searching arXiv
- Outputs results in tabular format
- Supports all search parameters via flags

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

1. **Rate Limiting**: The library enforces a minimum 3-second delay between API requests to comply with arXiv's usage policy. This is configurable via `Config.RateLimit`.

2. **Testing**: Tests include both unit tests with mock data (in `arxiv/test_data/`) and integration tests that hit the real arXiv API. Be mindful of rate limits when running tests.

3. **Error Handling**: The library returns errors from all operations. Always check errors, especially for network operations.

4. **Iterator Pattern**: The `SearchIterator` function uses Go 1.23's iterator pattern for efficient processing of large result sets without loading all results into memory at once.

5. **XML Parsing**: Response parsing handles the Atom feed format returned by arXiv's API. Test data files in `arxiv/test_data/` show expected XML structure.