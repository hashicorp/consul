// Package validation provides standalone utility functions 
// for validating inputs. 
package validation

import (
	"errors"
	"regexp"
)

// matches valid DNS labels according to RFC 1123
var validDNSLabel = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,62}[a-zA-Z0-9])?$`)

// IsValidDNSLabel returns true if the string given is a valid DNS label (RFC 1123).
// Note: the only difference between RFC 1035 and RFC 1123 labels is that in
// RFC 1123 labels can begin with a number.
func IsValidDNSLabel(name string) bool {
	return validDNSLabel.MatchString(name)
}

// RequireValidDNSLabel is similar to IsValidDNSLabel except it returns an error
// instead of false when name is not a valid DNS label. The error will contain
// reference to what constitutes a valid DNS label.
func RequireValidDNSLabel(name string) error {
	if !validDNSLabel.MatchString(name) {
		return errors.New("a valid DNS label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character")
	}
	return nil
}
