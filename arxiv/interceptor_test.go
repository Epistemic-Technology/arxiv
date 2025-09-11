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

func TestWithInterceptor(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/">
  <opensearch:totalResults>10</opensearch:totalResults>
  <opensearch:startIndex>0</opensearch:startIndex>
  <opensearch:itemsPerPage>1</opensearch:itemsPerPage>
  <entry>
    <id>http://arxiv.org/abs/1234.5678v1</id>
    <title>Test</title>
    <summary>Test summary</summary>
  </entry>
</feed>`))
	}))
	defer mockServer.Close()

	t.Run("single interceptor modifying params", func(t *testing.T) {
		var interceptorCalled bool
		interceptor := func(ctx context.Context, params SearchParams, next SearchFunc) (SearchResults, error) {
			interceptorCalled = true
			// Modify params before passing to next
			params.MaxResults = 100
			return next(ctx, params)
		}

		client := NewClient(
			WithBaseURL(mockServer.URL),
			WithHTTPClient(mockServer.Client()),
			WithInterceptor(interceptor),
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "test",
			MaxResults: 10,
		}

		_, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		if !interceptorCalled {
			t.Error("Interceptor was not called")
		}
	})

	t.Run("multiple interceptors chaining", func(t *testing.T) {
		var order []string

		interceptor1 := func(ctx context.Context, params SearchParams, next SearchFunc) (SearchResults, error) {
			order = append(order, "interceptor1-before")
			result, err := next(ctx, params)
			order = append(order, "interceptor1-after")
			return result, err
		}

		interceptor2 := func(ctx context.Context, params SearchParams, next SearchFunc) (SearchResults, error) {
			order = append(order, "interceptor2-before")
			result, err := next(ctx, params)
			order = append(order, "interceptor2-after")
			return result, err
		}

		client := NewClient(
			WithBaseURL(mockServer.URL),
			WithHTTPClient(mockServer.Client()),
			WithInterceptor(interceptor1, interceptor2),
		)

		ctx := context.Background()
		params := SearchParams{Query: "test"}

		_, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		expectedOrder := []string{
			"interceptor1-before",
			"interceptor2-before",
			"interceptor2-after",
			"interceptor1-after",
		}

		if len(order) != len(expectedOrder) {
			t.Fatalf("Expected %d calls, got %d", len(expectedOrder), len(order))
		}

		for i, expected := range expectedOrder {
			if order[i] != expected {
				t.Errorf("Call order[%d] = %s, want %s", i, order[i], expected)
			}
		}
	})

	t.Run("interceptor short-circuiting", func(t *testing.T) {
		var apiCalled atomic.Bool
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiCalled.Store(true)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/">
  <opensearch:totalResults>0</opensearch:totalResults>
</feed>`))
		}))
		defer testServer.Close()

		// Interceptor that returns cached result without calling next
		cacheInterceptor := func(ctx context.Context, params SearchParams, next SearchFunc) (SearchResults, error) {
			// Return cached result, don't call next
			return SearchResults{
				TotalResults: 999,
				Entries: []EntryMetadata{
					{Title: "Cached Result"},
				},
			}, nil
		}

		client := NewClient(
			WithBaseURL(testServer.URL),
			WithHTTPClient(testServer.Client()),
			WithInterceptor(cacheInterceptor),
		)

		ctx := context.Background()
		params := SearchParams{Query: "test"}

		result, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		if apiCalled.Load() {
			t.Error("API was called despite interceptor short-circuiting")
		}

		if result.TotalResults != 999 {
			t.Errorf("Expected cached result with TotalResults=999, got %d", result.TotalResults)
		}

		if len(result.Entries) != 1 || result.Entries[0].Title != "Cached Result" {
			t.Error("Expected cached entry not found")
		}
	})

	t.Run("interceptor error handling", func(t *testing.T) {
		expectedErr := errors.New("interceptor error")

		errorInterceptor := func(ctx context.Context, params SearchParams, next SearchFunc) (SearchResults, error) {
			return SearchResults{}, expectedErr
		}

		client := NewClient(
			WithBaseURL(mockServer.URL),
			WithHTTPClient(mockServer.Client()),
			WithInterceptor(errorInterceptor),
		)

		ctx := context.Background()
		params := SearchParams{Query: "test"}

		_, err := client.Search(ctx, params)
		if err == nil {
			t.Fatal("Expected error from interceptor")
		}

		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("interceptor with timing", func(t *testing.T) {
		var duration time.Duration

		timingInterceptor := func(ctx context.Context, params SearchParams, next SearchFunc) (SearchResults, error) {
			start := time.Now()
			result, err := next(ctx, params)
			duration = time.Since(start)
			return result, err
		}

		client := NewClient(
			WithBaseURL(mockServer.URL),
			WithHTTPClient(mockServer.Client()),
			WithInterceptor(timingInterceptor),
		)

		ctx := context.Background()
		params := SearchParams{Query: "test"}

		_, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		if duration == 0 {
			t.Error("Timing interceptor did not record duration")
		}
	})
}

func TestInterceptorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("real API with logging interceptor", func(t *testing.T) {
		var logEntries []string

		loggingInterceptor := func(ctx context.Context, params SearchParams, next SearchFunc) (SearchResults, error) {
			logEntries = append(logEntries, fmt.Sprintf("Before: Query=%s", params.Query))
			result, err := next(ctx, params)
			if err != nil {
				logEntries = append(logEntries, fmt.Sprintf("Error: %v", err))
			} else {
				logEntries = append(logEntries, fmt.Sprintf("After: Results=%d", result.TotalResults))
			}
			return result, err
		}

		client := NewClient(
			WithInterceptor(loggingInterceptor),
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "quantum computing",
			MaxResults: 5,
		}

		result, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		if len(logEntries) != 2 {
			t.Errorf("Expected 2 log entries, got %d", len(logEntries))
		}

		if result.TotalResults == 0 {
			t.Error("Expected some results for 'quantum computing' query")
		}
	})
}

func TestInterceptorWithRetry(t *testing.T) {
	t.Run("interceptor sees retry attempts", func(t *testing.T) {
		var attemptCount int32
		var interceptorCallCount int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&attemptCount, 1)
			if count < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/">
  <opensearch:totalResults>1</opensearch:totalResults>
</feed>`))
		}))
		defer server.Close()

		countingInterceptor := func(ctx context.Context, params SearchParams, next SearchFunc) (SearchResults, error) {
			atomic.AddInt32(&interceptorCallCount, 1)
			return next(ctx, params)
		}

		client := NewClient(
			WithBaseURL(server.URL),
			WithHTTPClient(server.Client()),
			WithRetry(RetryConfig{
				MaxAttempts:     3,
				InitialInterval: 10 * time.Millisecond,
				MaxInterval:     100 * time.Millisecond,
				Multiplier:      2.0,
			}),
			WithInterceptor(countingInterceptor),
		)

		ctx := context.Background()
		params := SearchParams{Query: "test"}

		_, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		// Interceptor should be called only once (wraps the entire retry logic)
		if interceptorCallCount != 1 {
			t.Errorf("Expected interceptor to be called once, got %d", interceptorCallCount)
		}

		// Server should have been called 3 times (2 failures + 1 success)
		if attemptCount != 3 {
			t.Errorf("Expected 3 server attempts, got %d", attemptCount)
		}
	})
}
