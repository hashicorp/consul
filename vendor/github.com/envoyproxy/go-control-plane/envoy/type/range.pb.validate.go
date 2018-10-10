// Code generated by protoc-gen-validate
// source: envoy/type/range.proto
// DO NOT EDIT!!!

package envoy_type

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogo/protobuf/types"
)

// ensure the imports are used
var (
	_ = bytes.MinRead
	_ = errors.New("")
	_ = fmt.Print
	_ = utf8.UTFMax
	_ = (*regexp.Regexp)(nil)
	_ = (*strings.Reader)(nil)
	_ = net.IPv4len
	_ = time.Duration(0)
	_ = (*url.URL)(nil)
	_ = (*mail.Address)(nil)
	_ = types.DynamicAny{}
)

// Validate checks the field values on Int64Range with the rules defined in the
// proto definition for this message. If any rules are violated, an error is returned.
func (m *Int64Range) Validate() error {
	if m == nil {
		return nil
	}

	// no validation rules for Start

	// no validation rules for End

	return nil
}

// Int64RangeValidationError is the validation error returned by
// Int64Range.Validate if the designated constraints aren't met.
type Int64RangeValidationError struct {
	Field  string
	Reason string
	Cause  error
	Key    bool
}

// Error satisfies the builtin error interface
func (e Int64RangeValidationError) Error() string {
	cause := ""
	if e.Cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.Cause)
	}

	key := ""
	if e.Key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sInt64Range.%s: %s%s",
		key,
		e.Field,
		e.Reason,
		cause)
}

var _ error = Int64RangeValidationError{}

// Validate checks the field values on DoubleRange with the rules defined in
// the proto definition for this message. If any rules are violated, an error
// is returned.
func (m *DoubleRange) Validate() error {
	if m == nil {
		return nil
	}

	// no validation rules for Start

	// no validation rules for End

	return nil
}

// DoubleRangeValidationError is the validation error returned by
// DoubleRange.Validate if the designated constraints aren't met.
type DoubleRangeValidationError struct {
	Field  string
	Reason string
	Cause  error
	Key    bool
}

// Error satisfies the builtin error interface
func (e DoubleRangeValidationError) Error() string {
	cause := ""
	if e.Cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.Cause)
	}

	key := ""
	if e.Key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sDoubleRange.%s: %s%s",
		key,
		e.Field,
		e.Reason,
		cause)
}

var _ error = DoubleRangeValidationError{}
