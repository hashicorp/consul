package telemetry

import (
	"fmt"
	"regexp"
)

// FilterList holds a map of filters, i.e. regular expressions.
// These filters are used to identify which Consul metrics can be transmitted to HCP.
type FilterList struct {
	filters map[string]*regexp.Regexp
}

// NewFilterList returns a FilterList which holds valid regex
// used to filter metrics. It will not fail if invalid REGEX is given, but returns a list of errors.
func NewFilterList(filters []string) (*FilterList, []error) {
	errs := make([]error, 0)
	f := &FilterList{}
	compiledList := map[string]*regexp.Regexp{}
	for idx, filter := range filters {
		re, err := regexp.Compile(filter)
		if err != nil {
			errs = append(errs, fmt.Errorf("compilation of filter at index %d failed: %w", idx, err))
		}
		compiledList[filter] = re
	}
	f.filters = compiledList
	return f, errs
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
