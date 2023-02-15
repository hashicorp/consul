package telemetry

import (
	"fmt"
	"regexp"
)

type FilterList struct {
	filters map[string]*regexp.Regexp
}

func (fl *FilterList) Match(name string) bool {
	for _, re := range fl.filters {
		if re.Match([]byte(name)) {
			return true
		}
	}
	return false
}

func (fl *FilterList) Set(filters []string) error {
	compiledList := map[string]*regexp.Regexp{}
	for idx, filter := range filters {
		re, err := regexp.Compile(filter)
		if err != nil {
			return fmt.Errorf("compilation of filter at index %d failed: %w", idx)
		}
		compiledList[filter] = re
	}
	fl.filters = compiledList
	return nil
}
