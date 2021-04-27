// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: envoy/service/ratelimit/v3/rls.proto

package envoy_service_ratelimit_v3

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
var _rls_uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

// Validate checks the field values on RateLimitRequest with the rules defined
// in the proto definition for this message. If any rules are violated, an
// error is returned.
func (m *RateLimitRequest) Validate() error {
	if m == nil {
		return nil
	}

	// no validation rules for Domain

	for idx, item := range m.GetDescriptors() {
		_, _ = idx, item

		if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return RateLimitRequestValidationError{
					field:  fmt.Sprintf("Descriptors[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	// no validation rules for HitsAddend

	return nil
}

// RateLimitRequestValidationError is the validation error returned by
// RateLimitRequest.Validate if the designated constraints aren't met.
type RateLimitRequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e RateLimitRequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e RateLimitRequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e RateLimitRequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e RateLimitRequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e RateLimitRequestValidationError) ErrorName() string { return "RateLimitRequestValidationError" }

// Error satisfies the builtin error interface
func (e RateLimitRequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sRateLimitRequest.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = RateLimitRequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = RateLimitRequestValidationError{}

// Validate checks the field values on RateLimitResponse with the rules defined
// in the proto definition for this message. If any rules are violated, an
// error is returned.
func (m *RateLimitResponse) Validate() error {
	if m == nil {
		return nil
	}

	// no validation rules for OverallCode

	for idx, item := range m.GetStatuses() {
		_, _ = idx, item

		if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return RateLimitResponseValidationError{
					field:  fmt.Sprintf("Statuses[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	for idx, item := range m.GetResponseHeadersToAdd() {
		_, _ = idx, item

		if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return RateLimitResponseValidationError{
					field:  fmt.Sprintf("ResponseHeadersToAdd[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	for idx, item := range m.GetRequestHeadersToAdd() {
		_, _ = idx, item

		if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return RateLimitResponseValidationError{
					field:  fmt.Sprintf("RequestHeadersToAdd[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	return nil
}

// RateLimitResponseValidationError is the validation error returned by
// RateLimitResponse.Validate if the designated constraints aren't met.
type RateLimitResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e RateLimitResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e RateLimitResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e RateLimitResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e RateLimitResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e RateLimitResponseValidationError) ErrorName() string {
	return "RateLimitResponseValidationError"
}

// Error satisfies the builtin error interface
func (e RateLimitResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sRateLimitResponse.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = RateLimitResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = RateLimitResponseValidationError{}

// Validate checks the field values on RateLimitResponse_RateLimit with the
// rules defined in the proto definition for this message. If any rules are
// violated, an error is returned.
func (m *RateLimitResponse_RateLimit) Validate() error {
	if m == nil {
		return nil
	}

	// no validation rules for RequestsPerUnit

	// no validation rules for Unit

	return nil
}

// RateLimitResponse_RateLimitValidationError is the validation error returned
// by RateLimitResponse_RateLimit.Validate if the designated constraints
// aren't met.
type RateLimitResponse_RateLimitValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e RateLimitResponse_RateLimitValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e RateLimitResponse_RateLimitValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e RateLimitResponse_RateLimitValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e RateLimitResponse_RateLimitValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e RateLimitResponse_RateLimitValidationError) ErrorName() string {
	return "RateLimitResponse_RateLimitValidationError"
}

// Error satisfies the builtin error interface
func (e RateLimitResponse_RateLimitValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sRateLimitResponse_RateLimit.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = RateLimitResponse_RateLimitValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = RateLimitResponse_RateLimitValidationError{}

// Validate checks the field values on RateLimitResponse_DescriptorStatus with
// the rules defined in the proto definition for this message. If any rules
// are violated, an error is returned.
func (m *RateLimitResponse_DescriptorStatus) Validate() error {
	if m == nil {
		return nil
	}

	// no validation rules for Code

	if v, ok := interface{}(m.GetCurrentLimit()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return RateLimitResponse_DescriptorStatusValidationError{
				field:  "CurrentLimit",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	// no validation rules for LimitRemaining

	return nil
}

// RateLimitResponse_DescriptorStatusValidationError is the validation error
// returned by RateLimitResponse_DescriptorStatus.Validate if the designated
// constraints aren't met.
type RateLimitResponse_DescriptorStatusValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e RateLimitResponse_DescriptorStatusValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e RateLimitResponse_DescriptorStatusValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e RateLimitResponse_DescriptorStatusValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e RateLimitResponse_DescriptorStatusValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e RateLimitResponse_DescriptorStatusValidationError) ErrorName() string {
	return "RateLimitResponse_DescriptorStatusValidationError"
}

// Error satisfies the builtin error interface
func (e RateLimitResponse_DescriptorStatusValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sRateLimitResponse_DescriptorStatus.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = RateLimitResponse_DescriptorStatusValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = RateLimitResponse_DescriptorStatusValidationError{}
