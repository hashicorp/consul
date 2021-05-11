// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: envoy/config/filter/network/kafka_broker/v2alpha1/kafka_broker.proto

package envoy_config_filter_network_kafka_broker_v2alpha1

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
var _kafka_broker_uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

// Validate checks the field values on KafkaBroker with the rules defined in
// the proto definition for this message. If any rules are violated, an error
// is returned.
func (m *KafkaBroker) Validate() error {
	if m == nil {
		return nil
	}

	if len(m.GetStatPrefix()) < 1 {
		return KafkaBrokerValidationError{
			field:  "StatPrefix",
			reason: "value length must be at least 1 bytes",
		}
	}

	return nil
}

// KafkaBrokerValidationError is the validation error returned by
// KafkaBroker.Validate if the designated constraints aren't met.
type KafkaBrokerValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e KafkaBrokerValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e KafkaBrokerValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e KafkaBrokerValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e KafkaBrokerValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e KafkaBrokerValidationError) ErrorName() string { return "KafkaBrokerValidationError" }

// Error satisfies the builtin error interface
func (e KafkaBrokerValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sKafkaBroker.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = KafkaBrokerValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = KafkaBrokerValidationError{}
