// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: envoy/extensions/filters/network/echo/v3/echo.proto

package envoy_extensions_filters_network_echo_v3

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
var _echo_uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

// Validate checks the field values on Echo with the rules defined in the proto
// definition for this message. If any rules are violated, an error is returned.
func (m *Echo) Validate() error {
	if m == nil {
		return nil
	}

	return nil
}

// EchoValidationError is the validation error returned by Echo.Validate if the
// designated constraints aren't met.
type EchoValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e EchoValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e EchoValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e EchoValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e EchoValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e EchoValidationError) ErrorName() string { return "EchoValidationError" }

// Error satisfies the builtin error interface
func (e EchoValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sEcho.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = EchoValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = EchoValidationError{}
