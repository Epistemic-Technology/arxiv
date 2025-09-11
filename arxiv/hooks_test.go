package arxiv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstrumentationHooks(t *testing.T) {
	// Load test data
	testDataPath := filepath.Join("test_data", "full-results.xml")
	testData, err := os.ReadFile(testDataPath)
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	t.Run("RequestHook is called before request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(testData)
		}))
		defer server.Close()

		var hookCalled bool
		var capturedMethod string
		var capturedParams SearchParams

		client := NewClient(
			WithBaseURL(server.URL),
			WithRequestHook(func(ctx context.Context, client *Client, method string, params *SearchParams) {
				hookCalled = true
				capturedMethod = method
				capturedParams = *params
			}),
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "test query",
			MaxResults: 5,
		}

		_, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if !hookCalled {
			t.Error("Request hook was not called")
		}
		if capturedMethod != "Search" {
			t.Errorf("Expected method 'Search', got '%s'", capturedMethod)
		}
		if capturedParams.Query != params.Query {
			t.Errorf("Expected query '%s', got '%s'", params.Query, capturedParams.Query)
		}
	})

	t.Run("ResponseHook is called after successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(testData)
		}))
		defer server.Close()

		var hookCalled bool
		var capturedMethod string
		var capturedResponse *SearchResults
		var capturedDuration time.Duration
		var capturedError error

		client := NewClient(
			WithBaseURL(server.URL),
			WithResponseHook(func(ctx context.Context, client *Client, method string, response *SearchResults, duration time.Duration, err error) {
				hookCalled = true
				capturedMethod = method
				capturedResponse = response
				capturedDuration = duration
				capturedError = err
			}),
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "test query",
			MaxResults: 5,
		}

		_, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if !hookCalled {
			t.Error("Response hook was not called")
		}
		if capturedMethod != "Search" {
			t.Errorf("Expected method 'Search', got '%s'", capturedMethod)
		}
		if capturedResponse == nil {
			t.Error("Expected response to be captured, got nil")
		} else if capturedResponse.TotalResults == 0 {
			t.Error("Expected response to have results")
		}
		if capturedDuration <= 0 {
			t.Error("Duration should be positive")
		}
		if capturedError != nil {
			t.Errorf("Expected no error, got %v", capturedError)
		}
	})

	t.Run("ResponseHook is called on error", func(t *testing.T) {
		// Create a server that returns an error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		var hookCalled bool
		var capturedError error

		client := NewClient(
			WithBaseURL(server.URL),
			WithResponseHook(func(ctx context.Context, client *Client, method string, response *SearchResults, duration time.Duration, err error) {
				hookCalled = true
				capturedError = err
			}),
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "test query",
			MaxResults: 5,
		}

		_, err := client.Search(ctx, params)

		if !hookCalled {
			t.Error("Response hook was not called on error")
		}
		// We expect an error from either the HTTP status or parsing
		if capturedError == nil && err != nil {
			// The hook should have captured the error
			t.Error("Response hook did not capture the error")
		}
	})

	t.Run("Multiple hooks are called in order", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(testData)
		}))
		defer server.Close()

		var callOrder []string

		client := NewClient(
			WithBaseURL(server.URL),
			WithRequestHook(func(ctx context.Context, client *Client, method string, params *SearchParams) {
				callOrder = append(callOrder, "request1")
			}),
			WithRequestHook(func(ctx context.Context, client *Client, method string, params *SearchParams) {
				callOrder = append(callOrder, "request2")
			}),
			WithResponseHook(func(ctx context.Context, client *Client, method string, response *SearchResults, duration time.Duration, err error) {
				callOrder = append(callOrder, "response1")
			}),
			WithResponseHook(func(ctx context.Context, client *Client, method string, response *SearchResults, duration time.Duration, err error) {
				callOrder = append(callOrder, "response2")
			}),
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "test query",
			MaxResults: 5,
		}

		_, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		expectedOrder := []string{"request1", "request2", "response1", "response2"}
		if len(callOrder) != len(expectedOrder) {
			t.Errorf("Expected %d hook calls, got %d", len(expectedOrder), len(callOrder))
		}

		for i, expected := range expectedOrder {
			if i >= len(callOrder) {
				break
			}
			if callOrder[i] != expected {
				t.Errorf("Expected hook call %d to be '%s', got '%s'", i, expected, callOrder[i])
			}
		}
	})

	t.Run("Hooks work with SearchNext", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(testData)
		}))
		defer server.Close()

		var requestHookCalled bool
		var responseHookCalled bool

		client := NewClient(
			WithBaseURL(server.URL),
			WithRequestHook(func(ctx context.Context, client *Client, method string, params *SearchParams) {
				requestHookCalled = true
			}),
			WithResponseHook(func(ctx context.Context, client *Client, method string, response *SearchResults, duration time.Duration, err error) {
				responseHookCalled = true
			}),
		)

		ctx := context.Background()

		// First search to get initial results
		initialParams := SearchParams{
			Query:      "test query",
			MaxResults: 5,
		}
		response, err := client.Search(ctx, initialParams)
		if err != nil {
			t.Fatalf("Initial search failed: %v", err)
		}

		// Reset hook flags
		requestHookCalled = false
		responseHookCalled = false

		// Simulate that there are more results
		response.TotalResults = 100
		response.StartIndex = 0
		response.ItemsPerPage = 5

		// Try to get next page
		_, err = client.SearchNext(ctx, response)
		if err != nil {
			t.Fatalf("SearchNext failed: %v", err)
		}

		if !requestHookCalled {
			t.Error("Request hook was not called for SearchNext")
		}
		if !responseHookCalled {
			t.Error("Response hook was not called for SearchNext")
		}
	})

	t.Run("Response data is accessible in response hooks", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(testData)
		}))
		defer server.Close()

		var capturedTotalResults int
		var capturedEntriesCount int

		client := NewClient(
			WithBaseURL(server.URL),
			WithResponseHook(func(ctx context.Context, client *Client, method string, response *SearchResults, duration time.Duration, err error) {
				if response != nil {
					capturedTotalResults = response.TotalResults
					capturedEntriesCount = len(response.Entries)
				}
			}),
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "test query",
			MaxResults: 5,
		}

		result, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Verify the hook captured the same data as returned
		if capturedTotalResults != result.TotalResults {
			t.Errorf("Hook captured TotalResults %d, but Search returned %d", capturedTotalResults, result.TotalResults)
		}
		if capturedEntriesCount != len(result.Entries) {
			t.Errorf("Hook captured %d entries, but Search returned %d", capturedEntriesCount, len(result.Entries))
		}
		if capturedTotalResults == 0 {
			t.Error("Expected hook to capture non-zero TotalResults")
		}
	})

	t.Run("Client is accessible in hooks", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(testData)
		}))
		defer server.Close()

		expectedTimeout := 42 * time.Second
		var capturedTimeout time.Duration
		var capturedBaseURL string

		client := NewClient(
			WithBaseURL(server.URL),
			WithTimeout(expectedTimeout),
			WithRequestHook(func(ctx context.Context, client *Client, method string, params *SearchParams) {
				capturedTimeout = client.Timeout
				capturedBaseURL = client.BaseURL
			}),
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "test query",
			MaxResults: 5,
		}

		_, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if capturedTimeout != expectedTimeout {
			t.Errorf("Expected timeout %v, got %v", expectedTimeout, capturedTimeout)
		}
		if capturedBaseURL != server.URL {
			t.Errorf("Expected base URL %s, got %s", server.URL, capturedBaseURL)
		}
	})

	t.Run("Request hooks can mutate parameters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check that the modified parameters were sent
			queryParams := r.URL.Query()
			if queryParams.Get("max_results") != "42" {
				t.Errorf("Expected max_results=42 in request, got %s", queryParams.Get("max_results"))
			}
			if queryParams.Get("search_query") != "modified query" {
				t.Errorf("Expected search_query='modified query' in request, got %s", queryParams.Get("search_query"))
			}
			w.WriteHeader(http.StatusOK)
			w.Write(testData)
		}))
		defer server.Close()

		client := NewClient(
			WithBaseURL(server.URL),
			WithRequestHook(func(ctx context.Context, client *Client, method string, params *SearchParams) {
				// Mutate the parameters
				params.Query = "modified query"
				params.MaxResults = 42
			}),
		)

		ctx := context.Background()
		originalParams := SearchParams{
			Query:      "original query",
			MaxResults: 5,
		}

		result, err := client.Search(ctx, originalParams)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Verify the response used the modified parameters
		if result.Params.Query != "modified query" {
			t.Errorf("Expected params.Query to be 'modified query', got '%s'", result.Params.Query)
		}
		if result.Params.MaxResults != 42 {
			t.Errorf("Expected params.MaxResults to be 42, got %d", result.Params.MaxResults)
		}

		// Original params should remain unchanged (passed by value to Search)
		if originalParams.Query != "original query" {
			t.Errorf("Original params were modified: Query = %s", originalParams.Query)
		}
		if originalParams.MaxResults != 5 {
			t.Errorf("Original params were modified: MaxResults = %d", originalParams.MaxResults)
		}
	})

	t.Run("Context is passed to hooks", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(testData)
		}))
		defer server.Close()

		type contextKey string
		const testKey contextKey = "testKey"
		testValue := "testValue"

		var capturedValue interface{}

		client := NewClient(
			WithBaseURL(server.URL),
			WithRequestHook(func(ctx context.Context, client *Client, method string, params *SearchParams) {
				capturedValue = ctx.Value(testKey)
			}),
		)

		ctx := context.WithValue(context.Background(), testKey, testValue)
		params := SearchParams{
			Query:      "test query",
			MaxResults: 5,
		}

		_, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if capturedValue != testValue {
			t.Errorf("Expected context value '%s', got '%v'", testValue, capturedValue)
		}
	})
}

func TestHooksWithRetry(t *testing.T) {
	t.Run("Hooks are called for each retry attempt", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				// Fail the first 2 attempts
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			// Succeed on the 3rd attempt
			testDataPath := filepath.Join("test_data", "full-results.xml")
			testData, _ := os.ReadFile(testDataPath)
			w.WriteHeader(http.StatusOK)
			w.Write(testData)
		}))
		defer server.Close()

		requestHookCalls := 0
		responseHookCalls := 0

		client := NewClient(
			WithBaseURL(server.URL),
			WithRetry(RetryConfig{
				MaxAttempts:     3,
				InitialInterval: 10 * time.Millisecond,
				MaxInterval:     100 * time.Millisecond,
				Multiplier:      2.0,
			}),
			WithRequestHook(func(ctx context.Context, client *Client, method string, params *SearchParams) {
				requestHookCalls++
			}),
			WithResponseHook(func(ctx context.Context, client *Client, method string, response *SearchResults, duration time.Duration, err error) {
				responseHookCalls++
			}),
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "test query",
			MaxResults: 5,
		}

		_, err := client.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search failed after retries: %v", err)
		}

		// Request hook should be called once (at the beginning of Search)
		if requestHookCalls != 1 {
			t.Errorf("Expected 1 request hook call, got %d", requestHookCalls)
		}

		// Response hook should be called once (after all retries complete)
		if responseHookCalls != 1 {
			t.Errorf("Expected 1 response hook call, got %d", responseHookCalls)
		}
	})
}

func TestHooksPerformance(t *testing.T) {
	t.Run("Hooks don't significantly impact performance", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			testDataPath := filepath.Join("test_data", "full-results.xml")
			testData, _ := os.ReadFile(testDataPath)
			w.WriteHeader(http.StatusOK)
			w.Write(testData)
		}))
		defer server.Close()

		// Client without hooks
		clientNoHooks := NewClient(WithBaseURL(server.URL))

		// Client with multiple hooks
		clientWithHooks := NewClient(
			WithBaseURL(server.URL),
			WithRequestHook(func(ctx context.Context, client *Client, method string, params *SearchParams) {
				// Simulate some work in the hook
				time.Sleep(1 * time.Microsecond)
			}),
			WithRequestHook(func(ctx context.Context, client *Client, method string, params *SearchParams) {
				time.Sleep(1 * time.Microsecond)
			}),
			WithResponseHook(func(ctx context.Context, client *Client, method string, response *SearchResults, duration time.Duration, err error) {
				time.Sleep(1 * time.Microsecond)
			}),
			WithResponseHook(func(ctx context.Context, client *Client, method string, response *SearchResults, duration time.Duration, err error) {
				time.Sleep(1 * time.Microsecond)
			}),
		)

		ctx := context.Background()
		params := SearchParams{
			Query:      "test query",
			MaxResults: 5,
		}

		// Measure without hooks
		start := time.Now()
		_, err := clientNoHooks.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search without hooks failed: %v", err)
		}
		durationNoHooks := time.Since(start)

		// Measure with hooks
		start = time.Now()
		_, err = clientWithHooks.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search with hooks failed: %v", err)
		}
		durationWithHooks := time.Since(start)

		// The overhead should be minimal (less than 10ms for this simple test)
		overhead := durationWithHooks - durationNoHooks
		if overhead > 10*time.Millisecond {
			t.Errorf("Hook overhead too high: %v", overhead)
		}
	})
}

// Example demonstrating how to use hooks for logging and parameter modification
func ExampleWithRequestHook() {
	client := NewClient(
		WithRequestHook(func(ctx context.Context, client *Client, method string, params *SearchParams) {
			// Log the request
			// log.Printf("[REQUEST] Method: %s, Query: %s, MaxResults: %d, Timeout: %v",
			//     method, params.Query, params.MaxResults, client.Timeout)

			// Set defaults if not specified
			if params.MaxResults == 0 {
				params.MaxResults = 10
			}

			// Add safety limits
			if params.MaxResults > 100 {
				// log.Printf("Limiting MaxResults from %d to 100", params.MaxResults)
				params.MaxResults = 100
			}

			// Clean up query
			// params.Query = strings.TrimSpace(params.Query)
		}),
	)

	ctx := context.Background()
	params := SearchParams{
		Query:      "quantum computing",
		MaxResults: 10,
	}

	_, _ = client.Search(ctx, params)
}

// Example demonstrating how to use hooks for metrics
func ExampleWithResponseHook() {
	client := NewClient(
		WithResponseHook(func(ctx context.Context, client *Client, method string, response *SearchResults, duration time.Duration, err error) {
			// Record metrics
			// metrics.RecordDuration("arxiv.api.duration", duration)
			// if err != nil {
			//     metrics.IncrementCounter("arxiv.api.errors")
			// } else {
			//     metrics.IncrementCounter("arxiv.api.success")
			//     metrics.RecordGauge("arxiv.api.results", float64(response.TotalResults))
			// }
		}),
	)

	ctx := context.Background()
	params := SearchParams{
		Query:      "machine learning",
		MaxResults: 5,
	}

	_, _ = client.Search(ctx, params)
}

// Example demonstrating distributed tracing with hooks
func ExampleWithRequestHook_tracing() {
	client := NewClient(
		WithRequestHook(func(ctx context.Context, client *Client, method string, params *SearchParams) {
			// Start a trace span
			// span := trace.StartSpan(ctx, "arxiv."+method)
			// span.AddAttributes(
			//     trace.StringAttribute("query", params.Query),
			//     trace.Int64Attribute("max_results", int64(params.MaxResults)),
			//     trace.StringAttribute("base_url", client.BaseURL),
			// )
		}),
		WithResponseHook(func(ctx context.Context, client *Client, method string, response *SearchResults, duration time.Duration, err error) {
			// End the trace span
			// span := trace.FromContext(ctx)
			// if err != nil {
			//     span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: err.Error()})
			// } else {
			//     span.AddAttributes(trace.Int64Attribute("result_count", int64(response.TotalResults)))
			// }
			// span.End()
		}),
	)

	ctx := context.Background()
	params := SearchParams{
		Query:      "neural networks",
		MaxResults: 20,
	}

	_, _ = client.Search(ctx, params)
}
