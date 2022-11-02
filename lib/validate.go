package lib

import (
	"fmt"
	"regexp"
)

var (
	validBasicName = regexp.MustCompile("^[a-z0-9_-]+$")
)

func ValidateString(validator regexp.Regexp, value string, allowEmpty bool) error {
	if value == "" {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("value cannot be empty")
	} else if !validBasicName.MatchString(value) {
		return fmt.Errorf("value not in specified format")
	}
	return nil
}

func IsValidBasicName(value string, allowEmpty bool) bool {
	if err := ValidateString(*validBasicName, value, allowEmpty); err != nil {
		return false
	} else {
		return true
	}
}
