package telemetry

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/go-multierror"
)

// filterList holds a map of filters, i.e. regular expressions.
// These filters are used to identify which Consul metrics can be transmitted to HCP.
type filterList struct {
	isValid *regexp.Regexp
}

// newFilterList returns a FilterList which holds valid regex used to filter metrics.
// It will fail if there are 0 valid regex filters given.
func newFilterList(filters []string) (*filterList, error) {
	var mErr error
	var validFilters []string
	for _, filter := range filters {
		_, err := regexp.Compile(filter)
		if err != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("compilation of filter %q failed: %w", filter, err))
			continue
		}
		validFilters = append(validFilters, filter)
	}

	if len(validFilters) == 0 {
		return nil, multierror.Append(mErr, fmt.Errorf("no valid filters"))
	}

	// Combine the valid regex strings with an OR.
	finalRegex := strings.Join(validFilters, "|")
	composedRegex, err := regexp.Compile(finalRegex)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex: %w", err)
	}

	f := &filterList{
		isValid: composedRegex,
	}
	return f, nil
}

// Match returns true if the metric name matches a REGEX in the allowed metric filters.
func (fl *filterList) Match(name string) bool {
	return fl.isValid.MatchString(name)
}
