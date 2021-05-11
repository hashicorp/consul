// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: envoy/config/transport_socket/tap/v2alpha/tap.proto

package envoy_config_transport_socket_tap_v2alpha

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
var _tap_uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

// Validate checks the field values on Tap with the rules defined in the proto
// definition for this message. If any rules are violated, an error is returned.
func (m *Tap) Validate() error {
	if m == nil {
		return nil
	}

	if m.GetCommonConfig() == nil {
		return TapValidationError{
			field:  "CommonConfig",
			reason: "value is required",
		}
	}

	if v, ok := interface{}(m.GetCommonConfig()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return TapValidationError{
				field:  "CommonConfig",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	if m.GetTransportSocket() == nil {
		return TapValidationError{
			field:  "TransportSocket",
			reason: "value is required",
		}
	}

	if v, ok := interface{}(m.GetTransportSocket()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return TapValidationError{
				field:  "TransportSocket",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	return nil
}

// TapValidationError is the validation error returned by Tap.Validate if the
// designated constraints aren't met.
type TapValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e TapValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e TapValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e TapValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e TapValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e TapValidationError) ErrorName() string { return "TapValidationError" }

// Error satisfies the builtin error interface
func (e TapValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sTap.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = TapValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = TapValidationError{}
