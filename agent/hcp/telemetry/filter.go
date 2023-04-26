package telemetry

import (
	"fmt"
	"regexp"

	"github.com/hashicorp/go-multierror"
)

// FilterList holds a map of filters, i.e. regular expressions.
// These filters are used to identify which Consul metrics can be transmitted to HCP.
type FilterList struct {
	filters map[string]*regexp.Regexp
}

// NewFilterList returns a FilterList which holds valid regex
// used to filter metrics. It will not fail if invalid REGEX is given, but returns a list of errors.
func NewFilterList(filters []string) (*FilterList, error) {
	var mErr error
	compiledList := make(map[string]*regexp.Regexp, len(filters))
	for idx, filter := range filters {
		re, err := regexp.Compile(filter)
		if err != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("compilation of filter at index %d failed: %w", idx, err))
		}
		compiledList[filter] = re
	}
	f := &FilterList{
		filters: compiledList,
	}
	return f, mErr
}

// Match returns true if the metric name matches a REGEX in the allowed metric filters.
func (fl *FilterList) Match(name string) bool {
	for _, re := range fl.filters {
		if re.Match([]byte(name)) {
			return true
		}
	}
	return false
}
