package arxiv

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type queryOperator string

const (
	opAnd    queryOperator = "AND"
	opOr     queryOperator = "OR"
	opAndNot queryOperator = "ANDNOT"
)

type queryField string

const (
	fieldTitle    queryField = "ti"
	fieldAbstract queryField = "abs"
	fieldAuthor   queryField = "au"
	fieldCategory queryField = "cat"
	fieldComment  queryField = "co"
	fieldJournal  queryField = "jr"
	fieldAll      queryField = "all"
)

type queryNode interface {
	encode() string
}

type fieldQuery struct {
	field queryField
	value string
}

func (f *fieldQuery) encode() string {
	// Don't URL encode here - it will be encoded by url.Values.Encode() in makeGetQuery
	return fmt.Sprintf("%s:%s", f.field, f.value)
}

type groupQuery struct {
	nodes []queryNode
}

func (g *groupQuery) encode() string {
	var parts []string
	for _, node := range g.nodes {
		parts = append(parts, node.encode())
	}
	return "(" + strings.Join(parts, " ") + ")"
}

type operatorNode struct {
	op queryOperator
}

func (o *operatorNode) encode() string {
	return string(o.op)
}

type dateRangeQuery struct {
	field     string
	startDate time.Time
	endDate   time.Time
}

func (d *dateRangeQuery) encode() string {
	start := d.startDate.Format("200601021504")
	end := d.endDate.Format("200601021504")
	// Use spaces instead of + since url.Values.Encode() will handle the encoding
	return fmt.Sprintf("%s:[%s TO %s]", d.field, start, end)
}

// SearchQuery represents a search query for the arXiv API.
type SearchQuery struct {
	nodes []queryNode
}

// NewSearchQuery creates a new SearchQuery builder.
func NewSearchQuery() *SearchQuery {
	return &SearchQuery{
		nodes: []queryNode{},
	}
}

// Title adds a title search term.
func (q *SearchQuery) Title(value string) *SearchQuery {
	q.nodes = append(q.nodes, &fieldQuery{field: fieldTitle, value: value})
	return q
}

// Abstract adds an abstract search term.
func (q *SearchQuery) Abstract(value string) *SearchQuery {
	q.nodes = append(q.nodes, &fieldQuery{field: fieldAbstract, value: value})
	return q
}

// Author adds an author search term.
func (q *SearchQuery) Author(value string) *SearchQuery {
	q.nodes = append(q.nodes, &fieldQuery{field: fieldAuthor, value: value})
	return q
}

// Category adds a category search term.
func (q *SearchQuery) Category(value string) *SearchQuery {
	q.nodes = append(q.nodes, &fieldQuery{field: fieldCategory, value: value})
	return q
}

// Comment adds a comment search term.
func (q *SearchQuery) Comment(value string) *SearchQuery {
	q.nodes = append(q.nodes, &fieldQuery{field: fieldComment, value: value})
	return q
}

// Journal adds a journal reference search term.
func (q *SearchQuery) Journal(value string) *SearchQuery {
	q.nodes = append(q.nodes, &fieldQuery{field: fieldJournal, value: value})
	return q
}

// All adds a search term that searches all fields.
func (q *SearchQuery) All(value string) *SearchQuery {
	q.nodes = append(q.nodes, &fieldQuery{field: fieldAll, value: value})
	return q
}

// And adds an AND operator.
func (q *SearchQuery) And() *SearchQuery {
	if len(q.nodes) > 0 {
		q.nodes = append(q.nodes, &operatorNode{op: opAnd})
	}
	return q
}

// Or adds an OR operator.
func (q *SearchQuery) Or() *SearchQuery {
	if len(q.nodes) > 0 {
		q.nodes = append(q.nodes, &operatorNode{op: opOr})
	}
	return q
}

// AndNot adds an ANDNOT operator.
func (q *SearchQuery) AndNot() *SearchQuery {
	if len(q.nodes) > 0 {
		q.nodes = append(q.nodes, &operatorNode{op: opAndNot})
	}
	return q
}

// Group adds a grouped sub-query.
func (q *SearchQuery) Group(fn func(g *SearchQuery)) *SearchQuery {
	group := NewSearchQuery()
	fn(group)
	if len(group.nodes) > 0 {
		q.nodes = append(q.nodes, &groupQuery{nodes: group.nodes})
	}
	return q
}

// SubmittedBetween adds a date range query for submission date.
func (q *SearchQuery) SubmittedBetween(start, end time.Time) *SearchQuery {
	if len(q.nodes) > 0 {
		q.nodes = append(q.nodes, &operatorNode{op: opAnd})
	}
	q.nodes = append(q.nodes, &dateRangeQuery{
		field:     "submittedDate",
		startDate: start,
		endDate:   end,
	})
	return q
}

// LastUpdatedBetween adds a date range query for last update date.
func (q *SearchQuery) LastUpdatedBetween(start, end time.Time) *SearchQuery {
	if len(q.nodes) > 0 {
		q.nodes = append(q.nodes, &operatorNode{op: opAnd})
	}
	q.nodes = append(q.nodes, &dateRangeQuery{
		field:     "lastUpdatedDate",
		startDate: start,
		endDate:   end,
	})
	return q
}

// String encodes the SearchQuery to a string suitable for the arXiv API.
func (q *SearchQuery) String() string {
	var parts []string
	for _, node := range q.nodes {
		parts = append(parts, node.encode())
	}
	return strings.Join(parts, " ")
}

// ParseSearchQuery parses a search query string into a SearchQuery.
// This is a basic implementation that handles simple cases.
func ParseSearchQuery(query string) (*SearchQuery, error) {
	q := NewSearchQuery()
	if query == "" {
		return q, nil
	}

	// This is a simplified parser. A full implementation would need
	// a proper tokenizer and parser to handle all cases correctly.
	// For now, we'll handle basic field:value patterns and operators.

	tokens := tokenizeQuery(query)
	for i := range len(tokens) {
		token := tokens[i]

		switch strings.ToUpper(token) {
		case "AND":
			q.And()
		case "OR":
			q.Or()
		case "ANDNOT":
			q.AndNot()
		default:
			// Check if it's a field:value pattern
			if strings.Contains(token, ":") {
				parts := strings.SplitN(token, ":", 2)
				if len(parts) == 2 {
					field := parts[0]
					value, _ := url.QueryUnescape(parts[1])

					switch field {
					case "ti":
						q.Title(value)
					case "abs":
						q.Abstract(value)
					case "au":
						q.Author(value)
					case "cat":
						q.Category(value)
					case "co":
						q.Comment(value)
					case "jr":
						q.Journal(value)
					case "all":
						q.All(value)
					case "submittedDate", "lastUpdatedDate":
						// Date range parsing would go here
						// For now, we'll skip complex date parsing
					default:
						// Unknown field, add as-is
					}
				}
			}
		}
	}

	return q, nil
}

// tokenizeQuery splits a query string into tokens.
func tokenizeQuery(query string) []string {
	var tokens []string
	var current strings.Builder
	inParens := 0
	inBrackets := false

	for i := 0; i < len(query); i++ {
		ch := query[i]

		switch ch {
		case '(':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			inParens++
		case ')':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			inParens--
		case '[':
			inBrackets = true
			current.WriteByte(ch)
		case ']':
			inBrackets = false
			current.WriteByte(ch)
		case ' ':
			if inParens > 0 || inBrackets {
				current.WriteByte(ch)
			} else if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}
