package telemetry

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/go-multierror"
)

// newFilterRegex returns a valid regex used to filter metrics.
// It will fail if there are 0 valid regex filters given.
func newFilterRegex(filters []string) (*regexp.Regexp, error) {
	var mErr error
	validFilters := make([]string, 0, len(filters))
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

	return composedRegex, nil
}
