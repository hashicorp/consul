// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: envoy/config/trace/v2alpha/xray.proto

package envoy_config_trace_v2alpha

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
var _xray_uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

// Validate checks the field values on XRayConfig with the rules defined in the
// proto definition for this message. If any rules are violated, an error is returned.
func (m *XRayConfig) Validate() error {
	if m == nil {
		return nil
	}

	if v, ok := interface{}(m.GetDaemonEndpoint()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return XRayConfigValidationError{
				field:  "DaemonEndpoint",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	if utf8.RuneCountInString(m.GetSegmentName()) < 1 {
		return XRayConfigValidationError{
			field:  "SegmentName",
			reason: "value length must be at least 1 runes",
		}
	}

	if v, ok := interface{}(m.GetSamplingRuleManifest()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return XRayConfigValidationError{
				field:  "SamplingRuleManifest",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	return nil
}

// XRayConfigValidationError is the validation error returned by
// XRayConfig.Validate if the designated constraints aren't met.
type XRayConfigValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e XRayConfigValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e XRayConfigValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e XRayConfigValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e XRayConfigValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e XRayConfigValidationError) ErrorName() string { return "XRayConfigValidationError" }

// Error satisfies the builtin error interface
func (e XRayConfigValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sXRayConfig.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = XRayConfigValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = XRayConfigValidationError{}
