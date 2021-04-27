// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: envoy/service/tap/v2alpha/tapds.proto

package envoy_service_tap_v2alpha

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
var _tapds_uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

// Validate checks the field values on TapResource with the rules defined in
// the proto definition for this message. If any rules are violated, an error
// is returned.
func (m *TapResource) Validate() error {
	if m == nil {
		return nil
	}

	if len(m.GetName()) < 1 {
		return TapResourceValidationError{
			field:  "Name",
			reason: "value length must be at least 1 bytes",
		}
	}

	if v, ok := interface{}(m.GetConfig()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return TapResourceValidationError{
				field:  "Config",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	return nil
}

// TapResourceValidationError is the validation error returned by
// TapResource.Validate if the designated constraints aren't met.
type TapResourceValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e TapResourceValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e TapResourceValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e TapResourceValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e TapResourceValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e TapResourceValidationError) ErrorName() string { return "TapResourceValidationError" }

// Error satisfies the builtin error interface
func (e TapResourceValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sTapResource.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = TapResourceValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = TapResourceValidationError{}
