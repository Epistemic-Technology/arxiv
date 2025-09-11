package arxiv

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		response *http.Response
		want     bool
	}{
		{
			name:     "nil error and response",
			err:      nil,
			response: nil,
			want:     false,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			response: nil,
			want:     true,
		},
		{
			name:     "429 Too Many Requests",
			err:      nil,
			response: &http.Response{StatusCode: http.StatusTooManyRequests},
			want:     true,
		},
		{
			name:     "500 Internal Server Error",
			err:      nil,
			response: &http.Response{StatusCode: http.StatusInternalServerError},
			want:     true,
		},
		{
			name:     "502 Bad Gateway",
			err:      nil,
			response: &http.Response{StatusCode: http.StatusBadGateway},
			want:     true,
		},
		{
			name:     "503 Service Unavailable",
			err:      nil,
			response: &http.Response{StatusCode: http.StatusServiceUnavailable},
			want:     true,
		},
		{
			name:     "504 Gateway Timeout",
			err:      nil,
			response: &http.Response{StatusCode: http.StatusGatewayTimeout},
			want:     true,
		},
		{
			name:     "408 Request Timeout",
			err:      nil,
			response: &http.Response{StatusCode: http.StatusRequestTimeout},
			want:     true,
		},
		{
			name:     "200 OK",
			err:      nil,
			response: &http.Response{StatusCode: http.StatusOK},
			want:     false,
		},
		{
			name:     "404 Not Found",
			err:      nil,
			response: &http.Response{StatusCode: http.StatusNotFound},
			want:     false,
		},
		{
			name:     "400 Bad Request",
			err:      nil,
			response: &http.Response{StatusCode: http.StatusBadRequest},
			want:     false,
		},
		{
			name:     "non-retryable error",
			err:      errors.New("some error"),
			response: nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableError(tt.err, tt.response)
			if got != tt.want {
				t.Errorf("isRetryableError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name    string
		attempt int
		config  *RetryConfig
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name:    "nil config",
			attempt: 1,
			config:  nil,
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "attempt 0",
			attempt: 0,
			config: &RetryConfig{
				InitialInterval: 1 * time.Second,
				MaxInterval:     30 * time.Second,
				Multiplier:      2.0,
			},
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "first attempt",
			attempt: 1,
			config: &RetryConfig{
				InitialInterval: 1 * time.Second,
				MaxInterval:     30 * time.Second,
				Multiplier:      2.0,
			},
			wantMin: 900 * time.Millisecond,  // 1s - 10% jitter
			wantMax: 1100 * time.Millisecond, // 1s + 10% jitter
		},
		{
			name:    "second attempt",
			attempt: 2,
			config: &RetryConfig{
				InitialInterval: 1 * time.Second,
				MaxInterval:     30 * time.Second,
				Multiplier:      2.0,
			},
			wantMin: 1800 * time.Millisecond, // 2s - 10% jitter
			wantMax: 2200 * time.Millisecond, // 2s + 10% jitter
		},
		{
			name:    "third attempt",
			attempt: 3,
			config: &RetryConfig{
				InitialInterval: 1 * time.Second,
				MaxInterval:     30 * time.Second,
				Multiplier:      2.0,
			},
			wantMin: 3600 * time.Millisecond, // 4s - 10% jitter
			wantMax: 4400 * time.Millisecond, // 4s + 10% jitter
		},
		{
			name:    "capped at max interval",
			attempt: 10,
			config: &RetryConfig{
				InitialInterval: 1 * time.Second,
				MaxInterval:     10 * time.Second,
				Multiplier:      2.0,
			},
			wantMin: 9 * time.Second,  // 10s - 10% jitter
			wantMax: 11 * time.Second, // 10s + 10% jitter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to account for jitter randomness
			for i := 0; i < 10; i++ {
				got := calculateBackoff(tt.attempt, tt.config)
				if got < tt.wantMin || got > tt.wantMax {
					t.Errorf("calculateBackoff() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
				}
			}
		})
	}
}

func TestClientWithRetry(t *testing.T) {
	t.Run("successful after retries", func(t *testing.T) {
		var attemptCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&attemptCount, 1)
			if count < 3 {
				// Fail first 2 attempts
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			// Succeed on third attempt
			w.Header().Set("Content-Type", "application/atom+xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, testXMLResponse)
		}))
		defer server.Close()

		client := NewClient(
			WithBaseURL(server.URL),
			WithRetry(RetryConfig{
				MaxAttempts:     3,
				InitialInterval: 10 * time.Millisecond,
				MaxInterval:     100 * time.Millisecond,
				Multiplier:      2.0,
			}),
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "test",
			MaxResults: 1,
		}

		response, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		if atomic.LoadInt32(&attemptCount) != 3 {
			t.Errorf("Expected 3 attempts, got %d", attemptCount)
		}

		if len(response.Entries) != 1 {
			t.Errorf("Expected 1 entry, got %d", len(response.Entries))
		}
	})

	t.Run("max attempts exceeded", func(t *testing.T) {
		var attemptCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attemptCount, 1)
			// Always fail
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := NewClient(
			WithBaseURL(server.URL),
			WithRetry(RetryConfig{
				MaxAttempts:     2,
				InitialInterval: 10 * time.Millisecond,
				MaxInterval:     100 * time.Millisecond,
				Multiplier:      2.0,
			}),
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "test",
			MaxResults: 1,
		}

		_, err := client.Search(ctx, params)
		if err == nil {
			t.Error("Expected error when max attempts exceeded")
		}

		if atomic.LoadInt32(&attemptCount) != 2 {
			t.Errorf("Expected 2 attempts, got %d", attemptCount)
		}
	})

	t.Run("non-retryable error", func(t *testing.T) {
		var attemptCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attemptCount, 1)
			// Return non-retryable error
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		client := NewClient(
			WithBaseURL(server.URL),
			WithRetry(RetryConfig{
				MaxAttempts:     3,
				InitialInterval: 10 * time.Millisecond,
				MaxInterval:     100 * time.Millisecond,
				Multiplier:      2.0,
			}),
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "test",
			MaxResults: 1,
		}

		_, err := client.Search(ctx, params)
		if err == nil {
			t.Error("Expected error for bad request")
		}

		if atomic.LoadInt32(&attemptCount) != 1 {
			t.Errorf("Expected 1 attempt (no retry for bad request), got %d", attemptCount)
		}
	})

	t.Run("context cancellation during retry", func(t *testing.T) {
		var attemptCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attemptCount, 1)
			// Always fail to trigger retry
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := NewClient(
			WithBaseURL(server.URL),
			WithRetry(RetryConfig{
				MaxAttempts:     5,
				InitialInterval: 100 * time.Millisecond,
				MaxInterval:     1 * time.Second,
				Multiplier:      2.0,
			}),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		params := SearchParams{
			Query:      "test",
			MaxResults: 1,
		}

		_, err := client.Search(ctx, params)
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context deadline exceeded error, got %v", err)
		}

		// Should have attempted at least once but not all 5 times
		attempts := atomic.LoadInt32(&attemptCount)
		if attempts < 1 || attempts >= 5 {
			t.Errorf("Expected 1-2 attempts before context cancellation, got %d", attempts)
		}
	})

	t.Run("no retry when RetryConfig not set", func(t *testing.T) {
		var attemptCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attemptCount, 1)
			// Always fail
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := NewClient(
			WithBaseURL(server.URL),
			// No retry config
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "test",
			MaxResults: 1,
		}

		_, err := client.Search(ctx, params)
		if err == nil {
			t.Error("Expected error for service unavailable")
		}

		if atomic.LoadInt32(&attemptCount) != 1 {
			t.Errorf("Expected 1 attempt (no retry), got %d", attemptCount)
		}
	})
}

const testXMLResponse = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <link href="http://arxiv.org/api/query?search_query=test&amp;id_list=&amp;max_results=1" rel="self" type="application/atom+xml"/>
  <title type="html">ArXiv Query: search_query=test&amp;id_list=&amp;max_results=1</title>
  <id>http://arxiv.org/api/test</id>
  <updated>2024-01-01T00:00:00-05:00</updated>
  <opensearch:totalResults xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/">1</opensearch:totalResults>
  <opensearch:startIndex xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/">0</opensearch:startIndex>
  <opensearch:itemsPerPage xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/">1</opensearch:itemsPerPage>
  <entry>
    <id>http://arxiv.org/abs/1234.5678v1</id>
    <updated>2024-01-01T00:00:00Z</updated>
    <published>2024-01-01T00:00:00Z</published>
    <title>Test Article</title>
    <summary>This is a test summary.</summary>
    <author>
      <name>Test Author</name>
    </author>
    <arxiv:comment xmlns:arxiv="http://arxiv.org/schemas/atom">Test comment</arxiv:comment>
    <link href="http://arxiv.org/abs/1234.5678v1" rel="alternate" type="text/html"/>
    <link title="pdf" href="http://arxiv.org/pdf/1234.5678v1" rel="related" type="application/pdf"/>
    <arxiv:primary_category xmlns:arxiv="http://arxiv.org/schemas/atom" term="cs.AI" scheme="http://arxiv.org/schemas/atom"/>
    <category term="cs.AI" scheme="http://arxiv.org/schemas/atom"/>
  </entry>
</feed>`
