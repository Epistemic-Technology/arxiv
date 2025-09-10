package arxiv

import (
	"testing"
	"time"
)

func TestSearchQuery_BasicFields(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *SearchQuery
		expected string
	}{
		{
			name: "title search",
			builder: func() *SearchQuery {
				return NewSearchQuery().Title("quantum computing")
			},
			expected: "ti:quantum computing",
		},
		{
			name: "abstract search",
			builder: func() *SearchQuery {
				return NewSearchQuery().Abstract("machine learning")
			},
			expected: "abs:machine learning",
		},
		{
			name: "author search",
			builder: func() *SearchQuery {
				return NewSearchQuery().Author("Doe, John")
			},
			expected: "au:Doe, John",
		},
		{
			name: "category search",
			builder: func() *SearchQuery {
				return NewSearchQuery().Category("cs.LG")
			},
			expected: "cat:cs.LG",
		},
		{
			name: "comment search",
			builder: func() *SearchQuery {
				return NewSearchQuery().Comment("10 pages")
			},
			expected: "co:10 pages",
		},
		{
			name: "journal search",
			builder: func() *SearchQuery {
				return NewSearchQuery().Journal("Nature")
			},
			expected: "jr:Nature",
		},
		{
			name: "all fields search",
			builder: func() *SearchQuery {
				return NewSearchQuery().All("electron")
			},
			expected: "all:electron",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := tt.builder()
			result := q.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSearchQuery_Operators(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *SearchQuery
		expected string
	}{
		{
			name: "AND operator",
			builder: func() *SearchQuery {
				return NewSearchQuery().
					Title("quantum").
					And().
					Abstract("computing")
			},
			expected: "ti:quantum AND abs:computing",
		},
		{
			name: "OR operator",
			builder: func() *SearchQuery {
				return NewSearchQuery().
					Title("quantum").
					Or().
					Title("classical")
			},
			expected: "ti:quantum OR ti:classical",
		},
		{
			name: "ANDNOT operator",
			builder: func() *SearchQuery {
				return NewSearchQuery().
					Category("cs.LG").
					AndNot().
					Author("Smith")
			},
			expected: "cat:cs.LG ANDNOT au:Smith",
		},
		{
			name: "multiple operators",
			builder: func() *SearchQuery {
				return NewSearchQuery().
					Title("neural").
					And().
					Abstract("network").
					Or().
					Abstract("graph")
			},
			expected: "ti:neural AND abs:network OR abs:graph",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := tt.builder()
			result := q.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSearchQuery_Groups(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *SearchQuery
		expected string
	}{
		{
			name: "simple group",
			builder: func() *SearchQuery {
				return NewSearchQuery().
					Group(func(g *SearchQuery) {
						g.Title("quantum").Or().Title("classical")
					})
			},
			expected: "(ti:quantum OR ti:classical)",
		},
		{
			name: "group with AND",
			builder: func() *SearchQuery {
				return NewSearchQuery().
					Group(func(g *SearchQuery) {
						g.Title("graph neural networks").Or().Abstract("graph neural networks")
					}).
					And().
					Category("cs.LG")
			},
			expected: "(ti:graph neural networks OR abs:graph neural networks) AND cat:cs.LG",
		},
		{
			name: "complex example from requirement",
			builder: func() *SearchQuery {
				return NewSearchQuery().
					Group(func(g *SearchQuery) {
						g.Title("graph neural networks").Or().Abstract("graph neural networks")
					}).
					And().
					Category("cs.LG").
					AndNot().
					Author("Doe, John")
			},
			expected: "(ti:graph neural networks OR abs:graph neural networks) AND cat:cs.LG ANDNOT au:Doe, John",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := tt.builder()
			result := q.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSearchQuery_DateRanges(t *testing.T) {
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2023, 12, 31, 23, 59, 0, 0, time.UTC)

	tests := []struct {
		name     string
		builder  func() *SearchQuery
		expected string
	}{
		{
			name: "submitted date range",
			builder: func() *SearchQuery {
				return NewSearchQuery().
					SubmittedBetween(start, end)
			},
			expected: "submittedDate:[202301010000 TO 202312312359]",
		},
		{
			name: "last updated date range",
			builder: func() *SearchQuery {
				return NewSearchQuery().
					LastUpdatedBetween(start, end)
			},
			expected: "lastUpdatedDate:[202301010000 TO 202312312359]",
		},
		{
			name: "query with date range",
			builder: func() *SearchQuery {
				return NewSearchQuery().
					Category("cs.LG").
					SubmittedBetween(start, end)
			},
			expected: "cat:cs.LG AND submittedDate:[202301010000 TO 202312312359]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := tt.builder()
			result := q.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSearchQuery_ComplexExample(t *testing.T) {
	// Test the exact example from the requirement
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 23, 59, 0, 0, time.UTC)

	q := NewSearchQuery().
		Group(func(g *SearchQuery) {
			g.Title("graph neural networks").Or().Abstract("graph neural networks")
		}).
		And().
		Category("cs.LG").
		AndNot().
		Author("Doe, John").
		SubmittedBetween(start, end)

	expected := "(ti:graph neural networks OR abs:graph neural networks) AND cat:cs.LG ANDNOT au:Doe, John AND submittedDate:[202301010000 TO 202412312359]"
	result := q.String()

	if result != expected {
		t.Errorf("Complex example failed:\nexpected: %q\ngot:      %q", expected, result)
	}
}

func TestSearchQuery_EmptyQuery(t *testing.T) {
	q := NewSearchQuery()
	result := q.String()
	if result != "" {
		t.Errorf("expected empty string for empty query, got %q", result)
	}
}

func TestSearchQuery_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *SearchQuery
		expected string
	}{
		{
			name: "quotes in title",
			builder: func() *SearchQuery {
				return NewSearchQuery().Title(`"quantum computing"`)
			},
			expected: "ti:\"quantum computing\"",
		},
		{
			name: "ampersand in author",
			builder: func() *SearchQuery {
				return NewSearchQuery().Author("Smith & Jones")
			},
			expected: "au:Smith & Jones",
		},
		{
			name: "parentheses in abstract",
			builder: func() *SearchQuery {
				return NewSearchQuery().Abstract("f(x) = x^2")
			},
			expected: "abs:f(x) = x^2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := tt.builder()
			result := q.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParseSearchQuery_BasicFields(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, q *SearchQuery)
	}{
		{
			name:  "parse title",
			input: "ti:quantum computing",
			validate: func(t *testing.T, q *SearchQuery) {
				expected := "ti:quantum computing"
				if q.String() != expected {
					t.Errorf("expected %q, got %q", expected, q.String())
				}
			},
		},
		{
			name:  "parse with AND",
			input: "ti:quantum AND abs:computing",
			validate: func(t *testing.T, q *SearchQuery) {
				expected := "ti:quantum AND abs:computing"
				if q.String() != expected {
					t.Errorf("expected %q, got %q", expected, q.String())
				}
			},
		},
		{
			name:  "parse with OR",
			input: "cat:cs.LG OR cat:cs.AI",
			validate: func(t *testing.T, q *SearchQuery) {
				expected := "cat:cs.LG OR cat:cs.AI"
				if q.String() != expected {
					t.Errorf("expected %q, got %q", expected, q.String())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := ParseSearchQuery(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, q)
		})
	}
}

func TestSearchQuery_ChainedOperations(t *testing.T) {
	// Test that we can chain multiple operations fluently
	q := NewSearchQuery().
		Title("machine learning").
		And().
		Group(func(g *SearchQuery) {
			g.Category("cs.LG").Or().Category("cs.AI")
		}).
		AndNot().
		Author("Anonymous").
		And().
		Journal("Nature")

	expected := "ti:machine learning AND (cat:cs.LG OR cat:cs.AI) ANDNOT au:Anonymous AND jr:Nature"
	result := q.String()

	if result != expected {
		t.Errorf("Chained operations failed:\nexpected: %q\ngot:      %q", expected, result)
	}
}

func TestSearchQuery_MultipleGroups(t *testing.T) {
	q := NewSearchQuery().
		Group(func(g *SearchQuery) {
			g.Title("quantum").Or().Title("classical")
		}).
		And().
		Group(func(g *SearchQuery) {
			g.Category("physics.quant-ph").Or().Category("physics.class-ph")
		})

	expected := "(ti:quantum OR ti:classical) AND (cat:physics.quant-ph OR cat:physics.class-ph)"
	result := q.String()

	if result != expected {
		t.Errorf("Multiple groups failed:\nexpected: %q\ngot:      %q", expected, result)
	}
}

func TestSearchQuery_NestedGroups(t *testing.T) {
	// While the current implementation doesn't support nested groups directly,
	// we can test that groups work correctly when building complex queries
	q := NewSearchQuery().
		All("neural network").
		And().
		Group(func(g *SearchQuery) {
			g.Category("cs.LG").
				Or().
				Category("cs.AI").
				Or().
				Category("cs.NE")
		}).
		AndNot().
		Title("survey")

	expected := "all:neural network AND (cat:cs.LG OR cat:cs.AI OR cat:cs.NE) ANDNOT ti:survey"
	result := q.String()

	if result != expected {
		t.Errorf("Complex grouped query failed:\nexpected: %q\ngot:      %q", expected, result)
	}
}
