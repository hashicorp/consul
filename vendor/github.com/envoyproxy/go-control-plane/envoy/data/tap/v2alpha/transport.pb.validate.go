// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: envoy/data/tap/v2alpha/transport.proto

package envoy_data_tap_v2alpha

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
var _transport_uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

// Validate checks the field values on Connection with the rules defined in the
// proto definition for this message. If any rules are violated, an error is returned.
func (m *Connection) Validate() error {
	if m == nil {
		return nil
	}

	if v, ok := interface{}(m.GetLocalAddress()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return ConnectionValidationError{
				field:  "LocalAddress",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	if v, ok := interface{}(m.GetRemoteAddress()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return ConnectionValidationError{
				field:  "RemoteAddress",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	return nil
}

// ConnectionValidationError is the validation error returned by
// Connection.Validate if the designated constraints aren't met.
type ConnectionValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ConnectionValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ConnectionValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ConnectionValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ConnectionValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ConnectionValidationError) ErrorName() string { return "ConnectionValidationError" }

// Error satisfies the builtin error interface
func (e ConnectionValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sConnection.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ConnectionValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ConnectionValidationError{}

// Validate checks the field values on SocketEvent with the rules defined in
// the proto definition for this message. If any rules are violated, an error
// is returned.
func (m *SocketEvent) Validate() error {
	if m == nil {
		return nil
	}

	if v, ok := interface{}(m.GetTimestamp()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return SocketEventValidationError{
				field:  "Timestamp",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	switch m.EventSelector.(type) {

	case *SocketEvent_Read_:

		if v, ok := interface{}(m.GetRead()).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return SocketEventValidationError{
					field:  "Read",
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	case *SocketEvent_Write_:

		if v, ok := interface{}(m.GetWrite()).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return SocketEventValidationError{
					field:  "Write",
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	case *SocketEvent_Closed_:

		if v, ok := interface{}(m.GetClosed()).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return SocketEventValidationError{
					field:  "Closed",
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	return nil
}

// SocketEventValidationError is the validation error returned by
// SocketEvent.Validate if the designated constraints aren't met.
type SocketEventValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e SocketEventValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e SocketEventValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e SocketEventValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e SocketEventValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e SocketEventValidationError) ErrorName() string { return "SocketEventValidationError" }

// Error satisfies the builtin error interface
func (e SocketEventValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sSocketEvent.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = SocketEventValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = SocketEventValidationError{}

// Validate checks the field values on SocketBufferedTrace with the rules
// defined in the proto definition for this message. If any rules are
// violated, an error is returned.
func (m *SocketBufferedTrace) Validate() error {
	if m == nil {
		return nil
	}

	// no validation rules for TraceId

	if v, ok := interface{}(m.GetConnection()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return SocketBufferedTraceValidationError{
				field:  "Connection",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	for idx, item := range m.GetEvents() {
		_, _ = idx, item

		if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return SocketBufferedTraceValidationError{
					field:  fmt.Sprintf("Events[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	// no validation rules for ReadTruncated

	// no validation rules for WriteTruncated

	return nil
}

// SocketBufferedTraceValidationError is the validation error returned by
// SocketBufferedTrace.Validate if the designated constraints aren't met.
type SocketBufferedTraceValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e SocketBufferedTraceValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e SocketBufferedTraceValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e SocketBufferedTraceValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e SocketBufferedTraceValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e SocketBufferedTraceValidationError) ErrorName() string {
	return "SocketBufferedTraceValidationError"
}

// Error satisfies the builtin error interface
func (e SocketBufferedTraceValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sSocketBufferedTrace.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = SocketBufferedTraceValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = SocketBufferedTraceValidationError{}

// Validate checks the field values on SocketStreamedTraceSegment with the
// rules defined in the proto definition for this message. If any rules are
// violated, an error is returned.
func (m *SocketStreamedTraceSegment) Validate() error {
	if m == nil {
		return nil
	}

	// no validation rules for TraceId

	switch m.MessagePiece.(type) {

	case *SocketStreamedTraceSegment_Connection:

		if v, ok := interface{}(m.GetConnection()).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return SocketStreamedTraceSegmentValidationError{
					field:  "Connection",
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	case *SocketStreamedTraceSegment_Event:

		if v, ok := interface{}(m.GetEvent()).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return SocketStreamedTraceSegmentValidationError{
					field:  "Event",
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	return nil
}

// SocketStreamedTraceSegmentValidationError is the validation error returned
// by SocketStreamedTraceSegment.Validate if the designated constraints aren't met.
type SocketStreamedTraceSegmentValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e SocketStreamedTraceSegmentValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e SocketStreamedTraceSegmentValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e SocketStreamedTraceSegmentValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e SocketStreamedTraceSegmentValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e SocketStreamedTraceSegmentValidationError) ErrorName() string {
	return "SocketStreamedTraceSegmentValidationError"
}

// Error satisfies the builtin error interface
func (e SocketStreamedTraceSegmentValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sSocketStreamedTraceSegment.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = SocketStreamedTraceSegmentValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = SocketStreamedTraceSegmentValidationError{}

// Validate checks the field values on SocketEvent_Read with the rules defined
// in the proto definition for this message. If any rules are violated, an
// error is returned.
func (m *SocketEvent_Read) Validate() error {
	if m == nil {
		return nil
	}

	if v, ok := interface{}(m.GetData()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return SocketEvent_ReadValidationError{
				field:  "Data",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	return nil
}

// SocketEvent_ReadValidationError is the validation error returned by
// SocketEvent_Read.Validate if the designated constraints aren't met.
type SocketEvent_ReadValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e SocketEvent_ReadValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e SocketEvent_ReadValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e SocketEvent_ReadValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e SocketEvent_ReadValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e SocketEvent_ReadValidationError) ErrorName() string { return "SocketEvent_ReadValidationError" }

// Error satisfies the builtin error interface
func (e SocketEvent_ReadValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sSocketEvent_Read.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = SocketEvent_ReadValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = SocketEvent_ReadValidationError{}

// Validate checks the field values on SocketEvent_Write with the rules defined
// in the proto definition for this message. If any rules are violated, an
// error is returned.
func (m *SocketEvent_Write) Validate() error {
	if m == nil {
		return nil
	}

	if v, ok := interface{}(m.GetData()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return SocketEvent_WriteValidationError{
				field:  "Data",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	// no validation rules for EndStream

	return nil
}

// SocketEvent_WriteValidationError is the validation error returned by
// SocketEvent_Write.Validate if the designated constraints aren't met.
type SocketEvent_WriteValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e SocketEvent_WriteValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e SocketEvent_WriteValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e SocketEvent_WriteValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e SocketEvent_WriteValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e SocketEvent_WriteValidationError) ErrorName() string {
	return "SocketEvent_WriteValidationError"
}

// Error satisfies the builtin error interface
func (e SocketEvent_WriteValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sSocketEvent_Write.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = SocketEvent_WriteValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = SocketEvent_WriteValidationError{}

// Validate checks the field values on SocketEvent_Closed with the rules
// defined in the proto definition for this message. If any rules are
// violated, an error is returned.
func (m *SocketEvent_Closed) Validate() error {
	if m == nil {
		return nil
	}

	return nil
}

// SocketEvent_ClosedValidationError is the validation error returned by
// SocketEvent_Closed.Validate if the designated constraints aren't met.
type SocketEvent_ClosedValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e SocketEvent_ClosedValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e SocketEvent_ClosedValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e SocketEvent_ClosedValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e SocketEvent_ClosedValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e SocketEvent_ClosedValidationError) ErrorName() string {
	return "SocketEvent_ClosedValidationError"
}

// Error satisfies the builtin error interface
func (e SocketEvent_ClosedValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sSocketEvent_Closed.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = SocketEvent_ClosedValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = SocketEvent_ClosedValidationError{}
