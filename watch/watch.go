package watch

import (
	"fmt"
	"strings"
	"sync"

	"github.com/armon/consul-api"
)

// WatchPlan is the parsed version of a watch specification. A watch provides
// the details of a query, which generates a view into the Consul data store.
// This view is watched for changes and a handler is invoked to take any
// appropriate actions.
type WatchPlan struct {
	Query      string
	Datacenter string
	Token      string
	Type       string
	Func       WatchFunc
	Handler    HandlerFunc

	address    string
	client     *consulapi.Client
	lastIndex  uint64
	lastResult interface{}

	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex
}

// WatchFunc is used to watch for a diff
type WatchFunc func(*WatchPlan) (uint64, interface{}, error)

// HandlerFunc is used to handle new data
type HandlerFunc func(uint64, interface{})

// Parse takes a watch query and compiles it into a WatchPlan or an error
func Parse(query string) (*WatchPlan, error) {
	tokens, err := tokenize(query)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse: %v", err)
	}
	params := collapse(tokens)
	plan := &WatchPlan{
		Query:  query,
		stopCh: make(chan struct{}),
	}

	// Parse the generic parameters
	if err := assignValue(params, "datacenter", &plan.Datacenter); err != nil {
		return nil, err
	}
	if err := assignValue(params, "token", &plan.Token); err != nil {
		return nil, err
	}
	if err := assignValue(params, "type", &plan.Type); err != nil {
		return nil, err
	}

	// Ensure there is a watch type
	if plan.Type == "" {
		return nil, fmt.Errorf("Watch type must be specified")
	}

	// Look for a factory function
	factory := watchFuncFactory[plan.Type]
	if factory == nil {
		return nil, fmt.Errorf("Unsupported watch type: %s", plan.Type)
	}

	// Get the watch func
	fn, err := factory(params)
	if err != nil {
		return nil, err
	}
	plan.Func = fn

	// Ensure all parameters are consumed
	if len(params) != 0 {
		var bad []string
		for key := range params {
			bad = append(bad, key)
		}
		return nil, fmt.Errorf("Invalid parameters: %v", bad)
	}
	return plan, nil
}

// assignValue is used to extract a value ensuring it is only
// defined once
func assignValue(params map[string][]string, name string, out *string) error {
	if vals, ok := params[name]; ok {
		if len(vals) != 1 {
			return fmt.Errorf("Multiple definitions of %s", name)
		}
		*out = vals[0]
		delete(params, name)
	}
	return nil
}

// token is used to represent a "datacenter:foobar" pair, where
// datacenter is the param and foobar is the value
type token struct {
	param string
	val   string
}

func (t *token) GoString() string {
	return fmt.Sprintf("%#v", *t)
}

// tokenize splits a query string into a slice of tokens
func tokenize(query string) ([]*token, error) {
	var tokens []*token
	for i := 0; i < len(query); i++ {
		char := query[i]

		// Ignore whitespace
		if char == ' ' || char == '\t' || char == '\n' {
			continue
		}

		// Read the next token
		next, offset, err := readToken(query[i:])
		if err != nil {
			return nil, err
		}

		// Store the token
		tokens = append(tokens, next)

		// Increment the offset
		i += offset
	}
	return tokens, nil
}

// readToken is used to read a single token
func readToken(query string) (*token, int, error) {
	// Get the token
	param, offset, err := readParameter(query)
	if err != nil {
		return nil, 0, err
	}

	// Get the value
	query = query[offset:]
	val, offset2, err := readValue(query)
	if err != nil {
		return nil, 0, err
	}

	// Return the new token
	token := &token{
		param: param,
		val:   val,
	}
	return token, offset + offset2, nil
}

// readParameter scans for the next parameter
func readParameter(query string) (string, int, error) {
	for i := 0; i < len(query); i++ {
		char := query[i]
		if char == ':' {
			if i == 0 {
				return "", 0, fmt.Errorf("Missing parameter name")
			} else {
				return query[:i], i + 1, nil
			}
		}
	}
	return "", 0, fmt.Errorf("Parameter delimiter not found")
}

// readValue is used to scan for the next value
func readValue(query string) (string, int, error) {
	// Handle quoted values
	if query[0] == '\'' || query[0] == '"' {
		quoteChar := query[0:1]
		endChar := strings.Index(query[1:], quoteChar)
		if endChar == -1 {
			return "", 0, fmt.Errorf("Missing end of quotation")
		}
		return query[1 : endChar+1], endChar + 2, nil
	}

	// Look for white space
	endChar := strings.IndexAny(query, " \t\n")
	if endChar == -1 {
		return query, len(query), nil
	}
	return query[:endChar], endChar, nil
}

// collapse is used to collapse a token stream into a map
// of parameter name to list of values.
func collapse(tokens []*token) map[string][]string {
	out := make(map[string][]string)
	for _, t := range tokens {
		existing := out[t.param]
		out[t.param] = append(existing, t.val)
	}
	return out
}
