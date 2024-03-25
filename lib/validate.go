package lib

import (
	"fmt"
	"regexp"
)

var (
	validBasicName = regexp.MustCompile("^[a-z0-9_-]+$")
)

func ValidateBasicName(field string, value string, allowEmpty bool) error {
	if value == "" {
		if allowEmpty {
			return nil
		} else {
			return fmt.Errorf("%s cannot be empty", field)
		}
	} else if !validBasicName.MatchString(value) {
		return fmt.Errorf("%s can only contain lowercase alphanumeric, - or _ characters."+
			" received: %q", field, value)
	}
	return nil
}
