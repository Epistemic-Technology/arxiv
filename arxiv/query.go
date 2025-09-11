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
	fieldTitle         queryField = "ti"
	fieldAbstract      queryField = "abs"
	fieldAuthor        queryField = "au"
	fieldCategory      queryField = "cat"
	fieldComment       queryField = "co"
	fieldJournal       queryField = "jr"
	fieldAll           queryField = "all"
	fieldSubmittedDate queryField = "submittedDate"
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
	field     queryField
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
		field:     fieldSubmittedDate,
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
// The query string should not use special characters such as + for
// spaces or % encoded characters.
func ParseSearchQuery(query string) (*SearchQuery, error) {
	q := NewSearchQuery()
	if query == "" {
		return q, nil
	}

	tokens := tokenizeQuery(query)
	for i, token := range tokens {
		switch queryOperator(strings.ToUpper(token)) {
		case opAnd:
			q.And()
		case opOr:
			q.Or()
		case opAndNot:
			q.AndNot()
		default:
			// Check if it's a field:value pattern
			if strings.Contains(token, ":") {
				parts := strings.SplitN(token, ":", 2)
				if len(parts) == 2 {
					field := parts[0]
					value, err := url.QueryUnescape(parts[1])
					if err != nil {
						return nil, fmt.Errorf("invalid URL encoding: %w", err)
					}

					switch queryField(field) {
					case fieldTitle:
						q.Title(value)
					case fieldAbstract:
						q.Abstract(value)
					case fieldAuthor:
						q.Author(value)
					case fieldCategory:
						q.Category(value)
					case fieldComment:
						q.Comment(value)
					case fieldJournal:
						q.Journal(value)
					case fieldAll:
						q.All(value)
					case fieldSubmittedDate:
						// Parse date range format: [YYYYMMDDTTTT TO YYYYMMDDTTTT]
						if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
							dateStr := strings.Trim(value, "[]")
							parts := strings.Split(dateStr, " TO ")
							if len(parts) == 2 {
								startDate, err1 := parseArxivDate(parts[0])
								endDate, err2 := parseArxivDate(parts[1])
								if err1 == nil && err2 == nil {
									if i > 0 && !isOperator(tokens[i-1]) {
										q.And()
									}
									q.nodes = append(q.nodes, &dateRangeQuery{
										field:     fieldSubmittedDate,
										startDate: startDate,
										endDate:   endDate,
									})
								} else {
									return nil, fmt.Errorf("invalid date format: %s", value)
								}
							} else {
								return nil, fmt.Errorf("invalid date range format: %s", value)
							}
						}
					default:
						return nil, fmt.Errorf("unknown field: %s", field)
					}
				}
			}
		}
	}

	return q, nil
}

// IsValidSearchQuery checks if a search query is valid.
func IsValidSearchQuery(query string) bool {
	_, err := ParseSearchQuery(query)
	return err == nil
}

// parseArxivDate parses a date in arXiv format (YYYYMMDDTTTT) to time.Time.
// The format is YYYYMMDDTTTT where TTTT is 24-hour time to the minute in GMT.
func parseArxivDate(dateStr string) (time.Time, error) {
	if len(dateStr) != 12 {
		return time.Time{}, fmt.Errorf("invalid date format: %s", dateStr)
	}

	year := dateStr[0:4]
	month := dateStr[4:6]
	day := dateStr[6:8]
	hour := dateStr[8:10]
	minute := dateStr[10:12]

	// Format for time.Parse: "2006-01-02 15:04"
	formatted := fmt.Sprintf("%s-%s-%s %s:%s", year, month, day, hour, minute)
	return time.Parse("2006-01-02 15:04", formatted)
}

// tokenizeQuery splits a query string into tokens.
func tokenizeQuery(query string) []string {
	var tokens []string
	var current strings.Builder
	inParens := 0
	inBrackets := false
	inFieldValue := false

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
		case ':':
			// When we see a colon, we're entering a field value
			current.WriteByte(ch)
			if !inBrackets && !inFieldValue {
				inFieldValue = true
			}
		case ' ':
			if inParens > 0 || inBrackets {
				current.WriteByte(ch)
			} else if inFieldValue {
				// Check if the next word is an operator
				nextWord := getNextWord(query, i+1)
				if isOperator(nextWord) {
					// End the field value
					if current.Len() > 0 {
						tokens = append(tokens, current.String())
						current.Reset()
					}
					inFieldValue = false
				} else if strings.Contains(nextWord, ":") {
					// Next token is another field
					if current.Len() > 0 {
						tokens = append(tokens, current.String())
						current.Reset()
					}
					inFieldValue = false
				} else {
					// Continue the field value
					current.WriteByte(ch)
				}
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

// getNextWord gets the next word starting from position i in the string
func getNextWord(s string, i int) string {
	// Skip leading spaces
	for i < len(s) && s[i] == ' ' {
		i++
	}

	start := i
	for i < len(s) && s[i] != ' ' && s[i] != '(' && s[i] != ')' {
		i++
	}

	if start < len(s) {
		return s[start:i]
	}
	return ""
}

// isOperator checks if a word is a boolean operator
func isOperator(word string) bool {
	op := queryOperator(strings.ToUpper(word))
	return op == opAnd || op == opOr || op == opAndNot
}
