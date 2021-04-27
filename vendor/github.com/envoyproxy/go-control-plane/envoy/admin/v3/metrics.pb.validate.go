// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: envoy/admin/v3/metrics.proto

package envoy_admin_v3

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

	"github.com/golang/protobuf/ptypes"
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
	_ = ptypes.DynamicAny{}
)

// define the regex for a UUID once up-front
var _metrics_uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

// Validate checks the field values on SimpleMetric with the rules defined in
// the proto definition for this message. If any rules are violated, an error
// is returned.
func (m *SimpleMetric) Validate() error {
	if m == nil {
		return nil
	}

	// no validation rules for Type

	// no validation rules for Value

	// no validation rules for Name

	return nil
}

// SimpleMetricValidationError is the validation error returned by
// SimpleMetric.Validate if the designated constraints aren't met.
type SimpleMetricValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e SimpleMetricValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e SimpleMetricValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e SimpleMetricValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e SimpleMetricValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e SimpleMetricValidationError) ErrorName() string { return "SimpleMetricValidationError" }

// Error satisfies the builtin error interface
func (e SimpleMetricValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sSimpleMetric.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = SimpleMetricValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = SimpleMetricValidationError{}
